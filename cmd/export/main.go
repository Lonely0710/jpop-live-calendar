// Command export syncs the public sources of sources.yaml once, without the
// app's window, and writes the upcoming shows and ticket rounds as JSON for
// the project website (docs/). It keeps the app's crawling rules: robots.txt,
// the per-site delay and the per-source timeout.
//
//	go run ./cmd/export -o docs/data/events.json -cache .cache/cache.json
//
// Only facts read from official announcements are written; times that are
// not announced stay empty and are never guessed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/cache"
	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/crawler"
	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/geo"
)

const version = "1.0.2"

type moment struct {
	Date string `json:"date"`           // YYYY-MM-DD, Asia/Tokyo
	Time string `json:"time,omitempty"` // HH:MM; empty while not announced
}

type artist struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	Official string `json:"official,omitempty"`
}

type event struct {
	ID       string   `json:"id"`
	Artists  []string `json:"artists"`
	Title    string   `json:"title"`
	Tour     string   `json:"tour,omitempty"`
	Type     string   `json:"type"`
	Status   string   `json:"status"`
	Date     string   `json:"date"`
	Open     string   `json:"open,omitempty"`
	Start    string   `json:"start,omitempty"`
	Venue    string   `json:"venue,omitempty"`
	Pref     string   `json:"pref,omitempty"` // 東京都
	PrefCode int      `json:"pref_code,omitempty"`
	URL      string   `json:"url,omitempty"`
}

type round struct {
	ID       string   `json:"id"`
	Artists  []string `json:"artists"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Lottery  bool     `json:"lottery"`
	Tour     string   `json:"tour,omitempty"`
	Events   []string `json:"events,omitempty"`
	Opens    *moment  `json:"opens,omitempty"`
	Closes   *moment  `json:"closes,omitempty"`
	Result   *moment  `json:"result,omitempty"`
	Source   string   `json:"source,omitempty"`
	Purchase string   `json:"purchase,omitempty"`
	Review   bool     `json:"needs_review,omitempty"`
}

type source struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	Err   string `json:"error,omitempty"`
	Count int    `json:"announcements"`
}

type output struct {
	GeneratedAt time.Time `json:"generated_at"`
	Version     string    `json:"version"`
	Timezone    string    `json:"timezone"`
	Artists     []artist  `json:"artists"`
	Events      []event   `json:"events"`
	Rounds      []round   `json:"rounds"`
	Sources     []source  `json:"sources"`
}

func main() {
	out := flag.String("o", "docs/data/events.json", "output JSON file")
	cfgPath := flag.String("config", "config/sources.yaml", "sources.yaml")
	cachePath := flag.String("cache", "", "announcement cache kept between runs (optional)")
	timeout := flag.Duration("timeout", 3*time.Minute, "timeout per source")
	flag.Parse()

	b, err := os.ReadFile(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Parse(b)
	if err != nil {
		log.Fatalf("%s: %v", *cfgPath, err)
	}
	var srcs []domain.SourceConfig
	names := map[string]string{}
	for _, s := range cfg.Sources {
		if s.Kind == domain.SourceDemo { // never publish the made-up data
			continue
		}
		srcs = append(srcs, s)
		names[s.ID] = s.Name
	}

	cat := catalog.New(dedup.Options{Artists: cfg.Artists, SourceNames: names})
	sch := crawler.New(cat, srcs)
	sch.Timeout = *timeout
	if *cachePath != "" {
		anns, err := cache.Load(*cachePath)
		if err != nil {
			log.Printf("cache: %v", err)
		}
		cat.SetInclude(sch.Enabled)
		cat.Restore(anns)
		log.Printf("cache: %d announcements restored", len(anns))
	}

	start := time.Now()
	runs := sch.RunOnce(context.Background())
	failed := 0
	for _, r := range runs {
		if r.State != domain.SyncSucceeded {
			failed++
			log.Printf("✗ %s: %s", r.SourceID, r.Err)
		} else {
			log.Printf("✓ %s: %d fetched, %d new, %d updated", r.SourceID, r.Fetched, r.New, r.Updated)
		}
	}
	log.Printf("synced %d sources in %s, %d failed", len(runs), time.Since(start).Round(time.Second), failed)
	if len(runs) > 0 && failed == len(runs) {
		log.Fatal("every source failed; keeping the previous data")
	}
	if *cachePath != "" {
		if err := os.MkdirAll(filepath.Dir(*cachePath), 0o755); err == nil {
			if err := cache.Save(*cachePath, version, cat.Announcements()); err != nil {
				log.Printf("cache: %v", err)
			}
		}
	}

	o := build(cat.Snapshot(), sch, cfg)
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	j, _ := json.MarshalIndent(o, "", " ")
	if err := os.WriteFile(*out, append(j, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %d artists, %d upcoming shows, %d open rounds\n", *out, len(o.Artists), len(o.Events), len(o.Rounds))
}

func tokyo() *time.Location {
	if l, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		return l
	}
	return time.FixedZone("JST", 9*3600)
}

func mom(m *domain.Moment) *moment {
	if m == nil {
		return nil
	}
	x := &moment{Date: m.Date.String()}
	if m.Clock != nil {
		x.Time = m.Clock.String()
	}
	return x
}

func clock(c *domain.Clock) string {
	if c == nil {
		return ""
	}
	return c.String()
}

func build(s *catalog.Snapshot, sch *crawler.Scheduler, cfg config.Config) output {
	loc := tokyo()
	now := time.Now()
	today := domain.DateOf(now, loc)
	o := output{GeneratedAt: now.UTC().Truncate(time.Second), Version: version, Timezone: "Asia/Tokyo",
		Artists: []artist{}, Events: []event{}, Rounds: []round{}, Sources: []source{}}

	colors := map[string]string{}
	for _, a := range cfg.Artists {
		colors[a.Name] = a.Color
	}
	name := map[string]string{}
	for _, a := range s.Artists {
		name[a.ID] = a.Name
	}
	names := func(ids []string) []string {
		out := []string{}
		for _, id := range ids {
			if n := name[id]; n != "" {
				out = append(out, n)
			}
		}
		return out
	}

	used := map[string]bool{}
	jp := geo.Japan()
	for _, e := range s.Events {
		if e.StartDate.Before(today) {
			continue
		}
		ev := event{ID: e.ID, Artists: names(e.ArtistIDs), Title: e.Title, Tour: e.Tour, Type: string(e.Type),
			Status: string(e.Status), Date: e.StartDate.String(), Open: clock(e.OpenAt), Start: clock(e.StartAt),
			Venue: e.VenueName, URL: e.TicketURL}
		if e.Prefecture != "" {
			ev.Pref = geo.Suffixed(e.Prefecture)
			if i := jp.Find(e.Prefecture); i >= 0 {
				ev.PrefCode = i + 1
			}
		}
		for _, n := range ev.Artists {
			used[n] = true
		}
		o.Events = append(o.Events, ev)
	}
	sort.SliceStable(o.Events, func(i, j int) bool { return o.Events[i].Date < o.Events[j].Date })

	for _, r := range s.Rounds {
		// Keep rounds with something still ahead: an opening, a deadline
		// or a result. A round without any date stays out of the board.
		ahead := false
		for _, m := range []*domain.Moment{r.OpensAt, r.ClosesAt, r.ResultAt} {
			if m != nil && !m.Date.Before(today) {
				ahead = true
			}
		}
		if !ahead {
			continue
		}
		rd := round{ID: r.ID, Artists: names(r.ArtistIDs), Name: r.Name, Type: string(r.Type), Lottery: r.Lottery,
			Tour: r.Tour, Events: r.EventIDs, Opens: mom(r.OpensAt), Closes: mom(r.ClosesAt), Result: mom(r.ResultAt),
			Source: r.SourceURL, Purchase: r.PurchaseURL, Review: r.NeedsReview}
		for _, n := range rd.Artists {
			used[n] = true
		}
		o.Rounds = append(o.Rounds, rd)
	}

	for _, a := range s.Artists {
		if !a.Followed && !used[a.Name] {
			continue
		}
		c := a.Color
		if c == "" {
			c = colors[a.Name]
		}
		o.Artists = append(o.Artists, artist{ID: a.ID, Name: a.Name, Color: c, Official: a.OfficialURL})
	}
	sort.SliceStable(o.Artists, func(i, j int) bool { return o.Artists[i].Name < o.Artists[j].Name })

	for _, st := range sch.Statuses() {
		src := source{ID: st.Config.ID, Name: st.Config.Name, State: string(st.State), Err: st.LastRun.Err, Count: s.PerSource[st.Config.ID]}
		if !st.Config.Enabled {
			src.State = "disabled"
		}
		o.Sources = append(o.Sources, src)
	}
	return o
}
