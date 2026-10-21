package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/text/encoding/japanese"

	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/sources"
)

func TestFetcherPoliteness(t *testing.T) {
	var hits, flaky atomic.Int32
	sjis, _ := japanese.ShiftJIS.NewEncoder().String("<html><head><meta charset=\"Shift_JIS\"></head>ライヴ情報</html>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("User-Agent") == "" || !strings.HasPrefix(r.Header.Get("User-Agent"), "LiveCalendar/") {
			t.Errorf("user agent = %q", r.Header.Get("User-Agent"))
		}
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nDisallow: /private/\nAllow: /private/open\n"))
		case "/feed":
			if r.Header.Get("If-None-Match") == `"v1"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"v1"`)
			w.Write([]byte("<rss/>"))
		case "/sjis":
			w.Write([]byte(sjis))
		case "/flaky":
			if flaky.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Write([]byte("ok"))
		case "/private/open":
			w.Write([]byte("open"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), Delay: 50 * time.Millisecond}
	ctx := context.Background()

	start := time.Now()
	r, err := f.Get(ctx, srv.URL+"/feed")
	if err != nil || r.Text != "<rss/>" {
		t.Fatalf("feed = %+v %v", r, err)
	}
	r, err = f.Get(ctx, srv.URL+"/feed")
	if err != nil || !r.NotModified {
		t.Fatalf("second feed should be 304: %+v %v", r, err)
	}
	seeded := &Fetcher{Client: srv.Client(), Delay: 10 * time.Millisecond}
	seeded.SetValidators(srv.URL+"/feed", `"v1"`, "")
	if r, err := seeded.Get(ctx, srv.URL+"/feed"); err != nil || !r.NotModified || r.ETag != `"v1"` {
		t.Errorf("seeded validator: %+v %v", r, err)
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Error("requests to one host must be spaced by Delay")
	}
	if _, err := f.Get(ctx, srv.URL+"/private/x"); !errors.Is(err, ErrDisallowed) {
		t.Errorf("disallowed page: %v", err)
	}
	if r, err := f.Get(ctx, srv.URL+"/private/open"); err != nil || r.Text != "open" {
		t.Errorf("allowed page: %v", err)
	}
	if r, err := f.Get(ctx, srv.URL+"/sjis"); err != nil || !strings.Contains(r.Text, "ライヴ情報") {
		t.Errorf("shift_jis = %q %v", r.Text, err)
	}
	f.Retries = 1
	if r, err := f.Get(ctx, srv.URL+"/flaky"); err != nil || r.Text != "ok" {
		t.Errorf("retry: %+v %v", r, err)
	}
	if _, err := f.Get(ctx, srv.URL+"/missing"); err == nil {
		t.Error("404 should fail")
	}
}

func TestRobotsGroups(t *testing.T) {
	r := parseRobots(strings.NewReader("User-agent: Googlebot\nDisallow: /\n\nUser-agent: *\nDisallow: /magazine/pdf/\n"), "livecalendar")
	if !r.allowed("/news/index.xml") || r.allowed("/magazine/pdf/1.pdf") {
		t.Errorf("rules = %+v", r)
	}
	r = parseRobots(strings.NewReader("User-agent: LiveCalendar\nDisallow: /\nUser-agent: *\nAllow: /\n"), "livecalendar")
	if r.allowed("/news/") {
		t.Error("our own group must apply")
	}
	w := parseRobots(strings.NewReader("User-agent: *\nAllow: /robots.txt\nDisallow: /*\nAllow: /pub/*.jpg$\n"), "livecalendar")
	for path, want := range map[string]bool{"/robots.txt": true, "/search/artist?q=x": false, "/": false, "/pub/a.jpg": true, "/pub/a.jpg?x": false} {
		if w.allowed(path) != want {
			t.Errorf("wildcards: allowed(%q) = %v, want %v", path, !want, want)
		}
	}
}

type fakeAdapter func(src domain.SourceConfig) (sources.Result, error)

func (f fakeAdapter) Fetch(_ context.Context, src domain.SourceConfig, _ sources.Env) (sources.Result, error) {
	return f(src)
}

func TestSchedulerIsolatesSources(t *testing.T) {
	cat := catalog.New(dedup.Options{})
	srcs := []domain.SourceConfig{
		{ID: "ok", Kind: "good", Enabled: true, Interval: time.Hour},
		{ID: "bad", Kind: "bad", Enabled: true},
		{ID: "boom", Kind: "boom", Enabled: true},
		{ID: "off", Kind: "good", Enabled: false},
	}
	s := New(cat, srcs)
	var changes, synced atomic.Int32
	s.OnChange = func() { changes.Add(1) }
	s.AfterSync = func() { synced.Add(1) }
	s.Adapters = map[domain.SourceKind]sources.Adapter{
		"good": fakeAdapter(func(src domain.SourceConfig) (sources.Result, error) {
			return sources.Result{Announcements: []domain.PublicAnnouncement{{
				URL: "https://x.example/" + src.ID, Title: "X、ワンマン開催",
				RawText: "▼ライヴ情報\n12月5日(土) 東京 渋谷WWW", PublishedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, domain.Tokyo),
			}}}, nil
		}),
		"bad": fakeAdapter(func(domain.SourceConfig) (sources.Result, error) {
			return sources.Result{}, errors.New("服务器返回 503")
		}),
		"boom": fakeAdapter(func(domain.SourceConfig) (sources.Result, error) { panic("parser bug") }),
	}
	runs := s.RunOnce(context.Background())
	if len(runs) != 3 {
		t.Fatalf("runs = %+v", runs)
	}
	want := map[string]domain.SyncState{"ok": domain.SyncSucceeded, "bad": domain.SyncFailed, "boom": domain.SyncFailed}
	for _, r := range runs {
		if r.State != want[r.SourceID] {
			t.Errorf("%s = %s (%s)", r.SourceID, r.State, r.Err)
		}
	}
	if snap := cat.Snapshot(); len(snap.Events) != 1 || snap.Events[0].SourceIDs[0] != "ok" {
		t.Fatalf("catalog = %+v", snap.Events)
	}
	if changes.Load() < 2 {
		t.Error("OnChange not called")
	}
	if synced.Load() != 1 {
		t.Errorf("AfterSync called %d times, want 1", synced.Load())
	}
	if len(s.Log()) != 3 {
		t.Errorf("log = %d", len(s.Log()))
	}
	// A second run of the same content changes nothing.
	again := s.RunOnce(context.Background(), "ok")
	if again[0].New != 0 || again[0].Fetched != 1 {
		t.Errorf("rerun = %+v", again[0])
	}
	if synced.Load() != 2 {
		t.Errorf("AfterSync called %d times, want 2", synced.Load())
	}
	if due := s.Due(time.Now()); len(due) != 0 {
		t.Errorf("ok was just synced; due = %v", due)
	}
	if due := s.Due(time.Now().Add(2 * time.Hour)); len(due) != 1 || due[0] != "ok" {
		t.Errorf("due later = %v", due)
	}
	s.SetEnabled("ok", false)
	if len(cat.Snapshot().Events) != 0 {
		t.Error("disabled source should be hidden")
	}
	s.SetEnabled("ok", true)
	if len(cat.Snapshot().Events) != 1 {
		t.Error("re-enabled source should show again")
	}
}

func TestSchedulerAdd(t *testing.T) {
	cat := catalog.New(dedup.Options{})
	s := New(cat, nil)
	now := time.Date(2026, 10, 21, 22, 0, 0, 0, domain.Tokyo)
	s.Now = func() time.Time { return now }
	src := domain.SourceConfig{ID: "added", Name: "新增来源", Kind: domain.SourceOfficialSite, URL: "https://example.com/live", Enabled: true, Interval: 6 * time.Hour}
	s.Add(src)
	if sts := s.Statuses(); len(sts) != 1 || sts[0].Config.ID != "added" {
		t.Fatalf("statuses = %+v", sts)
	}
	if due := s.Due(now); len(due) != 1 || due[0] != "added" {
		t.Errorf("a new source is not due: %v", due)
	}
	s.Add(src) // the same ID keeps the source it has
	if sts := s.Statuses(); len(sts) != 1 {
		t.Errorf("adding twice made %d sources", len(sts))
	}
	s.Remove("added")
	if sts := s.Statuses(); len(sts) != 0 {
		t.Errorf("remove left %+v", sts)
	}
}

func TestRobotsUnavailable(t *testing.T) {
	var back atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			if !back.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("メンテナンス中"))
				return
			}
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/news/":
			w.Write([]byte("page"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	f := &Fetcher{Client: srv.Client(), Delay: 10 * time.Millisecond}
	ctx := context.Background()
	_, err := f.Get(ctx, srv.URL+"/news/")
	if !errors.Is(err, ErrRobotsUnavailable) {
		t.Fatalf("a 503 robots.txt: %v", err)
	}
	if errors.Is(err, ErrDisallowed) {
		t.Errorf("a maintenance page is not a chosen rule: %v", err)
	}
	if !strings.Contains(err.Error(), "维护中") {
		t.Errorf("the error should name the maintenance, not a rule: %v", err)
	}
	hs := f.host(strings.TrimPrefix(srv.URL, "http://"))
	if hs.robots == nil || !hs.robots.unavailable || hs.robots.status != http.StatusServiceUnavailable {
		t.Fatalf("robots = %+v", hs.robots)
	}
	// A 503 is retried soon, not kept for a day.
	hs.robotsAt = time.Now().Add(-robotsRetryTTL - time.Second)
	back.Store(true)
	r, err := f.Get(ctx, srv.URL+"/news/")
	if err != nil || r.Text != "page" {
		t.Fatalf("after the maintenance: %+v %v", r, err)
	}
	if hs.robots.unavailable {
		t.Error("the readable robots.txt is still marked unavailable")
	}
}

func TestSchedulerRetriesTemporaryFailure(t *testing.T) {
	now := time.Date(2026, 10, 21, 22, 0, 0, 0, domain.Tokyo)
	cat := catalog.New(dedup.Options{})
	s := New(cat, []domain.SourceConfig{{ID: "m", Kind: "m", Enabled: true, Interval: time.Hour}})
	s.Now = func() time.Time { return now }
	var calls atomic.Int32
	s.Adapters = map[domain.SourceKind]sources.Adapter{
		"m": fakeAdapter(func(domain.SourceConfig) (sources.Result, error) {
			calls.Add(1)
			return sources.Result{}, fmt.Errorf("%w（HTTP 503，站点可能维护中）", ErrRobotsUnavailable)
		}),
	}
	s.RunOnce(context.Background())
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
	st := s.Statuses()[0]
	if st.State != domain.SyncFailed || !st.LastRun.Retry {
		t.Fatalf("status = %+v", st)
	}
	if want := now.Add(5 * time.Minute); !st.Next.Equal(want) {
		t.Errorf("next retry = %v, want %v", st.Next, want)
	}
	if due := s.Due(now.Add(4 * time.Minute)); len(due) != 0 {
		t.Errorf("retried too early: %v", due)
	}
	if due := s.Due(now.Add(5 * time.Minute)); len(due) != 1 || due[0] != "m" {
		t.Errorf("not due at the retry time: %v", due)
	}
	// A second temporary failure backs off to ten minutes.
	now = now.Add(5 * time.Minute)
	s.RunOnce(context.Background(), "m")
	st = s.Statuses()[0]
	if want := now.Add(10 * time.Minute); !st.Next.Equal(want) {
		t.Errorf("second retry = %v, want %v", st.Next, want)
	}
}
