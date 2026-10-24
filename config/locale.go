package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Locales are the interface languages the app ships, the source first. An
// empty tag in locale.json means following the system's language.
var Locales = []string{"zh-Hans", "ja", "en"}

// LocalePath is the file of the chosen interface language. It sits next to
// the user's sources.yaml; MyGo keeps its own copy of the locale too, but
// cannot tell whether the user chose a language or the system.
func LocalePath() (string, error) {
	p, err := UserPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "locale.json"), nil
}

type localeFile struct {
	Locale string `json:"locale"`
}

// LoadLocale reads the chosen interface language; a missing file or an
// empty tag means following the system.
func LoadLocale() (string, error) {
	p, err := LocalePath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var f localeFile
	if err := json.Unmarshal(b, &f); err != nil {
		return "", err
	}
	if f.Locale != "" && !ValidLocale(f.Locale) {
		return "", fmt.Errorf("locale.json: unknown language %q", f.Locale)
	}
	return f.Locale, nil
}

// SaveLocale writes the chosen interface language; "" follows the system.
func SaveLocale(tag string) error {
	if tag != "" && !ValidLocale(tag) {
		return fmt.Errorf("unknown language %q", tag)
	}
	p, err := LocalePath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(localeFile{Locale: tag}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ValidLocale reports whether tag is one of the app's interface languages.
func ValidLocale(tag string) bool {
	return slices.Contains(Locales, tag)
}
