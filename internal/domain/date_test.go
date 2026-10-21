package domain

import (
	"testing"
	"time"
)

func TestTokyo(t *testing.T) {
	utc := time.Date(2025, 10, 19, 16, 30, 0, 0, time.UTC) // 01:30 JST next day
	if got := DateOf(utc, Tokyo); got != (Date{2025, 10, 20}) {
		t.Fatalf("DateOf = %v", got)
	}
}

func TestMonthGrid(t *testing.T) {
	g := Date{2025, 10, 15}.MonthGrid(time.Monday)
	if g[0] != (Date{2025, 9, 29}) || g[0].Weekday() != time.Monday {
		t.Fatalf("first cell = %v", g[0])
	}
	if g[41] != (Date{2025, 11, 9}) {
		t.Fatalf("last cell = %v", g[41])
	}
}

func TestAddMonthsClamps(t *testing.T) {
	if got := (Date{2025, 1, 31}).AddMonths(1); got != (Date{2025, 2, 28}) {
		t.Fatalf("got %v", got)
	}
	if got := (Date{2025, 12, 10}).AddMonths(1); got != (Date{2026, 1, 10}) {
		t.Fatalf("got %v", got)
	}
}

func TestWeekStart(t *testing.T) {
	if got := (Date{2025, 10, 19}).WeekStart(time.Monday); got != (Date{2025, 10, 13}) {
		t.Fatalf("got %v", got)
	}
	if got := (Date{2025, 10, 19}).WeekStart(time.Sunday); got != (Date{2025, 10, 19}) {
		t.Fatalf("got %v", got)
	}
}
