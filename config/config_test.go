package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

// TestSourceTranslations checks that a source's name and note follow the
// interface language, so switching to ja or en leaves no Chinese behind
// in the "数据源" page or in its buttons' tooltips.
func TestSourceTranslations(t *testing.T) {
	c := Default()
	demo := c.Sources[0]
	for tag, want := range map[string]string{
		"zh-Hans": "演示数据（虚构）",
		"":        "演示数据（虚构）",
		"ja":      "デモデータ（架空）",
		"en":      "Demo data (fictional)",
		"ja-JP":   "デモデータ（架空）",
	} {
		if got := demo.DisplayName(tag); got != want {
			t.Errorf("DisplayName(%q) = %q, want %q", tag, got, want)
		}
	}
	for _, s := range c.Sources {
		for _, tag := range []string{"ja", "en"} {
			// Japanese keeps （）: only the Chinese words are wrong there.
			for _, zh := range []string{"官网", "厂牌", "官方站点"} {
				if strings.Contains(s.DisplayName(tag), zh) {
					t.Errorf("%s in %s: %q still says %q", s.ID, tag, s.DisplayName(tag), zh)
				}
			}
			if s.Note != "" && s.DisplayNote(tag) == s.Note {
				t.Errorf("%s in %s: the note is untranslated: %q", s.ID, tag, s.Note)
			}
		}
		if strings.ContainsAny(s.DisplayName("en"), "（）") {
			t.Errorf("%s in en: %q keeps full-width brackets", s.ID, s.DisplayName("en"))
		}
	}
	// A source the user adds keeps the name and note they typed.
	plain := domain.SourceConfig{Name: "my blog", Note: "n"}
	if plain.DisplayName("ja") != "my blog" || plain.DisplayNote("en") != "n" {
		t.Errorf("a source without translations changed: %+v", plain)
	}
}

// TestAvatarsSetting checks the four ways of finding a picture, and that
// anything else is refused before the app runs on a setting it cannot use.
func TestAvatarsSetting(t *testing.T) {
	src := "sources:\n  - id: a\n    kind: demo\n"
	for raw, want := range map[string]string{
		"":           AvatarsDeezer, // what a file without the key means
		"deezer":     AvatarsDeezer,
		"qq":         AvatarsQQ,
		"qqmusic":    AvatarsQQ,
		" QQ ":       AvatarsQQ,
		"itunes":     AvatarsITunes,
		"apple":      AvatarsITunes,
		"off":        AvatarsOff,
		"none":       AvatarsOff,
		"  ITUNES  ": AvatarsITunes,
	} {
		c, err := Parse([]byte(src + "avatars: '" + raw + "'\n"))
		if err != nil {
			t.Errorf("avatars %q: %v", raw, err)
			continue
		}
		if c.Avatars != want {
			t.Errorf("avatars %q = %q, want %q", raw, c.Avatars, want)
		}
	}
	if _, err := Parse([]byte(src + "avatars: lastfm\n")); err == nil {
		t.Error("an unknown avatars setting was accepted")
	}
	if Default().Avatars != AvatarsDeezer {
		t.Errorf("the built-in configuration uses %q", Default().Avatars)
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
