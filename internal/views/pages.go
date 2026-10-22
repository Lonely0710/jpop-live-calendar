package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/avatar"
	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/crawler"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/normalize"
	"jpop-live-calendar/internal/theme"
)

func page(c *ui.Context, fn func()) {
	ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(22, theme.ContentPadding+4).Gap(16).Children(fn)
	})
}

// Artists.

// artistSummaries lists the artists with the pinned ones first, keeping
// the catalog's order among each group.
func (a *App) artistSummaries() []catalog.ArtistSummary {
	sums := a.snap.ArtistSummaries(a.today())
	sort.SliceStable(sums, func(i, j int) bool {
		return a.pins[sums[i].Artist.ID] && !a.pins[sums[j].Artist.ID]
	})
	return sums
}

// togglePin puts an artist at the top of the list or back with the others,
// and saves the choice locally.
func (a *App) togglePin(c *ui.Context, id string) {
	if a.pins[id] {
		delete(a.pins, id)
	} else {
		a.pins[id] = true
	}
	if a.opt.SavePins == nil {
		return
	}
	ids := make([]string, 0, len(a.pins))
	for p := range a.pins {
		ids = append(ids, p)
	}
	sort.Strings(ids)
	if err := a.opt.SavePins(ids); err != nil {
		c.Toast("置顶状态未能保存：" + err.Error())
	}
}

// pinButton is the pin at the right of an artist's row: outlined when the
// artist is among the others, coral when pinned.
func (a *App) pinButton(c *ui.Context, id, name string) ui.Element {
	on := a.pins[id]
	label, icon := "置顶 "+name, icons.Pin
	if on {
		label, icon = "取消置顶 "+name, icons.PinActive
	}
	b := ui.ButtonBase(c).Size(26, 26).Shrink(0).Center().Radius(13).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(theme.Hover)
	}
	if b.Children(func() { mygoicons.Image(c, icon, 16) }).Clicked() {
		a.togglePin(c, id)
	}
	return b
}

func (a *App) artistsPage(c *ui.Context) {
	sums := a.artistSummaries()
	// The page itself does not scroll: the list and the details have their
	// own scroll views, so a wheel or trackpad over the list moves only the
	// list instead of the whole page.
	ui.Column(c).Grow(1).Shrink(1).FillWidth().Padding(22, 0, 22, theme.ContentPadding+4).Gap(16).Children(func() {
		pageHeader(c, "艺人")
		if len(sums) == 0 {
			card(c).Height(320).Margin(0, theme.ContentPadding+4, 0, 0).Children(func() {
				emptyState(c, icons.Artist, "还没有艺人", "同步数据源后，这里会按艺人与巡演整理 Live 信息。当前版本不提供关注、收藏等用户功能。")
			})
			return
		}
		var sel *catalog.ArtistSummary
		for i := range sums {
			if sums[i].Artist.ID == a.artistID {
				sel = &sums[i]
			}
		}
		if sel == nil {
			sel = &sums[0]
		}
		ui.Row(c).Grow(1).Shrink(1).MinHeight(0).Gap(16).AlignItems(ui.Stretch).Children(func() {
			ui.Scroll(c).Width(250).Shrink(0).ScrollbarInsets(12, 6, 12).Label("艺人列表").Children(func() {
				card(c).FillWidth().Padding(6).Gap(2).Children(func() {
					for _, s := range sums {
						c.Key(s.Artist.ID)
						row := ui.Row(c).FillWidth().Padding(4, 6).Gap(4).Radius(theme.Radius).AlignItems(ui.Center)
						if s.Artist.ID == sel.Artist.ID {
							row.Background(theme.SelectedBg)
						} else if row.Hovered() {
							row.Background(theme.Hover)
						}
						row.Children(func() {
							b := ui.ButtonBase(c).Row().Grow(1).Shrink(1).AlignItems(ui.Center).Gap(10).Padding(4, 4).
								Radius(theme.Radius).Justify(ui.Start).Label(s.Artist.Name)
							if b.Children(func() {
								a.avatarView(c, s.Artist.ID, s.Artist.Name, 34)
								ui.Column(c).Shrink(1).Gap(2).Children(func() {
									ui.Text(c, s.Artist.Name).Bold().SingleLine()
									next := "暂无即将到来的公演"
									if s.Next != nil {
										next = "下一场 " + shortDay(s.Next.StartDate)
									}
									line := fmt.Sprintf("%d 场 · %s", s.Shows, next)
									if s.Shows == 0 && len(s.Tours) == 0 {
										line = "关注中 · 尚未找到公开的公演"
									}
									caption(c, line).SingleLine()
								})
							}).Clicked() {
								a.artistID = s.Artist.ID
								a.tourPickOpen = false
							}
							a.pinButton(c, s.Artist.ID, s.Artist.Name)
						})
					}
				})
			})
			ui.Scroll(c).Grow(1).Shrink(1).MinWidth(0).ScrollbarInsets(12, 0, 12).Label("艺人详情").Children(func() {
				ui.Column(c).FillWidth().Padding(0, theme.ContentPadding+4, 0, 0).Gap(12).Children(func() { a.artistDetail(c, *sel) })
			})
		})
	})
}

func (a *App) artistDetail(c *ui.Context, s catalog.ArtistSummary) {
	pal := theme.Artist(a.artistColor(s.Artist.ID, s.Artist.Name))
	active := make([]catalog.Tour, 0, len(s.Tours))
	ended := 0
	for _, t := range s.Tours {
		if a.endedTour(t) {
			ended++
			continue
		}
		active = append(active, t)
	}
	tours, shows := s.Tours, s.Shows
	if a.hideEnded {
		tours = active
		shows = 0
		for _, t := range tours {
			shows += len(t.Events)
		}
		if pick := a.tourPick[s.Artist.ID]; pick != "" && !hasTour(tours, pick) {
			delete(a.tourPick, s.Artist.ID)
		}
	}
	ui.Row(c).Gap(14).AlignItems(ui.Center).Padding(12, 14).Radius(theme.CardRadius).Background(pal.Tint).Children(func() {
		a.avatarView(c, s.Artist.ID, s.Artist.Name, 64)
		ui.Column(c).Gap(4).Shrink(1).Children(func() {
			ui.Text(c, s.Artist.Name).FontSize(18).Bold().TextColor(pal.Text)
			if len(s.Artist.Aliases) > 0 {
				caption(c, "alias: "+strings.Join(s.Artist.Aliases, "、"))
			}
			caption(c, fmt.Sprintf("%d 场公演 · %d 个未截止的票务受付", s.Shows, s.Pending))
		})
		if len(s.Tours) > 0 {
			ui.Spacer(c)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if ended > 0 {
					a.endedToggle(c, pal)
				}
				if len(s.Tours) > 1 {
					a.tourFilter(c, s, tours, shows)
				}
			})
		}
	})
	if len(s.Tours) == 0 {
		caption(c, "已启用的数据源里暂时没有这位艺人的公开公演或票务信息；各数据源的覆盖情况见「数据源」。")
	}
	if pick := a.tourPick[s.Artist.ID]; pick != "" {
		var only []catalog.Tour
		for _, t := range tours {
			if tourKey(t.Name) == pick {
				only = append(only, t)
			}
		}
		if len(only) > 0 {
			tours = only
		} else {
			delete(a.tourPick, s.Artist.ID)
		}
	}
	for _, t := range tours {
		c.Key("tour:" + t.Name)
		name := tourLabel(t.Name)
		card(c).Children(func() {
			p := ui.CollapsibleBase(c, a.tourShown(s.Artist.ID, t.Name))
			tr := p.Trigger.FillWidth().AlignItems(ui.Center).Gap(8).Padding(12, 16).Radius(theme.CardRadius).
				Label("展开或折叠 " + name)
			if tr.Hovered() {
				tr.Background(theme.Hover.Alpha(0.5))
			}
			tr.Children(func() {
				mygoicons.Image(c, icons.Live, 16)
				ui.Text(c, name).Bold().Shrink(1)
				ui.Spacer(c)
				caption(c, fmt.Sprintf("%d 场", len(t.Events)))
				mygoicons.Image(c, icons.Next, 12).Opacity(0.7).Rotate(90 * p.Progress())
			})
			if panel := p.Panel(func() {
				a.tourEvents(c, t)
				if len(t.Rounds) > 0 {
					caption(c, "票务")
					for _, r := range t.Rounds {
						c.Key(r.ID)
						row := ui.Row(c).Gap(8).AlignItems(ui.Center)
						row.Children(func() {
							dot(c, theme.StatusWarning, 6)
							ui.Text(c, r.Name).FontSize(12.5).Shrink(1)
							ui.Spacer(c)
							if r.ClosesAt == nil && r.OpensAt != nil {
								caption(c, "开始 "+momentLabel(r.OpensAt))
							} else {
								caption(c, "截止 "+momentLabel(r.ClosesAt))
							}
						})
						if a.endedRound(r) {
							row.Children(func() { endedVeil(c, 4) })
						}
					}
				}
			}); panel.Valid() {
				panel.Gap(8).Padding(0, 16, 14)
			}
		})
	}
	switch {
	case len(s.Tours) == 0:
	case len(tours) == 0:
		caption(c, fmt.Sprintf("已隐藏 %d 个已结束的巡演；点右上角的眼睛可以重新显示。", ended))
	default:
		caption(c, "点击场次可在日历中查看详情。")
	}
}

// endedToggle is the artist header's eye: it hides the tours whose shows
// have all ended, switching to the crossed-out view icon while on.
func (a *App) endedToggle(c *ui.Context, pal theme.ArtistPalette) {
	label := "隐藏已结束的巡演"
	if a.hideEnded {
		label = "显示已结束的巡演"
	}
	b := ui.ButtonBase(c).Size(28, 28).Shrink(0).Center().Radius(14).Label(label).Tooltip(label)
	switch {
	case a.hideEnded:
		b.Background(pal.Strong)
	case b.Hovered():
		b.Background(theme.CloudWhite)
	default:
		b.Background(theme.CloudWhite.Alpha(0.7))
	}
	if b.Children(func() {
		icon := icons.View
		if a.hideEnded {
			icon = icons.ViewDisabled
		}
		mygoicons.Image(c, icon, 15)
	}).Clicked() {
		a.hideEnded = !a.hideEnded
		a.tourPickOpen = false
	}
}

// endedTour reports whether every show of a tour is over. A tour without
// shows is not treated as over, since there is nothing on the calendar to
// hide.
func (a *App) endedTour(t catalog.Tour) bool {
	if len(t.Events) == 0 {
		return false
	}
	for _, ev := range t.Events {
		if !a.endedEvent(ev) {
			return false
		}
	}
	return true
}

func hasTour(tours []catalog.Tour, key string) bool {
	for _, t := range tours {
		if tourKey(t.Name) == key {
			return true
		}
	}
	return false
}

// tourFilter is the artist header's tour directory: a popover of the
// artist's tours, one of which filters the cards below; 全部巡演 shows all.
func (a *App) tourFilter(c *ui.Context, s catalog.ArtistSummary, tours []catalog.Tour, shows int) {
	label := "全部巡演"
	if pick := a.tourPick[s.Artist.ID]; pick != "" {
		for _, t := range tours {
			if tourKey(t.Name) == pick {
				label = tourLabel(t.Name)
				break
			}
		}
	}
	turn := float32(0)
	if a.tourPickOpen {
		turn = 90
	}
	b := ui.ButtonBase(c).Row().Height(28).Padding(0, 10).Gap(6).Radius(14).AlignItems(ui.Center).
		MaxWidth(240).Label("巡演主题：" + label).Tooltip("按巡演主题筛选")
	b.Background(theme.CloudWhite.Alpha(0.7))
	if b.Hovered() || a.tourPickOpen {
		b.Background(theme.CloudWhite)
	}
	if b.Children(func() {
		mygoicons.Image(c, icons.Filter, 13)
		ui.Text(c, label).FontSize(12.5).Shrink(1).SingleLine()
		mygoicons.Image(c, icons.Next, 11).Opacity(0.75).Rotate(turn)
	}).Clicked() {
		a.tourPickOpen = !a.tourPickOpen
	}
	if p := ui.Popover(c, b, &a.tourPickOpen, func() { a.tourChoices(c, s, tours, shows) }); p.Valid() {
		p.AttachTo(b, ui.AnchorBottomEnd, ui.AnchorTopEnd)
	}
}

func (a *App) tourChoices(c *ui.Context, s catalog.ArtistSummary, tours []catalog.Tour, shows int) {
	ui.Column(c).Width(300).Padding(6).Gap(1).Children(func() {
		ui.Text(c, "巡演主题").FontSize(11.5).TextColor(theme.TextSecondary).Padding(4, 8, 6, 8)
		a.tourChoice(c, s, "", "全部巡演", shows)
		for _, t := range tours {
			a.tourChoice(c, s, tourKey(t.Name), tourLabel(t.Name), len(t.Events))
		}
	})
}

func (a *App) tourChoice(c *ui.Context, s catalog.ArtistSummary, key, label string, shows int) {
	on := a.tourPick[s.Artist.ID] == key
	row := ui.ButtonBase(c).FillWidth().Row().Padding(6, 8).Gap(8).Radius(6).AlignItems(ui.Center).
		Justify(ui.Start).Label("只看" + label)
	switch {
	case on:
		row.Background(theme.SelectedBg)
	case row.Hovered():
		row.Background(theme.Hover)
	}
	if row.Children(func() {
		t := ui.Text(c, label).FontSize(12.5).Shrink(1).SingleLine()
		if on {
			t.Bold()
		}
		ui.Spacer(c)
		caption(c, fmt.Sprintf("%d 场", shows))
	}).Clicked() {
		a.tourPick[s.Artist.ID] = key
		a.tourPickOpen = false
		if key != "" {
			for _, t := range s.Tours {
				if tourKey(t.Name) == key {
					*a.tourShown(s.Artist.ID, t.Name) = true
				}
			}
		}
	}
}

// tourKey identifies a tour in the artist page's filter; the unnamed
// "single shows" tour is valid, so 全部巡演 is the empty key.
func tourKey(name string) string { return "\x00" + name }

func tourLabel(name string) string {
	if name == "" {
		return "单场公演 / 活动出演"
	}
	return name
}

// tourShown is a tour card's open state on the artist page: a tour is open
// until the artist folds it.
func (a *App) tourShown(artistID, tour string) *bool {
	key := artistID + "\x00" + tour
	open := a.tourOpen[key]
	if open == nil {
		open = new(bool)
		*open = true
		a.tourOpen[key] = open
	}
	return open
}

// tourEvents lists a tour's shows, adding a month heading when the tour
// spans more than one month: a long tour or a "single shows" block is then
// easy to scan by month.
func (a *App) tourEvents(c *ui.Context, t catalog.Tour) {
	events := t.Events
	if t.Name == "" {
		// The "single shows" block mixes years of festivals. Show what is
		// still ahead first, then the past most recent first, instead of
		// starting with 2019.
		today := a.today()
		events = append([]domain.PublicEvent(nil), t.Events...)
		sort.SliceStable(events, func(i, j int) bool {
			ui := !events[i].StartDate.Before(today)
			uj := !events[j].StartDate.Before(today)
			if ui != uj {
				return ui
			}
			if ui {
				return events[i].StartDate.Before(events[j].StartDate)
			}
			return events[j].StartDate.Before(events[i].StartDate)
		})
	}
	months := map[int]int{}
	for _, ev := range events {
		months[monthKey(ev.StartDate)]++
	}
	last := 0
	for _, ev := range events {
		key := monthKey(ev.StartDate)
		if len(months) > 1 && key != last {
			last = key
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 0, 1).Children(func() {
				ui.Text(c, monthTitle(ev.StartDate)).FontSize(11.5).Bold().TextColor(theme.TextSecondary)
				caption(c, fmt.Sprintf("%d 场", months[key]))
			})
		}
		c.Key(ev.ID)
		a.showRow(c, ev, t.Name == "")
	}
}

func monthKey(d domain.Date) int { return d.Year*100 + int(d.Month) }

// Sources.

func kindLabel(k domain.SourceKind) string {
	switch k {
	case domain.SourceDemo:
		return "演示"
	case domain.SourceRSS:
		return "RSS / Atom"
	case domain.SourceOfficialSite:
		return "官网页面"
	case domain.SourceAPI:
		return "官网数据接口（JSON）"
	}
	return string(k)
}

// layerName is the tag beside a source's title: what the source watches.
func layerName(src domain.SourceConfig) string {
	switch src.Layer {
	case domain.LayerLive:
		return "日程层"
	case domain.LayerNews:
		return "公告/票务层"
	}
	return ""
}

// everyLabel is the compact interval under a source's sync button.
func everyLabel(src domain.SourceConfig) string {
	if src.Interval <= 0 {
		return "手动"
	}
	return "每 " + intervalShort(src.Interval)
}

func intervalShort(d time.Duration) string {
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return d.String()
}

func shortDuration(d time.Duration) string {
	if d%time.Hour == 0 {
		return fmt.Sprintf("%d 小时", int(d/time.Hour))
	}
	return fmt.Sprintf("%d 分钟", int(d/time.Minute))
}

func stateLabel(st crawler.SourceStatus) (string, ui.Color, icons.Name) {
	switch {
	case !st.Config.Enabled:
		return "已关闭", theme.TextSecondary, icons.Suspended
	case st.State == domain.SyncRunning:
		return "同步中…", theme.StatusInfo, icons.Sync
	case st.State == domain.SyncFailed && !st.Next.IsZero():
		return "失败 · 稍后重试", theme.StatusError, icons.SyncFailed
	case st.State == domain.SyncFailed:
		return "失败", theme.StatusError, icons.SyncFailed
	case st.State == domain.SyncSucceeded && st.LastRun.NotModified:
		return "成功 · 无变化", theme.StatusSuccess, icons.Synced
	case st.State == domain.SyncSucceeded:
		return "成功", theme.StatusSuccess, icons.Synced
	}
	return "等待同步", theme.TextSecondary, icons.Sync
}

func (a *App) sourcesPage(c *ui.Context) {
	sched := a.opt.Scheduler
	sts := sched.Statuses()
	page(c, func() {
		pageHeader(c, "数据源")
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "立即同步全部").Disabled(sched.Running()).Clicked() {
				a.Refresh()
			}
			if ui.Button(c, "新增数据源").Clicked() {
				a.sourceForm.open = !a.sourceForm.open
				a.sourceForm.err = ""
			}
			caption(c, "配置文件："+a.opt.Config.Path)
		})
		if a.sourceForm.open {
			a.sourceFormView(c)
		}
		for _, g := range a.sourceGroups(sts) {
			c.Key("sources:" + g.key)
			a.sourceGroupCard(c, g)
		}
		a.syncLog(c)
	})
}

// sourceGroup is one artist's sources, or the sources without an artist.
type sourceGroup struct {
	key        string
	artistID   string
	artistName string
	statuses   []crawler.SourceStatus
}

// sourceGroups groups the sources by artist in configuration order. A
// source naming an artist the configuration does not list still gets its
// own block, under the written name.
func (a *App) sourceGroups(sts []crawler.SourceStatus) []sourceGroup {
	var out []sourceGroup
	index := map[string]int{}
	for _, st := range sts {
		key, id, name := "other", "", "其他来源"
		if foundID, foundName, ok := a.sourceArtist(st.Config); ok {
			key, id, name = "artist:"+foundID, foundID, foundName
		} else if st.Config.Artist != "" {
			key = "name:" + normalize.Key(st.Config.Artist)
			id = avatar.ArtistID(domain.Artist{Name: st.Config.Artist})
			name = st.Config.Artist
		}
		at, ok := index[key]
		if !ok {
			at = len(out)
			index[key] = at
			out = append(out, sourceGroup{key: key, artistID: id, artistName: name})
		}
		out[at].statuses = append(out[at].statuses, st)
	}
	return out
}

// sourceGroupCard is one artist's block: the artist, a summary line, then
// each of their sources as a nested row.
func (a *App) sourceGroupCard(c *ui.Context, g sourceGroup) {
	ok, failed, off := groupCounts(g.statuses)
	summary := fmt.Sprintf("%d 个数据源", len(g.statuses))
	if ok > 0 {
		summary += fmt.Sprintf(" · %d 正常", ok)
	}
	if failed > 0 {
		summary += fmt.Sprintf(" · %d 失败", failed)
	}
	if off > 0 {
		summary += fmt.Sprintf(" · %d 已关闭", off)
	}
	card(c).Padding(12, 14).Gap(8).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if g.artistID != "" {
				a.avatarView(c, g.artistID, g.artistName, 34)
			} else {
				ui.Box(c).Size(34, 34).Radius(17).Background(theme.PaperIvory).Center().Children(func() {
					mygoicons.Image(c, icons.DataStatus, 18)
				})
			}
			ui.Column(c).Gap(1).Shrink(1).Children(func() {
				ui.Text(c, g.artistName).FontSize(15).Bold().SingleLine()
				caption(c, summary).SingleLine()
			})
			ui.Spacer(c)
			if failed > 0 {
				ui.Badge(c, fmt.Sprint(failed)).Background(theme.StatusError).TextColor(theme.CloudWhite).
					Label(fmt.Sprintf("%d 个来源失败", failed))
			}
		})
		for _, st := range g.statuses {
			a.sourceRow(c, st)
		}
	})
}

func groupCounts(sts []crawler.SourceStatus) (ok, failed, off int) {
	for _, st := range sts {
		switch {
		case !st.Config.Enabled:
			off++
		case st.State == domain.SyncFailed:
			failed++
		default:
			ok++
		}
	}
	return ok, failed, off
}

// sourceRow is one source inside its artist's block.
func (a *App) sourceRow(c *ui.Context, st crawler.SourceStatus) {
	src := st.Config
	sched := a.opt.Scheduler
	c.Key(src.ID)
	ui.Column(c).FillWidth().Padding(9, 11).Gap(6).Radius(theme.Radius).Background(theme.PaperIvory).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			mygoicons.Image(c, sourceKindIcon(src.Kind), 16).Tooltip(kindLabel(src.Kind))
			ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(1).Children(func() {
				ui.Text(c, src.Name).FontSize(13).Bold().SingleLine()
				if name := layerName(src); name != "" {
					ui.Box(c).Padding(1, 5).Radius(5).Background(theme.CloudWhite).Children(func() {
						ui.Text(c, name).FontSize(10).TextColor(theme.TextSecondary).SingleLine()
					})
				}
			})
			ui.Spacer(c)
			label, col, sicon := stateLabel(st)
			ui.Row(c).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
				mygoicons.Image(c, sicon, 13)
				ui.Text(c, label).FontSize(12).Bold().TextColor(col).SingleLine()
			})
			on, ok := a.srcOn[src.ID]
			if !ok {
				on = new(bool)
				a.srcOn[src.ID] = on
			}
			*on = src.Enabled
			ui.Switch(c, on).Shrink(0).Label("启用 " + src.Name)
			if *on != src.Enabled {
				sched.SetEnabled(src.ID, *on)
				if *on {
					a.Refresh(src.ID)
				}
			}
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			if st.LastRun.FinishedAt.IsZero() {
				caption(c, "尚未同步").Font("monospace")
			} else {
				caption(c, "上次同步 "+stamp(st.LastRun.FinishedAt)).Font("monospace")
			}
			add, upd := st.LastRun.New, st.LastRun.Updated
			addColor, updColor := theme.StatusSuccess, theme.StatusWarning
			if add == 0 {
				addColor = theme.TextDisabled
			}
			if upd == 0 {
				updColor = theme.TextDisabled
			}
			ui.Text(c, fmt.Sprintf("+%d", add)).Font("monospace").FontSize(11.5).Bold().TextColor(addColor).
				Tooltip(fmt.Sprintf("新增 %d 条", add))
			ui.Text(c, fmt.Sprintf("~%d", upd)).Font("monospace").FontSize(11.5).Bold().TextColor(updColor).
				Tooltip(fmt.Sprintf("更新 %d 条", upd))
			ui.Spacer(c)
			tip := "仅手动同步"
			if src.Interval > 0 {
				tip = "每 " + shortDuration(src.Interval) + "自动同步"
			}
			ui.Text(c, everyLabel(src)).FontSize(10).TextColor(theme.TextSecondary).SingleLine().Tooltip(tip)
		})
		if src.Note != "" {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				mygoicons.Image(c, icons.Info, 14)
				caption(c, src.Note).Shrink(1)
			})
		}
		if st.State == domain.SyncFailed && st.LastRun.Err != "" {
			ui.Row(c).Gap(8).Padding(7, 9).Radius(theme.Radius).Background(theme.StatusError.Alpha(0.1)).AlignItems(ui.Center).Children(func() {
				mygoicons.Image(c, icons.CrawlError, 14)
				ui.Text(c, st.LastRun.Err).FontSize(12).Shrink(1)
			})
		}
		for i, w := range st.Warnings {
			if i == 3 {
				caption(c, fmt.Sprintf("另有 %d 条提示", len(st.Warnings)-3))
				break
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				mygoicons.Image(c, icons.ParseError, 14)
				caption(c, w).Shrink(1)
			})
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if iconButtonRotating(c, icons.Sync, "同步 "+src.Name, st.State == domain.SyncRunning,
				!src.Enabled || st.State == domain.SyncRunning).Clicked() {
				a.Refresh(src.ID)
			}
			if src.URL != "" && iconTextButton(c, icons.Open, "打开原文").Tooltip(src.URL).Clicked() {
				c.OpenURL(src.URL)
			}
			ui.Spacer(c)
			if src.Kind == domain.SourceDemo {
				return // the built-in demo stays
			}
			del := a.deleteOpen[src.ID]
			if del == nil {
				del = new(bool)
				a.deleteOpen[src.ID] = del
			}
			if iconButton(c, icons.Delete, "删除 "+src.Name, false, st.State == domain.SyncRunning).Clicked() {
				*del = true
			}
			switch ui.AlertDialog(c, del, "删除数据源「"+src.Name+"」？",
				"将从配置文件中移除该数据源，并清除它抓取的公告。此操作无法撤销。", "取消", "删除") {
			case 1:
				a.removeSource(c, src)
			}
		})
	})
}

// sourceArtist is the configured artist whose own site a source is.
func (a *App) sourceArtist(src domain.SourceConfig) (id, name string, ok bool) {
	if src.Artist == "" {
		return "", "", false
	}
	key := normalize.Key(src.Artist)
	for _, ar := range a.opt.Config.Artists {
		if normalize.Key(ar.Name) == key {
			return avatar.ArtistID(ar), ar.Name, true
		}
		for _, al := range ar.Aliases {
			if normalize.Key(al) == key {
				return avatar.ArtistID(ar), ar.Name, true
			}
		}
	}
	return "", "", false
}

// removeSource deletes a source from the scheduler, the catalog and the
// configuration file.
func (a *App) removeSource(c *ui.Context, src domain.SourceConfig) {
	a.opt.Scheduler.Remove(src.ID)
	delete(a.srcOn, src.ID)
	delete(a.sourceOff, src.ID)
	delete(a.deleteOpen, src.ID)
	srcs := a.opt.Config.Sources[:0:0]
	for _, s := range a.opt.Config.Sources {
		if s.ID != src.ID {
			srcs = append(srcs, s)
		}
	}
	a.opt.Config.Sources = srcs
	if a.opt.RemoveSource == nil {
		c.Toast("已删除「" + src.Name + "」（仅本次运行有效）")
		return
	}
	path, err := a.opt.RemoveSource(src.ID)
	if err != nil {
		c.Toast("已删除「" + src.Name + "」，但写入配置失败：" + err.Error())
		return
	}
	a.opt.Config.Path = path
	c.Toast("已删除「" + src.Name + "」")
}

func (a *App) syncLog(c *ui.Context) {
	log := a.opt.Scheduler.Log()
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			mygoicons.Image(c, icons.UpdateLog, 16)
			ui.Text(c, "同步记录").Bold()
			caption(c, "最近 50 次")
		})
		if len(log) == 0 {
			caption(c, "还没有同步记录")
			return
		}
		card(c).Padding(6, 14).Children(func() {
			for i, r := range log {
				if i == 12 {
					break
				}
				c.Key(i)
				ui.Row(c).Gap(12).Padding(6, 0).AlignItems(ui.Center).Children(func() {
					caption(c, stamp(r.FinishedAt)).Font("monospace").Width(118).Shrink(0)
					ui.Text(c, a.sourceName(r.SourceID)).Font("monospace").FontSize(12).Width(150).SingleLine().Shrink(0)
					switch r.State {
					case domain.SyncFailed:
						ui.Text(c, "失败："+r.Err).Font("monospace").FontSize(12).TextColor(theme.StatusError).SingleLine().Shrink(1)
					default:
						s := fmt.Sprintf("成功 · 读取 %d · 新增 %d · 更新 %d · 用时 %.1fs", r.Fetched, r.New, r.Updated, r.FinishedAt.Sub(r.StartedAt).Seconds())
						if r.NotModified {
							s = "成功 · 无变化（304）"
						}
						ui.Text(c, s).Font("monospace").FontSize(12).SingleLine().Shrink(1)
					}
				})
			}
		})
	})
}

// Settings.

func (a *App) settingsPage(c *ui.Context) {
	page(c, func() {
		pageHeader(c, "设置")
		card(c).Padding(6, 16).Children(func() {
			row := func(label, value string) {
				ui.Row(c).Padding(10, 0).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, label).Width(140).Shrink(0)
					ui.Text(c, value).TextColor(theme.TextSecondary).Shrink(1)
				})
			}
			row("版本", "Live Calendar "+a.opt.Version)
			row("时区", "Asia/Tokyo（固定）")
			row("每周起始", "周一")
			row("配置文件", a.opt.Config.Path)
			row("数据存储", "公开公告保存在 cache.json；启动先显示上次同步的结果，再后台同步（不使用数据库）")
			row("头像缓存", "公开头像与主题色保存在 avatars.json；启动先用缓存，条件请求只替换有变化的图片")
			row(platformText("菜单栏", "系统托盘"), platformText(
				"关闭窗口后留在菜单栏继续同步；点击菜单栏图标或 Dock 图标重新打开，⌘Q 才退出",
				"关闭窗口后留在系统托盘继续同步；点击托盘图标打开窗口，或在托盘菜单退出"))
			notify := "系统通知：抽选截止前 3 天提醒（首次会请求授权；同一截止只提醒一次）"
			if a.notifyOff {
				notify = "系统通知：本次运行不可用（未授权或不是打包后的 App）"
			}
			row("截止提醒", notify)
			row("默认头像", "没有头像或尚未加载时，按艺人主题色绘制三种默认插画之一")
			ui.Row(c).Padding(10, 0).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "显示演示数据").Width(140).Shrink(0)
				sched := a.opt.Scheduler
				on, ok := a.srcOn["demo"]
				if !ok {
					on = new(bool)
					a.srcOn["demo"] = on
				}
				*on = sched.Enabled("demo")
				ui.Switch(c, on).Label("显示演示数据")
				if *on != sched.Enabled("demo") {
					sched.SetEnabled("demo", *on)
					if *on {
						a.Refresh("demo")
					}
				}
				caption(c, "虚构的艺人与公告，用于离线体验各种合并规则")
			})
			avatars := "Deezer 公开接口（按艺人名查找，每秒至多 1 次）"
			if a.opt.Config.Avatars == config.AvatarsOff {
				avatars = "关闭（只用 sources.yaml 中的 avatar_url / color）"
			}
			row("艺人头像", avatars)
			row("艺人主题色", "取头像中面积最大的色块（忽略占比不高时的白底与黑影）；可在 sources.yaml 用 color 指定")
			row("艺人置顶", "艺人列表右侧的图钉把艺人放到最前；保存在本地 pins.json，不属于账号数据")
			row("侧边栏", sidebarShortcut+" 展开 / 收起为图标栏；详情面板 "+inspectorShortcut)
			row("用户功能", "后续版本开放（当前无需登录）")
		})
		card(c).Padding(14, 16).Gap(6).Children(func() {
			ui.Text(c, "数据原则").Bold()
			for _, s := range []string{
				"只读取公开的官方或新闻信息，按 RFC 9309 遵守 robots.txt，同一站点请求间隔 ≥ 1 秒。",
				"头像查询调用的是 Deezer 面向程序的公开 JSON 接口，不抓取网页；可用 avatars: off 关闭。",
				"公告没写的时间、场地不做猜测，显示为「未公布」；无法确定的内容标记为「需确认」。",
				"延期、中止、截止时间变化会保留变更记录，不会静默覆盖。",
				"每条信息都能追溯到来源公告，可打开原文核对。",
			} {
				ui.Text(c, "· "+s).FontSize(12.5)
			}
		})
	})
}
