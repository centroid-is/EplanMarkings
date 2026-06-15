package ui

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/centroid-is/print-wire-labels/internal/parser"
)

// executePrint renders each selected label to a 1-bit PNG and sends it to the
// networked THM MultiMark printer wrapped in the cab image-download/print
// envelope, over a raw TCP socket. This mirrors what M-Print PRO emits: the
// label text is rasterised on the PC side (so the font is whatever we render,
// e.g. Tahoma) and downloaded as a bitmap, rather than sent as font commands.
func executePrint(labels []parser.WireLabel, cfg PrintConfig) {
	addr := net.JoinHostPort(cfg.PrinterHost, cfg.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Printf("print: cannot connect to %s: %v\n", addr, err)
		return
	}
	defer conn.Close()

	roll := rollByIndex(cfg.RollIndex)
	for _, l := range labels {
		_, png, err := renderLabel(l.TerminalSide, l.ComponentSide, roll, cfg)
		if err != nil {
			fmt.Printf("print: render %q failed: %v\n", l.TerminalSide+" "+l.ComponentSide, err)
			return
		}
		if _, err := conn.Write(wrapCabJob(png, roll, cfg.Copies)); err != nil {
			fmt.Printf("print: send to %s failed: %v\n", addr, err)
			return
		}
	}
	fmt.Printf("print: sent %d label(s) to %s\n", len(labels), addr)
}

// wrapCabJob frames a rendered PNG in the cab JScript download/print sequence
// captured from an M-Print PRO print:
//
//	eIMG;*                      erase stored images
//	dPNG;0 <ESC>: …png… <ESC>end-of-data   download the bitmap
//	mm / zO / J                 mm units, new label
//	H40,+15,T,B30               heating / speed
//	O T,P
//	Sl1;0.0,0.00,H,H,W           label size (height, width in mm)
//	I:Field1;0.00,0.00,0;0       place the image at the origin
//	A<copies>                    print
func wrapCabJob(png []byte, roll MarkerRoll, copies string) []byte {
	if copies == "" {
		copies = "1"
	}
	var b bytes.Buffer
	b.WriteString("eIMG;*\r\n")
	b.WriteString("dPNG;0\r\n")
	b.WriteString("\x1b:")
	b.Write(png)
	b.WriteString("\x1bend-of-data\r\n")
	b.WriteString("mm\r\n")
	b.WriteString("zO\r\n")
	b.WriteString("J\r\n")
	b.WriteString("H40,+15,T,B30\r\n")
	b.WriteString("O T,P\r\n")
	fmt.Fprintf(&b, "Sl1;0.0,0.00,%.2f,%.2f,%.2f\r\n", roll.HeightMM, roll.HeightMM, roll.WidthMM)
	b.WriteString("I:Field1;0.00,0.00,0;0\r\n")
	fmt.Fprintf(&b, "A%s\r\n", copies)
	return b.Bytes()
}
