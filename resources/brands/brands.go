// Package brands holds the marks of the music services a source comes
// from: Sony Music's artist API, Spotify, Apple Music and QQ Music. They
// are embedded so the app carries them, and shown instead of naming the
// service in words.
package brands

import (
	_ "embed"
	"net/url"
	"strings"
)

var (
	//go:embed sony.svg
	sonySVG []byte
	//go:embed spotify.svg
	spotifySVG []byte
	//go:embed apple-music.svg
	appleSVG []byte
	//go:embed qq-music.svg
	qqSVG []byte
)

// Brand is a music service a source belongs to.
type Brand struct {
	// Name is the service as it is written in the interface.
	Name string
	// SVG is the mark.
	SVG []byte
}

var (
	Sony       = Brand{Name: "Sony Music", SVG: sonySVG}
	Spotify    = Brand{Name: "Spotify", SVG: spotifySVG}
	AppleMusic = Brand{Name: "Apple Music", SVG: appleSVG}
	QQMusic    = Brand{Name: "QQ Music", SVG: qqSVG}
)

// hosts maps a source's host, and the hosts below it, to its service. Only
// the services whose marks the app carries are listed; a source of an
// artist's own site has no brand.
var hosts = []struct {
	suffix string
	brand  Brand
}{
	{"sonymusic.co.jp", Sony},
	{"spotify.com", Spotify},
	{"music.apple.com", AppleMusic},
	{"qq.com", QQMusic},
}

// For returns the service a source's address belongs to.
func For(rawURL string) (Brand, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return Brand{}, false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range hosts {
		if host == h.suffix || strings.HasSuffix(host, "."+h.suffix) {
			return h.brand, true
		}
	}
	return Brand{}, false
}
