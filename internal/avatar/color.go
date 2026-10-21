package avatar

import (
	"image"
	"image/color"
)

// Dominant returns the color of the largest block of one color in a
// picture, and the share of the picture it covers.
//
// Every pixel falls in one of 512 blocks of similar colors (8 steps of
// red, green and blue); the color is the mean of the pixels of the block
// with the most. Pixels that are almost white or almost black are left
// out when other colors cover at least a tenth of the picture: they are
// the backdrop or the shadows of a photo, not its color, and would make
// every artist's theme gray.
func Dominant(img image.Image) (color.RGBA, float64) {
	b := img.Bounds()
	if b.Empty() {
		return color.RGBA{128, 128, 128, 255}, 0
	}
	type block struct{ n, r, g, b int }
	var blocks [512]block
	step := max(1, max(b.Dx(), b.Dy())/96) // about 96×96 samples
	total, colored := 0, 0
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A < 128 {
				continue
			}
			total++
			i := int(c.R>>5)<<6 | int(c.G>>5)<<3 | int(c.B>>5)
			bl := &blocks[i]
			bl.n++
			bl.r += int(c.R)
			bl.g += int(c.G)
			bl.b += int(c.B)
			if !extreme(c.R, c.G, c.B) {
				colored++
			}
		}
	}
	if total == 0 {
		return color.RGBA{128, 128, 128, 255}, 0
	}
	skipExtremes := colored*10 >= total
	best := -1
	for i, bl := range blocks {
		if bl.n == 0 {
			continue
		}
		if skipExtremes && extreme(uint8(bl.r/bl.n), uint8(bl.g/bl.n), uint8(bl.b/bl.n)) {
			continue
		}
		if best < 0 || bl.n > blocks[best].n {
			best = i
		}
	}
	if best < 0 { // every block's mean is extreme: take the largest
		for i, bl := range blocks {
			if bl.n > 0 && (best < 0 || bl.n > blocks[best].n) {
				best = i
			}
		}
	}
	bl := blocks[best]
	return color.RGBA{uint8(bl.r / bl.n), uint8(bl.g / bl.n), uint8(bl.b / bl.n), 255}, float64(bl.n) / float64(total)
}

// extreme reports colors near white or black with little hue.
func extreme(r, g, b uint8) bool {
	hi := max(r, g, b)
	lo := min(r, g, b)
	return (lo >= 225) || (hi <= 30) || (hi-lo < 18 && (lo >= 200 || hi <= 45))
}
