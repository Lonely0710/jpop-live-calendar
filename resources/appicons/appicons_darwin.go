package appicons

import "embed"

// darwin holds 512-px PNGs on the macOS icon grid (an 824/1024 tile with a
// transparent margin), for the Dock and the bundle's Finder icon.
//
//go:embed darwin/*.png
var darwin embed.FS

// PNG is the image the Dock shows for an icon while the app runs.
func PNG(key string) []byte {
	b, _ := darwin.ReadFile("darwin/" + Normalize(key) + ".png")
	return b
}

// ICO is not used on macOS.
func ICO(string) []byte { return nil }
