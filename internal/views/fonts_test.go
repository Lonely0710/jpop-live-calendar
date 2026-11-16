package views

import (
	"log"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestMain registers the app's own faces before any test runs: the window
// embeds them in main.go, and the screenshots must show what the window
// shows rather than a serif the machine happens to have.
func TestMain(m *testing.M) {
	for _, f := range []struct{ path, name string }{
		{"../../resources/Pacifico-Regular.ttf", "Pacifico"},
		{"../../resources/EBGaramond-Regular.ttf", "EB Garamond"},
		{"../../resources/PPEditorialNew-Ultrabold.otf", "PP Editorial New"},
	} {
		b, err := os.ReadFile(f.path)
		if err != nil {
			log.Printf("%s: %v", f.name, err)
			continue
		}
		if err := ui.RegisterFont(b, f.name); err != nil {
			log.Printf("%s: %v", f.name, err)
		}
	}
	os.Exit(m.Run())
}
