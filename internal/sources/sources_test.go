package sources

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"jpop-live-calendar/internal/dedup"
	"jpop-live-calendar/internal/domain"
)

type fakeGetter struct {
	pages map[string]string
	got   []string
}

func (f *fakeGetter) Get(_ context.Context, url string) (*Response, error) {
	f.got = append(f.got, url)
	p, ok := f.pages[url]
	if !ok {
		return nil, errors.New("404")
	}
	return &Response{URL: url, Status: 200, Text: p}, nil
}

func read(t *testing.T, name string) string {
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var skreamLike = domain.ArticleOptions{
	Enabled: true, BodyStart: `<div id="entrybody">`, BodyEnd: `<div id="categories">`,
	ArtistStart: `<h3>関連アーティスト</h3>`, ArtistEnd: `</div>`,
	Keywords: []string{"ツアー", "ライヴ"}, MaxPerSync: 5,
}

func TestParseFeed(t *testing.T) {
	items, err := ParseFeed(read(t, "feed.xml"))
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	if items[0].Published.Format(time.RFC3339) != "2026-10-20T12:00:00+09:00" || items[0].Categories[0] != "Example Band(エグザンプル・バンド)" {
		t.Errorf("item = %+v", items[0])
	}
	atom := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><title>A</title><link rel="alternate" href="https://x.example/a"/><updated>2026-10-01T10:00:00+09:00</updated><summary>s</summary></entry></feed>`
	items, err = ParseFeed(atom)
	if err != nil || len(items) != 1 || items[0].Link != "https://x.example/a" {
		t.Fatalf("atom = %+v %v", items, err)
	}
	if _, err := ParseFeed("<html><body>not a feed</body></html>"); err == nil {
		t.Error("html should not parse as a feed")
	}
}

func TestRSSOpensOnlyLiveArticles(t *testing.T) {
	g := &fakeGetter{pages: map[string]string{
		"https://news.example/feed.xml":                      read(t, "feed.xml"),
		"https://news.example/2026/10/example_band_tour.php": read(t, "article.html"),
	}}
	src := domain.SourceConfig{ID: "ex", Kind: domain.SourceRSS, URL: "https://news.example/feed.xml", Article: skreamLike}
	res, err := RSS{}.Fetch(context.Background(), src, Env{Get: g})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.got) != 2 {
		t.Errorf("fetched %v; the release article must not be opened", g.got)
	}
	if len(res.Announcements) != 2 {
		t.Fatalf("announcements = %d", len(res.Announcements))
	}
	tour := res.Announcements[0]
	if tour.MetadataOnly || strings.Contains(tour.RawText, "ここは本文ではない") || !strings.Contains(tour.RawText, "▼ツアー情報") {
		t.Errorf("body = %q", tour.RawText)
	}
	if strings.Join(tour.ArtistHints, "|") != "Example Band|ゲスト" {
		t.Errorf("hints = %v", tour.ArtistHints)
	}
	if !res.Announcements[1].MetadataOnly {
		t.Error("unopened item should be metadata only")
	}

	// Known pages are not fetched again.
	g.got = nil
	_, err = RSS{}.Fetch(context.Background(), src, Env{Get: g, Known: func(u string) bool { return strings.Contains(u, "tour") }})
	if err != nil || len(g.got) != 1 {
		t.Errorf("second sync fetched %v, %v", g.got, err)
	}

	built := dedup.Build(res.Announcements, dedup.Options{})
	if len(built.Events) != 2 || len(built.Rounds) != 1 || len(built.Rounds[0].EventIDs) != 2 {
		t.Fatalf("built %d events %d rounds", len(built.Events), len(built.Rounds))
	}
	for _, e := range built.Events {
		if e.StartAt == nil || e.StartAt.String() != "18:30" || e.StartDate.Year != 2027 {
			t.Errorf("event %+v", e)
		}
	}
}

func TestRSSMissingMarkerWarns(t *testing.T) {
	g := &fakeGetter{pages: map[string]string{
		"https://news.example/feed.xml":                      read(t, "feed.xml"),
		"https://news.example/2026/10/example_band_tour.php": "<html><body><p>changed layout</p></body></html>",
	}}
	src := domain.SourceConfig{ID: "ex", URL: "https://news.example/feed.xml", Article: skreamLike}
	res, err := RSS{}.Fetch(context.Background(), src, Env{Get: g})
	if err != nil || len(res.Warnings) == 0 {
		t.Fatalf("warnings = %v err = %v", res.Warnings, err)
	}
}

func TestHTMLText(t *testing.T) {
	got := HTMLText("<p>a<br />b&amp;c</p><script>x()</script><p>　d　e </p>")
	if got != "a\nb&c\n\nd e" {
		t.Errorf("got %q", got)
	}
	// A post's date and its category share a paragraph on the Yorushika
	// site; the category is not part of the date.
	got = HTMLText(`<p class="date">2026.10.09<span class="category">suis</span></p>`)
	if got != "2026.10.09\nsuis" {
		t.Errorf("category label: got %q", got)
	}
}

func TestDemoBuildsEveryCase(t *testing.T) {
	now := time.Date(2026, 10, 20, 15, 0, 0, 0, domain.Tokyo)
	res, err := Demo{}.Fetch(context.Background(), domain.SourceConfig{ID: "demo"}, Env{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	b := dedup.Build(res.Announcements, dedup.Options{Artists: DemoArtists})
	byDate := map[string][]domain.PublicEvent{}
	for _, e := range b.Events {
		byDate[e.StartDate.String()] = append(byDate[e.StartDate.String()], e)
	}
	// Stellarium Tokyo: written twice, time filled in later.
	tokyo := byDate["2026-11-10"]
	if len(tokyo) != 1 || tokyo[0].StartAt == nil || tokyo[0].StartAt.String() != "18:00" || len(tokyo[0].AnnouncementIDs) != 2 {
		t.Fatalf("tokyo = %+v", tokyo)
	}
	// Yakou Kurage: one-man written twice (merged) and a day event at
	// another venue and time (a separate show, nothing to review).
	nov4 := byDate["2026-11-04"]
	if len(nov4) != 2 || nov4[0].NeedsReview || nov4[1].NeedsReview {
		t.Fatalf("nov 4 = %+v", nov4)
	}
	if nagoya := byDate["2026-11-14"]; len(nagoya) != 1 || nagoya[0].Status != domain.StatusPostponed {
		t.Fatalf("nagoya = %+v", nagoya)
	}
	if moved := byDate["2026-12-19"]; len(moved) != 1 || len(moved[0].Notes) == 0 || moved[0].Status != domain.StatusAnnounced {
		t.Fatalf("振替 = %+v", moved)
	}
	if party := byDate["2026-10-28"]; len(party) != 1 || party[0].Status != domain.StatusCancelled || len(party[0].Changes) == 0 {
		t.Fatalf("cancelled = %+v", party)
	}
	if prose := byDate["2026-11-29"]; len(prose) != 1 || !prose[0].NeedsReview {
		t.Fatalf("prose = %+v", prose)
	}
	if fes := byDate["2026-11-18"]; len(fes) != 1 || len(fes[0].ArtistIDs) != 4 || fes[0].Type != domain.TypeFestival {
		t.Fatalf("fes = %+v", fes)
	}
	var moved bool
	for _, r := range b.Rounds {
		if strings.Contains(r.Name, "最速") && len(r.Changes) == 1 && r.ClosesAt.Date.String() == "2026-10-23" {
			moved = true
		}
		if len(r.EventIDs) == 0 {
			t.Errorf("round %s not linked", r.Name)
		}
	}
	if !moved {
		t.Error("最速先行 deadline change not recorded")
	}
	if len(b.Events) != 13 {
		for _, e := range b.Events {
			t.Logf("%s %s %s", e.StartDate, e.VenueName, e.Title)
		}
		t.Errorf("events = %d", len(b.Events))
	}
}

func TestSonyLiveFeed(t *testing.T) {
	u := "https://www.sonymusic.co.jp/json/v2/artist/ryokusyaka/live/start/0/count/100"
	g := &fakeGetter{pages: map[string]string{u: read(t, "sony_live.jsonp")}}
	src := domain.SourceConfig{ID: "s", Kind: domain.SourceAPI, URL: u, Artist: "緑黄色社会"}
	res, err := API{}.Fetch(context.Background(), src, Env{Get: g, Now: func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, domain.Tokyo) }})
	if err != nil || len(res.Announcements) != 2 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	a := res.Announcements[0]
	if a.Title != `ARENA TOUR 2026 "あたまご"` || a.URL != "https://www.sonymusic.co.jp/artist/ryokusyaka/live/55020" || !a.Schedule {
		t.Errorf("announcement = %+v", a)
	}
	if !strings.Contains(a.RawText, "2026.12.19(土) 千葉県 LaLa arena TOKYO-BAY\n17:00 / 18:00") {
		t.Errorf("shows not written as a schedule:\n%s", a.RawText)
	}
	if strings.Contains(a.RawText, "9月19日(土) 千葉") || !strings.Contains(a.RawText, "オフィシャル最速先行") {
		t.Errorf("the notes should keep the tickets, not their own list of shows:\n%s", a.RawText)
	}
	if b := res.Announcements[1]; !strings.Contains(b.RawText, "2/12(金) 13(土) 大阪 Zepp Osaka Bayside") {
		t.Errorf("notes-only tour = %q", b.RawText)
	}
}

func TestRSSInclude(t *testing.T) {
	g := &fakeGetter{pages: map[string]string{
		"https://news.example/feed.xml": read(t, "feed.xml"),
	}}
	src := domain.SourceConfig{
		ID: "ex", Kind: domain.SourceRSS, URL: "https://news.example/feed.xml",
		Article: domain.ArticleOptions{Include: "全国ツアー"},
	}
	res, err := RSS{}.Fetch(context.Background(), src, Env{Get: g})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Announcements) != 1 || !strings.Contains(res.Announcements[0].Title, "全国ツアー") {
		t.Fatalf("announcements = %+v", res.Announcements)
	}
	if len(g.got) != 1 {
		t.Errorf("fetched %v; a metadata-only include must not open pages", g.got)
	}
}

func TestRSSTitleOnlyAndMaxAge(t *testing.T) {
	g := &fakeGetter{pages: map[string]string{
		"https://news.example/feed.xml": read(t, "feed.xml"),
	}}
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, domain.Tokyo)
	src := domain.SourceConfig{
		ID: "ex", Kind: domain.SourceRSS, URL: "https://news.example/feed.xml",
		Article: domain.ArticleOptions{Enabled: true, TitleOnly: true, MaxAge: 24 * time.Hour, Keywords: []string{"存在しない記事"}},
	}
	res, err := RSS{}.Fetch(context.Background(), src, Env{Get: g, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Announcements) != 1 {
		t.Fatalf("announcements = %+v (the 2 day old item should be skipped)", res.Announcements)
	}
	a := res.Announcements[0]
	if !a.MetadataOnly || a.RawText != "" {
		t.Errorf("title-only item = %+v", a)
	}
	if len(g.got) != 1 {
		t.Errorf("fetched %v; no article should be opened", g.got)
	}
}
