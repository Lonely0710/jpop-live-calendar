package sources

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/locales"
)

// Website reads an official site.
//
// Without Follow, the page itself (a news or live page) is one
// announcement. With Follow, the page is a list: every link whose address
// matches the Follow pattern (and whose text has one of the keywords, when
// given) is opened, newest first as the list orders them, up to
// MaxPerSync, and each page is one announcement.
//
// With Artist set the site is that artist's own: every announcement is
// theirs, and it is read as a schedule, where every dated line is a show.
type Website struct{}

func (Website) Fetch(ctx context.Context, src domain.SourceConfig, env Env) (Result, error) {
	var res Result
	resp, err := env.Get.Get(ctx, src.URL)
	if err != nil {
		return res, err
	}
	if resp.NotModified {
		res.NotModified = true
		return res, nil
	}
	now := env.now()
	page := func(u, body, fallback string) domain.PublicAnnouncement {
		text, hints, _ := article(body, src.Article)
		raw := htmlTitle(body)
		title := pageTitle(raw, src.Artist)
		if title == "" {
			title = linkTitle(fallback)
		}
		if title == "" {
			title = strings.TrimSpace(raw)
		}
		when := now
		if d, ok := urlDate(u); ok {
			when = d
		}
		if src.Artist != "" {
			hints = []string{src.Artist}
		}
		return domain.PublicAnnouncement{
			SourceID: src.ID, URL: u, Title: title, RawText: text, ArtistHints: hints,
			FetchedAt: now, PublishedAt: when, Schedule: src.Artist != "",
		}
	}
	base := resp.URL
	if base == "" {
		base = src.URL
	}
	// The pages read, for the tour pages they link to.
	bodies := []struct{ url, body string }{{base, resp.Text}}
	if src.Follow == "" {
		if _, ok := between(resp.Text, src.Article.BodyStart, src.Article.BodyEnd); !ok && src.Article.BodyStart != "" {
			res.Warnings = append(res.Warnings, locales.SiteNoMarker())
		}
		res.Announcements = []domain.PublicAnnouncement{page(src.URL, resp.Text, src.Name)}
		err := features(ctx, src, env, bodies, map[string]bool{src.URL: true}, &res, page)
		return res, err
	}
	pattern, err := regexp.Compile(src.Follow)
	if err != nil {
		return res, fmt.Errorf("%s", locales.SiteBadFollow(err.Error()))
	}
	links := Links(resp.Text, base)
	max := src.Article.MaxPerSync
	if max <= 0 {
		max = 5
	}
	opened := 0
	seen := map[string]bool{}
	for _, l := range links {
		if seen[l.URL] || !pattern.MatchString(l.URL) || !wanted(l.Text, src.Article) {
			continue
		}
		seen[l.URL] = true
		if opened == max {
			break // the list's older pages: past shows, read when they were new
		}
		if opened > 0 {
			if err := sleep(ctx, src.Article.Delay); err != nil {
				return res, err
			}
		}
		opened++
		detail, err := env.Get.Get(ctx, l.URL)
		switch {
		case err != nil:
			res.Warnings = append(res.Warnings, locales.SiteItemFailed(l.Text, err.Error()))
		case detail.NotModified:
			res.Announcements = append(res.Announcements, domain.PublicAnnouncement{
				SourceID: src.ID, URL: l.URL, Title: l.Text, FetchedAt: now, PublishedAt: now,
				MetadataOnly: true, Schedule: src.Artist != "",
			})
		default:
			res.Announcements = append(res.Announcements, page(l.URL, detail.Text, l.Text))
			bodies = append(bodies, struct{ url, body string }{l.URL, detail.Text})
		}
	}
	if opened == 0 && src.Feature == "" {
		res.Warnings = append(res.Warnings, locales.SiteNoLinks())
	}
	err = features(ctx, src, env, bodies, seen, &res, page)
	return res, err
}

// maxFeatures is how many tour pages one sync opens.
const maxFeatures = 3

// features opens the tour pages the pages read link to ("…/feature/
// asiatour2026"), matching the source's Feature pattern: a tour's page is
// found from its announcement, the way a reader finds it, and never
// configured by hand. Each is one announcement; pages already opened are
// skipped.
func features(ctx context.Context, src domain.SourceConfig, env Env, bodies []struct{ url, body string }, seen map[string]bool,
	res *Result, page func(u, body, fallback string) domain.PublicAnnouncement) error {
	if src.Feature == "" {
		return nil
	}
	pattern, err := regexp.Compile(src.Feature)
	if err != nil {
		return fmt.Errorf("%s", locales.SiteBadFeature(err.Error()))
	}
	opened := 0
	for _, b := range bodies {
		for _, l := range Links(b.body, b.url) {
			u := strings.TrimSuffix(l.URL, "/")
			if seen[l.URL] || seen[u] || !pattern.MatchString(l.URL) {
				continue
			}
			seen[l.URL], seen[u] = true, true
			if opened == maxFeatures {
				return nil
			}
			if err := sleep(ctx, src.Article.Delay); err != nil {
				return err
			}
			opened++
			detail, err := env.Get.Get(ctx, l.URL)
			switch {
			case err != nil:
				res.Warnings = append(res.Warnings, locales.SiteFeatureFailed(l.URL, err.Error()))
			case detail.NotModified:
				res.Announcements = append(res.Announcements, domain.PublicAnnouncement{
					SourceID: src.ID, URL: l.URL, Title: l.Text, FetchedAt: env.now(), PublishedAt: env.now(),
					MetadataOnly: true, Schedule: src.Artist != "",
				})
			default:
				res.Announcements = append(res.Announcements, page(l.URL, detail.Text, l.Text))
			}
		}
	}
	return nil
}

// Link is an <a> of a page: its absolute address and its text.
type Link struct{ URL, Text string }

var anchorRe = regexp.MustCompile(`(?is)<a\b[^>]*?\bhref\s*=\s*["']([^"'#]+)[^"']*["'][^>]*>(.*?)</a>`)

// Links returns the links of a page in order, resolved against base.
func Links(page, base string) []Link {
	b, err := url.Parse(base)
	if err != nil {
		return nil
	}
	var out []Link
	for _, m := range anchorRe.FindAllStringSubmatch(page, -1) {
		ref, err := url.Parse(strings.TrimSpace(html.UnescapeString(m[1])))
		if err != nil {
			continue
		}
		u := b.ResolveReference(ref)
		if u.Scheme != "https" && u.Scheme != "http" {
			continue
		}
		text := strings.Join(strings.Fields(HTMLText(m[2])), " ")
		out = append(out, Link{URL: u.String(), Text: text})
	}
	return out
}

var (
	urlDateRe   = regexp.MustCompile(`/(20\d\d)[/-](\d{1,2})[/-](\d{1,2})(?:[/?#]|$)`)
	titleSepRe  = regexp.MustCompile(`\s+[|｜\-–—]\s*|\s*[|｜]\s*`)
	siteWordsRe = regexp.MustCompile(`(?i)^official\b|\bofficial$|オフィシャル$|公式$|official\s*(?:web\s*)?(?:site|home ?page)|official\s*fan ?club(?:\s*「.*」)?$|オフィシャル(?:サイト|ファンクラブ)|公式(?:サイト|ホームページ)|web ?site|^(news|topics|live|schedule|tour|info)$|universal music|sony music`)
)

// urlDate reads the date in a post's address, as /2026/10/15/: the day it
// was posted, which dates its shows written without a year.
func urlDate(u string) (time.Time, bool) {
	m := urlDateRe.FindStringSubmatch(u)
	if m == nil {
		return time.Time{}, false
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	if mo < 1 || mo > 12 || d < 1 || d > 31 {
		return time.Time{}, false
	}
	return time.Date(y, time.Month(mo), d, 12, 0, 0, 0, domain.Tokyo), true
}

// pageTitle drops the site's name from a page title: "米津玄師 2026 TOUR /
// GHOST | 米津玄師 official site" is the tour's name.
func pageTitle(t, artist string) string {
	parts := titleSepRe.Split(t, -1)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || siteWordsRe.MatchString(p) || strings.EqualFold(p, artist) {
			continue
		}
		return p
	}
	return "" // only the site's name: the link's text is better
}

var (
	linkDateHeadRe = regexp.MustCompile(`^\d{1,4}[./]\d{1,2}(?:[./]\d{1,2})?\s*(?:\([^)]*\))?\s+`)
	linkDateTailRe = regexp.MustCompile(`\s+\d{4}[./]\d{1,2}[./]\d{1,2}(?:\s+[A-Za-z/]+)?$`)
)

// linkTitle is a list link's text without the dates and the category
// around it: "10.14 『ユイカ』TOUR「Anemone」…スタート! 2026.10.14 LIVE/EVENT".
func linkTitle(t string) string {
	t = strings.TrimSpace(t)
	t = linkDateTailRe.ReplaceAllString(t, "")
	t = linkDateHeadRe.ReplaceAllString(t, "")
	return strings.TrimSpace(t)
}
