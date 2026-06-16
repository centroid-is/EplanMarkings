package ui

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"sort"
	"sync"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"

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
	rollClicks  []widget.Clickable

	// Label preview (cached; rebuilt when inputs change)
	previewOp    paint.ImageOp
	previewKey   string
	previewErr   error
	previewValid bool

	// Config sidebar
	cfgFields    []configFieldInfo
	cfgSaveBtn   widget.Clickable
	cfgCancelBtn widget.Clickable
	cfgList      widget.List

	// File loading
	openBtn     widget.Clickable
	currentFile string
	expl        *explorer.Explorer

	// Print status (written by the print goroutine, read in the frame loop)
	win         *app.Window
	mu          sync.Mutex
	printStatus string
	printLevel  statusLevel

	// Pending file load + load banner (mu-guarded; applied in the frame loop)
	pendingReady  bool
	pendingSheets []parser.SheetData
	pendingName   string
	pendingErr    error
	loadMsg       string
	loadLevel     statusLevel
}

func (s *appState) setLoadResult(sheets []parser.SheetData, name string, err error) {
	s.mu.Lock()
	s.pendingSheets = sheets
	s.pendingName = name
	s.pendingErr = err
	s.pendingReady = true
	w := s.win
	s.mu.Unlock()
	if w != nil {
		w.Invalidate()
	}
}

func (s *appState) getLoadBanner() (statusLevel, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLevel, s.loadMsg
}

// applyPendingLoad applies a completed file load. Called from the frame loop so
// it is safe to mutate navigation state here.
func (s *appState) applyPendingLoad() {
	s.mu.Lock()
	if !s.pendingReady {
		s.mu.Unlock()
		return
	}
	sheets, name, err := s.pendingSheets, s.pendingName, s.pendingErr
	s.pendingReady = false
	s.pendingSheets = nil
	if err != nil {
		s.loadLevel, s.loadMsg = statusError, "Load failed: "+err.Error()
		s.mu.Unlock()
		return
	}
	s.loadLevel, s.loadMsg = statusSuccess, fmt.Sprintf("Loaded %d sheet(s) from %s", len(sheets), name)
	s.mu.Unlock()

	s.applySheets(sheets, name)
}

// applySheets installs parsed sheets and resets navigation/selection state.
func (s *appState) applySheets(sheets []parser.SheetData, name string) {
	for i := range sheets {
		sort.Strings(sheets[i].Locations)
	}
	s.sheets = sheets
	s.currentFile = name
	s.selectedSheet = -1
	s.selectedLocIdx = -1
	s.labels = nil
	s.labelChecks = nil
	s.locClicks = nil
	s.selectAll.Value = false
	s.sheetClicks = make([]widget.Clickable, len(sheets))
}

type statusLevel int

const (
	statusNone statusLevel = iota
	statusInfo
	statusSuccess
	statusError
)

func (s *appState) setPrintStatus(level statusLevel, msg string) {
	s.mu.Lock()
	s.printLevel = level
	s.printStatus = msg
	w := s.win
	s.mu.Unlock()
	if w != nil {
		w.Invalidate()
	}
}

func (s *appState) getPrintStatus() (statusLevel, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.printLevel, s.printStatus
}

// PrintConfig holds network printer settings for the THM MultiMark (cab
// JScript over a raw TCP socket).
type PrintConfig struct {
	PrinterHost string // printer IP/hostname, e.g. "10.50.10.198"
	Port        string // JScript raw socket port, default "9100"
	FontName    string // label font, e.g. "Tahoma"
	FontSize    string // font height in points for label text
	Copies      string // copies printed per selected label
	RollIndex   int    // selected MarkerRolls entry (loaded marker material)
}

func defaultPrintConfig() PrintConfig {
	return PrintConfig{
		PrinterHost: "10.50.10.198",
		Port:        "9100",
		FontName:    "Tahoma",
		FontSize:    "10",
		Copies:      "1",
		RollIndex:   0,
	}
}

func (s *appState) initConfigFields() {
	s.cfgFields = []configFieldInfo{
		{
			label:  "Printer IP",
			help:   "IP address or hostname of the THM MultiMark printer on the network (e.g. 10.50.10.198)",
			editor: &widget.Editor{SingleLine: true},
		},
		{
			label:  "Port",
			help:   "Raw TCP socket port for the printer. Default is 9100",
			editor: &widget.Editor{SingleLine: true},
		},
		{
			label:  "Font",
			help:   "Font family for label text, e.g. Tahoma. Must be installed in the OS font folder",
			editor: &widget.Editor{SingleLine: true},
		},
		{
			label:  "Font Size (pt)",
			help:   "Font size in points for label text. Rendered at 300 dpi to match M-Print PRO",
			editor: &widget.Editor{SingleLine: true},
		},
		{
			label:  "Copies",
			help:   "Number of copies to print for each selected label",
			editor: &widget.Editor{SingleLine: true},
		},
	}
	s.cfgFields[0].editor.SetText(s.printConfig.PrinterHost)
	s.cfgFields[1].editor.SetText(s.printConfig.Port)
	s.cfgFields[2].editor.SetText(s.printConfig.FontName)
	s.cfgFields[3].editor.SetText(s.printConfig.FontSize)
	s.cfgFields[4].editor.SetText(s.printConfig.Copies)
	s.cfgList.List.Axis = layout.Vertical
}

func Run(excelPath string) {
	state := &appState{
		selectedSheet:  -1,
		selectedLocIdx: -1,
		rollClicks:     make([]widget.Clickable, len(MarkerRolls)),
		printConfig:    defaultPrintConfig(),
	}
	state.sheetList.List.Axis = layout.Vertical
	state.locList.List.Axis = layout.Vertical
	state.labelList.List.Axis = layout.Vertical

	// Try to load the default file, but don't fail if it's missing — the user
	// can open one from the UI.
	if sheets, err := parser.ParseFile(excelPath); err == nil {
		state.applySheets(sheets, filepath.Base(excelPath))
		state.loadLevel, state.loadMsg = statusSuccess, fmt.Sprintf("Loaded %d sheet(s) from %s", len(sheets), filepath.Base(excelPath))
	} else {
		state.loadLevel, state.loadMsg = statusInfo, `No file loaded — click "Open Excel…"`
	}

	go func() {
		w := new(app.Window)
		w.Option(app.Title("PrintWireLabels"), app.Size(unit.Dp(1000), unit.Dp(600)))
		state.mu.Lock()
		state.win = w
		state.mu.Unlock()
		state.expl = explorer.NewExplorer(w)
		th := SolarizedDarkTheme()

		var ops op.Ops
		for {
			evt := w.Event()
			state.expl.ListenEvents(evt)
			switch e := evt.(type) {
			case app.DestroyEvent:
				return
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				state.applyPendingLoad()
				fillBackground(gtx, th.Palette.Bg)
				layoutApp(gtx, th, state)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}

// openExcelDialog runs the native file picker on a background goroutine and
// hands the parsed result back to the frame loop.
func (s *appState) openExcelDialog() {
	expl := s.expl
	if expl == nil {
		return
	}
	go func() {
		r, err := expl.ChooseFile(".xlsx")
		if err != nil {
			if errors.Is(err, explorer.ErrUserDecline) {
				return
			}
			s.setLoadResult(nil, "", err)
			return
		}
		defer r.Close()

		name := "selected file"
		if f, ok := r.(interface{ Name() string }); ok {
			name = filepath.Base(f.Name())
		}
		sheets, perr := parser.ParseReader(r)
		if perr != nil {
			s.setLoadResult(nil, "", perr)
			return
		}
		s.setLoadResult(sheets, name, nil)
	}()
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

	// Handle open-file button
	if state.openBtn.Clicked(gtx) {
		state.openExcelDialog()
	}

	// Handle roll selection
	for i := range state.rollClicks {
		if state.rollClicks[i].Clicked(gtx) {
			state.printConfig.RollIndex = i
		}
	}

	// Refresh the cached label preview if any input changed
	state.updatePreview()

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
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(4), Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.H6(th, "Strips")
				lbl.Color = ColorHeader()
				return lbl.Layout(gtx)
			})
		}),
		// Open Excel button
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &state.openBtn, "Open Excel…")
				btn.Background = ColorSurface()
				btn.Color = th.Palette.Fg
				return btn.Layout(gtx)
			})
		}),
		// Current file / load status
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			level, msg := state.getLoadBanner()
			if msg == "" {
				return layout.Dimensions{}
			}
			col := ColorMuted()
			switch level {
			case statusSuccess:
				col = ColorMuted()
			case statusError:
				col = ColorError()
			case statusInfo:
				col = ColorAccent()
			}
			return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(6), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, msg)
				lbl.Color = col
				return lbl.Layout(gtx)
			})
		}),
		layout.Rigid(horzSeparator),
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
		// Print status line
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutPrintStatus(gtx, th, state)
		}),
		// Roll selector
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutRollBar(gtx, th, state)
		}),
		// Label preview
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutPreview(gtx, th, state)
		}),
		layout.Rigid(horzSeparator),
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
		state.printConfig.PrinterHost = state.cfgFields[0].editor.Text()
		state.printConfig.Port = state.cfgFields[1].editor.Text()
		state.printConfig.FontName = state.cfgFields[2].editor.Text()
		state.printConfig.FontSize = state.cfgFields[3].editor.Text()
		state.printConfig.Copies = state.cfgFields[4].editor.Text()
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
				lbl := material.H6(th, "Printer Config")
				lbl.Color = ColorHeader()
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),

			// Config fields
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(state.cfgFields)*2)
				for i := range state.cfgFields {
					field := &state.cfgFields[i]
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutConfigField(gtx, th, field)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
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

// previewSample picks the label to show in the preview: the first checked
// label, else the first label, else a placeholder.
func previewSample(s *appState) (string, string) {
	for i := range s.labelChecks {
		if i < len(s.labels) && s.labelChecks[i].Value {
			return s.labels[i].TerminalSide, s.labels[i].ComponentSide
		}
	}
	if len(s.labels) > 0 {
		return s.labels[0].TerminalSide, s.labels[0].ComponentSide
	}
	return "-X1:1", "-K1:A:1"
}

// updatePreview re-renders the cached preview image when the roll, font, or
// sample label changes.
func (s *appState) updatePreview() {
	term, comp := previewSample(s)
	roll := rollByIndex(s.printConfig.RollIndex)
	key := fmt.Sprintf("%d|%s|%s|%s|%s", s.printConfig.RollIndex,
		s.printConfig.FontName, s.printConfig.FontSize, term, comp)
	if key == s.previewKey {
		return
	}
	s.previewKey = key

	img, _, err := renderLabel(term, comp, roll, s.printConfig)
	if err != nil {
		s.previewErr = err
		s.previewValid = false
		return
	}
	s.previewErr = nil
	s.previewOp = paint.NewImageOp(img)
	s.previewValid = true
}

func layoutPrintStatus(gtx layout.Context, th *material.Theme, state *appState) layout.Dimensions {
	level, msg := state.getPrintStatus()
	if msg == "" {
		return layout.Dimensions{}
	}
	col := ColorMuted()
	switch level {
	case statusSuccess:
		col = ColorSuccess()
	case statusError:
		col = ColorError()
	case statusInfo:
		col = ColorAccent()
	}
	return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(th, msg)
		lbl.Color = col
		return lbl.Layout(gtx)
	})
}

func layoutRollBar(gtx layout.Context, th *material.Theme, state *appState) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, "Roll:")
				lbl.Color = ColorMuted()
				return layout.Inset{Right: unit.Dp(8), Top: unit.Dp(6)}.Layout(gtx, lbl.Layout)
			}),
		}
		for i := range MarkerRolls {
			i := i
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutRollChip(gtx, th, &state.rollClicks[i], MarkerRolls[i], i == state.printConfig.RollIndex)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
			)
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func layoutRollChip(gtx layout.Context, th *material.Theme, click *widget.Clickable, roll MarkerRoll, selected bool) layout.Dimensions {
	return material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
		macro := op.Record(gtx.Ops)
		dims := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(10), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, roll.Name)
			if selected {
				lbl.Color = th.Palette.ContrastFg
			} else {
				lbl.Color = th.Palette.Fg
			}
			return lbl.Layout(gtx)
		})
		call := macro.Stop()

		bg := ColorSurface()
		if selected {
			bg = ColorAccent()
		}
		rect := clip.UniformRRect(image.Rect(0, 0, dims.Size.X, dims.Size.Y), gtx.Dp(unit.Dp(4))).Push(gtx.Ops)
		paint.ColorOp{Color: bg}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		rect.Pop()

		call.Add(gtx.Ops)
		return dims
	})
}

func layoutPreview(gtx layout.Context, th *material.Theme, state *appState) layout.Dimensions {
	roll := rollByIndex(state.printConfig.RollIndex)
	caption := fmt.Sprintf("Preview — %s, %s %spt", roll.Label(), state.printConfig.FontName, state.printConfig.FontSize)

	return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(6), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, caption)
				lbl.Color = ColorMuted()
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !state.previewValid {
					msg := "preview unavailable"
					if state.previewErr != nil {
						msg = "preview: " + state.previewErr.Error()
					}
					lbl := material.Caption(th, msg)
					lbl.Color = ColorError()
					return lbl.Layout(gtx)
				}
				return layoutPreviewImage(gtx, roll, state.previewOp)
			}),
		)
	})
}

func layoutPreviewImage(gtx layout.Context, roll MarkerRoll, img paint.ImageOp) layout.Dimensions {
	h := gtx.Dp(unit.Dp(52))
	aspect := 1.0
	if roll.HeightMM > 0 {
		aspect = roll.WidthMM / roll.HeightMM
	}
	w := int(float64(h) * aspect)
	if max := gtx.Constraints.Max.X; w > max && max > 0 {
		w = max
		h = int(float64(w) / aspect)
	}

	// White marker background
	rect := clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	rect.Pop()

	igtx := gtx
	igtx.Constraints = layout.Exact(image.Pt(w, h))
	widget.Image{Src: img, Fit: widget.Contain, Position: layout.Center}.Layout(igtx)
	return layout.Dimensions{Size: image.Pt(w, h)}
}

func printSelected(state *appState) {
	var selected []parser.WireLabel
	for i, check := range state.labelChecks {
		if check.Value {
			selected = append(selected, state.labels[i])
		}
	}
	if len(selected) == 0 {
		state.setPrintStatus(statusInfo, "No labels selected")
		return
	}

	cfg := state.printConfig
	roll := rollByIndex(cfg.RollIndex)
	state.setPrintStatus(statusInfo, fmt.Sprintf("Printing %d label(s) to %s…", len(selected), cfg.PrinterHost))

	go func() {
		n, err := executePrint(selected, cfg)
		if err != nil {
			state.setPrintStatus(statusError, fmt.Sprintf("Print failed after %d/%d: %v", n, len(selected), err))
			return
		}
		state.setPrintStatus(statusSuccess, fmt.Sprintf("Sent %d label(s) to %s (%s)", n, cfg.PrinterHost, roll.Name))
	}()
}
