// Package normalize turns the many ways Japanese announcements write the
// same thing into one: widths, dates without a year, times, prefectures,
// artist and venue names. Values it cannot settle stay empty.
package normalize

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"

	"jpop-live-calendar/internal/domain"
)

// Text folds compatibility characters (KANGXI radicals such as "⽇" to
// "日"), full-width ASCII to half-width and half-width kana to full-width,
// unifies wave dashes and trims each line's spaces.
func Text(s string) string {
	s = norm.NFKC.String(width.Fold.String(s))
	s = strings.NewReplacer("〜", "~", "～", "~", "−", "-", "‐", "-", "–", "-", "\u00a0", " ", "\r\n", "\n", "\r", "\n").Replace(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.Join(lines, "\n")
}

// Key is the matching form of a name: folded, lower-case, without spaces
// and punctuation, so "Zepp Shinjuku (TOKYO)" and "ZeppShinjuku" agree.
func Key(s string) string {
	s = strings.ToLower(norm.NFKC.String(width.Fold.String(s)))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ID returns a short stable identifier for the parts.
func ID(prefix string, parts ...string) string {
	h := sha1.Sum([]byte(strings.Join(parts, "\x1f")))
	return prefix + hex.EncodeToString(h[:6])
}

// Hash returns the content hash of a text.
func Hash(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

var prefectures = []string{
	"北海道", "青森", "岩手", "宮城", "秋田", "山形", "福島", "茨城", "栃木", "群馬",
	"埼玉", "千葉", "東京", "神奈川", "新潟", "富山", "石川", "福井", "山梨", "長野",
	"岐阜", "静岡", "愛知", "三重", "滋賀", "京都", "大阪", "兵庫", "奈良", "和歌山",
	"鳥取", "島根", "岡山", "広島", "山口", "徳島", "香川", "愛媛", "高知", "福岡",
	"佐賀", "長崎", "熊本", "大分", "宮崎", "鹿児島", "沖縄",
}

// Prefecture returns the prefecture a token names ("東京", "東京都",
// "大阪府"), or "".
func Prefecture(tok string) string {
	tok = strings.TrimSpace(tok)
	for _, p := range prefectures {
		switch tok {
		case p, p + "都", p + "府", p + "県":
			return p
		}
	}
	return ""
}

// cities names the prefecture of a city a schedule may give instead:
// "仙台 宮城・セキスイハイムスーパーアリーナ".
var cities = map[string]string{
	"札幌": "北海道", "仙台": "宮城", "さいたま": "埼玉", "幕張": "千葉", "横浜": "神奈川",
	"川崎": "神奈川", "金沢": "石川", "名古屋": "愛知", "神戸": "兵庫", "高松": "香川",
	"松山": "愛媛", "博多": "福岡", "北九州": "福岡", "那覇": "沖縄",
}

// SplitPrefecture splits "広島 広島LIVE VANQUISH" into the prefecture and
// the venue; without a leading prefecture it returns the text as venue.
// A leading city ("仙台 ...") names its prefecture, and a repeated
// prefecture ("大阪 大阪・大阪城ホール") is dropped from the venue.
func SplitPrefecture(s string) (pref, venue string) {
	s = strings.TrimSpace(s)
	for _, o := range []string{"その他 ", "海外 "} {
		s = strings.TrimSpace(strings.TrimPrefix(s, o))
	}
	pref, venue = splitPrefecture(s)
	if pref == "" {
		if first, rest, ok := strings.Cut(s, " "); ok && cities[first] != "" {
			pref, venue = cities[first], strings.TrimSpace(rest)
		}
	}
	if pref != "" {
		if p, v := splitPrefecture(venue); p == pref && v != "" && !strings.Contains(venue, " ") {
			venue = v
		} else if first, rest, ok := strings.Cut(venue, "・"); ok && Prefecture(first) == pref && rest != "" {
			venue = strings.TrimSpace(rest)
		}
	}
	return pref, venue
}

func splitPrefecture(s string) (pref, venue string) {
	first, rest, ok := strings.Cut(s, " ")
	if ok {
		if p := Prefecture(first); p != "" {
			return p, strings.TrimSpace(rest)
		}
	}
	for _, sep := range []string{"・", "/"} {
		if first, rest, ok := strings.Cut(s, sep); ok {
			if p := Prefecture(first); p != "" {
				return p, strings.TrimSpace(rest)
			}
		}
	}
	return "", s
}

var weekdays = map[string]time.Weekday{
	"日": time.Sunday, "月": time.Monday, "火": time.Tuesday, "水": time.Wednesday,
	"木": time.Thursday, "金": time.Friday, "土": time.Saturday,
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// Weekday reads the weekday of "(木)", "(火・祝)", "(Sat)"; ok is false
// when there is none.
func Weekday(s string) (time.Weekday, bool) {
	s = strings.ToLower(strings.Trim(strings.TrimSpace(s), "()"))
	for _, k := range []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"} {
		if strings.HasPrefix(s, k) {
			return weekdays[k], true
		}
	}
	for _, r := range s {
		w, ok := weekdays[string(r)]
		return w, ok
	}
	return 0, false
}

// InferYear chooses the year of a month and day written without one,
// relative to ref (the announcement's publication): the weekday, when
// given, must match; otherwise the first such day from 60 days before
// ref. ok is false when no year matches the weekday (a typo to review).
func InferYear(month time.Month, day int, wd *time.Weekday, ref time.Time) (int, bool) {
	ref = ref.In(domain.Tokyo)
	floor := domain.DateOf(ref, domain.Tokyo).AddDays(-60)
	var best *domain.Date
	for y := ref.Year() - 1; y <= ref.Year()+2; y++ {
		d := domain.Date{Year: y, Month: month, Day: day}
		if d.Time(time.UTC).Day() != day { // 2/29 in a common year
			continue
		}
		if wd != nil && d.Weekday() != *wd {
			continue
		}
		if d.Before(floor) {
			continue
		}
		if best == nil || d.Before(*best) {
			best = &d
		}
	}
	if best == nil {
		if wd != nil {
			y, _ := InferYear(month, day, nil, ref)
			return y, false
		}
		return ref.Year(), false
	}
	return best.Year, true
}

var clockRe = regexp.MustCompile(`(\d{1,2})\s*(?::|時)\s*(\d{2})?`)

// ParseClock reads "18:00", "18時", "18時30分"; ok is false for none.
func ParseClock(s string) (domain.Clock, bool) {
	m := clockRe.FindStringSubmatch(s)
	if m == nil {
		return domain.Clock{}, false
	}
	h := atoi(m[1])
	min := 0
	if m[2] != "" {
		min = atoi(m[2])
	}
	if h > 29 || min > 59 {
		return domain.Clock{}, false
	}
	return domain.Clock{Hour: h, Minute: min}, true
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// Atoi converts the digits of s.
func Atoi(s string) int { return atoi(s) }

// URL normalizes an announcement URL for de-duplication: no fragment, no
// tracking parameters, no trailing slash.
func URL(u string) string {
	u = strings.TrimSpace(u)
	if i := strings.IndexByte(u, '#'); i >= 0 {
		u = u[:i]
	}
	if base, q, ok := strings.Cut(u, "?"); ok {
		var keep []string
		for _, p := range strings.Split(q, "&") {
			if !strings.HasPrefix(p, "utm_") && p != "" {
				keep = append(keep, p)
			}
		}
		u = base
		if len(keep) > 0 {
			u += "?" + strings.Join(keep, "&")
		}
	}
	return strings.TrimSuffix(u, "/")
}
