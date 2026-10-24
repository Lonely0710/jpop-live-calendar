package views

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/locales"
)

func (a *App) calendar(c *ui.Context) {
	ui.Column(c).Grow(1).Shrink(1).Padding(14, theme.ContentPadding, theme.ContentPadding).Gap(10).Children(func() {
		if a.query != "" {
			a.searchView(c)
			return
		}
		a.summaryBar(c)
		switch Mode(a.mode) {
		case ModeMonth:
			a.monthView(c)
		case ModeWeek:
			a.timeGrid(c, a.cursor.WeekStart(theme.WeekStart), 7)
		case ModeDay:
			a.timeGrid(c, a.cursor, 1)
		case ModeAgenda:
			a.agendaView(c)
		}
	})
}

// summaryBar is the line above the calendar: the legend, and, in a window
// too narrow to keep the counts beside the title, the counts too. Items
// that do not fit are left out rather than clipped at the card's edge.
func (a *App) summaryBar(c *ui.Context) {
	avail := a.sheetWidth(c) - 2*theme.ContentPadding
	showStats := !a.statsInToolbar(c)
	used := float32(0)
	if showStats {
		used = a.statsWidth(c) + 14
	}
	measure := func(s string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: s, Size: float32(theme.CaptionSize)})
		return w
	}
	fits := func(w float32) bool {
		return avail <= 0 || used+w+20 <= avail
	}
	ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
		if showStats {
			a.statsLine(c)
		}
		liveW := 15 + 5 + measure(locales.CalendarLiveLegend())
		if !fits(liveW) {
			return
		}
		used += liveW
		ui.Row(c).Gap(5).Shrink(0).AlignItems(ui.Center).Tooltip(locales.CalendarLiveTooltip()).Children(func() {
			liveSwatch(c)
			caption(c, locales.CalendarLiveLegend()).SingleLine()
		})
		labels := []string{
			locales.LegendLottery(), locales.LegendResult(),
			locales.LegendSale(), locales.LegendPostponed(),
		}
		for i, col := range theme.Legend[1:] {
			w := 7 + 5 + measure(labels[i])
			if !fits(w) {
				break
			}
			used += 14 + w
			ui.Row(c).Gap(5).Shrink(0).AlignItems(ui.Center).Children(func() {
				dot(c, col, 7)
				caption(c, labels[i]).SingleLine()
			})
		}
	})
}

// statsLine is the monospaced overview of the visible counts, the zone and
// the filter state.
func (a *App) statsLine(c *ui.Context) ui.Element {
	lives, tickets := a.visibleCounts()
	return ui.Row(c).Gap(8).Shrink(0).AlignItems(ui.Center).Tooltip(locales.CalendarStatsTooltip()).Children(func() {
		stat(c, fmt.Sprint(lives), "Live")
		divider(c)
		stat(c, fmt.Sprint(tickets), locales.CalendarStatTickets())
		divider(c)
		zone(c)
		if a.filtersActive() {
			tag(c, locales.CalendarFiltered(), theme.StatusInfo)
		}
	})
}

func (a *App) visibleCounts() (lives, tickets int) {
	first, last := a.visibleRange()
	evs := a.entries(first, last)
	for _, e := range evs {
		if e.Kind == domain.EntryLive {
			lives++
		}
	}
	return lives, len(evs) - lives
}

func zone(c *ui.Context) ui.Element {
	return ui.Text(c, "Tokyo").Font("monospace").FontSize(theme.CaptionSize).
		TextColor(theme.TextSecondary).SingleLine().Shrink(0)
}

// toolbarStatsChrome is the room the toolbar's other parts take beside the
// title: the navigation, view switch, search, actions, padding and gaps.
// It is measured with a little to spare, so the counts never push the
// toolbar's items into its overflow menu; English stays inside it by
// abbreviating its month ("Oct 2026") and its views ("M / W / D / L").
const toolbarStatsChrome = 700

// statsInToolbar reports whether the counts fit beside the calendar's
// title, left of the view switch.
func (a *App) statsInToolbar(c *ui.Context) bool {
	if a.query != "" {
		return false
	}
	w, _ := c.Size()
	side := a.sideWidth
	if side <= 0 {
		side = theme.SidebarWidth
	}
	side = theme.RailWidth + (side-theme.RailWidth)*a.sideShown
	tw, _ := c.MeasureText(0, ui.Span{Text: a.title(), Size: theme.TitleSize, Weight: 700})
	return w-side-tw-toolbarStatsChrome >= a.statsWidth(c)
}

// statsWidth measures the counts line, with room kept for the filter tag
// so that turning a filter on does not change where the line sits.
func (a *App) statsWidth(c *ui.Context) float32 {
	lives, tickets := a.visibleCounts()
	measure := func(text string, size float32, weight int) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: text, Font: "monospace", Size: size, Weight: weight})
		return w
	}
	statW := func(n int, unit string) float32 {
		return measure(fmt.Sprint(n), 13, 700) + 4 + measure(unit, float32(theme.CaptionSize), 400)
	}
	tagText := measure(locales.CalendarFiltered(), 10, 700) + 8 // tag's padding
	return statW(lives, "Live") + statW(tickets, locales.CalendarStatTickets()) +
		measure("Tokyo", float32(theme.CaptionSize), 400) + 1 + 1 + tagText + 8*5
}

// stat is a count in the summary: a monospaced number and its unit.
func stat(c *ui.Context, value, unit string) ui.Element {
	return ui.Row(c).Gap(4).Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, value).Font("monospace").FontSize(13).Bold().TextColor(theme.StatNumber)
		ui.Text(c, unit).Font("monospace").FontSize(theme.CaptionSize).TextColor(theme.TextSecondary).SingleLine()
	})
}

func divider(c *ui.Context) ui.Element {
	return ui.Box(c).Width(1).Height(12).Shrink(0).Background(theme.MistGray)
}

// liveSwatch is the legend's marker of Live: a tiny Live chip, a tinted
// block with the thin line on its left.
func liveSwatch(c *ui.Context) {
	ui.Row(c).Size(15, 11).Radius(3).Shrink(0).Background(theme.LiveCoral.Alpha(0.2).Over(theme.CloudWhite)).Children(func() {
		ui.Box(c).Size(2.5, 8).Radius(1.25).Margin(1.5, 0, 1.5, 1.5).Background(theme.LiveCoral)
	})
}

func (a *App) visibleRange() (domain.Date, domain.Date) {
	switch Mode(a.mode) {
	case ModeWeek:
		s := a.cursor.WeekStart(theme.WeekStart)
		return s, s.AddDays(6)
	case ModeDay:
		return a.cursor, a.cursor
	case ModeAgenda:
		return a.cursor, a.cursor.AddDays(59)
	}
	g := a.cursor.MonthGrid(theme.WeekStart)
	return g[0], g[41]
}

func byDate(evs []domain.Entry) map[domain.Date][]domain.Entry {
	m := map[domain.Date][]domain.Entry{}
	for _, e := range evs {
		m[e.Date] = append(m[e.Date], e)
	}
	return m
}

// Heights in a month cell, to fit as many chips as the cell has room for.
const (
	cellPad     = 4
	cellHeader  = 20
	liveChipH   = 32
	ticketChipH = 18
	chipGap     = 2
	moreH       = 15
)

func chipHeight(e domain.Entry) float32 {
	if e.Kind == domain.EntryLive {
		return liveChipH
	}
	return ticketChipH
}

// fitChips is how many of evs a cell of height h shows, keeping room for
// the "还有 N 场" line when some do not fit; at least one.
func fitChips(evs []domain.Entry, h float32) int {
	if h <= 0 {
		h = 2*cellPad + cellHeader + 2*(liveChipH+chipGap) + moreH + chipGap // first frame
	}
	avail := h - 2*cellPad - cellHeader - chipGap
	used := float32(0)
	for i, e := range evs {
		ch := chipHeight(e) + chipGap
		need := ch
		if i < len(evs)-1 {
			need += moreH
		}
		if used+need > avail && i > 0 {
			return i
		}
		used += ch
	}
	return len(evs)
}

func (a *App) monthView(c *ui.Context) {
	today := a.today()
	days := a.cursor.MonthGrid(theme.WeekStart)
	entries := byDate(a.entries(days[0], days[41]))
	ui.Grid(c).Columns(7).Children(func() {
		for i, h := range weekdayHeaders() {
			col := theme.TextSecondary
			if i >= 5 { // Saturday and Sunday, as Japanese calendars mark them
				col = theme.LiveCoral.Mix(theme.InkCharcoal, 0.25)
			}
			caption(c, h).Bold().TextAlign(ui.Center).Padding(2, 0, 4).TextColor(col)
		}
	})
	// Hairlines: the grid's background shows through 1-DIP gaps.
	ui.Grid(c).Columns(7).GridRows(6).Gap(1).Grow(1).Shrink(1).Background(theme.MistGray).
		Border(1, theme.MistGray).Radius(theme.CardRadius).Clip().Children(func() {
		for _, d := range days {
			c.Key(d)
			a.dayCell(c, d, d.Month == a.cursor.Month, d == today, entries[d])
		}
	})
}

func (a *App) dayCell(c *ui.Context, d domain.Date, inMonth, isToday bool, evs []domain.Entry) {
	bg := theme.CloudWhite
	switch {
	case isToday:
		bg = theme.TodayTint
	case !inMonth:
		bg = theme.OutsideMonth
	}
	selected := d == a.selected && a.entryID == ""
	b := ui.ButtonBase(c).Column().Justify(ui.Start).AlignItems(ui.Stretch).Padding(cellPad).Gap(chipGap).
		Background(bg).Label(shortDay(d)).Clip()
	switch {
	case selected:
		b.Background(theme.WarmSpotlight.Alpha(0.35).Over(bg))
	case b.Hovered():
		b.Background(theme.Hover.Alpha(0.6).Over(bg))
	}
	if isToday {
		b.Border(1.5, theme.LiveCoral.Alpha(0.55))
	}
	n := fitChips(evs, b.Bounds().H)
	lives := 0
	for _, e := range evs {
		if e.Kind == domain.EntryLive {
			lives++
		}
	}
	clicked := b.Children(func() {
		ui.Row(c).Height(cellHeader).Gap(4).AlignItems(ui.Center).Children(func() {
			dayNumber(c, d.Day, inMonth, isToday)
			if isToday {
				ui.Text(c, locales.CommonToday()).FontSize(10.5).Bold().TextColor(theme.LiveCoral).SingleLine()
			} else if d.Day == 1 {
				caption(c, dateOf(d, "MMM")).SingleLine()
			}
			ui.Spacer(c)
			if lives > 0 {
				ui.Text(c, locales.CalendarLivesCount(lives)).FontSize(10.5).TextColor(theme.TextSecondary).SingleLine()
			}
		})
		for _, e := range evs[:n] {
			c.Key(e.ID)
			a.chip(c, e, true)
		}
		if rest := evs[n:]; len(rest) > 0 {
			allLives := true
			for _, e := range rest {
				if e.Kind != domain.EntryLive {
					allLives = false
				}
			}
			label := locales.CalendarMoreItems(len(rest))
			if allLives {
				label = locales.CalendarMoreShows(len(rest))
			}
			more := ui.ButtonBase(c).Height(moreH).Padding(0, 4).Radius(4).Justify(ui.Start).
				Label(shortDay(d) + " " + label)
			if more.Hovered() {
				more.Background(theme.Hover)
			}
			if more.Children(func() {
				ui.Text(c, label).FontSize(10.5).Bold().TextColor(theme.TextSecondary).SingleLine()
			}).Clicked() {
				a.cursor, a.selected, a.entryID = d, d, ""
				a.setMode(ModeDay)
			}
		}
	}).Clicked()
	if clicked {
		a.selected, a.entryID = d, ""
		a.inspector = true
	}
}

func dayNumber(c *ui.Context, day int, inMonth, isToday bool) {
	if isToday {
		ui.Box(c).Size(20, 20).Radius(10).Background(theme.LiveCoral).Center().Shrink(0).Children(func() {
			ui.Textf(c, "%d", day).FontSize(11.5).Bold().TextColor(theme.CloudWhite)
		})
		return
	}
	col := theme.InkCharcoal
	if !inMonth {
		col = theme.TextDisabled
	}
	ui.Box(c).Size(20, 20).Center().Shrink(0).Children(func() {
		ui.Textf(c, "%d", day).FontSize(11.5).TextColor(col)
	})
}

// chipLabel names an entry for its button: kind, time and title.
func chipLabel(e domain.Entry) string {
	t := locales.CalendarTimeTba()
	if e.At != nil {
		t = e.At.String()
	}
	return fmt.Sprintf("%s %s %s %s", e.Kind.Label(lang()), shortDay(e.Date), t, e.Title)
}

// liveLine is a Live chip's time and venue: "19:00 · 大阪 大阪城ホール".
func liveLine(e domain.Entry) string { return liveTime(e) + " · " + liveVenue(e) }

// liveTime is the time a Live starts, or that it is not out yet.
func liveTime(e domain.Entry) string {
	if e.At != nil {
		return e.At.String()
	}
	return locales.CalendarTimeTba()
}

// liveVenue is where a Live is: "大阪 大阪城ホール".
func liveVenue(e domain.Entry) string {
	if v := strings.ReplaceAll(e.Detail, " · ", " "); v != "" {
		return v
	}
	return locales.CalendarVenueTba()
}

// chip is one entry in a calendar cell. Live shows are cards in their
// artist's theme color: the name in bold, then time and venue. Ticket
// steps are an outlined pill with a dot of their kind's color, so the two
// read apart at a glance. Deadlines are bold; cancelled shows are struck
// through.
func (a *App) chip(c *ui.Context, e domain.Entry, compact bool) {
	if e.Kind == domain.EntryLive {
		a.liveChip(c, e, compact)
		return
	}
	col := theme.KindColor(e.Kind, e.Status)
	ended := a.endedEntry(e)
	selected := a.entryID == e.ID
	tip := e.Title + " · " + e.Detail
	if e.Changed {
		tip += " · " + locales.CalendarUpdated()
	}
	b := ui.ButtonBase(c).FillWidth().Gap(5).Radius(5).Justify(ui.Start).Label(chipLabel(e)).Tooltip(tip)
	if compact {
		b.Height(ticketChipH).Padding(0, 5, 0, 0)
	} else {
		b.MinHeight(24).Padding(3, 7, 3, 0)
	}
	switch {
	case selected:
		b.Background(col.Alpha(0.32).Over(theme.CloudWhite))
	case b.Hovered():
		b.Background(theme.Hover)
	}
	b.Border(1, col.Alpha(0.55))
	if b.Children(func() {
		ui.Box(c).Width(1)
		dot(c, col, 6)
		if e.At != nil {
			ui.Text(c, e.At.String()).FontSize(10).TextColor(theme.TextSecondary).SingleLine().Shrink(0)
		}
		_, artist := a.entryArtist(e)
		title := e.Kind.Short(lang()) + " " + e.Title
		if artist != "" && compact {
			title = e.Kind.Short(lang()) + " " + artist
		} else if !compact {
			title = e.Kind.Label(lang()) + " · " + e.Title
		}
		t := ui.Text(c, title).FontSize(10.5).SingleLine().Shrink(1)
		if e.Important {
			t.Bold()
		}
		if !compact {
			t.FontSize(theme.CaptionSize)
			if e.Changed {
				tag(c, locales.CalendarUpdateTag(), theme.StatusInfo)
			}
			if e.NeedsReview {
				tag(c, locales.CalendarNeedsReview(), theme.StatusWarning)
			}
		}
	}).Clicked() {
		a.selectEntry(e)
	}
	if ended {
		b.Children(func() { endedVeil(c, 5) })
	}
}

// liveChip is a Live show on the calendar, in its artist's colors.
func (a *App) liveChip(c *ui.Context, e domain.Entry, compact bool) {
	pal := a.entryPalette(e)
	_, artist := a.entryArtist(e)
	name := artist
	if name == "" {
		name = e.Title
	}
	ended := a.endedEntry(e)
	selected := a.entryID == e.ID
	off := e.Status == domain.StatusPostponed || e.Status == domain.StatusCancelled
	tip := e.Title + "\n" + liveLine(e)
	if off {
		tip += " · " + e.Status.Label(lang())
	} else if e.Changed {
		tip += " · " + locales.CalendarUpdated()
	}
	b := ui.ButtonBase(c).FillWidth().Gap(5).Radius(6).Justify(ui.Start).AlignItems(ui.Stretch).
		Label(chipLabel(e)).Tooltip(tip).Clip()
	if compact {
		b.Height(liveChipH)
	} else {
		b.MinHeight(56)
	}
	switch {
	case selected:
		b.Background(pal.Strong)
	case b.Hovered():
		b.Background(pal.Hover)
	default:
		b.Background(pal.Tint)
	}
	bar := pal.Accent
	if off {
		bar = theme.StatusError
	}
	info := pal.Text.Mix(theme.TextSecondary, 0.35)
	if b.Children(func() {
		// The thin line of the artist's color, inset from the block's
		// rounded corners, as in Calendar's and Fantastical's events.
		ui.Box(c).Width(3).Radius(1.5).Shrink(0).Margin(3, 0, 3, 3).Background(bar)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Justify(ui.Center).Gap(1).Padding(2, 5, 2, 0).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				t := ui.Text(c, name).Lang("ja").FontSize(11).Bold().TextColor(pal.Text).SingleLine().Shrink(1)
				if e.Status == domain.StatusCancelled {
					t.Strikethrough()
				}
				if !compact {
					t.FontSize(12.5)
					ui.Spacer(c)
					if off {
						tag(c, e.Status.Label(lang()), theme.StatusError)
					} else if e.Changed {
						tag(c, locales.CalendarUpdateTag(), theme.StatusInfo)
					}
					if e.NeedsReview {
						tag(c, locales.CalendarNeedsReview(), theme.StatusWarning)
					}
				}
			})
			if compact {
				// One line in a month's cell: the time, then the venue.
				ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
					if off {
						ui.Text(c, e.Status.Label(lang())).FontSize(10).Bold().TextColor(theme.StatusError).SingleLine().Shrink(0).Margin(0, 3, 0, 0)
					}
					chipInfo(c, icons.Time, liveTime(e)+" "+liveVenue(e), info, 9).Shrink(1)
				})
				return
			}
			if artist != "" && e.Title != artist {
				ui.Text(c, e.Title).Lang("ja").FontSize(11).TextColor(pal.Text).SingleLine()
			}
			chipInfo(c, icons.Venue, liveVenue(e), info, 11)
			chipInfo(c, icons.Time, liveTime(e), info, 11)
		})
	}).Clicked() {
		a.selectEntry(e)
	}
	if ended {
		b.Children(func() { endedVeil(c, 6) })
	}
}

// chipInfo is a line of a Live chip: a small icon in the chip's color,
// then the text.
func chipInfo(c *ui.Context, icon icons.Name, text string, col ui.Color, size float32) ui.Element {
	return ui.Row(c).Gap(3).AlignItems(ui.Center).MinWidth(0).Children(func() {
		mygoicons.Mono(c, icon, size).TextColor(col).Shrink(0)
		ui.Text(c, text).FontSize(size + 1).TextColor(col).SingleLine().Shrink(1)
	})
}

// tag is a tiny colored label.
func tag(c *ui.Context, s string, col ui.Color) {
	ui.Box(c).Padding(0, 4).Radius(4).Background(col.Alpha(0.16)).Shrink(0).Children(func() {
		ui.Text(c, s).FontSize(10).Bold().TextColor(col.Mix(theme.InkCharcoal, 0.25)).SingleLine()
	})
}

// timeGrid is the week (n = 7) or day (n = 1) view: a header of days, the
// row of entries whose time is not announced, then 24 hours.
func (a *App) timeGrid(c *ui.Context, first domain.Date, n int) {
	today := a.today()
	entries := byDate(a.entries(first, first.AddDays(n-1)))
	tracks := []ui.Track{ui.Fixed(theme.TimeGutter)}
	for range n {
		tracks = append(tracks, ui.Fr(1))
	}
	// The scroll view shows about half a day: scroll so the evening (or
	// the last timed entry) is in view without hiding the first one.
	firstH, lastH := 24, 21
	for _, evs := range entries {
		for _, e := range evs {
			if e.At != nil {
				firstH, lastH = min(firstH, e.At.Hour), max(lastH, e.At.Hour)
			}
		}
	}
	scrollTo := min(lastH, firstH+12, 23)
	card(c).Grow(1).Shrink(1).Clip().Children(func() {
		ui.Grid(c).ColumnTracks(tracks...).BorderWidth(0, 0, 1, 0).BorderColor(theme.MistGray).Children(func() {
			ui.Box(c)
			for i := range n {
				d := first.AddDays(i)
				c.Key(d)
				b := ui.ButtonBase(c).Column().Padding(6, 6).AlignItems(ui.Center).Gap(2).Label(dayTitle(d))
				if d == a.selected && a.entryID == "" {
					b.Background(theme.WarmSpotlight.Alpha(0.35))
				} else if b.Hovered() {
					b.Background(theme.Hover)
				}
				if b.Children(func() {
					caption(c, weekdayName(d.Weekday()))
					dayNumber(c, d.Day, true, d == today)
				}).Clicked() {
					a.selected, a.entryID, a.inspector = d, "", true
				}
			}
		})
		ui.Grid(c).ColumnTracks(tracks...).BorderWidth(0, 0, 1, 0).BorderColor(theme.MistGray).Children(func() {
			ui.Column(c).Padding(8, 8, 8, 0).AlignItems(ui.End).Children(func() {
				caption(c, locales.CalendarAllDay())
				caption(c, locales.CalendarTimeTba())
			})
			for i := range n {
				d := first.AddDays(i)
				c.Key(d)
				ui.Column(c).MinHeight(44).Padding(4).Gap(3).BorderWidth(0, 0, 0, 1).BorderColor(theme.MistGray).Children(func() {
					for _, e := range entries[d] {
						if e.At == nil {
							c.Key(e.ID)
							a.chip(c, e, n > 1)
						}
					}
				})
			}
		})
		ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
			ui.Grid(c).ColumnTracks(tracks...).Children(func() {
				for h := range 24 {
					c.Key(h)
					l := caption(c, fmt.Sprintf("%02d:00", h)).TextAlign(ui.End).Padding(4, 8, 0, 0).Height(theme.HourHeight)
					if h == scrollTo && a.scrollHours {
						l.ScrollIntoView()
					}
					for i := range n {
						d := first.AddDays(i)
						c.Key(hourKey{h, d})
						ui.Column(c).MinHeight(theme.HourHeight).Padding(3).Gap(2).
							BorderWidth(0, 0, 1, 1).BorderColor(theme.MistGray.Alpha(0.7)).Children(func() {
							for _, e := range entries[d] {
								if e.At != nil && e.At.Hour == h || (e.At != nil && h == 23 && e.At.Hour > 23) {
									c.Key(e.ID)
									a.chip(c, e, n > 1)
								}
							}
						})
					}
				}
			})
		})
	})
	a.scrollHours = false
}

type hourKey struct {
	h int
	d domain.Date
}

// entryRow is an entry in a list: time, kind, title, detail.
func (a *App) entryRow(c *ui.Context, e domain.Entry) {
	col := theme.KindColor(e.Kind, e.Status)
	ended := a.endedEntry(e)
	b := ui.ButtonBase(c).FillWidth().Padding(10, 12).Gap(12).Radius(theme.Radius).Justify(ui.Start).
		Label(chipLabel(e)).Border(1, theme.MistGray)
	switch {
	case a.entryID == e.ID:
		b.Background(theme.SelectedBg)
	case b.Hovered():
		b.Background(theme.Hover.Alpha(0.6).Over(theme.CloudWhite))
	default:
		b.Background(theme.CloudWhite)
	}
	if b.Children(func() {
		t := locales.CommonTba()
		if e.At != nil {
			t = e.At.String()
		}
		ui.Text(c, t).Width(52).FontSize(12.5).TextColor(theme.TextSecondary).Shrink(0)
		if e.Kind == domain.EntryLive {
			col = a.entryPalette(e).Accent
			ui.Box(c).Width(4).Height(30).Radius(2).Background(col)
		} else {
			ui.Box(c).Size(4, 30).Center().Children(func() { dot(c, col, 7) })
		}
		if id, name := a.entryArtist(e); id != "" {
			a.avatarView(c, id, name, 30)
		}
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, e.Kind.Label(lang())).FontSize(theme.CaptionSize).Bold().TextColor(col.Mix(theme.InkCharcoal, 0.3)).Shrink(0)
				t := ui.Text(c, e.Title).Lang("ja").SingleLine().Shrink(1)
				if e.Important {
					t.Bold()
				}
				if e.Status == domain.StatusCancelled {
					t.Strikethrough()
				}
			})
			caption(c, a.entryDetail(e)).Lang("ja").SingleLine()
		})
		if e.Status == domain.StatusPostponed || e.Status == domain.StatusCancelled {
			tag(c, e.Status.Label(lang()), theme.StatusError)
		}
		if e.Changed {
			tag(c, locales.CalendarUpdateTag(), theme.StatusInfo)
		}
		if e.NeedsReview {
			tag(c, locales.CalendarNeedsReview(), theme.StatusWarning)
		}
	}).Clicked() {
		a.selectEntry(e)
	}
	if ended {
		b.Children(func() { endedVeil(c, theme.Radius) })
	}
}

// entryDetail is the second line of a list row: artists and venue, or the
// round name.
func (a *App) entryDetail(e domain.Entry) string {
	s := a.artistNames(e.ArtistIDs)
	if e.Detail != "" {
		if s != "" {
			s += " · "
		}
		s += e.Detail
	}
	if s == "" {
		s = locales.CalendarVenueTba()
	}
	return s
}

func (a *App) entryList(c *ui.Context, evs []domain.Entry, showYear bool) {
	ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
		ui.Column(c).Gap(6).Padding(0, 2, 8, 0).Children(func() {
			var cur domain.Date
			for _, e := range evs {
				if e.Date != cur {
					cur = e.Date
					c.Key(cur)
					label := shortDay(cur) + " " + weekdayName(cur.Weekday())
					if showYear || cur.Year != a.today().Year {
						label = dayTitle(cur)
					}
					if cur == a.today() {
						label += locales.CalendarTodaySuffix()
					}
					ui.Text(c, label).Bold().Padding(10, 0, 2)
				}
				c.Key(e.ID)
				a.entryRow(c, e)
			}
		})
	})
}

func (a *App) agendaView(c *ui.Context) {
	first, last := a.visibleRange()
	evs := a.entries(first, last)
	if len(evs) == 0 {
		card(c).Grow(1).Children(func() {
			emptyState(c, icons.List, locales.CalendarEmptyTitle(),
				locales.CalendarEmptyBody(shortDay(first), shortDay(last)))
		})
		return
	}
	caption(c, locales.CalendarRangeCount(shortDay(first), shortDay(last), len(evs)))
	a.entryList(c, evs, false)
}

func (a *App) searchView(c *ui.Context) {
	res := a.searchResults()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		mygoicons.Image(c, icons.Search, 16)
		ui.Text(c, locales.CalendarSearchResults(a.query, len(res))).Bold()
		caption(c, scopeLabel(a.scope)+locales.CalendarSearchHint())
		ui.Spacer(c)
		ui.Segmented(c, &a.scope, scopeLabels()...).Label(locales.FilterDateRange())
		if ui.Button(c, locales.SearchClear()).Clicked() {
			a.query = ""
		}
	})
	if len(res) == 0 {
		card(c).Grow(1).Children(func() {
			emptyState(c, icons.Search, locales.CalendarNoResults(), locales.CalendarNoResultsBody())
		})
		return
	}
	a.entryList(c, res, true)
}
