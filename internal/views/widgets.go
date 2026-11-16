package views

import (
	"time"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
)

// iconButton is a square toolbar button showing a library icon in its own
// colors. label names it for the tooltip, tests and assistive technology.
func iconButton(c *ui.Context, name icons.Name, label string, active, disabled bool) ui.Element {
	b := ui.ButtonBase(c).Size(theme.ToolButton, theme.ToolButton).Radius(theme.Radius).
		Label(label).Tooltip(label).Disabled(disabled)
	switch {
	case active:
		b.Background(theme.SelectedBg)
	case b.Pressed() && !disabled:
		b.Background(theme.Pressed)
	case b.Hovered() && !disabled:
		b.Background(theme.Hover)
	}
	return b.Children(func() {
		img := mygoicons.Image(c, name, theme.IconSize)
		if disabled {
			img.Opacity(0.4)
		}
	})
}

// iconButtonRotating is iconButton with an icon that turns while rotating
// is set, as the sync button of a source does while its sync runs.
func iconButtonRotating(c *ui.Context, name icons.Name, label string, rotating, disabled bool) ui.Element {
	b := ui.ButtonBase(c).Size(theme.ToolButton, theme.ToolButton).Radius(theme.Radius).
		Label(label).Tooltip(label).Disabled(disabled)
	switch {
	case b.Pressed() && !disabled:
		b.Background(theme.Pressed)
	case b.Hovered() && !disabled:
		b.Background(theme.Hover)
	}
	return b.Children(func() {
		img := mygoicons.Image(c, name, theme.IconSize)
		if rotating && !c.Preferences().ReduceMotion {
			img.Rotate(img.Loop("spin:"+label, time.Second, ui.Linear) * 360)
		}
		if disabled && !rotating {
			img.Opacity(0.4)
		}
	})
}

// iconTextButton is MyGo's push button with a library icon before its
// text, as the Button page shows.
func iconTextButton(c *ui.Context, name icons.Name, text string) ui.Element {
	return ui.Button(c, "").Label(text).Children(func() {
		mygoicons.Image(c, name, 15)
		ui.Text(c, text).SingleLine()
	})
}

// sourceKindIcon is the icon of a kind of source.
func sourceKindIcon(k domain.SourceKind) icons.Name {
	switch k {
	case domain.SourceOfficialSite:
		return icons.SourceSite
	case domain.SourceAPI:
		return icons.API
	case domain.SourceDemo:
		return icons.Announcement
	}
	return icons.RSS
}

// dot is a small round marker.
func dot(c *ui.Context, col ui.Color, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Radius(size / 2).Background(col)
}

// caption is secondary text.
func caption(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(theme.CaptionSize).TextColor(theme.TextSecondary)
}

// countLabel is the figure a row puts on its right, next to a Spacer ("36
// shows", "締切 10/31 (土)"). It keeps to one line whatever the row is
// squeezed to: the name on the left gives way, the count never breaks into
// "36 / shows".
func countLabel(c *ui.Context, s string) ui.Element {
	return caption(c, s).SingleLine().Shrink(0)
}

// dateScopeChoice is the 全部日期 / 今天起 / 已过去 choice: one pill group, the
// chosen one filled coral, so the day range the list shows reads at a
// glance instead of as a pale chip.
func dateScopeChoice(c *ui.Context, sel *int, labels []string, aria string) ui.Element {
	seg := ui.SegmentedBase(c, sel, len(labels))
	seg.Track.Height(28).Padding(2).Gap(2).Radius(theme.Radius).AlignItems(ui.Center).
		Background(theme.PaperIvory).Border(1, theme.MistGray).Shrink(0).Label(aria).Children(func() {
		for i, text := range labels {
			s := seg.Segment(i).Height(24).Padding(0, 10).Radius(theme.Radius - 2).AlignItems(ui.Center).Label(text)
			switch {
			case *sel == i:
				s.Background(theme.LiveCoral)
			case s.Hovered():
				s.Background(theme.Hover)
			}
			s.Children(func() {
				t := ui.Text(c, text).FontSize(12).SingleLine()
				if *sel == i {
					t.Bold().TextColor(theme.CloudWhite)
				} else {
					t.TextColor(theme.TextSecondary)
				}
			})
		}
	})
	return seg.Track
}

// emptyState is the centered placeholder of a page without content.
func emptyState(c *ui.Context, name icons.Name, title, body string) ui.Element {
	return ui.Column(c).Grow(1).Center().Gap(10).Padding(32).Children(func() {
		ui.Box(c).Size(56, 56).Radius(28).Background(theme.WarmSpotlight.Alpha(0.45)).Center().Children(func() {
			mygoicons.Image(c, name, 28)
		})
		ui.Text(c, title).FontSize(15).Bold()
		ui.Text(c, body).TextColor(theme.TextSecondary).TextAlign(ui.Center).MaxWidth(360)
	})
}

// card is a panel on the sheet: Cloud White inside a hairline so light it
// only catches the edge, lifted a little by its shadow. A material of glass
// would outline it with the dark rim MyGo draws along its sides.
func card(c *ui.Context) ui.Element {
	return ui.Column(c).Background(theme.CloudWhite).Border(1, theme.CardLine).
		Radius(theme.CardRadius).Shadow(0, 1, 4, 0, theme.SheetShadow)
}

// pageHeader is the heading of the non-calendar pages.
func pageHeader(c *ui.Context, title string) {
	ui.Text(c, title).FontSize(20).Bold()
}
