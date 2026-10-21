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
	app.region = "关东"
	if !app.placeMatches(tokyo) || app.placeMatches(osaka) {
		t.Error("关东 should keep Tokyo and drop Osaka")
	}
	app.region, app.city = allRegions, "大阪"
	if app.placeMatches(tokyo) || !app.placeMatches(osaka) {
		t.Error("大阪 should keep Osaka and drop Tokyo")
	}
	app.region, app.city = "关西（近畿）", "大阪"
	if !app.placeMatches(osaka) {
		t.Error("关西 + 大阪 should keep Osaka")
	}
	if app.placeMatches(tokyo) {
		t.Error("关西 + 大阪 should drop Tokyo")
	}
	if !hasString(cityOptions("关东"), "东京") || hasString(cityOptions("关东"), "大阪") {
		t.Error("关东 city options should be its capital only")
	}
}
