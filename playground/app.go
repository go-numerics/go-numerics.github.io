// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package playground is the View of the go-numerics playground: a go-widgets
// canvas application compiled to WebAssembly. It builds toolkit widgets, lays
// them out in boxes and binds each one to the ViewModel (package vm) — it
// draws nothing by hand and holds no state of its own beyond layout and
// pointer capture. cmd/playground-wasm is the browser shell around it.
package playground

import (
	"github.com/go-numerics/go-numerics.github.io/playground/vm"
	"github.com/go-widgets/mvvmtk"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// Logical sizes, before the device scale (toolkit.Scaled) is applied.
const (
	margin    = 12
	gap       = 8
	barH      = 34
	rowH      = 30
	labelH    = 22
	leftW     = 500
	codeH     = 190
	narrowW   = 860 // below this the two columns stack
	argsHNar  = 150 // stacked: the parameters' height
	tableHNar = 330 // stacked: the result table's height
	nameColW  = 220
)

// baseFontPx is the body text size in logical pixels.
const baseFontPx = 14

// SetupText selects the toolkit's anti-aliased text at the device scale, so
// one logical pixel is one CSS pixel and text is crisp on a HiDPI screen. It
// must run before the first layout measures text.
func SetupText(scale float64) {
	if scale <= 0 {
		scale = 1
	}
	toolkit.SetMetricScale(scale)
	_ = toolkit.UseOpenTypeTextSize(baseFontPx)
}

// State is the View: the widgets, their layout, the ViewModel they are bound
// to, and the pointer capture.
type State struct {
	w, h  int
	dark  bool
	theme *toolkit.Theme
	VM    *vm.ViewModel

	// top bar
	title    *toolkit.Label
	families *toolkit.DropDown
	defaults *toolkit.Button

	// left: the problem, the solver, the parameters, the numbers
	caseLbl *toolkit.Label
	cases   *toolkit.DropDown
	methLbl *toolkit.Label
	methods *toolkit.DropDown
	argsLbl *toolkit.Label
	args    *toolkit.TextView
	run     *toolkit.Button
	table   *toolkit.Table

	// right: what ran, the plot, the program
	resTitle *toolkit.Label
	doc      *toolkit.Label
	verdict  *toolkit.Label
	timing   *toolkit.Label
	errLabel *toolkit.Label
	plots    *toolkit.DropDown
	plot     *toolkit.XYPlot
	readout  *toolkit.Label
	lang     *toolkit.ViewSwitcher
	copyB    *toolkit.Button
	code     *toolkit.TextView

	status *toolkit.Label

	// The box tree. The boxes are the toolkit's own types, not wrappers: the
	// focus system walks them to find the widget that receives the keyboard.
	root, left, right *toolkit.VBox
	body              toolkit.Widget // an HBox, or a VBox when narrow
	top               toolkit.Widget // an HBox, or a VBox (title above the menu) when narrow
	runBar, codeBar   *toolkit.HBox

	dropdowns []*toolkit.DropDown

	// Pointer capture: the widget a press landed on gets the drag and the
	// release, in its own coordinates, even when the pointer leaves it.
	dragTarget toolkit.Widget

	dirty  bool
	unbind []func()
}

// vbox makes a VBox with the given spacing.
func vbox(spacing int) *toolkit.VBox {
	b := toolkit.NewVBox()
	b.Spacing = spacing
	return b
}

// hbox makes an HBox with the given spacing.
func hbox(spacing int) *toolkit.HBox {
	b := toolkit.NewHBox()
	b.Spacing = spacing
	return b
}

// NewState builds the View over a fresh ViewModel, w×h device pixels.
func NewState(w, h int, dark bool) *State {
	s := &State{VM: vm.New()}
	s.dark = dark
	s.theme = Theme(dark)
	inv := func() { s.dirty = true }
	v := s.VM

	s.title = toolkit.NewLabel("go-numerics playground")
	s.families = toolkit.NewDropDown(v.FamilyNames, 0)
	s.defaults = toolkit.NewButton("Defaults", nil)

	s.caseLbl = toolkit.NewLabel("Problem")
	s.cases = toolkit.NewDropDown(nil, 0)
	s.methLbl = toolkit.NewLabel("Solver")
	s.methods = toolkit.NewDropDown(nil, 0)
	s.argsLbl = toolkit.NewLabel("Parameters — edit the values, then Run")
	s.args = toolkit.NewTextView("")
	s.run = toolkit.NewButton("Run", nil)
	s.table = toolkit.NewTable(nil, nil)

	s.resTitle = toolkit.NewLabel("")
	s.doc = toolkit.NewLabel("")
	s.verdict = toolkit.NewLabel("")
	s.timing = toolkit.NewLabel("")
	s.errLabel = toolkit.NewLabel("")
	s.plots = toolkit.NewDropDown(nil, 0)
	s.plot = toolkit.NewXYPlot()
	s.readout = toolkit.NewLabel("")
	s.lang = toolkit.NewViewSwitcher(vm.CodeTabs, 0)
	s.copyB = toolkit.NewButton("Copy", nil)
	s.code = toolkit.NewTextView("")
	s.code.ShowLineNumbers = true
	s.args.ShowLineNumbers = false

	s.status = toolkit.NewLabel("")
	for _, l := range []*toolkit.Label{s.doc, s.verdict, s.timing, s.errLabel, s.readout, s.status, s.resTitle} {
		l.Ellipsis = true
	}

	// Styles first: BindCommand remembers a button's resting style to restore
	// it when the command becomes executable again.
	s.run.Style = toolkit.ButtonProminent
	s.defaults.Style = toolkit.ButtonSecondary
	s.copyB.Style = toolkit.ButtonSecondary

	s.unbind = append(s.unbind,
		mvvmtk.BindSelectedIndex(s.families, v.FamilyIndex, inv),
		mvvmtk.BindCommand(s.defaults, v.Defaults, inv),
		mvvmtk.BindDropDownOptions(s.cases, v.Cases, func(n string) string { return n }, inv),
		mvvmtk.BindSelectedIndex(s.cases, v.CaseIndex, inv),
		mvvmtk.BindDropDownOptions(s.methods, v.Methods, func(n string) string { return n }, inv),
		mvvmtk.BindSelectedIndex(s.methods, v.MethodIndex, inv),
		bindText(s.args, v.Args, inv),
		mvvmtk.BindCommand(s.run, v.Run, inv),
		bindTable(s.table, v.Table, inv),
		mvvmtk.BindLabel(s.resTitle, v.Title, inv),
		mvvmtk.BindLabel(s.doc, v.CaseDoc, inv),
		mvvmtk.BindLabel(s.verdict, v.Verdict, inv),
		mvvmtk.BindLabel(s.timing, v.Timing, inv),
		mvvmtk.BindLabel(s.errLabel, v.Error, inv),
		mvvmtk.BindDropDownOptions(s.plots, v.PlotNames, func(n string) string { return n }, inv),
		mvvmtk.BindSelectedIndex(s.plots, v.PlotIndex, inv),
		bindPlot(s.plot, v.Plot, inv),
		bindHover(s.plot, v.HoverOn, v.HoverIndex, inv),
		mvvmtk.BindLabel(s.readout, v.Readout, inv),
		mvvmtk.BindViewSwitcher(s.lang, v.CodeLang, inv),
		mvvmtk.BindCommand(s.copyB, v.CopyCode, inv),
		mvvmtk.BindTextView(s.code, v.Code, inv),
		mvvmtk.BindLabel(s.status, v.Status, inv),
	)
	s.dropdowns = []*toolkit.DropDown{s.families, s.cases, s.methods, s.plots}
	s.applyTheme()
	s.Resize(w, h)
	return s
}

// layout (re)builds the box tree for the current width: two columns side by
// side, or stacked on a narrow screen.
func (s *State) layout() {
	sc := toolkit.Scaled
	narrow := s.w < sc(narrowW)

	// On a narrow screen the menu takes the whole width, so the title gets a
	// row of its own.
	topH := sc(barH)
	if narrow {
		pick := hbox(sc(gap))
		pick.AddFlex(s.families, 1)
		pick.AddFixed(s.defaults, sc(100))
		top := vbox(sc(gap))
		top.AddFixed(s.title, sc(barH))
		top.AddFixed(pick, sc(barH))
		s.top, topH = top, sc(2*barH+gap)
	} else {
		top := hbox(sc(gap))
		top.AddFlex(s.title, 1)
		top.AddFixed(s.families, sc(240))
		top.AddFixed(s.defaults, sc(100))
		s.top = top
	}

	s.runBar = hbox(sc(gap))
	s.runBar.AddFlex(s.run, 1)

	s.left = vbox(sc(gap / 2))
	s.left.AddFixed(s.caseLbl, sc(labelH))
	s.left.AddFixed(s.cases, sc(rowH))
	s.left.AddFixed(s.methLbl, sc(labelH))
	s.left.AddFixed(s.methods, sc(rowH))
	s.left.AddFixed(s.argsLbl, sc(labelH))
	if narrow {
		s.left.AddFixed(s.args, sc(argsHNar))
	} else {
		s.left.AddFlex(s.args, 1)
	}
	s.left.AddFixed(s.runBar, sc(rowH+2))
	if narrow {
		s.left.AddFixed(s.table, sc(tableHNar))
	} else {
		s.left.AddFlex(s.table, 2)
	}

	s.codeBar = hbox(sc(gap))
	s.codeBar.AddFlex(s.lang, 1)
	s.codeBar.AddFixed(s.copyB, sc(90))

	s.right = vbox(sc(2))
	s.right.AddFixed(s.resTitle, sc(labelH+4))
	s.right.AddFixed(s.doc, sc(labelH))
	s.right.AddFixed(s.verdict, sc(labelH))
	s.right.AddFixed(s.timing, sc(labelH))
	s.right.AddFixed(s.errLabel, sc(labelH))
	s.right.AddFixed(s.plots, sc(rowH))
	s.right.AddFlex(s.plot, 1)
	s.right.AddFixed(s.readout, sc(labelH))
	s.right.AddFixed(s.codeBar, sc(rowH+2))
	s.right.AddFixed(s.code, sc(codeH))

	if narrow {
		// Stacked: the left column's height is the sum of its fixed parts.
		leftH := 3*sc(labelH) + 2*sc(rowH) + sc(argsHNar) + sc(rowH+2) + sc(tableHNar) + 7*sc(gap/2)
		b := vbox(sc(gap * 2))
		b.AddFixed(s.left, leftH)
		b.AddFlex(s.right, 1)
		s.body = b
	} else {
		b := hbox(sc(gap * 2))
		b.AddFixed(s.left, sc(leftW))
		b.AddFlex(s.right, 1)
		s.body = b
	}
	s.root = vbox(sc(gap))
	s.root.AddFixed(s.top, topH)
	s.root.AddFlex(s.body, 1)
	s.root.AddFixed(s.status, sc(labelH))
	m := sc(margin)
	s.root.SetBounds(toolkit.Rect{X: m, Y: m, W: s.w - 2*m, H: s.h - 2*m})
	s.dirty = true
}

// Resize lays the scene out for w×h device pixels.
func (s *State) Resize(w, h int) {
	s.w, s.h = max(w, 1), max(h, 1)
	s.layout()
}

// minPlotH is the least height, in logical pixels, the stacked layout leaves
// the plot.
const minPlotH = 300

// MinHeight is the least height, in device pixels, the scene needs at width
// w. Side by side it is 0: every widget fits whatever the window gives.
// Stacked, everything has a fixed height but the plot, which took what was
// left, and a short window left it nothing. The host makes the canvas at
// least this tall and lets the page scroll.
//
// It is measured, not computed: the layout is laid out once at a tall height
// and the plot's share is replaced by minPlotH, so formula and layout cannot
// drift.
func (s *State) MinHeight(w int) int {
	if w >= toolkit.Scaled(narrowW) {
		return 0
	}
	ow, oh := s.w, s.h
	const tall = 1 << 15
	s.w, s.h = max(w, 1), tall
	s.layout()
	need := tall - s.plot.Bounds().H + toolkit.Scaled(minPlotH)
	s.w, s.h = ow, oh
	s.layout()
	return need
}

// Close unbinds every widget from the ViewModel: later changes of the
// ViewModel no longer reach the scene.
func (s *State) Close() {
	for _, u := range s.unbind {
		u()
	}
	s.unbind = nil
}

// Size is the scene size in device pixels.
func (s *State) Size() (int, int) { return s.w, s.h }

// Dark reports whether the dark palette is active.
func (s *State) Dark() bool { return s.dark }

// SetTheme switches between the light and the dark palette.
func (s *State) SetTheme(dark bool) {
	s.dark = dark
	s.theme = Theme(dark)
	s.applyTheme()
	s.dirty = true
}

// applyTheme sets the ink of the labels that are not plain body text.
func (s *State) applyTheme() {
	muted := mutedInk(s.dark)
	s.title.Ink = brandText(s.dark)
	s.title.FontSize = toolkit.Scaled(19)
	s.resTitle.Ink = brandText(s.dark)
	for _, l := range []*toolkit.Label{s.doc, s.timing, s.readout, s.status, s.caseLbl, s.methLbl, s.argsLbl} {
		l.Ink = muted
	}
	s.errLabel.Ink = errorInk(s.dark)
}

// Draw paints the whole scene into buf (w×h RGBA). Open popovers are drawn
// last so they float above the widgets under them.
func (s *State) Draw(buf []byte) {
	p := painter.NewPixelPainter(buf, s.w, s.h)
	fillBackground(buf, s.theme.Background)
	s.root.Draw(p, s.theme)
	for _, d := range s.dropdowns {
		d.DrawPopover(p, s.theme)
	}
	s.dirty = false
}

// Dirty reports whether something changed since the last Draw.
func (s *State) Dirty() bool { return s.dirty }

// Theme returns the playground palette: the toolkit defaults with the
// landing's neutral backgrounds and its purple accent (go-numerics.github.io's
// [params.brand]: base #7445ad in light, bright #b794e4 in dark).
func Theme(dark bool) *toolkit.Theme {
	if dark {
		t := toolkit.DefaultDark()
		t.Background = toolkit.RGB(0x0b, 0x0e, 0x14)
		t.Surface = toolkit.RGB(0x14, 0x1a, 0x24)
		t.SurfaceAlt = toolkit.RGB(0x1c, 0x23, 0x31)
		t.Border = toolkit.RGB(0x2a, 0x33, 0x42)
		t.OnBackground = toolkit.RGB(0xe6, 0xed, 0xf3)
		t.OnSurface = toolkit.RGB(0xe6, 0xed, 0xf3)
		// The bright purple reads on the dark background (curves, fills); it
		// carries the dark ink, not white, as text.
		t.Accent = toolkit.RGB(0xb7, 0x94, 0xe4)
		ink := toolkit.RGB(0x1a, 0x0f, 0x2e)
		t.Extra = map[string]toolkit.RGBA{"OnAccent": ink, "accent_fg_color": ink}
		return t
	}
	t := toolkit.DefaultLight()
	t.Background = toolkit.RGB(0xff, 0xff, 0xff)
	t.Surface = toolkit.RGB(0xf7, 0xf8, 0xfb)
	t.SurfaceAlt = toolkit.RGB(0xe7, 0xe9, 0xee)
	t.Border = toolkit.RGB(0xe0, 0xe3, 0xea)
	t.OnBackground = toolkit.RGB(0x0f, 0x11, 0x15)
	t.OnSurface = toolkit.RGB(0x0f, 0x11, 0x15)
	t.Accent = toolkit.RGB(0x74, 0x45, 0xad)
	white := toolkit.RGB(0xff, 0xff, 0xff)
	t.Extra = map[string]toolkit.RGBA{"OnAccent": white, "accent_fg_color": white}
	return t
}

// brandText is the accent as a text colour: the landing's --accent.
func brandText(dark bool) toolkit.RGBA {
	if dark {
		return toolkit.RGB(0xb7, 0x94, 0xe4)
	}
	return toolkit.RGB(0x74, 0x45, 0xad)
}

// mutedInk is the landing's --slate.
func mutedInk(dark bool) toolkit.RGBA {
	if dark {
		return toolkit.RGB(0x9a, 0xa4, 0xb2)
	}
	return toolkit.RGB(0x47, 0x55, 0x69)
}

// errorInk is a red that reads on the background of each theme.
func errorInk(dark bool) toolkit.RGBA {
	if dark {
		return toolkit.RGB(0xf7, 0x8b, 0x8b)
	}
	return toolkit.RGB(0xb4, 0x23, 0x18)
}

// fillBackground clears the frame to the theme background before the widgets
// paint over it: a buffer clear, not drawing.
func fillBackground(buf []byte, c toolkit.RGBA) {
	for i := 0; i+3 < len(buf); i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = c.R, c.G, c.B, c.A
	}
}
