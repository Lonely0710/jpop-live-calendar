package views

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// isDarwin keeps the macOS shortcuts and wording while the same app runs
// on Windows.
var isDarwin = runtime.GOOS == "darwin"

// platformText picks the wording of the current desktop.
func platformText(mac, other string) string {
	if isDarwin {
		return mac
	}
	return other
}

// platformModifiers picks the keystroke of the current desktop.
func platformModifiers(mac, other ui.Modifiers) ui.Modifiers {
	if isDarwin {
		return mac
	}
	return other
}

// The shortcuts as the interface writes them: their glyphs set apart, so
// "⌘Q" reads as keys wherever a tooltip or a settings row spells it out.
var (
	todayShortcut     = spacedKeys(platformText("⌘T", "Ctrl+T"))
	syncShortcut      = spacedKeys(platformText("⌘R", "Ctrl+R"))
	inspectorShortcut = spacedKeys(platformText("⌥⌘I", "Ctrl+Alt+I"))
	sidebarShortcut   = spacedKeys(platformText("⌃⌘S", "Ctrl+Shift+S"))
	sidebarModifiers  = platformModifiers(ui.Cmd|ui.Ctrl, ui.Cmd|ui.Shift)
)

func viewShortcut(i int) string {
	if isDarwin {
		return spacedKeys(fmt.Sprintf("⌘%d", i+1))
	}
	return fmt.Sprintf("Ctrl+%d", i+1)
}

// spacedKeys sets a shortcut's glyphs apart: "⌃⌘S" reads "⌃ ⌘ S". Windows
// spells its keys in words ("Ctrl+Shift+S") and keeps them as written.
func spacedKeys(keys string) string {
	if strings.Contains(keys, "+") {
		return keys
	}
	var b strings.Builder
	for _, r := range keys {
		if r == ' ' {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isKeys reports whether a code run is a keystroke ("⌘Q", "Ctrl+R") rather
// than a file name or a key of a file.
func isKeys(s string) bool {
	return strings.ContainsAny(s, "⌘⌃⌥⇧") || strings.Contains(s, "+")
}
