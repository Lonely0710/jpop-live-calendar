// Package crawler syncs the configured public sources into the catalog.
//
// Every source runs in its own goroutine with its own timeout and a
// recover, so a failing, slow or panicking source never stops the others
// and never touches the data the others brought. The UI never fetches: it
// asks the scheduler to run and reads the catalog's snapshot.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/sources"
	"jpop-live-calendar/locales"
)

// SourceStatus is what the UI shows about a source.
type SourceStatus struct {
	Config      domain.SourceConfig
	State       domain.SyncState
	LastRun     domain.SyncRun
	LastSuccess time.Time
	Warnings    []string
	Next        time.Time // next automatic sync, or the next retry after a temporary failure

	failures int // consecutive temporary failures, for the retry backoff
}

// Scheduler runs sources into a catalog.
type Scheduler struct {
	Catalog  *catalog.Catalog
	Adapters map[domain.SourceKind]sources.Adapter
	Getter   sources.Getter
	Now      func() time.Time
	Timeout  time.Duration // per source; 0 is 2 minutes
	OnChange func()        // called after any state change, from any goroutine

	// AfterSync, when set, is called at the end of every RunOnce that ran a
	// source, from a background goroutine: the cache is saved here.
	AfterSync func()

	mu      sync.Mutex
	order   []string
	status  map[string]*SourceStatus
	log     []domain.SyncRun
	running map[string]bool
}

// New returns a scheduler for the sources.
func New(cat *catalog.Catalog, srcs []domain.SourceConfig) *Scheduler {
	s := &Scheduler{
		Catalog:  cat,
		Adapters: sources.Default(),
		Getter:   NewFetcher(),
		status:   map[string]*SourceStatus{},
		running:  map[string]bool{},
	}
	for _, c := range srcs {
		s.order = append(s.order, c.ID)
		s.status[c.ID] = &SourceStatus{Config: c, State: domain.SyncIdle}
	}
	return s
}

func (s *Scheduler) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Scheduler) changed() {
	if s.OnChange != nil {
		s.OnChange()
	}
}

// Statuses returns every source's status in configuration order.
func (s *Scheduler) Statuses() []SourceStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SourceStatus, 0, len(s.order))
	for _, id := range s.order {
		st := *s.status[id]
		st.Warnings = append([]string(nil), st.Warnings...)
		out = append(out, st)
	}
	return out
}

// Log returns the latest sync runs, newest first.
func (s *Scheduler) Log() []domain.SyncRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.SyncRun, len(s.log))
	for i, r := range s.log {
		out[len(s.log)-1-i] = r
	}
	return out
}

// Running reports whether any source is syncing.
func (s *Scheduler) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.running) > 0
}

// Enabled reports whether a source is on.
func (s *Scheduler) Enabled(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.status[id]
	return ok && st.Config.Enabled
}

// SetEnabled turns a source on or off; its stored announcements stay but
// are hidden while it is off.
func (s *Scheduler) SetEnabled(id string, on bool) {
	s.mu.Lock()
	if st, ok := s.status[id]; ok {
		st.Config.Enabled = on
		if !on {
			st.State = domain.SyncSkipped
		} else if st.State == domain.SyncSkipped {
			st.State = domain.SyncIdle
		}
	}
	s.mu.Unlock()
	s.Catalog.SetInclude(s.Enabled)
	s.changed()
}

// Add appends a source to the scheduler, as the 数据源 page's form does;
// the next RunOnce reads it. Adding an existing ID keeps the old source.
func (s *Scheduler) Add(cfg domain.SourceConfig) {
	s.mu.Lock()
	if _, ok := s.status[cfg.ID]; ok {
		s.mu.Unlock()
		return
	}
	s.order = append(s.order, cfg.ID)
	s.status[cfg.ID] = &SourceStatus{Config: cfg, State: domain.SyncIdle}
	s.mu.Unlock()
	s.Catalog.SetInclude(s.Enabled)
	s.changed()
}

// Remove deletes a source: it no longer shows, syncs or brings
// announcements to the catalog, which drops the ones it brought.
func (s *Scheduler) Remove(id string) {
	s.mu.Lock()
	if _, ok := s.status[id]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.status, id)
	for i, x := range s.order {
		if x == id {
			s.order = append(s.order[:i:i], s.order[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	s.Catalog.SetInclude(s.Enabled)
	s.changed()
}

// Due returns the enabled sources whose interval has passed, or whose
// temporary failure is due for a retry.
func (s *Scheduler) Due(now time.Time) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, id := range s.order {
		st := s.status[id]
		if !st.Config.Enabled || s.running[id] || st.Config.Interval <= 0 {
			continue
		}
		if !st.Next.IsZero() {
			if !now.Before(st.Next) {
				out = append(out, id)
			}
			continue
		}
		if st.LastRun.FinishedAt.IsZero() || !now.Before(st.LastRun.FinishedAt.Add(st.Config.Interval)) {
			out = append(out, id)
		}
	}
	return out
}

// retryBackoff is how long a temporary failure waits before its next
// attempt: 5 minutes, doubling to an hour at most, never beyond the
// source's own interval.
func retryBackoff(failures int, interval time.Duration) time.Duration {
	d := 5 * time.Minute
	for i := 1; i < failures && d < time.Hour; i++ {
		d *= 2
	}
	if d > time.Hour {
		d = time.Hour
	}
	if interval > 0 && d > interval {
		d = interval
	}
	return d
}

// RunOnce syncs the given sources (every enabled one when none is given)
// concurrently, rebuilds the catalog and returns the runs in order.
func (s *Scheduler) RunOnce(ctx context.Context, ids ...string) []domain.SyncRun {
	s.mu.Lock()
	if len(ids) == 0 {
		ids = s.order
	}
	var todo []domain.SourceConfig
	for _, id := range ids {
		st, ok := s.status[id]
		if !ok || !st.Config.Enabled || s.running[id] {
			continue
		}
		s.running[id] = true
		st.State = domain.SyncRunning
		todo = append(todo, st.Config)
	}
	s.mu.Unlock()
	if len(todo) == 0 {
		return nil
	}
	s.changed()

	runs := make([]domain.SyncRun, len(todo))
	var wg sync.WaitGroup
	for i, src := range todo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runs[i] = s.runSource(ctx, src)
		}()
	}
	wg.Wait()
	s.Catalog.SetInclude(s.Enabled) // rebuilds

	s.mu.Lock()
	for _, r := range runs {
		st := s.status[r.SourceID]
		delete(s.running, r.SourceID)
		if st == nil {
			continue // removed while it ran
		}
		st.LastRun, st.State = r, r.State
		switch {
		case r.State == domain.SyncSucceeded:
			st.LastSuccess = r.FinishedAt
			st.failures = 0
			if st.Config.Interval > 0 {
				st.Next = r.FinishedAt.Add(st.Config.Interval)
			} else {
				st.Next = time.Time{}
			}
		case r.Retry && st.Config.Interval > 0:
			// A robots.txt that could not be read, as on a site under
			// maintenance: retry soon, backing off while it stays down.
			st.failures++
			st.Next = r.FinishedAt.Add(retryBackoff(st.failures, st.Config.Interval))
		default:
			st.failures = 0
			st.Next = time.Time{}
		}
		s.log = append(s.log, r)
	}
	if len(s.log) > 50 {
		s.log = s.log[len(s.log)-50:]
	}
	s.mu.Unlock()
	s.changed()
	if s.AfterSync != nil {
		s.AfterSync()
	}
	return runs
}

func (s *Scheduler) runSource(ctx context.Context, src domain.SourceConfig) (run domain.SyncRun) {
	run = domain.SyncRun{SourceID: src.ID, StartedAt: s.now()}
	var warnings []string
	defer func() {
		if r := recover(); r != nil {
			run.State, run.Err = domain.SyncFailed, locales.CrawlInternal(fmt.Sprint(r))
		}
		run.FinishedAt = s.now()
		s.mu.Lock()
		if st := s.status[src.ID]; st != nil {
			st.Warnings = warnings
		}
		s.mu.Unlock()
	}()
	ad := s.Adapters[src.Kind]
	if ad == nil {
		run.State, run.Err = domain.SyncFailed, locales.CrawlUnsupported(string(src.Kind))
		return run
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := ad.Fetch(cctx, src, sources.Env{Get: s.Getter, Known: s.Catalog.Has, Now: s.now})
	warnings = res.Warnings
	if err != nil {
		run.State, run.Err = domain.SyncFailed, cleanErr(err)
		run.Retry = errors.Is(err, ErrRobotsUnavailable)
		return run
	}
	for i := range res.Announcements {
		res.Announcements[i].SourceID = src.ID
	}
	st := s.Catalog.Ingest(res.Announcements)
	run.State, run.Fetched, run.New, run.Updated, run.NotModified = domain.SyncSucceeded, st.Fetched, st.New, st.Updated, res.NotModified
	return run
}

func cleanErr(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "context deadline exceeded") {
		return locales.CrawlTimeout()
	}
	return msg
}

// Start syncs every enabled source now and then each when its interval
// has passed, until ctx is done.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		s.RunOnce(ctx)
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if due := s.Due(s.now()); len(due) > 0 {
					s.RunOnce(ctx, due...)
				}
			}
		}
	}()
}
