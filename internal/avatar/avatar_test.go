package avatar

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/url"
	"testing"

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
	got, err := deezer(context.Background(), g, ar)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://img.example/big.jpg" {
		t.Errorf("picked %q, want the best-known alias", got)
	}
}
