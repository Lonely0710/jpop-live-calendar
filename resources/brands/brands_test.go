package brands

import "testing"

func TestFor(t *testing.T) {
	cases := map[string]string{
		"https://www.sonymusic.co.jp/json/v2/artist/YOASOBI/live/start/0/count/50": "Sony Music",
		"https://open.spotify.com/artist/xyz":                                      "Spotify",
		"https://music.apple.com/jp/artist/1":                                      "Apple Music",
		"https://y.qq.com/n/ryqq/singer/1":                                         "QQ Music",
		"https://yorushika.com/news/5/":                                            "",
	}
	for u, want := range cases {
		b, ok := For(u)
		if want == "" {
			if ok {
				t.Errorf("%s: unexpected %s", u, b.Name)
			}
			continue
		}
		if !ok || b.Name != want {
			t.Errorf("%s: got %q ok=%v, want %q", u, b.Name, ok, want)
		}
	}
}
