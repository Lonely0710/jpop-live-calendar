// Package catalog is the in-memory store of the calendar.
//
// There is no database. Announcements are kept in a dedup.Store; after each
// sync the catalog rebuilds an immutable Snapshot from all of them, which
// the UI reads without locks. On launch internal/cache restores the
// announcements saved after the last sync before the first snapshot.
package catalog

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
)

// Catalog is safe for use from the crawler goroutines and the UI thread.
type Catalog struct {
	store *dedup.Store
	snap  atomic.Pointer[Snapshot]

	mu        sync.Mutex
	opts      dedup.Options
	localeGen uint64
}

// New returns an empty catalog.
func New(opts dedup.Options) *Catalog {
	c := &Catalog{store: dedup.NewStore(), opts: opts}
	c.snap.Store(&Snapshot{})
	return c
}

// IngestStats counts what an ingest did.
type IngestStats struct{ Fetched, New, Updated int }

// Ingest stores announcements; call Rebuild afterwards.
func (c *Catalog) Ingest(anns []domain.PublicAnnouncement) IngestStats {
	st := IngestStats{Fetched: len(anns)}
	for _, a := range anns {
		switch c.store.Put(a) {
		case dedup.New:
			st.New++
		case dedup.Updated:
			st.Updated++
		}
	}
	return st
}

// Has reports whether the full text of an announcement URL is stored, so
// adapters can skip fetching it again.
func (c *Catalog) Has(url string) bool { return c.store.Has(url) }

// Announcements returns every stored announcement, oldest first, including
// those hidden by the source filter: the cache keeps them too.
func (c *Catalog) Announcements() []domain.PublicAnnouncement { return c.store.All() }

// Restore puts announcements saved by the cache back into the store and
// rebuilds. Their revisions are kept, so a later change continues the
// history instead of starting over at revision 1.
func (c *Catalog) Restore(anns []domain.PublicAnnouncement) {
	for _, a := range anns {
		c.store.Put(a)
	}
	c.Rebuild()
}

// SetInclude changes which sources are shown and rebuilds.
func (c *Catalog) SetInclude(include func(sourceID string) bool) {
	c.mu.Lock()
	c.opts.Include = include
	c.mu.Unlock()
	c.Rebuild()
}

// SetLocale changes the language of the derived text — notes, change logs
// and the search index. Call Rebuild afterwards; a build that started
// before this call never replaces the snapshot.
func (c *Catalog) SetLocale(tag string) {
	c.mu.Lock()
	c.opts.Locale = tag
	c.localeGen++
	c.mu.Unlock()
}

// Rebuild merges every stored announcement into a new snapshot.
func (c *Catalog) Rebuild() *Snapshot {
	c.mu.Lock()
	opts := c.opts
	gen := c.localeGen
	c.mu.Unlock()
	res := dedup.Build(c.store.All(), opts)
	s := newSnapshot(res, c.store.Count())
	c.mu.Lock()
	if gen == c.localeGen {
		c.snap.Store(s)
	}
	current := c.snap.Load()
	c.mu.Unlock()
	return current
}

// Snapshot returns the latest snapshot; never nil.
func (c *Catalog) Snapshot() *Snapshot { return c.snap.Load() }

// Snapshot is an immutable view of the calendar.
type Snapshot struct {
	Events        []domain.PublicEvent
	Rounds        []domain.PublicTicketRound
	Artists       []domain.Artist
	Venues        []domain.Venue
	Announcements []domain.PublicAnnouncement
	Entries       []domain.Entry
	PerSource     map[string]int // announcements stored per source
	BuiltAt       time.Time

	events  map[string]int
	rounds  map[string]int
	artists map[string]int
	anns    map[string]int
	entries map[string]int
}

func newSnapshot(r dedup.Result, perSource map[string]int) *Snapshot {
	s := &Snapshot{
		Events: r.Events, Rounds: r.Rounds, Artists: r.Artists, Venues: r.Venues,
		Announcements: r.Announcements, Entries: r.Entries, PerSource: perSource, BuiltAt: time.Now(),
		events: map[string]int{}, rounds: map[string]int{}, artists: map[string]int{}, anns: map[string]int{}, entries: map[string]int{},
	}
	for i, e := range s.Events {
		s.events[e.ID] = i
	}
	for i, r := range s.Rounds {
		s.rounds[r.ID] = i
	}
	for i, a := range s.Artists {
		s.artists[a.ID] = i
	}
	for i, a := range s.Announcements {
		s.anns[a.ID] = i
	}
	for i, e := range s.Entries {
		s.entries[e.ID] = i
	}
	return s
}

// Between returns the entries dated first to last (inclusive), in order.
func (s *Snapshot) Between(first, last domain.Date) []domain.Entry {
	i := sort.Search(len(s.Entries), func(i int) bool { return !s.Entries[i].Date.Before(first) })
	var out []domain.Entry
	for ; i < len(s.Entries) && !last.Before(s.Entries[i].Date); i++ {
		out = append(out, s.Entries[i])
	}
	return out
}

// Entry returns an entry by ID.
func (s *Snapshot) Entry(id string) (domain.Entry, bool) {
	if i, ok := s.entries[id]; ok {
		return s.Entries[i], true
	}
	return domain.Entry{}, false
}

// Event returns a show by ID.
func (s *Snapshot) Event(id string) (domain.PublicEvent, bool) {
	if i, ok := s.events[id]; ok {
		return s.Events[i], true
	}
	return domain.PublicEvent{}, false
}

// Round returns a ticket round by ID.
func (s *Snapshot) Round(id string) (domain.PublicTicketRound, bool) {
	if i, ok := s.rounds[id]; ok {
		return s.Rounds[i], true
	}
	return domain.PublicTicketRound{}, false
}

// Artist returns an artist by ID.
func (s *Snapshot) Artist(id string) (domain.Artist, bool) {
	if i, ok := s.artists[id]; ok {
		return s.Artists[i], true
	}
	return domain.Artist{}, false
}

// Announcement returns an announcement by ID.
func (s *Snapshot) Announcement(id string) (domain.PublicAnnouncement, bool) {
	if i, ok := s.anns[id]; ok {
		return s.Announcements[i], true
	}
	return domain.PublicAnnouncement{}, false
}

// RoundsFor returns the rounds that sell a show, by deadline.
func (s *Snapshot) RoundsFor(eventID string) []domain.PublicTicketRound {
	var out []domain.PublicTicketRound
	for _, r := range s.Rounds {
		for _, id := range r.EventIDs {
			if id == eventID {
				out = append(out, r)
				break
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return firstMoment(out[i]).Before(firstMoment(out[j])) })
	return out
}

func firstMoment(r domain.PublicTicketRound) time.Time {
	for _, m := range []*domain.Moment{r.OpensAt, r.ClosesAt, r.ResultAt} {
		if m != nil {
			return m.Time(domain.Tokyo)
		}
	}
	return time.Time{}
}

// Tour groups an artist's shows.
type Tour struct {
	Name   string // "" for shows outside a named tour
	Events []domain.PublicEvent
	Rounds []domain.PublicTicketRound
}

// ArtistSummary is an artist with their shows.
type ArtistSummary struct {
	Artist  domain.Artist
	Tours   []Tour
	Shows   int
	Next    *domain.PublicEvent // next show on or after today
	Pending int                 // rounds still open or not yet open
}

// ArtistSummaries lists the followed artists and every other artist with
// at least one show or round: artists with shows first, then by name.
func (s *Snapshot) ArtistSummaries(today domain.Date) []ArtistSummary {
	var out []ArtistSummary
	for _, a := range s.Artists {
		sum := ArtistSummary{Artist: a}
		tours := map[string]*Tour{}
		var order []string
		tour := func(name string) *Tour {
			t, ok := tours[name]
			if !ok {
				t = &Tour{Name: name}
				tours[name] = t
				order = append(order, name)
			}
			return t
		}
		// Events outside a named tour often still name their tour in the
		// title (a parser miss): when the same title covers several shows,
		// treat it as their tour instead of one giant "single shows" block.
		titles := map[string]int{}
		for _, e := range s.Events {
			if has(e.ArtistIDs, a.ID) && e.Tour == "" && e.Title != "" {
				titles[e.Title]++
			}
		}
		eventTour := map[string]string{}
		for _, e := range s.Events {
			if !has(e.ArtistIDs, a.ID) {
				continue
			}
			name := e.Tour
			if name == "" && titles[e.Title] > 1 {
				name = e.Title
			}
			t := tour(name)
			eventTour[e.ID] = name
			t.Events = append(t.Events, e)
			sum.Shows++
			if !e.StartDate.Before(today) && (sum.Next == nil || e.StartDate.Before(sum.Next.StartDate)) {
				e := e
				sum.Next = &e
			}
		}
		for _, r := range s.Rounds {
			if !has(r.ArtistIDs, a.ID) {
				continue
			}
			name := r.Tour
			if name == "" {
				names := map[string]bool{}
				for _, id := range r.EventIDs {
					names[eventTour[id]] = true
				}
				if len(names) == 1 {
					for n := range names {
						name = n
					}
				}
			}
			tour(name).Rounds = append(tour(name).Rounds, r)
			if r.ClosesAt == nil || !r.ClosesAt.Date.Before(today) {
				sum.Pending++
			}
		}
		if sum.Shows == 0 && len(order) == 0 && !a.Followed {
			continue
		}
		for _, name := range order {
			t := tours[name]
			sort.SliceStable(t.Events, func(i, j int) bool { return t.Events[i].StartDate.Before(t.Events[j].StartDate) })
			sum.Tours = append(sum.Tours, *t)
		}
		// Upcoming tours first, nearest first; then past tours, most recent
		// first. A huge "single shows" block of old festivals sinks to the
		// bottom instead of hiding the coming tour.
		sort.SliceStable(sum.Tours, func(i, j int) bool {
			ui, ud := tourAnchor(sum.Tours[i], today)
			uj, jd := tourAnchor(sum.Tours[j], today)
			if ui != uj {
				return ui
			}
			if ui {
				return ud.Before(jd)
			}
			return jd.Before(ud)
		})
		out = append(out, sum)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Shows > 0) != (out[j].Shows > 0) {
			return out[i].Shows > 0
		}
		return out[i].Artist.Name < out[j].Artist.Name
	})
	return out
}

// tourAnchor is the day a tour is ordered by: its next show, or its last
// show when every show is past.
func tourAnchor(t Tour, today domain.Date) (upcoming bool, at domain.Date) {
	for _, e := range t.Events {
		if !e.StartDate.Before(today) {
			return true, e.StartDate
		}
		at = e.StartDate
	}
	return false, at
}

func has(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
