package mygoicons

import (
	"testing"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"
)

func TestAllIconsParse(t *testing.T) {
	for _, n := range icons.All {
		if _, err := ui.ParseSVG(icons.SVG(n)); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
}

func TestRenderGallery(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Grid(c).Columns(10).Gap(8).Padding(16).Children(func() {
			for _, n := range icons.All {
				Image(c, n, 24)
			}
		})
	}, 400, 300)
	tt.Frame()
}
