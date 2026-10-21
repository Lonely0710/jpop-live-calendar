package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// PinsPath is the file of the artists pinned to the top of the artists
// page. It sits next to the user's sources.yaml.
func PinsPath() (string, error) {
	p, err := UserPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "pins.json"), nil
}

// LoadPins reads the IDs of the pinned artists; a missing file means none.
func LoadPins() ([]string, error) {
	p, err := PinsPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(b, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// SavePins writes the IDs of the pinned artists.
func SavePins(ids []string) error {
	p, err := PinsPath()
	if err != nil {
		return err
	}
	if ids == nil {
		ids = []string{}
	}
	b, err := json.MarshalIndent(ids, "", "  ")
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
