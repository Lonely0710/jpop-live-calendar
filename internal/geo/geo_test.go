package geo

import (
	"testing"

	"jpop-live-calendar/internal/normalize"
)

func TestJapan(t *testing.T) {
	m := Japan()
	if len(m.Prefectures) != 47 {
		t.Fatalf("%d prefectures", len(m.Prefectures))
	}
	if m.Size.X < 1000 || m.Size.Y < 1000 {
		t.Fatalf("size %v", m.Size)
	}
	for i, p := range m.Prefectures {
		if p.Name != normalize.Prefectures()[i] {
			t.Errorf("prefecture %d is %q", i, p.Name)
		}
		// Each capital lies in its own prefecture: the outlines and the
		// markers agree, the inset included.
		if got := m.Hit(p.Capital); got != i {
			name := "the sea"
			if got >= 0 {
				name = m.Prefectures[got].Name
			}
			t.Errorf("%s's capital %v hits %s", p.Name, p.Capital, name)
		}
	}
	if ok := m.Find("沖縄"); ok < 0 || !m.Inset.Contains(m.Prefectures[ok].Capital) {
		t.Error("沖縄 should be in the inset")
	}
	if m.Inset.Contains(m.Prefectures[m.Find("東京")].Capital) {
		t.Error("東京 should not be in the inset")
	}
	if m.Hit(Point{1, 1}) != -1 {
		t.Error("the corner is sea")
	}
}

func TestSuffixed(t *testing.T) {
	for in, want := range map[string]string{"東京": "東京都", "北海道": "北海道", "大阪": "大阪府", "京都": "京都府", "沖縄": "沖縄県"} {
		if got := Suffixed(in); got != want {
			t.Errorf("Suffixed(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodeRejectsCorrupt(t *testing.T) {
	if _, err := decode([]byte("nope")); err == nil {
		t.Error("garbage decoded")
	}
	if _, err := decode(japanBin[:len(japanBin)/2]); err == nil {
		t.Error("a truncated map decoded")
	}
}
