package ui

import (
	"math"
	"strconv"
	"testing"
)

// Contrast is the whole point of a palette, so it is a test and not a taste:
// text has to be readable on the page it sits on, structure has to recede, and
// a selected row has to be legible (1.55:1 shipped once — a row you could see
// and not read).
func contrast(a, b string) float64 {
	lum := func(c string) float64 {
		n, _ := strconv.Atoi(c)
		cube := []float64{0, 95, 135, 175, 215, 255}
		var r, g, bl float64
		if n >= 232 {
			v := 8 + float64(n-232)*10
			r, g, bl = v, v, v
		} else {
			i := n - 16
			r, g, bl = cube[i/36], cube[(i%36)/6], cube[i%6]
		}
		f := func(u float64) float64 {
			u /= 255
			if u <= 0.03928 {
				return u / 12.92
			}
			return math.Pow((u+0.055)/1.055, 2.4)
		}
		return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(bl)
	}
	hi, lo := lum(a), lum(b)
	if lo > hi {
		hi, lo = lo, hi
	}
	return (hi + 0.05) / (lo + 0.05)
}

func TestThePaletteIsReadable(t *testing.T) {
	const page = "234" // a dark terminal

	for _, c := range []struct {
		name, colour string
		min          float64
	}{
		{"colText", colText, 7},
		{"colDim", colDim, 4.5}, // WCAG AA for secondary text
		{"colFaint", colFaint, 3},
		{"colOK", colOK, 4.5},
		{"colErr", colErr, 4.5},
		{"colRun", colRun, 4.5},
		{"colRead", colRead, 4.5},
		{"colWrite", colWrite, 4.5},
		{"spinnerColor", spinnerColor, 3},
	} {
		if got := contrast(c.colour, page); got < c.min {
			t.Errorf("%s (%s) is %.1f:1 on the page, needs %.1f:1", c.name, c.colour, got, c.min)
		}
	}

	// Structure must NOT reach text contrast: that is what keeps it structure.
	if got := contrast(colRule, page); got > 2.5 {
		t.Errorf("colRule is %.1f:1 — too loud for a gutter", got)
	}

	// A selected row: legible text, and a fill the eye can see.
	if got := contrast(colSelFG, colSelBG); got < 4.5 {
		t.Errorf("selected text is %.1f:1 on its fill, needs 4.5:1", got)
	}
	if got := contrast(colSelBG, page); got < 1.4 {
		t.Errorf("the selected fill is %.1f:1 off the page — invisible", got)
	}

	// Failure and success must differ in LUMINANCE as well as hue, or they are
	// one colour to a red-green eye.
	if d := math.Abs(contrast(colOK, page) - contrast(colErr, page)); d < 0.3 {
		t.Errorf("worked and failed are %.2f apart in contrast — tell them apart by lightness too", d)
	}
}
