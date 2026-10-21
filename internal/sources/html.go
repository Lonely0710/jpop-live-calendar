package sources

import (
	"html"
	"regexp"
	"strings"
)

var (
	dropRe  = regexp.MustCompile(`(?is)<(script|style|noscript|svg|iframe)\b.*?</(script|style|noscript|svg|iframe)>|<!--.*?-->`)
	breakRe = regexp.MustCompile(`(?i)<br\s*/?>|</(p|div|li|h[1-6]|tr|dt|dd|section|article|table|ul|ol)>|<(p|li|tr|h[1-6])\b[^>]*>`)
	tagRe   = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRe = regexp.MustCompile(`[ \t\f\v\x{3000}]+`)
	blankRe = regexp.MustCompile(`\n{3,}`)
	// A post's date and its category labels share a paragraph on some
	// sites: "2026.10.09<span class="category">suis</span>". The labels
	// are not part of the date.
	categoryRe = regexp.MustCompile(`(?i)<span\b[^>]*\bclass\s*=\s*["'][^"']*\bcategory\b[^"']*["'][^>]*>`)
)

// HTMLText turns an HTML fragment into plain text with one line per
// paragraph or <br>.
func HTMLText(s string) string {
	s = dropRe.ReplaceAllString(s, "")
	s = categoryRe.ReplaceAllString(s, "\n")
	s = breakRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(spaceRe.ReplaceAllString(l, " "))
	}
	s = strings.Join(lines, "\n")
	return strings.TrimSpace(blankRe.ReplaceAllString(s, "\n\n"))
}

// between returns the part of s after start and before the next end; ok is
// false when start is missing. An empty start means the beginning, an
// empty or missing end the end.
func between(s, start, end string) (string, bool) {
	if start != "" {
		i := strings.Index(s, start)
		if i < 0 {
			return "", false
		}
		s = s[i+len(start):]
	}
	if end != "" {
		if j := strings.Index(s, end); j >= 0 {
			s = s[:j]
		}
	}
	return s, true
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func htmlTitle(s string) string {
	if m := titleRe.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(html.UnescapeString(m[1]))
	}
	return ""
}

// names splits an artist list ("macico, マコトコンドウ" or one per line).
func names(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		for _, n := range strings.Split(l, ",") {
			n = strings.TrimSpace(n)
			if n != "" && len([]rune(n)) <= 60 {
				out = append(out, n)
			}
		}
	}
	return out
}
