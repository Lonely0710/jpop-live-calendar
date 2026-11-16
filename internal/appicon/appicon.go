// Package appicon changes the app's icon to one the user chose in 设置, for
// good: the icon shows while the app runs (the Dock, ⌘Tab, the windows'
// title bars and taskbar buttons) and also where the system shows the app
// while it does not run:
//
//   - macOS: the .app bundle gets the icon as its custom Finder icon, as
//     "Get Info → paste an icon" does, so Finder, Launchpad and the Dock
//     show it after a quit too.
//   - Windows: the .exe's own icon cannot change, but the shortcuts the
//     user starts the app from (Start menu, desktop, pinned to the taskbar)
//     are pointed at a copy of the icon's .ico.
//
// An update replaces the bundle, and a reinstall writes the shortcuts
// again, both with the built-in icon: Reconcile, at every launch, writes
// the chosen icon back once it is gone.
package appicon

import (
	"errors"

	"github.com/egoist/mygo"

	"jpop-live-calendar/resources/appicons"
)

// ErrNotInstalled reports a build that runs outside an installed app
// (mygo dev, a bare binary), where there is nothing to write the icon to.
var ErrNotInstalled = errors.New("appicon: not running from an installed app")

// Show sets the icon of the running app. It does not touch the system's
// copy; Apply and Reconcile call it. Call it once the app is ready.
func Show(key string) error {
	png := appicons.PNG(appicons.Normalize(key))
	if png == nil {
		return nil // no icon of the running app on this platform
	}
	return mygo.App.Dock.SetIcon(png)
}

// Apply shows key's icon now and writes it where the system shows the app
// while it does not run. done receives the result of the second part,
// maybe on another goroutine (Windows writes the shortcuts in the
// background); nil for done ignores it. Call it on the main thread once
// the app is ready.
func Apply(key string, done func(error)) {
	key = appicons.Normalize(key)
	if done == nil {
		done = func(error) {}
	}
	if err := Show(key); err != nil {
		done(err)
		return
	}
	persist(key, done)
}

// Reconcile runs at launch: it shows the chosen icon and, if an update or
// a reinstall put the built-in one back, writes the chosen icon again.
// done is as in Apply. Call it on the main thread once the app is ready.
func Reconcile(key string, done func(error)) {
	key = appicons.Normalize(key)
	if done == nil {
		done = func(error) {}
	}
	if key == appicons.Default {
		// The app's own icon: nothing to show over it, and nothing to
		// write back (a custom icon the user set in Finder stays).
		done(nil)
		return
	}
	if err := Show(key); err != nil {
		done(err)
		return
	}
	reconcile(key, done)
}
