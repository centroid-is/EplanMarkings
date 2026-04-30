package ui

import (
	"image"
	"image/color"
	"sort"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/centroid-is/print-wire-labels/internal/parser"
)

type configFieldInfo struct {
	label   string
	help    string
	editor  *widget.Editor
	infoBtn widget.Clickable
	showTip bool
}

type appState struct {
	sheets         []parser.SheetData
	selectedSheet  int
	selectedLocIdx int
	labels         []parser.WireLabel
	sheetClicks    []widget.Clickable
	locClicks      []widget.Clickable
	sheetList      widget.List
	locList        widget.List
	labelList      widget.List

	// Checkboxes for label selection
	labelChecks []widget.Bool
	selectAll   widget.Bool

	// Print
	printBtn    widget.Clickable
	configBtn   widget.Clickable
	showConfig  bool
	printConfig PrintConfig

	// Config sidebar
	cfgFields    []configFieldInfo
	cfgSaveBtn   widget.Clickable
	cfgCancelBtn widget.Clickable
	cfgList      widget.List
}

// PrintConfig holds Brother P-touch CLI arguments.
type PrintConfig struct {
	PtouchCmd string
	TapeWidth string
	FontSize  string
	Copies    string
}

func defaultPrintConfig() PrintConfig {
	return PrintConfig{
		PtouchCmd: "ptouch-print",
		TapeWidth: "12",
		FontSize:  "12",
		Copies:    "1",
	}
}

func (s *appState) initConfigFields() {
	s.cfgFields = []configFieldInfo{
		{
			label:  "Command",
			help:   "Path or name of the ptouch-print CLI executable. Install from github.com/philpem/printer-driver-ptouch",
			editor: &widget.Editor{SingleLine: true},
		},
		{
			label:  "Tape Width (mm)",
			help:   "Width of the label tape in millimeters. Common sizes: 6, 9, 12, 18, 24, 36",
			editor: &widget.Editor{SingleLine: true},
		},
		{
			label:  "Font Size",
			help:   "Font size in points for label text. Adjust based on tape width for readability",
			editor: &widget.Editor{SingleLine: true},
		},
		{
			label:  "Copies",
			help:   "Number of copies to print for each selected label",
			editor: &widget.Editor{SingleLine: true},
		},
	}
	s.cfgFields[0].editor.SetText(s.printConfig.PtouchCmd)
	s.cfgFields[1].editor.SetText(s.printConfig.TapeWidth)
	s.cfgFields[2].editor.SetText(s.printConfig.FontSize)
	s.cfgFields[3].editor.SetText(s.printConfig.Copies)
	s.cfgList.List.Axis = layout.Vertical
}

func Run(excelPath string) {
	sheets, err := parser.ParseFile(excelPath)
	if err != nil {
		panic(err)
	}

	for i := range sheets {
		sort.Strings(sheets[i].Locations)
	}

	state := &appState{
		sheets:         sheets,
		selectedSheet:  -1,
		selectedLocIdx: -1,
		sheetClicks:    make([]widget.Clickable, len(sheets)),
		printConfig:    defaultPrintConfig(),
	}
	state.sheetList.List.Axis = layout.Vertical
	state.locList.List.Axis = layout.Vertical
	state.labelList.List.Axis = layout.Vertical

	go func() {
		w := new(app.Window)
		w.Option(app.Title("PrintWireLabels"), app.Size(unit.Dp(1000), unit.Dp(600)))
		th := SolarizedDarkTheme()

		var ops op.Ops
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				return
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				fillBackground(gtx, th.Palette.Bg)
				layoutApp(gtx, th, state)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}

func fillBackground(gtx layout.Context, col color.NRGBA) {
	rect := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	c := clip.Rect(rect).Push(gtx.Ops)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	c.Pop()
}

func layoutApp(gtx layout.Context, th *material.Theme, state *appState) layout.Dimensions {
	// Handle sheet clicks
	for i := range state.sheetClicks {
		if state.sheetClicks[i].Clicked(gtx) {
			state.selectedSheet = i
			state.selectedLocIdx = -1
			state.labels = nil
			state.labelChecks = nil
			state.selectAll.Value = false
			state.locClicks = make([]widget.Clickable, len(state.sheets[i].Locations))
		}
	}

	// Handle location clicks
	for i := range state.locClicks {
		if state.locClicks[i].Clicked(gtx) {
			state.selectedLocIdx = i
			sd := state.sheets[state.selectedSheet]
			loc := sd.Locations[i]
			state.labels = parser.FilterLabels(sd, loc)
			state.labelChecks = make([]widget.Bool, len(state.labels))
			state.selectAll.Value = false
		}
	}

	// Handle select all
	if state.selectAll.Update(gtx) {
		for i := range state.labelChecks {
			state.labelChecks[i].Value = state.selectAll.Value
		}
	}

	// Handle print button
	if state.printBtn.Clicked(gtx) {
		printSelected(state)
	}

	// Handle config button
	if state.configBtn.Clicked(gtx) {
		state.showConfig = true
		state.initConfigFields()
	}

	// Main layout with optional config sidebar overlay
	mainDims := layout.Flex{}.Layout(gtx,
		layout.Flexed(0.20, func(gtx layout.Context) layout.Dimensions {
			return layoutSheetList(gtx, th, state)
		}),
		layout.Rigid(vertSeparator(gtx)),
		layout.Flexed(0.15, func(gtx layout.Context) layout.Dimensions {
			return layoutLocationList(gtx, th, state)
		}),
		layout.Rigid(vertSeparator(gtx)),
		layout.Flexed(0.65, func(gtx layout.Context) layout.Dimensions {
			return layoutLabelTable(gtx, th, state)
		}),
	)

	// Draw config sidebar overlay on right side
	if state.showConfig {
		layoutConfigSidebar(gtx, th, state)
	}

	return mainDims
}

func vertSeparator(gtx layout.Context) func(gtx layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		w := gtx.Dp(unit.Dp(1))
		h := gtx.Constraints.Max.Y
		rect := image.Rect(0, 0, w, h)
		c := clip.Rect(rect).Push(gtx.Ops)
		paint.ColorOp{Color: ColorMuted()}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		c.Pop()
		return layout.Dimensions{Size: image.Point{X: w, Y: h}}
	}
}

func horzSeparator(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(1))
	w := gtx.Constraints.Max.X
	rect := image.Rect(0, 0, w, h)
	c := clip.Rect(rect).Push(gtx.Ops)
	paint.ColorOp{Color: ColorMuted()}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	c.Pop()
	return layout.Dimensions{Size: image.Point{X: w, Y: h}}
}

func layoutSheetList(gtx layout.Context, th *material.Theme, state *appState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.H6(th, "Strips")
				lbl.Color = ColorHeader()
				return lbl.Layout(gtx)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &state.sheetList).Layout(gtx, len(state.sheets), func(gtx layout.Context, i int) layout.Dimensions {
				return layoutListItem(gtx, th, &state.sheetClicks[i], state.sheets[i].Name, i == state.selectedSheet)
			})
		}),
	)
}

func layoutLocationList(gtx layout.Context, th *material.Theme, state *appState) layout.Dimensions {
	if state.selectedSheet < 0 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, "Select a strip")
			lbl.Color = ColorMuted()
			return lbl.Layout(gtx)
		})
	}

	locs := state.sheets[state.selectedSheet].Locations
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.H6(th, "Locations")
				lbl.Color = ColorHeader()
				return lbl.Layout(gtx)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &state.locList).Layout(gtx, len(locs), func(gtx layout.Context, i int) layout.Dimensions {
				return layoutListItem(gtx, th, &state.locClicks[i], locs[i], i == state.selectedLocIdx)
			})
		}),
	)
}

func layoutListItem(gtx layout.Context, th *material.Theme, click *widget.Clickable, text string, selected bool) layout.Dimensions {
	if selected {
		rect := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Dp(unit.Dp(36)))
		c := clip.Rect(rect).Push(gtx.Ops)
		paint.ColorOp{Color: ColorSurface()}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		c.Pop()
	}

	return material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: unit.Dp(8), Bottom: unit.Dp(8),
			Left: unit.Dp(12), Right: unit.Dp(12),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, text)
			if selected {
				lbl.Color = ColorAccent()
			}
			return lbl.Layout(gtx)
		})
	})
}

func layoutLabelTable(gtx layout.Context, th *material.Theme, state *appState) layout.Dimensions {
	if state.selectedLocIdx < 0 {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, "Select a location to view labels")
			lbl.Color = ColorMuted()
			return lbl.Layout(gtx)
		})
	}

	loc := state.sheets[state.selectedSheet].Locations[state.selectedLocIdx]
	headerText := state.sheets[state.selectedSheet].Name + " / " + loc

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header row with title + buttons
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(4), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						lbl := material.H6(th, headerText)
						lbl.Color = ColorHeader()
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &state.printBtn, "Print")
						btn.Background = ColorAccent()
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &state.configBtn, "\xe2\x9a\x99") // gear ⚙
						btn.Background = ColorMuted()
						return btn.Layout(gtx)
					}),
				)
			})
		}),
		// Column headers with select all
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						cb := material.CheckBox(th, &state.selectAll, "")
						cb.Color = ColorAccent()
						cb.IconColor = ColorAccent()
						return cb.Layout(gtx)
					}),
					layout.Flexed(0.45, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body1(th, "Terminal")
						lbl.Color = ColorAccent()
						return lbl.Layout(gtx)
					}),
					layout.Flexed(0.55, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body1(th, "Component")
						lbl.Color = ColorAccent()
						return lbl.Layout(gtx)
					}),
				)
			})
		}),
		layout.Rigid(horzSeparator),
		// Label rows
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(state.labels) == 0 {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body1(th, "No labels for this location")
					lbl.Color = ColorMuted()
					return lbl.Layout(gtx)
				})
			}

			return material.List(th, &state.labelList).Layout(gtx, len(state.labels), func(gtx layout.Context, i int) layout.Dimensions {
				l := state.labels[i]
				return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							cb := material.CheckBox(th, &state.labelChecks[i], "")
							cb.Color = ColorAccent()
							cb.IconColor = ColorAccent()
							return cb.Layout(gtx)
						}),
						layout.Flexed(0.45, func(gtx layout.Context) layout.Dimensions {
							return material.Body1(th, l.TerminalSide).Layout(gtx)
						}),
						layout.Flexed(0.55, func(gtx layout.Context) layout.Dimensions {
							return material.Body1(th, l.ComponentSide).Layout(gtx)
						}),
					)
				})
			})
		}),
	)
}

// layoutConfigSidebar draws the config panel as a right-side overlay.
func layoutConfigSidebar(gtx layout.Context, th *material.Theme, state *appState) {
	sidebarWidth := gtx.Dp(unit.Dp(340))
	screenW := gtx.Constraints.Max.X
	screenH := gtx.Constraints.Max.Y
	offsetX := screenW - sidebarWidth

	// Handle save
	if state.cfgSaveBtn.Clicked(gtx) {
		state.printConfig.PtouchCmd = state.cfgFields[0].editor.Text()
		state.printConfig.TapeWidth = state.cfgFields[1].editor.Text()
		state.printConfig.FontSize = state.cfgFields[2].editor.Text()
		state.printConfig.Copies = state.cfgFields[3].editor.Text()
		state.showConfig = false
		return
	}
	if state.cfgCancelBtn.Clicked(gtx) {
		state.showConfig = false
		return
	}

	// Handle info button toggles
	for i := range state.cfgFields {
		if state.cfgFields[i].infoBtn.Clicked(gtx) {
			state.cfgFields[i].showTip = !state.cfgFields[i].showTip
		}
	}

	// Dim overlay behind sidebar
	dimRect := clip.Rect{Max: image.Pt(screenW, screenH)}.Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{A: 0x80}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	dimRect.Pop()

	// Sidebar background
	sidebarOff := op.Offset(image.Pt(offsetX, 0)).Push(gtx.Ops)
	sidebarRect := clip.Rect{Max: image.Pt(sidebarWidth, screenH)}.Push(gtx.Ops)
	paint.ColorOp{Color: solBase03}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	sidebarRect.Pop()

	// Left border
	borderRect := clip.Rect{Max: image.Pt(gtx.Dp(unit.Dp(2)), screenH)}.Push(gtx.Ops)
	paint.ColorOp{Color: ColorAccent()}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	borderRect.Pop()

	// Content inside sidebar
	sidebarGtx := gtx
	sidebarGtx.Constraints = layout.Exact(image.Pt(sidebarWidth, screenH))

	layout.Inset{Top: unit.Dp(16), Left: unit.Dp(16), Right: unit.Dp(16), Bottom: unit.Dp(16)}.Layout(sidebarGtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Title
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.H6(th, "P-touch Config")
				lbl.Color = ColorHeader()
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),

			// Config fields
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutConfigField(gtx, th, &state.cfgFields[0])
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutConfigField(gtx, th, &state.cfgFields[1])
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutConfigField(gtx, th, &state.cfgFields[2])
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutConfigField(gtx, th, &state.cfgFields[3])
			}),

			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),

			// Buttons
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &state.cfgSaveBtn, "Save")
						btn.Background = ColorAccent()
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &state.cfgCancelBtn, "Cancel")
						btn.Background = ColorMuted()
						return btn.Layout(gtx)
					}),
				)
			}),
		)
	})

	sidebarOff.Pop()
}

func layoutConfigField(gtx layout.Context, th *material.Theme, field *configFieldInfo) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Label row with info button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, field.label)
					lbl.Color = ColorMuted()
					return lbl.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, &field.infoBtn, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(2), Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(th, "\xe2\x93\x98") // ⓘ
							if field.showTip {
								lbl.Color = ColorAccent()
							} else {
								lbl.Color = ColorMuted()
							}
							return lbl.Layout(gtx)
						})
					})
				}),
			)
		}),
		// Tooltip text (shown when info clicked)
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !field.showTip {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, field.help)
				lbl.Color = ColorAccent()
				return lbl.Layout(gtx)
			})
		}),
		// Editor with background
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				// Background
				macro := op.Record(gtx.Ops)
				dims := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, field.editor, "")
					ed.Color = th.Palette.Fg
					return ed.Layout(gtx)
				})
				call := macro.Stop()

				rect := clip.UniformRRect(image.Rect(0, 0, dims.Size.X, dims.Size.Y), gtx.Dp(unit.Dp(4))).Push(gtx.Ops)
				paint.ColorOp{Color: ColorSurface()}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				rect.Pop()

				call.Add(gtx.Ops)
				return dims
			})
		}),
	)
}

func printSelected(state *appState) {
	var selected []parser.WireLabel
	for i, check := range state.labelChecks {
		if check.Value {
			selected = append(selected, state.labels[i])
		}
	}
	if len(selected) == 0 {
		return
	}
	go executePrint(selected, state.printConfig)
}
