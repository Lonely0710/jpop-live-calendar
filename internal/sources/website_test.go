package sources

import (
	"context"
	"strings"
	"testing"
	"time"

	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
)

func TestPageTitleDropsSiteName(t *testing.T) {
	cases := map[string]string{
		"「Ringo Jam Tour “SHADOWS”」香川公演 -Mrs. GREEN APPLE OFFICIAL SITE|OFFICIAL FAN CLUB 「Ringo Jam」": "「Ringo Jam Tour “SHADOWS”」香川公演",
		"RADWIMPS Official Fan Club Tour 2026 “ユーリンチー” | RADWIMPS OFFICIAL SITE":                       "RADWIMPS Official Fan Club Tour 2026 “ユーリンチー”",
		"米津玄師 2026 TOUR / GHOST | 米津玄師 official site":                                                  "米津玄師 2026 TOUR / GHOST",
	}
	for in, want := range cases {
		if got := pageTitle(in, "Mrs. GREEN APPLE"); got != want {
			t.Errorf("pageTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLinkTitle(t *testing.T) {
	in := "10.14 『ユイカ』TOUR「Anemone」オフィシャル最速先行スタート！ 2026.10.14 LIVE/EVENT"
	if got := linkTitle(in); got != "『ユイカ』TOUR「Anemone」オフィシャル最速先行スタート！" {
		t.Errorf("linkTitle = %q", got)
	}
	if got := pageTitle("NEWS | 『ユイカ』オフィシャルサイト&ファンクラブ「栞」", "『ユイカ』"); got != "" {
		t.Errorf("pageTitle = %q, want the link's text", got)
	}
}

// inlineSource is the higedan NEWS source: a list page whose announcements are
// inline, so the items are split out instead of followed.
func inlineSource(opt domain.ArticleOptions) domain.SourceConfig {
	if opt.Item == "" {
		opt.Item, opt.ItemTitle, opt.ItemDate = `<div class="trigger`, `<p class="tit">`, `<p class="date">`
		opt.BodyStart = `<div class="block--body">`
	}
	opt.MaxPerSync = 3
	return domain.SourceConfig{
		ID: "official-higedan-news", Name: "Official髭男dism 官网 NEWS",
		URL: "https://higedan.example/news/3/", Artist: "Official髭男dism",
		Kind: domain.SourceOfficialSite, Article: opt,
	}
}

func fetchInline(t *testing.T, src domain.SourceConfig, page string) Result {
	t.Helper()
	now := time.Date(2026, 11, 2, 16, 0, 0, 0, domain.Tokyo)
	env := Env{Get: &fakeGetter{pages: map[string]string{src.URL: page}}, Now: func() time.Time { return now }}
	res, err := Website{}.Fetch(context.Background(), src, env)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestInlineItems(t *testing.T) {
	res := fetchInline(t, inlineSource(domain.ArticleOptions{}), read(t, "higedan_list.html"))
	if len(res.Announcements) != 3 {
		t.Fatalf("announcements = %d, want 3 (max_per_sync)", len(res.Announcements))
	}
	first := res.Announcements[0]
	if first.Title != "「破顔」がJR東日本の企業広告CMソングに決定 !" {
		t.Errorf("title = %q", first.Title)
	}
	if got := first.PublishedAt.Format("2006-01-02"); got != "2026-11-02" {
		t.Errorf("published = %s, want the item's own date", got)
	}
	if first.URL != "https://higedan.example/news/3/" {
		t.Errorf("url = %q, want the page a reader opens", first.URL)
	}
	if !strings.Contains(first.RawText, "あたりまえの毎日") {
		t.Errorf("raw text = %q", first.RawText)
	}
	if !first.Schedule {
		t.Error("an artist's own page is read as a schedule")
	}

	// One page, three announcements: the anchor is what keeps them apart.
	store := dedup.NewStore()
	for _, a := range res.Announcements {
		store.Put(a)
	}
	if all := store.All(); len(all) != 3 {
		t.Fatalf("stored announcements = %d, want 3 distinct", len(all))
	}
}

func TestInlineItemsFiltersAndFallbacks(t *testing.T) {
	// The title is what the filters read, as for a list of links.
	src := inlineSource(domain.ArticleOptions{Exclude: []string{"グッズ"}})
	res := fetchInline(t, src, read(t, "higedan_list.html"))
	if len(res.Announcements) != 2 {
		t.Fatalf("announcements = %d, want 2 after excluding グッズ", len(res.Announcements))
	}
	for _, a := range res.Announcements {
		if strings.Contains(a.Title, "グッズ") {
			t.Errorf("excluded title kept: %q", a.Title)
		}
	}

	// A page without the marker is a page whose structure changed.
	empty := fetchInline(t, inlineSource(domain.ArticleOptions{}), "<html><body><p>ただいま準備中です</p></body></html>")
	if len(empty.Announcements) != 0 || len(empty.Warnings) != 1 {
		t.Fatalf("announcements=%d warnings=%v, want none and one", len(empty.Announcements), empty.Warnings)
	}
}
