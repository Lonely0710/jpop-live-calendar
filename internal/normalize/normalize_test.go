package normalize

import (
	"testing"
	"time"

	"jpop-live-calendar/internal/domain"
)

func TestText(t *testing.T) {
	if got := Text("１２：００～　（土）"); got != "12:00~ (土)" {
		t.Fatalf("got %q", got)
	}
	if got := Text("⽇本ガイシホール"); got != "日本ガイシホール" {
		t.Fatalf("compatibility ideograph: got %q", got)
	}
}

func TestKey(t *testing.T) {
	if Key("Zepp Shinjuku (TOKYO)") != Key("ＺｅｐｐＳｈｉｎｊｕｋｕ（ｔｏｋｙｏ）") {
		t.Fatal("width/space folding")
	}
	if Key("⽇本ガイシホール") != Key("日本ガイシホール") {
		t.Fatal("compatibility ideograph")
	}
}

func TestInferYear(t *testing.T) {
	ref := time.Date(2026, 10, 20, 12, 0, 0, 0, domain.Tokyo)
	thu := time.Thursday
	if y, ok := InferYear(2, 4, &thu, ref); y != 2027 || !ok { // 2027-02-04 is a Thursday
		t.Fatalf("got %d %v", y, ok)
	}
	if y, _ := InferYear(11, 28, nil, ref); y != 2026 {
		t.Fatalf("got %d", y)
	}
	if y, _ := InferYear(9, 1, nil, ref); y != 2026 { // within 60 days back
		t.Fatalf("got %d", y)
	}
	if y, _ := InferYear(3, 1, nil, ref); y != 2027 {
		t.Fatalf("got %d", y)
	}
	mon := time.Monday
	if _, ok := InferYear(11, 28, &mon, time.Date(2026, 11, 1, 0, 0, 0, 0, domain.Tokyo)); ok {
		// 2026-11-28 is a Saturday; 2027-11-28 a Sunday; 2028-11-28 a Tuesday
		t.Fatal("weekday mismatch must be reported")
	}
}

func TestParseClock(t *testing.T) {
	for in, want := range map[string]domain.Clock{"18:00": {Hour: 18, Minute: 0}, "18時": {Hour: 18, Minute: 0}, "開演 18時30分": {Hour: 18, Minute: 30}, "25:00": {Hour: 25, Minute: 0}} {
		if got, ok := ParseClock(in); !ok || got != want {
			t.Errorf("%q: %v %v", in, got, ok)
		}
	}
}

func TestSplitPrefecture(t *testing.T) {
	p, v := SplitPrefecture("広島 広島LIVE VANQUISH")
	if p != "広島" || v != "広島LIVE VANQUISH" {
		t.Fatalf("%q %q", p, v)
	}
	p, v = SplitPrefecture("Zepp Shinjuku (TOKYO)")
	if p != "" || v != "Zepp Shinjuku (TOKYO)" {
		t.Fatalf("%q %q", p, v)
	}
}

func TestArtists(t *testing.T) {
	r := NewArtists([]domain.Artist{{Name: "ザ・クロマニヨンズ", Aliases: []string{"The Cro-Magnons"}}})
	a := r.Resolve("THE CRO-MAGNONS")
	if a == nil || a.Name != "ザ・クロマニヨンズ" {
		t.Fatalf("got %+v", a)
	}
	if got := r.Find("ザ・クロマニヨンズ ツアー 2027"); len(got) != 1 {
		t.Fatalf("find: %v", got)
	}
}

func TestURL(t *testing.T) {
	if got := URL("https://x.jp/news/1/?utm_source=rss#top"); got != "https://x.jp/news/1" {
		t.Fatal(got)
	}
}

func TestSplitPrefectureCitiesAndRepeats(t *testing.T) {
	for in, want := range map[string]string{
		"仙台 宮城・セキスイハイムスーパーアリーナ":  "宮城|セキスイハイムスーパーアリーナ",
		"大阪 大阪・大阪城ホール":           "大阪|大阪城ホール",
		"その他 香港・AsiaWorld-Arena": "|香港・AsiaWorld-Arena",
		"広島 広島LIVE VANQUISH":     "広島|広島LIVE VANQUISH",
		"東京 東京ドーム":               "東京|東京ドーム",
	} {
		p, v := SplitPrefecture(in)
		if p+"|"+v != want {
			t.Errorf("SplitPrefecture(%q) = %q|%q, want %q", in, p, v, want)
		}
	}
}
