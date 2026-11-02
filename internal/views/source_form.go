package views

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	icons "jpop-live-calendar/packages/spotlight-icons"

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
	// seg holds the indexes the segmented controls of kind, layer and
	// interval bind, kept here so their pointers stay put between frames.
	seg [3]int
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

// sourceFormView is the folded-out form of the 数据源 page: a label column
// on the left, and on the right either a segmented choice or a text field
// sized to what it holds, so choosing and typing read apart.
func (a *App) sourceFormView(c *ui.Context) {
	f := &a.sourceForm
	card(c).Padding(16, 20).Gap(0).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Start).Padding(0, 0, 14, 0).Children(func() {
			ui.Column(c).Gap(3).Grow(1).Shrink(1).Children(func() {
				ui.Text(c, locales.SourceFormTitle()).FontSize(15).Bold()
				codeText(c, locales.SourceFormNote(), theme.CaptionSize).TextColor(theme.TextSecondary)
			})
			if iconButton(c, icons.Clear, locales.CommonCancel(), false, false).Clicked() {
				f.open = false
			}
		})
		formDivider(c)
		formSection(c, func() {
			formLine(c, locales.FieldKind(), "", func() {
				segChoice(c, &f.kind, &f.seg[0], sourceKindIDs, sourceKindLabel, locales.FieldKind())
			})
			formLine(c, locales.FieldUrl(), "", func() {
				ui.Column(c).Gap(5).Grow(1).Shrink(1).Children(func() {
					ui.TextInput(c, &f.url).Width(460).MaxWidthPercent(100).Font("monospace").
						Label(locales.FieldUrl()).Placeholder("https://")
					help := locales.SourceFormHelpWebsite()
					if f.kind == string(domain.SourceRSS) {
						help = locales.SourceFormHelpRss()
					}
					caption(c, help)
				})
			})
			formLine(c, locales.FieldName(), "", func() {
				ui.TextInput(c, &f.name).Width(300).MaxWidthPercent(100).Label(locales.FieldName()).
					Placeholder(locales.FieldNamePlaceholder())
			})
		})
		formDivider(c)
		formSection(c, func() {
			formLine(c, locales.FieldLayer(), "", func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Shrink(1).Children(func() {
					segChoice(c, &f.layer, &f.seg[1], sourceLayerIDs, sourceLayerLabel, locales.FieldLayer())
					hint := locales.SourceLayerLiveHint()
					if f.layer == domain.LayerNews {
						hint = locales.SourceLayerNewsHint()
					}
					caption(c, hint).Shrink(1)
				})
			})
			formLine(c, locales.FieldInterval(), "", func() {
				segChoice(c, &f.interval, &f.seg[2], sourceIntervalIDs, sourceIntervalLabel, locales.FieldInterval())
			})
		})
		formDivider(c)
		formSection(c, func() {
			formLine(c, locales.FieldArtist(), locales.FieldOptional(), func() {
				ui.TextInput(c, &f.artist).Width(240).MaxWidthPercent(100).Label(locales.FieldArtistOptional()).
					Placeholder(locales.FieldArtistPlaceholder())
			})
			formLine(c, locales.FieldNote(), locales.FieldOptional(), func() {
				ui.TextInput(c, &f.note).Width(460).MaxWidthPercent(100).Label(locales.FieldNoteOptional()).
					Placeholder(locales.FieldNotePlaceholder())
			})
		})
		formDivider(c)
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(14, 0, 0, 0).Children(func() {
			ui.Column(c).Gap(3).Grow(1).Shrink(1).Children(func() {
				if f.err != "" {
					ui.Text(c, f.err).FontSize(12.5).Bold().TextColor(theme.StatusError)
				}
				caption(c, locales.SourceFormAfter())
			})
			if ui.Button(c, locales.CommonCancel()).Clicked() {
				f.open = false
			}
			if primaryButton(c, locales.SourceFormAdd()).Clicked() {
				a.addSource(c)
			}
		})
	})
}

// formLabelW is the width of the form's label column.
const formLabelW = 104

// formSection is a block of form lines between two dividers.
func formSection(c *ui.Context, lines func()) {
	ui.Column(c).Gap(12).Padding(14, 0).Children(lines)
}

// formDivider is the hairline between the blocks of a form.
func formDivider(c *ui.Context) {
	ui.Box(c).Height(1).FillWidth().Background(theme.MistGray.Alpha(0.8))
}

// formLine is one labelled control: the label, and a note such as 可选 under
// it, in the label column; the control on the right.
func formLine(c *ui.Context, label, note string, control func()) {
	ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Width(formLabelW).Shrink(0).Gap(1).Padding(6, 0, 0, 0).Children(func() {
			ui.Text(c, label).FontSize(13).Bold()
			if note != "" {
				caption(c, note)
			}
		})
		ui.Row(c).Grow(1).Shrink(1).AlignItems(ui.Center).MinHeight(30).Children(control)
	})
}

// segChoice is a segmented control over stable IDs: idx is the index it
// binds, kept beside *sel so its pointer is stable.
func segChoice(c *ui.Context, sel *string, idx *int, ids []string, label func(string) string, aria string) {
	*idx = 0
	labels := make([]string, len(ids))
	for i, id := range ids {
		labels[i] = label(id)
		if id == *sel {
			*idx = i
		}
	}
	seg := ui.SegmentedBase(c, idx, len(ids))
	seg.Track.Height(30).Padding(2).Gap(2).Radius(theme.Radius).AlignItems(ui.Center).
		Background(theme.PaperIvory).Border(1, theme.MistGray).Shrink(0).Label(aria).Children(func() {
		for i, text := range labels {
			s := seg.Segment(i).Height(24).Padding(0, 12).Radius(theme.Radius - 2).AlignItems(ui.Center).Label(text)
			switch {
			case *idx == i:
				s.Background(theme.LiveCoral)
			case s.Hovered():
				s.Background(theme.Hover)
			}
			s.Children(func() {
				t := ui.Text(c, text).FontSize(12.5).SingleLine()
				if *idx == i {
					t.Bold().TextColor(theme.CloudWhite)
				}
			})
		}
	})
	*sel = ids[*idx]
}

// primaryButton is the coral button of a form's main action.
func primaryButton(c *ui.Context, text string) ui.Element {
	b := ui.ButtonBase(c).Height(30).Padding(0, 16).Radius(theme.Radius).Label(text).Background(theme.LiveCoral)
	switch {
	case b.Pressed():
		b.Background(theme.LiveCoral.Alpha(0.8).Over(theme.InkCharcoal))
	case b.Hovered():
		b.Background(theme.LiveCoral.Alpha(0.88).Over(theme.InkCharcoal))
	}
	return b.Children(func() {
		ui.Text(c, text).FontSize(13).Bold().TextColor(theme.CloudWhite).SingleLine()
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
