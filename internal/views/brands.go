package views

import (
	"sync"

	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/packages/spotlight-icons/mygoicons"
	"jpop-live-calendar/resources/brands"
)

// The marks of the services a source can come from, parsed once: a page
// that lists sources shows Sony Music's mark for its artist API, instead
// of saying "Sony Music" in the source's name.
var (
	brandMu   sync.Mutex
	brandSVGs = map[string]*ui.SVG{}
)

// brandMark is the mark of the service a source's address belongs to; ok
// is false for an artist's own site.
func brandMark(c *ui.Context, rawURL string, size float32) (ui.Element, bool) {
	b, ok := brands.For(rawURL)
	if !ok {
		return ui.Element{}, false
	}
	brandMu.Lock()
	svg := brandSVGs[b.Name]
	if svg == nil {
		svg = ui.MustParseSVG(b.SVG)
		brandSVGs[b.Name] = svg
	}
	brandMu.Unlock()
	return ui.Box(c).Size(size, size).Shrink(0).Role(ui.RoleImage).Label(b.Name).Tooltip(b.Name).Children(func() {
		ui.Image(c, svg).Size(size, size)
	}), true
}

// sourceMark is a source's icon: the service's mark when it has one, the
// kind's icon otherwise. The kind stays in the tooltip.
func sourceMark(c *ui.Context, src domain.SourceConfig, size float32) ui.Element {
	if mark, ok := brandMark(c, src.URL, size); ok {
		return mark // the service, named in words only in the tooltip
	}
	return mygoicons.Image(c, sourceKindIcon(src.Kind), size).Tooltip(kindLabel(src.Kind))
}
