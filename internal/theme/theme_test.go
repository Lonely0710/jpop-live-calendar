package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestArtistPaletteContrast(t *testing.T) {
	for _, h := range []string{"#CDAE2D", "#C6CED3", "#050C2D", "#F4E7D7", "#030303", "#3975AB", "#FFFF00"} {
		p := Artist(ui.Hex(h))
		if got := Contrast(p.Text, p.Tint); got < 4.5 {
			t.Errorf("%s: name on chip contrast %.2f < 4.5", h, got)
		}
		if got := Contrast(p.OnBase, p.Base); got < 3 {
			t.Errorf("%s: initial on avatar contrast %.2f < 3", h, got)
		}
	}
}
