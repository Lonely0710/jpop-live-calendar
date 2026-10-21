package views

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
)

// inspectorPanel is the right column: the selected entry, or the entries
// of the selected day.
func (a *App) inspectorPanel(c *ui.Context) {
	e, ok := a.snap.Entry(a.entryID)
	ui.Column(c).Grow(1).Shrink(1).MinHeight(0).Children(func() {
		ui.Row(c).Padding(10, 8, 6, 16).AlignItems(ui.Center).Gap(4).Children(func() {
			if ok {
				if capsuleIcon(c, icons.Previous, "返回当天列表", false).Clicked() {
					a.entryID = ""
				}
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).Height(theme.CapsuleH - 4).Children(func() {
				mygoicons.Image(c, icons.Info, 16)
				ui.Text(c, "详情").Bold()
			})
		})
		if !ok {
			a.dayInspector(c)
			return
		}
		ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
			ui.Column(c).Padding(4, 18, 18).Gap(14).Children(func() {
				if e.EventID != "" {
					a.eventInspector(c, e)
				} else {
					a.roundInspector(c, e)
				}
			})
		})
	})
}

func (a *App) dayInspector(c *ui.Context) {
	d := a.selected
	evs := a.entries(d, d)
	ui.Column(c).Padding(0, 18, 14).Gap(4).BorderWidth(0, 0, 1, 0).BorderColor(theme.GlassLine).Children(func() {
		ui.Text(c, shortDay(d)).FontSize(24).Bold()
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			caption(c, weekdayName(d.Weekday())+" · Asia/Tokyo")
			if d == a.today() {
				ui.Text(c, "今天").FontSize(theme.CaptionSize).Bold().TextColor(theme.LiveCoral)
			}
		})
	})
	if len(evs) == 0 {
		emptyState(c, icons.Calendar, "当天没有公开事件", "选中日历中的日期或事件，查看 Live 场次、抽选受付与官方公告。")
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(12, 14).Gap(6).Children(func() {
			for _, e := range evs {
				c.Key(e.ID)
				a.chip(c, e, false)
			}
		})
	})
}

func kindIcon(k domain.EntryKind) icons.Name {
	switch k {
	case domain.EntryLotteryOpen:
		return icons.PreorderStart
	case domain.EntryLotteryClose:
		return icons.PreorderEnd
	case domain.EntryResult:
		return icons.Result
	case domain.EntrySaleOpen:
		return icons.GeneralSale
	}
	return icons.Live
}

func (a *App) entryHeader(c *ui.Context, e domain.Entry, title, sub string) {
	col := theme.KindColor(e.Kind, e.Status)
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Row(c).Gap(5).Padding(2, 8).Radius(10).Background(col.Alpha(0.15)).AlignItems(ui.Center).Children(func() {
				mygoicons.Image(c, kindIcon(e.Kind), 14)
				ui.Text(c, e.Kind.Label()).FontSize(theme.CaptionSize).Bold().TextColor(col.Mix(theme.InkCharcoal, 0.3))
			})
			if e.Status == domain.StatusPostponed || e.Status == domain.StatusCancelled || e.Status == domain.StatusSoldOut {
				tag(c, e.Status.Label(), theme.StatusError)
			}
			if e.Changed {
				tag(c, "有更新", theme.StatusInfo)
			}
			if e.NeedsReview {
				tag(c, "需确认", theme.StatusWarning)
			}
		})
		t := ui.Text(c, title).FontSize(17).Bold()
		if e.Status == domain.StatusCancelled {
			t.Strikethrough()
		}
		if sub != "" {
			ui.Text(c, sub).TextColor(theme.TextSecondary)
		}
	})
}

// field is an icon, a label and a value.
func field(c *ui.Context, name icons.Name, label, value string, strong bool) {
	ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
		mygoicons.Image(c, name, 16)
		ui.Column(c).Gap(1).Shrink(1).Children(func() {
			caption(c, label)
			t := ui.Text(c, value)
			if strong {
				t.Bold()
			}
		})
	})
}

func (a *App) eventInspector(c *ui.Context, e domain.Entry) {
	ev, ok := a.snap.Event(e.EventID)
	if !ok {
		return
	}
	a.entryHeader(c, e, ev.Title, "")
	if len(ev.ArtistIDs) > 0 {
		ui.Column(c).Gap(6).Children(func() {
			for _, id := range ev.ArtistIDs {
				ar, ok := a.snap.Artist(id)
				if !ok {
					continue
				}
				c.Key("artist:" + id)
				pal := theme.Artist(a.artistColor(id, ar.Name))
				b := ui.ButtonBase(c).FillWidth().Padding(6, 8).Gap(10).Radius(theme.Radius).Justify(ui.Start).
					Background(pal.Tint).Label("艺人 " + ar.Name)
				if b.Hovered() {
					b.Background(pal.Hover)
				}
				if b.Children(func() {
					a.avatarView(c, id, ar.Name, 36)
					ui.Column(c).Shrink(1).Gap(1).Children(func() {
						ui.Text(c, ar.Name).Bold().TextColor(pal.Text).SingleLine()
						caption(c, "查看该艺人的全部巡演").SingleLine()
					})
				}).Clicked() {
					a.section, a.artistID = SectionArtists, id
				}
			}
		})
	}
	ui.Column(c).Gap(10).Children(func() {
		field(c, icons.Calendar, "日期", dayTitle(ev.StartDate), false)
		times := "时间未公布"
		if ev.OpenAt != nil || ev.StartAt != nil {
			times = fmt.Sprintf("开场 %s / 开演 %s", clockOr(ev.OpenAt), clockOr(ev.StartAt))
		}
		field(c, icons.ShowTime, "开场 / 开演", times, false)
		venue := "场地未公布"
		if ev.VenueName != "" {
			venue = ev.VenueName
			if ev.Prefecture != "" {
				venue = ev.Prefecture + " · " + venue
			}
		}
		field(c, icons.Venue, "场地", venue, false)
		field(c, icons.Live, "类型 / 状态", eventTypeLabel(ev.Type)+" · "+ev.Status.Label(), ev.Status != domain.StatusAnnounced)
		if ev.Tour != "" && ev.Tour != ev.Title {
			field(c, icons.Artist, "巡演", ev.Tour, false)
		}
		for _, n := range ev.Notes {
			field(c, icons.Info, "备注", n, false)
		}
	})
	a.reviewBox(c, ev.ReviewNotes)
	rounds := a.snap.RoundsFor(ev.ID)
	section(c, fmt.Sprintf("票务节点（%d）", len(rounds)), func() {
		if len(rounds) == 0 {
			caption(c, "公告中还没有票务信息")
		}
		for _, r := range rounds {
			c.Key(r.ID)
			a.roundCard(c, r)
		}
		if ev.TicketURL != "" && len(rounds) == 0 {
			a.linkRow(c, "购票页面", ev.TicketURL)
		}
	})
	a.changeLog(c, ev.Changes)
	a.sourceList(c, ev.AnnouncementIDs)
}

func clockOr(cl *domain.Clock) string {
	if cl == nil {
		return "未公布"
	}
	return cl.String()
}

// roundCard summarizes a ticket round inside an event's inspector; a click
// opens the round.
func (a *App) roundCard(c *ui.Context, r domain.PublicTicketRound) {
	now := a.opt.Now()
	b := ui.ButtonBase(c).Column().FillWidth().AlignItems(ui.Stretch).Padding(10, 12).Gap(4).Radius(theme.Radius).
		Border(1, theme.MistGray).Label("票务 " + r.Name)
	if b.Hovered() {
		b.Background(theme.Hover.Alpha(0.5))
	}
	if b.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			dot(c, theme.StatusWarning, 6)
			ui.Text(c, r.Name).Bold().Shrink(1)
			ui.Spacer(c)
			caption(c, lotteryLabel(r))
		})
		caption(c, fmt.Sprintf("受付 %s ~ %s", momentLabel(r.OpensAt), momentLabel(r.ClosesAt)))
		if r.ClosesAt != nil {
			caption(c, "截止"+countdownSuffix(countdown(r.ClosesAt, now)))
		}
		if r.ResultAt != nil {
			caption(c, "结果公布 "+momentLabel(r.ResultAt))
		}
	}).Clicked() {
		for _, k := range []domain.EntryKind{domain.EntryLotteryClose, domain.EntryLotteryOpen, domain.EntrySaleOpen, domain.EntryResult} {
			if e, ok := a.snap.Entry(string(k) + ":" + r.ID); ok {
				a.selectEntry(e)
				a.cursor = e.Date
				break
			}
		}
	}
}

func countdownSuffix(s string) string {
	if s == "" {
		return ""
	}
	return " · " + s
}

func lotteryLabel(r domain.PublicTicketRound) string {
	if r.Lottery {
		return r.Type.Label() + " · 抽选"
	}
	return r.Type.Label() + " · 先着"
}

func (a *App) roundInspector(c *ui.Context, e domain.Entry) {
	r, ok := a.snap.Round(e.RoundID)
	if !ok {
		return
	}
	now := a.opt.Now()
	title := r.Tour
	if title == "" {
		title = e.Title
	}
	a.entryHeader(c, e, r.Name, title)
	ui.Column(c).Gap(10).Children(func() {
		field(c, icons.Artist, "艺人", orDash(a.artistNames(r.ArtistIDs)), false)
		field(c, icons.Price, "方式", lotteryLabel(r), false)
		field(c, icons.PreorderStart, "受付开始", momentLabel(r.OpensAt)+countdownSuffix(countdown(r.OpensAt, now)), e.Kind == domain.EntryLotteryOpen || e.Kind == domain.EntrySaleOpen)
		field(c, icons.PreorderEnd, "受付截止", momentLabel(r.ClosesAt)+countdownSuffix(countdown(r.ClosesAt, now)), true)
		if r.Lottery || r.ResultAt != nil {
			field(c, icons.Result, "结果公布", momentLabel(r.ResultAt), e.Kind == domain.EntryResult)
		}
		if r.PurchaseURL != "" {
			a.linkRow(c, "购票 / 受付页面", r.PurchaseURL)
		}
	})
	a.reviewBox(c, r.ReviewNotes)
	section(c, fmt.Sprintf("对应场次（%d）", len(r.EventIDs)), func() {
		if len(r.EventIDs) == 0 {
			caption(c, "公告没有写明对应哪些场次")
		}
		for _, id := range r.EventIDs {
			ev, ok := a.snap.Event(id)
			if !ok {
				continue
			}
			c.Key(id)
			a.showRow(c, ev, false)
		}
	})
	a.changeLog(c, r.Changes)
	a.sourceList(c, r.AnnouncementIDs)
}

// showRow is a show a ticket round is for: its day, time and venue.
func (a *App) showRow(c *ui.Context, ev domain.PublicEvent, withTitle bool) {
	pal := theme.Artist(theme.KindColor(domain.EntryLive, ev.Status))
	for _, id := range ev.ArtistIDs {
		if ar, ok := a.snap.Artist(id); ok {
			pal = theme.Artist(a.artistColor(id, ar.Name))
			break
		}
	}
	col := pal.Accent
	if ev.Status == domain.StatusPostponed || ev.Status == domain.StatusCancelled {
		col = theme.StatusError
	}
	when := fmt.Sprintf("%s %s", shortDay(ev.StartDate), weekdayName(ev.StartDate.Weekday()))
	if ev.StartAt != nil {
		when += " " + ev.StartAt.String()
	}
	where := ev.VenueName
	if ev.Prefecture != "" {
		where = ev.Prefecture + " · " + where
	}
	if where == "" {
		where = "场地未公布"
	}
	if withTitle {
		where = ev.Title + " · " + where
	}
	b := ui.ButtonBase(c).FillWidth().Gap(8).Radius(5).Padding(5, 8, 5, 0).Justify(ui.Start).
		Label(when + " " + where).Background(pal.Tint)
	if b.Hovered() {
		b.Background(pal.Hover)
	}
	if b.Children(func() {
		ui.Box(c).Width(3).FillHeight().Background(col)
		ui.Column(c).Gap(1).Shrink(1).Children(func() {
			ui.Text(c, when).FontSize(theme.CaptionSize).Bold().SingleLine()
			caption(c, where).SingleLine()
		})
		ui.Spacer(c)
		if ev.Status == domain.StatusPostponed || ev.Status == domain.StatusCancelled {
			tag(c, ev.Status.Label(), theme.StatusError)
		}
	}).Clicked() {
		a.showEntry("live:" + ev.ID)
	}
}

func orDash(s string) string {
	if s == "" {
		return "未识别"
	}
	return s
}

// section is a titled group in the inspector.
func section(c *ui.Context, title string, fn func()) {
	ui.Column(c).Gap(8).Padding(12, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(theme.MistGray).Children(func() {
		ui.Text(c, title).FontSize(theme.CaptionSize).Bold().TextColor(theme.TextSecondary)
		fn()
	})
}

func (a *App) reviewBox(c *ui.Context, notes []string) {
	if len(notes) == 0 {
		return
	}
	ui.Column(c).Gap(4).Padding(10, 12).Radius(theme.Radius).Background(theme.StatusWarning.Alpha(0.12)).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			mygoicons.Image(c, icons.ParseError, 14)
			ui.Text(c, "需要确认").FontSize(theme.CaptionSize).Bold()
		})
		for _, n := range notes {
			ui.Text(c, "· "+n).FontSize(theme.CaptionSize)
		}
		caption(c, "以下信息未经猜测补全，请以原文为准。")
	})
}

func (a *App) linkRow(c *ui.Context, label, url string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		mygoicons.Image(c, icons.Link, 16)
		ui.Link(c, label, url).Tooltip(url).Shrink(1)
		ui.Spacer(c)
		if iconButton(c, icons.ViewSwitch, "复制链接 "+label, false, false).Clicked() {
			c.WriteClipboard(url)
			c.Toast("已复制链接")
		}
	})
}

func (a *App) changeLog(c *ui.Context, changes []domain.Change) {
	if len(changes) == 0 {
		return
	}
	disclosure(c, fmt.Sprintf("变更记录（%d）", len(changes)), &a.showChanges, func() {
		for i := len(changes) - 1; i >= 0; i-- {
			ch := changes[i]
			c.Key(i)
			ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
				mygoicons.Image(c, icons.UpdateLog, 14)
				ui.Column(c).Gap(1).Shrink(1).Children(func() {
					ui.Row(c).Gap(4).Wrap().Children(func() {
						ui.Text(c, ch.Field).FontSize(theme.CaptionSize).Bold()
						ui.Text(c, ch.Old).FontSize(theme.CaptionSize).Strikethrough().TextColor(theme.TextSecondary)
						ui.Text(c, "→ "+ch.New).FontSize(theme.CaptionSize)
					})
					caption(c, "公告发布于 "+stamp(ch.At))
				})
			})
		}
	})
}

// disclosure is a titled group of the inspector that opens and closes,
// as section is, its content flush with the title: ui.Collapsible
// indents its panel under the label.
func disclosure(c *ui.Context, title string, open *bool, fn func()) {
	ui.Column(c).Gap(0).Padding(12, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(theme.MistGray).Children(func() {
		p := ui.CollapsibleBase(c, open)
		tr := p.Trigger.FillWidth().AlignItems(ui.Center).Gap(6).Padding(2, 4, 2, 0).Radius(6).Label(title)
		if tr.Hovered() {
			tr.Background(theme.GlassHover)
		}
		tr.Children(func() {
			ui.Text(c, title).FontSize(theme.CaptionSize).Bold().TextColor(theme.TextSecondary).SingleLine()
			ui.Spacer(c)
			mygoicons.Image(c, icons.Next, 12).Opacity(0.7).Rotate(90 * p.Progress())
		})
		if panel := p.Panel(fn); panel.Valid() {
			panel.Gap(8).Padding(8, 0, 0)
		}
	})
}

// rawText is the text an announcement was parsed from, under its own
// disclosure, in the system's monospaced font.
func rawText(c *ui.Context, text string, open *bool) {
	p := ui.CollapsibleBase(c, open)
	tr := p.Trigger.AlignSelf(ui.Start).AlignItems(ui.Center).Gap(4).Padding(2, 6, 2, 0).Radius(6).Label("解析用文本")
	if tr.Hovered() {
		tr.Background(theme.GlassHover)
	}
	tr.Children(func() {
		mygoicons.Image(c, icons.Next, 11).Opacity(0.7).Rotate(90 * p.Progress())
		ui.Text(c, "解析用文本").FontSize(theme.CaptionSize).TextColor(theme.TextSecondary)
	})
	if panel := p.Panel(func() {
		ui.Text(c, text).Font("monospace").FontSize(11).TextColor(theme.TextSecondary).Selectable()
	}); panel.Valid() {
		panel.Padding(4, 8, 6).Radius(6).Background(theme.CloudWhite.Alpha(0.7))
	}
}

// sourceList shows the announcements a record came from, each with its
// source, its publish and fetch time, and links to the original.
func (a *App) sourceList(c *ui.Context, annIDs []string) {
	disclosure(c, fmt.Sprintf("来源公告（%d）", len(annIDs)), &a.showSources, func() {
		for i := len(annIDs) - 1; i >= 0; i-- {
			an, ok := a.snap.Announcement(annIDs[i])
			if !ok {
				continue
			}
			c.Key(an.ID)
			ui.Column(c).Gap(4).Padding(10, 12).Radius(theme.Radius).Background(theme.PaperIvory).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					mygoicons.Image(c, sourceKindIcon(a.sourceKind(an.SourceID)), 14)
					caption(c, a.sourceName(an.SourceID)).Bold()
				})
				ui.Text(c, an.Title).FontSize(12.5).MaxLines(3)
				caption(c, "发布 "+stamp(an.PublishedAt))
				caption(c, fmt.Sprintf("抓取 %s · 第 %d 版", stamp(an.FetchedAt), an.Revision))
				open := a.rawOpen[an.ID]
				if open == nil {
					open = new(bool)
					a.rawOpen[an.ID] = open
				}
				rawText(c, an.RawText, open)
				if a.sourceKind(an.SourceID) == domain.SourceDemo {
					caption(c, "演示数据：内容为虚构，没有原文页面")
					return
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if iconTextButton(c, icons.Open, "打开原文").Tooltip(an.URL).Clicked() {
						c.OpenURL(an.URL)
					}
					if iconButton(c, icons.ViewSwitch, "复制原文链接", false, false).Clicked() {
						c.WriteClipboard(an.URL)
						c.Toast("已复制原文链接")
					}
				})
			})
		}
	})
}
