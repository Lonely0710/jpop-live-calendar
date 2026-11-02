package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrefsWeekStartRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("AppData", home)

	// No prefs.json yet: the week starts on Monday.
	p, err := LoadPrefs()
	if err != nil || p.Weekday() != time.Monday {
		t.Fatalf("defaults = %+v, %v", p, err)
	}
	if err := SaveWeekStart(time.Sunday); err != nil {
		t.Fatal(err)
	}
	path, err := PrefsPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	p, err = LoadPrefs()
	if err != nil || p.Weekday() != time.Sunday {
		t.Fatalf("loaded = %+v, %v", p, err)
	}
	if err := SaveWeekStart(time.Monday); err != nil {
		t.Fatal(err)
	}
	if p, err = LoadPrefs(); err != nil || p.Weekday() != time.Monday {
		t.Fatalf("loaded = %+v, %v", p, err)
	}
}

func TestPrefsUnknownWeekStart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("AppData", home)
	path, err := PrefsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"week_start":"tue"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrefs(); err == nil {
		t.Fatal("expected an error for an unknown week_start")
	}
}
