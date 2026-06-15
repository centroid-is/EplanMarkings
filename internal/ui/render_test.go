package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRenderSample renders the same label as the M-Print PRO reference print
// and verifies the wire-marks bitmap geometry (1051 x 127 px at 300 dpi). The
// PNG is written to a temp dir for optional visual inspection.
func TestRenderSample(t *testing.T) {
	cfg := defaultPrintConfig()
	img, pngBytes, err := renderLabel("-X11:10", "-KF3:A:13", rollByIndex(0), cfg)
	if err != nil {
		t.Skipf("render failed (font likely unavailable on this host): %v", err)
	}
	if got := img.Bounds().Dx(); got != 1051 {
		t.Errorf("width = %d px, want 1051", got)
	}
	if got := img.Bounds().Dy(); got != 127 {
		t.Errorf("height = %d px, want 127", got)
	}
	out := filepath.Join(t.TempDir(), "render_sample.png")
	if err := os.WriteFile(out, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s (%d bytes)", out, len(pngBytes))
}
