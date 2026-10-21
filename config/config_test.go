package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"jpop-live-calendar/internal/domain"
)

func TestDefault(t *testing.T) {
	c := Default()
	if len(c.Sources) < 2 || c.Sources[0].Kind != domain.SourceDemo {
		t.Fatalf("sources = %+v", c.Sources)
	}
	yz := c.Sources[1]
	if yz.ID != "official-yonezu" || yz.Kind != domain.SourceOfficialSite || yz.Artist != "米津玄師" || yz.Interval != 12*time.Hour || yz.Layer != domain.LayerLive || yz.Article.MaxPerSync != 3 || yz.Article.Delay != 2*time.Second {
		t.Errorf("official-yonezu = %+v", yz)
	}
	for _, s := range c.Sources {
		if s.ID == "skream-news" {
			t.Error("skream-news is still configured")
		}
	}
	if len(c.Artists) == 0 {
		t.Error("no artists")
	}
}

func TestParseErrors(t *testing.T) {
	for _, bad := range []string{
		"sources:\n  - name: x\n",
		"sources:\n  - id: a\n    kind: ftp\n",
		"sources:\n  - id: a\n    kind: rss\n    url: file:///etc/passwd\n",
		"sources:\n  - id: a\n    kind: demo\n  - id: a\n    kind: demo\n",
		"sources:\n  - id: a\n    kind: demo\n    interval: soon\n",
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	c, err := Parse([]byte("sources:\n  - id: a\n    kind: rss\n    url: https://x.example/feed\n    interval: 1m\n"))
	if err != nil || c.Sources[0].Interval != 10*time.Minute {
		t.Errorf("min interval: %+v %v", c.Sources, err)
	}
	c, err = Parse([]byte("sources:\n  - id: a\n    kind: rss\n    url: https://x.example/feed\n    article:\n      include: 'Official髭男dism'\n      title_only: true\n      max_age: 90d\n"))
	if err != nil || c.Sources[0].Article.Include != "Official髭男dism" || !c.Sources[0].Article.TitleOnly || c.Sources[0].Article.MaxAge != 90*24*time.Hour {
		t.Errorf("article.include/title_only/max_age: %+v %v", c.Sources, err)
	}
	if _, err := Parse([]byte("sources:\n  - id: a\n    kind: rss\n    url: https://x.example/feed\n    article:\n      include: '('\n")); err == nil {
		t.Error("an invalid article.include was accepted")
	}
}

func TestRemoveSource(t *testing.T) {
	base := Default()
	for i, s := range base.Sources {
		out, err := removeSource(embedded, s.ID)
		if err != nil {
			t.Fatalf("remove %s: %v", s.ID, err)
		}
		c, err := Parse(out)
		if err != nil {
			t.Fatalf("remove %s: %v\n%s", s.ID, err, out)
		}
		if len(c.Sources) != len(base.Sources)-1 || len(c.Artists) != len(base.Artists) {
			t.Fatalf("remove %s: %d sources, %d artists", s.ID, len(c.Sources), len(c.Artists))
		}
		for j, x := range c.Sources {
			k := j
			if j >= i {
				k++
			}
			if x.ID != base.Sources[k].ID {
				t.Fatalf("remove %s: source %d is %s", s.ID, j, x.ID)
			}
		}
		if bytes.Contains(out, []byte("\n\n\n")) {
			t.Errorf("remove %s: left two blank lines", s.ID)
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sources.yaml")
	if err := os.WriteFile(path, embedded, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := RemoveSource(path, "official-yonezu")
	if err != nil || got != path {
		t.Fatalf("RemoveSource = %q, %v", got, err)
	}
	b, _ := os.ReadFile(path)
	if bytes.Contains(b, []byte("id: official-yonezu\n")) || !bytes.Contains(b, []byte("# Live Calendar")) {
		t.Errorf("file after removal:\n%s", b)
	}
	if _, err := RemoveSource(path, "nope"); err == nil {
		t.Error("removed a missing source")
	}
	if _, err := RemoveSource(Embedded+"（broken）", "demo"); err == nil {
		t.Error("wrote over a broken file")
	}
}
