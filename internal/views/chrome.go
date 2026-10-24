package views

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/locales"
)

type navItem struct {
	id        string // the item's ID in the ui.Sidebar
	icon      icons.Name
	label     func() string // in the language the app shows
	section   Section
	mode      Mode // for calendar items
	ownColors bool // the icon keeps its own colors instead of the sidebar's tint
}

var navItems = []navItem{
	{"calendar", icons.Calendar, locales.NavCalendar, SectionCalendar, ModeMonth, false},
	{"list", icons.List, locales.NavList, SectionCalendar, ModeAgenda, false},
	{"map", icons.MapRoute, locales.NavMap, SectionMap, 0, true},
	{"artists", icons.Artist, locales.NavArtists, SectionArtists, 0, false},
	{"sources", icons.DataStatus, locales.NavSources, SectionSources, 0, false},
}

var settingsItem = navItem{"settings", icons.Settings, locales.NavSettings, SectionSettings, 0, false}

// languageItem is the language entry at the bottom of the sidebar: it
// opens the language's popover instead of a page.
var languageItem = navItem{"language", icons.Globe, locales.NavLanguage, -1, 0, false}

// accountItem is the user's entry at the bottom of the sidebar, above the
// settings: it opens the account's popover instead of a page.
var accountItem = navItem{"account", icons.Account, locales.NavAccount, -1, 0, false}

// footerItems are the items at the bottom of the sidebar and the rail.
var footerItems = []navItem{languageItem, accountItem, settingsItem}

// navID is the ID of the sidebar's item the page and the view are.
func (a *App) navID() string {
	switch a.section {
	case SectionCalendar:
		if Mode(a.mode) == ModeAgenda {
			return "list"
		}
		return "calendar"
	case SectionArtists:
		return "artists"
	case SectionSources:
		return "sources"
	case SectionMap:
		return "map"
	}
	return "settings"
}

// navigate goes to the page of the sidebar's item id.
func (a *App) navigate(id string) {
	for _, it := range append(navItems, settingsItem) {
		if it.id != id {
			continue
		}
		if it.section == SectionMap && a.section != SectionMap {
			// The calendar's chosen show stays with the calendar: the map's
			// inspector opens on the prefecture chosen last.
			a.entryID = ""
		}
		a.section = it.section
		a.tourPickOpen = false
		if it.section == SectionCalendar {
			if it.mode == ModeAgenda {
				a.setMode(ModeAgenda)
			} else if Mode(a.mode) == ModeAgenda {
				a.setMode(ModeMonth)
			}
		}
	}
}

// navList is a ui.Sidebar of items, the source list of MyGo: the arrows
// choose among them while it has the focus. With compact, the items
// show only their icons, their labels in tooltips: the rail.
func (a *App) navList(c *ui.Context, items []navItem, title string, compact bool) {
	sel := a.navID()
	var chosen ui.Element
	chosenIcon := icons.Calendar
	chosenOwn := false
	hasChosen := false
	sb := ui.Sidebar(c, &sel, func() {
		build := func() {
			for _, it := range items {
				label := it.label()
				if compact {
					label = ""
				}
				item := ui.SidebarItem(c, it.id, mygoicons.Get(it.icon), label).Height(32)
				if it.ownColors {
					icon := mygoicons.Get(it.icon)
					item.DrawOver(func(p *ui.Painter, r ui.Rect) {
						p.Image(icon, sidebarIconRect(r), ui.Contain)
					})
				}
				if it.id == sel {
					chosen, chosenIcon, chosenOwn, hasChosen = item, it.icon, it.ownColors, true
				}
				if compact {
					item.Label(it.label()).Tooltip(it.label())
				}
				if it.id == accountItem.id {
					if a.userOpen {
						item.Background(theme.GlassSelected)
					}
					ui.Popover(c, item, &a.userOpen, func() { a.userPanel(c) })
				}
				if it.id == languageItem.id {
					if a.langOpen {
						item.Background(theme.GlassSelected)
					}
					ui.Popover(c, item, &a.langOpen, func() { a.languagePanel(c) })
				}
				if b := a.badge(it); b != "" {
					item.Children(func() {
						badge := ui.Badge(c, b).Background(theme.StatusError).TextColor(theme.CloudWhite)
						if compact {
							badge.Absolute().Top(1).Right(1).FontSize(9)
						}
					})
				}
			}
		}
		if title == "" || compact {
			build()
		} else {
			ui.SidebarSection(c, title, nil, build)
		}
	}).Background(ui.Transparent).Padding(0, theme.NavInset).Label(title)
	if compact {
		sb.FillWidth()
	}
	// MyGo paints a chosen item coral while the sidebar has the focus and
	// gray while it lacks it. Use the app's warm selection and its own icon
	// color in both cases, so the item looks the same whether it was
	// chosen by the pointer or by the keyboard.
	if hasChosen {
		chosen.Background(theme.SelectedBg).TextColor(theme.InkCharcoal)
		if sb.Focused() {
			// MyGo already painted the item's icon in the accent text color
			// (white); paint the app's icon over it in the selection's warm
			// background, then the focus ring MyGo would draw itself.
			icon := mygoicons.Get(chosenIcon)
			chosen.DrawOver(func(p *ui.Painter, r ui.Rect) {
				ir := sidebarIconRect(r)
				p.Fill(ir, theme.SelectedBg, 0)
				if chosenOwn {
					p.Image(icon, ir, ui.Contain)
				} else {
					p.Icon(icon, ir, theme.LiveCoral)
				}
				if sb.FocusVisible() {
					p.FocusRing(r, [4]float32{theme.Radius, theme.Radius, theme.Radius, theme.Radius})
				}
			})
		}
	}
	switch {
	case sel == languageItem.id:
		a.langOpen = true
	case sel == accountItem.id:
		a.userOpen = true
	case sel != a.navID():
		a.navigate(sel)
	}
}

// The sidebar item's metrics, as MyGo's SidebarItem lays them out: a row
// with 8 DIPs of horizontal and 6 of vertical padding holding its 16 DIP
// icon, centered vertically in the 32 DIP row.
const (
	sidebarIconSize = 16
	sidebarIconLeft = 8
)

// sidebarIconRect is where a sidebar item paints its icon inside r.
func sidebarIconRect(r ui.Rect) ui.Rect {
	return ui.Rect{
		X: r.X + sidebarIconLeft,
		Y: r.Y + (r.H-sidebarIconSize)/2,
		W: sidebarIconSize,
		H: sidebarIconSize,
	}
}

// userPanel is the account's popover: no sign-in yet.
func (a *App) userPanel(c *ui.Context) {
	ui.Column(c).Width(240).Padding(14).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			mygoicons.Image(c, icons.Account, 20)
			ui.Text(c, locales.NavAccount()).Bold()
		})
		ui.Text(c, locales.AccountSoon()).TextColor(theme.TextSecondary)
		caption(c, locales.AccountNote())
	})
}

// languagePanel is the language's popover: the system's language, or one
// of the languages the app is written in. Each language's name is its
// own, so it reads the same whatever the app shows now.
func (a *App) languagePanel(c *ui.Context) {
	ui.Column(c).Width(240).Padding(14).Gap(10).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			mygoicons.Image(c, icons.Globe, 20)
			ui.Text(c, locales.LangTitle()).Bold()
		})
		ui.RadioGroup(c, func() {
			ui.Radio(c, &a.langChoice, "", locales.LangFollowSystem()).
				OnChange(func() { a.ChooseLocale("") })
			for _, tag := range localeChoices {
				tag := tag
				ui.Radio(c, &a.langChoice, tag, i18n.Autonym(tag)).
					OnChange(func() { a.ChooseLocale(tag) })
			}
		}).Label(locales.LangTitle())
	})
}

// sidebar is the content of the sidebar's pane of glass, under the
// window's top band.
func (a *App) sidebar(c *ui.Context) {
	ui.Column(c).Padding(14, 16, 8, 16).Gap(2).DragWindow().Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			mygoicons.Image(c, icons.Live, 22)
			ui.Text(c, "Jpop Calendar").Font("Pacifico").FontSize(21).SingleLine()
		})
	})
	a.navList(c, navItems, locales.NavBrowse(), false)
	ui.Column(c).Padding(14, 10, 4).Gap(6).Children(func() {
		sectionLabel(c, locales.NavRecentDeadlines())
		a.deadlines(c)
	})
	ui.Spacer(c).DragWindow()
	ui.Box(c).Padding(0, 0, 4).Children(func() {
		a.navList(c, footerItems, "", false)
	})
	a.syncFooter(c)
}

// rail is the collapsed sidebar: the same source lists with only their
// icons, the settings at the bottom, as in the Codex app. Each icon
// stays where it is in the full sidebar, so nothing jumps.
func (a *App) rail(c *ui.Context) {
	ui.Column(c).FillWidth().Padding(14, 0, 6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(36, 32).Center().DragWindow().Tooltip("Live Calendar").Children(func() {
			mygoicons.Image(c, icons.Live, 22)
		})
		ui.Box(c).Height(1).Width(24).Margin(6, 0, 4).Background(theme.GlassLine)
	})
	a.navList(c, navItems, locales.NavBrowse(), true)
	ui.Spacer(c).DragWindow()
	ui.Box(c).FillWidth().Padding(0, 0, 4).Children(func() {
		a.navList(c, footerItems, "", true)
	})
	icon, text, _ := a.syncSummary()
	ui.Box(c).FillWidth().Height(36).Center().Tooltip(text).Margin(0, 0, 8).Children(func() {
		mygoicons.Image(c, icon, 16)
	})
}

func (a *App) badge(it navItem) string {
	if it.section == SectionSources {
		failed := 0
		for _, st := range a.opt.Scheduler.Statuses() {
			if st.State == domain.SyncFailed {
				failed++
			}
		}
		if failed > 0 {
			return fmt.Sprint(failed)
		}
	}
	return ""
}

// deadlines lists the next ticket deadlines in the sidebar.
func (a *App) deadlines(c *ui.Context) {
	today := a.today()
	n := 0
	for _, e := range a.snap.Between(today, today.AddDays(30)) {
		if e.Kind != domain.EntryLotteryClose || !a.match(e, nil) {
			continue
		}
		c.Key("dl:" + e.ID)
		b := ui.ButtonBase(c).FillWidth().Padding(6, 8).Gap(8).Radius(theme.Radius).Justify(ui.Start).
			Label(locales.DeadlineItemLabel(e.Title))
		if b.Hovered() {
			b.Background(theme.GlassHover)
		}
		if b.Children(func() {
			dot(c, theme.StatusWarning, 6)
			ui.Column(c).Shrink(1).Gap(1).Children(func() {
				ui.Text(c, e.Title).Lang("ja").FontSize(theme.CaptionSize).SingleLine()
				left := today.DaysUntil(e.Date)
				label := locales.DeadlineIn(shortDay(e.Date), left)
				if left == 0 {
					label = locales.DeadlineToday()
					if e.At != nil {
						label += " " + e.At.String()
					}
				}
				ui.Text(c, label).FontSize(10.5).TextColor(theme.TextSecondary).SingleLine()
			})
		}).Clicked() {
			a.showEntry(e.ID)
		}
		n++
		if n == 3 {
			break
		}
	}
	if n == 0 {
		caption(c, locales.NoDeadlines()).Padding(0, 8)
	}
}

func sectionLabel(c *ui.Context, s string) {
	ui.Text(c, s).FontSize(theme.CaptionSize).Bold().TextColor(theme.TextSecondary).Padding(8, 8, 4, 8)
}

// syncSummary describes the sync state for the footer and the toolbar.
func (a *App) syncSummary() (icon icons.Name, text string, last time.Time) {
	s := a.opt.Scheduler
	failed, enabled := 0, 0
	for _, st := range s.Statuses() {
		if !st.Config.Enabled {
			continue
		}
		enabled++
		if st.State == domain.SyncFailed {
			failed++
		}
		if st.LastRun.FinishedAt.After(last) {
			last = st.LastRun.FinishedAt
		}
	}
	switch {
	case s.Running():
		return icons.Sync, locales.SyncRunning(), last
	case enabled == 0:
		return icons.Suspended, locales.SyncNoSources(), last
	case last.IsZero():
		return icons.Sync, locales.SyncNever(), last
	case failed > 0:
		return icons.SyncFailed, locales.SyncUpdatedFailed(clockOf(last), failed), last
	}
	return icons.Synced, locales.SyncUpdated(clockOf(last)), last
}

func clockOf(t time.Time) string { return t.In(theme.DisplayZone).Format("15:04") }

func (a *App) syncFooter(c *ui.Context) {
	ui.Row(c).Padding(10, 16, 12, 16).Gap(8).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(theme.GlassLine).Children(func() {
		icon, text, _ := a.syncSummary()
		a.syncGlyph(c, icon, 16)
		caption(c, text).Font("monospace").SingleLine()
	})
}

// syncGlyph draws the sync symbol; it turns while a sync runs, unless the
// desktop asks for less motion.
func (a *App) syncGlyph(c *ui.Context, name icons.Name, size float32) ui.Element {
	icon := mygoicons.Image(c, name, size)
	if name == icons.Sync && a.opt.Scheduler.Running() && !c.Preferences().ReduceMotion {
		icon.Rotate(icon.Loop("sync-spin", time.Second, ui.Linear) * 360)
	}
	return icon
}

// toolbar is the window's top band, one line with the traffic lights:
// the sidebar's toggle next to them, then, from the left edge of the
// content sheet (sheetLeft), the navigation, the title, the view, search
// and the actions, in capsules of glass.
func (a *App) toolbar(c *ui.Context, bar ui.TitleBar, sheetLeft float32) {
	h := max(bar.Height, float32(theme.TitleBarHeight))
	lead := bar.Left + 6 // the toggle, past the traffic lights
	if bar.Left == 0 {
		lead = theme.PaneInset + 4 // full screen: no traffic lights
	}
	toggleW := float32(theme.ToggleButton + 4)
	start := max(sheetLeft+6, lead+toggleW+12)
	ui.Row(c).Height(h).Shrink(0).Padding(0, theme.PaneInset+4+bar.Right, 0, 0).AlignItems(ui.Center).
		DragWindow().Children(func() {
		ui.Box(c).Width(lead).Shrink(0)
		a.sidebarToggle(c)
		ui.Box(c).Width(start - lead - toggleW).Shrink(0)
		// MyGo's toolbar: one stop of Tab, the arrows move among its
		// controls, and what does not fit goes into its overflow menu.
		ui.Toolbar(c, func() { a.toolbarItems(c) }).Grow(1).Shrink(1).Padding(0).Gap(8).Label(locales.ToolbarLabel())
	})
}

func (a *App) toolbarItems(c *ui.Context) {
	cal := a.section == SectionCalendar
	if cal {
		capsule(c).Children(func() {
			if capsuleIcon(c, icons.Previous, a.stepLabel(-1), false).Clicked() {
				a.step(-1)
			}
			today := ui.ButtonBase(c).Height(theme.CapsuleH-4).Padding(0, 10).Radius((theme.CapsuleH - 4) / 2).
				Label(locales.ToolbarToday()).Tooltip(locales.ToolbarTodayTooltip(todayShortcut))
			if today.Hovered() {
				today.Background(theme.GlassHover)
			}
			if today.Children(func() { ui.Text(c, locales.ToolbarToday()).FontSize(12.5).Bold() }).Clicked() {
				a.goToday()
			}
			if capsuleIcon(c, icons.Next, a.stepLabel(1), false).Clicked() {
				a.step(1)
			}
		})
	}
	// The calendar's period belongs in the top band; the other pages
	// already carry their title in the content sheet, so repeating it here
	// would show it twice.
	if cal {
		ui.Text(c, a.title()).FontSize(theme.TitleSize).Bold().SingleLine().Shrink(1).Margin(0, 0, 0, 4)
		if a.statsInToolbar(c) {
			a.statsLine(c)
		}
	}
	ui.Spacer(c).DragWindow()
	if cal {
		a.viewSwitch(c)
	}
	a.searchBox(c)
	capsule(c).Children(func() {
		f := capsuleIcon(c, icons.Filter, locales.ToolbarFilter(), a.filterOpen || a.filtersActive())
		if f.Clicked() {
			a.filterOpen = !a.filterOpen
		}
		ui.Popover(c, f, &a.filterOpen, func() { a.filterPanel(c) })
		icon, status, _ := a.syncSummary()
		running := a.opt.Scheduler.Running()
		r := capsuleGlyph(c, locales.ToolbarSync(syncShortcut, status), running, func() { a.syncGlyph(c, icon, 17) })
		if r.Clicked() {
			a.Refresh()
		}
		if a.hasInspector() {
			label := locales.ToolbarInspectorShow(inspectorShortcut)
			if a.inspector {
				label = locales.ToolbarInspectorHide(inspectorShortcut)
			}
			capsuleToggle(c, icons.SidebarRight, label, &a.inspector)
		}
	})
}

// modeIcons are the views' icons, in the order of Mode.
var modeIcons = []icons.Name{icons.Month, icons.Week, icons.Day, icons.List}

// viewSwitch is the segmented control of the calendar's views, MyGo's
// SegmentedBase in a capsule of glass: one icon-and-text segment per view,
// the arrows choosing among them while it has the focus.
func (a *App) viewSwitch(c *ui.Context) {
	seg := ui.SegmentedBase(c, &a.mode, len(modeIcons))
	seg.Track.Height(theme.CapsuleH).Padding(2).AlignItems(ui.Center).Radius(theme.CapsuleH / 2).
		Material(glass.Glass{}).Shrink(0).Label(locales.ToolbarView()).Children(func() {
		for i := range modeIcons {
			text := modeLabel(Mode(i))
			s := float32(theme.CapsuleH - 4)
			label := locales.ToolbarViewLabel(text, viewShortcut(i))
			seg := seg.Segment(i).Height(s).Padding(0, 11, 0, 9).Gap(5).Radius(s / 2).AlignItems(ui.Center).
				Label(label).Tooltip(label)
			switch {
			case a.mode == i:
				seg.Background(theme.GlassSelected)
			case seg.Hovered():
				seg.Background(theme.GlassHover)
			}
			seg.Children(func() {
				mygoicons.Image(c, modeIcons[i], 15)
				t := ui.Text(c, text).FontSize(12.5).SingleLine()
				if a.mode == i {
					t.Bold()
				}
			})
		}
	})
}

// sidebarToggle shows or hides the sidebar, a MyGo toggle pressed while
// the sidebar shows; it stays in the top band, next to the traffic
// lights, whether the sidebar shows or not.
func (a *App) sidebarToggle(c *ui.Context) {
	label := locales.SidebarHide(sidebarShortcut)
	if a.sideHide {
		label = locales.SidebarShow(sidebarShortcut)
	}
	shown := !a.sideHide
	b := ui.ToggleBase(c.Key("sidebar-toggle"), &shown).Size(theme.ToggleButton+4, theme.ToggleButton).Shrink(0).
		Radius(theme.ToggleButton / 2).Label(label).Tooltip(label)
	switch {
	case b.Pressed():
		b.Background(theme.GlassPressed)
	case b.Hovered():
		b.Background(theme.GlassHover)
	}
	b.Children(func() { mygoicons.Image(c, icons.SidebarLeft, theme.IconSize) })
	a.sideHide = !shown
}

// searchBox is the toolbar's search field, a capsule of glass with the
// library's search icon and its clear button.
func (a *App) searchBox(c *ui.Context) {
	box := capsule(c).Width(214).Padding(0, 4, 0, 10).Gap(6).Tooltip(locales.SearchTooltip())
	box.Children(func() {
		mygoicons.Image(c, icons.Search, 16)
		in := ui.TextInputBase(c, &a.query).Grow(1).Shrink(1).MinWidth(0).FontSize(12.5).
			Placeholder(locales.SearchPlaceholder()).Label(locales.SearchLabel())
		if in.Shortcut(0, ui.KeyEscape) {
			a.query = ""
		}
		if a.query != "" {
			if capsuleIcon(c, icons.Clear, locales.SearchClear(), false).Size(24, 24).Clicked() {
				a.query = ""
			}
		} else {
			ui.Box(c).Size(24, 24)
		}
	})
}

// capsule is a row of glass in the toolbar.
func capsule(c *ui.Context) ui.Element {
	return ui.Row(c).Height(theme.CapsuleH).Padding(2, 6).Gap(4).AlignItems(ui.Center).
		Radius(theme.CapsuleH / 2).Material(glass.Glass{}).Shrink(0)
}

// capsuleIcon is a round icon button in a capsule. The round background
// stays inset from the capsule's edge, so a selected button does not crowd
// the ones next to it.
func capsuleIcon(c *ui.Context, name icons.Name, label string, active bool) ui.Element {
	return capsuleGlyph(c, label, active, func() { mygoicons.Image(c, name, 17) })
}

// capsuleGlyph is capsuleIcon with a glyph drawn by draw.
func capsuleGlyph(c *ui.Context, label string, active bool, draw func()) ui.Element {
	s := float32(theme.CapsuleH - 8)
	b := ui.ButtonBase(c).Size(s, s).Radius(s / 2).Center().Label(label).Tooltip(label)
	switch {
	case active:
		b.Background(theme.GlassSelected)
	case b.Pressed():
		b.Background(theme.GlassPressed)
	case b.Hovered():
		b.Background(theme.GlassHover)
	}
	return b.Children(draw)
}

// capsuleToggle is a round toggle in a capsule, pressed while *on.
func capsuleToggle(c *ui.Context, name icons.Name, label string, on *bool) ui.Element {
	s := float32(theme.CapsuleH - 8)
	b := ui.ToggleBase(c, on).Size(s, s).Radius(s / 2).Label(label).Tooltip(label)
	switch {
	case *on:
		b.Background(theme.GlassSelected)
	case b.Pressed():
		b.Background(theme.GlassPressed)
	case b.Hovered():
		b.Background(theme.GlassHover)
	}
	return b.Children(func() { mygoicons.Image(c, name, 17) })
}

// filterPanel is the filter's popover, in two columns: the kinds and the
// other switches on the left, the sources on the right in a scroll of
// their own, since there are many.
func (a *App) filterPanel(c *ui.Context) {
	if a.region == "" {
		a.region = allRegions
	}
	if a.city == "" {
		a.city = allCities
	}
	ui.Column(c).Padding(14).Gap(10).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.FilterTitle()).Bold()
			ui.Spacer(c)
			if ui.Button(c, locales.FilterReset()).Disabled(!a.filtersActive()).Clicked() {
				a.resetFilters()
			}
		})
		ui.Row(c).Gap(14).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Width(236).Gap(10).Children(func() {
				caption(c, locales.FilterKinds())
				for i, k := range domain.Kinds {
					c.Key("kind:" + string(k))
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						dot(c, theme.KindColor(k, ""), 7)
						ui.Checkbox(c, &a.kindOn[i], k.Label(lang()))
					})
				}
				ui.Divider(c)
				ui.Checkbox(c, &a.onlyChanged, locales.FilterChanged())
				ui.Checkbox(c, &a.onlyReview, locales.FilterReview())
				caption(c, locales.FilterDateRange())
				ui.Segmented(c, &a.scope, scopeLabels()...).Label(locales.FilterDateRange())
				caption(c, locales.FilterRegion())
				placeSelect(c, &a.region, regionIDs(), regionLabel, locales.FilterRegion(), 236)
				caption(c, locales.FilterCity())
				cities := cityIDs(a.region)
				if !hasString(cities, a.city) {
					a.city = allCities
				}
				placeSelect(c, &a.city, cities, cityLabel, locales.FilterCityLabel(), 236)
			})
			ui.Box(c).Width(1).AlignSelf(ui.Stretch).Background(theme.MistGray)
			ui.Column(c).Width(250).Gap(8).Children(func() {
				sts := a.opt.Scheduler.Statuses()
				on := 0
				for _, st := range sts {
					if off := a.sourceOff[st.Config.ID]; off == nil || !*off {
						on++
					}
				}
				caption(c, locales.FilterSources(on, len(sts)))
				ui.Scroll(c).MaxHeight(390).Label(locales.FilterSourcesLabel()).Children(func() {
					ui.Column(c).Gap(10).Padding(0, 8, 0, 0).Children(func() {
						for _, st := range sts {
							id := st.Config.ID
							off, ok := a.sourceOff[id]
							if !ok {
								off = new(bool)
								a.sourceOff[id] = off
							}
							on := !*off
							c.Key("src:" + id)
							ui.Checkbox(c, &on, st.Config.Name)
							*off = !on
						}
					})
				})
			})
		})
	})
}

func (a *App) stepLabel(dir int) string {
	prev := map[Mode]string{
		ModeMonth:  locales.StepPrevMonth(),
		ModeWeek:   locales.StepPrevWeek(),
		ModeDay:    locales.StepPrevDay(),
		ModeAgenda: locales.StepPrevPage(),
	}[Mode(a.mode)]
	next := map[Mode]string{
		ModeMonth:  locales.StepNextMonth(),
		ModeWeek:   locales.StepNextWeek(),
		ModeDay:    locales.StepNextDay(),
		ModeAgenda: locales.StepNextPage(),
	}[Mode(a.mode)]
	if dir < 0 {
		return prev
	}
	return next
}
