// Package views builds the interface of Live Calendar with MyGo.
//
// The App holds all UI state; View builds the window from it on every frame
// (immediate mode). The UI never fetches anything while it builds: syncs
// run in the background through the crawler and the views read the
// catalog's latest immutable snapshot.
package views

import (
	"context"
	"image"
	"image/color"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/avatar"
	"jpop-live-calendar/internal/cache"
	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/crawler"
	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
)

// Section is the page the sidebar chose.
type Section int

const (
	SectionCalendar Section = iota
	SectionArtists
	SectionSources
	SectionSettings
)

// Mode is the calendar's view; its order is the toolbar's segmented control.
type Mode int

const (
	ModeMonth Mode = iota
	ModeWeek
	ModeDay
	ModeAgenda
)

var modeLabels = []string{"月", "周", "日", "列表"}

// Search date scopes.
const (
	ScopeAll = iota
	ScopeUpcoming
	ScopePast
)

var scopeLabels = []string{"全部日期", "今天起", "已过去"}

// Options wires the App to its environment; the zero value works offline
// with the demo source.
type Options struct {
	Now       func() time.Time
	Go        func(func()) // runs background work; default: a goroutine
	Post      func(func()) // runs fn on the UI thread (mygo Window.Update)
	Config    config.Config
	Catalog   *catalog.Catalog
	Scheduler *crawler.Scheduler
	Version   string

	// Avatars holds the artists' pictures and theme colors; AvatarGetters
	// finds them when Start runs (none: no lookups, as in tests).
	Avatars       *avatar.Store
	AvatarGetters avatar.Getters
	// SaveAvatars writes the pictures to the disk cache; nil keeps them in
	// memory only (tests).
	SaveAvatars func(map[string]avatar.Avatar) error

	// RemoveSource deletes a source from the configuration file and
	// returns the file it wrote; nil keeps deletions in memory (tests).
	RemoveSource func(id string) (path string, err error)

	// AddSource writes a source to the configuration file and returns the
	// file it wrote; nil keeps additions in memory (tests).
	AddSource func(src domain.SourceConfig) (path string, err error)

	// Pinned lists the artists shown first; SavePins writes the list and
	// nil keeps pinning in memory (tests).
	Pinned   []string
	SavePins func(ids []string) error

	// CachePath is the JSON snapshot of the announcements from the last
	// sync: loaded before the first frame and rewritten after each sync.
	// Empty disables the cache (tests).
	CachePath string

	// Notify shows a system notification of a deadline; nil disables the
	// reminders (tests). Notified holds the deadlines already announced by
	// their notification ID, and SaveNotified writes the map.
	Notify       func(Reminder) error
	Notified     map[string]string
	SaveNotified func(map[string]string) error
}

// App is the state of the main window.
type App struct {
	opt  Options
	snap *catalog.Snapshot

	section      Section
	mode         int // Mode, as an int for ui.Segmented
	cursor       domain.Date
	selected     domain.Date
	entryID      string // selected entry; "" shows the selected day
	query        string
	inspector    bool
	sideHide     bool    // the sidebar is collapsed
	sideShown    float32 // how far the sidebar is shown this frame, 0 to 1
	userOpen     bool
	sideWidth    float32 // expanded sidebar width, dragged by its edge
	inspectWidth float32 // inspector width, dragged by its edge

	filterOpen  bool
	kindOn      [5]bool // by domain.Kinds
	sourceOff   map[string]*bool
	onlyChanged bool
	onlyReview  bool
	scope       int
	region      string // allRegions or a japanRegions label
	city        string // allCities or a region capital label

	showChanges  bool
	showSources  bool
	scrollHours  bool
	artistID     string
	pins         map[string]bool
	srcOn        map[string]*bool
	rawOpen      map[string]*bool
	tourOpen     map[string]*bool  // the artist page's tours, by artist and name
	tourPick     map[string]string // the artist page's tour filter, by artist ID; "" shows all
	tourPickOpen bool              // the artist page's tour filter popover is open
	hideEnded    bool              // the artist page hides tours whose shows have all ended
	deleteOpen   map[string]*bool  // the delete dialogs of the sources
	sourceForm   sourceDraft       // the 数据源 page's "新增数据源" form

	bitmaps     map[string]*ui.Bitmap // avatar pictures, made once
	avatarSVGs  map[string]*ui.SVG    // default avatars, recolored and parsed once
	backdropBmp *ui.Bitmap
	colors      map[string]ui.Color // configured theme colors by artist ID

	notified   map[string]string // deadline notifications already shown
	notifyBusy bool              // a batch is being shown on a goroutine
	notifyOff  bool              // showing failed this run (denied, unsupported)

	cacheMu  sync.Mutex // serializes cache saves from overlapping syncs
	avatarMu sync.Mutex // serializes avatar cache saves
}

// New returns the app showing today's month.
func New(opt Options) *App {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.Go == nil {
		opt.Go = func(f func()) { go f() }
	}
	if opt.Post == nil {
		opt.Post = func(f func()) { f() }
	}
	if opt.Config.Sources == nil {
		opt.Config = config.Default()
	}
	if opt.Catalog == nil {
		opt.Catalog = catalog.New(dedup.Options{Artists: opt.Config.Artists, SourceNames: sourceNames(opt.Config.Sources)})
	}
	if opt.Scheduler == nil {
		opt.Scheduler = crawler.New(opt.Catalog, opt.Config.Sources)
		opt.Scheduler.Now = opt.Now
	}
	if opt.Version == "" {
		opt.Version = "1.0.1"
	}
	if opt.Avatars == nil {
		opt.Avatars = avatar.NewStore()
	}
	post := opt.Post
	opt.Scheduler.OnChange = func() { post(func() {}) } // redraw
	opt.Avatars.OnChange = func() { post(func() {}) }
	// The snapshot of the last sync shows before the first frame; the
	// background sync then updates it as usual.
	if opt.CachePath != "" {
		anns, err := cache.Load(opt.CachePath)
		if err != nil {
			log.Printf("cache: %v", err)
		}
		if len(anns) > 0 {
			opt.Catalog.SetInclude(opt.Scheduler.Enabled)
			opt.Catalog.Restore(anns)
			log.Printf("cache: restored %d announcements from %s", len(anns), opt.CachePath)
		}
	}
	today := domain.DateOf(opt.Now(), theme.DisplayZone)
	notified := map[string]string{}
	for id, at := range opt.Notified {
		notified[id] = at
	}
	a := &App{opt: opt, cursor: today, selected: today, inspector: true,
		sourceOff: map[string]*bool{}, srcOn: map[string]*bool{}, rawOpen: map[string]*bool{}, tourOpen: map[string]*bool{}, tourPick: map[string]string{}, deleteOpen: map[string]*bool{}, showSources: true, showChanges: true,
		bitmaps: map[string]*ui.Bitmap{}, avatarSVGs: map[string]*ui.SVG{}, colors: map[string]ui.Color{}, pins: map[string]bool{},
		notified: notified, sourceForm: newSourceDraft(), region: allRegions, city: allCities,
		sideWidth: theme.SidebarWidth, inspectWidth: theme.InspectorWidth}
	for _, id := range opt.Pinned {
		a.pins[id] = true
	}
	for _, ar := range opt.Config.Artists {
		if ar.Color != "" {
			a.colors[avatar.ArtistID(ar)] = ui.Hex(ar.Color)
		}
	}
	for i := range a.kindOn {
		a.kindOn[i] = true
	}
	a.snap = opt.Catalog.Snapshot()
	opt.Scheduler.AfterSync = func() {
		a.saveCache()
		// Fresh deadlines: the UI thread shows the reminders, as a system
		// notification may wait for the permission.
		a.opt.Post(a.notifyDeadlines)
		// Avatars that failed while a site was down (or its robots.txt was
		// unreadable) are retried after the sync that may have fixed it.
		a.refreshAvatars(context.Background(), false)
	}
	return a
}

func sourceNames(srcs []domain.SourceConfig) map[string]string {
	m := map[string]string{}
	for _, s := range srcs {
		m[s.ID] = s.Name
	}
	return m
}

// Start syncs every enabled source now and then on each source's interval,
// and looks up the artists' avatars in the background.
func (a *App) Start(ctx context.Context) {
	a.opt.Scheduler.Start(ctx)
	a.refreshAvatars(ctx, true)
	// The cache restored before the first frame may already hold deadlines.
	a.notifyDeadlines()
	// Deadlines can enter the reminder window while the app is idle in the
	// menu bar, with no sync due: check again every half hour. Avatars of a
	// site that was down are retried on the same tick.
	go func() {
		t := time.NewTicker(30 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				a.opt.Post(a.notifyDeadlines)
				a.refreshAvatars(ctx, false)
			}
		}
	}()
}

// refreshAvatars checks the cached pictures (only when asked) and retries
// the ones that failed. Store.Fetch skips the ones already found, so a site
// that came back gets its pictures without a restart; the disk cache is
// written once the pass ends.
func (a *App) refreshAvatars(ctx context.Context, revalidate bool) {
	g := a.opt.AvatarGetters
	if g.Pictures == nil {
		return
	}
	a.opt.Go(func() {
		ran := false
		if revalidate {
			ran = a.opt.Avatars.Revalidate(ctx, g, a.opt.Config.Artists)
		}
		if a.opt.Avatars.Fetch(ctx, g, a.opt.Config.Artists) {
			ran = true
		}
		if ran {
			a.saveAvatars()
		}
	})
}

// saveAvatars writes the public pictures for the next launch.
func (a *App) saveAvatars() {
	save := a.opt.SaveAvatars
	if save == nil {
		return
	}
	a.avatarMu.Lock()
	defer a.avatarMu.Unlock()
	if err := save(a.opt.Avatars.Snapshot()); err != nil {
		log.Printf("avatars.json: %v", err)
	}
}

func (a *App) today() domain.Date { return domain.DateOf(a.opt.Now(), theme.DisplayZone) }

// saveCache keeps the announcements of this sync for the next launch. It
// runs on the syncing goroutine, never on the UI thread.
func (a *App) saveCache() {
	if a.opt.CachePath == "" {
		return
	}
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if err := cache.Save(a.opt.CachePath, a.opt.Version, a.opt.Catalog.Announcements()); err != nil {
		log.Printf("cache: %v", err)
	}
}

// Refresh syncs the given sources (all enabled when none) in the
// background.
func (a *App) Refresh(ids ...string) {
	s := a.opt.Scheduler
	if len(ids) == 0 && s.Running() {
		return
	}
	a.opt.Go(func() {
		s.RunOnce(context.Background(), ids...)
		a.opt.Post(func() {})
	})
}

func (a *App) sourceName(id string) string {
	for _, s := range a.opt.Config.Sources {
		if s.ID == id {
			return s.Name
		}
	}
	return id
}

func (a *App) sourceKind(id string) domain.SourceKind {
	for _, s := range a.opt.Config.Sources {
		if s.ID == id {
			return s.Kind
		}
	}
	return ""
}

// Filtering.

func (a *App) kindVisible(k domain.EntryKind) bool {
	for i, x := range domain.Kinds {
		if x == k {
			return a.kindOn[i]
		}
	}
	return true
}

func (a *App) filtersActive() bool {
	for _, on := range a.kindOn {
		if !on {
			return true
		}
	}
	for _, off := range a.sourceOff {
		if *off {
			return true
		}
	}
	return a.onlyChanged || a.onlyReview || a.scope != ScopeAll ||
		(a.region != "" && a.region != allRegions) || (a.city != "" && a.city != allCities)
}

func (a *App) resetFilters() {
	for i := range a.kindOn {
		a.kindOn[i] = true
	}
	for _, off := range a.sourceOff {
		*off = false
	}
	a.onlyChanged, a.onlyReview, a.scope = false, false, ScopeAll
	a.region, a.city = allRegions, allCities
}

func (a *App) match(e domain.Entry, q []string) bool {
	if !a.kindVisible(e.Kind) || (a.onlyChanged && !e.Changed) || (a.onlyReview && !e.NeedsReview) {
		return false
	}
	if !a.placeMatches(e) {
		return false
	}
	if len(a.sourceOff) > 0 {
		shown := false
		for _, id := range e.SourceIDs {
			if off, ok := a.sourceOff[id]; !ok || !*off {
				shown = true
				break
			}
		}
		if !shown {
			return false
		}
	}
	for _, t := range q {
		if !strings.Contains(e.Search, t) {
			return false
		}
	}
	return true
}

func (a *App) terms() []string { return strings.Fields(strings.ToLower(a.query)) }

// entries returns the visible entries from first to last.
func (a *App) entries(first, last domain.Date) []domain.Entry {
	q := a.terms()
	var out []domain.Entry
	for _, e := range a.snap.Between(first, last) {
		if a.match(e, q) {
			out = append(out, e)
		}
	}
	return out
}

// searchResults returns every entry matching the query in the scope.
func (a *App) searchResults() []domain.Entry {
	q := a.terms()
	today := a.today()
	var out []domain.Entry
	for _, e := range a.snap.Entries {
		switch a.scope {
		case ScopeUpcoming:
			if e.Date.Before(today) {
				continue
			}
		case ScopePast:
			if !e.Date.Before(today) {
				continue
			}
		}
		if a.match(e, q) {
			out = append(out, e)
		}
	}
	return out
}

func (a *App) selectEntry(e domain.Entry) {
	a.entryID = e.ID
	a.selected = e.Date
	a.inspector = true
}

func (a *App) showEntry(id string) {
	if e, ok := a.snap.Entry(id); ok {
		a.section = SectionCalendar
		if Mode(a.mode) == ModeAgenda {
			a.mode = int(ModeMonth)
		}
		a.cursor = e.Date
		a.selectEntry(e)
	}
}

// Navigation.

func (a *App) step(n int) {
	switch Mode(a.mode) {
	case ModeMonth:
		a.cursor = a.cursor.AddMonths(n)
	case ModeWeek:
		a.cursor = a.cursor.AddDays(7 * n)
	case ModeAgenda:
		a.cursor = a.cursor.AddDays(30 * n)
	default:
		a.cursor = a.cursor.AddDays(n)
	}
	if Mode(a.mode) == ModeDay {
		a.selected, a.entryID = a.cursor, ""
	}
}

func (a *App) goToday() {
	a.cursor = a.today()
	a.selected, a.entryID = a.cursor, ""
}

func (a *App) setMode(m Mode) {
	if Mode(a.mode) != m {
		a.mode = int(m)
		a.scrollHours = true
	}
}

func (a *App) shortcuts(c *ui.Context) {
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyLeft):
		a.step(-1)
	case c.Shortcut(ui.Cmd, ui.KeyRight):
		a.step(1)
	case c.Shortcut(ui.Cmd, ui.KeyT):
		a.goToday()
	case c.Shortcut(ui.Cmd, ui.KeyR):
		a.Refresh()
	case c.Shortcut(ui.Cmd|ui.Alt, ui.KeyI):
		a.inspector = !a.inspector
	case c.Shortcut(sidebarModifiers, ui.KeyS):
		a.sideHide = !a.sideHide
	case c.Shortcut(0, ui.KeyEscape) && a.entryID != "":
		a.entryID = ""
	}
	for i, k := range []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4} {
		if c.Shortcut(ui.Cmd, k) {
			a.section = SectionCalendar
			a.setMode(Mode(i))
		}
	}
}

// View builds the window: a warm backdrop under two floating panes of
// Liquid Glass, the sidebar and the inspector, both collapsible, and the
// content on a Cloud White sheet between them.
func (a *App) View(c *ui.Context) {
	a.snap = a.opt.Catalog.Snapshot()
	c.SetTheme(theme.Spotlight())
	if a.sideWidth <= 0 {
		a.sideWidth = theme.SidebarWidth
	}
	if a.inspectWidth <= 0 {
		a.inspectWidth = theme.InspectorWidth
	}
	a.shortcuts(c)
	prev := a.mode
	bar := c.TitleBar()
	c.Root().Background(theme.BackdropBottom)
	ui.Box(c).Fill().Children(func() {
		// Absolute elements paint over the ones in the flow: the backdrop
		// is in the flow, the window's content floats over it.
		a.backdrop(c)
		col := ui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0)
		shown := col.Animate("sidebar", b2f(!a.sideHide), 250*time.Millisecond)
		a.sideShown = shown
		inspect := col.Animate("inspector", b2f(a.inspectorShown()), 220*time.Millisecond)
		// The sidebar narrows to the rail of icons as it collapses.
		sideW := theme.RailWidth + (a.sideWidth-theme.RailWidth)*shown
		col.Children(func() {
			a.toolbar(c, bar, theme.PaneInset+sideW+theme.PaneGap)
			ui.Row(c).Grow(1).Shrink(1).AlignItems(ui.Stretch).Padding(0, theme.PaneInset, theme.PaneInset).
				Gap(theme.PaneGap).Children(func() {
				ui.Column(c).Width(sideW).Shrink(0).Radius(theme.PaneRadius).Material(glass.Glass{}).Clip().
					Children(func() {
						if sideW > (a.sideWidth+theme.RailWidth)/2 {
							ui.Column(c).Width(a.sideWidth).Grow(1).Shrink(0).Children(func() { a.sidebar(c) })
						} else {
							a.rail(c)
						}
					})
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Background(theme.CloudWhite).Radius(theme.SheetRadius).
					Shadow(0, 1, 3, 0, theme.SheetShadow).Clip().Children(func() {
					switch a.section {
					case SectionCalendar:
						a.calendar(c)
					case SectionArtists:
						a.artistsPage(c)
					case SectionSources:
						a.sourcesPage(c)
					case SectionSettings:
						a.settingsPage(c)
					}
				})
				// The inspector slides in from the right, on the calendar only:
				// the other pages fill the window's width.
				if slot := (a.inspectWidth + theme.PaneGap) * inspect; slot > 0.5 {
					// The row's gap is the slot's own, so it closes with it.
					ui.Row(c).Width(slot).Shrink(0).AlignItems(ui.Stretch).Margin(0, 0, 0, -theme.PaneGap).
						Clip().Children(func() {
						ui.Column(c).Width(a.inspectWidth).Shrink(0).Margin(0, 0, 0, theme.PaneGap).
							Radius(theme.PaneRadius).Material(glass.Glass{}).Children(func() {
							a.inspectorResizer(c)
							a.inspectorPanel(c)
						})
					})
				}
				a.sideResizer(c, sideW, shown)
			})
		})
	})
	if a.mode != prev {
		a.scrollHours = true
	}
}

func (a *App) inspectorShown() bool { return a.inspector && a.section == SectionCalendar }

// sideResizer is the sidebar's right edge: drag it, or focus it and use the
// arrows, to change the expanded sidebar's width. It is hidden while the
// sidebar collapses to its rail.
func (a *App) sideResizer(c *ui.Context, sideW float32, shown float32) {
	if a.sideHide || shown < 0.99 {
		return
	}
	h := ui.Box(c).Absolute().Left(theme.PaneInset + sideW - 3).Top(0).FillHeight().Width(6).
		Cursor(ui.CursorResizeEW).Focusable().Role(ui.RoleSplitter).
		Label("调整侧边栏宽度").Tooltip("拖动调整侧边栏宽度；聚焦后按 ←/→")
	if dx, _, ok := h.Dragged(); ok && dx != 0 {
		a.sideWidth = clampWidth(a.sideWidth+dx, 180, 340)
	}
	if h.Shortcut(0, ui.KeyLeft) {
		a.sideWidth = clampWidth(a.sideWidth-10, 180, 340)
	}
	if h.Shortcut(0, ui.KeyRight) {
		a.sideWidth = clampWidth(a.sideWidth+10, 180, 340)
	}
	h.Draw(func(p *ui.Painter, r ui.Rect) {
		if h.Hovered() || h.FocusVisible() {
			p.Fill(ui.Rect{X: r.X + r.W/2 - 0.5, Y: r.Y + 8, W: 1, H: max(r.H-16, 0)}, theme.LiveCoral.Alpha(0.7), 0)
		}
	})
}

// inspectorResizer is the inspector's left edge.
func (a *App) inspectorResizer(c *ui.Context) {
	h := ui.Box(c).Absolute().Left(0).Top(0).FillHeight().Width(6).
		Cursor(ui.CursorResizeEW).Focusable().Role(ui.RoleSplitter).
		Label("调整详情面板宽度").Tooltip("拖动调整详情面板宽度；聚焦后按 ←/→")
	if dx, _, ok := h.Dragged(); ok && dx != 0 {
		a.inspectWidth = clampWidth(a.inspectWidth-dx, 260, 400)
	}
	if h.Shortcut(0, ui.KeyLeft) {
		a.inspectWidth = clampWidth(a.inspectWidth+10, 260, 400)
	}
	if h.Shortcut(0, ui.KeyRight) {
		a.inspectWidth = clampWidth(a.inspectWidth-10, 260, 400)
	}
	h.Draw(func(p *ui.Painter, r ui.Rect) {
		if h.Hovered() || h.FocusVisible() {
			p.Fill(ui.Rect{X: r.X + r.W/2 - 0.5, Y: r.Y + 8, W: 1, H: max(r.H-16, 0)}, theme.LiveCoral.Alpha(0.7), 0)
		}
	})
}

func clampWidth(v, lo, hi float32) float32 { return min(max(v, lo), hi) }

// backdrop is the warm picture the glass shows through: Paper Ivory
// lit by two soft glows of the brand colors. It is drawn once, small, and
// scaled to the window: blurry by nature, it needs no detail.
func (a *App) backdrop(c *ui.Context) {
	if a.backdropBmp == nil {
		a.backdropBmp = ui.NewBitmap(backdropImage(320, 205))
	}
	ui.Image(c, a.backdropBmp).Fit(ui.Cover).Fill()
}

func backdropImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	type glow struct {
		x, y, r float64 // center and radius, in fractions of the width
		col     ui.Color
		alpha   float64
	}
	glows := []glow{
		{0.08, -0.05, 0.42, theme.WarmSpotlight, 0.85},
		{0.98, 1.05, 0.45, theme.LiveCoral, 0.30},
		{0.62, 0.15, 0.30, theme.CloudWhite, 0.55},
	}
	for y := 0; y < h; y++ {
		base := theme.BackdropTop.Mix(theme.BackdropBottom, float32(y)/float32(h))
		for x := 0; x < w; x++ {
			r, g, b := float64(base.R), float64(base.G), float64(base.B)
			for _, gl := range glows {
				dx := float64(x)/float64(w) - gl.x
				dy := (float64(y) - gl.y*float64(h)) / float64(w)
				k := gl.alpha * math.Exp(-(dx*dx+dy*dy)/(gl.r*gl.r))
				r += (float64(gl.col.R) - r) * k
				g += (float64(gl.col.G) - g) * k
				b += (float64(gl.col.B) - b) * k
			}
			img.SetRGBA(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
		}
	}
	return img
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
