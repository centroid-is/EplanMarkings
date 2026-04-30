package ui

import (
	"fmt"
	"os/exec"

	"github.com/centroid-is/print-wire-labels/internal/parser"
)

func executePrint(labels []parser.WireLabel, cfg PrintConfig) {
	for _, l := range labels {
		text := l.TerminalSide + "  " + l.ComponentSide
		args := []string{
			"--tape-width", cfg.TapeWidth,
			"--font-size", cfg.FontSize,
			"--copies", cfg.Copies,
			"--text", text,
		}

		cmd := exec.Command(cfg.PtouchCmd, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("print error for %q: %v\n%s\n", text, err, out)
		} else {
			fmt.Printf("printed: %s\n", text)
		}
	}
}
