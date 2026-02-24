package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const iconSize = 128

var boldFace xfont.Face

func init() {
	tt, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return
	}
	boldFace, _ = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    52,
		DPI:     72,
		Hinting: xfont.HintingFull,
	})
}

type palette struct {
	inner, outer color.RGBA
}

func paletteForLabel(label string) palette {
	lower := strings.ToLower(label)
	switch {
	case lower == "blocked" || lower == "blo":
		return palette{
			inner: color.RGBA{230, 60, 60, 255},
			outer: color.RGBA{45, 20, 20, 255},
		}
	case lower == "" || lower == "---" || lower == "default" || lower == "def":
		return palette{
			inner: color.RGBA{80, 170, 255, 255},
			outer: color.RGBA{30, 30, 35, 255},
		}
	default:
		return palette{
			inner: color.RGBA{110, 220, 90, 255},
			outer: color.RGBA{20, 35, 20, 255},
		}
	}
}

func drawFilledCircle(img *image.RGBA, cx, cy int, r float64, c color.RGBA) {
	r2 := r * r
	minX := int(math.Floor(float64(cx) - r))
	maxX := int(math.Ceil(float64(cx) + r))
	minY := int(math.Floor(float64(cy) - r))
	maxY := int(math.Ceil(float64(cy) + r))
	bounds := img.Bounds()
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if !image.Pt(x, y).In(bounds) {
				continue
			}
			dx := float64(x-cx) + 0.5
			dy := float64(y-cy) + 0.5
			if dx*dx+dy*dy <= r2 {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawTextCentered(img *image.RGBA, label string, textColor color.RGBA) {
	if boldFace == nil || label == "" || label == "---" {
		return
	}
	d := &xfont.Drawer{
		Dst:  img,
		Src:  image.NewUniform(textColor),
		Face: boldFace,
	}
	w := d.MeasureString(label).Round()
	metrics := boldFace.Metrics()
	ascent := metrics.Ascent.Round()
	descent := metrics.Descent.Round()
	h := ascent + descent
	x := (iconSize - w) / 2
	y := (iconSize+h)/2 - descent
	d.Dot = fixed.P(x, y)
	d.DrawString(label)
}

// GenerateIcon creates a dynamic tray icon with the given label and returns a fyne.Resource.
func GenerateIcon(label string) fyne.Resource {
	img := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))

	pal := paletteForLabel(label)
	cx, cy := iconSize/2, iconSize/2

	drawFilledCircle(img, cx, cy, float64(cx)-6, pal.outer)
	drawFilledCircle(img, cx, cy, float64(cx)-20, pal.inner)

	if label != "" && label != "---" {
		drawTextCentered(img, label, color.RGBA{255, 255, 255, 255})
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return fyne.NewStaticResource("tray_icon.png", buf.Bytes())
}

// PolicyLabel returns the human-readable label for a policy.
func PolicyLabel(policy string, policies map[string]interface{}, deny bool) string {
	if deny {
		return "Blocked"
	}
	if policy == "" {
		return "Default"
	}
	if info, ok := policies[policy]; ok {
		if m, ok := info.(map[string]interface{}); ok {
			if desc, ok := m["description"].(string); ok && desc != "" {
				return desc
			}
		}
	}
	return policy
}

// PolicyShort returns the 3-character abbreviation for display in the tray icon.
func PolicyShort(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "---"
	}
	runes := []rune(label)
	if len(runes) >= 3 {
		return string(runes[:3])
	}
	return label
}
