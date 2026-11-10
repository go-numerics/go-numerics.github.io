// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package playground

import (
	"math"
	"runtime"

	"github.com/go-numerics/go-numerics.github.io/playground/engine"
	"github.com/go-widgets/toolkit"
)

// Debug is a snapshot of what the scene shows, for the browser proofs: they
// assert on it after driving the page with real mouse and keyboard events.
type Debug struct {
	Family     string        `json:"family"`
	Case       string        `json:"case"`
	Method     string        `json:"method"`
	Families   []string      `json:"families"`
	Cases      []string      `json:"cases"`
	Methods    []string      `json:"methods"`
	Args       string        `json:"args"`
	Title      string        `json:"title"`
	Verdict    string        `json:"verdict"`
	Timing     string        `json:"timing"`
	Error      string        `json:"error"`
	Rows       []engine.Row  `json:"rows"`
	PlotNames  []string      `json:"plotNames"`
	PlotIndex  int           `json:"plotIndex"`
	Series     []DebugSeries `json:"series"`
	Hover      bool          `json:"hover"`
	HoverAt    int           `json:"hoverAt"`
	Readout    string        `json:"readout"`
	Lang       int           `json:"lang"`
	Code       string        `json:"code"`
	Status     string        `json:"status"`
	Dark       bool          `json:"dark"`
	Goroutines int           `json:"goroutines"`
	MaxProcs   int           `json:"maxProcs"`
}

// DebugSeries is one drawn series: its label, length and a few exact values.
type DebugSeries struct {
	Label string    `json:"label"`
	N     int       `json:"n"`
	X     []float64 `json:"x"`
	Y     []float64 `json:"y"`
}

// maxDebugPoints bounds the points a snapshot carries per series.
const maxDebugPoints = 8192

// Debug reports the scene's current state.
func (s *State) Debug() Debug {
	v := s.VM
	d := Debug{
		Families: v.FamilyNames, Cases: v.Cases.Slice(), Methods: v.Methods.Slice(),
		Args: v.Args.Get(), Title: v.Title.Get(), Verdict: v.Verdict.Get(), Timing: v.Timing.Get(), Error: v.Error.Get(),
		PlotNames: v.PlotNames.Slice(), PlotIndex: v.PlotIndex.Get(),
		Hover: v.HoverOn.Get(), HoverAt: v.HoverIndex.Get(), Readout: v.Readout.Get(),
		Lang: v.CodeLang.Get(), Code: v.Code.Get(), Status: v.Status.Get(), Dark: s.dark,
		Goroutines: runtime.NumGoroutine(), MaxProcs: runtime.GOMAXPROCS(0),
	}
	d.Family = pick(v.FamilyNames, v.FamilyIndex.Get())
	d.Case = pick(d.Cases, v.CaseIndex.Get())
	d.Method = pick(d.Methods, v.MethodIndex.Get())
	if t := v.Table.Get(); t != nil {
		d.Rows = t.Rows
	}
	if p := v.Plot.Get(); p != nil {
		for _, ser := range p.Series {
			n := min(len(ser.X), maxDebugPoints)
			d.Series = append(d.Series, DebugSeries{Label: ser.Label, N: len(ser.X), X: finiteOrNil(ser.X[:n]), Y: finiteOrNil(ser.Y[:min(len(ser.Y), n)])})
		}
	}
	return d
}

func pick(list []string, i int) string {
	if i < 0 || i >= len(list) {
		return ""
	}
	return list[i]
}

// Rects names the rectangles of the widgets a test clicks, in device pixels.
func (s *State) Rects() map[string]toolkit.Rect {
	return map[string]toolkit.Rect{
		"families": s.families.Bounds(), "defaults": s.defaults.Bounds(),
		"cases": s.cases.Bounds(), "methods": s.methods.Bounds(),
		"args": s.args.Bounds(), "run": s.run.Bounds(), "table": s.table.Bounds(),
		"plots": s.plots.Bounds(), "plot": s.plot.Bounds(), "readout": s.readout.Bounds(),
		"lang": s.lang.Bounds(), "copy": s.copyB.Bounds(), "code": s.code.Bounds(),
		"error": s.errLabel.Bounds(), "verdict": s.verdict.Bounds(),
		"familiesPopover": s.families.PopoverBounds(), "casesPopover": s.cases.PopoverBounds(),
		"methodsPopover": s.methods.PopoverBounds(), "plotsPopover": s.plots.PopoverBounds(),
	}
}

// finiteOrNil returns v, or nil when a value is not finite: JSON has no
// spelling for NaN or an infinity, and the snapshot must always encode.
func finiteOrNil(v []float64) []float64 {
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
	}
	return v
}
