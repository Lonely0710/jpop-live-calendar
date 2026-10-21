package dedup

import (
	"strings"
	"testing"
	"time"

	"jpop-live-calendar/internal/domain"
)

func at(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, domain.Tokyo) }

func ann(src, url, title, text string, published time.Time) domain.PublicAnnouncement {
	return domain.PublicAnnouncement{SourceID: src, URL: url, Title: title, RawText: text, PublishedAt: published, FetchedAt: published}
}

const first = `▼ツアー情報
"ステラリウム TOUR 2026「残光」"
11月3日（火・祝）大阪 BIGCAT
11月8日（日）東京 Spotify O-EAST（東京）
▼チケット情報
【オフィシャル最速先行（抽選）】
受付期間：10月12日（月）12:00～10月20日（火）23:59
当落発表：10月23日（金）18:00`

func TestStoreRevisions(t *testing.T) {
	s := NewStore()
	a := ann("x", "https://example.com/n/1/?utm_source=rss", "t", "body", at(1, 9))
	if s.Put(a) != New {
		t.Fatal("first put should be new")
	}
	b := a
	b.URL = "https://example.com/n/1"
	if s.Put(b) != Unchanged {
		t.Fatal("same text under the same URL should be unchanged")
	}
	b.RawText = "body updated"
	if s.Put(b) != Updated || s.All()[0].Revision != 2 {
		t.Fatal("changed text should be a new revision")
	}
	meta := b
	meta.RawText, meta.MetadataOnly = "", true
	if s.Put(meta) != Unchanged || s.All()[0].RawText != "body updated" {
		t.Fatal("title-only copy must not replace the body")
	}
}

func TestRestoredCopyKeepsRevision(t *testing.T) {
	s := NewStore()
	a := ann("x", "https://example.com/n/2", "t", "body", at(2, 9))
	a.Revision = 4
	if s.Put(a) != New || s.All()[0].Revision != 4 {
		t.Fatalf("restored revision = %d, want 4", s.All()[0].Revision)
	}
	b := a
	b.RawText = "body updated"
	if s.Put(b) != Updated || s.All()[0].Revision != 5 {
		t.Fatalf("next revision = %d, want 5", s.All()[0].Revision)
	}
}

func TestMergeAcrossAnnouncements(t *testing.T) {
	anns := []domain.PublicAnnouncement{
		ann("a", "https://a.example/1", "ステラリウム、全国ツアー開催", first, at(10, 12)),
		// Another source repeats the Tokyo show with the time and a venue
		// spelled differently, and adds a round.
		ann("b", "https://b.example/9", "ステラリウム、東京公演の詳細発表", `▼公演情報
"ステラリウム TOUR 2026「残光」"
11月8日（日）東京 Spotify O-EAST
OPEN 17:00 / START 18:00
▼チケット情報
【プレイガイド先行（抽選）】
受付期間：10月24日（土）12:00～11月1日（日）23:59`, at(11, 12)),
		// The official round's deadline moves.
		ann("a", "https://a.example/2", "ステラリウム、最速先行の受付期間延長", `▼ツアー情報
"ステラリウム TOUR 2026「残光」"
▼チケット情報
【オフィシャル最速先行（抽選）】
受付期間：10月12日（月）12:00～10月22日（木）23:59`, at(15, 12)),
		ann("a", "https://a.example/3", "ステラリウム、大阪公演の延期を発表", `▼公演情報
11月3日（火・祝）大阪 BIGCAT 延期
振替公演：1月16日（土）大阪 BIGCAT`, at(20, 12)),
	}
	opt := Options{Artists: []domain.Artist{{Name: "ステラリウム", Aliases: []string{"Stellarium"}}}}
	res := Build(anns, opt)
	if len(res.Events) != 3 {
		for _, e := range res.Events {
			t.Logf("%s %s %s", e.StartDate, e.VenueName, e.Title)
		}
		t.Fatalf("events = %d, want 3 (Osaka, Tokyo, 振替)", len(res.Events))
	}
	var tokyo, osaka *domain.PublicEvent
	for i := range res.Events {
		switch res.Events[i].StartDate.String() {
		case "2026-11-08":
			tokyo = &res.Events[i]
		case "2026-11-03":
			osaka = &res.Events[i]
		}
	}
	if tokyo == nil || tokyo.StartAt == nil || tokyo.StartAt.String() != "18:00" || len(tokyo.SourceIDs) != 2 {
		t.Fatalf("tokyo = %+v", tokyo)
	}
	if len(tokyo.Changes) == 0 || tokyo.Changes[0].Old != "未公布" {
		t.Errorf("tokyo changes = %+v", tokyo.Changes)
	}
	if osaka == nil || osaka.Status != domain.StatusPostponed || len(osaka.Changes) != 1 {
		t.Fatalf("osaka = %+v", osaka)
	}
	if len(res.Rounds) != 2 {
		t.Fatalf("rounds = %+v", res.Rounds)
	}
	fast := res.Rounds[0]
	if fast.ClosesAt.String() != "2026-10-22 23:59" || len(fast.Changes) != 1 || fast.Changes[0].Old != "2026-10-20 23:59" {
		t.Errorf("deadline change = %+v %+v", fast.ClosesAt, fast.Changes)
	}
	if len(fast.EventIDs) != 2 {
		t.Errorf("fast round shows = %v", fast.EventIDs)
	}
	if len(res.Rounds[1].EventIDs) != 1 || res.Rounds[1].EventIDs[0] != tokyo.ID {
		t.Errorf("playguide round shows = %v", res.Rounds[1].EventIDs)
	}
	kinds := map[domain.EntryKind]int{}
	for _, e := range res.Entries {
		kinds[e.Kind]++
	}
	if kinds[domain.EntryLive] != 3 || kinds[domain.EntryLotteryOpen] != 2 || kinds[domain.EntryLotteryClose] != 2 || kinds[domain.EntryResult] != 1 {
		t.Errorf("entry kinds = %v", kinds)
	}
	for _, e := range res.Entries {
		if e.Kind == domain.EntryLotteryClose && e.RoundID == fast.ID && !e.Changed {
			t.Error("moved deadline should be marked changed")
		}
		if !strings.Contains(e.Search, "stellarium") {
			t.Errorf("alias not searchable: %q", e.Search)
		}
	}
}

func TestSameDayDifferentVenueNeedsReview(t *testing.T) {
	res := Build([]domain.PublicAnnouncement{
		ann("a", "https://a.example/1", "夜光クラゲ、ワンマン開催", "▼ライヴ情報\n12月5日（土）東京 Spotify O-EAST", at(1, 9)),
		ann("b", "https://b.example/1", "夜光クラゲ、イベント出演", "▼イベント情報\n12月5日（土）東京 渋谷CLUB QUATTRO", at(2, 9)),
	}, Options{Artists: []domain.Artist{{Name: "夜光クラゲ"}}})
	if len(res.Events) != 2 || !res.Events[0].NeedsReview || !res.Events[1].NeedsReview {
		t.Fatalf("events = %+v", res.Events)
	}
}

func TestCompatibilityIdeographVenueMerges(t *testing.T) {
	res := Build([]domain.PublicAnnouncement{
		ann("a", "https://a.example/1", "ステラリウム、愛知公演の詳細", `"ステラリウム TOUR 2026「残光」"
11月8日（日）愛知 日本ガイシホール
OPEN 17:00 / START 18:00`, at(10, 12)),
		// The feature page spells the venue with a KANGXI radical (⽇).
		ann("b", "https://b.example/1", "ステラリウム、全国ツアー開催", `"ステラリウム TOUR 2026「残光」"
11月8日（日）愛知 ⽇本ガイシホール`, at(11, 12)),
	}, Options{Artists: []domain.Artist{{Name: "ステラリウム"}}})
	if len(res.Events) != 1 || len(res.Events[0].SourceIDs) != 2 || res.Events[0].VenueName != "日本ガイシホール" {
		t.Fatalf("events = %+v", res.Events)
	}
}

func TestIncludeFilter(t *testing.T) {
	res := Build([]domain.PublicAnnouncement{
		ann("demo", "https://a.example/1", "夜光クラゲ、ワンマン開催", "▼ライヴ情報\n12月5日（土）東京 Spotify O-EAST", at(1, 9)),
	}, Options{Include: func(id string) bool { return id != "demo" }})
	if len(res.Events) != 0 {
		t.Fatal("excluded source should not build events")
	}
}
