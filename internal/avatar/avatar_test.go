package avatar

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/url"
	"strings"
	"testing"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/sources"
)

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func TestDominantLargestBlock(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	fill(img, img.Bounds(), color.RGBA{250, 250, 250, 255})             // white backdrop, 50%
	fill(img, image.Rect(0, 0, 100, 30), color.RGBA{200, 40, 40, 255})  // red, 30%
	fill(img, image.Rect(0, 30, 100, 50), color.RGBA{40, 60, 200, 255}) // blue, 20%
	c, share := Dominant(img)
	if c.R < 180 || c.G > 60 || c.B > 60 {
		t.Fatalf("dominant = %v, want the red block (white backdrop skipped)", c)
	}
	if share < 0.25 || share > 0.35 {
		t.Fatalf("share = %.2f, want about 0.30", share)
	}
}

func TestDominantMonochrome(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	fill(img, img.Bounds(), color.RGBA{5, 5, 5, 255})
	if c, _ := Dominant(img); c.R > 20 {
		t.Fatalf("black picture gave %v, want black", c)
	}
	if c, s := Dominant(image.NewRGBA(image.Rectangle{})); s != 0 || c.A != 255 {
		t.Fatalf("empty picture gave %v %.2f", c, s)
	}
}

func TestFallbackStable(t *testing.T) {
	a, b := Fallback("米津玄師"), Fallback("米津玄師")
	if a != b {
		t.Fatal("fallback color must be stable")
	}
	if Fallback("Ado") == Fallback("Aimer") {
		t.Fatal("different names should usually get different colors")
	}
}

type fakePictureGetter struct {
	notModified bool
	body        []byte
	etag        string
	last        string
	seededETag  string
	seededLast  string
}

func (f *fakePictureGetter) Get(context.Context, string) (*sources.Response, error) {
	r := &sources.Response{Status: 200, Body: f.body, ETag: f.etag, LastModified: f.last}
	if f.notModified {
		r.Status, r.NotModified = 304, true
	}
	return r, nil
}

func (f *fakePictureGetter) SetValidators(_, etag, last string) {
	f.seededETag, f.seededLast = etag, last
}

func pngBytes(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	fill(img, img.Bounds(), c)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRevalidateKeepsUnchanged(t *testing.T) {
	body := pngBytes(t, color.RGBA{200, 40, 40, 255})
	img, _, _ := image.Decode(bytes.NewReader(body))
	ar := domain.Artist{Name: "A", AvatarURL: "https://img.example/a.png"}
	id := ArtistID(ar)
	s := NewStore()
	s.Restore(map[string]Avatar{id: {
		ArtistID: id, URL: ar.AvatarURL, Source: "配置", Image: img,
		Color: color.RGBA{200, 40, 40, 255}, Hash: hashBytes(body),
		ETag: `"v1"`, LastModified: "yesterday",
	}})
	g := &fakePictureGetter{notModified: true, etag: `"v1"`, last: "yesterday"}
	s.Revalidate(context.Background(), Getters{Pictures: g}, []domain.Artist{ar})
	got, _ := s.Get(id)
	if got.Hash != hashBytes(body) || got.Image == nil {
		t.Errorf("unchanged picture was replaced: %+v", got)
	}
	if g.seededETag != `"v1"` || g.seededLast != "yesterday" {
		t.Errorf("validators seeded = %q %q", g.seededETag, g.seededLast)
	}
}

func TestRevalidateReplacesChanged(t *testing.T) {
	oldBody := pngBytes(t, color.RGBA{200, 40, 40, 255})
	newBody := pngBytes(t, color.RGBA{40, 60, 200, 255})
	img, _, _ := image.Decode(bytes.NewReader(oldBody))
	ar := domain.Artist{Name: "A", AvatarURL: "https://img.example/a.png"}
	id := ArtistID(ar)
	s := NewStore()
	s.Restore(map[string]Avatar{id: {
		ArtistID: id, URL: ar.AvatarURL, Source: "配置", Image: img,
		Color: color.RGBA{200, 40, 40, 255}, Hash: hashBytes(oldBody), ETag: `"v1"`,
	}})
	g := &fakePictureGetter{body: newBody, etag: `"v2"`}
	s.Revalidate(context.Background(), Getters{Pictures: g}, []domain.Artist{ar})
	got, _ := s.Get(id)
	if got.Hash != hashBytes(newBody) || got.ETag != `"v2"` {
		t.Errorf("changed picture was not replaced: %+v", got)
	}
}

type deezerGetter map[string]string

func (g deezerGetter) Get(_ context.Context, raw string) (*sources.Response, error) {
	return &sources.Response{Status: 200, Text: g[raw]}, nil
}

func TestDeezerPicksBestKnown(t *testing.T) {
	q := func(s string) string {
		return "https://api.deezer.com/search/artist?limit=10&q=" + url.QueryEscape(s)
	}
	g := deezerGetter{
		q("Official髭男dism"):        `{"data":[{"name":"Official髭男dism","nb_fan":184,"picture_medium":"https://img.example/small.jpg","picture_big":"https://img.example/small-big.jpg"}]}`,
		q("Official HIGE DANdism"): `{"data":[{"name":"OFFICIAL HIGE DANDISM","nb_fan":22808,"picture_medium":"https://img.example/big.jpg","picture_big":"https://img.example/big-big.jpg"}]}`,
	}
	ar := domain.Artist{Name: "Official髭男dism", Aliases: []string{"Official HIGE DANdism"}}
	got, unreachable, err := deezer(context.Background(), g, ar)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://img.example/big.jpg" {
		t.Errorf("picked %q, want the best-known alias", got)
	}
	if unreachable {
		t.Error("an answer from Deezer was called unreachable")
	}
}

// downGetter is a catalog that never answers, as api.deezer.com from the
// mainland: the connection is refused before any answer arrives.
type downGetter struct{ calls int }

func (g *downGetter) Get(context.Context, string) (*sources.Response, error) {
	g.calls++
	return nil, errors.New("EOF")
}

// jsonGetter answers every request with one body and counts the calls.
type jsonGetter struct {
	body  string
	calls []string
}

func (g *jsonGetter) Get(_ context.Context, raw string) (*sources.Response, error) {
	g.calls = append(g.calls, raw)
	return &sources.Response{Status: 200, Text: g.body}, nil
}

// TestFallsBackToITunes checks what happens on a network that cannot reach
// Deezer: the first artist tries it once, then the cover comes from iTunes
// and the rest of the pass does not spend a request on Deezer again.
func TestFallsBackToITunes(t *testing.T) {
	s := NewStore()
	down := &downGetter{}
	itunes := &jsonGetter{body: `{"results":[{"artistName":"Ado","collectionType":"Album","trackCount":12,"artworkUrl100":"https://img.example/a/100x100bb.jpg"}]}`}
	artists := []domain.Artist{{Name: "Ado"}, {Name: "Ado"}}
	g := Getters{Lookup: down, Albums: itunes, Pictures: &fakePictureGetter{body: pngBytes(t, color.RGBA{30, 90, 200, 255})}}
	s.Fetch(context.Background(), g, artists)
	if down.calls != 1 {
		t.Errorf("Deezer was asked %d times, want 1: an unreachable catalog is not retried for every artist", down.calls)
	}
	av, ok := s.Get(ArtistID(artists[0]))
	if !ok || av.Image == nil || av.Source != "iTunes" {
		t.Fatalf("no cover from iTunes: %+v", av)
	}
	if av.URL != "https://img.example/a/600x600bb.jpg" {
		t.Errorf("picture = %q, want the 600 px cover", av.URL)
	}
	if !strings.Contains(itunes.calls[0], "country=jp") {
		t.Errorf("iTunes was asked %q, want Japan's storefront first", itunes.calls[0])
	}
	if s.Unreachable() {
		t.Error("the pass gave up although iTunes answered")
	}
}

// TestBothCatalogsUnreachableStopsThePass checks that a pass ends after one
// artist when neither catalog answers, instead of spending every artist's
// seconds on the same failure.
func TestBothCatalogsUnreachableStopsThePass(t *testing.T) {
	s := NewStore()
	deezer, apple := &downGetter{}, &downGetter{}
	artists := []domain.Artist{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	s.Fetch(context.Background(), Getters{Lookup: deezer, Albums: apple, Pictures: &fakePictureGetter{}}, artists)
	if deezer.calls != 1 || apple.calls != 1 {
		t.Errorf("asked Deezer %d and iTunes %d times, want 1 each", deezer.calls, apple.calls)
	}
	if !s.Unreachable() {
		t.Error("the store does not report that no catalog could be reached")
	}

	// Once a pass reaches a catalog, the state clears.
	up := &jsonGetter{body: `{"results":[{"artistName":"A","collectionType":"Album","trackCount":8,"artworkUrl100":"https://img.example/a/100x100bb.jpg"}]}`}
	s.Fetch(context.Background(), Getters{Lookup: &downGetter{}, Albums: up, Pictures: &fakePictureGetter{body: pngBytes(t, color.RGBA{200, 40, 40, 255})}}, artists[:1])
	if s.Unreachable() {
		t.Error("the state did not clear after a catalog answered")
	}
}

// TestAlbumsPicksTheBestCover checks the choice among Apple's answers: only
// rows by the artist itself, a full album before a single, and the same
// cover at 600 px.
func TestAlbumsPicksTheBestCover(t *testing.T) {
	body := `{"results":[
		{"artistName":"Ado Tribute Band","collectionType":"Album","trackCount":20,"artworkUrl100":"https://img.example/other/100x100bb.jpg"},
		{"artistName":"ADO","collectionType":"Single","trackCount":1,"artworkUrl100":"https://img.example/single/100x100bb.jpg"},
		{"artistName":"ADO","collectionType":"Album","trackCount":3,"artworkUrl100":"https://img.example/ep/100x100bb.png"},
		{"artistName":"ADO","collectionType":"Album","trackCount":14,"artworkUrl100":"https://img.example/album/100x100bb.jpg"}
	]}`
	got, unreachable, err := albums(context.Background(), &jsonGetter{body: body}, domain.Artist{Name: "Ado"})
	if err != nil {
		t.Fatal(err)
	}
	if unreachable {
		t.Error("an answer from iTunes was called unreachable")
	}
	if got != "https://img.example/album/600x600bb.jpg" {
		t.Errorf("picked %q, want the longest album's cover at 600 px", got)
	}
}

func TestAlbumsMissingAndUnreachable(t *testing.T) {
	// Rows by another artist only: no picture, but Apple did answer.
	got, unreachable, err := albums(context.Background(),
		&jsonGetter{body: `{"results":[{"artistName":"Someone Else","collectionType":"Album","trackCount":9,"artworkUrl100":"https://img.example/x/100x100bb.jpg"}]}`},
		domain.Artist{Name: "Ado"})
	if err == nil || got != "" || unreachable {
		t.Errorf("a miss = %q, %v, %v; want an error that is not unreachable", got, unreachable, err)
	}
	// No answer at all.
	if _, unreachable, err := albums(context.Background(), &downGetter{}, domain.Artist{Name: "Ado"}); err == nil || !unreachable {
		t.Errorf("a network error = %v, %v; want unreachable", unreachable, err)
	}
}

func TestCover600(t *testing.T) {
	for raw, want := range map[string]string{
		"https://img.example/a/100x100bb.jpg":  "https://img.example/a/600x600bb.jpg",
		"https://img.example/a/100x100bb.png":  "https://img.example/a/600x600bb.png",
		"https://img.example/a/100x100-60.jpg": "https://img.example/a/100x100-60.jpg",
		"https://img.example/a/cover.jpg":      "https://img.example/a/cover.jpg",
	} {
		if got := cover600(raw); got != want {
			t.Errorf("cover600(%q) = %q, want %q", raw, got, want)
		}
	}
}

// qqGetter answers QQ Music's search with one body and records the terms.
type qqGetter struct {
	body  string
	terms []string
}

func (g *qqGetter) Get(_ context.Context, raw string) (*sources.Response, error) {
	g.terms = append(g.terms, raw)
	return &sources.Response{Status: 200, Text: g.body}, nil
}

// TestQQMusicPicksTheSameName checks the photo QQ Music gives back: the
// singer whose name is exactly one of the artist's, not the first row
// (which is another singer of a similar name).
func TestQQMusicPicksTheSameName(t *testing.T) {
	body := `{"data":{"singer":{"itemlist":[
		{"name":"阿杜","mid":"0022VtZd19rZpi"},
		{"name":"Ado","mid":"002D5FGW4Ckwyy"},
		{"name":"ADO","mid":"001zzzz"}
	]}}}`
	get := &qqGetter{body: body}
	got, unreachable, err := qqMusic(context.Background(), get, domain.Artist{Name: "Ado"})
	if err != nil {
		t.Fatal(err)
	}
	if unreachable {
		t.Error("an answer from QQ Music was called unreachable")
	}
	if got != "https://y.gtimg.cn/music/photo_new/T001R300x300M000002D5FGW4Ckwyy.jpg" {
		t.Errorf("picture = %q, want the singer's photo", got)
	}
	if !strings.Contains(get.terms[0], "key=Ado") {
		t.Errorf("asked %q", get.terms[0])
	}
}

// TestQQMusicFallsBackToITunes checks the chain behind avatars: qq: tries QQ
// Music, and takes Apple's cover for an artist it does not know.
func TestQQMusicFallsBackToITunes(t *testing.T) {
	s := NewStore()
	qq := &qqGetter{body: `{"data":{"singer":{"itemlist":[]}}}`}
	itunes := &jsonGetter{body: `{"results":[{"artistName":"Zed","collectionType":"Album","trackCount":9,"artworkUrl100":"https://img.example/z/100x100bb.jpg"}]}`}
	ar := domain.Artist{Name: "Zed"}
	g := Getters{QQ: qq, Albums: itunes, Pictures: &fakePictureGetter{body: pngBytes(t, color.RGBA{90, 40, 160, 255})}}
	s.Fetch(context.Background(), g, []domain.Artist{ar})
	av, ok := s.Get(ArtistID(ar))
	if !ok || av.Source != "iTunes" || av.Image == nil {
		t.Fatalf("no cover from iTunes: %+v", av)
	}
	if len(qq.terms) != 1 {
		t.Errorf("QQ Music was asked %d times, want 1", len(qq.terms))
	}
}

// TestQQUnreachableStopsAsking checks that an unreachable QQ Music is not
// retried for every artist of the pass.
func TestQQUnreachableStopsAsking(t *testing.T) {
	s := NewStore()
	down := &downGetter{}
	itunes := &jsonGetter{body: `{"results":[]}`}
	artists := []domain.Artist{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	s.Fetch(context.Background(), Getters{QQ: down, Albums: itunes, Pictures: &fakePictureGetter{}}, artists)
	if down.calls != 1 {
		t.Errorf("QQ Music was asked %d times, want 1", down.calls)
	}
}

// TestNewGettersModes checks which catalogs each avatars setting reaches
// for, so that `qq` really goes to QQ Music and `off` to nothing.
func TestNewGettersModes(t *testing.T) {
	want := map[string][3]bool{ // Lookup, QQ, Albums
		config.AvatarsDeezer: {true, true, true},
		config.AvatarsQQ:     {false, true, true},
		config.AvatarsITunes: {false, false, true},
		config.AvatarsOff:    {false, false, false},
	}
	for mode, w := range want {
		g := NewGetters(mode)
		got := [3]bool{g.Lookup != nil, g.QQ != nil, g.Albums != nil}
		if got != w {
			t.Errorf("%s: lookups Deezer/QQ/iTunes = %v, want %v", mode, got, w)
		}
		if g.Pictures == nil {
			t.Errorf("%s: no picture fetcher", mode)
		}
	}
}

// TestPhotoBeforeCover checks the order of the default setting: a photo from
// either catalog wins, and Apple's covers are only asked when both have no
// match.
func TestPhotoBeforeCover(t *testing.T) {
	s := NewStore()
	noMatch := &jsonGetter{body: `{"data":[]}`} // Deezer answered, knows nobody by that name
	qq := &qqGetter{body: `{"data":{"singer":{"itemlist":[{"name":"Ado","mid":"002D5FGW4Ckwyy"}]}}}`}
	itunes := &jsonGetter{body: `{"results":[{"artistName":"Ado","collectionType":"Album","trackCount":9,"artworkUrl100":"https://img.example/a/100x100bb.jpg"}]}`}
	ar := domain.Artist{Name: "Ado"}
	g := Getters{Lookup: noMatch, QQ: qq, Albums: itunes, Pictures: &fakePictureGetter{body: pngBytes(t, color.RGBA{20, 20, 20, 255})}}
	s.Fetch(context.Background(), g, []domain.Artist{ar})
	av, _ := s.Get(ArtistID(ar))
	if av.Source != "QQ" {
		t.Errorf("source = %q, want QQ: a photo comes before a cover", av.Source)
	}
	if av.URL != "https://y.gtimg.cn/music/photo_new/T001R300x300M000002D5FGW4Ckwyy.jpg" {
		t.Errorf("picture = %q", av.URL)
	}
	if len(itunes.calls) != 0 {
		t.Errorf("iTunes was asked %d times, want 0", len(itunes.calls))
	}
}
