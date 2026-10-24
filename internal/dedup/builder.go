package dedup

import (
	"sort"
	"strings"
	"time"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/normalize"
	"jpop-live-calendar/internal/parser"
	"jpop-live-calendar/locales"
)

// Options configure a build.
type Options struct {
	Artists     []domain.Artist            // configured names and aliases
	SourceNames map[string]string          // source ID → display name
	Include     func(sourceID string) bool // nil includes every source
	Locale      string                     // UI language of derived notes; "" is zh-Hans
}

// Result is the merged calendar.
type Result struct {
	Events        []domain.PublicEvent
	Rounds        []domain.PublicTicketRound
	Artists       []domain.Artist
	Venues        []domain.Venue
	Announcements []domain.PublicAnnouncement
	Entries       []domain.Entry
}

type builder struct {
	opt     Options
	artists *normalize.Artists
	events  []*domain.PublicEvent
	rounds  []*domain.PublicTicketRound
	roundBy map[string]*domain.PublicTicketRound
	venues  map[string]*domain.Venue
}

// Build reads every announcement oldest first and merges what they say.
// Later announcements fill in what earlier ones left out; when they
// contradict an earlier one the newer value wins and the old one stays in
// the record's Changes.
func Build(anns []domain.PublicAnnouncement, opt Options) Result {
	b := &builder{
		opt:     opt,
		artists: normalize.NewArtists(opt.Artists),
		roundBy: map[string]*domain.PublicTicketRound{},
		venues:  map[string]*domain.Venue{},
	}
	var used []domain.PublicAnnouncement
	sorted := append([]domain.PublicAnnouncement(nil), anns...)
	sort.SliceStable(sorted, func(i, j int) bool { return published(sorted[i]).Before(published(sorted[j])) })
	for _, a := range sorted {
		if opt.Include != nil && !opt.Include(a.SourceID) {
			continue
		}
		if a.ID == "" {
			a.ID = normalize.ID("n-", key(a))
		}
		used = append(used, a)
		b.read(a)
	}
	b.linkRounds()

	var res Result
	res.Announcements = used
	for _, e := range b.events {
		res.Events = append(res.Events, *e)
	}
	for _, r := range b.rounds {
		res.Rounds = append(res.Rounds, *r)
	}
	res.Artists = b.artists.All()
	for _, v := range b.venues {
		res.Venues = append(res.Venues, *v)
	}
	sort.Slice(res.Venues, func(i, j int) bool { return res.Venues[i].Name < res.Venues[j].Name })
	res.Entries = Entries(res.Events, res.Rounds, res.Artists, opt.SourceNames, opt.Locale)
	return res
}

// msgs says the notes and change fields in the build's language.
func (b *builder) msgs() locales.Messages { return locales.In(b.opt.Locale) }

func (b *builder) read(a domain.PublicAnnouncement) {
	when := published(a)
	res := parser.Parse(parser.Input{Title: a.Title, Text: a.RawText, ArtistHints: a.ArtistHints, Published: when, Schedule: a.Schedule, Locale: b.opt.Locale})
	if len(res.Events) == 0 && len(res.Rounds) == 0 {
		return
	}
	names := res.Artists
	guessed := res.ArtistGuessed
	if len(names) == 0 || guessed {
		if found := b.artists.Find(a.Title); len(found) > 0 {
			names, guessed = nil, false
			for _, f := range found {
				names = append(names, f.Name)
			}
		}
	}
	var artistIDs []string
	for _, n := range names {
		if ar := b.artists.Resolve(n); ar != nil {
			artistIDs = appendUnique(artistIDs, ar.ID)
		}
	}
	var annNotes []string
	if guessed {
		annNotes = append(annNotes, b.msgs().NoteArtistGuessed())
	}
	if len(artistIDs) == 0 {
		annNotes = append(annNotes, b.msgs().NoteArtistUnknown())
	}
	annNotes = append(annNotes, res.Notes...)

	var touched []string
	for _, pe := range res.Events {
		e := b.mergeEvent(a, when, pe, artistIDs, annNotes)
		touched = appendUnique(touched, e.ID)
	}
	for _, pr := range res.Rounds {
		r := b.mergeRound(a, when, pr, res.Tour, artistIDs, annNotes)
		for _, id := range touched {
			r.EventIDs = appendUnique(r.EventIDs, id)
		}
	}
}

func venueMatch(a, b string) bool {
	ka, kb := normalize.Key(a), normalize.Key(b)
	if ka == "" || kb == "" {
		return false
	}
	return ka == kb || strings.Contains(ka, kb) || strings.Contains(kb, ka)
}

func overlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

func (b *builder) mergeEvent(a domain.PublicAnnouncement, when time.Time, pe parser.Event, artistIDs, annNotes []string) *domain.PublicEvent {
	var same, unsure []*domain.PublicEvent
	for _, e := range b.events {
		if e.StartDate != pe.Date || !overlap(e.ArtistIDs, artistIDs) {
			continue
		}
		switch {
		case venueMatch(e.VenueName, pe.Venue):
			same = append(same, e)
		case e.VenueName == "" || pe.Venue == "":
			unsure = append(unsure, e)
		case e.StartAt != nil && pe.StartAt != nil && *e.StartAt != *pe.StartAt:
			// Another venue at another time: a day and a night show.
		default:
			// Another venue, times unknown: a moved venue, a typo or a
			// second show. Kept apart and flagged for review.
			unsure = append(unsure, e)
		}
	}
	var e *domain.PublicEvent
	switch {
	case len(same) > 0:
		e = same[0]
	case len(unsure) == 1 && (unsure[0].VenueName == "" || pe.Venue == ""):
		e = unsure[0]
		addNote(e, b.msgs().NoteVenueIncomplete())
	}
	if e == nil {
		e = b.newEvent(a, when, pe, artistIDs)
		for _, o := range unsure {
			addNote(e, b.msgs().NotePossibleSame(o.Title, o.VenueName))
			addNote(o, b.msgs().NotePossibleSame(e.Title, e.VenueName))
		}
	} else {
		b.update(e, a, when, pe)
	}
	for _, id := range artistIDs {
		e.ArtistIDs = appendUnique(e.ArtistIDs, id)
	}
	for _, n := range append(append([]string(nil), annNotes...), pe.Notes...) {
		if n == b.msgs().NoteNoVenue() && e.VenueName != "" {
			continue
		}
		addNote(e, n)
	}
	for _, n := range pe.Info {
		e.Notes = appendUnique(e.Notes, n)
	}
	e.AnnouncementIDs = appendUnique(e.AnnouncementIDs, a.ID)
	e.SourceIDs = appendUnique(e.SourceIDs, a.SourceID)
	return e
}

func (b *builder) newEvent(a domain.PublicAnnouncement, when time.Time, pe parser.Event, artistIDs []string) *domain.PublicEvent {
	title := pe.Tour
	if title == "" {
		title = strings.TrimSpace(a.Title)
	}
	e := &domain.PublicEvent{
		ID:         normalize.ID("e-", pe.Date.String(), strings.Join(artistIDs, ","), normalize.Key(pe.Venue), a.ID),
		Title:      title,
		Tour:       pe.Tour,
		Type:       pe.Type,
		Status:     domain.StatusAnnounced,
		Timezone:   domain.TokyoName,
		StartDate:  pe.Date,
		OpenAt:     pe.OpenAt,
		StartAt:    pe.StartAt,
		VenueName:  pe.Venue,
		Prefecture: pe.Prefecture,
		TicketURL:  pe.TicketURL,
		FirstSeen:  when,
		UpdatedAt:  when,
	}
	if pe.Status != "" {
		e.Status = pe.Status
	}
	b.setVenue(e)
	b.events = append(b.events, e)
	return e
}

func (b *builder) setVenue(e *domain.PublicEvent) {
	if e.VenueName == "" {
		return
	}
	k := normalize.Key(e.VenueName)
	for vk, v := range b.venues {
		if venueMatch(vk, k) {
			e.VenueID = v.ID
			if v.Prefecture == "" {
				v.Prefecture = e.Prefecture
			}
			return
		}
	}
	v := &domain.Venue{ID: normalize.ID("v-", k), Name: e.VenueName, Prefecture: e.Prefecture}
	b.venues[k] = v
	e.VenueID = v.ID
}

func change(field, old, new string, a domain.PublicAnnouncement, when time.Time) domain.Change {
	return domain.Change{Field: field, Old: old, New: new, At: when, Source: a.URL}
}

func (b *builder) clockText(c *domain.Clock) string {
	if c == nil {
		return b.msgs().CommonTba()
	}
	return c.String()
}

func (b *builder) update(e *domain.PublicEvent, a domain.PublicAnnouncement, when time.Time, pe parser.Event) {
	setClock := func(field string, cur **domain.Clock, v *domain.Clock) {
		if v == nil || (*cur != nil && **cur == *v) {
			return
		}
		e.Changes = append(e.Changes, change(field, b.clockText(*cur), v.String(), a, when))
		*cur = v
	}
	setClock(b.msgs().ChangeFieldOpenTime(), &e.OpenAt, pe.OpenAt)
	setClock(b.msgs().ChangeFieldStartTime(), &e.StartAt, pe.StartAt)
	if e.VenueName == "" && pe.Venue != "" {
		e.Changes = append(e.Changes, change(b.msgs().ChangeFieldVenue(), b.msgs().CommonTba(), pe.Venue, a, when))
		e.VenueName, e.Prefecture = pe.Venue, pe.Prefecture
		b.setVenue(e)
		removeNote(e, b.msgs().NoteNoVenue())
	}
	if e.TicketURL == "" && pe.TicketURL != "" {
		e.TicketURL = pe.TicketURL
	}
	if e.Prefecture == "" && pe.Prefecture != "" {
		e.Prefecture = pe.Prefecture
	}
	if e.Tour == "" && pe.Tour != "" {
		e.Tour = pe.Tour
		e.Title = pe.Tour
	}
	if pe.Status != "" && pe.Status != e.Status {
		e.Changes = append(e.Changes, change(b.msgs().ChangeFieldStatus(), e.Status.Label(b.opt.Locale), pe.Status.Label(b.opt.Locale), a, when))
		e.Status = pe.Status
	}
	if when.After(e.UpdatedAt) {
		e.UpdatedAt = when
	}
}

func roundKey(artistIDs []string, tour string, r parser.Round) string {
	ids := append([]string(nil), artistIDs...)
	sort.Strings(ids)
	return strings.Join(ids, ",") + "|" + normalize.Key(tour) + "|" + string(r.Type) + "|" + normalize.Key(r.Name)
}

func (b *builder) momentText(m *domain.Moment) string {
	if m == nil {
		return b.msgs().CommonTba()
	}
	return m.String()
}

func (b *builder) mergeRound(a domain.PublicAnnouncement, when time.Time, pr parser.Round, tour string, artistIDs, annNotes []string) *domain.PublicTicketRound {
	k := roundKey(artistIDs, tour, pr)
	r, ok := b.roundBy[k]
	if !ok {
		r = &domain.PublicTicketRound{
			ID:          normalize.ID("r-", k),
			Name:        pr.Name,
			Type:        pr.Type,
			Lottery:     pr.Lottery,
			ArtistIDs:   artistIDs,
			Tour:        tour,
			OpensAt:     pr.OpensAt,
			ClosesAt:    pr.ClosesAt,
			ResultAt:    pr.ResultAt,
			PurchaseURL: pr.URL,
			SourceURL:   a.URL,
		}
		b.roundBy[k] = r
		b.rounds = append(b.rounds, r)
	} else {
		set := func(field string, cur **domain.Moment, v *domain.Moment) {
			if v == nil || (*cur != nil && (*cur).Equal(*v)) {
				return
			}
			r.Changes = append(r.Changes, change(field, b.momentText(*cur), v.String(), a, when))
			*cur = v
		}
		set(b.msgs().ChangeFieldOpens(), &r.OpensAt, pr.OpensAt)
		set(b.msgs().ChangeFieldCloses(), &r.ClosesAt, pr.ClosesAt)
		set(b.msgs().ChangeFieldResults(), &r.ResultAt, pr.ResultAt)
		if pr.URL != "" && pr.URL != r.PurchaseURL {
			if r.PurchaseURL != "" {
				r.Changes = append(r.Changes, change(b.msgs().ChangeFieldTicketUrl(), r.PurchaseURL, pr.URL, a, when))
			}
			r.PurchaseURL = pr.URL
		}
		r.SourceURL = a.URL
	}
	for _, n := range append(append([]string(nil), annNotes...), pr.Notes...) {
		if !contains(r.ReviewNotes, n) {
			r.ReviewNotes = append(r.ReviewNotes, n)
			r.NeedsReview = true
		}
	}
	r.AnnouncementIDs = appendUnique(r.AnnouncementIDs, a.ID)
	r.SourceIDs = appendUnique(r.SourceIDs, a.SourceID)
	return r
}

// linkRounds connects rounds announced without shows to the shows of the
// same artist and tour.
func (b *builder) linkRounds() {
	for _, r := range b.rounds {
		if len(r.EventIDs) == 0 {
			for _, e := range b.events {
				if overlap(e.ArtistIDs, r.ArtistIDs) && r.Tour != "" && normalize.Key(e.Tour) == normalize.Key(r.Tour) {
					r.EventIDs = append(r.EventIDs, e.ID)
				}
			}
		}
		if len(r.EventIDs) == 0 {
			if note := b.msgs().NoteUnlinkedRound(); !contains(r.ReviewNotes, note) {
				r.ReviewNotes = append(r.ReviewNotes, note)
			}
			r.NeedsReview = true
		}
		for _, id := range r.EventIDs {
			for _, e := range b.events {
				if e.ID == id && e.TicketURL == "" && r.PurchaseURL != "" {
					e.TicketURL = r.PurchaseURL
				}
			}
		}
	}
}

func removeNote(e *domain.PublicEvent, n string) {
	var keep []string
	for _, x := range e.ReviewNotes {
		if x != n {
			keep = append(keep, x)
		}
	}
	e.ReviewNotes = keep
	e.NeedsReview = len(keep) > 0
}

func addNote(e *domain.PublicEvent, n string) {
	if n == "" || contains(e.ReviewNotes, n) {
		return
	}
	e.ReviewNotes = append(e.ReviewNotes, n)
	e.NeedsReview = true
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func appendUnique(xs []string, ys ...string) []string {
	for _, y := range ys {
		if y != "" && !contains(xs, y) {
			xs = append(xs, y)
		}
	}
	return xs
}
