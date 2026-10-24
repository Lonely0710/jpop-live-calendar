package views

import (
	"testing"
	"time"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/domain"
)

func TestPlaceFilters(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 29, 0, 0, domain.Tokyo)
	app := New(Options{Config: config.Default(), Catalog: demoCatalog(t, now), Now: func() time.Time { return now }})
	var tokyo, osaka domain.Entry
	for _, e := range app.snap.Entries {
		prefs := app.entryPrefectures(e)
		if e.Kind != domain.EntryLive {
			continue
		}
		if tokyo.ID == "" && len(prefs) == 1 && prefs[0] == "東京" {
			tokyo = e
		}
		if osaka.ID == "" && len(prefs) == 1 && prefs[0] == "大阪" {
			osaka = e
		}
	}
	if tokyo.ID == "" || osaka.ID == "" {
		t.Fatalf("demo data lacks Tokyo/Osaka entries: %q %q", tokyo.ID, osaka.ID)
	}
	if !app.placeMatches(tokyo) || !app.placeMatches(osaka) {
		t.Error("all places must match the defaults")
	}
	app.region = "kanto"
	if !app.placeMatches(tokyo) || app.placeMatches(osaka) {
		t.Error("kanto should keep Tokyo and drop Osaka")
	}
	app.region, app.city = allRegions, "osaka"
	if app.placeMatches(tokyo) || !app.placeMatches(osaka) {
		t.Error("osaka should keep Osaka and drop Tokyo")
	}
	app.region, app.city = "kansai", "osaka"
	if !app.placeMatches(osaka) {
		t.Error("kansai + osaka should keep Osaka")
	}
	if app.placeMatches(tokyo) {
		t.Error("kansai + osaka should drop Tokyo")
	}
	if !hasString(cityIDs("kanto"), "tokyo") || hasString(cityIDs("kanto"), "osaka") {
		t.Error("kanto city options should be its capital only")
	}
}
