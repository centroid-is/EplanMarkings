package ui

import (
	"os"
	"testing"

	"github.com/centroid-is/print-wire-labels/internal/parser"
)

// TestLivePrint sends one real wire-marks label to the configured printer over
// TCP. It is gated behind LIVE_PRINT=1 so it never runs in normal test runs.
// The label matches the M-Print PRO reference print for side-by-side comparison.
func TestLivePrint(t *testing.T) {
	if os.Getenv("LIVE_PRINT") == "" {
		t.Skip("set LIVE_PRINT=1 to send a real print job")
	}
	cfg := defaultPrintConfig()
	n, err := executePrint([]parser.WireLabel{
		{TerminalSide: "-X11:10", ComponentSide: "-KF3:A:13"},
	}, cfg)
	if err != nil {
		t.Fatalf("live print failed after %d: %v", n, err)
	}
	t.Logf("sent %d label(s) to %s", n, cfg.PrinterHost)
}
