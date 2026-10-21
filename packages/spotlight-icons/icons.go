package spotlighticons

import (
	"embed"
	"io/fs"
	"sort"
)

//go:embed svg/*.svg
var files embed.FS

// Name identifies an icon; it is the SVG file name without ".svg".
type Name string

// Group is the category an icon belongs to in the library.
type Group string

const (
	GroupActions    Group = "actions"
	GroupSources    Group = "sources"
	GroupDetails    Group = "details"
	GroupStatus     Group = "status"
	GroupNavigation Group = "navigation"
	GroupViews      Group = "views"
)

// Palette lists the only colors the library uses (Spotlight color spec).
var Palette = map[string]string{
	"InkCharcoal":   "#2E2A28",
	"LiveCoral":     "#E85A3A",
	"WarmSpotlight": "#FFD9A8",
	"MistGray":      "#D8D1C7",
	"SoftAsh":       "#C7C0B6",
	"CloudWhite":    "#FFFDF8",
	"PaperIvory":    "#F7F3EA",
	"StatusSuccess": "#6B8F71",
	"StatusInfo":    "#7D92A8",
	"StatusWarning": "#D99A3D",
	"StatusError":   "#C65B52",
}

// Lookup returns the SVG source of the icon and whether it exists.
func Lookup(n Name) ([]byte, bool) {
	b, err := files.ReadFile("svg/" + string(n) + ".svg")
	if err != nil {
		return nil, false
	}
	return b, true
}

// SVG returns the SVG source of the icon, or nil if there is none.
func SVG(n Name) []byte {
	b, _ := Lookup(n)
	return b
}

// MustSVG is SVG that panics for an unknown name.
func MustSVG(n Name) []byte {
	b, ok := Lookup(n)
	if !ok {
		panic("spotlighticons: unknown icon " + string(n))
	}
	return b
}

// FS returns the icons as a file system of "<name>.svg" files.
func FS() fs.FS {
	sub, err := fs.Sub(files, "svg")
	if err != nil {
		panic(err)
	}
	return sub
}

// Names returns the names of all icons, sorted.
func Names() []Name {
	out := append([]Name(nil), All...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// GroupOf returns the group of an icon ("" if unknown).
func GroupOf(n Name) Group { return groups[n] }

// InGroup returns the icons of a group in library order.
func InGroup(g Group) []Name {
	var out []Name
	for _, n := range All {
		if groups[n] == g {
			out = append(out, n)
		}
	}
	return out
}
