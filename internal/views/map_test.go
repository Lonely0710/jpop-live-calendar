package views

import (
	"image/png"
	"math"
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

// Two prefectures whose capitals sit closer together than their markers are
// wide — Tokyo and Kanagawa — end up touching, not covering each other, and
// the busier one keeps its ground.
func TestMarkersDoNotOverlap(t *testing.T) {
	st := mapStats{byPref: map[string][]domain.Entry{}}
	for name, n := range map[string]int{"東京": 6, "神奈川": 2, "大阪": 3, "兵庫": 3, "京都": 3} {
		st.byPref[name] = make([]domain.Entry, n)
	}
	var m mapView
	ms := m.layoutMarkers(st, ui.Rect{W: 900, H: 620})
	if len(ms) != 5 {
		t.Fatalf("markers = %d", len(ms))
	}
	for i := range ms {
		for j := i + 1; j < len(ms); j++ {
			a, b := ms[i], ms[j]
			d := float32(math.Hypot(float64(b.x-a.x), float64(b.y-a.y)))
			if need := a.r + b.r + mapMarkerGap; d < need-0.5 {
				t.Errorf("%d and %d shows are %.1f apart, %.1f needed", a.n, b.n, d, need)
			}
		}
	}
	// Tokyo is busier than Kanagawa, so it is the one that moved less.
	moved := func(mk mapMarker) float32 {
		return float32(math.Hypot(float64(mk.x-mk.ax), float64(mk.y-mk.ay)))
	}
	var tokyo, kanagawa mapMarker
	for _, mk := range ms {
		switch mk.n {
		case 6:
			tokyo = mk
		case 2:
			kanagawa = mk
		}
	}
	if moved(tokyo) >= moved(kanagawa) {
		t.Errorf("Tokyo moved %.1f, Kanagawa %.1f: the busier should give way less", moved(tokyo), moved(kanagawa))
	}
}

// A frame's paint may run more than once, so the outlines of a camera are
// new paths rather than the old ones reset: the paths an earlier run
// already painted keep their geometry.
func TestMapOutlinesAreRemadeNotReset(t *testing.T) {
	app := mapApp(t)
	app.mapScope = MapAll
	tt := ui.NewTester(app.View, 1280, 820)
	tt.Frame()
	if err := tt.Click("地图"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	m := &app.mapCam
	if len(m.paths) == 0 {
		t.Fatal("no outlines after the first frame")
	}
	first := m.paths[0]
	box := first.Bounds()
	if box.W <= 0 || box.H <= 0 {
		t.Fatalf("the first outline is empty: %+v", box)
	}
	m.zoomAt(2, m.box.W/2, m.box.H/2)
	tt.Frame()
	tt.Frame()
	if len(m.paths) == 0 {
		t.Fatal("no outlines after zooming")
	}
	if m.paths[0] == first {
		t.Error("the outlines were reset in place; a paint already recorded would lose them")
	}
	if got := first.Bounds(); got != box {
		t.Errorf("the painted outline changed under the frame: %+v, want %+v", got, box)
	}
}

// clipRing keeps a ring inside the box as it is, cuts one that crosses the
// edge to it, and drops one wholly outside.
func TestClipRing(t *testing.T) {
	box := ui.Rect{X: 0, Y: 0, W: 100, H: 100}
	var buf []ui.Point
	square := func(x, y, w float32) []ui.Point {
		return []ui.Point{{X: x, Y: y}, {X: x + w, Y: y}, {X: x + w, Y: y + w}, {X: x, Y: y + w}}
	}
	if got := clipRing(square(10, 10, 50), box, &buf); len(got) != 4 {
		t.Errorf("a ring inside the box = %d points, want 4", len(got))
	}
	if got := clipRing(square(200, 200, 50), box, &buf); got != nil {
		t.Errorf("a ring outside the box = %v, want nothing", got)
	}
	// A ring far larger than the box comes back as the box itself.
	got := clipRing(square(-1000, -1000, 3000), box, &buf)
	if len(got) < 4 {
		t.Fatalf("a ring around the box = %d points", len(got))
	}
	for _, p := range got {
		if p.X < box.X-0.5 || p.X > box.X+box.W+0.5 || p.Y < box.Y-0.5 || p.Y > box.Y+box.H+0.5 {
			t.Errorf("clipped point %+v is outside %+v", p, box)
			break
		}
	}
	// Half in, half out: the part kept ends at the box's edge.
	got = clipRing(square(50, 50, 200), box, &buf)
	if len(got) == 0 {
		t.Fatal("a ring half inside came back empty")
	}
	for _, p := range got {
		if p.X > box.X+box.W+0.5 || p.Y > box.Y+box.H+0.5 {
			t.Errorf("clipped point %+v is past the box's edge", p)
			break
		}
	}
}

// Zoomed in, MyGo drops a path whose mask passes 4096 pixels a side, so the
// outlines are clipped to the box: deep in, the land is still there, and no
// path is bigger than the box.
func TestMapOutlinesSurviveDeepZoom(t *testing.T) {
	app := mapApp(t)
	app.mapScope = MapAll
	tt := ui.NewTester(app.View, 1280, 820)
	tt.Frame()
	if err := tt.Click("地图"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	m := &app.mapCam
	m.zoom = 16
	tt.Frame()
	tt.Frame()
	box := m.box
	if len(m.paths) == 0 {
		t.Fatal("no outlines at all")
	}
	drawn := 0
	for i, p := range m.paths {
		b := p.Bounds()
		if b.W == 0 && b.H == 0 {
			continue // outside the view, as it should be
		}
		drawn++
		if b.W > box.W+48+1 || b.H > box.H+48+1 {
			t.Errorf("%s is %+.0fx%+.0f, past the box %+.0fx%+.0f",
				geo.Japan().Prefectures[i].Name, b.W, b.H, box.W, box.H)
		}
	}
	if drawn == 0 {
		t.Error("not one prefecture outlines the view at 16x")
	}
}
