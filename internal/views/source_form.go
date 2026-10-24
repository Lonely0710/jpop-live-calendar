package views

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/locales"
)

// sourceDraft is the 数据源 page's "新增数据源" form. It lives in the app
// state so the fields keep their text while the view rebuilds. Its choices
// are stable IDs, which the labels of the current language name.
type sourceDraft struct {
	open     bool
	name     string
	kind     string
	layer    string
	interval string
	url      string
	artist   string
	note     string
	err      string
}

var (
	sourceKindIDs     = []string{string(domain.SourceOfficialSite), string(domain.SourceRSS)}
	sourceLayerIDs    = []string{domain.LayerLive, domain.LayerNews}
	sourceIntervalIDs = []string{"6h", "12h", "24h", "0"}
)

func newSourceDraft() sourceDraft {
	return sourceDraft{kind: sourceKindIDs[0], layer: sourceLayerIDs[0], interval: "12h"}
}

func sourceKindLabel(id string) string {
	if id == string(domain.SourceRSS) {
		return locales.SourceKindRssOption()
	}
	return locales.SourceKindWebsiteOption()
}

func sourceLayerLabel(id string) string {
	if id == domain.LayerNews {
		return locales.SourceLayerNewsOption()
	}
	return locales.SourceLayerLiveOption()
}

func sourceIntervalLabel(id string) string {
	if id == "0" {
		return locales.SourcesIntervalManual()
	}
	return id
}

func parseSourceInterval(s string) time.Duration {
	switch s {
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h":
		return 24 * time.Hour
	}
	return 0
}

// sourceFormView is the folded-out form of the 数据源 page.
func (a *App) sourceFormView(c *ui.Context) {
	f := &a.sourceForm
	card(c).Padding(14, 16).Gap(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.SourceFormTitle()).Bold()
			caption(c, locales.SourceFormNote())
			ui.Spacer(c)
			if ui.Button(c, locales.CommonCancel()).Clicked() {
				f.open = false
			}
		})
		fieldRow(c, locales.FieldName(), func() {
			ui.TextInput(c, &f.name).Grow(1).Shrink(1).Label(locales.FieldName()).Placeholder(locales.FieldNamePlaceholder())
		})
		ui.Row(c).Gap(14).AlignItems(ui.End).Children(func() {
			fieldRow(c, locales.FieldKind(), func() {
				placeSelect(c, &f.kind, sourceKindIDs, sourceKindLabel, locales.FieldKind(), 150)
			})
			fieldRow(c, locales.FieldLayer(), func() {
				placeSelect(c, &f.layer, sourceLayerIDs, sourceLayerLabel, locales.FieldLayer(), 150)
			})
			fieldRow(c, locales.FieldInterval(), func() {
				placeSelect(c, &f.interval, sourceIntervalIDs, sourceIntervalLabel, locales.FieldInterval(), 140)
			})
		})
		fieldRow(c, locales.FieldUrl(), func() {
			ui.TextInput(c, &f.url).Grow(1).Shrink(1).Label(locales.FieldUrl()).Placeholder("https://…")
		})
		ui.Row(c).Gap(14).AlignItems(ui.End).Children(func() {
			ui.Column(c).Shrink(1).Children(func() {
				fieldRow(c, locales.FieldArtistOptional(), func() {
					ui.TextInput(c, &f.artist).Width(210).Label(locales.FieldArtistOptional()).Placeholder(locales.FieldArtistPlaceholder())
				})
			})
			ui.Column(c).Grow(1).Children(func() {
				fieldRow(c, locales.FieldNoteOptional(), func() {
					ui.TextInput(c, &f.note).Grow(1).Label(locales.FieldNoteOptional()).Placeholder(locales.FieldNotePlaceholder())
				})
			})
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			caption(c, locales.SourceFormHelp())
		})
		if f.err != "" {
			ui.Text(c, f.err).FontSize(12.5).TextColor(theme.StatusError)
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, locales.SourceFormAdd()).Clicked() {
				a.addSource(c)
			}
			caption(c, locales.SourceFormAfter())
		})
	})
}

// fieldRow is a labelled form control.
func fieldRow(c *ui.Context, label string, body func()) {
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(theme.TextSecondary)
		body()
	})
}

// draftSource validates the form and builds the source it names.
func (a *App) draftSource() (domain.SourceConfig, error) {
	f := &a.sourceForm
	name := strings.TrimSpace(f.name)
	rawURL := strings.TrimSpace(f.url)
	switch {
	case name == "":
		return domain.SourceConfig{}, errors.New(locales.SourceErrName())
	case !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://"):
		return domain.SourceConfig{}, errors.New(locales.SourceErrUrl())
	}
	return domain.SourceConfig{
		ID:       a.sourceID(rawURL),
		Name:     name,
		Kind:     domain.SourceKind(f.kind),
		URL:      rawURL,
		Enabled:  true,
		Interval: parseSourceInterval(f.interval),
		Artist:   strings.TrimSpace(f.artist),
		Note:     strings.TrimSpace(f.note),
		Layer:    f.layer,
	}, nil
}

// addSource validates the form, appends the source to the configuration
// and the scheduler, and syncs it once.
func (a *App) addSource(c *ui.Context) {
	f := &a.sourceForm
	src, err := a.draftSource()
	if err != nil {
		f.err = err.Error()
		return
	}
	name := src.Name
	// Add in memory first: the source works for this run even if writing
	// the configuration fails, as removing one does.
	a.opt.Config.Sources = append(a.opt.Config.Sources, src)
	a.opt.Scheduler.Add(src)
	f.open = false
	f.err = ""
	*f = newSourceDraft()
	a.Refresh(src.ID)
	if a.opt.AddSource == nil {
		c.Toast(locales.SourceAddedVolatile(name))
		return
	}
	file, err := a.opt.AddSource(src)
	if err != nil {
		c.Toast(locales.SourceAddedError(name, err.Error()))
		return
	}
	a.opt.Config.Path = file
	c.Toast(locales.SourceAdded(name))
}

// sourceID makes a unique id from the URL's host and last path segment,
// as sources.yaml writes them: example-com-live, example-com-live-2.
func (a *App) sourceID(rawURL string) string {
	base := "source"
	if u, err := url.Parse(rawURL); err == nil {
		host := sourceSlug(strings.TrimPrefix(strings.ToLower(u.Host), "www."))
		last := sourceSlug(path.Base(strings.TrimRight(u.Path, "/")))
		switch {
		case host != "" && last != "" && last != ".":
			base = host + "-" + last
		case host != "":
			base = host
		}
	}
	taken := map[string]bool{}
	for _, s := range a.opt.Config.Sources {
		taken[s.ID] = true
	}
	if a.opt.Scheduler != nil {
		for _, st := range a.opt.Scheduler.Statuses() {
			taken[st.Config.ID] = true
		}
	}
	id := base
	for i := 2; taken[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	return id
}

// sourceSlug keeps the ASCII letters and digits of s, joining words with
// one dash; Japanese names leave nothing, which sourceID fills in.
func sourceSlug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
