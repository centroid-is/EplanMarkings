package ui

import (
	"image/color"

	"gioui.org/widget/material"
)

// Solarized color palette.
var (
	solBase03  = color.NRGBA{R: 0x00, G: 0x2b, B: 0x36, A: 0xff}
	solBase02  = color.NRGBA{R: 0x07, G: 0x36, B: 0x42, A: 0xff}
	solBase01  = color.NRGBA{R: 0x58, G: 0x6e, B: 0x75, A: 0xff}
	solBase0   = color.NRGBA{R: 0x83, G: 0x94, B: 0x96, A: 0xff}
	solBase1   = color.NRGBA{R: 0x93, G: 0xa1, B: 0xa1, A: 0xff}
	solCyan    = color.NRGBA{R: 0x2a, G: 0xa1, B: 0x98, A: 0xff}
	solBlue    = color.NRGBA{R: 0x26, G: 0x8b, B: 0xd2, A: 0xff}
	solRed     = color.NRGBA{R: 0xdc, G: 0x32, B: 0x2f, A: 0xff}
	solGreen   = color.NRGBA{R: 0x85, G: 0x99, B: 0x00, A: 0xff}
	solYellow  = color.NRGBA{R: 0xb5, G: 0x89, B: 0x00, A: 0xff}
	solOrange  = color.NRGBA{R: 0xcb, G: 0x4b, B: 0x16, A: 0xff}
	solMagenta = color.NRGBA{R: 0xd3, G: 0x36, B: 0x82, A: 0xff}
	solViolet  = color.NRGBA{R: 0x6c, G: 0x71, B: 0xc4, A: 0xff}
)

func SolarizedDarkTheme() *material.Theme {
	th := material.NewTheme()
	th.Palette.Bg = solBase03
	th.Palette.Fg = solBase0
	th.Palette.ContrastBg = solBlue
	th.Palette.ContrastFg = solBase03
	th.Face = "Go Mono, monospace"
	th.TextSize = 14
	return th
}

func ColorAccent() color.NRGBA  { return solCyan }
func ColorMuted() color.NRGBA   { return solBase01 }
func ColorSurface() color.NRGBA { return solBase02 }
func ColorError() color.NRGBA   { return solRed }
func ColorSuccess() color.NRGBA { return solGreen }
func ColorHeader() color.NRGBA  { return solYellow }
