package views

import (
	"fmt"
	"runtime"

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

var (
	todayShortcut     = platformText("⌘T", "Ctrl+T")
	syncShortcut      = platformText("⌘R", "Ctrl+R")
	inspectorShortcut = platformText("⌥⌘I", "Ctrl+Alt+I")
	sidebarShortcut   = platformText("⌃⌘S", "Ctrl+Shift+S")
	sidebarModifiers  = platformModifiers(ui.Cmd|ui.Ctrl, ui.Cmd|ui.Shift)
)

func viewShortcut(i int) string {
	if isDarwin {
		return fmt.Sprintf("⌘%d", i+1)
	}
	return fmt.Sprintf("Ctrl+%d", i+1)
}
