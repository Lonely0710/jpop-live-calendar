package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// The first column of the calendar's week, as prefs.json spells it.
const (
	WeekStartMonday = "mon" // the default
	WeekStartSunday = "sun"
)

// Prefs are the choices kept across launches that are not part of the
// sources: the first column of the calendar's week.
type Prefs struct {
	WeekStart string `json:"week_start"`
}

// PrefsPath is the file of the preferences. It sits next to the user's
// sources.yaml, as locale.json and pins.json do.
func PrefsPath() (string, error) {
	p, err := UserPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "prefs.json"), nil
}

// LoadPrefs reads prefs.json; a missing file keeps the defaults, a week
// that starts on Monday.
func LoadPrefs() (Prefs, error) {
	p, err := PrefsPath()
	if err != nil {
		return Prefs{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Prefs{WeekStart: WeekStartMonday}, nil
	}
	if err != nil {
		return Prefs{}, err
	}
	var f Prefs
	if err := json.Unmarshal(b, &f); err != nil {
		return Prefs{}, err
	}
	if f.WeekStart == "" {
		f.WeekStart = WeekStartMonday
	}
	if f.WeekStart != WeekStartMonday && f.WeekStart != WeekStartSunday {
		return Prefs{}, fmt.Errorf("prefs.json: unknown week_start %q", f.WeekStart)
	}
	return f, nil
}

// Weekday is the first column the preferences ask for.
func (p Prefs) Weekday() time.Weekday {
	if p.WeekStart == WeekStartSunday {
		return time.Sunday
	}
	return time.Monday
}

// SaveWeekStart writes the first column of the week, keeping the rest of
// the preferences as they were.
func SaveWeekStart(w time.Weekday) error {
	key := WeekStartMonday
	if w == time.Sunday {
		key = WeekStartSunday
	}
	p, err := PrefsPath()
	if err != nil {
		return err
	}
	prefs, err := LoadPrefs()
	if err != nil {
		prefs = Prefs{}
	}
	prefs.WeekStart = key
	b, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
