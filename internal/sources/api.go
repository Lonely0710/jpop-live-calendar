package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"

	"jpop-live-calendar/internal/domain"
)

// API reads the structured public data an official site loads in the
// browser: the same JSON its own Live page shows, so a site whose
// schedule is drawn by JavaScript can be read without a browser.
//
// One kind is known: Sony Music's artist feed
// (https://www.sonymusic.co.jp/json/v2/artist/<folder>/live/start/0/count/100),
// which the sites of the Sony Music artists (YOASOBI, King Gnu, Aimer,
// 優里, 緑黄色社会, natori…) all use. Others need their own terms and code.
type API struct{}

func (API) Fetch(ctx context.Context, src domain.SourceConfig, env Env) (Result, error) {
	u, err := url.Parse(src.URL)
	if err != nil {
		return Result{}, err
	}
	if strings.HasSuffix(u.Host, "sonymusic.co.jp") && strings.Contains(u.Path, "/json/v2/artist/") {
		if strings.Contains(u.Path, "/information/") {
			return sonyNews(ctx, src, env)
		}
		return sonyLive(ctx, src, env)
	}
	return Result{}, fmt.Errorf("不认识的数据接口 %s（目前支持 Sony Music 艺人 Live 接口）", u.Host)
}

// sonyItem is a tour, or a single show, of Sony Music's feed.
type sonyItem struct {
	TourName        string `json:"tourName"`
	TourInfo        string `json:"tourInfo"` // HTML: dates, venues, ticket rounds
	Link            string `json:"link"`
	RelatedLinkURL  string `json:"relatedLinkUrl"`
	RelatedLinkName string `json:"relatedLinkName"`
	LiveItem        []struct {
		Date  string `json:"date"` // 2026.10.27
		Youbi string `json:"youbi"`
		Area  string `json:"area"` // 大阪府
		Place string `json:"place"`
		Open  string `json:"open"`
		Start string `json:"start"`
	} `json:"liveItem"`
}

// sonyLive turns each tour of the feed into an announcement written as a
// schedule: one line per show ("2026.10.27(火) 大阪府 大阪城ホール", then
// "18:00 / 19:00"), then the tour's notes, where the ticket rounds are.
func sonyLive(ctx context.Context, src domain.SourceConfig, env Env) (Result, error) {
	var res Result
	var feed struct {
		Items []sonyItem `json:"items"`
	}
	resp, err := jsonp(ctx, src, env, &feed)
	if err != nil || resp.NotModified {
		res.NotModified = resp != nil && resp.NotModified
		return res, err
	}
	now := env.now()
	for _, it := range feed.Items {
		title := strings.TrimSpace(norm.NFKC.String(it.TourName))
		title = strings.TrimSpace(strings.TrimPrefix(title, "[終了]"))
		if title == "" {
			continue
		}
		var b strings.Builder
		b.WriteString(title + "\n")
		for _, l := range it.LiveItem {
			line := strings.TrimSpace(fmt.Sprintf("%s(%s) %s %s", l.Date, l.Youbi, l.Area, l.Place))
			b.WriteString(norm.NFKC.String(line) + "\n")
			switch {
			case l.Open != "" && l.Start != "":
				b.WriteString(l.Open + " / " + l.Start + "\n")
			case l.Start != "":
				b.WriteString("開演 " + l.Start + "\n")
			}
		}
		if info := norm.NFKC.String(HTMLText(it.TourInfo)); info != "" {
			if len(it.LiveItem) > 0 {
				// The shows are above: the notes are for the tickets.
				b.WriteString("\n■チケット・公演情報\n")
				info = afterSchedule(info)
			}
			b.WriteString(info + "\n")
		}
		link := it.Link
		if link == "" {
			link = it.RelatedLinkURL
		}
		ref, err := url.Parse(link)
		if err != nil || link == "" {
			continue
		}
		page := resp.URL
		if page == "" {
			page = src.URL
		}
		base, _ := url.Parse(page)
		res.Announcements = append(res.Announcements, domain.PublicAnnouncement{
			SourceID: src.ID, URL: base.ResolveReference(ref).String(), Title: title,
			RawText: strings.TrimSpace(b.String()), ArtistHints: []string{src.Artist},
			FetchedAt: now, PublishedAt: now, Schedule: src.Artist != "",
		})
	}
	if len(res.Announcements) == 0 {
		res.Warnings = append(res.Warnings, "接口里暂时没有 Live")
	}
	return res, nil
}

var leadingDateRe = regexp.MustCompile(`^\s*(?:\d{4}\s*[年./]\s*)?\d{1,2}\s*[月/.]\s*\d{1,2}`)

// afterSchedule drops the notes' own list of the shows, which the feed
// already gives as liveItem: the notes are kept from the line after the
// last one that starts with a date ("2026年12月6日(日) 開場 14:30"), so a
// show is not read twice, with its venue a line apart.
func afterSchedule(info string) string {
	lines := strings.Split(info, "\n")
	last := -1
	for i, l := range lines {
		if leadingDateRe.MatchString(l) && !ticketLineRe.MatchString(l) {
			last = i
		}
	}
	return strings.Join(lines[last+1:], "\n")
}

var ticketLineRe = regexp.MustCompile(`先行|抽選|受付|発売|販売|~`)

// jsonp reads a JSON or JSONP ("callback({...});") answer into v; resp
// is nil on a failed request.
func jsonp(ctx context.Context, src domain.SourceConfig, env Env, v any) (*Response, error) {
	resp, err := env.Get.Get(ctx, src.URL)
	if err != nil {
		return nil, err
	}
	if resp.NotModified {
		return resp, nil
	}
	body := strings.TrimSpace(resp.Text)
	if i := strings.Index(body, "("); i >= 0 && !strings.HasPrefix(body, "{") {
		body = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(body[i+1:]), ";"), ")")
	}
	if err := json.Unmarshal([]byte(body), v); err != nil {
		return resp, fmt.Errorf("接口返回的不是预期的 JSON：%w", err)
	}
	return resp, nil
}

// sonyNews reads Sony Music's news feed of an artist
// (…/json/v2/artist/<folder>/information/start/0/count/20): the official
// announcements, where the ticket rounds and new shows are told first.
// With keywords, only the news whose title has one is kept.
func sonyNews(ctx context.Context, src domain.SourceConfig, env Env) (Result, error) {
	var res Result
	var feed struct {
		Items []struct {
			ID       json.Number `json:"id"`
			Title    string      `json:"title"`
			Category string      `json:"category"`
			Date     string      `json:"date"` // 2026.10.16
			Article  string      `json:"article"`
			Link     string      `json:"link"`
		} `json:"items"`
	}
	resp, err := jsonp(ctx, src, env, &feed)
	if err != nil || resp.NotModified {
		res.NotModified = resp != nil && resp.NotModified
		return res, err
	}
	now := env.now()
	base, _ := url.Parse(src.URL)
	for _, it := range feed.Items {
		title := strings.TrimSpace(norm.NFKC.String(it.Title))
		if title == "" || !wanted(title, src.Article) {
			continue
		}
		ref, err := url.Parse(it.Link)
		if err != nil || it.Link == "" {
			continue
		}
		when := now
		if t, err := time.ParseInLocation("2006.01.02", it.Date, domain.Tokyo); err == nil {
			when = t.Add(12 * time.Hour)
		}
		text := strings.ReplaceAll(it.Article, "\r\n", "<br>")
		text = norm.NFKC.String(HTMLText(strings.ReplaceAll(text, "\n", "<br>")))
		var hints []string
		if src.Artist != "" {
			hints = []string{src.Artist}
		}
		res.Announcements = append(res.Announcements, domain.PublicAnnouncement{
			SourceID: src.ID, URL: base.ResolveReference(ref).String(), Title: title,
			RawText: strings.TrimSpace(title + "\n" + text), ArtistHints: hints,
			FetchedAt: now, PublishedAt: when,
		})
	}
	if len(res.Announcements) == 0 {
		res.Warnings = append(res.Warnings, "接口里暂时没有符合关键词的公告")
	}
	return res, nil
}
