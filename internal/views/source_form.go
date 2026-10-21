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
)

// sourceDraft is the 数据源 page's "新增数据源" form. It lives in the app
// state so the fields keep their text while the view rebuilds.
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
	sourceKindOptions     = []string{"官网 website", "RSS"}
	sourceLayerOptions    = []string{"日程 live", "公告 news"}
	sourceIntervalOptions = []string{"6h", "12h", "24h", "0（仅手动）"}
)

func newSourceDraft() sourceDraft {
	return sourceDraft{kind: sourceKindOptions[0], layer: sourceLayerOptions[0], interval: "12h"}
}

func parseSourceKind(s string) domain.SourceKind {
	if strings.HasPrefix(s, "RSS") {
		return domain.SourceRSS
	}
	return domain.SourceOfficialSite
}

func parseSourceLayer(s string) string {
	if strings.HasPrefix(s, "公告") {
		return domain.LayerNews
	}
	return domain.LayerLive
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
			ui.Text(c, "新增数据源").Bold()
			caption(c, "写入 sources.yaml；只添加公开、允许访问的官方或 RSS 来源")
			ui.Spacer(c)
			if ui.Button(c, "取消").Clicked() {
				f.open = false
			}
		})
		fieldRow(c, "名称", func() {
			ui.TextInput(c, &f.name).Grow(1).Shrink(1).Label("名称").Placeholder("例：某乐队官网 LIVE")
		})
		ui.Row(c).Gap(14).AlignItems(ui.End).Children(func() {
			fieldRow(c, "类型", func() { ui.Select(c, &f.kind, sourceKindOptions).Width(150).Label("类型") })
			fieldRow(c, "层级", func() { ui.Select(c, &f.layer, sourceLayerOptions).Width(150).Label("层级") })
			fieldRow(c, "间隔", func() { ui.Select(c, &f.interval, sourceIntervalOptions).Width(140).Label("间隔") })
		})
		fieldRow(c, "地址", func() {
			ui.TextInput(c, &f.url).Grow(1).Shrink(1).Label("地址").Placeholder("https://…")
		})
		ui.Row(c).Gap(14).AlignItems(ui.End).Children(func() {
			ui.Column(c).Shrink(1).Children(func() {
				fieldRow(c, "艺人（可选）", func() {
					ui.TextInput(c, &f.artist).Width(210).Label("艺人").Placeholder("sources.yaml 中的艺人名")
				})
			})
			ui.Column(c).Grow(1).Children(func() {
				fieldRow(c, "备注（可选）", func() {
					ui.TextInput(c, &f.note).Grow(1).Label("备注").Placeholder("显示在数据源卡片上")
				})
			})
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			caption(c, "官网地址请填 LIVE / SCHEDULE / NEWS 列表页；RSS 填 feed 地址。机器翻译或需要登录的页面不要添加。")
		})
		if f.err != "" {
			ui.Text(c, f.err).FontSize(12.5).TextColor(theme.StatusError)
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "添加数据源").Clicked() {
				a.addSource(c)
			}
			caption(c, "添加后立即同步一次；可在卡片上停用或删除。")
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
		return domain.SourceConfig{}, errors.New("请填写名称")
	case !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://"):
		return domain.SourceConfig{}, errors.New("地址必须以 https:// 开头（只读取公开页面）")
	}
	return domain.SourceConfig{
		ID:       a.sourceID(rawURL),
		Name:     name,
		Kind:     parseSourceKind(f.kind),
		URL:      rawURL,
		Enabled:  true,
		Interval: parseSourceInterval(f.interval),
		Artist:   strings.TrimSpace(f.artist),
		Note:     strings.TrimSpace(f.note),
		Layer:    parseSourceLayer(f.layer),
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
		c.Toast("已添加「" + name + "」（仅本次运行有效）")
		return
	}
	file, err := a.opt.AddSource(src)
	if err != nil {
		c.Toast("已添加「" + name + "」，但写入配置失败：" + err.Error())
		return
	}
	a.opt.Config.Path = file
	c.Toast("已添加「" + name + "」并同步")
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
