package sources

import "testing"

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
