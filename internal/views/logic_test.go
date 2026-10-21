package views

import (
	"context"
	"testing"
	"time"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/sources"
	"jpop-live-calendar/internal/theme"

	"github.com/egoist/mygo/ui"
)

func TestSourceID(t *testing.T) {
	a := &App{opt: Options{Config: config.Config{Sources: []domain.SourceConfig{
		{ID: "example-com-live"},
		{ID: "example-com-live-2"},
	}}}}
	cases := map[string]string{
		"https://example.com/live/":             "example-com-live-3",
		"https://www.Example.com/news/feed.xml": "example-com-feed-xml",
		"https://例え.jp/":                        "jp",
	}
	for raw, want := range cases {
		if got := a.sourceID(raw); got != want {
			t.Errorf("sourceID(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := parseSourceKind("RSS"); got != domain.SourceRSS {
		t.Errorf("RSS kind = %q", got)
	}
	if got := parseSourceLayer("公告 news"); got != domain.LayerNews {
		t.Errorf("news layer = %q", got)
	}
	if got := parseSourceInterval("6h"); got != 6*time.Hour {
		t.Errorf("6h = %v", got)
	}
}

func TestDraftSource(t *testing.T) {
	a := &App{opt: Options{Config: config.Config{Sources: []domain.SourceConfig{{ID: "example-com-live"}}}}}
	a.sourceForm = sourceDraft{name: "某乐队", kind: "RSS", layer: "公告 news", interval: "6h",
		url: "https://example.com/live/", artist: "某乐队", note: " 说明 "}
	src, err := a.draftSource()
	if err != nil {
		t.Fatalf("draftSource: %v", err)
	}
	if src.ID != "example-com-live-2" || src.Name != "某乐队" || src.Kind != domain.SourceRSS ||
		src.Layer != domain.LayerNews || src.Interval != 6*time.Hour || src.Artist != "某乐队" || src.Note != "说明" || !src.Enabled {
		t.Errorf("source = %+v", src)
	}
	a.sourceForm.name = " "
	if _, err := a.draftSource(); err == nil {
		t.Error("a source without a name was accepted")
	}
	a.sourceForm.name = "某乐队"
	a.sourceForm.url = "ftp://example.com"
	if _, err := a.draftSource(); err == nil {
		t.Error("a source without an http URL was accepted")
	}
}

func TestDefaultAvatar(t *testing.T) {
	for _, id := range []string{"a-1", "a-2", "artist", "", "ステラリウム"} {
		i := defaultAvatarIndex(id)
		if i < 0 || i >= len(defaultAvatars) {
			t.Fatalf("index of %q = %d", id, i)
		}
		if again := defaultAvatarIndex(id); again != i {
			t.Errorf("index of %q changed", id)
		}
	}
	for _, col := range []ui.Color{
		theme.LiveCoral, theme.WarmSpotlight, ui.Hex("#3D86F5"), ui.Hex("#F7F3EA"), ui.Hex("#2E2A28"),
	} {
		disc, ink, accent := defaultAvatarPalette(col)
		if c := theme.Contrast(ink, disc); c < 4.5 {
			t.Errorf("lines %v on %v: contrast %.2f", ink, disc, c)
		}
		if c := theme.Contrast(accent, disc); c < 3 {
			t.Errorf("accent %v on %v: contrast %.2f", accent, disc, c)
		}
	}
}

func demoCatalog(t *testing.T, now time.Time) *catalog.Catalog {
	t.Helper()
	cat := catalog.New(dedup.Options{Artists: sources.DemoArtists})
	res, err := sources.Demo{}.Fetch(context.Background(), domain.SourceConfig{ID: "demo"}, sources.Env{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	cat.Ingest(res.Announcements)
	cat.Rebuild()
	return cat
}

func TestDeadlineReminders(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 29, 0, 0, domain.Tokyo)
	app := New(Options{Config: config.Default(), Catalog: demoCatalog(t, now), Now: func() time.Time { return now }})
	notes := app.deadlineReminders(now)
	if len(notes) == 0 {
		t.Fatal("no deadline reminders from the demo data")
	}
	for _, n := range notes {
		if n.ID == "" || n.Title == "" || n.Body == "" || n.EntryID == "" {
			t.Errorf("incomplete reminder %+v", n)
		}
		if got := EntryIDFromNotification(n.ID); got != n.EntryID {
			t.Errorf("EntryIDFromNotification(%q) = %q, want %q", n.ID, got, n.EntryID)
		}
	}
	app.notified[notes[0].ID] = now.Format(time.RFC3339)
	for _, n := range app.deadlineReminders(now) {
		if n.ID == notes[0].ID {
			t.Errorf("a notified deadline came back: %+v", n)
		}
	}
	if got := EntryIDFromNotification("something-else"); got != "" {
		t.Errorf("foreign notification = %q", got)
	}
}

func TestNotifyDeadlines(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 29, 0, 0, domain.Tokyo)
	var shown []Reminder
	app := New(Options{
		Config: config.Default(), Catalog: demoCatalog(t, now), Now: func() time.Time { return now },
		// Run the notification work now, and count its results.
		Go:   func(f func()) { f() },
		Post: func(f func()) { f() },
		Notify: func(r Reminder) error {
			shown = append(shown, r)
			return nil
		},
	})
	want := len(app.deadlineReminders(now))
	app.notifyDeadlines()
	if len(shown) != want {
		t.Fatalf("showed %d notifications, want %d", len(shown), want)
	}
	if len(app.notified) != want {
		t.Errorf("recorded %d deadlines, want %d", len(app.notified), want)
	}
	app.notifyDeadlines()
	if len(shown) != want {
		t.Errorf("a second check showed %d more", len(shown)-want)
	}
}
