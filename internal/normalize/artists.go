package normalize

import (
	"sort"
	"strings"

	"jpop-live-calendar/internal/domain"
)

// Artists unifies artist names: configured aliases first, then every
// spelling seen, keyed by Key.
type Artists struct {
	byKey map[string]*domain.Artist
}

// NewArtists starts a registry from configured artists.
func NewArtists(known []domain.Artist) *Artists {
	r := &Artists{byKey: map[string]*domain.Artist{}}
	for _, a := range known {
		a := a
		if a.ID == "" {
			a.ID = ID("a-", Key(a.Name))
		}
		p := &a
		for _, n := range append([]string{a.Name}, a.Aliases...) {
			if k := Key(n); k != "" {
				r.byKey[k] = p
			}
		}
	}
	return r
}

// Resolve returns the artist a name means, adding it when new.
func (r *Artists) Resolve(name string) *domain.Artist {
	name = strings.TrimSpace(name)
	k := Key(name)
	if k == "" {
		return nil
	}
	if a, ok := r.byKey[k]; ok {
		if name != a.Name && !contains(a.Aliases, name) {
			a.Aliases = append(a.Aliases, name)
		}
		return a
	}
	a := &domain.Artist{ID: ID("a-", k), Name: name}
	r.byKey[k] = a
	return a
}

// Find returns the known artists named in text, longest names first.
func (r *Artists) Find(text string) []*domain.Artist {
	t := Key(text)
	keys := make([]string, 0, len(r.byKey))
	for k := range r.byKey {
		if len([]rune(k)) >= 3 {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	seen := map[string]bool{}
	var out []*domain.Artist
	for _, k := range keys {
		if strings.Contains(t, k) && !seen[r.byKey[k].ID] {
			seen[r.byKey[k].ID] = true
			out = append(out, r.byKey[k])
		}
	}
	return out
}

// All returns every artist, by name.
func (r *Artists) All() []domain.Artist {
	seen := map[string]bool{}
	var out []domain.Artist
	for _, a := range r.byKey {
		if !seen[a.ID] {
			seen[a.ID] = true
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
