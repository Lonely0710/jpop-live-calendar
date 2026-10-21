package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
)

func sample() domain.PublicAnnouncement {
	return domain.PublicAnnouncement{
		ID:          "n-abc123",
		SourceID:    "official-stellarium",
		URL:         "https://example.com/news/1",
		Title:       "ステラリウム、全国ツアー開催",
		PublishedAt: time.Date(2026, 10, 12, 12, 0, 0, 0, domain.Tokyo),
		FetchedAt:   time.Date(2026, 10, 12, 12, 1, 0, 0, domain.Tokyo),
		ContentHash: "hash",
		RawText:     "11月8日（日）東京 Spotify O-EAST\nOPEN 17:00 / START 18:00",
		ArtistHints: []string{"ステラリウム"},
		Revision:    3,
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "cache.json")
	want := sample()
	if err := Save(path, "0.7.0", []domain.PublicAnnouncement{want}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("announcements = %d, want 1", len(got))
	}
	a := got[0]
	if a.ID != want.ID || a.SourceID != want.SourceID || a.URL != want.URL || a.Title != want.Title {
		t.Errorf("identity = %+v", a)
	}
	if !a.PublishedAt.Equal(want.PublishedAt) || !a.FetchedAt.Equal(want.FetchedAt) {
		t.Errorf("times = %v / %v", a.PublishedAt, a.FetchedAt)
	}
	if a.ContentHash != want.ContentHash || a.RawText != want.RawText || a.Revision != want.Revision {
		t.Errorf("body = %+v", a)
	}
	if len(a.ArtistHints) != 1 || a.ArtistHints[0] != want.ArtistHints[0] {
		t.Errorf("artist hints = %v", a.ArtistHints)
	}
}

func TestChangeLogSurvivesRestart(t *testing.T) {
	opts := dedup.Options{Artists: []domain.Artist{{Name: "ステラリウム"}}}
	anns := []domain.PublicAnnouncement{
		{
			SourceID: "official-x", URL: "https://example.com/1", Title: "ステラリウム、全国ツアー開催",
			RawText:     "▼ツアー情報\n\"ステラリウム TOUR 2026「残光」\"\n11月8日（日）東京 Spotify O-EAST",
			PublishedAt: time.Date(2026, 10, 10, 12, 0, 0, 0, domain.Tokyo),
			FetchedAt:   time.Date(2026, 10, 10, 12, 0, 0, 0, domain.Tokyo),
		},
		{
			SourceID: "official-x", URL: "https://example.com/2", Title: "ステラリウム、東京公演の詳細発表",
			RawText:     "▼公演情報\n\"ステラリウム TOUR 2026「残光」\"\n11月8日（日）東京 Spotify O-EAST\nOPEN 17:00 / START 18:00",
			PublishedAt: time.Date(2026, 10, 11, 12, 0, 0, 0, domain.Tokyo),
			FetchedAt:   time.Date(2026, 10, 11, 12, 0, 0, 0, domain.Tokyo),
		},
	}
	before := catalog.New(opts)
	before.Ingest(anns)
	first := before.Rebuild()
	if len(first.Events) != 1 || len(first.Events[0].Changes) == 0 {
		t.Fatalf("events = %+v", first.Events)
	}

	path := filepath.Join(t.TempDir(), "cache.json")
	if err := Save(path, "0.7.0", before.Announcements()); err != nil {
		t.Fatal(err)
	}
	saved, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	after := catalog.New(opts)
	after.Restore(saved)
	second := after.Snapshot()

	if len(second.Events) != 1 {
		t.Fatalf("events after restart = %d, want 1", len(second.Events))
	}
	got, want := second.Events[0], first.Events[0]
	if got.StartDate != want.StartDate || got.VenueName != want.VenueName || got.StartAt == nil || got.StartAt.String() != "18:00" {
		t.Errorf("event after restart = %+v", got)
	}
	if len(got.Changes) != len(want.Changes) {
		t.Fatalf("changes after restart = %+v, want %+v", got.Changes, want.Changes)
	}
	for i := range want.Changes {
		if got.Changes[i].Field != want.Changes[i].Field || got.Changes[i].Old != want.Changes[i].Old || got.Changes[i].New != want.Changes[i].New {
			t.Errorf("change %d = %+v, want %+v", i, got.Changes[i], want.Changes[i])
		}
	}
}

func TestMissingFileIsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || got != nil {
		t.Fatalf("Load = %v, %v; want nil, nil", got, err)
	}
}

func TestVersionMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	b, err := json.Marshal(file{Version: Version + 1, Announcements: []domain.PublicAnnouncement{sample()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "版本") {
		t.Fatalf("Load = %v, %v; want a version error", got, err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("a version mismatch must keep the file: %v", statErr)
	}
}

func TestCorruptFileIsMovedAside(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"announcements":[`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "损坏") {
		t.Fatalf("Load = %v, %v; want a corrupt error", got, err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("corrupt cache still at %s", path)
	}
	if _, statErr := os.Stat(path + ".corrupt"); statErr != nil {
		t.Fatalf("corrupt copy missing: %v", statErr)
	}
}

func TestEmptySaveKeepsPrevious(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := Save(path, "0.7.0", []domain.PublicAnnouncement{sample()}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, "0.7.0", nil); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("announcements = %d, want the previous snapshot", len(got))
	}
}

func TestSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if err := Save(filepath.Join(dir, "cache.json"), "0.7.0", []domain.PublicAnnouncement{sample()}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "cache.json" {
			t.Errorf("leftover file %s", e.Name())
		}
	}
}
