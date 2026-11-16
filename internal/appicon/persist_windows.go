package appicon

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"

	"jpop-live-calendar/config"
	"jpop-live-calendar/resources/appicons"
)

func persist(key string, done func(error)) {
	exe, err := os.Executable()
	if err != nil {
		done(err)
		return
	}
	go func() { done(pointShortcuts(exe, key)) }()
}

// reconcile writes the shortcuts again: the script only saves the ones
// that point elsewhere, which after a reinstall is all of them.
func reconcile(key string, done func(error)) { persist(key, done) }

// pointShortcuts gives the shortcuts to exe the icon of key: a copy of its
// .ico, or the executable's own icon for Default.
func pointShortcuts(exe, key string) error {
	icon := exe + ",0"
	if ico := appicons.ICO(key); ico != nil {
		p, err := writeICO(exe, key, ico)
		if err != nil {
			return err
		}
		icon = p + ",0"
	}
	n, err := updateShortcuts(exe, icon)
	if n > 0 {
		refreshShell()
	}
	return err
}

// writeICO keeps the .ico the shortcuts point to: next to the executable,
// in the install folder the uninstaller removes, or else next to
// prefs.json. It is written only when missing or different.
func writeICO(exe, key string, ico []byte) (string, error) {
	dirs := []string{filepath.Join(filepath.Dir(exe), "icons")}
	if p, err := config.PrefsPath(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(p), "icons"))
	}
	var last error
	for _, dir := range dirs {
		p := filepath.Join(dir, key+".ico")
		if b, err := os.ReadFile(p); err == nil && bytes.Equal(b, ico) {
			return p, nil
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			last = err
			continue
		}
		if err := os.WriteFile(p, ico, 0o644); err != nil {
			last = err
			continue
		}
		return p, nil
	}
	return "", last
}

// shortcutScript points every shortcut to $env:LC_EXE in the Start menu,
// on the desktop and pinned to the taskbar at $env:LC_ICON, and prints how
// many it saved. The paths come in the environment, so nothing needs
// quoting.
const shortcutScript = `
$ErrorActionPreference = 'Stop'
$exe = $env:LC_EXE
$icon = $env:LC_ICON
$sh = New-Object -ComObject WScript.Shell
$dirs = @(
  [Environment]::GetFolderPath('Programs'),
  [Environment]::GetFolderPath('Desktop'),
  (Join-Path $env:APPDATA 'Microsoft\Internet Explorer\Quick Launch\User Pinned\TaskBar')
)
$n = 0
foreach ($d in $dirs) {
  if (-not $d -or -not (Test-Path -LiteralPath $d)) { continue }
  foreach ($f in Get-ChildItem -LiteralPath $d -Filter *.lnk -File -Recurse -Depth 1 -ErrorAction SilentlyContinue) {
    $l = $sh.CreateShortcut($f.FullName)
    if ($l.TargetPath -ieq $exe -and $l.IconLocation -ne $icon) {
      $l.IconLocation = $icon
      $l.Save()
      $n++
    }
  }
}
Write-Output $n
`

// updateShortcuts runs shortcutScript in a hidden PowerShell and returns
// how many shortcuts it changed.
func updateShortcuts(exe, icon string) (int, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePS(shortcutScript))
	cmd.Env = append(os.Environ(), "LC_EXE="+exe, "LC_ICON="+icon)
	// CREATE_NO_WINDOW: no console flashes up.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return 0, fmt.Errorf("appicon: shortcuts: %w", err)
		}
		return 0, fmt.Errorf("appicon: shortcuts: %w: %s", err, msg)
	}
	n, err := strconv.Atoi(strings.TrimSpace(out.String()))
	if err != nil {
		return 0, errors.New("appicon: shortcuts: unexpected output " + strconv.Quote(out.String()))
	}
	return n, nil
}

// encodePS is a script for -EncodedCommand: UTF-16LE, in base64.
func encodePS(s string) string {
	u := utf16.Encode([]rune(s))
	b := make([]byte, 2*len(u))
	for i, r := range u {
		b[2*i], b[2*i+1] = byte(r), byte(r>>8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// refreshShell tells Explorer that icons changed (SHCNE_ASSOCCHANGED), so
// the shortcuts show the new one without a sign-out.
func refreshShell() {
	proc := syscall.NewLazyDLL("shell32.dll").NewProc("SHChangeNotify")
	if proc.Find() == nil {
		proc.Call(0x08000000, 0, 0, 0)
	}
}
