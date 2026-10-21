package views

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
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

// summaryBar is the line above the calendar: a monospaced overview of the
// visible counts, the zone and the legend.
func (a *App) summaryBar(c *ui.Context) {
	first, last := a.visibleRange()
	evs := a.entries(first, last)
	lives := 0
	for _, e := range evs {
		if e.Kind == domain.EntryLive {
			lives++
		}
	}
	ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(10).Shrink(0).AlignItems(ui.Center).Tooltip("当前视图范围内的公开事件统计").Children(func() {
			stat(c, fmt.Sprint(lives), "场 Live")
			divider(c)
			stat(c, fmt.Sprint(len(evs)-lives), "个票务节点")
			divider(c)
			ui.Text(c, "Asia/Tokyo 时间").Font("monospace").FontSize(theme.CaptionSize).
				TextColor(theme.TextSecondary).SingleLine().Shrink(0)
		})
		if a.filtersActive() {
			tag(c, "已筛选", theme.StatusInfo)
		}
		ui.Spacer(c)
		ui.Row(c).Gap(5).Shrink(0).AlignItems(ui.Center).Tooltip("Live 以艺人主题色显示：取自头像中面积最大的色块").Children(func() {
			liveSwatch(c)
			caption(c, "Live（艺人主题色）").SingleLine()
		})
		for _, l := range theme.Legend[1:] {
			ui.Row(c).Gap(5).Shrink(0).AlignItems(ui.Center).Children(func() {
				dot(c, l.Color, 7)
				caption(c, l.Label).SingleLine()
			})
		}
	})
}

// stat is a count in the summary: a monospaced number and its unit.
func stat(c *ui.Context, value, unit string) ui.Element {
	return ui.Row(c).Gap(4).Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, value).Font("monospace").FontSize(13).Bold()
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
				ui.Text(c, "今天").FontSize(10.5).Bold().TextColor(theme.LiveCoral).SingleLine()
			} else if d.Day == 1 {
				caption(c, fmt.Sprintf("%d月", int(d.Month))).SingleLine()
			}
			ui.Spacer(c)
			if lives > 0 {
				ui.Text(c, fmt.Sprintf("%d 场", lives)).FontSize(10.5).TextColor(theme.TextSecondary).SingleLine()
			}
		})
		for _, e := range evs[:n] {
			c.Key(e.ID)
			a.chip(c, e, true)
		}
		if rest := evs[n:]; len(rest) > 0 {
			unit := "场"
			for _, e := range rest {
				if e.Kind != domain.EntryLive {
					unit = "项"
				}
			}
			label := fmt.Sprintf("还有 %d %s", len(rest), unit)
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
	t := "时间未公布"
	if e.At != nil {
		t = e.At.String()
	}
	return fmt.Sprintf("%s %s %s %s", e.Kind.Label(), shortDay(e.Date), t, e.Title)
}

// liveLine is a Live chip's time and venue: "19:00 · 大阪 大阪城ホール".
func liveLine(e domain.Entry) string { return liveTime(e) + " · " + liveVenue(e) }

// liveTime is the time a Live starts, or that it is not out yet.
func liveTime(e domain.Entry) string {
	if e.At != nil {
		return e.At.String()
	}
	return "时间未定"
}

// liveVenue is where a Live is: "大阪 大阪城ホール".
func liveVenue(e domain.Entry) string {
	if v := strings.ReplaceAll(e.Detail, " · ", " "); v != "" {
		return v
	}
	return "场地未公布"
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
	selected := a.entryID == e.ID
	tip := e.Title + " · " + e.Detail
	if e.Changed {
		tip += " · 有更新"
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
		title := e.Kind.Short() + " " + e.Title
		if artist != "" && compact {
			title = e.Kind.Short() + " " + artist
		} else if !compact {
			title = e.Kind.Label() + " · " + e.Title
		}
		t := ui.Text(c, title).FontSize(10.5).SingleLine().Shrink(1)
		if e.Important {
			t.Bold()
		}
		if !compact {
			t.FontSize(theme.CaptionSize)
			if e.Changed {
				tag(c, "更新", theme.StatusInfo)
			}
			if e.NeedsReview {
				tag(c, "需确认", theme.StatusWarning)
			}
		}
	}).Clicked() {
		a.selectEntry(e)
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
	selected := a.entryID == e.ID
	off := e.Status == domain.StatusPostponed || e.Status == domain.StatusCancelled
	tip := name + " · " + e.Title + " · " + liveLine(e)
	if off {
		tip += " · " + e.Status.Label()
	} else if e.Changed {
		tip += " · 有更新"
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
				t := ui.Text(c, name).FontSize(11).Bold().TextColor(pal.Text).SingleLine().Shrink(1)
				if e.Status == domain.StatusCancelled {
					t.Strikethrough()
				}
				if !compact {
					t.FontSize(12.5)
					ui.Spacer(c)
					if off {
						tag(c, e.Status.Label(), theme.StatusError)
					} else if e.Changed {
						tag(c, "更新", theme.StatusInfo)
					}
					if e.NeedsReview {
						tag(c, "需确认", theme.StatusWarning)
					}
				}
			})
			if compact {
				// One line in a month's cell: the time, then the venue.
				ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
					if off {
						ui.Text(c, e.Status.Label()).FontSize(10).Bold().TextColor(theme.StatusError).SingleLine().Shrink(0).Margin(0, 3, 0, 0)
					}
					chipInfo(c, icons.Time, liveTime(e)+" "+liveVenue(e), info, 9).Shrink(1)
				})
				return
			}
			if artist != "" && e.Title != artist {
				ui.Text(c, e.Title).FontSize(11).TextColor(pal.Text).SingleLine()
			}
			chipInfo(c, icons.Venue, liveVenue(e), info, 11)
			chipInfo(c, icons.Time, liveTime(e), info, 11)
		})
	}).Clicked() {
		a.selectEntry(e)
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
				caption(c, "全天 /")
				caption(c, "时间未公布")
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
		t := "未公布"
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
				ui.Text(c, e.Kind.Label()).FontSize(theme.CaptionSize).Bold().TextColor(col.Mix(theme.InkCharcoal, 0.3)).Shrink(0)
				t := ui.Text(c, e.Title).SingleLine().Shrink(1)
				if e.Important {
					t.Bold()
				}
				if e.Status == domain.StatusCancelled {
					t.Strikethrough()
				}
			})
			caption(c, a.entryDetail(e)).SingleLine()
		})
		if e.Status == domain.StatusPostponed || e.Status == domain.StatusCancelled {
			tag(c, e.Status.Label(), theme.StatusError)
		}
		if e.Changed {
			tag(c, "更新", theme.StatusInfo)
		}
		if e.NeedsReview {
			tag(c, "需确认", theme.StatusWarning)
		}
	}).Clicked() {
		a.selectEntry(e)
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
		s = "场地未公布"
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
						label += " · 今天"
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
			emptyState(c, icons.List, "这段时间没有公开事件",
				fmt.Sprintf("%s – %s 之间暂无 Live 或票务节点。可以切换月份，或在「数据源」开启更多数据源。", shortDay(first), shortDay(last)))
		})
		return
	}
	caption(c, fmt.Sprintf("%s – %s，共 %d 项", shortDay(first), shortDay(last), len(evs)))
	a.entryList(c, evs, false)
}

func (a *App) searchView(c *ui.Context) {
	res := a.searchResults()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		mygoicons.Image(c, icons.Search, 16)
		ui.Text(c, fmt.Sprintf("「%s」的搜索结果：%d 项", a.query, len(res))).Bold()
		caption(c, scopeLabels[a.scope]+" · 可搜索艺人、巡演、场馆、城市、类型、来源")
		ui.Spacer(c)
		ui.Segmented(c, &a.scope, scopeLabels...).Label("搜索日期范围")
		if ui.Button(c, "清除搜索").Clicked() {
			a.query = ""
		}
	})
	if len(res) == 0 {
		card(c).Grow(1).Children(func() {
			emptyState(c, icons.Search, "没有找到匹配的事件", "试试艺人别名、场馆名、都道府县（如「東京」），或调整筛选与日期范围。")
		})
		return
	}
	a.entryList(c, res, true)
}
