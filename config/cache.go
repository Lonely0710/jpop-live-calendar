package config

import "path/filepath"

// CachePath is the file of the announcements from the last sync. It sits
// next to the user's sources.yaml.
func CachePath() (string, error) {
	p, err := UserPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "cache.json"), nil
}

// AvatarCachePath is the file of the public artist pictures. It sits next
// to the user's sources.yaml, and contains no user data.
func AvatarCachePath() (string, error) {
	p, err := UserPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "avatars.json"), nil
}
