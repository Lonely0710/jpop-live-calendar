package config

import (
	"os"
	"slices"
	"testing"
)

func TestPinsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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
