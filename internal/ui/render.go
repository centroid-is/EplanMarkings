package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// printDPI is the THM MultiMark print resolution. The PNG that M-Print PRO
// downloads is rendered at this density (e.g. an 89.00 x 10.75 mm label is
// 1051 x 127 px).
const printDPI = 300.0

// Horizontal centres of the two text fields, as a fraction of label width.
// Measured from an M-Print PRO reference print (fields at px 383 and 677 of a
// 1051 px-wide wire-marker image).
const (
	fieldCenterTerminal  = 0.365
	fieldCenterComponent = 0.645
)

func mmToPx(mmVal float64) int {
	return int(mmVal/25.4*printDPI + 0.5)
}

// renderLabel rasterises a wire label into the 1-bit black-on-white bitmap the
// printer expects, mirroring M-Print PRO: terminal side and component side as
// two horizontally-centred Tahoma fields, vertically centred. It returns the
// image (for the on-screen preview) and its PNG encoding (for the printer).
func renderLabel(termSide, compSide string, roll MarkerRoll, cfg PrintConfig) (image.Image, []byte, error) {
	w := mmToPx(roll.WidthMM)
	h := mmToPx(roll.HeightMM)
	if w <= 0 || h <= 0 {
		return nil, nil, fmt.Errorf("invalid label size %gx%g mm", roll.WidthMM, roll.HeightMM)
	}

	sizePt := atofDefault(cfg.FontSize, 10)
	face, err := loadFontFace(cfg.FontName, sizePt)
	if err != nil {
		return nil, nil, err
	}
	defer face.Close()

	// 1-bit palette: index 0 = white (background), index 1 = black (ink).
	img := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.White, color.Black})

	drawCentered(img, face, termSide, fieldCenterTerminal*float64(w))
	drawCentered(img, face, compSide, fieldCenterComponent*float64(w))

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, nil, err
	}
	return img, buf.Bytes(), nil
}

// drawCentered draws text centred horizontally on centerX and vertically within
// the image.
func drawCentered(img *image.Paletted, face font.Face, text string, centerX float64) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.Black), Face: face}
	textW := d.MeasureString(text).Round()
	m := face.Metrics()
	ascent, descent := m.Ascent.Round(), m.Descent.Round()
	baseline := (img.Bounds().Dy()-(ascent+descent))/2 + ascent
	d.Dot = fixed.P(int(centerX)-textW/2, baseline)
	d.DrawString(text)
}

// loadFontFace loads a TrueType font by family name (or absolute path) at the
// given point size and the print DPI.
func loadFontFace(name string, sizePt float64) (font.Face, error) {
	path, err := fontPath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading font %s: %w", path, err)
	}
	ft, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing font %s: %w", path, err)
	}
	return opentype.NewFace(ft, &opentype.FaceOptions{
		Size:    sizePt,
		DPI:     printDPI,
		Hinting: font.HintingFull,
	})
}

// fontPath resolves a font family name to a file in the OS font directory.
func fontPath(name string) (string, error) {
	if name == "" {
		name = "Tahoma"
	}
	if filepath.IsAbs(name) {
		return name, nil
	}
	base := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	for _, dir := range fontDirs() {
		for _, ext := range []string{".ttf", ".TTF", ".otf", ".OTF"} {
			p := filepath.Join(dir, base+ext)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("font %q not found in %v", name, fontDirs())
}

func fontDirs() []string {
	switch runtime.GOOS {
	case "windows":
		win := os.Getenv("WINDIR")
		if win == "" {
			win = `C:\Windows`
		}
		return []string{filepath.Join(win, "Fonts")}
	case "darwin":
		return []string{"/Library/Fonts", "/System/Library/Fonts"}
	default:
		return []string{"/usr/share/fonts", "/usr/local/share/fonts", "/usr/share/fonts/truetype"}
	}
}

func atofDefault(s string, def float64) float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil && v > 0 {
		return v
	}
	return def
}
