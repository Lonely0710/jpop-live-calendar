package views

import (
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/catalog"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/locales"
)

// TestCountLabelsStayOnOneLine checks the figure a row puts on its right
// keeps to a single line even with a name long enough to fill the row: a
// tour's "36 shows" must not break into "36 / shows", which is what the
// tour filter's popover did before countLabel stopped it shrinking.
func TestCountLabelsStayOnOneLine(t *testing.T) {
	now := time.Date(2026, 10, 21, 21, 29, 0, 0, domain.Tokyo)
	for _, tag := range []string{"zh-Hans", "ja", "en"} {
		i18n.SetLocaleFunc(func() string { return tag })
		app := New(Options{Config: config.Default(), Catalog: demoCatalog(t, now), Now: func() time.Time { return now }})
		s := catalog.ArtistSummary{Artist: domain.Artist{ID: "a-long", Name: "Neon Orchard"}}
		// The popover's real width, its rows, and a name long enough to
		// leave the count almost no room.
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Width(340).Padding(6).Gap(1).Children(func() {
				app.tourChoice(c, s, "", locales.ArtistAllTours(), 43)
				app.tourChoice(c, s, "k1", "ZUTOMAYO Hall Tour 2027 「アビが鳴く」", 36)
				app.tourChoice(c, s, "k2", "ROCK IN JAPAN FESTIVAL 2026", 1)
			})
		}, 400, 300)
		tt.SetLocale(tag)
		app.InitLocale(tag)
		tt.Frame()
		tt.Frame()

		height := func(shows int) (float32, bool) {
			r, ok := tt.Find(locales.ShowsCount(shows))
			return r.H, ok
		}
		// "1 show" always fits: its height is one caption line.
		line, ok := height(1)
		if !ok {
			t.Fatalf("%s: the count of the short row is not drawn", tag)
		}
		for _, shows := range []int{43, 36} {
			h, ok := height(shows)
			if !ok {
				t.Fatalf("%s: %q is not drawn", tag, locales.ShowsCount(shows))
			}
			if h > line*1.2 {
				t.Errorf("%s: %q is %.1f high, %.1f for one line: it wrapped",
					tag, locales.ShowsCount(shows), h, line)
			}
		}
	}
	i18n.SetLocaleFunc(nil)
}
