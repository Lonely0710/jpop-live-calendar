package views

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/avatar"
	"jpop-live-calendar/internal/cache"
	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/crawler"
	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
)

// TestScreenshots renders the pictures the READMEs and the website show, one
// set per interface language. It is not part of go test ./...: it needs the
// cache of a real sync and it rewrites files under assets/, so it only runs
// when told where to write:
//
//	LIVE_CALENDAR_SHOTS=/tmp/shots go test -count=1 ./internal/views -run TestScreenshots
//
// By default the shows come from the announcements and avatars the last real
// sync left in the cache, so the pictures are offline and repeatable; the
// sync status in them reads 尚未同步. With LIVE_CALENDAR_SYNC=1 the sources of
// sources.yaml are synced once first, which is slower and hits the network
// with the app's own rules, but leaves a real 更新于 in the footer and real
// rows on the sources page.
func TestScreenshots(t *testing.T) {
	out := os.Getenv("LIVE_CALENDAR_SHOTS")
	if out == "" {
		t.Skip("LIVE_CALENDAR_SHOTS not set")
	}

	cfg := config.Default()
	cat := catalog.New(dedup.Options{Artists: cfg.Artists, SourceNames: sourceNames(cfg.Sources), Locale: localeChoices[0]})
	sched := crawler.New(cat, cfg.Sources)
	now := time.Now()
	sched.Now = func() time.Time { return now }

	if os.Getenv("LIVE_CALENDAR_SYNC") != "" {
		runs := sched.RunOnce(context.Background())
		succeeded := 0
		for _, r := range runs {
			if r.State == domain.SyncSucceeded {
				succeeded++
			} else {
				t.Logf("%s: %s (%v)", r.SourceID, r.State, r.Err)
			}
		}
		t.Logf("synced %d sources, %d succeeded", len(runs), succeeded)
		if succeeded == 0 {
			t.Fatal("no source synced: the pictures would show an empty app")
		}
	} else {
		cachePath, err := config.CachePath()
		if err != nil {
			t.Fatal(err)
		}
		anns, err := cache.Load(cachePath)
		if err != nil || len(anns) == 0 {
			t.Skipf("no snapshot of a real sync: %v", err)
		}
		cat.Restore(anns)
		t.Logf("restored %d announcements", len(anns))
	}

	store := avatar.NewStore()
	if avatarPath, err := config.AvatarCachePath(); err == nil {
		if cached, err := cache.LoadAvatars(avatarPath); err != nil {
			t.Logf("avatars.json: %v", err)
		} else {
			store.Restore(cached)
		}
	}

	for _, tag := range localeChoices {
		i18n.SetLocaleFunc(func() string { return tag })
		app := New(Options{Config: cfg, Catalog: cat, Scheduler: sched, Avatars: store, Now: func() time.Time { return now }})
		app.InitLocale(tag)
		tt := ui.NewTester(app.View, 1440, 900)
		tt.SetLocale(tag)
		dir := filepath.Join(out, shotLang(tag))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		shotPages(t, app, tt, dir)
	}
	i18n.SetLocaleFunc(nil)
}

// shotLang is the language tag as the website and the screenshot folders
// spell it; zh-Hans is simply zh there.
func shotLang(tag string) string {
	if tag == "zh-Hans" {
		return "zh"
	}
	return tag
}

// shotPages walks the pages the READMEs and the site show, saving each one.
func shotPages(t *testing.T, app *App, tt *ui.Tester, dir string) {
	t.Helper()
	save := func(name string) {
		t.Helper()
		// The inspector slides in and out over 220ms, so let the tester's
		// clock run before the picture is taken.
		tt.Frame()
		tt.Advance(500 * time.Millisecond)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}

	// The month, with the selected day's shows in the inspector.
	app.section = SectionCalendar
	app.mode = int(ModeMonth)
	app.inspector = true
	save("month-live.png")

	// One show: its ticket rounds and its change log.
	if id, day, ok := busiestShow(app); ok {
		app.entryID, app.selected, app.cursor = id, day, day
		save("inspector.png")
		app.entryID = ""
	}

	// Every prefecture at once.
	app.section = SectionMap
	app.mapScope = MapAll
	app.mapPref = ""
	save("map.png")

	// The artist page, opened on the first artist listed.
	app.section = SectionArtists
	app.inspector = false
	for _, a := range app.snap.Artists {
		app.artistID = a.ID
		break
	}
	save("artists-live.png")

	// The sources page with the sync status of each one.
	app.section = SectionSources
	save("sources.png")
}

// busiestShow is the upcoming show that makes the best picture: the busiest
// month, then the show with the most ticket rounds, skipping the ones the
// parser still flags as 需确认.
func busiestShow(app *App) (string, domain.Date, bool) {
	rounds := map[string]int{}
	for _, r := range app.snap.Rounds {
		for _, id := range r.EventIDs {
			rounds[id]++
		}
	}
	review := map[string]bool{}
	for _, e := range app.snap.Events {
		review[e.ID] = e.NeedsReview
	}
	density := map[string]int{}
	for _, e := range app.snap.Entries {
		if e.Kind == domain.EntryLive {
			density[e.Date.String()[:7]]++
		}
	}

	today := domain.DateOf(app.opt.Now(), domain.Tokyo)
	bestID, bestDay, bestScore := "", domain.Date{}, -1
	for _, e := range app.snap.Entries {
		if e.Kind != domain.EntryLive || e.Date.Before(today) || review[e.EventID] {
			continue
		}
		score := density[e.Date.String()[:7]]*10 + rounds[e.EventID]
		if score > bestScore {
			bestID, bestDay, bestScore = e.ID, e.Date, score
		}
	}
	return bestID, bestDay, bestID != ""
}
