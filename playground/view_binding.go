// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

// The bindings mvvmtk does not provide. This is the one file allowed to set a
// widget's data fields (mvvmlint exempts *_binding.go): each function here is a
// one-way or two-way link between a ViewModel Observable and a widget, used
// once from NewState — never a per-frame copy.

package playground

import (
	"github.com/go-numerics/go-numerics.github.io/playground/engine"
	"github.com/go-numerics/go-numerics.github.io/playground/vm"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/toolkit"
)

// bindText links a TextView and a string Observable both ways. A value from
// the ViewModel goes through SetText, which parks the caret at the end.
func bindText(t *toolkit.TextView, obs *mvvm.Observable[string], invalidate func()) (unbind func()) {
	t.SetText(obs.Get())
	a := obs.Subscribe(func(s string) {
		if t.Text().Get() != s {
			t.SetText(s)
		}
		invalidate()
	})
	b := t.Text().Subscribe(func(s string) { obs.Set(s) })
	return func() { a(); b() }
}

// bindTable shows the result as a two-column table: the library's numbers,
// then a heading row, then the checks against mathematics.
func bindTable(t *toolkit.Table, obs *mvvm.Observable[*vm.Table], invalidate func()) (unbind func()) {
	show := func(tb *vm.Table) {
		t.Selected().Set(-1)
		t.ScrollRow().Set(0)
		t.ScrollX().Set(0)
		if tb == nil || len(tb.Rows) == 0 {
			t.Columns, t.Rows = nil, nil
			invalidate()
			return
		}
		t.Columns = []toolkit.TableColumn{
			{Title: "result", Width: toolkit.Scaled(nameColW)},
			{Title: "value"},
		}
		rows := make([][]string, 0, len(tb.Rows)+1)
		for i, r := range tb.Rows {
			if i == tb.CheckFrom {
				rows = append(rows, []string{"— against mathematics —", ""})
			}
			rows = append(rows, []string{r.Name, r.Value})
		}
		t.Rows = rows
		invalidate()
	}
	show(obs.Get())
	return obs.Subscribe(show)
}

// bindPlot shows a plot of the ViewModel in an XYPlot: its captions, its
// series, and no stale hover from the plot before.
func bindPlot(p *toolkit.XYPlot, obs *mvvm.Observable[*engine.Plot], invalidate func()) (unbind func()) {
	show := func(pl *engine.Plot) {
		p.Hover().Set(false)
		if pl == nil {
			p.XLabel, p.YLabel = "", ""
			p.SetSeries()
			invalidate()
			return
		}
		p.XLabel, p.YLabel = pl.XLabel, pl.YLabel
		series := make([]toolkit.PlotSeries, len(pl.Series))
		for i, s := range pl.Series {
			series[i] = toolkit.PlotSeries{Label: s.Label, X: s.X, Y: s.Y, Style: plotStyle(s.Style)}
			if i > 0 {
				series[i].Ink = extraInks[(i-1)%len(extraInks)]
			}
		}
		p.SetSeries(series...)
		invalidate()
	}
	show(obs.Get())
	return obs.Subscribe(show)
}

// extraInks colour the second, third, … series: mid-tones that read on both
// backgrounds and none of them the purple of the first curve.
var extraInks = []toolkit.RGBA{
	toolkit.RGB(0xE8, 0x71, 0x0A), // orange
	toolkit.RGB(0x12, 0x8C, 0xB5), // teal-blue
	toolkit.RGB(0xD9, 0x30, 0x25), // red
	toolkit.RGB(0x2E, 0x9E, 0x4F), // green
}

// plotStyle maps the engine's drawing styles onto the toolkit's.
func plotStyle(s engine.Style) toolkit.PlotStyle {
	switch s {
	case engine.Stem:
		return toolkit.PlotStem
	case engine.Markers:
		return toolkit.PlotMarkers
	}
	return toolkit.PlotLine
}

// bindHover reports the plot widget's own pointer tracking to the ViewModel,
// which turns it into the exact readout.
func bindHover(p *toolkit.XYPlot, on *mvvm.Observable[bool], index *mvvm.Observable[int], invalidate func()) (unbind func()) {
	a := p.Hover().Subscribe(func(b bool) { on.Set(b); invalidate() })
	b := p.HoverIndex().Subscribe(func(i int) { index.Set(i); invalidate() })
	return func() { a(); b() }
}
