package appicons

import "embed"

// windows holds a 256-px PNG of each icon for the windows' title bars and
// taskbar buttons, and a .ico (16 to 256 px) the shortcuts point to.
//
//go:embed windows/*.png windows/*.ico
var windows embed.FS

// PNG is the image the windows show for an icon while the app runs.
func PNG(key string) []byte {
	b, _ := windows.ReadFile("windows/" + Normalize(key) + ".png")
	return b
}

// ICO is the .ico of an icon for the shortcuts; nil for Default, whose
// shortcuts point at the executable's own icon.
func ICO(key string) []byte {
	key = Normalize(key)
	if key == Default {
		return nil
	}
	b, _ := windows.ReadFile("windows/" + key + ".ico")
	return b
}
