package views

import (
	"log"

	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/locales"
	"jpop-live-calendar/resources/appicons"
)

// appIconSize is the side of an icon in the App 图标 card, and appIconTile
// the width of its tile, the name under the icon included.
const (
	appIconSize = 72
	appIconTile = 112
)

// appIconName is the name an icon reads by, under it and for assistive
// technologies.
func appIconName(key string) string {
	switch key {
	case appicons.SpotlightCalendar:
		return locales.SettingsAppIconSpotlightCalendar()
	case appicons.TicketSquare:
		return locales.SettingsAppIconTicketSquare()
	case appicons.SpotlightLive10:
		return locales.SettingsAppIconSpotlightLive10()
	}
	return locales.SettingsAppIconDefault()
}

func (a *App) iconThumb(key string) *ui.Bitmap {
	if b, ok := a.iconThumbs[key]; ok {
		return b
	}
	b, err := ui.DecodeBitmap(appicons.Thumb(key))
	if err != nil {
		log.Printf("app icon %s: %v", key, err)
	}
	a.iconThumbs[key] = b // nil too: decoded once
	return b
}

// appIconCard is the 设置 card of the app icons, a row of its own under
// General and Artists: every icon whole at 72 pt with its name, the chosen
// one ringed in coral. A click applies the icon now and for good.
func (a *App) appIconCard(c *ui.Context) {
	card(c).FillWidth().Padding(12, 16, 14).Children(func() {
		settingsHeading(c, locales.SettingsAppIcon(), "")
		ui.Box(c).Height(1).FillWidth().Margin(10, 0, 12, 0).Background(theme.GridLine)
		ui.Row(c).Gap(12).Padding(0, 12).AlignItems(ui.Start).Children(func() {
			for _, key := range appicons.Keys {
				a.appIconTile(c, key)
			}
		})
		if a.appIconFailed {
			ui.Text(c, locales.SettingsAppIconFailed()).FontSize(theme.CaptionSize).
				TextColor(theme.LiveCoral).Margin(10, 12, 0, 12)
		}
	})
}

func (a *App) appIconTile(c *ui.Context, key string) {
	name := appIconName(key)
	chosen := a.appIcon == key
	b := ui.ButtonBase(c).Width(appIconTile).Padding(10, 6, 8).Radius(theme.Radius+4).
		Label(name).Border(2, ui.Color{})
	switch {
	case chosen:
		b.Border(2, theme.LiveCoral).Background(theme.SelectedBg)
	case b.Hovered():
		b.Background(theme.Hover)
	}
	if b.Children(func() {
		ui.Column(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if bmp := a.iconThumb(key); bmp != nil {
				ui.Image(c, bmp).Fit(ui.Contain).Size(appIconSize, appIconSize).Shrink(0)
			}
			t := ui.Text(c, name).FontSize(12).SingleLine()
			if chosen {
				t.Bold().TextColor(theme.LiveCoral)
			} else {
				t.TextColor(theme.TextSecondary)
			}
		})
	}).Clicked() && !chosen {
		a.setAppIcon(key)
	}
}

func (a *App) setAppIcon(key string) {
	a.appIcon, a.appIconFailed = key, false
	if a.opt.SetAppIcon == nil {
		return
	}
	a.opt.SetAppIcon(key, func(err error) {
		if err == nil {
			return
		}
		log.Printf("app icon %s: %v", key, err)
		a.opt.Post(func() {
			if a.appIcon == key { // not a later choice's
				a.appIconFailed = true
			}
		})
	})
}
