package ui

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"testing"
)

// referencePRN is an M-Print PRO print stream for the wire-marks roll, used as
// the ground truth for the cab envelope and label rendering.
const referencePRN = "../../HS 1.6-4.825 MM W.prn"

// TestRenderSample renders the same label as the reference print and verifies
// the wire-marks bitmap geometry (1051 x 127 px at 300 dpi).
func TestRenderSample(t *testing.T) {
	cfg := defaultPrintConfig()
	img, _, err := renderLabel("-X11:10", "-KF3:A:13", rollByIndex(0), cfg)
	if err != nil {
		t.Skipf("render failed (font likely unavailable on this host): %v", err)
	}
	if got := img.Bounds().Dx(); got != 1051 {
		t.Errorf("width = %d px, want 1051", got)
	}
	if got := img.Bounds().Dy(); got != 127 {
		t.Errorf("height = %d px, want 127", got)
	}
}

// TestEnvelopeMatchesReference feeds the reference print's own PNG back through
// wrapCabJob and requires the result to equal the .prn byte-for-byte, proving
// the cab download/print envelope we emit is exactly what M-Print PRO sends.
func TestEnvelopeMatchesReference(t *testing.T) {
	raw, err := os.ReadFile(referencePRN)
	if err != nil {
		t.Skipf("reference print not available: %v", err)
	}
	refPNG, ok := extractReferencePNG(raw)
	if !ok {
		t.Fatal("could not locate PNG payload in reference print")
	}

	got := wrapCabJob(refPNG, rollByIndex(0), "1")
	if !bytes.Equal(got, raw) {
		t.Errorf("envelope mismatch: produced %d bytes, reference is %d bytes", len(got), len(raw))
	}
}

// TestRenderMatchesReferenceLayout requires our rendered bitmap to match the
// reference PNG's dimensions and the horizontal centres of the two text fields.
func TestRenderMatchesReferenceLayout(t *testing.T) {
	raw, err := os.ReadFile(referencePRN)
	if err != nil {
		t.Skipf("reference print not available: %v", err)
	}
	refPNG, ok := extractReferencePNG(raw)
	if !ok {
		t.Fatal("could not locate PNG payload in reference print")
	}
	refImg, err := png.Decode(bytes.NewReader(refPNG))
	if err != nil {
		t.Fatalf("decoding reference PNG: %v", err)
	}

	img, _, err := renderLabel("-X11:10", "-KF3:A:13", rollByIndex(0), defaultPrintConfig())
	if err != nil {
		t.Skipf("render failed (font likely unavailable on this host): %v", err)
	}

	if img.Bounds().Size() != refImg.Bounds().Size() {
		t.Fatalf("size = %v, reference = %v", img.Bounds().Size(), refImg.Bounds().Size())
	}

	refGroups := darkColumnGroups(refImg, 25)
	myGroups := darkColumnGroups(img, 25)
	if len(refGroups) != 2 || len(myGroups) != 2 {
		t.Fatalf("expected 2 text fields; reference=%v mine=%v", refGroups, myGroups)
	}
	for i := 0; i < 2; i++ {
		refC := (refGroups[i][0] + refGroups[i][1]) / 2
		myC := (myGroups[i][0] + myGroups[i][1]) / 2
		if d := refC - myC; d < -30 || d > 30 {
			t.Errorf("field %d centre off by %d px (reference %d, mine %d)", i, refC-myC, refC, myC)
		}
	}
}

// extractReferencePNG pulls the PNG payload out of a cab print stream, which is
// framed as "<ESC>:" … PNG … "<ESC>end-of-data".
func extractReferencePNG(raw []byte) ([]byte, bool) {
	start := bytes.Index(raw, []byte("\x1b:"))
	end := bytes.Index(raw, []byte("\x1bend-of-data"))
	if start < 0 || end < 0 || end <= start+2 {
		return nil, false
	}
	return raw[start+2 : end], true
}

// darkColumnGroups returns [start,end] column ranges containing ink, splitting
// on horizontal gaps of at least gap pixels (i.e. one range per text field).
func darkColumnGroups(img image.Image, gap int) [][2]int {
	b := img.Bounds()
	dark := make([]bool, b.Dx())
	for x := 0; x < b.Dx(); x++ {
		for y := 0; y < b.Dy(); y++ {
			if r, _, _, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA(); r < 0x8000 {
				dark[x] = true
				break
			}
		}
	}

	var groups [][2]int
	inRun := false
	start, last := 0, 0
	for x := 0; x < len(dark); x++ {
		if dark[x] {
			if !inRun {
				start, inRun = x, true
			}
			last = x
		} else if inRun && x-last >= gap {
			groups = append(groups, [2]int{start, last})
			inRun = false
		}
	}
	if inRun {
		groups = append(groups, [2]int{start, last})
	}
	return groups
}
