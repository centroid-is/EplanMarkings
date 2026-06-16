package parser

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// TerminalRow represents a single row from a terminal strip tab.
type TerminalRow struct {
	TargetDesignFrom string
	ConnectionFrom   string
	Terminal         string
	TargetDesignTo   string
	ConnectionTo     string
}

// WireLabel represents a printable wire label with both ends.
type WireLabel struct {
	TerminalSide  string // e.g. "-X13:3"
	ComponentSide string // e.g. "-K4:A:10"
}

// SheetData holds parsed data for one terminal strip tab.
type SheetData struct {
	Name             string
	StripDesignator  string // e.g. "-X13"
	Rows             []TerminalRow
	Locations        []string // unique location designators
}

// ParseFile opens an Excel file by path and parses all terminal strip tabs.
func ParseFile(path string) ([]SheetData, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("opening excel file: %w", err)
	}
	defer f.Close()
	return parseSheets(f)
}

// ParseReader parses an Excel workbook from a reader (e.g. a file chosen via a
// dialog) and parses all terminal strip tabs.
func ParseReader(r io.Reader) ([]SheetData, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("reading excel: %w", err)
	}
	defer f.Close()
	return parseSheets(f)
}

func parseSheets(f *excelize.File) ([]SheetData, error) {
	var sheets []SheetData
	for _, name := range f.GetSheetList() {
		sd, err := parseSheet(f, name)
		if err != nil {
			return nil, fmt.Errorf("parsing sheet %q: %w", name, err)
		}
		sheets = append(sheets, sd)
	}
	return sheets, nil
}

func parseSheet(f *excelize.File, name string) (SheetData, error) {
	sd := SheetData{
		Name:            name,
		StripDesignator: ExtractStripDesignator(name),
	}

	rows, err := f.GetRows(name)
	if err != nil {
		return sd, err
	}

	// Data starts at row 8 (index 7), header is row 7 (index 6)
	locationSet := make(map[string]bool)
	for i := 7; i < len(rows); i++ {
		row := rows[i]
		tr := TerminalRow{}
		if len(row) > 2 {
			tr.TargetDesignFrom = strings.TrimSpace(row[2])
		}
		if len(row) > 3 {
			tr.ConnectionFrom = strings.TrimSpace(row[3])
		}
		if len(row) > 4 {
			tr.Terminal = strings.TrimSpace(row[4])
		}
		if len(row) > 5 {
			tr.TargetDesignTo = strings.TrimSpace(row[5])
		}
		if len(row) > 6 {
			tr.ConnectionTo = strings.TrimSpace(row[6])
		}

		// Skip completely empty rows
		if tr.Terminal == "" && tr.TargetDesignFrom == "" && tr.TargetDesignTo == "" {
			continue
		}

		sd.Rows = append(sd.Rows, tr)

		if loc := ExtractLocation(tr.TargetDesignFrom); loc != "" {
			locationSet[loc] = true
		}
		if loc := ExtractLocation(tr.TargetDesignTo); loc != "" {
			locationSet[loc] = true
		}
	}

	for loc := range locationSet {
		sd.Locations = append(sd.Locations, loc)
	}
	return sd, nil
}

// ExtractStripDesignator extracts the strip part from a tab name.
// e.g. "+U1-X13" -> "-X13", "+U4.U4-X41" -> "-X41"
func ExtractStripDesignator(tabName string) string {
	idx := strings.LastIndex(tabName, "-")
	if idx < 0 {
		return tabName
	}
	return tabName[idx:]
}

// ExtractLocation extracts the location designator (+ prefixed) from a target designation.
// e.g. "=GD1+U1-K4" -> "+U1", "+U3.U3-S31" -> "+U3.U3", "+U1-X12" -> "+U1"
func ExtractLocation(designation string) string {
	plusIdx := strings.Index(designation, "+")
	if plusIdx < 0 {
		return ""
	}
	rest := designation[plusIdx:]
	dashIdx := strings.Index(rest, "-")
	if dashIdx < 0 {
		return rest
	}
	return rest[:dashIdx]
}

// ExtractComponent extracts the component designator (- prefixed after location) from a target designation.
// e.g. "=GD1+U1-K4" -> "-K4", "+U3.U3-S31" -> "-S31"
func ExtractComponent(designation string) string {
	plusIdx := strings.Index(designation, "+")
	if plusIdx < 0 {
		// No location prefix, look for component directly
		dashIdx := strings.Index(designation, "-")
		if dashIdx < 0 {
			return ""
		}
		return designation[dashIdx:]
	}
	rest := designation[plusIdx:]
	dashIdx := strings.Index(rest, "-")
	if dashIdx < 0 {
		return ""
	}
	return rest[dashIdx:]
}

// FilterLabels generates wire labels for a given location within a sheet.
func FilterLabels(sd SheetData, location string) []WireLabel {
	var labels []WireLabel

	for _, row := range sd.Rows {
		if row.Terminal == "" {
			continue
		}

		termSide := sd.StripDesignator + ":" + row.Terminal

		// Check "from" side for matching location
		if loc := ExtractLocation(row.TargetDesignFrom); loc == location {
			comp := ExtractComponent(row.TargetDesignFrom)
			if comp != "" {
				connPart := row.ConnectionFrom
				compSide := comp
				if connPart != "" {
					compSide = comp + ":" + connPart
				}
				labels = append(labels, WireLabel{
					TerminalSide:  termSide,
					ComponentSide: compSide,
				})
				continue
			}
		}

		// Check "to" side for matching location
		if loc := ExtractLocation(row.TargetDesignTo); loc == location {
			comp := ExtractComponent(row.TargetDesignTo)
			if comp != "" {
				connPart := row.ConnectionTo
				compSide := comp
				if connPart != "" {
					compSide = comp + ":" + connPart
				}
				labels = append(labels, WireLabel{
					TerminalSide:  termSide,
					ComponentSide: compSide,
				})
			}
		}
	}

	sort.Slice(labels, func(i, j int) bool {
		return terminalNum(labels[i].TerminalSide) < terminalNum(labels[j].TerminalSide)
	})

	return labels
}

// terminalNum extracts the numeric terminal number from a terminal side string for sorting.
// e.g. "-X13:3" -> 3, "-X13:10" -> 10
func terminalNum(termSide string) int {
	idx := strings.LastIndex(termSide, ":")
	if idx < 0 {
		return 0
	}
	n, _ := strconv.Atoi(termSide[idx+1:])
	return n
}
