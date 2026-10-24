package avatar

import (
	"net/http"
	"time"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/crawler"
)

// NewGetters returns the getters the app uses for a config's avatars
// setting.
//
// The lookups are calls to public JSON APIs made for programs, not
// crawling of pages: api.deezer.com's robots.txt (written for search
// engines) disallows everything, so the lookup getters do not read it, but
// they are still rate limited to one request per second and send the app's
// User-Agent. Pictures are downloaded through an ordinary polite fetcher.
func NewGetters(mode string) Getters {
	g := Getters{Pictures: crawler.NewFetcher()}
	switch mode {
	case config.AvatarsOff:
	case config.AvatarsQQ:
		// QQ Music's photo when it has one, Apple's cover when it does
		// not: both answer from the mainland.
		g.QQ = lookupFetcher()
		g.Albums = lookupFetcher()
	case config.AvatarsITunes:
		g.Albums = lookupFetcher()
	default:
		// A photo first, a cover last: Deezer (blocked from the mainland),
		// then QQ Music (answers there), then Apple's covers for whatever
		// neither of them knows.
		g.Lookup = lookupFetcher()
		g.QQ = lookupFetcher()
		g.Albums = lookupFetcher()
	}
	return g
}

// lookupFetcher is the fetcher of one public API: its own client, one
// request per second, robots.txt left to the site's own terms as above.
func lookupFetcher() *crawler.Fetcher {
	return &crawler.Fetcher{
		Client:       &http.Client{Timeout: 15 * time.Second},
		Delay:        time.Second,
		IgnoreRobots: true,
	}
}
