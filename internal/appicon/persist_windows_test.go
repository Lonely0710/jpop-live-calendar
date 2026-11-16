package appicon

import (
	"encoding/base64"
	"testing"
)

func TestEncodePS(t *testing.T) {
	b, err := base64.StdEncoding.DecodeString(encodePS("ab"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "a\x00b\x00" {
		t.Fatalf("encodePS = %q", b)
	}
}
