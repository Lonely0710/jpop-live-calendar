package cache

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"jpop-live-calendar/internal/avatar"
)

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func TestAvatarCache(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	fill(img, img.Bounds(), color.RGBA{200, 40, 40, 255})
	path := filepath.Join(t.TempDir(), "avatars.json")
	want := avatar.Avatar{
		ArtistID: "a-1", URL: "https://img.example/a.png", Source: "配置", Image: img,
		Color: color.RGBA{200, 40, 40, 255}, Share: 0.5, Manual: true,
		ETag: `"v1"`, LastModified: "yesterday", Hash: "abc", FetchedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := SaveAvatars(path, map[string]avatar.Avatar{"a-1": want}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadAvatars(path)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := got["a-1"]
	if !ok {
		t.Fatal("avatar missing after load")
	}
	if a.URL != want.URL || a.Source != want.Source || a.Color != want.Color || !a.Manual || a.ETag != want.ETag || a.Hash != want.Hash {
		t.Errorf("avatar = %+v", a)
	}
	if a.Image == nil || a.Image.Bounds() != image.Rect(0, 0, 8, 8) {
		t.Errorf("image = %v", a.Image)
	}
	if !a.FetchedAt.Equal(want.FetchedAt) {
		t.Errorf("fetched at = %v, want %v", a.FetchedAt, want.FetchedAt)
	}
}

func TestAvatarCacheMissingAndCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "avatars.json")
	if m, err := LoadAvatars(path); err != nil || len(m) != 0 {
		t.Fatalf("missing cache: %v %v", m, err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAvatars(path); err == nil {
		t.Fatal("a corrupt cache was accepted")
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Errorf("corrupt cache was not moved: %v", err)
	}
}
