package spotlighticons

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

var colorRe = regexp.MustCompile(`#[0-9A-Fa-f]{6}\b`)

func TestLibrary(t *testing.T) {
	allowed := map[string]bool{}
	for _, v := range Palette {
		allowed[strings.ToUpper(v)] = true
	}
	files, _ := fs.Glob(FS(), "*.svg")
	if len(files) != len(All) {
		t.Fatalf("svg/ has %d files but All has %d names; run go generate", len(files), len(All))
	}
	for _, n := range All {
		b, ok := Lookup(n)
		if !ok {
			t.Fatalf("%s: missing", n)
		}
		s := string(b)
		if !strings.Contains(s, `viewBox="0 0 24 24"`) {
			t.Errorf("%s: viewBox must be 0 0 24 24", n)
		}
		if strings.Contains(s, "<mask") {
			t.Errorf("%s: masks are not supported by MyGo's SVG renderer", n)
		}
		for _, c := range colorRe.FindAllString(s, -1) {
			if !allowed[strings.ToUpper(c)] {
				t.Errorf("%s: color %s is outside the Spotlight palette", n, c)
			}
		}
		if GroupOf(n) == "" {
			t.Errorf("%s: no group", n)
		}
	}
}

// Every icon has its own picture: one meaning, one name. Two names for the
// same drawing made the interface pick them at random.
func TestNoDuplicatePictures(t *testing.T) {
	seen := map[string]Name{}
	for _, n := range All {
		b, _ := Lookup(n)
		s := strings.Join(strings.Fields(string(b)), " ")
		if other, ok := seen[s]; ok {
			t.Errorf("%s and %s are the same picture", other, n)
		}
		seen[s] = n
	}
}
