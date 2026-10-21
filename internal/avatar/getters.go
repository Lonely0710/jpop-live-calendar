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
// The Deezer lookup is a call to a public JSON API made for programs, not
// crawling of pages: api.deezer.com's robots.txt (written for search
// engines) disallows everything, so the lookup getter does not read it, but
// it is still rate limited to one request per second and sends the app's
// User-Agent. Pictures are downloaded through an ordinary polite fetcher.
func NewGetters(mode string) Getters {
	g := Getters{Pictures: crawler.NewFetcher()}
	if mode != config.AvatarsOff {
		g.Lookup = &crawler.Fetcher{
			Client:       &http.Client{Timeout: 15 * time.Second},
			Delay:        time.Second,
			IgnoreRobots: true,
		}
	}
	return g
}
