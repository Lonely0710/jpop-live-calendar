package views

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/locales"
)

// inspectorPanel is the right column: the selected entry, or the entries
// of the selected day.
func (a *App) inspectorPanel(c *ui.Context) {
	e, ok := a.snap.Entry(a.entryID)
	ui.Column(c).Grow(1).Shrink(1).MinHeight(0).Children(func() {
		ui.Row(c).Padding(10, 8, 6, 16).AlignItems(ui.Center).Gap(4).Children(func() {
			onMap := a.section == SectionMap
			switch {
			case ok:
				back := locales.InspectorBackDay()
				if onMap {
					back = locales.InspectorBackRegion()
				}
				if capsuleIcon(c, icons.Previous, back, false).Clicked() {
					a.entryID = ""
				}
			case onMap && a.mapPref != "":
				if capsuleIcon(c, icons.Previous, locales.InspectorBackNational(), false).Clicked() {
					a.mapPref = ""
				}
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).Height(theme.CapsuleH - 4).Children(func() {
				mygoicons.Image(c, icons.Info, 16)
				ui.Text(c, locales.InspectorTitle()).Bold()
			})
		})
		if !ok {
			if a.section == SectionMap {
				a.mapInspector(c)
			} else {
				a.dayInspector(c)
			}
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
				ui.Text(c, locales.CommonToday()).FontSize(theme.CaptionSize).Bold().TextColor(theme.LiveCoral)
			}
		})
	})
	if len(evs) == 0 {
		emptyState(c, icons.Calendar, locales.InspectorEmptyTitle(), locales.InspectorEmptyBody())
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
				ui.Text(c, e.Kind.Label(lang())).FontSize(theme.CaptionSize).Bold().TextColor(col.Mix(theme.InkCharcoal, 0.3))
			})
			if e.Status == domain.StatusPostponed || e.Status == domain.StatusCancelled || e.Status == domain.StatusSoldOut {
				tag(c, e.Status.Label(lang()), theme.StatusError)
			}
			if e.Changed {
				tag(c, locales.CalendarUpdated(), theme.StatusInfo)
			}
			if e.NeedsReview {
				tag(c, locales.CalendarNeedsReview(), theme.StatusWarning)
			}
		})
		t := ui.Text(c, title).Lang("ja").FontSize(17).Bold()
		if e.Status == domain.StatusCancelled {
			t.Strikethrough()
		}
		if sub != "" {
			ui.Text(c, sub).Lang("ja").TextColor(theme.TextSecondary)
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
					Background(pal.Tint).Label(locales.InspectorArtistLabel(ar.Name))
				if b.Hovered() {
					b.Background(pal.Hover)
				}
				if b.Children(func() {
					a.avatarView(c, id, ar.Name, 36)
					ui.Column(c).Shrink(1).Gap(1).Children(func() {
						ui.Text(c, ar.Name).Lang("ja").Bold().TextColor(pal.Text).SingleLine()
						caption(c, locales.InspectorArtistAll()).SingleLine()
					})
				}).Clicked() {
					a.section, a.artistID = SectionArtists, id
				}
			}
		})
	}
	ui.Column(c).Gap(10).Children(func() {
		field(c, icons.Calendar, locales.InspectorFieldDate(), dayTitle(ev.StartDate), false)
		times := locales.CalendarTimeTba()
		if ev.OpenAt != nil || ev.StartAt != nil {
			times = locales.InspectorTimes(clockOr(ev.OpenAt), clockOr(ev.StartAt))
		}
		field(c, icons.ShowTime, locales.InspectorFieldTimes(), times, false)
		venue := locales.CalendarVenueTba()
		if ev.VenueName != "" {
			venue = ev.VenueName
			if ev.Prefecture != "" {
				venue = ev.Prefecture + " · " + venue
			}
		}
		field(c, icons.Venue, locales.InspectorFieldVenue(), venue, false)
		field(c, icons.Live, locales.InspectorFieldTypeStatus(), eventTypeLabel(ev.Type)+" · "+ev.Status.Label(lang()), ev.Status != domain.StatusAnnounced)
		if ev.Tour != "" && ev.Tour != ev.Title {
			field(c, icons.Artist, locales.InspectorFieldTour(), ev.Tour, false)
		}
		for _, n := range ev.Notes {
			field(c, icons.Info, locales.InspectorFieldNotes(), n, false)
		}
	})
	a.reviewBox(c, ev.ReviewNotes)
	rounds := a.snap.RoundsFor(ev.ID)
	section(c, locales.InspectorRounds(len(rounds)), func() {
		if len(rounds) == 0 {
			caption(c, locales.InspectorNoRounds())
		}
		for _, r := range rounds {
			c.Key(r.ID)
			a.roundCard(c, r)
		}
		if ev.TicketURL != "" && len(rounds) == 0 {
			a.linkRow(c, locales.InspectorTicketPage(), ev.TicketURL)
		}
	})
	a.changeLog(c, ev.Changes)
	a.sourceList(c, ev.AnnouncementIDs)
}

func clockOr(cl *domain.Clock) string {
	if cl == nil {
		return locales.CommonTba()
	}
	return cl.String()
}

// roundCard summarizes a ticket round inside an event's inspector; a click
// opens the round.
func (a *App) roundCard(c *ui.Context, r domain.PublicTicketRound) {
	now := a.opt.Now()
	ended := a.endedRound(r)
	b := ui.ButtonBase(c).Column().FillWidth().AlignItems(ui.Stretch).Padding(10, 12).Gap(4).Radius(theme.Radius).
		Border(1, theme.MistGray).Label(locales.InspectorRoundLabel(r.Name))
	if b.Hovered() {
		b.Background(theme.Hover.Alpha(0.5))
	}
	if b.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			dot(c, theme.StatusWarning, 6)
			ui.Text(c, r.Name).Lang("ja").Bold().Shrink(1)
			ui.Spacer(c)
			countLabel(c, lotteryLabel(r))
		})
		caption(c, locales.InspectorRoundWindow(momentLabel(r.OpensAt), momentLabel(r.ClosesAt)))
		if r.ClosesAt != nil {
			caption(c, locales.InspectorDeadline()+countdownSuffix(countdown(r.ClosesAt, now)))
		}
		if r.ResultAt != nil {
			caption(c, locales.InspectorResultAt(momentLabel(r.ResultAt)))
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
	if ended {
		b.Children(func() { endedVeil(c, theme.Radius) })
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
		return r.Type.Label(lang()) + locales.InspectorLotterySuffix()
	}
	return r.Type.Label(lang()) + locales.InspectorFirstcomeSuffix()
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
		field(c, icons.Artist, locales.InspectorFieldArtist(), orDash(a.artistNames(r.ArtistIDs)), false)
		field(c, icons.Price, locales.InspectorFieldMethod(), lotteryLabel(r), false)
		field(c, icons.PreorderStart, locales.InspectorOpens(), momentLabel(r.OpensAt)+countdownSuffix(countdown(r.OpensAt, now)), e.Kind == domain.EntryLotteryOpen || e.Kind == domain.EntrySaleOpen)
		field(c, icons.PreorderEnd, locales.InspectorCloses(), momentLabel(r.ClosesAt)+countdownSuffix(countdown(r.ClosesAt, now)), true)
		if r.Lottery || r.ResultAt != nil {
			field(c, icons.Result, locales.InspectorResults(), momentLabel(r.ResultAt), e.Kind == domain.EntryResult)
		}
		if r.PurchaseURL != "" {
			a.linkRow(c, locales.InspectorPurchasePage(), r.PurchaseURL)
		}
	})
	a.reviewBox(c, r.ReviewNotes)
	section(c, locales.InspectorEvents(len(r.EventIDs)), func() {
		if len(r.EventIDs) == 0 {
			caption(c, locales.InspectorNoEvents())
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
	ended := a.endedEvent(ev)
	when := fmt.Sprintf("%s %s", shortDay(ev.StartDate), weekdayName(ev.StartDate.Weekday()))
	if ev.StartAt != nil {
		when += " " + ev.StartAt.String()
	}
	where := ev.VenueName
	if ev.Prefecture != "" {
		where = ev.Prefecture + " · " + where
	}
	if where == "" {
		where = locales.CalendarVenueTba()
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
			caption(c, where).Lang("ja").SingleLine()
		})
		ui.Spacer(c)
		if ev.Status == domain.StatusPostponed || ev.Status == domain.StatusCancelled {
			tag(c, ev.Status.Label(lang()), theme.StatusError)
		}
	}).Clicked() {
		a.showEntry("live:" + ev.ID)
	}
	if ended {
		b.Children(func() { endedVeil(c, 5) })
	}
}

func orDash(s string) string {
	if s == "" {
		return locales.InspectorUnknown()
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
			ui.Text(c, locales.InspectorReviewTitle()).FontSize(theme.CaptionSize).Bold()
		})
		for _, n := range notes {
			ui.Text(c, "· "+n).FontSize(theme.CaptionSize)
		}
		caption(c, locales.InspectorReviewBody())
	})
}

func (a *App) linkRow(c *ui.Context, label, url string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		mygoicons.Image(c, icons.Link, 16)
		ui.Link(c, label, url).Tooltip(url).Shrink(1)
		ui.Spacer(c)
		if iconButton(c, icons.ViewSwitch, locales.InspectorCopyLink(label), false, false).Clicked() {
			c.WriteClipboard(url)
			c.Toast(locales.InspectorCopied())
		}
	})
}

func (a *App) changeLog(c *ui.Context, changes []domain.Change) {
	if len(changes) == 0 {
		return
	}
	disclosure(c, locales.InspectorChanges(len(changes)), &a.showChanges, func() {
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
					caption(c, locales.InspectorAnnouncedAt(stamp(ch.At)))
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
	tr := p.Trigger.AlignSelf(ui.Start).AlignItems(ui.Center).Gap(4).Padding(2, 6, 2, 0).Radius(6).Label(locales.InspectorParseText())
	if tr.Hovered() {
		tr.Background(theme.GlassHover)
	}
	tr.Children(func() {
		mygoicons.Image(c, icons.Next, 11).Opacity(0.7).Rotate(90 * p.Progress())
		ui.Text(c, locales.InspectorParseText()).FontSize(theme.CaptionSize).TextColor(theme.TextSecondary)
	})
	if panel := p.Panel(func() {
		ui.Text(c, text).Lang("ja").Font("monospace").FontSize(11).TextColor(theme.TextSecondary).Selectable()
	}); panel.Valid() {
		panel.Padding(4, 8, 6).Radius(6).Background(theme.CloudWhite.Alpha(0.7))
	}
}

// sourceList shows the announcements a record came from, each with its
// source, its publish and fetch time, and links to the original.
func (a *App) sourceList(c *ui.Context, annIDs []string) {
	disclosure(c, locales.InspectorSourceAnnouncements(len(annIDs)), &a.showSources, func() {
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
				ui.Text(c, an.Title).Lang("ja").FontSize(12.5).MaxLines(3)
				caption(c, locales.InspectorPublished(stamp(an.PublishedAt)))
				caption(c, locales.InspectorFetched(stamp(an.FetchedAt), an.Revision))
				open := a.rawOpen[an.ID]
				if open == nil {
					open = new(bool)
					a.rawOpen[an.ID] = open
				}
				rawText(c, an.RawText, open)
				if a.sourceKind(an.SourceID) == domain.SourceDemo {
					caption(c, locales.InspectorDemo())
					return
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if iconTextButton(c, icons.Open, locales.InspectorOpenSource()).Tooltip(an.URL).Clicked() {
						c.OpenURL(an.URL)
					}
					if iconButton(c, icons.ViewSwitch, locales.InspectorCopySource(), false, false).Clicked() {
						c.WriteClipboard(an.URL)
						c.Toast(locales.InspectorSourceCopied())
					}
				})
			})
		}
	})
}
