// Command _audit checks one thing per source: can its selectors match anything
// on the page at all? A follow / feature pattern that matches no link, or an
// item marker that is gone, means the source can never produce an
// announcement — the kind of breakage that stays silent in the app.
//
// It reads one page per source through crawler.Fetcher, so robots.txt, the
// per-host delay and the app's User-Agent all apply, and it deliberately says
// nothing about keyword filters: those are per-day, not structural.
//
//	go run ./internal/sources/_audit              # every enabled website source
//	go run ./internal/sources/_audit -only foo    # IDs containing foo
//	go run ./internal/sources/_audit -fetch foo   # run the adapter itself
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/crawler"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/parser"
	"jpop-live-calendar/internal/sources"
)

func main() {
	only := flag.String("only", "", "only sources whose ID contains this")
	all := flag.Bool("all", false, "include disabled sources")
	fetch := flag.String("fetch", "", "run the adapter of this source (opens its detail pages too)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	f := crawler.NewFetcher()
	ctx := context.Background()
	if *fetch != "" {
		runAdapter(ctx, f, cfg.Sources, *fetch)
		return
	}

	broken, total := 0, 0
	for _, src := range cfg.Sources {
		if src.Kind != domain.SourceOfficialSite {
			continue
		}
		if *only != "" && !strings.Contains(src.ID, *only) {
			continue
		}
		if !src.Enabled && !*all {
			continue
		}
		total++
		state, detail, problem := probe(ctx, f, src)
		mark := "ok  "
		if problem != "" {
			mark, broken = "FAIL", broken+1
		}
		fmt.Printf("%s %-32s %-8s %s\n", mark, src.ID, state, detail)
		if problem != "" {
			fmt.Printf("       └ %s\n", problem)
		}
	}
	fmt.Printf("\n%d website sources, %d whose selectors cannot match\n", total, broken)
}

func probe(ctx context.Context, f *crawler.Fetcher, src domain.SourceConfig) (state, detail, problem string) {
	started := time.Now()
	resp, err := f.Get(ctx, src.URL)
	if err != nil {
		return "error", err.Error(), "页面抓不到"
	}
	if resp.NotModified {
		return "304", "not modified", ""
	}
	links := sources.Links(resp.Text, resp.URL)
	follow := match(links, src.Follow)
	feature := match(links, src.Feature)
	items := 0
	if src.Article.Item != "" {
		items = strings.Count(resp.Text, src.Article.Item)
	}
	detail = fmt.Sprintf("links=%-3d follow=%-3d feature=%-3d items=%-3d %.1fs",
		len(links), follow, feature, items, time.Since(started).Seconds())

	switch {
	case src.Article.Item != "":
		if items == 0 {
			return "ok", detail, "item 标记在页面上找不到（页面结构已变化）"
		}
	case src.Follow == "":
		// The page itself is one announcement: nothing to match.
	case follow == 0 && feature == 0:
		return "ok", detail, "follow / feature 一个链接都匹配不到（配置与站点对不上，该源永远是 0 条）"
	}
	return "ok", detail, ""
}

// match counts the links whose address fits pattern; -1 means the pattern
// itself does not compile.
func match(links []sources.Link, pattern string) int {
	if pattern == "" {
		return 0
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return -1
	}
	n := 0
	for _, l := range links {
		if re.MatchString(l.URL) {
			n++
		}
	}
	return n
}

// runAdapter is the honest test: it runs one source the way a sync does and
// prints what came back — announcements, the shows they parse into, warnings.
func runAdapter(ctx context.Context, f *crawler.Fetcher, srcs []domain.SourceConfig, id string) {
	adapters := sources.Default()
	for _, src := range srcs {
		if !src.Enabled || !strings.Contains(src.ID, id) {
			continue
		}
		res, err := adapters[src.Kind].Fetch(ctx, src, sources.Env{Get: f, Now: time.Now})
		fmt.Printf("\n%s: %d announcements, err=%v\n", src.ID, len(res.Announcements), err)
		for _, a := range res.Announcements {
			out := parser.Parse(parser.Input{
				Title: a.Title, Text: a.RawText, ArtistHints: a.ArtistHints,
				Published: a.PublishedAt, Schedule: a.Schedule, Locale: "zh-Hans",
			})
			fmt.Printf("  · %s  [%s  %s]  → %d 场次\n", a.Title, a.PublishedAt.Format("2006-01-02"), a.Anchor, len(out.Events))
			for _, e := range out.Events {
				fmt.Printf("      %04d-%02d-%02d %s (%s)\n", e.Date.Year, e.Date.Month, e.Date.Day, e.Venue, e.Prefecture)
			}
		}
		for _, w := range res.Warnings {
			fmt.Printf("  ! %s\n", w)
		}
	}
}
