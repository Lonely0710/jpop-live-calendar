package sources

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"jpop-live-calendar/internal/domain"
)

// Demo serves fictional announcements, dated around today, in the layout
// of real Japanese live news. Every artist, venue line and link is made
// up; it exists so the calendar can be tried offline and so each merge
// rule (time announced later, a deadline moved, a postponement, the same
// show written twice, an uncertain match) has something to show.
type Demo struct{}

// DemoArtists are the demo's artists with their aliases.
var DemoArtists = []domain.Artist{
	{Name: "ステラリウム", Aliases: []string{"Stellarium"}},
	{Name: "夜光クラゲ", Aliases: []string{"Yakou Kurage"}},
	{Name: "Neon Orchard", Aliases: []string{"ネオンオーチャード"}},
	{Name: "白昼夢ポスト", Aliases: []string{"Hakuchumu Post"}},
}

type demoAnn struct {
	day   int // published, days from today
	slug  string
	title string
	text  string
}

var demoAnns = []demoAnn{
	{-20, "neon-orchard-spring-party", "Neon Orchard、大阪のイベント出演決定", `▼イベント情報
"Spring Orchard Party"
{d+8} 大阪 Live House Pangea
OPEN 17:00 / START 17:30
出演:Neon Orchard / ほか`},
	{-12, "stellarium-tour-zanko", "ステラリウム、全国ツアー\"ステラリウム TOUR「残光」\"開催決定", `ステラリウムが、全国6都市を回るツアーを開催する。
▼ツアー情報
"ステラリウム TOUR「残光」"
{d+18} 大阪 BIGCAT OPEN 17:30 / START 18:30
{d+21} 東京 Spotify O-EAST
{d+25} 愛知 名古屋CLUB QUATTRO OPEN 17:30 / START 18:30
{d+32} 福岡 DRUM LOGOS OPEN 17:00 / START 18:00
{d+39} 北海道 札幌PENNY LANE 24 OPEN 17:30 / START 18:30
{d+46} 宮城 仙台Rensa OPEN 17:30 / START 18:30
※東京公演の開場/開演時間は後日発表
▼チケット情報
【オフィシャル最速先行(抽選)】
受付期間:{d-3 12:00}~{d+2 23:59}
当落発表:{d+5 18:00}
https://example.com/demo/stellarium/ticket
一般発売日:{d+12 10:00}~`},
	{-10, "white-daydream-single", "白昼夢ポスト、ニュー・シングル配信リリース", `▼リリース情報
{d+3} 配信リリース
DIGITAL SINGLE「ひるやすみ」`},
	{-8, "yakou-kurage-oneman", "夜光クラゲ、初のワンマン・ライヴ開催決定", `▼ライヴ情報
"夜光クラゲ ONE-MAN LIVE「深海」"
{d+15} 東京 渋谷WWW X
OPEN 18:00 / START 19:00
チケット:https://example.com/demo/yakou/ticket
▼チケット情報
【FC先行(抽選)】
受付期間:{d-8 20:00}~{d-1 23:59}
当落発表:{d+1 12:00}
一般発売日:{d+6 10:00}~`},
	{-6, "hakuchu-fes-2026", "白昼夢ポスト主催\"HAKUCHU FES 2026\"、2日間にわたり開催", `▼イベント情報
"HAKUCHU FES 2026"
{d+29} 神奈川 KT Zepp Yokohama
{d+30} 神奈川 KT Zepp Yokohama
OPEN 11:00 / START 12:00
出演:白昼夢ポスト / Neon Orchard / 夜光クラゲ / ステラリウム
▼チケット情報
【オフィシャル先行(先着)】
受付期間:{d+1 12:00}~{d+9 23:59}
https://example.com/demo/hakuchu/ticket`},
	{-5, "stellarium-tokyo-detail", "ステラリウム、東京公演の開演時間決定&プレイガイド先行スタート", `▼ツアー情報
"ステラリウム TOUR「残光」"
{d+21} 東京 Spotify O-EAST(東京)
OPEN 17:00 / START 18:00
▼チケット情報
【プレイガイド先行(抽選)】
受付期間:{d+3 12:00}~{d+8 23:59}
当落発表:{d+10 15:00}
https://example.com/demo/stellarium/playguide`},
	{-4, "night-aquarium", "夜光クラゲ、Neon Orchardとの昼公演イベント出演決定", `▼イベント情報
"NIGHT AQUARIUM"
{d+15} 東京 下北沢SHELTER
OPEN 12:00 / START 12:30
出演:夜光クラゲ / Neon Orchard`},
	{-3, "neon-orchard-oneman", "Neon Orchard、初のワンマン・ライヴ開催を発表", `Neon Orchardが、{d+40}に渋谷CLUB QUATTROにて初のワンマン・ライヴを開催することを発表した。開演時間などの詳細は後日発表される。`},
	{-2, "yakou-kurage-oneman-detail", "夜光クラゲ ワンマン公演 詳細", `■公演概要
"夜光クラゲ ONE-MAN LIVE「深海」"
{d+15} 東京 WWW X
OPEN 18:00 / START 19:00`},
	{-1, "stellarium-fast-extended", "ステラリウム、ツアー最速先行の受付期間を延長", `▼ツアー情報
"ステラリウム TOUR「残光」"
▼チケット情報
【オフィシャル最速先行(抽選)】
受付期間:{d-3 12:00}~{d+3 23:59}
当落発表:{d+5 18:00}`},
	{-1, "neon-orchard-spring-party-cancel", "\"Spring Orchard Party\"開催中止のお知らせ", `▼イベント情報
"Spring Orchard Party"
{d+8} 大阪 Live House Pangea 中止
出演:Neon Orchard`},
	{0, "stellarium-nagoya-postponed", "ステラリウム、名古屋公演の延期を発表", `▼ツアー情報
"ステラリウム TOUR「残光」"
{d+25} 愛知 名古屋CLUB QUATTRO 延期
振替公演:{d+60} 愛知 名古屋CLUB QUATTRO OPEN 17:30 / START 18:30
※お持ちのチケットは振替公演にそのまま有効`},
}

var demoRe = regexp.MustCompile(`\{d([+-]\d+)(?: (\d{1,2}:\d{2}))?\}`)

var jaWeekday = []string{"日", "月", "火", "水", "木", "金", "土"}

func jaDate(d domain.Date) string {
	return fmt.Sprintf("%d月%d日(%s)", int(d.Month), d.Day, jaWeekday[d.Weekday()])
}

// DemoText expands "{d+3}" and "{d-2 18:00}" relative to today.
func DemoText(s string, today domain.Date) string {
	return demoRe.ReplaceAllStringFunc(s, func(m string) string {
		g := demoRe.FindStringSubmatch(m)
		n, _ := strconv.Atoi(g[1])
		out := jaDate(today.AddDays(n))
		if g[2] != "" {
			out += g[2]
		}
		return out
	})
}

func (Demo) Fetch(_ context.Context, src domain.SourceConfig, env Env) (Result, error) {
	now := env.now().In(domain.Tokyo)
	today := domain.DateOf(now, domain.Tokyo)
	var res Result
	for i, d := range demoAnns {
		pub := today.AddDays(d.day).Time(domain.Tokyo).Add(time.Duration(9+i%8) * time.Hour)
		if pub.After(now) {
			pub = now
		}
		res.Announcements = append(res.Announcements, domain.PublicAnnouncement{
			SourceID:    src.ID,
			URL:         "https://example.com/demo/news/" + d.slug,
			Title:       d.title,
			RawText:     DemoText(d.text, today),
			PublishedAt: pub,
			FetchedAt:   now,
		})
	}
	return res, nil
}
