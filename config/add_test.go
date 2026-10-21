package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jpop-live-calendar/internal/domain"
)

const addTestYAML = `# a comment above the list
avatars: deezer
sources:
  - id: one
    name: 一
    kind: website
    url: https://example.com/live
    enabled: true
    interval: 12h
    # the next source's comment
  - id: two
    name: 二
    kind: rss
    url: https://example.com/feed.xml
    enabled: false

artists:
  - name: 艺人
    aliases: [alias]
`

func TestAddSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(path, []byte(addTestYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := AddSource(path, domain.SourceConfig{
		ID: "three", Name: "三", Kind: domain.SourceRSS, URL: "https://example.com/news/feed.xml",
		Enabled: true, Interval: 6 * time.Hour, Layer: domain.LayerNews, Note: "说明",
	})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if got != path {
		t.Errorf("wrote %q, want %q", got, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{"# a comment above the list", "# the next source's comment", "id: three", "interval: 6h", "layer: news"} {
		if !strings.Contains(text, want) {
			t.Errorf("file lost %q:\n%s", want, text)
		}
	}
	c, err := Parse(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(c.Sources) != 3 {
		t.Fatalf("got %d sources, want 3", len(c.Sources))
	}
	s := c.Sources[2]
	if s.ID != "three" || s.Name != "三" || s.Kind != domain.SourceRSS || s.Interval != 6*time.Hour || s.Note != "说明" {
		t.Errorf("added source = %+v", s)
	}
	if len(c.Artists) != 1 || c.Artists[0].Name != "艺人" {
		t.Errorf("artists changed: %+v", c.Artists)
	}
}

func TestAddSourceDuplicate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(path, []byte(addTestYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSource(path, domain.SourceConfig{ID: "one", Name: "重复", Kind: domain.SourceOfficialSite, URL: "https://example.com"}); err == nil {
		t.Fatal("adding a duplicate id succeeded")
	}
}

func TestAddSourceToEmptyList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(path, []byte("avatars: off\nsources: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AddSource(path, domain.SourceConfig{ID: "one", Name: "一", Kind: domain.SourceOfficialSite, URL: "https://example.com"}); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "sources: []") {
		t.Errorf("empty list was kept:\n%s", b)
	}
	c, err := Parse(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(c.Sources) != 1 || c.Sources[0].ID != "one" {
		t.Errorf("sources = %+v", c.Sources)
	}
}

// TestAddSourceEmbedded adds to a copy of the real configuration: its 35
// sources, its comments and the artists list after the sources must all
// survive.
func TestAddSourceEmbedded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(path, embedded, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := Parse(embedded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AddSource(path, domain.SourceConfig{
		ID: "custom-feed", Name: "自建 RSS", Kind: domain.SourceRSS, URL: "https://example.com/feed.xml",
		Enabled: true, Interval: 6 * time.Hour, Layer: domain.LayerNews,
	}); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Live Calendar — 公开数据源与艺人别名", "  # ── 米津玄師 ──", "\nartists:\n"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("file lost %q", want)
		}
	}
	after, err := Parse(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(after.Sources) != len(before.Sources)+1 {
		t.Fatalf("sources %d -> %d", len(before.Sources), len(after.Sources))
	}
	if len(after.Artists) != len(before.Artists) {
		t.Errorf("artists %d -> %d", len(before.Artists), len(after.Artists))
	}
	if last := after.Sources[len(after.Sources)-1]; last.ID != "custom-feed" {
		t.Errorf("last source = %+v", last)
	}
}

func TestIntervalText(t *testing.T) {
	cases := map[time.Duration]string{
		0:                "0",
		6 * time.Hour:    "6h",
		30 * time.Minute: "30m",
		90 * time.Minute: "90m",
	}
	for d, want := range cases {
		if got := intervalText(d); got != want {
			t.Errorf("intervalText(%v) = %q, want %q", d, got, want)
		}
	}
}
