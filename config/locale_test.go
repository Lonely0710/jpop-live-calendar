package config

import "testing"

func TestValidLocale(t *testing.T) {
	for _, tag := range []string{"zh-Hans", "ja", "en"} {
		if !ValidLocale(tag) {
			t.Errorf("ValidLocale(%q) = false", tag)
		}
	}
	for _, tag := range []string{"", "zh", "zh-Hant", "ja-JP", "de"} {
		if ValidLocale(tag) {
			t.Errorf("ValidLocale(%q) = true", tag)
		}
	}
}
