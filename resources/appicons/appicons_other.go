//go:build !darwin && !windows

package appicons

// PNG has no runtime icon outside macOS and Windows.
func PNG(string) []byte { return nil }

// ICO has no shortcut icon outside Windows.
func ICO(string) []byte { return nil }
