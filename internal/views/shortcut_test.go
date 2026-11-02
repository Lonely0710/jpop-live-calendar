package views

import (
	"testing"
	"time"

	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/locales"
)

// The interface writes a shortcut with its glyphs set apart; Windows spells
// its keys in words and keeps them as written.
func TestSpacedKeys(t *testing.T) {
	cases := map[string]string{
		"⌃⌘S":          "⌃ ⌘ S",
		"⌘Q":           "⌘ Q",
		"⌥⌘I":          "⌥ ⌘ I",
		"Ctrl+Shift+S": "Ctrl+Shift+S",
		"Ctrl+T":       "Ctrl+T",
	}
	for in, want := range cases {
		if got := spacedKeys(in); got != want {
			t.Errorf("spacedKeys(%q) = %q, want %q", in, got, want)
		}
	}
}

// A backquoted run is a shortcut when it holds a modifier glyph or a "+",
// and a file name otherwise.
func TestIsKeys(t *testing.T) {
	for _, s := range []string{"⌘Q", "⌃ ⌘ S", "Ctrl+R"} {
		if !isKeys(s) {
			t.Errorf("isKeys(%q) = false", s)
		}
	}
	for _, s := range []string{"cache.json", "robots.txt", "/Users/me/config/sources.yaml", "Asia/Tokyo"} {
		if isKeys(s) {
			t.Errorf("isKeys(%q) = true", s)
		}
	}
}

// The week's first column is the user's choice: the month's headers follow
// theme.WeekStart.
func TestWeekdayHeadersFollowWeekStart(t *testing.T) {
	was := theme.WeekStart
	defer func() { theme.WeekStart = was }()

	theme.WeekStart = time.Monday
	if got := weekdayHeaders(); got[0] != locales.WeekdayShortMon() {
		t.Errorf("a week starting Monday begins with %q", got[0])
	}
	theme.WeekStart = time.Sunday
	if got := weekdayHeaders(); got[0] != locales.WeekdayShortSun() {
		t.Errorf("a week starting Sunday begins with %q", got[0])
	}
}
