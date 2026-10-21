// Package dedup keeps each announcement once and folds the announcements
// that describe the same show or ticket round into one record.
package dedup

import (
	"sort"
	"sync"
	"time"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/normalize"
)

// Outcome is what storing an announcement did.
type Outcome int

const (
	Unchanged Outcome = iota
	New
	Updated
)

// Store holds announcements keyed by their normalized URL.
type Store struct {
	mu    sync.Mutex
	byKey map[string]*domain.PublicAnnouncement
}

// NewStore returns an empty store.
func NewStore() *Store { return &Store{byKey: map[string]*domain.PublicAnnouncement{}} }

func key(a domain.PublicAnnouncement) string {
	if a.URL != "" {
		return normalize.URL(a.URL)
	}
	return a.SourceID + ":" + a.ID
}

// Put stores a, reporting whether it is new, changed or the same as the
// stored copy. A title-only copy never replaces a stored body.
func (s *Store) Put(a domain.PublicAnnouncement) Outcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(a)
	if a.ID == "" {
		a.ID = normalize.ID("n-", k)
	}
	if a.ContentHash == "" {
		a.ContentHash = normalize.Hash(a.Title + "\n" + a.RawText)
	}
	old, ok := s.byKey[k]
	if !ok {
		if a.Revision == 0 { // a copy restored from the cache keeps its revision
			a.Revision = 1
		}
		s.byKey[k] = &a
		return New
	}
	if a.FetchedAt.After(old.FetchedAt) {
		old.FetchedAt = a.FetchedAt
	}
	if a.ContentHash == old.ContentHash || (a.MetadataOnly && !old.MetadataOnly) {
		return Unchanged
	}
	a.ID = old.ID
	a.Revision = old.Revision + 1
	if a.PublishedAt.IsZero() {
		a.PublishedAt = old.PublishedAt
	}
	s.byKey[k] = &a
	return Updated
}

// Has reports whether the full text of url is stored.
func (s *Store) Has(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.byKey[normalize.URL(url)]
	return ok && !a.MetadataOnly
}

// All returns the announcements oldest first.
func (s *Store) All() []domain.PublicAnnouncement {
	s.mu.Lock()
	out := make([]domain.PublicAnnouncement, 0, len(s.byKey))
	for _, a := range s.byKey {
		out = append(out, *a)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if !out[i].PublishedAt.Equal(out[j].PublishedAt) {
			return out[i].PublishedAt.Before(out[j].PublishedAt)
		}
		return out[i].URL < out[j].URL
	})
	return out
}

// Count returns how many announcements each source has.
func (s *Store) Count() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]int{}
	for _, a := range s.byKey {
		out[a.SourceID]++
	}
	return out
}

func published(a domain.PublicAnnouncement) time.Time {
	if !a.PublishedAt.IsZero() {
		return a.PublishedAt
	}
	return a.FetchedAt
}
