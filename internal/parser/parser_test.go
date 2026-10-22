package parser

import (
	"strings"
	"testing"
	"time"

	"jpop-live-calendar/internal/domain"
)

var published = time.Date(2026, 10, 15, 12, 0, 0, 0, domain.Tokyo)

func clock(c *domain.Clock) string {
	if c == nil {
		return "-"
	}
	return c.String()
}

func moment(m *domain.Moment) string {
	if m == nil {
		return "-"
	}
	return m.String()
}

func TestTourList(t *testing.T) {
	res := Parse(Input{
		Title:       "ザ・クロマニヨンズ、全国ツアー開催決定",
		ArtistHints: []string{"ザ・クロマニヨンズ"},
		Published:   published,
		Text: `ザ・クロマニヨンズが、来年2月より全国ツアーを開催する。
▼ツアー情報
"ザ・クロマニヨンズ ツアー Hola! Leopardo 2027"
2月4日（木）広島 広島LIVE VANQUISH
2月6日（土）・7日（日）福岡 DRUM LOGOS
OPEN 17:30 / START 18:30
▼リリース情報
3月3日（水）リリース
NEW ALBUM『Hola! Leopardo』`,
	})
	if len(res.Events) != 3 {
		t.Fatalf("events = %d: %+v", len(res.Events), res.Events)
	}
	e := res.Events[0]
	if e.Date.String() != "2027-02-04" || e.Prefecture != "広島" || e.Venue != "広島LIVE VANQUISH" {
		t.Errorf("first = %+v", e)
	}
	if e.Tour != "ザ・クロマニヨンズ ツアー Hola! Leopardo 2027" {
		t.Errorf("tour = %q", e.Tour)
	}
	if res.Events[2].Date.String() != "2027-02-07" || res.Events[2].Venue != "DRUM LOGOS" {
		t.Errorf("extra day = %+v", res.Events[2])
	}
	for _, e := range res.Events {
		if clock(e.OpenAt) != "17:30" || clock(e.StartAt) != "18:30" {
			t.Errorf("times of %s = %s/%s", e.Date, clock(e.OpenAt), clock(e.StartAt))
		}
	}
	if len(res.Artists) != 1 || res.Artists[0] != "ザ・クロマニヨンズ" || res.ArtistGuessed {
		t.Errorf("artists = %v guessed=%v", res.Artists, res.ArtistGuessed)
	}
}

func TestTimeNotAnnouncedStaysEmpty(t *testing.T) {
	res := Parse(Input{
		Title:     "Yu Seki、ワンマン・ライヴ開催決定",
		Published: published,
		Text: `▼ライヴ情報
"Yu Seki ONE-MAN LIVE"
10月28日（水）Zepp Shinjuku (TOKYO)
11月12日（木）大阪市内某所　18:00～`,
	})
	if len(res.Events) != 2 {
		t.Fatalf("events = %+v", res.Events)
	}
	a, b := res.Events[0], res.Events[1]
	if a.Venue != "Zepp Shinjuku (TOKYO)" || a.StartAt != nil || a.OpenAt != nil {
		t.Errorf("first = %+v", a)
	}
	if a.Date.String() != "2026-10-28" {
		t.Errorf("date = %s", a.Date)
	}
	if clock(b.StartAt) != "18:00" || b.Venue != "大阪市内某所" {
		t.Errorf("second = %+v start=%s", b, clock(b.StartAt))
	}
	if res.Artists[0] != "Yu Seki" || !res.ArtistGuessed {
		t.Errorf("artists = %v", res.Artists)
	}
}

func TestPerformersAndOpenStart(t *testing.T) {
	res := Parse(Input{
		Title:     "macico、マコトコンドウとのツーマン・イベント開催",
		Published: published,
		Text: `▼イベント情報
"macico × マコトコンドウ 2MAN"
12月20日（日）東京 下北沢SHELTER
OPEN 12:00 / START 12:30
出演：macico / マコトコンドウ
LivePocket：https://livepocket.jp/e/example`,
	})
	if len(res.Events) != 1 {
		t.Fatalf("events = %+v", res.Events)
	}
	e := res.Events[0]
	if e.Type != domain.TypeFestival || clock(e.OpenAt) != "12:00" || clock(e.StartAt) != "12:30" {
		t.Errorf("event = %+v", e)
	}
	if e.TicketURL != "https://livepocket.jp/e/example" {
		t.Errorf("ticket url = %q", e.TicketURL)
	}
	if strings.Join(res.Artists, ",") != "macico,マコトコンドウ" || res.ArtistGuessed {
		t.Errorf("artists = %v", res.Artists)
	}
}

func TestTicketRounds(t *testing.T) {
	res := Parse(Input{
		Title:     "夜光クラゲ、ワンマン公演開催",
		Published: published,
		Text: `■公演情報
"夜光クラゲ ONE-MAN LIVE「深海」"
12月5日（土）東京 Spotify O-EAST
OPEN 17:00 / START 18:00
■チケット情報
【オフィシャル最速先行（抽選）】
受付期間：10月16日（金）12:00～10月25日（日）23:59
当落発表：10月28日（水）18:00
https://example.com/ticket/fast
【プレイガイド先行（先着）】
受付期間：11月1日（日）10:00～8日（日）23:59
一般発売日：11月21日（土）10:00～`,
	})
	if len(res.Events) != 1 || res.Events[0].Venue != "Spotify O-EAST" {
		t.Fatalf("events = %+v", res.Events)
	}
	if len(res.Rounds) != 3 {
		t.Fatalf("rounds = %+v", res.Rounds)
	}
	fast, pg, gen := res.Rounds[0], res.Rounds[1], res.Rounds[2]
	if fast.Type != domain.RoundOfficial || !fast.Lottery ||
		moment(fast.OpensAt) != "2026-10-16 12:00" || moment(fast.ClosesAt) != "2026-10-25 23:59" ||
		moment(fast.ResultAt) != "2026-10-28 18:00" || fast.URL != "https://example.com/ticket/fast" {
		t.Errorf("fast = %+v %s %s %s", fast, moment(fast.OpensAt), moment(fast.ClosesAt), moment(fast.ResultAt))
	}
	if pg.Type != domain.RoundPlayguide || pg.Lottery || moment(pg.ClosesAt) != "2026-11-08 23:59" {
		t.Errorf("playguide = %+v closes=%s", pg, moment(pg.ClosesAt))
	}
	if gen.Type != domain.RoundGeneral || moment(gen.OpensAt) != "2026-11-21 10:00" || gen.ClosesAt != nil {
		t.Errorf("general = %+v opens=%s", gen, moment(gen.OpensAt))
	}
}

func TestPostponed(t *testing.T) {
	res := Parse(Input{
		Title:     "ステラリウム、大阪公演の延期を発表",
		Published: published,
		Text: `▼公演情報
11月3日（火・祝）大阪 BIGCAT 延期
振替公演：1月16日（土）大阪 BIGCAT`,
	})
	if len(res.Events) < 1 || res.Events[0].Status != domain.StatusPostponed {
		t.Fatalf("events = %+v", res.Events)
	}
	if res.Status != domain.StatusPostponed {
		t.Errorf("title status = %q", res.Status)
	}
}

func TestReleaseOnlyHasNoEvents(t *testing.T) {
	res := Parse(Input{
		Title:     "白昼夢ポスト、ニュー・シングル配信リリース",
		Published: published,
		Text: `▼リリース情報
11月11日（水）配信リリース
DIGITAL SINGLE「ひかり」`,
	})
	if len(res.Events) != 0 || len(res.Rounds) != 0 {
		t.Errorf("got %+v %+v", res.Events, res.Rounds)
	}
}

func TestProseNeedsReview(t *testing.T) {
	res := Parse(Input{
		Title:     "Neon Orchard、初のワンマン・ライヴ開催",
		Published: published,
		Text:      `Neon Orchardが、12月12日（土）に渋谷WWWにて初のワンマン・ライヴを開催する。`,
	})
	if len(res.Notes) == 0 {
		t.Errorf("prose should need review: %+v", res)
	}
}

func TestGuestAppearanceIsNotTheArtistsSchedule(t *testing.T) {
	res := Parse(Input{
		Title:     "TK from 凛として時雨の日本武道館公演にsuisがゲスト出演。",
		Published: published, Schedule: true, ArtistHints: []string{"ヨルシカ"},
		Text: `2026.10.09
suis
12月27日（日）に日本武道館にて行われるTK from 凛として時雨のライブにsuisがゲスト出演することが決定しました。
TK from 凛として時雨「TOUR 2026 生癖 -seiheki-」
15th Anniversary FINAL
12月27日 (日) 東京・日本武道館`,
	})
	if len(res.Events) != 0 || res.Tour != "" {
		t.Fatalf("a member's guest appearance is not the artist's schedule: events=%+v tour=%q", res.Events, res.Tour)
	}
	res = Parse(Input{
		Title:     "ヨルシカ、ROCK FES 2027にゲスト出演決定",
		Published: published, Schedule: true, ArtistHints: []string{"ヨルシカ"},
		Text: "2027.07.18(日) 千葉 幕張メッセ",
	})
	if len(res.Events) != 1 {
		t.Fatalf("the artist's own festival appearance: events=%+v", res.Events)
	}
}

func TestScheduleDateAsOfIsNotAShow(t *testing.T) {
	res := Parse(Input{
		Title: "ヨルシカ LIVE TOUR 2026「一人称」", Published: published, Schedule: true, ArtistHints: []string{"ヨルシカ"},
		Text: `2026年3月3日現在のイメージレイアウトとなります。予めご了承ください。
2026.10.09(金) 東京 渋谷WWW`,
	})
	if len(res.Events) != 1 || res.Events[0].Date.String() != "2026-10-09" || res.Events[0].Venue != "渋谷WWW" {
		t.Fatalf("events = %+v", res.Events)
	}
}

func TestScheduleNewsBadgeIsNotAShow(t *testing.T) {
	res := Parse(Input{
		Title: "ZUTOMAYO Hall Tour 2027 「アピクラ電機」LÚ.NIM.LÀL", Published: published, Schedule: true, ArtistHints: []string{"ZUTOMAYO"},
		Text: `2026.10.16new
HALL TOUR 2027 「アピクラ電機」LÚ.NIM.LÀL 開催決定
01.15(FRI) 広島県 広島文化学園HBGホール OPEN 17:00 / START 18:00`,
	})
	if len(res.Events) != 1 || res.Events[0].Date.String() != "2027-01-15" || res.Events[0].Venue != "広島文化学園HBGホール" {
		t.Fatalf("events = %+v", res.Events)
	}
}

func TestScheduleYearDaysAndDuplicates(t *testing.T) {
	res := Parse(Input{
		Title: "ZEPP TOUR 2027", Published: published, Schedule: true, ArtistHints: []string{"X"},
		Text: `ZEPP TOUR 2027
<日程・会場>
2027年
2/12(金) 13(土) 大阪 Zepp Osaka Bayside
2/20(土) 仙台 宮城・仙台GIGS <Open 17:00 / Start 18:00>
2/20(土) 宮城 | 仙台GIGS
<チケット>
〈オフィシャル最速先行(抽選)〉
受付期間:1月5日(火)12:00~1月11日(月)23:59
*2/20公演のみの受付となります。
プレイガイド先行
受付期間:2026/8/20(木)12:00~2025/9/21(月)23:59`,
	})
	var got []string
	for _, e := range res.Events {
		got = append(got, e.Date.String()+" "+e.Prefecture+" "+e.Venue+" "+clock(e.StartAt))
	}
	want := []string{
		"2027-02-12 大阪 Zepp Osaka Bayside -",
		"2027-02-13 大阪 Zepp Osaka Bayside -",
		"2027-02-20 宮城 仙台GIGS 18:00",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("events:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(res.Rounds) != 2 {
		t.Fatalf("rounds = %+v", res.Rounds)
	}
	fast, pg := res.Rounds[0], res.Rounds[1]
	if fast.Name != "オフィシャル最速先行(抽選)" || !fast.Lottery || moment(fast.OpensAt) != "2027-01-05 12:00" || moment(fast.ClosesAt) != "2027-01-11 23:59" {
		t.Errorf("a round named on its own line, its note kept out: %+v %s %s", fast, moment(fast.OpensAt), moment(fast.ClosesAt))
	}
	if moment(pg.ClosesAt) != "2026-09-21 23:59" {
		t.Errorf("a deadline before its opening is a typo in the year: %s", moment(pg.ClosesAt))
	}
}

func TestRoundsBeforeShows(t *testing.T) {
	res := Parse(Input{
		Title: "YURIN LIVE", Published: published, Schedule: true, ArtistHints: []string{"優里"},
		Text: `YURIN LIVE
2026.10.27(火) 大阪府 大阪城ホール
18:00 / 19:00
■チケット・公演情報
ゆーりんち限定最速先行(抽選)
受付期間:7月25日(土)21:00~8月2日(日)23:59`,
	})
	if len(res.Events) != 1 || len(res.Rounds) != 1 {
		t.Fatalf("events=%+v rounds=%+v", res.Events, res.Rounds)
	}
	if r := res.Rounds[0]; moment(r.OpensAt) != "2026-07-25 21:00" || moment(r.ClosesAt) != "2026-08-02 23:59" {
		t.Errorf("tickets are sold before the show: %s ~ %s", moment(r.OpensAt), moment(r.ClosesAt))
	}
}

func TestQuotedTourNeedsOnePair(t *testing.T) {
	res := Parse(Input{
		Title: "「Mrs. GREEN APPLE Ringo Jam Tour “SHADOWS”」北海道公演", Published: published, Schedule: true, ArtistHints: []string{"Mrs. GREEN APPLE"},
		Text: `「Mrs. GREEN APPLE Ringo Jam Tour “SHADOWS”」北海道公演 -Mrs. GREEN APPLE OFFICIAL SITE|OFFICIAL FAN CLUB「Ringo Jam」
2026.11.03(火) 北海道 真駒内セキスイハイムアイスアリーナ
「Mrs. GREEN APPLE Ringo Jam Tour “SHADOWS”」`,
	})
	if res.Tour != "Mrs. GREEN APPLE Ringo Jam Tour “SHADOWS”" {
		t.Errorf("tour = %q", res.Tour)
	}
}

func TestTourFromTitleAndBody(t *testing.T) {
	cases := []struct{ title, text, want string }{
		{"「Mrs. GREEN APPLE Ringo Jam Tour “SHADOWS”」香川公演", "開催日時：2026年10月1日(木)\n開場 17:00 / 開演 18:00\n会場：あなぶきアリーナ香川", "Mrs. GREEN APPLE Ringo Jam Tour “SHADOWS”"},
		{"RADWIMPS Official Fan Club Tour 2026 “ユーリンチー”", "2026年5月29日(金) Zepp Haneda", "RADWIMPS Official Fan Club Tour 2026 “ユーリンチー”"},
		{"北九州メッセ（福岡県）", "LIVE STREAMING\nTOUR\nVaundy ASIA ARENA TOUR 2026 \"HORO\"\n＜公演詳細＞\n会場 : 北九州メッセ（福岡県）\n日時 : 2026年10月24日(土) 16:30 開場 / 18:00 開演", "Vaundy ASIA ARENA TOUR 2026 \"HORO\""},
	}
	for _, c := range cases {
		res := Parse(Input{Title: c.title, Text: c.text, Published: published, ArtistHints: []string{"X"}})
		if res.Tour != c.want || len(res.Events) == 0 {
			t.Errorf("%s: tour = %q (%d events), want %q", c.title, res.Tour, len(res.Events), c.want)
		}
	}
}

func TestSchedulePostsAndSplitDays(t *testing.T) {
	res := Parse(Input{
		Title: "LIVE SCHEDULE", Published: published, Schedule: true, ArtistHints: []string{"美波"},
		Text: `2026.10.02 13:00
美波「Timesick Traveler 2027」国内・アジアツアーの開催が決定いたしました
【国内公演日程】
1月23日（土）愛知 / Niterra日本特殊陶業市民会館ビレッジホール
OPEN 17:00 / START 18:00`,
	})
	if len(res.Events) != 1 || res.Events[0].Tour != "Timesick Traveler 2027" {
		t.Fatalf("events = %+v", res.Events)
	}
	res = Parse(Input{
		Title: "ヨルシカ LIVE TOUR 2026「一人称」", Published: published, Schedule: true, ArtistHints: []string{"ヨルシカ"},
		Text: "2026\n03.21\nSAT\n宮城\nゼビオアリーナ仙台\nOPEN / START17:00 / 18:00\n2026\n07.29\nWED\n会場：横浜アリーナ（https://www.yokohama-arena.co.jp/）",
	})
	if len(res.Events) != 2 || res.Events[0].Venue != "ゼビオアリーナ仙台" || res.Events[0].Prefecture != "宮城" || res.Events[1].Venue != "横浜アリーナ" {
		t.Fatalf("events = %+v", res.Events)
	}
	// The same site's detail pages write "OPEN / START：17:00 / 18:00"
	// (full-width colon) under the venue.
	res = Parse(Input{
		Title: "LIVE TOUR 2026「一人称」", Published: published, Schedule: true, ArtistHints: []string{"ヨルシカ"},
		Text: "日程：2026年3月21日(土)\n会場：ゼビオアリーナ仙台（\nhttps://www.xebioarena.com/\n）\nOPEN / START：17:00 / 18:00",
	})
	if len(res.Events) != 1 || res.Events[0].OpenAt == nil || res.Events[0].OpenAt.String() != "17:00" ||
		res.Events[0].StartAt == nil || res.Events[0].StartAt.String() != "18:00" {
		t.Fatalf("full-width colon times = %+v", res.Events)
	}
}

func TestTourIn(t *testing.T) {
	cases := map[string]string{
		"6月27(土)、28日(日)なとり ONE-MAN LIVE TOUR「行進」 愛知県芸術劇場 大ホール公演実施に関するご案内": "なとり ONE-MAN LIVE TOUR「行進」",
		"【SOLD OUT】あたらよ ASIA TOUR 2026 『夕立が去ったその後で』":                      "あたらよ ASIA TOUR 2026 『夕立が去ったその後で』",
		"milet 2年ぶり2度目のアジアツアー “milet Asia Tour 2027”の開催が決定!":              "milet Asia Tour 2027",
		"back number \"Grateful Yesterdays Tour 2026\"Live at 東京ドーム":      "",
		"2026年 全国ホールツアー 『なとり ONE-MAN LIVE TOUR「行進」』開催決定":                  "なとり ONE-MAN LIVE TOUR「行進」",
		"one room party vol.8【山形】（＊FC会員限定ライブ）":                            "one room party vol.8",
		"「ゆーりんち」限定!初の日本武道館公演開催決定!":                                        "",
	}
	for in, want := range cases {
		if got := tourIn(in); got != want {
			t.Errorf("tourIn(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVenueAboveDates(t *testing.T) {
	res := Parse(Input{
		Title: "『ユイカ』TOUR「Anemone」開催決定!", Published: published, Schedule: true, ArtistHints: []string{"『ユイカ』"},
		Text: `09.21
『ユイカ』TOUR「Anemone」開催決定！
2026.09.21
LIVE/EVENT
2027年3月より、『ユイカ』TOUR「Anemone」の開催が決定しました！
<『ユイカ』コメント>
まだまだビッグになるために進化していくので、ぜひAnemoneツアーでお会いしましょう！
本日(14日)より、『ユイカ』TOUR「Anemone」のオフィシャル最速先行がスタートしました！
『ユイカ』TOUR「Anemone」
<埼玉>大宮ソニックシティ
2027年3月13日(土)17:00開場／18:00開演
<大阪>NHK大阪ホール
2027年3月20日(土)17:00開場／18:00開演
2027年3月21日(日)15:00開場／16:00開演
【チケット料金】
全席指定 7,700円(税込)
<ファンクラブ先行>
■受付期間：2026年9月21日(月・祝)21:30〜2026年10月4日(日)23:59`,
	})
	if len(res.Events) != 3 {
		t.Fatalf("events = %+v", res.Events)
	}
	want := []string{"大宮ソニックシティ", "NHK大阪ホール", "NHK大阪ホール"}
	for i, e := range res.Events {
		if e.Venue != want[i] || e.OpenAt == nil || e.StartAt == nil || e.StartAt.Hour-e.OpenAt.Hour != 1 || e.Tour != "『ユイカ』TOUR「Anemone」" {
			t.Errorf("event %d = %+v", i, e)
		}
	}
	if res.Events[0].Prefecture != "埼玉" {
		t.Errorf("pref = %q", res.Events[0].Prefecture)
	}
	var fc *Round
	for i := range res.Rounds {
		if res.Rounds[i].Name == "ファンクラブ先行" {
			fc = &res.Rounds[i]
		}
	}
	if fc == nil || fc.OpensAt == nil || fc.ClosesAt == nil || fc.ClosesAt.Date.String() != "2026-10-04" {
		t.Errorf("rounds = %+v", res.Rounds)
	}
}
