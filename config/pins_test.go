package config

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestUserPathPerPlatform(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	p, err := UserPath()
	if err != nil {
		t.Fatal(err)
	}
	var want string
	switch runtime.GOOS {
	case "darwin":
		want = filepath.Join(home, "Library", "Application Support", "Live Calendar", "sources.yaml")
	case "windows":
		want = filepath.Join(home, "AppData", "Live Calendar", "sources.yaml")
	default:
		want = filepath.Join(home, ".config", "Live Calendar", "sources.yaml")
	}
	if p != want {
		t.Fatalf("UserPath = %q, want %q", p, want)
	}
}

func TestPinsRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	if ids, err := LoadPins(); err != nil || len(ids) != 0 {
		t.Fatalf("no pins yet: %v %v", ids, err)
	}
	want := []string{"a-one", "a-two"}
	if err := SavePins(want); err != nil {
		t.Fatal(err)
	}
	p, err := PinsPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPins()
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("loaded %v, %v", got, err)
	}
}
