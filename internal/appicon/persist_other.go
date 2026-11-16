//go:build !darwin && !windows

package appicon

// Nothing keeps an app icon outside macOS and Windows.
func persist(_ string, done func(error))   { done(ErrNotInstalled) }
func reconcile(_ string, done func(error)) { done(nil) }
