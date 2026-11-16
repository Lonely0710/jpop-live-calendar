package appicons

import (
	"bytes"
	"image/png"
	"testing"
)

func TestThumbs(t *testing.T) {
	for _, k := range Keys {
		b := Thumb(k)
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		if s := img.Bounds().Size(); s.X != 144 || s.Y != 144 {
			t.Fatalf("%s: thumb is %v", k, s)
		}
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize("gone"); got != Default {
		t.Fatalf("Normalize(gone) = %q", got)
	}
	if got := Normalize(""); got != Default {
		t.Fatalf("Normalize(\"\") = %q", got)
	}
	if got := Normalize(TicketSquare); got != TicketSquare {
		t.Fatalf("Normalize(%q) = %q", TicketSquare, got)
	}
}
