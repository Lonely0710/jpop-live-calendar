// Package avatar finds each artist's picture and takes their theme color
// from it: the color of the picture's largest block of one color.
//
// Pictures are looked up by name in Deezer's public artist search API (no
// key needed) unless the configuration gives one. The search is an API call,
// not page crawling: it goes through its own rate-limited Getter (Lookup),
// while the pictures themselves are downloaded through the polite crawler
// Getter (robots.txt, one request per host at a time). The pictures are
// kept in memory; the app may save them in internal/cache's JSON snapshot.
package avatar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // decoders of the pictures
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"sync"
	"time"

	_ "golang.org/x/image/webp"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/normalize"
	"jpop-live-calendar/internal/sources"
)

// Avatar is an artist's picture and theme color.
type Avatar struct {
	ArtistID string
	URL      string // the picture's address
	Source   string // where it came from: "Deezer" or "配置"
	Image    image.Image
	Color    color.RGBA // the theme color
	Share    float64    // of the picture the color's block covers
	Manual   bool       // Color was configured, not taken from the picture
	Err      string     // why there is no picture
	// The cache uses these to avoid downloading and replacing a picture
	// that did not change: the HTTP validators of the copy and the hash of
	// its bytes.
	ETag         string
	LastModified string
	Hash         string
	FetchedAt    time.Time
}

// Store holds the avatars found so far; it is safe for concurrent use.
type Store struct {
	mu      sync.RWMutex
	m       map[string]Avatar
	version int
	running bool

	// OnChange is called (on the fetching goroutine) after each avatar.
	OnChange func()
}

// NewStore returns an empty store.
func NewStore() *Store { return &Store{m: map[string]Avatar{}} }

// Get returns the avatar of an artist.
func (s *Store) Get(id string) (Avatar, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.m[id]
	return a, ok
}

// Version changes whenever an avatar is added.
func (s *Store) Version() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// Running reports whether Fetch is running.
func (s *Store) Running() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Restore fills the store with avatars loaded from the disk cache, before
// the first frame. It does not call OnChange: there is nothing to redraw
// yet.
func (s *Store) Restore(m map[string]Avatar) {
	s.mu.Lock()
	if s.m == nil {
		s.m = map[string]Avatar{}
	}
	for id, a := range m {
		s.m[id] = a
	}
	s.version++
	s.mu.Unlock()
}

// Snapshot returns a copy of the avatars, for the disk cache.
func (s *Store) Snapshot() map[string]Avatar {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Avatar, len(s.m))
	for id, a := range s.m {
		out[id] = a
	}
	return out
}

func (s *Store) put(a Avatar) {
	s.mu.Lock()
	s.m[a.ArtistID] = a
	s.version++
	f := s.OnChange
	s.mu.Unlock()
	if f != nil {
		f()
	}
}

// ArtistID is the catalog's identifier of a configured artist.
func ArtistID(a domain.Artist) string {
	if a.ID != "" {
		return a.ID
	}
	return normalize.ID("a-", normalize.Key(a.Name))
}

// Getters are what Fetch reads through.
type Getters struct {
	Lookup   sources.Getter // Deezer search API; nil turns lookups off
	Pictures sources.Getter // picture downloads
}

// Fetch finds the avatars of the artists that have none yet, one after
// another. Artists with a configured color and no picture keep the color.
func (s *Store) Fetch(ctx context.Context, g Getters, artists []domain.Artist) bool {
	return s.fetch(ctx, g, artists, false)
}

// Revalidate checks the pictures already stored for changes, with
// conditional requests: an unchanged picture (304 or the same bytes) keeps
// its cached copy, so only a real update replaces it.
func (s *Store) Revalidate(ctx context.Context, g Getters, artists []domain.Artist) bool {
	return s.fetch(ctx, g, artists, true)
}

// fetch returns whether it ran: a pass skipped because another one is
// running must not make the caller save a partial snapshot.
func (s *Store) fetch(ctx context.Context, g Getters, artists []domain.Artist, refresh bool) bool {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return false
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()
	for _, ar := range artists {
		if ctx.Err() != nil {
			return true
		}
		id := ArtistID(ar)
		old, ok := s.Get(id)
		if refresh {
			if !ok || old.Image == nil || old.URL == "" {
				continue
			}
			// The configuration may have changed since the cache was
			// written: fetch the new URL (or no picture) instead of
			// revalidating the old one.
			if ar.AvatarURL == "none" ||
				(ar.AvatarURL != "" && ar.AvatarURL != old.URL) ||
				(ar.AvatarURL == "" && old.Source == "配置") {
				refresh = false
			}
		} else if ok && old.Err == "" {
			continue
		}
		next := fetchOne(ctx, g, ar, id, old, refresh)
		if refresh && next.Hash == old.Hash && next.Err == old.Err {
			continue // unchanged: keep the cached picture, no redraw
		}
		s.put(next)
	}
	return true
}

// Fallback is the theme color of an artist without a picture: a fixed
// hue chosen by the name, so it stays the same between launches.
func Fallback(name string) color.RGBA {
	var h uint32 = 2166136261
	for _, b := range []byte(normalize.Key(name)) {
		h = (h ^ uint32(b)) * 16777619
	}
	return hsl(float64(h%360), 0.55, 0.48)
}

func hsl(h, s, l float64) color.RGBA {
	c := (1 - abs(2*l-1)) * s
	x := c * (1 - abs(mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	f := func(v float64) uint8 { return uint8((v+m)*255 + 0.5) }
	return color.RGBA{f(r), f(g), f(b), 255}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func mod(a, b float64) float64 { return a - b*float64(int(a/b)) }

func fetchOne(ctx context.Context, g Getters, ar domain.Artist, id string, old Avatar, refresh bool) Avatar {
	if refresh {
		return revalidateOne(ctx, g, ar, old)
	}
	av := Avatar{
		ArtistID: id, URL: ar.AvatarURL, Source: "配置", Color: Fallback(ar.Name),
		FetchedAt: time.Now(),
	}
	if c, ok := parseHex(ar.Color); ok {
		av.Color, av.Manual = c, true
	}
	if av.URL == "none" {
		av.URL, av.Err = "", "配置为不使用头像"
		return av
	}
	if av.URL == "" {
		if g.Lookup == nil {
			av.Err = "未配置头像（avatars: off）"
			return av
		}
		u, err := deezer(ctx, g.Lookup, ar)
		if err != nil {
			av.Err = err.Error()
			return av
		}
		av.URL, av.Source = u, "Deezer"
	}
	if old.URL == av.URL {
		seedValidators(g.Pictures, av.URL, old.ETag, old.LastModified)
	}
	resp, err := g.Pictures.Get(ctx, av.URL)
	if err != nil {
		av.Err = "头像下载失败：" + err.Error()
		return av
	}
	if resp.NotModified {
		if old.Image != nil {
			old.Err, old.FetchedAt = "", time.Now()
			return old
		}
		av.Err = "头像下载失败：内容为空"
		return av
	}
	if len(resp.Body) == 0 {
		av.Err = "头像下载失败：内容为空"
		return av
	}
	img, _, err := image.Decode(bytes.NewReader(resp.Body))
	if err != nil {
		av.Err = "头像无法解码：" + err.Error()
		return av
	}
	av.Image = img
	av.Hash = hashBytes(resp.Body)
	av.ETag, av.LastModified = resp.ETag, resp.LastModified
	if !av.Manual {
		av.Color, av.Share = Dominant(img)
	}
	return av
}

// revalidateOne asks whether a cached picture changed, with its validators.
// It keeps the cached image unless the server sent different bytes.
func revalidateOne(ctx context.Context, g Getters, ar domain.Artist, old Avatar) Avatar {
	av := old
	av.Err = ""
	av.FetchedAt = time.Now()
	if c, ok := parseHex(ar.Color); ok {
		av.Color, av.Manual = c, true
	} else if av.Manual {
		av.Manual = false
		if av.Image != nil {
			av.Color, av.Share = Dominant(av.Image)
		}
	}
	seedValidators(g.Pictures, av.URL, old.ETag, old.LastModified)
	resp, err := g.Pictures.Get(ctx, av.URL)
	if err != nil {
		av.Err = "头像下载失败：" + err.Error()
		return av
	}
	if resp.NotModified {
		av.ETag, av.LastModified = resp.ETag, resp.LastModified
		return av
	}
	if len(resp.Body) == 0 {
		av.Err = "头像下载失败：内容为空"
		return av
	}
	av.ETag, av.LastModified = resp.ETag, resp.LastModified
	hash := hashBytes(resp.Body)
	if av.Hash != "" && hash == av.Hash {
		return av // same picture: keep the cached one
	}
	img, _, err := image.Decode(bytes.NewReader(resp.Body))
	if err != nil {
		av.Err = "头像无法解码：" + err.Error()
		return av
	}
	av.Image = img
	av.Hash = hash
	if !av.Manual {
		av.Color, av.Share = Dominant(img)
	}
	return av
}

// seedValidators gives a fetcher the validators of the cached copy.
func seedValidators(get sources.Getter, raw, etag, lastModified string) {
	if cg, ok := get.(interface{ SetValidators(string, string, string) }); ok {
		cg.SetValidators(raw, etag, lastModified)
	}
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type deezerArtist struct {
	Name          string `json:"name"`
	Fans          int    `json:"nb_fan"`
	PictureMedium string `json:"picture_medium"`
	PictureBig    string `json:"picture_big"`
}

// deezer looks the artist up by each of their names and returns the
// picture of the best-known artist whose name is exactly one of them.
func deezer(ctx context.Context, get sources.Getter, ar domain.Artist) (string, error) {
	keys := map[string]bool{}
	names := append([]string{ar.Name}, ar.Aliases...)
	for _, n := range names {
		keys[normalize.Key(n)] = true
	}
	var best *deezerArtist
	for i, n := range names {
		if i == 6 {
			break // a sanity cap; configurations have far fewer aliases
		}
		resp, err := get.Get(ctx, "https://api.deezer.com/search/artist?limit=10&q="+url.QueryEscape(n))
		if err != nil {
			return "", fmt.Errorf("头像查询失败：%v", err)
		}
		var res struct {
			Data []deezerArtist `json:"data"`
		}
		if err := json.Unmarshal([]byte(resp.Text), &res); err != nil {
			return "", fmt.Errorf("头像查询结果无法读取：%v", err)
		}
		for j := range res.Data {
			d := &res.Data[j]
			if keys[normalize.Key(d.Name)] && d.PictureBig != "" && (best == nil || d.Fans > best.Fans) {
				best = d
			}
		}
		if best != nil && best.Fans >= 1000 {
			break // a well-known match: no need to spend more rate-limited queries
		}
	}
	if best == nil {
		return "", fmt.Errorf("Deezer 上没有找到同名艺人")
	}
	if best.PictureMedium != "" {
		return best.PictureMedium, nil // 250 px: enough for a 64 pt circle
	}
	return best.PictureBig, nil
}

func parseHex(s string) (color.RGBA, bool) {
	var r, g, b uint8
	if len(s) != 7 || s[0] != '#' {
		return color.RGBA{}, false
	}
	if _, err := fmt.Sscanf(s[1:], "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{r, g, b, 255}, true
}
