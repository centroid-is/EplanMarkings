package parser

import (
	"sort"
	"testing"
)

func TestExtractStripDesignator(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"+U1-X13", "-X13"},
		{"+U1-X1", "-X1"},
		{"+U4.U4-X41", "-X41"},
		{"+U1-X100", "-X100"},
		{"+U7.U7-X71", "-X71"},
		{"+U2-X21", "-X21"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractStripDesignator(tt.input)
			if got != tt.want {
				t.Errorf("ExtractStripDesignator(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractLocation(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"=GD1+U1-K4", "+U1"},
		{"+U3-X31", "+U3"},
		{"=GD1+U3.U3-S31", "+U3.U3"},
		{"+U7.U7-X71", "+U7.U7"},
		{"+U1-X12", "+U1"},
		{"=GD1+U1-F12", "+U1"},
		{"=GD1+U1-B1.1", "+U1"},
		{"", ""},
		{"noplus", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractLocation(tt.input)
			if got != tt.want {
				t.Errorf("ExtractLocation(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractComponent(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"=GD1+U1-K4", "-K4"},
		{"+U3-X31", "-X31"},
		{"=GD1+U3.U3-S31", "-S31"},
		{"+U7.U7-X71", "-X71"},
		{"=GD1+U1-B1.1", "-B1.1"},
		{"=GD1+U1-F12", "-F12"},
		{"", ""},
		{"noplus", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractComponent(tt.input)
			if got != tt.want {
				t.Errorf("ExtractComponent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFilterLabels(t *testing.T) {
	sd := SheetData{
		Name:            "+U1-X13",
		StripDesignator: "-X13",
		Rows: []TerminalRow{
			{
				TargetDesignFrom: "=GD1+U1",
				ConnectionFrom:   "1",
				Terminal:         "0",
				TargetDesignTo:   "+U3-X31",
				ConnectionTo:     "0",
			},
			{
				TargetDesignFrom: "+U1-X12",
				ConnectionFrom:   "1",
				Terminal:         "1",
				TargetDesignTo:   "+U3-X31",
				ConnectionTo:     "1",
			},
			{
				TargetDesignFrom: "=GD1+U1-K4",
				ConnectionFrom:   "A:10",
				Terminal:         "3",
				TargetDesignTo:   "+U3-X31",
				ConnectionTo:     "3",
			},
			{
				// Row with no terminal - should be skipped
				TargetDesignFrom: "=GD1+U1-K4",
				ConnectionFrom:   "A:11",
				Terminal:         "",
				TargetDesignTo:   "+U3-X31",
				ConnectionTo:     "4",
			},
		},
	}

	t.Run("filter +U1", func(t *testing.T) {
		labels := FilterLabels(sd, "+U1")

		// Row 0: =GD1+U1 has location +U1 but no component (no dash after location) -> skip
		// Row 1: +U1-X12 -> -X12:1
		// Row 2: =GD1+U1-K4 -> -K4:A:10
		// Row 3: skipped (no terminal)

		if len(labels) != 2 {
			t.Fatalf("expected 2 labels, got %d: %+v", len(labels), labels)
		}
		if labels[0].TerminalSide != "-X13:1" || labels[0].ComponentSide != "-X12:1" {
			t.Errorf("label[0] = %+v, want -X13:1 / -X12:1", labels[0])
		}
		if labels[1].TerminalSide != "-X13:3" || labels[1].ComponentSide != "-K4:A:10" {
			t.Errorf("label[1] = %+v, want -X13:3 / -K4:A:10", labels[1])
		}
	})

	t.Run("filter +U3", func(t *testing.T) {
		labels := FilterLabels(sd, "+U3")

		// Rows 0,1,2 have +U3-X31 on the "to" side
		if len(labels) != 3 {
			t.Fatalf("expected 3 labels, got %d: %+v", len(labels), labels)
		}
		if labels[0].TerminalSide != "-X13:0" || labels[0].ComponentSide != "-X31:0" {
			t.Errorf("label[0] = %+v, want -X13:0 / -X31:0", labels[0])
		}
	})

	t.Run("filter non-existent location", func(t *testing.T) {
		labels := FilterLabels(sd, "+U99")
		if len(labels) != 0 {
			t.Errorf("expected 0 labels, got %d", len(labels))
		}
	})
}

func TestParseFile(t *testing.T) {
	sheets, err := ParseFile("../../Terminal.xlsx")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(sheets) != 17 {
		t.Errorf("expected 17 sheets, got %d", len(sheets))
	}

	// Find the +U1-X13 sheet
	var x13 *SheetData
	for i := range sheets {
		if sheets[i].Name == "+U1-X13" {
			x13 = &sheets[i]
			break
		}
	}
	if x13 == nil {
		t.Fatal("sheet +U1-X13 not found")
	}

	if x13.StripDesignator != "-X13" {
		t.Errorf("strip designator = %q, want %q", x13.StripDesignator, "-X13")
	}

	if len(x13.Rows) == 0 {
		t.Fatal("no rows parsed for +U1-X13")
	}

	// Check that locations include +U1 and +U3
	sort.Strings(x13.Locations)
	hasU1, hasU3 := false, false
	for _, loc := range x13.Locations {
		if loc == "+U1" {
			hasU1 = true
		}
		if loc == "+U3" {
			hasU3 = true
		}
	}
	if !hasU1 || !hasU3 {
		t.Errorf("expected locations to include +U1 and +U3, got %v", x13.Locations)
	}

	// Test filtering for +U1 produces labels
	labels := FilterLabels(*x13, "+U1")
	if len(labels) == 0 {
		t.Error("expected labels for +U1 in X13, got none")
	}

	// Verify a known label exists: -X13:3 / -K4:A:10
	found := false
	for _, l := range labels {
		if l.TerminalSide == "-X13:3" && l.ComponentSide == "-K4:A:10" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected label -X13:3 / -K4:A:10 in results, labels: %+v", labels)
	}
}
