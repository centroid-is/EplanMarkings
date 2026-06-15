package ui

import "fmt"

// MarkerRoll describes a Weidmüller marker material that can be loaded in the
// printer. Geometry feeds both the cab JScript "S" (size/format) command and
// the on-screen preview. Dimensions are in millimetres.
//
// TODO(capture): the geometry values below are provisional. Confirm them from a
// packet capture of an M-Print PRO print for each article, then correct here.
type MarkerRoll struct {
	Name     string
	Article  string
	WidthMM  float64
	HeightMM float64
	GapMM    float64
}

// MarkerRolls is the fixed set of marker materials used in the cabinet.
//
// Wire-marks geometry (89.00 x 10.75 mm) is confirmed from an M-Print PRO
// reference print. Cable and terminal geometry are provisional placeholders —
// capture an M-Print print of each and correct WidthMM/HeightMM here.
var MarkerRolls = []MarkerRoll{
	{Name: "Wire marks", Article: "2437550000", WidthMM: 89.00, HeightMM: 10.75, GapMM: 0},
	{Name: "Cable marks", Article: "2005400000", WidthMM: 30, HeightMM: 8, GapMM: 0},
	{Name: "Terminal marks", Article: "2007110000", WidthMM: 5, HeightMM: 6, GapMM: 0},
}

func rollByIndex(i int) MarkerRoll {
	if i < 0 || i >= len(MarkerRolls) {
		return MarkerRolls[0]
	}
	return MarkerRolls[i]
}

// Label is the human-readable chip text, e.g. "Wire marks (2437550000)".
func (r MarkerRoll) Label() string {
	return fmt.Sprintf("%s (%s)", r.Name, r.Article)
}
