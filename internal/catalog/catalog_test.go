package catalog

import (
	"testing"
	"time"

	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
)

func TestRestoreKeepsHiddenSourcesAndRevisions(t *testing.T) {
	c := New(dedup.Options{})
	c.SetInclude(func(id string) bool { return id == "a" })
	anns := []domain.PublicAnnouncement{
		{ID: "n-a", SourceID: "a", URL: "https://a.example/1", Title: "A", RawText: "11月8日 東京 BIGCAT", FetchedAt: time.Now(), Revision: 3},
		{ID: "n-b", SourceID: "b", URL: "https://b.example/1", Title: "B", RawText: "11月9日 大阪 BIGCAT", FetchedAt: time.Now(), Revision: 2},
	}
	c.Restore(anns)
	if got := len(c.Snapshot().Announcements); got != 1 {
		t.Fatalf("visible announcements = %d, want 1", got)
	}
	all := c.Announcements()
	if len(all) != 2 {
		t.Fatalf("stored announcements = %d, want 2", len(all))
	}
	for _, a := range all {
		want := 2
		if a.SourceID == "a" {
			want = 3
		}
		if a.Revision != want {
			t.Errorf("%s revision = %d, want %d", a.ID, a.Revision, want)
		}
	}
}
