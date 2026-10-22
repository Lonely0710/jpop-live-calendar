package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// dataDir is the per-user directory of the app's public data: the
// configuration, the announcement cache and the pinned artists.
func dataDir() (string, error) {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "Live Calendar"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Live Calendar"), nil
}

// UserPath is the configuration file of the user, which Load reads
// before the one of the project and the built-in copy.
func UserPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sources.yaml"), nil
}
