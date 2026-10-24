package views

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/geo"
)

func mapApp(t *testing.T) *App {
	t.Helper()
	now := time.Date(2026, 10, 21, 21, 29, 0, 0, domain.Tokyo)
	return New(Options{Config: config.Default(), Catalog: demoCatalog(t, now), Now: func() time.Time { return now }})
}

func TestMapStats(t *testing.T) {
	app := mapApp(t)
	app.mapScope = MapAll
	all := app.mapStats()
	if all.total == 0 || len(all.prefs()) < 2 {
		t.Fatalf("demo data: %d shows in %d prefectures", all.total, len(all.prefs()))
	}
	placed := len(all.unknown)
	for pref, evs := range all.byPref {
		if geo.Japan().Find(pref) < 0 {
			t.Errorf("%q is not on the map", pref)
		}
		for _, e := range evs {
			if e.Kind != domain.EntryLive {
				t.Errorf("%s: a %s entry on the map", pref, e.Kind)
			}
		}
		placed += len(evs)
	}
	if placed != all.total {
		t.Errorf("placed %d of %d shows", placed, all.total)
	}

	app.mapScope = MapUpcoming
	up := app.mapStats()
	app.mapScope = MapPast
	past := app.mapStats()
	if up.total+past.total != all.total {
		t.Errorf("upcoming %d + past %d != all %d", up.total, past.total, all.total)
	}

	// The filters apply to the map as to the calendar.
	app.mapScope = MapAll
	app.region = "kanto"
	for pref := range app.mapStats().byPref {
		if r, _ := regionOf(pref); r.id != "kanto" {
			t.Errorf("kanto filter kept %s", pref)
		}
	}
}

func TestMapSelection(t *testing.T) {
	app := mapApp(t)
	app.mapScope = MapAll
	st := app.mapStats()
	prefs := st.prefs()
	app.stepPref(st, 1)
	if app.mapPref != prefs[0] {
		t.Fatalf("first step chose %q, want %q", app.mapPref, prefs[0])
	}
	app.stepPref(st, -1)
	if app.mapPref != prefs[len(prefs)-1] {
		t.Errorf("stepping back from the first chose %q", app.mapPref)
	}

	// A prefecture's marker sits on its capital.
	m := &app.mapCam
	m.box = ui.Rect{W: 800, H: 600}
	s, ox, oy := m.camera(m.box)
	cap := geo.Japan().Prefectures[geo.Japan().Find(prefs[0])].Capital
	if got := m.hitMarker(st, ox+cap.X*s, oy+cap.Y*s); got < 0 {
		t.Errorf("no marker on %s's capital", prefs[0])
	}

	// Zooming keeps the point under the pointer.
	before := m.toMap(200, 150)
	m.zoomAt(2, 200, 150)
	after := m.toMap(200, 150)
	if d := (before.X-after.X)*(before.X-after.X) + (before.Y-after.Y)*(before.Y-after.Y); d > 0.01 {
		t.Errorf("zoom moved the point under the pointer from %v to %v", before, after)
	}
	m.reset()
	if m.zoom != mapMinZoom {
		t.Errorf("reset left zoom %v", m.zoom)
	}
}

// TestMapPage drives the page in a window without a screen: the sidebar's
// 地图 opens it with the inspector on all Japan, and choosing a
// prefecture lists its shows. LIVE_CALENDAR_SNAPSHOT=dir saves pictures.
func TestMapPage(t *testing.T) {
	app := mapApp(t)
	app.mapScope = MapAll
	tt := ui.NewTester(app.View, 1280, 820)
	tt.Frame()
	if err := tt.Click("地图"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if app.section != SectionMap {
		t.Fatalf("section %v after clicking 地图", app.section)
	}
	if !tt.HasText("日本全国") {
		t.Error("the inspector should show all Japan")
	}
	snapshot(t, tt, "map.png")

	st := app.mapStats()
	pref := "東京"
	if err := tt.Click(geo.Suffixed(pref)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if app.mapPref != pref {
		t.Fatalf("chose %q, want %q", app.mapPref, pref)
	}
	if !tt.HasText(st.byPref[pref][0].Title) && !tt.HasText(app.artistNames(st.byPref[pref][0].ArtistIDs)) {
		t.Errorf("%s's shows are not listed", pref)
	}
	snapshot(t, tt, "map-pref.png")

	// Clicking a prefecture on the map chooses it too.
	m := &app.mapCam
	b := m.box
	target := geo.Japan().Prefectures[geo.Japan().Find(st.prefs()[len(st.prefs())-1])]
	s, ox, oy := m.camera(ui.Rect{W: b.W, H: b.H})
	tt.ClickAt(b.X+ox+target.Capital.X*s, b.Y+oy+target.Capital.Y*s)
	tt.Frame()
	if app.mapPref != target.Name {
		t.Errorf("clicking %s's marker chose %q", target.Name, app.mapPref)
	}
	m.zoomAt(2.5, b.W*0.62, b.H*0.62)
	tt.Frame()
	tt.Frame()
	snapshot(t, tt, "map-zoom.png")
}

func snapshot(t *testing.T, tt *ui.Tester, name string) {
	dir := os.Getenv("LIVE_CALENDAR_SNAPSHOT")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
