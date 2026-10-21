package sources

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/normalize"
)

// RSS reads an RSS 2.0, RSS 1.0 (RDF) or Atom feed. Each item becomes an
// announcement. When the source's Article options are enabled, the pages
// of items whose title has one of the keywords are opened (at most
// MaxPerSync per sync, skipping pages already stored) and their body is
// used as the announcement text; other items keep only their title.
type RSS struct{}

type feedItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        string   `xml:"guid"`
	PubDate     string   `xml:"pubDate"`
	DCDate      string   `xml:"http://purl.org/dc/elements/1.1/ date"`
	Description string   `xml:"description"`
	Content     string   `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	Categories  []string `xml:"category"`
	About       string   `xml:"http://www.w3.org/1999/02/22-rdf-syntax-ns# about,attr"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	ID        string     `xml:"id"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Category  []struct {
		Term string `xml:"term,attr"`
	} `xml:"category"`
}

type feedDoc struct {
	XMLName xml.Name
	Channel struct {
		Items []feedItem `xml:"item"`
	} `xml:"channel"`
	Items   []feedItem  `xml:"item"`  // RSS 1.0
	Entries []atomEntry `xml:"entry"` // Atom
}

// FeedItem is one item of a feed.
type FeedItem struct {
	Title      string
	Link       string
	Published  time.Time
	Summary    string // plain text
	Categories []string
}

// ParseFeed reads RSS 2.0, RSS 1.0 and Atom.
func ParseFeed(text string) ([]FeedItem, error) {
	dec := xml.NewDecoder(strings.NewReader(text))
	dec.Strict = false
	// The fetcher has already decoded the body to UTF-8.
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var doc feedDoc
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("feed is not valid XML: %w", err)
	}
	var out []FeedItem
	for _, it := range append(doc.Channel.Items, doc.Items...) {
		link := strings.TrimSpace(it.Link)
		if link == "" {
			link = strings.TrimSpace(it.GUID)
		}
		if link == "" {
			link = it.About
		}
		body := it.Content
		if body == "" {
			body = it.Description
		}
		date := it.PubDate
		if date == "" {
			date = it.DCDate
		}
		out = append(out, FeedItem{
			Title: strings.TrimSpace(it.Title), Link: link, Published: parseDate(date),
			Summary: HTMLText(body), Categories: trimAll(it.Categories),
		})
	}
	for _, e := range doc.Entries {
		var link string
		for _, l := range e.Links {
			if l.Rel == "" || l.Rel == "alternate" {
				link = l.Href
				break
			}
		}
		date := e.Published
		if date == "" {
			date = e.Updated
		}
		body := e.Content
		if body == "" {
			body = e.Summary
		}
		var cats []string
		for _, c := range e.Category {
			cats = append(cats, c.Term)
		}
		out = append(out, FeedItem{
			Title: strings.TrimSpace(e.Title), Link: strings.TrimSpace(link), Published: parseDate(date),
			Summary: HTMLText(body), Categories: trimAll(cats),
		})
	}
	if doc.XMLName.Local != "rss" && doc.XMLName.Local != "RDF" && doc.XMLName.Local != "feed" {
		return nil, fmt.Errorf("not a feed (root element <%s>)", doc.XMLName.Local)
	}
	return out, nil
}

func trimAll(xs []string) []string {
	var out []string
	for _, x := range xs {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

var dateLayouts = []string{
	time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC3339Nano,
	"Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST", "2 Jan 2006 15:04:05 -0700",
	"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02",
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, l := range dateLayouts {
		if t, err := time.ParseInLocation(l, s, domain.Tokyo); err == nil {
			return t
		}
	}
	return time.Time{}
}

var parenRe = regexp.MustCompile(`\s*[(（][^)）]*[)）]\s*$`)

func (RSS) Fetch(ctx context.Context, src domain.SourceConfig, env Env) (Result, error) {
	var res Result
	resp, err := env.Get.Get(ctx, src.URL)
	if err != nil {
		return res, err
	}
	if resp.NotModified {
		res.NotModified = true
		return res, nil
	}
	items, err := ParseFeed(resp.Text)
	if err != nil {
		return res, err
	}
	now := env.now()
	opt := src.Article
	var include *regexp.Regexp
	if opt.Include != "" {
		// The filter is config-driven, so it is compiled once per sync.
		include, err = regexp.Compile("(?i)" + opt.Include)
		if err != nil {
			return res, fmt.Errorf("article.include 不是有效的正则表达式：%w", err)
		}
	}
	opened := 0
	for _, it := range items {
		if it.Link == "" || it.Title == "" {
			continue
		}
		if include != nil && !include.MatchString(normalize.Text(it.Title)) {
			continue
		}
		if opt.MaxAge > 0 && !it.Published.IsZero() && now.Sub(it.Published) > opt.MaxAge {
			continue
		}
		raw := it.Summary
		if opt.TitleOnly {
			raw = ""
		}
		a := domain.PublicAnnouncement{
			SourceID: src.ID, URL: it.Link, Title: it.Title, PublishedAt: it.Published, FetchedAt: now,
			RawText: raw, MetadataOnly: opt.Enabled || opt.TitleOnly,
		}
		for _, c := range it.Categories {
			a.ArtistHints = append(a.ArtistHints, parenRe.ReplaceAllString(c, ""))
		}
		if a.PublishedAt.IsZero() {
			res.Warnings = append(res.Warnings, fmt.Sprintf("「%s」没有发布时间，按抓取时间处理", it.Title))
		}
		if opt.Enabled && wanted(it.Title, opt) && !env.known(it.Link) {
			if opt.MaxPerSync > 0 && opened >= opt.MaxPerSync {
				res.Warnings = append(res.Warnings, fmt.Sprintf("已达到每次同步最多打开 %d 篇的上限，其余下次同步", opt.MaxPerSync))
				opt.MaxPerSync = -1 // warn once
			} else if opt.MaxPerSync >= 0 {
				if opened > 0 {
					if err := sleep(ctx, opt.Delay); err != nil {
						return res, err
					}
				}
				opened++
				page, err := env.Get.Get(ctx, it.Link)
				if err != nil {
					res.Warnings = append(res.Warnings, fmt.Sprintf("「%s」正文抓取失败：%v", it.Title, err))
				} else if !page.NotModified {
					text, hints, ok := article(page.Text, opt)
					if !ok {
						res.Warnings = append(res.Warnings, fmt.Sprintf("「%s」没有找到正文标记，页面结构可能已变化", it.Title))
					}
					a.RawText, a.MetadataOnly = text, false
					if len(hints) > 0 {
						a.ArtistHints = hints
					}
				}
			}
		}
		res.Announcements = append(res.Announcements, a)
	}
	return res, nil
}

// wanted reports whether a title has one of the keywords and none of the
// excluded words.
func wanted(title string, opt domain.ArticleOptions) bool {
	return matchesKeywords(title, opt.Keywords) && (len(opt.Exclude) == 0 || !matchesKeywords(title, opt.Exclude))
}

func matchesKeywords(title string, kws []string) bool {
	if len(kws) == 0 {
		return true
	}
	t := strings.ToLower(normalize.Text(title))
	for _, k := range kws {
		k = strings.ToLower(normalize.Text(k))
		if p, ok := strings.CutPrefix(k, "^"); ok { // "^live": the text starts with it
			if strings.HasPrefix(t, p) {
				return true
			}
		} else if strings.Contains(t, k) {
			return true
		}
	}
	return false
}

// article extracts the body text and the artist list of an article page.
func article(page string, opt domain.ArticleOptions) (text string, artists []string, ok bool) {
	body, ok := between(page, opt.BodyStart, opt.BodyEnd)
	if !ok {
		body = page
	}
	text = HTMLText(body)
	if opt.ArtistStart != "" {
		if seg, found := between(page, opt.ArtistStart, opt.ArtistEnd); found {
			artists = names(HTMLText(seg))
		}
	}
	return text, artists, ok
}
