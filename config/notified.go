package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// NotifiedPath is the file of the ticket deadlines already announced by a
// system notification. It sits next to the user's sources.yaml.
func NotifiedPath() (string, error) {
	p, err := UserPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "notified.json"), nil
}

// LoadNotified reads the deadlines already announced, keyed by the
// notification's ID; a missing file means none. Only public deadlines are
// stored, never user data.
func LoadNotified() (map[string]string, error) {
	p, err := NotifiedPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
}

// SaveNotified writes the deadlines already announced.
func SaveNotified(m map[string]string) error {
	p, err := NotifiedPath()
	if err != nil {
		return err
	}
	if m == nil {
		m = map[string]string{}
	}
	b, err := json.MarshalIndent(m, "", "  ")
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
