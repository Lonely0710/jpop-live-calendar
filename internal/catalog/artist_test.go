package catalog

import (
	"testing"
	"time"

	"jpop-live-calendar/internal/domain"
)

func day(y int, m int, d int) domain.Date { return domain.Date{Year: y, Month: time.Month(m), Day: d} }

func TestArtistSummariesInfersTourFromRepeatedTitle(t *testing.T) {
	today := day(2026, 10, 21)
	snap := &Snapshot{
		Artists: []domain.Artist{{ID: "a1", Name: "A"}},
		Events: []domain.PublicEvent{
			{ID: "e1", ArtistIDs: []string{"a1"}, Title: "A Tour 2026", StartDate: day(2026, 11, 1)},
			{ID: "e2", ArtistIDs: []string{"a1"}, Title: "A Tour 2026", StartDate: day(2026, 12, 1)},
			{ID: "e3", ArtistIDs: []string{"a1"}, Title: "Rock Festival 2019", StartDate: day(2019, 8, 1)},
		},
		Rounds: []domain.PublicTicketRound{
			{ID: "r1", ArtistIDs: []string{"a1"}, EventIDs: []string{"e1"}},
		},
	}
	sums := snap.ArtistSummaries(today)
	if len(sums) != 1 {
		t.Fatalf("summaries = %+v", sums)
	}
	tours := sums[0].Tours
	if len(tours) != 2 {
		t.Fatalf("tours = %+v", tours)
	}
	if tours[0].Name != "A Tour 2026" || len(tours[0].Events) != 2 || len(tours[0].Rounds) != 1 {
		t.Errorf("first tour = %+v", tours[0])
	}
	if tours[1].Name != "" || len(tours[1].Events) != 1 {
		t.Errorf("second tour = %+v", tours[1])
	}
}
