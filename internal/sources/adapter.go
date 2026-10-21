// Package sources reads public announcement sources. Each kind of source
// (RSS/Atom feed, official site page, API, built-in demo data) has an
// Adapter that turns it into domain.PublicAnnouncement values; parsing and
// merging happen later, in internal/parser and internal/dedup.
//
// Adapters never write to the catalog and never touch the UI: the crawler
// calls them off the UI thread and stores what they return.
package sources

import (
	"context"
	"time"

	"jpop-live-calendar/internal/domain"
)

// Response is a fetched page, already decoded to UTF-8.
type Response struct {
	URL          string
	Status       int
	Text         string
	Body         []byte // the bytes as received, for images and JSON
	NotModified  bool   // 304: the cached copy is still current
	ETag         string // the validators of the response, for a cached copy
	LastModified string
}

// Getter fetches pages politely (robots.txt, rate limits, conditional
// requests). crawler.Fetcher implements it.
type Getter interface {
	Get(ctx context.Context, url string) (*Response, error)
}

// Env is what an adapter may use.
type Env struct {
	Get   Getter
	Known func(url string) bool // full text already stored; may be nil
	Now   func() time.Time
}

func (e Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e Env) known(url string) bool { return e.Known != nil && e.Known(url) }

// Result is what one fetch of a source produced.
type Result struct {
	Announcements []domain.PublicAnnouncement
	NotModified   bool     // nothing changed since the last fetch
	Warnings      []string // problems that did not stop the fetch
}

// Adapter reads one kind of source.
type Adapter interface {
	Fetch(ctx context.Context, src domain.SourceConfig, env Env) (Result, error)
}

// Default returns the adapters for every source kind.
func Default() map[domain.SourceKind]Adapter {
	return map[domain.SourceKind]Adapter{
		domain.SourceDemo:         Demo{},
		domain.SourceRSS:          RSS{},
		domain.SourceOfficialSite: Website{},
		domain.SourceAPI:          API{},
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
