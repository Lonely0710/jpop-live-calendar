// Package mygoicons adapts the Spotlight icons to MyGo.
//
// Two ways to draw an icon:
//
//   - Image keeps the icon's own colors (Ink Charcoal lines + Live Coral
//     accents). Use it for navigation, toolbar and status icons.
//   - Mono draws the icon in the current text color (ui.Icon), for places
//     that must follow the text, such as a disabled or inverted button.
package mygoicons

import (
	"sync"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
)

var (
	mu    sync.Mutex
	cache = map[icons.Name]*ui.SVG{}
)

// Get returns the parsed icon, parsing it once. It panics for an unknown
// name, which a test catches (every constant is generated from svg/).
func Get(n icons.Name) *ui.SVG {
	mu.Lock()
	defer mu.Unlock()
	if s, ok := cache[n]; ok {
		return s
	}
	s := ui.MustParseSVG(icons.MustSVG(n))
	cache[n] = s
	return s
}

// Image shows the icon in its own colors at size×size DIPs.
func Image(c *ui.Context, n icons.Name, size float32) ui.Element {
	return ui.Image(c, Get(n)).Size(size, size)
}

// Mono shows the icon in the text color at size×size DIPs.
func Mono(c *ui.Context, n icons.Name, size float32) ui.Element {
	return ui.Icon(c, Get(n)).Size(size, size)
}
