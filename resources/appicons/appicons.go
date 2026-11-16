// Package appicons holds the app icons the user can choose in 设置: a
// thumbnail of each for the settings row, and the images the running
// platform shows (see the darwin and windows files). They are embedded so
// the app carries them; internal/appicon applies them.
package appicons

import "embed"

// The keys of the icons, in the order the settings show them. prefs.json
// keeps one of them; Default is the icon the app is built with
// (resources/icon.png).
const (
	Default           = "default"
	SpotlightCalendar = "spotlight-calendar"
	TicketSquare      = "ticket-square"
	SpotlightLive10   = "spotlight-live10"
)

// Keys lists the icons in the order the settings show them.
var Keys = []string{Default, SpotlightCalendar, TicketSquare, SpotlightLive10}

//go:embed thumbs/*.png
var thumbs embed.FS

// Known reports whether key names one of the icons.
func Known(key string) bool {
	for _, k := range Keys {
		if k == key {
			return true
		}
	}
	return false
}

// Normalize returns key if it names an icon, and Default otherwise: a key
// of an icon a later version dropped falls back to the built-in one.
func Normalize(key string) string {
	if Known(key) {
		return key
	}
	return Default
}

// Thumb is the 144-px picture of an icon for the settings card (72 pt on
// a Retina screen): the tile itself, its transparent margin trimmed, so
// every icon shows whole and at the same size.
func Thumb(key string) []byte {
	b, _ := thumbs.ReadFile("thumbs/" + Normalize(key) + ".png")
	return b
}
