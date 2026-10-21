package crawler

import (
	"bufio"
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// robots holds the rules of robots.txt that apply to the app: the group
// for its user agent if there is one, otherwise the "*" group. Rules
// match as RFC 9309 says: by prefix, "*" standing for any characters and
// a final "$" for the end; the longest matching rule wins, Allow on a tie.
//
// unavailable marks a robots.txt that could not be read (a 5xx, as a site
// under maintenance, or a network error): RFC 9309 says to assume that it
// disallows everything, but only until it can be read again.
type robots struct {
	allow, disallow []string
	unavailable     bool
	status          int // the HTTP status of the unreadable robots.txt; 0 for a network error
}

func (r *robots) allowed(path string) bool {
	if r == nil {
		return true
	}
	if path == "" {
		path = "/"
	}
	best, ok := -1, true
	for _, p := range r.disallow {
		if p != "" && ruleMatch(p, path) && len(p) > best {
			best, ok = len(p), false
		}
	}
	for _, p := range r.allow {
		if p != "" && ruleMatch(p, path) && len(p) >= best {
			best, ok = len(p), true
		}
	}
	return ok
}

// ruleMatch reports whether a robots.txt path pattern matches path.
func ruleMatch(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	pattern = strings.TrimSuffix(pattern, "$")
	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(path, parts[0]) {
		return false
	}
	rest := path[len(parts[0]):]
	for i, part := range parts[1:] {
		last := i == len(parts)-2
		if last && anchored {
			return strings.HasSuffix(rest, part)
		}
		j := strings.Index(rest, part)
		if j < 0 {
			return false
		}
		rest = rest[j+len(part):]
	}
	return !anchored || rest == "" || len(parts) > 1 && parts[len(parts)-1] == ""
}

func (f *Fetcher) loadRobots(ctx context.Context, hs *hostState, u *url.URL) *robots {
	ru := u.Scheme + "://" + u.Host + "/robots.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ru, nil)
	if err != nil {
		return &robots{unavailable: true}
	}
	req.Header.Set("User-Agent", f.ua())
	resp, err := f.Client.Do(req)
	hs.last = time.Now()
	if err != nil {
		// Unreachable for now: RFC 9309 says to crawl nothing until it can
		// be read, and the fetcher retries soon (robotsRetryTTL).
		return &robots{disallow: []string{"/"}, unavailable: true}
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= 500:
		// A site under maintenance often answers robots.txt itself with a
		// 503. Treat it as disallowed for now, but retry soon: it is not a
		// rule the site chose.
		return &robots{disallow: []string{"/"}, unavailable: true, status: resp.StatusCode}
	case resp.StatusCode >= 400:
		return &robots{} // no robots.txt (any 4xx): no rules, as RFC 9309 says
	}
	return parseRobots(resp.Body, "livecalendar")
}

func parseRobots(r interface{ Read([]byte) (int, error) }, agent string) *robots {
	type group struct {
		agents          []string
		allow, disallow []string
	}
	var groups []*group
	var cur *group
	inAgents := false
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		switch k {
		case "user-agent":
			if !inAgents {
				cur = &group{}
				groups = append(groups, cur)
			}
			cur.agents = append(cur.agents, strings.ToLower(v))
			inAgents = true
		case "allow", "disallow":
			inAgents = false
			if cur == nil {
				continue
			}
			if k == "allow" {
				cur.allow = append(cur.allow, v)
			} else {
				cur.disallow = append(cur.disallow, v)
			}
		default:
			inAgents = false
		}
	}
	var star, mine *group
	for _, g := range groups {
		for _, a := range g.agents {
			if a == "*" && star == nil {
				star = g
			} else if a != "*" && a != "" && (strings.Contains(agent, a) || strings.Contains(a, agent)) {
				mine = g
			}
		}
	}
	g := mine
	if g == nil {
		g = star
	}
	if g == nil {
		return &robots{}
	}
	return &robots{allow: g.allow, disallow: g.disallow}
}
