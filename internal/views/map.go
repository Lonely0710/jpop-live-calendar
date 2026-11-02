package views

import (
	"fmt"
	"math"
	"sort"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/geo"
	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/locales"
)

// The map page: Japan's prefectures shaded by how many Live shows they
// hold, a marker with the count at each capital, and the inspector on the
// right listing the chosen prefecture's shows. It reads the snapshot
// only, as every page does; the outlines are embedded (package geo).

// mapUnknown is the inspector's list of the shows whose prefecture is not
// announced: they are counted, never placed on a guess.
const mapUnknown = "\x00unknown"

// Map date scopes; the map starts on the shows to come.
const (
	MapUpcoming = iota
	MapAll
	MapPast
)

func mapScopeLabel(scope int) string {
	switch scope {
	case MapAll:
		return locales.MapScopeAll()
	case MapPast:
		return locales.MapScopePast()
	}
	return locales.MapScopeToday()
}

func mapScopeLabels() []string {
	out := make([]string, 3)
	for i := range out {
		out[i] = mapScopeLabel(i)
	}
	return out
}

const (
	mapMinZoom = 1
	mapMaxZoom = 8
)

// mapView is the map's camera and what it drew last; it holds no
// ui.Element, only numbers and paths in DIPs.
type mapView struct {
	zoom    float32
	center  geo.Point // the map point at the box's center
	centred bool      // center was set (the zero value centers the map)
	box     ui.Rect   // the box the map drew in last
	hover   int       // the prefecture under the pointer, or -1
	moved   float32   // how far the press being made has dragged
	pressed bool

	// The prefectures' outlines in DIPs, rebuilt when the camera or the
	// box changes.
	paths []*ui.Path
	key   [5]float32
	// ring and clipBuf are the scratch of clipping the outlines.
	ring, clipBuf []ui.Point
}

// mapStats are the shows on the map: the Live entries that the search,
// the filters and the map's date scope keep, by prefecture.
type mapStats struct {
	byPref  map[string][]domain.Entry
	unknown []domain.Entry
	total   int
	max     int
}

func (a *App) mapStats() mapStats {
	st := mapStats{byPref: map[string][]domain.Entry{}}
	q := a.terms()
	today := a.today()
	for _, e := range a.snap.Entries {
		if e.Kind != domain.EntryLive {
			continue
		}
		switch a.mapScope {
		case MapUpcoming:
			if e.Date.Before(today) {
				continue
			}
		case MapPast:
			if !e.Date.Before(today) {
				continue
			}
		}
		if !a.match(e, q) {
			continue
		}
		st.total++
		prefs := a.entryPrefectures(e)
		if len(prefs) == 0 {
			st.unknown = append(st.unknown, e)
			continue
		}
		for _, p := range prefs {
			st.byPref[p] = append(st.byPref[p], e)
			st.max = max(st.max, len(st.byPref[p]))
		}
	}
	if a.mapScope == MapPast {
		// The latest first, as the past is looked back on.
		for _, evs := range st.byPref {
			reverseEntries(evs)
		}
		reverseEntries(st.unknown)
	}
	return st
}

func reverseEntries(evs []domain.Entry) {
	for i, j := 0, len(evs)-1; i < j; i, j = i+1, j-1 {
		evs[i], evs[j] = evs[j], evs[i]
	}
}

// mapPrefs are the prefectures holding shows, in the order of their codes.
func (st mapStats) prefs() []string {
	var out []string
	for _, p := range geo.Japan().Prefectures {
		if len(st.byPref[p.Name]) > 0 {
			out = append(out, p.Name)
		}
	}
	return out
}

// selectPref chooses a prefecture for the inspector ("" for all Japan).
func (a *App) selectPref(name string) {
	a.mapPref = name
	a.entryID = ""
	a.inspector = true
}

// stepPref chooses the next (dir 1) or previous prefecture with shows.
func (a *App) stepPref(st mapStats, dir int) {
	prefs := st.prefs()
	if len(prefs) == 0 {
		return
	}
	i := -1
	for j, p := range prefs {
		if p == a.mapPref {
			i = j
		}
	}
	switch {
	case i < 0 && dir < 0:
		i = len(prefs) - 1
	case i < 0:
		i = 0
	default:
		i = (i + dir + len(prefs)) % len(prefs)
	}
	a.selectPref(prefs[i])
}

func (a *App) mapPage(c *ui.Context) {
	st := a.mapStats()
	ui.Column(c).Grow(1).Shrink(1).FillWidth().Padding(22, theme.ContentPadding+4).Gap(14).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			pageHeader(c, locales.MapTitle())
			ui.Row(c).Gap(8).Shrink(0).AlignItems(ui.Center).Tooltip(locales.MapStatsTooltip() + " · " + locales.MapSource()).Children(func() {
				stat(c, fmt.Sprint(st.total), "Live")
				divider(c)
				stat(c, fmt.Sprint(len(st.prefs())), locales.MapPrefectures())
			})
			ui.Spacer(c)
			dateScopeChoice(c, &a.mapScope, mapScopeLabels(), locales.MapDateRange())
		})
		card(c).Grow(1).Shrink(1).MinHeight(0).Background(mapSea).Clip().Children(func() {
			a.mapCanvas(c, st)
			a.mapLegend(c, st)
			a.mapControls(c)
		})
	})
}

var (
	mapSea      = theme.CloudWhite.Mix(theme.PaperIvory, 0.5)
	mapLand     = theme.PaperIvory.Mix(theme.MistGray, 0.55)
	mapLandHot  = theme.LiveCoral.Mix(theme.WarmSpotlight, 0.25)
	mapBorder   = theme.CloudWhite
	mapInsetCol = theme.MistGray
)

// prefColor shades a prefecture by its count of shows: Warm Spotlight
// for one, toward Live Coral for the busiest.
func prefColor(n, most int) ui.Color {
	if n == 0 {
		return mapLand
	}
	t := float32(0.15)
	if most > 1 {
		t += 0.85 * float32(math.Log(float64(n))/math.Log(float64(most)))
	}
	return theme.WarmSpotlight.Mix(mapLandHot, t)
}

// markerRadius grows with the count, gently.
func markerRadius(n int) float32 {
	return min(8+2.6*float32(math.Sqrt(float64(n))), 19)
}

// mapCamera returns the scale (DIPs a map unit) and the origin that put
// the camera's center at the box's center.
func (m *mapView) camera(r ui.Rect) (s, ox, oy float32) {
	jp := geo.Japan()
	if !m.centred {
		m.center, m.centred = geo.Point{X: jp.Size.X / 2, Y: jp.Size.Y / 2}, true
	}
	if m.zoom < mapMinZoom {
		m.zoom = mapMinZoom
	}
	fit := min(r.W/jp.Size.X, r.H/jp.Size.Y) * 0.94
	s = fit * m.zoom
	return s, r.X + r.W/2 - m.center.X*s, r.Y + r.H/2 - m.center.Y*s
}

// toMap converts a point of the box's local coordinates to map units.
func (m *mapView) toMap(x, y float32) geo.Point {
	r := ui.Rect{W: m.box.W, H: m.box.H}
	s, ox, oy := m.camera(r)
	return geo.Point{X: (x - ox) / s, Y: (y - oy) / s}
}

// zoomAt zooms by f keeping the map point under (x, y), local to the box,
// where it is.
func (m *mapView) zoomAt(f, x, y float32) {
	before := m.toMap(x, y)
	m.zoom = min(max(m.zoom*f, mapMinZoom), mapMaxZoom)
	after := m.toMap(x, y)
	m.center.X += before.X - after.X
	m.center.Y += before.Y - after.Y
	m.clamp()
}

// pan moves the map by dx, dy DIPs.
func (m *mapView) pan(dx, dy float32) {
	s, _, _ := m.camera(ui.Rect{W: m.box.W, H: m.box.H})
	m.center.X -= dx / s
	m.center.Y -= dy / s
	m.clamp()
}

// clamp keeps the camera's center over the map.
func (m *mapView) clamp() {
	jp := geo.Japan()
	m.center.X = min(max(m.center.X, 0), jp.Size.X)
	m.center.Y = min(max(m.center.Y, 0), jp.Size.Y)
	if m.zoom <= mapMinZoom {
		m.center = geo.Point{X: jp.Size.X / 2, Y: jp.Size.Y / 2}
	}
}

func (m *mapView) reset() {
	m.zoom, m.centred = mapMinZoom, false
}

// hitMarker returns the prefecture whose marker is under (x, y), local
// to the box, where the markers are laid out, the topmost first, or -1.
func (m *mapView) hitMarker(st mapStats, x, y float32) int {
	ms := m.layoutMarkers(st, ui.Rect{W: m.box.W, H: m.box.H})
	for k := len(ms) - 1; k >= 0; k-- { // drawn last, on top
		mk := ms[k]
		rad := mk.r + 2
		if (x-mk.x)*(x-mk.x)+(y-mk.y)*(y-mk.y) <= rad*rad {
			return mk.pref
		}
	}
	return -1
}

// mapMarker is one prefecture's count marker: its anchor at the capital
// and where it is drawn, in DIPs.
type mapMarker struct {
	pref, n int
	ax, ay  float32 // the capital
	x, y, r float32 // the marker
}

// moved says the marker was pushed so far that it no longer covers its
// capital, which a dot and a leader line then show.
func (mk mapMarker) moved() bool {
	return float32(math.Hypot(float64(mk.x-mk.ax), float64(mk.y-mk.ay))) > mk.r-1
}

// mapMarkerGap is the room left between two markers.
const mapMarkerGap = 3

// layoutMarkers places the markers of the prefectures holding shows so
// that none covers another: Tokyo and Kanagawa, or the six of Kansai, sit
// closer together than their markers are wide. Each starts on its capital
// and overlapping pairs push apart, the busier one moving less, while a
// spring pulls each back home; it settles where the markers just touch.
// Zoomed in, the capitals are far enough apart and nothing moves. The
// result is in drawing order, the busiest last.
func (m *mapView) layoutMarkers(st mapStats, r ui.Rect) []mapMarker {
	jp := geo.Japan()
	s, ox, oy := m.camera(r)
	ms := make([]mapMarker, 0, len(st.byPref))
	for i, pr := range jp.Prefectures {
		n := len(st.byPref[pr.Name])
		if n == 0 {
			continue
		}
		x, y := ox+pr.Capital.X*s, oy+pr.Capital.Y*s
		ms = append(ms, mapMarker{pref: i, n: n, ax: x, ay: y, x: x, y: y, r: markerRadius(n)})
	}
	// Busiest first while laying out, so they claim their capitals.
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].n > ms[j].n })
	const springIters, maxIters = 40, 160
	for it := 0; it < maxIters; it++ {
		overlap := false
		for i := range ms {
			for j := i + 1; j < len(ms); j++ {
				a, b := &ms[i], &ms[j]
				dx, dy := b.x-a.x, b.y-a.y
				d := float32(math.Hypot(float64(dx), float64(dy)))
				need := a.r + b.r + mapMarkerGap
				if d >= need-0.01 {
					continue
				}
				overlap = true
				if d < 0.5 {
					// On top of each other: part along their capitals,
					// or straight down and right when those coincide.
					dx, dy = b.ax-a.ax, b.ay-a.ay
					if d = float32(math.Hypot(float64(dx), float64(dy))); d < 0.5 {
						dx, dy, d = 0.6, 0.8, 1
					}
				}
				ux, uy := dx/d, dy/d
				push := need - d
				// The share each moves goes against its area.
				wa := b.r * b.r / (a.r*a.r + b.r*b.r)
				a.x -= ux * push * wa
				a.y -= uy * push * wa
				b.x += ux * push * (1 - wa)
				b.y += uy * push * (1 - wa)
			}
		}
		if it < springIters {
			for k := range ms {
				ms[k].x += (ms[k].ax - ms[k].x) * 0.12
				ms[k].y += (ms[k].ay - ms[k].y) * 0.12
			}
			continue
		}
		if !overlap {
			break
		}
	}
	// Draw the quiet ones first, the busiest on top.
	for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
		ms[i], ms[j] = ms[j], ms[i]
	}
	return ms
}

// mapCanvas is the map itself: drag to move it, scroll (or ⌘ and the
// trackpad) to zoom, click a prefecture or its marker to list its shows.
// With the focus, ←/→ choose among the prefectures holding shows, + and −
// zoom, 0 shows the whole country and Esc goes back to it.
func (a *App) mapCanvas(c *ui.Context, st mapStats) {
	m := &a.mapCam
	jp := geo.Japan()
	box := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Focusable().Role(ui.RoleImage).
		Label(locales.MapControlTooltip())
	if m.pressed {
		box.Cursor(ui.CursorGrabbing)
	} else if m.hover >= 0 {
		box.Cursor(ui.CursorPointer)
	} else {
		box.Cursor(ui.CursorGrab)
	}
	box.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputScroll {
			return false
		}
		if ev.Precise && ev.Mods&(ui.Cmd|ui.Ctrl) == 0 {
			// Two fingers on a trackpad move the map, as in Maps.
			m.pan(-ev.DX, -ev.DY)
			return true
		}
		m.zoomAt(float32(math.Pow(1.0025, float64(-ev.DY))), ev.X, ev.Y)
		return true
	})
	x, y, over := box.PointerPosition()
	m.hover = -1
	if over {
		if i := m.hitMarker(st, x, y); i >= 0 {
			m.hover = i
		} else {
			m.hover = jp.Hit(m.toMap(x, y))
		}
	}
	if box.Pressed() {
		if !m.pressed {
			m.pressed, m.moved = true, 0
		}
	} else {
		m.pressed = false
	}
	if dx, dy, ok := box.Dragged(); ok && (dx != 0 || dy != 0) {
		m.moved += float32(math.Abs(float64(dx)) + math.Abs(float64(dy)))
		m.pan(dx, dy)
	}
	if box.DoubleClicked() {
		if over {
			m.zoomAt(2, x, y)
		}
	} else if box.Clicked() && m.moved < 4 {
		switch {
		case !over: // Enter or Space with the focus: the next prefecture
			a.stepPref(st, 1)
		case m.hover >= 0:
			a.selectPref(jp.Prefectures[m.hover].Name)
		default:
			a.selectPref("")
		}
	}
	switch {
	case box.Shortcut(0, ui.KeyRight), box.Shortcut(0, ui.KeyDown):
		a.stepPref(st, 1)
	case box.Shortcut(0, ui.KeyLeft), box.Shortcut(0, ui.KeyUp):
		a.stepPref(st, -1)
	case box.Shortcut(0, ui.KeyEqual), box.Shortcut(ui.Shift, ui.KeyEqual):
		m.zoomAt(1.5, m.box.W/2, m.box.H/2)
	case box.Shortcut(0, ui.KeyMinus):
		m.zoomAt(1/1.5, m.box.W/2, m.box.H/2)
	case box.Shortcut(0, ui.Key0):
		m.reset()
	case box.Shortcut(0, ui.KeyEscape) && (a.mapPref != "" || a.entryID != ""):
		a.selectPref("")
	}
	selected := a.mapHighlight()
	focus := box.FocusVisible()
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		m.box = r
		a.drawMap(p, r, st, selected)
		if focus {
			p.FocusRing(ui.Rect{X: r.X + 2, Y: r.Y + 2, W: r.W - 4, H: r.H - 4}, [4]float32{theme.CardRadius, theme.CardRadius, theme.CardRadius, theme.CardRadius})
		}
	})
}

// mapHighlight is the prefecture the inspector shows: the chosen one, or
// the one of the chosen show.
func (a *App) mapHighlight() map[string]bool {
	out := map[string]bool{}
	if e, ok := a.snap.Entry(a.entryID); ok {
		for _, p := range a.entryPrefectures(e) {
			out[p] = true
		}
	}
	if a.mapPref != "" && a.mapPref != mapUnknown {
		out[a.mapPref] = true
	}
	return out
}

func (a *App) drawMap(p *ui.Painter, r ui.Rect, st mapStats, selected map[string]bool) {
	m := &a.mapCam
	jp := geo.Japan()
	s, ox, oy := m.camera(r)
	// The outlines are remade rather than reset in place: a frame's paint
	// may run more than once, so a path already handed to the painter keeps
	// its own geometry.
	//
	// Zoomed in, a prefecture spans thousands of pixels, and MyGo drops a
	// path whose mask passes 4096 pixels a side: the land vanished, leaving
	// the markers in the sea. So each outline is clipped to the box, with a
	// margin that keeps the cut edges and their strokes out of sight.
	if key := [5]float32{s, ox, oy, r.W, r.H}; key != m.key || len(m.paths) != len(jp.Prefectures) {
		m.key = key
		const margin = 24
		view := ui.Rect{X: r.X - margin, Y: r.Y - margin, W: r.W + 2*margin, H: r.H + 2*margin}
		paths := make([]*ui.Path, len(jp.Prefectures))
		for i, pr := range jp.Prefectures {
			path := new(ui.Path)
			for _, ring := range pr.Rings {
				m.ring = m.ring[:0]
				for _, pt := range ring {
					m.ring = append(m.ring, ui.Point{X: ox + pt.X*s, Y: oy + pt.Y*s})
				}
				pts := clipRing(m.ring, view, &m.clipBuf)
				if len(pts) < 3 {
					continue
				}
				for j, pt := range pts {
					if j == 0 {
						path.MoveTo(pt.X, pt.Y)
					} else {
						path.LineTo(pt.X, pt.Y)
					}
				}
				path.Close()
			}
			paths[i] = path
		}
		m.paths = paths
	}
	p.Clip(r, theme.CardRadius, func() {
		// The inset of the Nansei Islands.
		in := jp.Inset
		ir := ui.Rect{X: ox + in.X*s, Y: oy + in.Y*s, W: in.W * s, H: in.H * s}
		p.Fill(ir, theme.CloudWhite.Alpha(0.6), 8)
		p.StrokeDashed(ir, mapInsetCol, 8, 1)
		p.Text(ir.X+10, ir.Y+8, locales.MapInset(), 10.5, theme.TextSecondary)

		width := min(max(s*2.2, 0.6), 1.6)
		for i, pr := range jp.Prefectures {
			n := len(st.byPref[pr.Name])
			col := prefColor(n, st.max)
			if i == m.hover {
				col = col.Mix(theme.InkCharcoal, 0.08)
			}
			p.FillPath(m.paths[i], col)
			p.StrokePath(m.paths[i], width, mapBorder)
		}
		for i, pr := range jp.Prefectures {
			if selected[pr.Name] {
				p.StrokePath(m.paths[i], 2, theme.LiveCoral)
			} else if i == m.hover {
				p.StrokePath(m.paths[i], 1.2, theme.InkCharcoal.Alpha(0.35))
			}
		}
		// The markers, laid out so none covers another, the busiest last
		// so they draw on top. One pushed off its capital keeps a dot
		// there and a leader line to it.
		ms := m.layoutMarkers(st, r)
		// The leaders belong to this draw, as the outlines do: a frame's
		// paint may run more than once, and a path reset under an earlier
		// run paints the later one's lines.
		lead := new(ui.Path)
		for _, mk := range ms {
			if mk.moved() {
				lead.MoveTo(mk.ax, mk.ay)
				lead.LineTo(mk.x, mk.y)
			}
		}
		p.StrokePath(lead, 1.2, theme.InkCharcoal.Alpha(0.45))
		for _, mk := range ms {
			if mk.moved() {
				p.Fill(ui.Rect{X: mk.ax - 3.5, Y: mk.ay - 3.5, W: 7, H: 7}, theme.CloudWhite, 3.5)
				p.Fill(ui.Rect{X: mk.ax - 2.5, Y: mk.ay - 2.5, W: 5, H: 5}, theme.LiveCoral, 2.5)
			}
		}
		for _, mk := range ms {
			i, n, cx, cy, rad := mk.pref, mk.n, mk.x, mk.y, mk.r
			pr := jp.Prefectures[i]
			ring := theme.CloudWhite
			if selected[pr.Name] {
				ring = theme.InkCharcoal
			}
			p.Shadow(ui.Rect{X: cx - rad, Y: cy - rad, W: 2 * rad, H: 2 * rad}, rad, 0, 1, 4, 0, theme.InkCharcoal.Alpha(0.25))
			p.Fill(ui.Rect{X: cx - rad - 1.5, Y: cy - rad - 1.5, W: 2*rad + 3, H: 2*rad + 3}, ring, rad+1.5)
			fill := theme.LiveCoral
			if i == m.hover {
				fill = fill.Mix(theme.InkCharcoal, 0.15)
			}
			p.Fill(ui.Rect{X: cx - rad, Y: cy - rad, W: 2 * rad, H: 2 * rad}, fill, rad)
			label := ui.Span{Text: fmt.Sprint(n), Size: min(max(rad*0.85, 10), 13), Weight: 700, Color: theme.CloudWhite}
			w, h := p.MeasureText(0, label)
			p.RichText(cx-w/2, cy-h/2, 0, label)
			// Names show once the map is large enough to fit them.
			if m.zoom >= 2.2 || selected[pr.Name] {
				name := ui.Span{Text: prefLabel(pr.Name), Size: 11, Weight: 600, Color: theme.InkCharcoal}
				nw, nh := p.MeasureText(0, name)
				lr := ui.Rect{X: cx + rad + 4, Y: cy - nh/2 - 2, W: nw + 10, H: nh + 4}
				p.Fill(lr, theme.CloudWhite.Alpha(0.9), 6)
				p.RichText(lr.X+5, lr.Y+2, 0, name)
			}
		}
		// The prefecture under the pointer, named by the pointer.
		if m.hover >= 0 {
			pr := jp.Prefectures[m.hover]
			text := prefLabel(pr.Name)
			if n := len(st.byPref[pr.Name]); n > 0 {
				text += locales.MapPrefShows(n)
			} else {
				text += locales.MapNoLive()
			}
			span := ui.Span{Text: text, Size: 12, Weight: 600, Color: theme.CloudWhite}
			w, h := p.MeasureText(0, span)
			tr := ui.Rect{X: r.X + 14, Y: r.Y + 14, W: w + 18, H: h + 10}
			p.Fill(tr, theme.InkCharcoal.Alpha(0.85), (h+10)/2)
			p.RichText(tr.X+9, tr.Y+5, 0, span)
		}
	})
}

// clipRing clips a closed ring to the rectangle v (Sutherland–Hodgman),
// one edge of v at a time; buf holds the scratch. A ring wholly inside v
// comes back as it is, one wholly outside as nothing.
func clipRing(pts []ui.Point, v ui.Rect, buf *[]ui.Point) []ui.Point {
	if len(pts) == 0 {
		return nil
	}
	minX, minY, maxX, maxY := pts[0].X, pts[0].Y, pts[0].X, pts[0].Y
	for _, p := range pts[1:] {
		minX, maxX = min(minX, p.X), max(maxX, p.X)
		minY, maxY = min(minY, p.Y), max(maxY, p.Y)
	}
	x0, y0, x1, y1 := v.X, v.Y, v.X+v.W, v.Y+v.H
	switch {
	case maxX < x0 || minX > x1 || maxY < y0 || minY > y1:
		return nil
	case minX >= x0 && maxX <= x1 && minY >= y0 && maxY <= y1:
		return pts
	}
	in := append((*buf)[:0], pts...)
	var out []ui.Point
	for edge := range 4 {
		inside := func(p ui.Point) bool {
			switch edge {
			case 0:
				return p.X >= x0
			case 1:
				return p.X <= x1
			case 2:
				return p.Y >= y0
			}
			return p.Y <= y1
		}
		cross := func(a, b ui.Point) ui.Point {
			var t float32
			switch edge {
			case 0:
				t = (x0 - a.X) / (b.X - a.X)
			case 1:
				t = (x1 - a.X) / (b.X - a.X)
			case 2:
				t = (y0 - a.Y) / (b.Y - a.Y)
			default:
				t = (y1 - a.Y) / (b.Y - a.Y)
			}
			return ui.Point{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
		}
		out = out[:0]
		for k, cur := range in {
			prev := in[(k+len(in)-1)%len(in)]
			switch ci, pi := inside(cur), inside(prev); {
			case ci && pi:
				out = append(out, cur)
			case ci:
				out = append(out, cross(prev, cur), cur)
			case pi:
				out = append(out, cross(prev, cur))
			}
		}
		if len(out) == 0 {
			return nil
		}
		in, out = out, in
	}
	*buf = in
	return in
}

// mapLegend is the key, in the Pacific next to the zoom buttons: the shades, the shows without a known
// prefecture, and the outlines' source.
func (a *App) mapLegend(c *ui.Context, st mapStats) {
	ui.Column(c).Absolute().Right(14+theme.ToolButton+6+12).Bottom(14).Padding(10, 12).Gap(6).Radius(theme.Radius).
		Background(theme.CloudWhite.Alpha(0.92)).Border(1, theme.MistGray).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			caption(c, locales.MapLegendLess())
			for _, t := range []int{1, 2, 4, 8} {
				most := max(st.max, 8)
				ui.Box(c).Size(18, 10).Radius(3).Background(prefColor(t*most/8, most))
			}
			caption(c, locales.MapLegendMore())
			ui.Box(c).Width(6)
			dot(c, theme.LiveCoral, 10)
			caption(c, locales.MapLegendCount())
		})
		if n := len(st.unknown); n > 0 {
			b := ui.ButtonBase(c).Row().Gap(6).AlignItems(ui.Center).Radius(4).Label(locales.MapUnknownButton(n))
			if b.Hovered() {
				b.Background(theme.Hover)
			}
			if b.Children(func() {
				mygoicons.Image(c, icons.Venue, 13)
				caption(c, locales.MapUnknownCaption(n))
			}).Clicked() {
				a.selectPref(mapUnknown)
			}
		}
	})
}

// mapControls are the zoom buttons in the corner.
func (a *App) mapControls(c *ui.Context) {
	m := &a.mapCam
	ui.Column(c).Absolute().Right(14).Bottom(14).Padding(3).Gap(2).Radius(theme.Radius).
		Background(theme.CloudWhite.Alpha(0.92)).Border(1, theme.MistGray).Children(func() {
		if iconButton(c, icons.Add, locales.MapZoomIn(), false, m.zoom >= mapMaxZoom).Clicked() {
			m.zoomAt(1.5, m.box.W/2, m.box.H/2)
		}
		if iconButton(c, icons.Remove, locales.MapZoomOut(), false, m.zoom <= mapMinZoom).Clicked() {
			m.zoomAt(1/1.5, m.box.W/2, m.box.H/2)
		}
		if iconButton(c, icons.Home, locales.MapReset(), false, m.zoom <= mapMinZoom).Clicked() {
			m.reset()
		}
	})
}

// mapInspector is the inspector on the map page: all Japan by region, a
// prefecture's shows, or the shows without a known prefecture.
func (a *App) mapInspector(c *ui.Context) {
	st := a.mapStats()
	scope := mapScopeLabel(a.mapScope)
	switch a.mapPref {
	case "":
		ui.Column(c).Padding(0, 18, 14).Gap(4).BorderWidth(0, 0, 1, 0).BorderColor(theme.GlassLine).Children(func() {
			ui.Text(c, locales.MapNational()).FontSize(24).Bold()
			caption(c, locales.MapNationalSummary(scope, st.total, len(st.prefs())))
		})
		if st.total == 0 {
			emptyState(c, icons.MapJapan, locales.MapEmptyTitle(), locales.MapEmptyBody())
			return
		}
		ui.Scroll(c).Grow(1).Label(locales.MapRegionsLabel()).Children(func() {
			ui.Column(c).Padding(10, 12, 14).Gap(4).Children(func() {
				for _, r := range japanRegions {
					n := 0
					for _, p := range r.prefectures {
						n += len(st.byPref[p])
					}
					if n == 0 {
						continue
					}
					c.Key("region:" + r.id)
					ui.Row(c).Padding(8, 6, 2).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, regionLabel(r.id)).FontSize(theme.CaptionSize).Bold().TextColor(theme.TextSecondary)
						ui.Spacer(c)
						caption(c, locales.CalendarLivesCount(n))
					})
					for _, p := range r.prefectures {
						if k := len(st.byPref[p]); k > 0 {
							a.prefRow(c, p, prefLabel(p), k, st.max)
						}
					}
				}
				if n := len(st.unknown); n > 0 {
					ui.Box(c).Height(6)
					a.prefRow(c, mapUnknown, locales.MapUnknown(), n, 0)
				}
			})
		})
	default:
		title, sub := locales.MapUnknown(), locales.MapUnknownBody()
		evs := st.unknown
		if a.mapPref != mapUnknown {
			title, evs = prefLabel(a.mapPref), st.byPref[a.mapPref]
			sub = locales.MapJapan()
			if r, ok := regionOf(a.mapPref); ok {
				sub = regionLabel(r.id)
			}
			sub += locales.MapPrefSummary(scope, len(evs))
		}
		ui.Column(c).Padding(0, 18, 14).Gap(4).BorderWidth(0, 0, 1, 0).BorderColor(theme.GlassLine).Children(func() {
			ui.Text(c, title).FontSize(24).Bold()
			caption(c, sub)
			if venues := venueCount(evs); venues > 1 {
				caption(c, locales.MapVenues(venues))
			}
		})
		if len(evs) == 0 {
			emptyState(c, icons.MapJapan, locales.MapPrefEmptyTitle(), locales.MapPrefEmptyBody())
			return
		}
		ui.Scroll(c).Grow(1).Label(locales.MapPrefLives(title)).Children(func() {
			ui.Column(c).Padding(8, 14, 14).Gap(6).Children(func() {
				var cur domain.Date
				for _, e := range evs {
					if e.Date != cur {
						cur = e.Date
						c.Key(cur)
						label := dayAndWeekday(cur)
						if cur.Year != a.today().Year {
							label = dayTitle(cur)
						}
						if cur == a.today() {
							label += locales.CalendarTodaySuffix()
						}
						ui.Text(c, label).FontSize(theme.CaptionSize).Bold().TextColor(theme.TextSecondary).Padding(6, 2, 0)
					}
					c.Key(e.ID)
					a.chip(c, e, false)
				}
			})
		})
	}
}

// venueCount is how many venues the shows are at.
func venueCount(evs []domain.Entry) int {
	seen := map[string]bool{}
	for _, e := range evs {
		if e.Detail != "" {
			seen[e.Detail] = true
		}
	}
	return len(seen)
}

// prefRow is a prefecture in the inspector's list of all Japan.
func (a *App) prefRow(c *ui.Context, key, label string, n, most int) {
	c.Key("pref:" + key)
	b := ui.ButtonBase(c).FillWidth().Padding(7, 8).Gap(10).Radius(theme.Radius).Justify(ui.Start).
		Label(locales.MapPrefTooltip(label, n))
	if b.Hovered() {
		b.Background(theme.GlassHover)
	}
	if b.Children(func() {
		col := theme.SoftAsh
		if most > 0 {
			col = prefColor(n, most)
		}
		ui.Box(c).Size(12, 12).Radius(3).Background(col)
		ui.Text(c, label).Grow(1).Shrink(1).SingleLine()
		ui.Text(c, fmt.Sprint(n)).FontSize(12).Bold().TextColor(theme.TextSecondary)
		mygoicons.Image(c, icons.Next, 14)
	}).Clicked() {
		a.selectPref(key)
	}
}
