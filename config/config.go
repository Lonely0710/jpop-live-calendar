// Package config loads sources.yaml: the public sources to sync and the
// artist aliases used to merge spellings.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"jpop-live-calendar/internal/domain"
)

//go:embed sources.yaml
var embedded []byte

// Config is the loaded file.
type Config struct {
	Sources []domain.SourceConfig
	Artists []domain.Artist
	Avatars string // AvatarsDeezer or AvatarsOff
	Path    string // where it was read from; "内置默认配置" when embedded
}

// How artist pictures are found.
const (
	AvatarsDeezer = "deezer" // Deezer's public artist search
	AvatarsOff    = "off"    // only configured avatar_url
)

type fileSource struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Kind     string `yaml:"kind"`
	URL      string `yaml:"url"`
	Enabled  bool   `yaml:"enabled"`
	Interval string `yaml:"interval"`
	Artist   string `yaml:"artist"`
	Follow   string `yaml:"follow"`
	Feature  string `yaml:"feature"`
	Layer    string `yaml:"layer"`
	Note     string `yaml:"note"`
	Article  struct {
		Enabled     bool     `yaml:"enabled"`
		BodyStart   string   `yaml:"body_start"`
		BodyEnd     string   `yaml:"body_end"`
		ArtistStart string   `yaml:"artist_start"`
		ArtistEnd   string   `yaml:"artist_end"`
		Keywords    []string `yaml:"keywords"`
		Exclude     []string `yaml:"exclude"`
		Include     string   `yaml:"include"`
		TitleOnly   bool     `yaml:"title_only"`
		MaxAge      string   `yaml:"max_age"`
		MaxPerSync  int      `yaml:"max_per_sync"`
		Delay       string   `yaml:"delay"`
	} `yaml:"article"`
}

type file struct {
	Avatars string       `yaml:"avatars"`
	Sources []fileSource `yaml:"sources"`
	Artists []struct {
		Name     string   `yaml:"name"`
		Aliases  []string `yaml:"aliases"`
		Official string   `yaml:"official_url"`
		Avatar   string   `yaml:"avatar_url"`
		Color    string   `yaml:"color"`
		Follow   *bool    `yaml:"follow"` // default true
	} `yaml:"artists"`
}

// Embedded is the label of the built-in copy.
const Embedded = "内置默认配置"

// Candidates returns the paths Load tries, in order.
func Candidates() []string {
	var out []string
	if p := os.Getenv("LIVE_CALENDAR_CONFIG"); p != "" {
		out = append(out, p)
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, "Library", "Application Support", "Live Calendar", "sources.yaml"))
	}
	return append(out, filepath.Join("config", "sources.yaml"))
}

// Load reads the first config file that exists, or the embedded copy. A
// file that exists but is invalid is an error, so a typo is not silently
// replaced by the defaults.
func Load() (Config, error) {
	for _, p := range Candidates() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		c, err := Parse(b)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", p, err)
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		c.Path = p
		return c, nil
	}
	c, err := Parse(embedded)
	c.Path = Embedded
	return c, err
}

// Default returns the embedded configuration.
func Default() Config {
	c, err := Parse(embedded)
	if err != nil {
		panic(err)
	}
	c.Path = Embedded
	return c
}

// Parse reads a sources.yaml.
func Parse(b []byte) (Config, error) {
	var f file
	if err := yaml.Unmarshal(b, &f); err != nil {
		return Config{}, err
	}
	var c Config
	switch strings.ToLower(strings.TrimSpace(f.Avatars)) {
	case "", AvatarsDeezer:
		c.Avatars = AvatarsDeezer
	case AvatarsOff, "false", "none":
		c.Avatars = AvatarsOff
	default:
		return Config{}, fmt.Errorf("avatars 应为 deezer 或 off，而不是 %q", f.Avatars)
	}
	seen := map[string]bool{}
	for i, s := range f.Sources {
		if s.ID == "" {
			return Config{}, fmt.Errorf("第 %d 个数据源缺少 id", i+1)
		}
		if seen[s.ID] {
			return Config{}, fmt.Errorf("数据源 id %q 重复", s.ID)
		}
		seen[s.ID] = true
		kind := domain.SourceKind(strings.ToLower(s.Kind))
		if kind == "official_site" {
			kind = domain.SourceOfficialSite
		}
		switch kind {
		case domain.SourceDemo, domain.SourceRSS, domain.SourceOfficialSite, domain.SourceAPI:
		default:
			return Config{}, fmt.Errorf("数据源 %q：未知类型 %q（可用 demo / rss / website / api）", s.ID, s.Kind)
		}
		if kind != domain.SourceDemo && !strings.HasPrefix(s.URL, "https://") && !strings.HasPrefix(s.URL, "http://") {
			return Config{}, fmt.Errorf("数据源 %q：url 必须以 https:// 开头", s.ID)
		}
		iv, err := duration(s.Interval)
		if err != nil {
			return Config{}, fmt.Errorf("数据源 %q：interval %w", s.ID, err)
		}
		if iv > 0 && iv < 10*time.Minute && kind != domain.SourceDemo {
			iv = 10 * time.Minute // be gentle with public sites
		}
		delay, err := duration(s.Article.Delay)
		if err != nil {
			return Config{}, fmt.Errorf("数据源 %q：delay %w", s.ID, err)
		}
		if delay < time.Second {
			delay = time.Second
		}
		if s.Follow != "" {
			if _, err := regexp.Compile(s.Follow); err != nil {
				return Config{}, fmt.Errorf("数据源 %q：follow 不是有效的正则表达式：%v", s.ID, err)
			}
		}
		if s.Feature != "" {
			if _, err := regexp.Compile(s.Feature); err != nil {
				return Config{}, fmt.Errorf("数据源 %q：feature 不是有效的正则表达式：%v", s.ID, err)
			}
		}
		if s.Article.Include != "" {
			if _, err := regexp.Compile("(?i)" + s.Article.Include); err != nil {
				return Config{}, fmt.Errorf("数据源 %q：article.include 不是有效的正则表达式：%v", s.ID, err)
			}
		}
		maxAge, err := duration(s.Article.MaxAge)
		if err != nil {
			return Config{}, fmt.Errorf("数据源 %q：article.max_age %w", s.ID, err)
		}
		if maxAge < 0 {
			return Config{}, fmt.Errorf("数据源 %q：article.max_age 不能为负数", s.ID)
		}
		switch s.Layer {
		case "", domain.LayerLive, domain.LayerNews:
		default:
			return Config{}, fmt.Errorf("数据源 %q：layer 应为 live 或 news", s.ID)
		}
		name := s.Name
		if name == "" {
			name = s.ID
		}
		c.Sources = append(c.Sources, domain.SourceConfig{
			ID: s.ID, Name: name, Kind: kind, URL: s.URL, Enabled: s.Enabled, Interval: iv,
			Article: domain.ArticleOptions{
				Enabled: s.Article.Enabled, BodyStart: s.Article.BodyStart, BodyEnd: s.Article.BodyEnd,
				ArtistStart: s.Article.ArtistStart, ArtistEnd: s.Article.ArtistEnd,
				Keywords: s.Article.Keywords, Exclude: s.Article.Exclude, Include: s.Article.Include,
				TitleOnly: s.Article.TitleOnly, MaxAge: maxAge,
				MaxPerSync: s.Article.MaxPerSync, Delay: delay,
			},
			Artist: strings.TrimSpace(s.Artist), Follow: s.Follow, Note: strings.TrimSpace(s.Note),
			Feature: s.Feature, Layer: s.Layer,
		})
	}
	for _, a := range f.Artists {
		if a.Name != "" {
			if a.Avatar != "" && a.Avatar != "none" && !strings.HasPrefix(a.Avatar, "https://") && !strings.HasPrefix(a.Avatar, "http://") {
				return Config{}, fmt.Errorf("艺人 %q：avatar_url 必须以 https:// 开头，或填 none", a.Name)
			}
			if a.Color != "" && !hexRe.MatchString(a.Color) {
				return Config{}, fmt.Errorf("艺人 %q：color 应为 #RRGGBB", a.Name)
			}
			c.Artists = append(c.Artists, domain.Artist{Name: a.Name, Aliases: a.Aliases, OfficialURL: a.Official,
				AvatarURL: a.Avatar, Color: a.Color, Followed: a.Follow == nil || *a.Follow})
		}
	}
	return c, nil
}

var hexRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func duration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n < 0 {
			return 0, fmt.Errorf("格式不正确（例：30m、2h、90d）")
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("格式不正确（例：30m、2h、90d）")
	}
	return d, nil
}
