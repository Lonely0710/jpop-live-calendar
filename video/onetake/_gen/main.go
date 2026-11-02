// Command gen writes the data the onetake promo draws, from the app's own
// local cache only (no network): the announcement the film parses, the shows
// and ticket rounds it produced, the month around them, and the prefecture
// outlines. The leading underscore keeps it out of `go test ./...`.
//
//	go run ./video/onetake/_gen -out video/onetake/build/data.js
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/cache"
	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/geo"
)

type show struct {
	ID     string `json:"id"`
	Date   string `json:"date"`
	Artist string `json:"artist"`
	Color  string `json:"color,omitempty"`
	Title  string `json:"title"`
	Start  string `json:"start,omitempty"`
	Open   string `json:"open,omitempty"`
	Venue  string `json:"venue,omitempty"`
	Pref   string `json:"pref,omitempty"`
	Status string `json:"status"`
	Ann    bool   `json:"ann,omitempty"` // produced by the film's announcement
}

type round struct {
	Name    string `json:"name"`
	Artist  string `json:"artist"`
	Lottery bool   `json:"lottery"`
	Opens   string `json:"opens,omitempty"`
	Closes  string `json:"closes,omitempty"`
	Ann     bool   `json:"ann,omitempty"`
}

type pref struct {
	Name    string      `json:"name"`
	Rings   [][]float32 `json:"rings"` // flat x,y pairs per ring
	Capital [2]float32  `json:"capital"`
}

type out struct {
	Announcement struct {
		Title     string `json:"title"`
		Raw       string `json:"raw"`
		URL       string `json:"url"`
		Source    string `json:"source"`
		Published string `json:"published"`
		Fetched   string `json:"fetched"`
		Revision  int    `json:"revision"`
	} `json:"announcement"`
	Version string  `json:"version"`
	Month   string  `json:"month"`
	Shows   []show  `json:"shows"`
	Rounds  []round `json:"rounds"`
	Map     struct {
		W     float32    `json:"w"`
		H     float32    `json:"h"`
		Inset [4]float32 `json:"inset"` // x, y, w, h of the Nansei Islands frame
		Prefs []pref     `json:"prefs"`
	} `json:"map"`
	Counts map[string]int `json:"counts"` // shows per prefecture in the month
}

func moment(m *domain.Moment) string {
	if m == nil {
		return ""
	}
	s := m.Date.String()
	if m.Clock != nil {
		s += " " + m.Clock.String()
	}
	return s
}

func main() {
	dir := flag.String("data", filepath.Join(os.Getenv("HOME"), "Library/Application Support/Live Calendar"), "the app's data directory")
	annID := flag.String("ann", "n-33bff4641104", "announcement the film parses")
	month := flag.String("month", "2026-10", "month shown")
	dst := flag.String("out", "video/onetake/build/data.js", "output file")
	flag.Parse()

	b, err := os.ReadFile("config/sources.yaml")
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Parse(b)
	if err != nil {
		log.Fatal(err)
	}
	names := map[string]string{}
	for _, s := range cfg.Sources {
		names[s.ID] = s.Name
	}
	anns, err := cache.Load(filepath.Join(*dir, "cache.json"))
	if err != nil {
		log.Fatalf("cache: %v", err)
	}
	cat := catalog.New(dedup.Options{Artists: cfg.Artists, SourceNames: names})
	cat.Restore(anns)
	snap := cat.Snapshot()

	colors := avatarColors(filepath.Join(*dir, "avatars.json"))
	artist := map[string]domain.Artist{}
	for _, a := range snap.Artists {
		artist[a.ID] = a
	}
	first := func(ids []string) (string, string) {
		for _, id := range ids {
			a := artist[id]
			c := a.Color
			if c == "" {
				c = colors[id]
			}
			return a.Name, c
		}
		return "", ""
	}
	has := func(ids []string) bool {
		for _, id := range ids {
			if id == *annID {
				return true
			}
		}
		return false
	}

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		tokyo = time.FixedZone("JST", 9*3600)
	}
	var o out
	if mb, err := os.ReadFile("mygo.json"); err == nil {
		var m struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(mb, &m) == nil {
			o.Version = m.Version
		}
	}
	o.Month = *month
	o.Counts = map[string]int{}
	found := false
	for _, a := range snap.Announcements {
		if a.ID != *annID {
			continue
		}
		found = true
		o.Announcement.Title, o.Announcement.Raw, o.Announcement.URL = a.Title, a.RawText, a.URL
		o.Announcement.Source = names[a.SourceID]
		o.Announcement.Published = a.PublishedAt.In(tokyo).Format("2006-01-02 15:04")
		o.Announcement.Fetched = a.FetchedAt.In(tokyo).Format("2006-01-02 15:04")
		o.Announcement.Revision = a.Revision
	}
	if !found {
		log.Fatalf("announcement %s is not in the cache", *annID)
	}

	// The visible grid runs from the Monday on or before the 1st, six weeks.
	m1, err := time.Parse("2006-01", *month)
	if err != nil {
		log.Fatalf("month: %v", err)
	}
	g0 := m1.AddDate(0, 0, -((int(m1.Weekday()) + 6) % 7))
	gridFrom, gridTo := g0.Format("2006-01-02"), g0.AddDate(0, 0, 41).Format("2006-01-02")
	for _, e := range snap.Events {
		d := e.StartDate.String()
		if d < gridFrom || d > gridTo {
			continue
		}
		name, color := first(e.ArtistIDs)
		s := show{ID: e.ID, Date: d, Artist: name, Color: color, Title: e.Title, Venue: e.VenueName,
			Pref: e.Prefecture, Status: string(e.Status), Ann: has(e.AnnouncementIDs)}
		if e.StartAt != nil {
			s.Start = e.StartAt.String()
		}
		if e.OpenAt != nil {
			s.Open = e.OpenAt.String()
		}
		o.Shows = append(o.Shows, s)
		if strings.HasPrefix(d, *month) && e.Prefecture != "" {
			o.Counts[e.Prefecture]++
		}
	}
	sort.SliceStable(o.Shows, func(i, j int) bool {
		if o.Shows[i].Date != o.Shows[j].Date {
			return o.Shows[i].Date < o.Shows[j].Date
		}
		return o.Shows[i].Start < o.Shows[j].Start
	})
	for _, r := range snap.Rounds {
		if !has(r.AnnouncementIDs) {
			continue
		}
		name, _ := first(r.ArtistIDs)
		o.Rounds = append(o.Rounds, round{Name: r.Name, Artist: name, Lottery: r.Lottery,
			Opens: moment(r.OpensAt), Closes: moment(r.ClosesAt), Ann: true})
	}

	jp := geo.Japan()
	o.Map.W, o.Map.H = jp.Size.X, jp.Size.Y
	o.Map.Inset = [4]float32{jp.Inset.X, jp.Inset.Y, jp.Inset.W, jp.Inset.H}
	for _, p := range jp.Prefectures {
		q := pref{Name: p.Name, Capital: [2]float32{p.Capital.X, p.Capital.Y}}
		for _, r := range p.Rings {
			flat := make([]float32, 0, len(r)*2)
			for _, pt := range r {
				flat = append(flat, pt.X, pt.Y)
			}
			q.Rings = append(q.Rings, flat)
		}
		o.Map.Prefs = append(o.Map.Prefs, q)
	}

	j, err := json.Marshal(o)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(*dst), 0o755); err != nil {
		log.Fatal(err)
	}
	js := "// Generated by video/onetake/_gen from the app's local cache. Do not edit.\nwindow.DATA=" + string(j) + ";\n"
	if err := os.WriteFile(*dst, []byte(js), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %d shows (%d from the announcement), %d rounds, %d prefectures\n",
		*dst, len(o.Shows), countAnn(o.Shows), len(o.Rounds), len(o.Map.Prefs))
}

func countAnn(s []show) int {
	n := 0
	for _, x := range s {
		if x.Ann {
			n++
		}
	}
	return n
}

// avatarColors reads the theme colors the app took from each artist's
// public avatar.
func avatarColors(path string) map[string]string {
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	var f struct {
		Avatars map[string]struct {
			Color struct{ R, G, B int } `json:"color"`
		} `json:"avatars"`
	}
	if json.Unmarshal(b, &f) != nil {
		return m
	}
	for id, a := range f.Avatars {
		m[id] = fmt.Sprintf("#%02X%02X%02X", a.Color.R, a.Color.G, a.Color.B)
	}
	return m
}
