package views

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/domain"
)

// readCatalog reads one of the app's message catalogs: the texts the UI is
// written in, so a page showing another language's wording stands out.
func readCatalog(t *testing.T, tag string) map[string]string {
	t.Helper()
	b, err := os.ReadFile("../../locales/" + tag + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// renderedTexts draws every page, popover and form in tag and returns the
// texts it shows. The test sets the language the way mygo.App.SetLocale
// does, which the views read through i18n.Locale.
func renderedTexts(t *testing.T, tag string) []string {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return tag })
	defer i18n.SetLocaleFunc(nil)
	now := time.Date(2026, 10, 21, 21, 29, 0, 0, domain.Tokyo)
	app := New(Options{Config: config.Default(), Catalog: demoCatalog(t, now), Now: func() time.Time { return now }})
	tt := ui.NewTester(app.View, 1440, 900)
	tt.SetLocale(tag)
	app.InitLocale(tag)
	seen := map[string]bool{}
	collect := func() {
		tt.Frame()
		for _, s := range tt.Texts() {
			for _, line := range strings.Split(s, "\n") {
				if line = strings.TrimSpace(line); line != "" {
					seen[line] = true
				}
			}
		}
		tt.Frame()
	}
	collect()
	for _, s := range []Section{SectionCalendar, SectionMap, SectionArtists, SectionSources, SectionSettings} {
		app.section = s
		collect()
	}
	app.langOpen = true
	app.userOpen = true
	collect()
	app.langOpen = false
	app.userOpen = false
	app.section = SectionSources
	app.sourceForm.open = true
	collect()
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestNoTextLeftUntranslated checks that the interface is written in the
// language it shows: in ja and en no page, tooltip label or form may
// still show a phrase from the zh-Hans catalog. Announcement text (artist,
// tour and venue names, which stay Japanese) is not a catalog phrase, so
// it passes.
func TestNoTextLeftUntranslated(t *testing.T) {
	zh := readCatalog(t, "zh-Hans")
	for _, tag := range []string{"ja", "en"} {
		other := readCatalog(t, tag)
		// The zh wording of every message this language says otherwise.
		// Phrases under three characters are place names, which the data
		// holds in Japanese anyway.
		var untranslated []string
		for k, v := range zh {
			if strings.HasPrefix(k, "@") || other[k] == "" || other[k] == v {
				continue
			}
			if strings.ContainsAny(v, "{}") || len([]rune(v)) < 3 {
				continue
			}
			untranslated = append(untranslated, v)
		}
		sort.Slice(untranslated, func(i, j int) bool { return len(untranslated[i]) > len(untranslated[j]) })
		for _, line := range renderedTexts(t, tag) {
			// A language names itself in its own language in the list.
			if tag != "zh-Hans" && strings.HasPrefix(line, "日本語") {
				continue
			}
			for _, phrase := range untranslated {
				if strings.Contains(line, phrase) {
					t.Errorf("%s shows %q, which is zh-Hans for %q", tag, line, phrase)
					break
				}
			}
		}
	}
}
