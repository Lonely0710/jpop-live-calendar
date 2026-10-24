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
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // decoders of the pictures
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"regexp"
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
	// unreachable records that the last pass stopped because the lookup
	// could not reach Deezer at all (a network that needs a proxy, say),
	// which the settings page says out loud: every artist fails the same
	// way, so nothing else explains the missing pictures.
	unreachable bool

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

// Unreachable reports whether the last pass gave up because Deezer could
// not be reached at all.
func (s *Store) Unreachable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unreachable
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
	// Lookup is Deezer's artist search: a real artist photo when it has
	// one. nil skips it.
	Lookup sources.Getter
	// QQ is QQ Music's search, which has Japanese artists' photos and
	// answers from the mainland, where Deezer's API does not. nil skips it.
	QQ sources.Getter
	// Albums is Apple's iTunes search, whose covers stand in for a photo.
	// nil skips it.
	Albums   sources.Getter
	Pictures sources.Getter // picture downloads
}

// catalog is one place a picture may come from: what to ask, how, and what
// this pass has learned about reaching it.
type catalog struct {
	name string
	get  sources.Getter
	find func(context.Context, sources.Getter, domain.Artist) (string, bool, error)
	down *bool // set once the pass found this one unreachable; nil: never mark
}

// catalogs are the lookups to try, in the order the mode configured them.
func (g Getters) catalogs(state *lookupState) []catalog {
	var out []catalog
	if g.Lookup != nil {
		out = append(out, catalog{"Deezer", g.Lookup, deezer, &state.deezerDown})
	}
	if g.QQ != nil {
		out = append(out, catalog{"QQ", g.QQ, qqMusic, &state.qqDown})
	}
	if g.Albums != nil {
		out = append(out, catalog{"iTunes", g.Albums, albums, nil})
	}
	return out
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
	s.unreachable = false
	s.mu.Unlock()
	var state lookupState
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
		next, offline := fetchOne(ctx, g, ar, id, old, refresh, &state)
		if refresh && next.Hash == old.Hash && next.Err == old.Err {
			continue // unchanged: keep the cached picture, no redraw
		}
		s.put(next)
		if offline {
			// Neither catalog could be reached: the rest of the artists
			// would spend their seconds failing the same way, so the pass
			// stops here. The next pass (a launch, a sync) tries again.
			s.mu.Lock()
			s.unreachable = true
			s.mu.Unlock()
			return true
		}
	}
	return true
}

// lookupState is what one pass learns about the catalogs: once one has
// failed to answer at all (a network that cannot reach it, as from the
// mainland), the rest of the pass skips it instead of spending a request
// and a retry on every artist.
type lookupState struct {
	deezerDown bool
	qqDown     bool
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

// fetchOne finds one artist's picture. offline reports that no catalog
// could be reached at all, as opposed to none of them knowing the artist:
// the pass stops on the former.
func fetchOne(ctx context.Context, g Getters, ar domain.Artist, id string, old Avatar, refresh bool, state *lookupState) (av Avatar, offline bool) {
	if refresh {
		return revalidateOne(ctx, g, ar, old), false
	}
	av = Avatar{
		ArtistID: id, URL: ar.AvatarURL, Source: "配置", Color: Fallback(ar.Name),
		FetchedAt: time.Now(),
	}
	if c, ok := parseHex(ar.Color); ok {
		av.Color, av.Manual = c, true
	}
	if av.URL == "none" {
		av.URL, av.Err = "", "配置为不使用头像"
		return av, false
	}
	if av.URL == "" {
		pic, source, offline, err := lookup(ctx, g, ar, state)
		if err != nil {
			av.Err = err.Error()
			return av, offline
		}
		av.URL, av.Source = pic, source
	}
	if old.URL == av.URL {
		seedValidators(g.Pictures, av.URL, old.ETag, old.LastModified)
	}
	resp, err := g.Pictures.Get(ctx, av.URL)
	if err != nil {
		av.Err = "头像下载失败：" + err.Error()
		return av, false
	}
	if resp.NotModified {
		if old.Image != nil {
			old.Err, old.FetchedAt = "", time.Now()
			return old, false
		}
		av.Err = "头像下载失败：内容为空"
		return av, false
	}
	if len(resp.Body) == 0 {
		av.Err = "头像下载失败：内容为空"
		return av, false
	}
	img, _, err := image.Decode(bytes.NewReader(resp.Body))
	if err != nil {
		av.Err = "头像无法解码：" + err.Error()
		return av, false
	}
	av.Image = img
	av.Hash = hashBytes(resp.Body)
	av.ETag, av.LastModified = resp.ETag, resp.LastModified
	if !av.Manual {
		av.Color, av.Share = Dominant(img)
	}
	return av, false
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

// lookup finds a picture for the artist, trying the configured catalogs in
// order and taking the first hit: a real photo from Deezer or QQ Music, or
// else the cover of a release from iTunes. offline says none of them could
// be reached at all, which is a network's fault, not the artist's, and ends
// the pass.
func lookup(ctx context.Context, g Getters, ar domain.Artist, state *lookupState) (pic, source string, offline bool, err error) {
	cats := g.catalogs(state)
	if len(cats) == 0 {
		return "", "", false, errors.New("未配置头像（avatars: off）")
	}
	var miss, unreachable error
	for _, c := range cats {
		if c.down != nil && *c.down {
			continue
		}
		got, down, err := c.find(ctx, c.get, ar)
		if err == nil {
			return got, c.name, false, nil
		}
		if down {
			if c.down != nil {
				*c.down = true
			}
			unreachable = err
			continue
		}
		if miss == nil {
			miss = err // the first catalog that did answer, and said no
		}
	}
	switch {
	case miss != nil:
		return "", "", false, miss
	case unreachable != nil:
		return "", "", true, unreachable
	default:
		// Everything was skipped as unreachable earlier in this pass.
		return "", "", true, errors.New("头像来源都连不上")
	}
}

type deezerArtist struct {
	Name          string `json:"name"`
	Fans          int    `json:"nb_fan"`
	PictureMedium string `json:"picture_medium"`
	PictureBig    string `json:"picture_big"`
}

// deezer looks the artist up by each of their names and returns the
// picture of the best-known artist whose name is exactly one of them.
// unreachable says the request never got an answer from Deezer (a blocked
// or offline network), which is not the artist's fault and stops the pass.
func deezer(ctx context.Context, get sources.Getter, ar domain.Artist) (pic string, unreachable bool, err error) {
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
			return "", true, fmt.Errorf("头像查询失败：%v", err)
		}
		var res struct {
			Data []deezerArtist `json:"data"`
		}
		if err := json.Unmarshal([]byte(resp.Text), &res); err != nil {
			return "", false, fmt.Errorf("头像查询结果无法读取：%v", err)
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
		return "", false, fmt.Errorf("Deezer 上没有找到同名艺人")
	}
	if best.PictureMedium != "" {
		return best.PictureMedium, false, nil // 250 px: enough for a 64 pt circle
	}
	return best.PictureBig, false, nil
}

// qqArtist is one singer of QQ Music's search answer: the name to match
// and the identifier their picture's address is built from.
type qqArtist struct {
	Name string `json:"name"`
	Mid  string `json:"mid"`
}

// qqPhoto is where QQ Music keeps a singer's picture, given their mid: the
// same 300 px photo its own pages show.
const qqPhoto = "https://y.gtimg.cn/music/photo_new/T001R300x300M000"

// qqMusic looks the artist up in QQ Music, Tencent's public search endpoint
// (no account, no key), and returns their photo. It is the catalog with
// Japanese artists' photos that answers from the mainland, where Deezer's
// API does not. unreachable says QQ Music did not answer at all.
func qqMusic(ctx context.Context, get sources.Getter, ar domain.Artist) (pic string, unreachable bool, err error) {
	keys := map[string]bool{}
	names := append([]string{ar.Name}, ar.Aliases...)
	for _, n := range names {
		keys[normalize.Key(n)] = true
	}
	if len(names) > 4 {
		names = names[:4] // a sanity cap; configurations have far fewer aliases
	}
	for _, n := range names {
		resp, err := get.Get(ctx, "https://c.y.qq.com/splcloud/fcgi-bin/smartbox_new.fcg?format=json&key="+url.QueryEscape(n))
		if err != nil {
			return "", true, fmt.Errorf("头像查询失败：%v", err)
		}
		var res struct {
			Data struct {
				Singer struct {
					Items []qqArtist `json:"itemlist"`
				} `json:"singer"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(resp.Text), &res); err != nil {
			return "", false, fmt.Errorf("头像查询结果无法读取：%v", err)
		}
		for _, a := range res.Data.Singer.Items {
			if a.Mid != "" && keys[normalize.Key(a.Name)] {
				return qqPhoto + a.Mid + ".jpg", false, nil
			}
		}
	}
	return "", false, fmt.Errorf("QQ 音乐上没有找到同名艺人")
}

// itunesAlbum is one row of Apple's search answer: who released it, what
// it is, and the cover, which stands in for a photo of the artist.
type itunesAlbum struct {
	Artist     string `json:"artistName"`
	Collection string `json:"collectionName"`
	Kind       string `json:"collectionType"`
	Tracks     int    `json:"trackCount"`
	Artwork    string `json:"artworkUrl100"`
}

// artSize is the size token of an Apple artwork URL ("…/100x100bb.jpg").
var artSize = regexp.MustCompile(`/\d+x\d+[a-z]*\.(jpe?g|png|webp)$`)

// albums looks the artist up in the iTunes catalog and returns the cover of
// one of their releases. Apple answers from the mainland, where Deezer's
// API does not, so this is what keeps avatars working without a proxy.
// unreachable reports that Apple did not answer at all.
func albums(ctx context.Context, get sources.Getter, ar domain.Artist) (pic string, unreachable bool, err error) {
	keys := map[string]bool{}
	names := append([]string{ar.Name}, ar.Aliases...)
	for _, n := range names {
		keys[normalize.Key(n)] = true
	}
	if len(names) > 4 {
		names = names[:4] // a sanity cap; configurations have far fewer aliases
	}
	for _, n := range names {
		pic, ok, err := itunesSearch(ctx, get, n, "jp", keys)
		if err != nil {
			return "", true, err
		}
		if ok {
			return pic, false, nil
		}
	}
	// Not in Japan's storefront: ask the default one, once.
	if pic, ok, err := itunesSearch(ctx, get, ar.Name, "", keys); err != nil {
		return "", true, err
	} else if ok {
		return pic, false, nil
	}
	return "", false, fmt.Errorf("iTunes 上没有找到同名艺人")
}

// itunesSearch asks one storefront for albums of term, and returns the
// cover of the best match among the rows whose artist is one of keys.
func itunesSearch(ctx context.Context, get sources.Getter, term, country string, keys map[string]bool) (string, bool, error) {
	raw := "https://itunes.apple.com/search?entity=album&limit=25&term=" + url.QueryEscape(term)
	if country != "" {
		raw += "&country=" + country
	}
	resp, err := get.Get(ctx, raw)
	if err != nil {
		return "", false, fmt.Errorf("头像查询失败：%v", err)
	}
	var res struct {
		Results []itunesAlbum `json:"results"`
	}
	if err := json.Unmarshal([]byte(resp.Text), &res); err != nil {
		return "", false, fmt.Errorf("头像查询结果无法读取：%v", err)
	}
	best := -1
	for i, a := range res.Results {
		if a.Artwork == "" || !keys[normalize.Key(a.Artist)] {
			continue
		}
		if best < 0 || betterCover(a, res.Results[best]) {
			best = i
		}
	}
	if best < 0 {
		return "", false, nil
	}
	return cover600(res.Results[best].Artwork), true, nil
}

// betterCover prefers a full album to a single, and the longer release
// between two of a kind: its cover says more about the artist than a
// single's does.
func betterCover(a, b itunesAlbum) bool {
	albumA, albumB := a.Kind == "Album", b.Kind == "Album"
	if albumA != albumB {
		return albumA
	}
	return a.Tracks > b.Tracks
}

// cover600 asks Apple's image service for the same cover at 600 px, which
// is what a 64 pt circle needs on a Retina display.
func cover600(raw string) string {
	if artSize.MatchString(raw) {
		return artSize.ReplaceAllString(raw, "/600x600bb.$1")
	}
	return raw
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
