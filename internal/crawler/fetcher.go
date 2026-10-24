package crawler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/htmlindex"

	"jpop-live-calendar/internal/sources"
	"jpop-live-calendar/locales"
)

// UserAgent identifies the app to the sites it reads.
const UserAgent = "LiveCalendar/1.0 (+personal calendar; respects robots.txt)"

// Fetcher gets public pages politely:
//   - it obeys robots.txt (cached per host for a day);
//   - it waits at least Delay between two requests to the same host;
//   - it sends If-None-Match / If-Modified-Since and reports 304s;
//   - it retries network errors, 429 and 5xx with backoff;
//   - it reads at most MaxBytes and decodes the charset to UTF-8.
type Fetcher struct {
	Client       *http.Client
	UserAgent    string
	Delay        time.Duration // per host; 0 is 1s
	MaxBytes     int64         // 0 is 4 MiB
	Retries      int           // extra attempts; 0 is 2
	IgnoreRobots bool          // tests and public APIs (see avatar.NewGetters)

	mu    sync.Mutex
	hosts map[string]*hostState
	cache map[string]validators
}

type hostState struct {
	mu       sync.Mutex // serializes requests to the host
	last     time.Time
	robots   *robots
	robotsAt time.Time
}

type validators struct{ etag, lastModified string }

// robotsVerdict is a robots.txt answer; its text follows the UI language
// when the error is made, while errors.Is still tells the two apart.
type robotsVerdict struct{ denied bool }

func (v robotsVerdict) Error() string {
	if v.denied {
		return locales.CrawlRobotsDenied()
	}
	return locales.CrawlRobotsUnavailable()
}

// ErrDisallowed reports a page robots.txt forbids.
var ErrDisallowed = robotsVerdict{denied: true}

// ErrRobotsUnavailable reports a page that cannot be read only because its
// robots.txt is temporarily unavailable (a 5xx, as a site under maintenance,
// or a network error). RFC 9309 says to crawl nothing until it can be read:
// it is retried sooner than an ordinary robots.txt.
var ErrRobotsUnavailable = robotsVerdict{}

// retryError is an error written out in the UI language that still matches
// its sentinel under errors.Is.
type retryError struct {
	target error
	msg    string
}

func (e retryError) Error() string { return e.msg }
func (e retryError) Unwrap() error { return e.target }

const (
	robotsCacheTTL = 24 * time.Hour  // a robots.txt that was read
	robotsRetryTTL = 5 * time.Minute // a robots.txt that could not be read
)

// NewFetcher returns a fetcher with the default settings.
func NewFetcher() *Fetcher {
	return &Fetcher{Client: &http.Client{Timeout: 20 * time.Second}}
}

func (f *Fetcher) host(h string) *hostState {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.hosts == nil {
		f.hosts = map[string]*hostState{}
	}
	if f.cache == nil {
		f.cache = map[string]validators{}
	}
	s, ok := f.hosts[h]
	if !ok {
		s = &hostState{}
		f.hosts[h] = s
	}
	return s
}

func (f *Fetcher) ua() string {
	if f.UserAgent != "" {
		return f.UserAgent
	}
	return UserAgent
}

// Get implements sources.Getter.
func (f *Fetcher) Get(ctx context.Context, raw string) (*sources.Response, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, errors.New(locales.CrawlBadUrl(raw))
	}
	hs := f.host(u.Host)
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if !f.IgnoreRobots {
		ttl := robotsCacheTTL
		if hs.robots != nil && hs.robots.unavailable {
			ttl = robotsRetryTTL
		}
		if hs.robots == nil || time.Since(hs.robotsAt) > ttl {
			hs.robots = f.loadRobots(ctx, hs, u)
			hs.robotsAt = time.Now()
		}
		if !hs.robots.allowed(u.RequestURI()) {
			if hs.robots.unavailable {
				reason := locales.CrawlUnreadable()
				if hs.robots.status != 0 {
					reason = fmt.Sprintf("HTTP %d", hs.robots.status)
				}
				msg := locales.CrawlRetry(locales.CrawlRobotsUnavailable(), reason, raw)
				return nil, retryError{target: ErrRobotsUnavailable, msg: msg}
			}
			return nil, retryError{target: ErrDisallowed, msg: locales.CrawlRobotsDenied() + "：" + raw}
		}
	}

	f.mu.Lock()
	v := f.cache[raw]
	f.mu.Unlock()
	var resp *http.Response
	var body []byte
	retries := f.Retries
	if retries == 0 {
		retries = 2
	}
	for attempt := 0; ; attempt++ {
		if err := f.wait(ctx, hs); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", f.ua())
		req.Header.Set("Accept-Language", "ja,en;q=0.5")
		if v.etag != "" {
			req.Header.Set("If-None-Match", v.etag)
		}
		if v.lastModified != "" {
			req.Header.Set("If-Modified-Since", v.lastModified)
		}
		resp, err = f.Client.Do(req)
		hs.last = time.Now()
		retry := err != nil || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !retry {
			body, err = f.read(resp)
			resp.Body.Close()
			if err != nil {
				return nil, err
			}
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if attempt >= retries || ctx.Err() != nil {
			if err != nil {
				return nil, errors.New(locales.CrawlNetwork(err.Error()))
			}
			return nil, errors.New(locales.CrawlStatus(resp.Status))
		}
		backoff := time.Duration(1<<attempt) * time.Second
		if resp != nil {
			if s := resp.Header.Get("Retry-After"); s != "" {
				var n int
				if _, e := fmt.Sscanf(s, "%d", &n); e == nil && n > 0 && n <= 120 {
					backoff = time.Duration(n) * time.Second
				}
			}
		}
		if err := sleepCtx(ctx, backoff); err != nil {
			return nil, err
		}
	}

	out := &sources.Response{URL: raw, Status: resp.StatusCode}
	switch {
	case resp.StatusCode == http.StatusNotModified:
		out.NotModified = true
		out.ETag, out.LastModified = v.etag, v.lastModified
		return out, nil
	case resp.StatusCode >= 400:
		return nil, errors.New(locales.CrawlStatus(resp.Status))
	}
	out.Body = body
	out.Text = decode(body, resp.Header.Get("Content-Type"))
	nv := validators{etag: resp.Header.Get("ETag"), lastModified: resp.Header.Get("Last-Modified")}
	if nv.etag == "" && nv.lastModified == "" {
		nv = v
	}
	out.ETag, out.LastModified = nv.etag, nv.lastModified
	f.mu.Lock()
	if nv.etag != "" || nv.lastModified != "" {
		f.cache[raw] = nv
	}
	f.mu.Unlock()
	return out, nil
}

// SetValidators seeds the conditional-request cache with the validators of
// a copy stored on disk, so the next Get can answer 304 for an unchanged
// picture instead of downloading it again.
func (f *Fetcher) SetValidators(raw, etag, lastModified string) {
	if etag == "" && lastModified == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cache == nil {
		f.cache = map[string]validators{}
	}
	v := f.cache[raw]
	if etag != "" {
		v.etag = etag
	}
	if lastModified != "" {
		v.lastModified = lastModified
	}
	f.cache[raw] = v
}

func (f *Fetcher) wait(ctx context.Context, hs *hostState) error {
	d := f.Delay
	if d == 0 {
		d = time.Second
	}
	if hs.last.IsZero() {
		return nil
	}
	return sleepCtx(ctx, time.Until(hs.last.Add(d)))
}

func (f *Fetcher) read(resp *http.Response) ([]byte, error) {
	max := f.MaxBytes
	if max == 0 {
		max = 4 << 20
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, errors.New(locales.CrawlReadFailed(err.Error()))
	}
	if int64(len(b)) > max {
		return nil, errors.New(locales.CrawlTooLarge(int(max >> 20)))
	}
	return b, nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
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

var (
	xmlEncRe  = regexp.MustCompile(`^<\?xml[^>]*encoding=["']([A-Za-z0-9_\-]+)["']`)
	metaEncRe = regexp.MustCompile(`(?i)<meta[^>]+charset=["']?([A-Za-z0-9_\-]+)`)
)

// decode converts a body to UTF-8 using the Content-Type, the XML
// declaration or a <meta charset>; it assumes UTF-8 otherwise.
func decode(b []byte, contentType string) string {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	label := ""
	if i := strings.Index(strings.ToLower(contentType), "charset="); i >= 0 {
		label = strings.Trim(contentType[i+len("charset="):], `"' ;`)
	}
	head := b
	if len(head) > 2048 {
		head = head[:2048]
	}
	isUTF8 := func(l string) bool { return strings.EqualFold(l, "utf-8") || strings.EqualFold(l, "utf8") }
	if label == "" || (isUTF8(label) && !utf8.Valid(b)) {
		label = ""
		if m := xmlEncRe.FindSubmatch(head); m != nil {
			label = string(m[1])
		} else if m := metaEncRe.FindSubmatch(head); m != nil {
			label = string(m[1])
		}
	}
	if label == "" || isUTF8(label) {
		return string(b)
	}
	enc, err := htmlindex.Get(label)
	if err != nil {
		return string(b)
	}
	out, err := enc.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(out)
}
