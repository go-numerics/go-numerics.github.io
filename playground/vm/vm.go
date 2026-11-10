// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package vm is the ViewModel of the go-numerics playground. It holds every
// piece of state the page shows or edits as a go-widgets/mvvm Observable,
// ObservableList or Command, and references no widget: the View binds to
// these, so the whole behaviour is unit-tested here without a canvas.
package vm

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/go-numerics/go-numerics.github.io/playground/engine"
	"github.com/go-widgets/mvvm"
)

// Code languages of the code panel, in tab order.
const (
	LangGo = iota
	LangPython
)

// CodeTabs are the titles of the code panel's tabs.
var CodeTabs = []string{"Go", "SciPy"}

// Table is what the result table shows: the library's own numbers, then the
// checks against mathematics.
type Table struct {
	Rows []engine.Row
	// CheckFrom is the index of the first row of the checks.
	CheckFrom int
}

// Prompt is the readout while the pointer is off the plot.
const Prompt = "Move the pointer over the plot: the exact computed value at that point appears here."

// ViewModel is the whole playground state.
type ViewModel struct {
	// FamilyNames is the fixed first menu.
	FamilyNames []string

	// --- the form -----------------------------------------------------------
	FamilyIndex *mvvm.Observable[int]
	Cases       *mvvm.ObservableList[string]
	CaseIndex   *mvvm.Observable[int]
	Methods     *mvvm.ObservableList[string]
	MethodIndex *mvvm.Observable[int]
	CaseDoc     *mvvm.Observable[string]
	// Args is the parameters, one "key=value  # what it is" per line, edited by
	// the visitor.
	Args *mvvm.Observable[string]

	// --- the result ---------------------------------------------------------
	Title   *mvvm.Observable[string]
	Verdict *mvvm.Observable[string]
	Timing  *mvvm.Observable[string]
	Table   *mvvm.Observable[*Table]
	Error   *mvvm.Observable[string] // the page's, or "the library says: …"

	// --- the plot -----------------------------------------------------------
	PlotNames *mvvm.ObservableList[string]
	PlotIndex *mvvm.Observable[int]
	Plot      *mvvm.Observable[*engine.Plot] // nil when there is none
	// HoverOn and HoverIndex are written by the View from the plot widget's
	// pointer tracking; Readout is what they mean.
	HoverOn    *mvvm.Observable[bool]
	HoverIndex *mvvm.Observable[int]
	Readout    *mvvm.Observable[string]

	// --- the code -----------------------------------------------------------
	CodeLang *mvvm.Observable[int]
	Code     *mvvm.Observable[string]

	Status *mvvm.Observable[string]

	// --- commands -----------------------------------------------------------
	Run       *mvvm.Command
	Defaults  *mvvm.Command
	CopyCode  *mvvm.Command
	Clipboard func(string) // the host installs it
	// Runtime describes where the code runs, for the footer; the host fills it
	// in and calls ShowStatus.
	Runtime string

	last  *engine.Result
	quiet bool // a menu is being rebuilt: its changes do not run anything
}

// New builds the ViewModel on the first problem and runs it.
func New() *ViewModel {
	v := &ViewModel{
		FamilyIndex: mvvm.NewObservable(0),
		Cases:       mvvm.NewObservableList[string](),
		CaseIndex:   mvvm.NewObservable(0),
		Methods:     mvvm.NewObservableList[string](),
		MethodIndex: mvvm.NewObservable(0),
		CaseDoc:     mvvm.NewObservable(""),
		Args:        mvvm.NewObservable(""),
		Title:       mvvm.NewObservable(""),
		Verdict:     mvvm.NewObservable(""),
		Timing:      mvvm.NewObservable(""),
		Table:       mvvm.NewObservableEq[*Table](nil, nil),
		Error:       mvvm.NewObservable(""),
		PlotNames:   mvvm.NewObservableList[string](),
		PlotIndex:   mvvm.NewObservable(0),
		Plot:        mvvm.NewObservableEq[*engine.Plot](nil, nil),
		HoverOn:     mvvm.NewObservable(false),
		HoverIndex:  mvvm.NewObservable(0),
		Readout:     mvvm.NewObservable(Prompt),
		CodeLang:    mvvm.NewObservable(LangGo),
		Code:        mvvm.NewObservable(""),
		Status:      mvvm.NewObservable(""),
		Runtime:     runtime.GOOS + "/" + runtime.GOARCH + ", GOMAXPROCS " + strconv.Itoa(runtime.GOMAXPROCS(0)),
	}
	for _, f := range engine.Families {
		v.FamilyNames = append(v.FamilyNames, f.Name)
	}
	v.Run = mvvm.NewCommand(v.run, nil)
	v.Defaults = mvvm.NewCommand(v.resetArgs, nil)
	v.CopyCode = mvvm.NewCommand(v.copyCode, func() bool { return v.Code.Get() != "" })
	mvvm.BindCanExecute(v.CopyCode, v.Code)

	v.FamilyIndex.SubscribeChanged(v.onFamily)
	v.CaseIndex.SubscribeChanged(v.onCase)
	v.MethodIndex.SubscribeChanged(v.onMethod)
	v.PlotIndex.SubscribeChanged(v.showPlot)
	v.CodeLang.SubscribeChanged(v.showCode)
	v.HoverOn.SubscribeChanged(v.showReadout)
	v.HoverIndex.SubscribeChanged(v.showReadout)

	v.onFamily()
	return v
}

// Last is the result on display, nil after an error of the page.
func (v *ViewModel) Last() *engine.Result { return v.last }

// current resolves the menus; the indexes are always valid because the lists
// are rebuilt from the catalogue.
func (v *ViewModel) current() (*engine.Case, string) {
	c, m, err := engine.Lookup(v.FamilyIndex.Get(), v.CaseIndex.Get(), v.MethodIndex.Get())
	if err != nil { // a host that set an index out of range: fall back to the first
		c, m, _ = engine.Lookup(v.FamilyIndex.Get(), 0, 0)
	}
	return c, m
}

// rebuild runs a menu change quietly, then runs the problem once.
func (v *ViewModel) rebuild(fn func()) {
	if v.quiet {
		return
	}
	v.quiet = true
	fn()
	v.quiet = false
	v.run()
}

func (v *ViewModel) onFamily() {
	v.rebuild(func() {
		fam := engine.Families[v.FamilyIndex.Get()]
		names := make([]string, len(fam.Cases))
		for i, c := range fam.Cases {
			names[i] = c.Name
		}
		v.Cases.Clear()
		v.Cases.Append(names...)
		v.CaseIndex.Set(0)
		v.fillMethods()
		v.resetArgs()
	})
}

func (v *ViewModel) onCase() {
	v.rebuild(func() {
		v.fillMethods()
		v.resetArgs()
	})
}

func (v *ViewModel) onMethod() { v.rebuild(v.resetArgs) }

// fillMethods offers the solvers of the selected case.
func (v *ViewModel) fillMethods() {
	fam := engine.Families[v.FamilyIndex.Get()]
	ci := v.CaseIndex.Get()
	if ci < 0 || ci >= len(fam.Cases) {
		ci = 0
	}
	v.Methods.Clear()
	v.Methods.Append(fam.Cases[ci].Methods...)
	v.MethodIndex.Set(0)
}

// resetArgs puts the selected case's default parameters in the form.
func (v *ViewModel) resetArgs() {
	c, m := v.current()
	v.CaseDoc.Set(c.Doc)
	v.Args.Set(engine.DefaultArgs(c, m))
}

// run solves the problem with the form's parameters.
func (v *ViewModel) run() {
	fi, ci, mi := v.FamilyIndex.Get(), v.CaseIndex.Get(), v.MethodIndex.Get()
	res, err := engine.Execute(fi, ci, mi, v.Args.Get())
	v.ShowStatus()
	if err != nil {
		v.last = nil
		v.Error.Set(err.Error())
		v.Title.Set("Not run")
		v.Verdict.Set("")
		v.Timing.Set("")
		v.Table.Set(nil)
		v.setPlots(nil)
		v.Code.Set("")
		return
	}
	v.last = res
	if res.Err != "" {
		v.Error.Set("the library says: " + res.Err)
	} else {
		v.Error.Set("")
	}
	v.Title.Set(res.Title)
	v.Verdict.Set(verdict(res))
	v.Timing.Set(timing(res))
	rows := append([]engine.Row(nil), res.Summary...)
	t := &Table{Rows: append(rows, res.Check...), CheckFrom: len(res.Summary)}
	v.Table.Set(t)
	v.setPlots(res.Plots)
	v.showCode()
}

// verdict is the library's own conclusion, in a line.
func verdict(r *engine.Result) string {
	switch {
	case len(r.Summary) == 0:
		return "no result: " + r.Err
	case r.Converged && r.Status == "converged":
		return "converged"
	case r.Converged:
		return "converged — " + r.Status
	}
	return "not converged — " + r.Status
}

// timing is the line about the time of one call.
func timing(r *engine.Result) string {
	s := fmt.Sprintf("one call: %s (mean of %d), uninstrumented", engine.FormatDuration(r.Per), r.Reps)
	if r.Points > 0 {
		s = fmt.Sprintf("one sweep of %d points: %s, %s per point (mean of %d)", r.Points,
			engine.FormatDuration(r.Per), engine.FormatDuration(r.Per/time.Duration(r.Points)), r.Reps)
	}
	return s + " — WebAssembly, one thread"
}

// setPlots offers the result's plots and shows the first.
func (v *ViewModel) setPlots(plots []engine.Plot) {
	names := make([]string, len(plots))
	for i, p := range plots {
		names[i] = p.Name
	}
	v.PlotNames.Clear()
	v.PlotNames.Append(names...)
	v.PlotIndex.Set(0)
	v.showPlot()
}

// showPlot publishes the selected plot and clears the hover.
func (v *ViewModel) showPlot() {
	v.HoverOn.Set(false)
	i := v.PlotIndex.Get()
	if v.last == nil || i < 0 || i >= len(v.last.Plots) {
		v.Plot.Set(nil)
	} else {
		v.Plot.Set(&v.last.Plots[i])
	}
	v.showReadout()
}

// showReadout prints the exact values at the hovered point: the x of the first
// series and, for every series that has a point at that x, its value. A series
// whose drawn values were transformed (a logarithm, a clip) reports the raw
// ones the library computed.
func (v *ViewModel) showReadout() {
	p := v.Plot.Get()
	i := v.HoverIndex.Get()
	if p == nil || !v.HoverOn.Get() || len(p.Series) == 0 || i < 0 || i >= len(p.Series[0].X) {
		v.Readout.Set(Prompt)
		return
	}
	x := p.Series[0].X[i]
	s := "x = " + engine.G(x)
	for _, ser := range p.Series {
		if i >= len(ser.X) || ser.X[i] != x || len(ser.X) != len(p.Series[0].X) {
			continue
		}
		name, y := ser.Label, ser.Y[i]
		if ser.Raw != nil {
			name, y = ser.RawName, ser.Raw[i]
		}
		s += "   " + name + " = " + engine.G(y)
	}
	v.Readout.Set(s)
}

// showCode shows the selected language's program.
func (v *ViewModel) showCode() {
	switch {
	case v.last == nil:
		v.Code.Set("")
	case v.CodeLang.Get() == LangPython:
		v.Code.Set(v.last.Python)
	default:
		v.Code.Set(v.last.Go)
	}
}

func (v *ViewModel) copyCode() {
	if v.Clipboard != nil {
		v.Clipboard(v.Code.Get())
		v.Status.Set("copied the " + CodeTabs[v.CodeLang.Get()] + " program")
		return
	}
	v.Status.Set("no clipboard in this host")
}

// ShowStatus puts the footer text: where the code runs, and what the timings
// are.
func (v *ViewModel) ShowStatus() {
	v.Status.Set("Computed in your browser by go-numerics compiled to WebAssembly (" + v.Runtime +
		"): one thread and no SIMD kernels, so slower than native Go. Nothing is computed in JavaScript.")
}
