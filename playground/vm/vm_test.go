// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package vm

import (
	"strings"
	"testing"
)

func TestStartsOnTheFirstProblemAndRunsIt(t *testing.T) {
	v := New()
	if v.FamilyNames[0] != "Scalar root" || len(v.FamilyNames) != 6 {
		t.Fatalf("families %v", v.FamilyNames)
	}
	if v.Cases.Len() != 5 || v.Methods.Len() != 6 || v.CaseDoc.Get() == "" || !strings.Contains(v.Args.Get(), "a=0  # left end") {
		t.Errorf("menus: %d cases, %d methods", v.Cases.Len(), v.Methods.Len())
	}
	tb := v.Table.Get()
	if tb == nil || len(tb.Rows) == 0 || tb.CheckFrom != 6 || tb.Rows[0].Name != "root" {
		t.Fatalf("table %+v", tb)
	}
	if v.Plot.Get() == nil || v.PlotNames.Len() != 2 || v.Code.Get() == "" || v.Last() == nil {
		t.Error("no plot, code or result")
	}
	if !strings.HasPrefix(v.Verdict.Get(), "converged") || !strings.Contains(v.Timing.Get(), "one call:") ||
		!strings.Contains(v.Status.Get(), "WebAssembly") || v.Error.Get() != "" || v.Title.Get() == "" {
		t.Errorf("verdict %q timing %q status %q error %q", v.Verdict.Get(), v.Timing.Get(), v.Status.Get(), v.Error.Get())
	}
	if v.Readout.Get() != Prompt {
		t.Errorf("readout %q", v.Readout.Get())
	}
}

func TestEveryMenuChoiceRunsTheProblem(t *testing.T) {
	v := New()
	for fi, name := range v.FamilyNames {
		v.CaseIndex.Set(1) // so that changing family has a case index to reset
		v.FamilyIndex.Set(fi)
		if fi > 0 && v.CaseIndex.Get() != 0 || v.Title.Get() == "" || v.Error.Get() != "" && !strings.Contains(v.Error.Get(), "the library says") {
			t.Errorf("%s: case %d, title %q, error %q", name, v.CaseIndex.Get(), v.Title.Get(), v.Error.Get())
		}
		for ci := 0; ci < v.Cases.Len(); ci++ {
			v.CaseIndex.Set(ci)
			for mi := 0; mi < v.Methods.Len(); mi++ {
				v.MethodIndex.Set(mi)
				if v.Last() == nil || !strings.Contains(v.Title.Get(), v.Methods.At(mi)) {
					t.Errorf("%s/%s/%s: title %q", name, v.Cases.At(ci), v.Methods.At(mi), v.Title.Get())
				}
				if !strings.Contains(v.Args.Get(), "=") {
					t.Errorf("no default arguments: %q", v.Args.Get())
				}
			}
		}
	}
}

func TestEditedArgumentsRunOnRunAndErrorsAreShown(t *testing.T) {
	v := New()
	before := v.Table.Get().Rows[0].Value
	v.Args.Set("a=0, b=2, xtol=1e-3")
	if v.Table.Get().Rows[0].Value != before {
		t.Error("typing alone ran the problem")
	}
	v.Run.Execute()
	if v.Table.Get().Rows[0].Value == before {
		t.Error("Run did not run with the new tolerance")
	}
	v.Args.Set("a=0 # a comment\nb=2\nxtol=1e-3")
	v.Run.Execute()
	if v.Error.Get() != "" {
		t.Errorf("a multi-line arguments text: %q", v.Error.Get())
	}

	v.Args.Set("a=0, nonsense=1")
	v.Run.Execute()
	if !strings.Contains(v.Error.Get(), "unknown parameter") || v.Table.Get() != nil || v.Plot.Get() != nil ||
		v.Code.Get() != "" || v.Last() != nil || v.PlotNames.Len() != 0 || v.Title.Get() != "Not run" || v.CopyCode.CanExecute() {
		t.Errorf("after a bad line: error %q", v.Error.Get())
	}

	// The library's own error: no sign change, so no result.
	v.Args.Set("a=2, b=3")
	v.Run.Execute()
	if !strings.HasPrefix(v.Error.Get(), "the library says: ") || !strings.HasPrefix(v.Verdict.Get(), "no result") {
		t.Errorf("error %q verdict %q", v.Error.Get(), v.Verdict.Get())
	}

	// Defaults restores the arguments.
	v.Defaults.Execute()
	v.Run.Execute()
	if v.Error.Get() != "" || !strings.HasPrefix(v.Args.Get(), "a=0  # left end of the bracket\nb=1  # right end") {
		t.Errorf("defaults: %q, error %q", v.Args.Get(), v.Error.Get())
	}
}

func TestNotConvergedAndSweepTiming(t *testing.T) {
	v := New()
	v.FamilyIndex.Set(3) // Quadrature
	v.MethodIndex.Set(1) // Romberg on sin x converges
	v.CaseIndex.Set(1)   // 1/√x: Romberg gives a value and the library's error
	v.MethodIndex.Set(1)
	if !strings.HasPrefix(v.Verdict.Get(), "not converged") || !strings.HasPrefix(v.Error.Get(), "the library says: ") {
		t.Errorf("verdict %q error %q", v.Verdict.Get(), v.Error.Get())
	}
	v.FamilyIndex.Set(5) // Special functions
	if !strings.Contains(v.Timing.Get(), "one sweep of 400 points") || !strings.Contains(v.Timing.Get(), "per point") {
		t.Errorf("timing %q", v.Timing.Get())
	}
}

func TestHoverReadoutGivesTheExactValues(t *testing.T) {
	v := New() // roots: the plot is f with markers
	p := v.Plot.Get()
	i := 100
	v.HoverIndex.Set(i)
	if v.Readout.Get() != Prompt {
		t.Error("an index alone shows a readout")
	}
	v.HoverOn.Set(true)
	r := v.Readout.Get()
	// The curve has Raw values named f; the markers series has another length.
	want := "x = " + gs(p.Series[0].X[i]) + "   f = " + gs(p.Series[0].Raw[i])
	if r != want {
		t.Errorf("readout %q, want %q", r, want)
	}
	v.HoverOn.Set(false)
	if v.Readout.Get() != Prompt {
		t.Error("the readout stays after the pointer left")
	}

	// A plot of several series that share their x: every value is listed.
	v.FamilyIndex.Set(4) // ODE, Lotka–Volterra
	v.HoverIndex.Set(3)
	v.HoverOn.Set(true)
	if r := v.Readout.Get(); !strings.Contains(r, "prey x = ") || !strings.Contains(r, "predators y = ") {
		t.Errorf("ODE readout %q", r)
	}
	// Choosing another plot clears the hover.
	v.PlotIndex.Set(1)
	if v.HoverOn.Get() || v.Readout.Get() != Prompt || v.Plot.Get().Name != "phase portrait" {
		t.Errorf("after the plot changed: hover %v, plot %q", v.HoverOn.Get(), v.Plot.Get().Name)
	}
	// An index beyond the series, and a plot index beyond the plots.
	v.HoverOn.Set(true)
	v.HoverIndex.Set(1 << 20)
	if v.Readout.Get() != Prompt {
		t.Error("an out-of-range hover shows a readout")
	}
	v.PlotIndex.Set(99)
	if v.Plot.Get() != nil || v.Readout.Get() != Prompt {
		t.Error("a plot index out of range shows a plot")
	}
	v.PlotIndex.Set(-1)
	if v.Plot.Get() != nil {
		t.Error("a negative plot index shows a plot")
	}
}

func TestCodeTabsAndCopy(t *testing.T) {
	v := New()
	if !strings.Contains(v.Code.Get(), "optimize.Brentq") {
		t.Errorf("Go: %q", v.Code.Get())
	}
	v.CodeLang.Set(LangPython)
	if !strings.Contains(v.Code.Get(), "root_scalar") {
		t.Errorf("SciPy: %q", v.Code.Get())
	}
	v.Run.Execute() // a new run keeps the chosen language
	if !strings.Contains(v.Code.Get(), "root_scalar") {
		t.Error("a run reset the language")
	}
	v.CopyCode.Execute()
	if !strings.Contains(v.Status.Get(), "no clipboard") {
		t.Errorf("status %q", v.Status.Get())
	}
	var got string
	v.Clipboard = func(s string) { got = s }
	v.CopyCode.Execute()
	if got != v.Code.Get() || !strings.Contains(v.Status.Get(), "copied the SciPy program") {
		t.Errorf("copied %q, status %q", got, v.Status.Get())
	}
	// A bad run empties the code and takes the copy away; a good one restores it.
	v.Args.Set("zz=1")
	v.Run.Execute()
	if v.CopyCode.CanExecute() {
		t.Error("copy offered with no program")
	}
	v.CodeLang.Set(LangGo) // switching tabs with nothing run leaves the panel empty
	if v.Code.Get() != "" {
		t.Errorf("code after a bad run: %q", v.Code.Get())
	}
	v.Defaults.Execute()
	v.Run.Execute()
	if !v.CopyCode.CanExecute() {
		t.Error("copy not offered")
	}
}

func TestAnIndexOutOfRangeFallsBackToTheFirst(t *testing.T) {
	v := New()
	v.CaseIndex.Set(99)
	if v.Methods.Len() == 0 || v.Args.Get() == "" {
		t.Error("the menus were not rebuilt for a case that does not exist")
	}
	if v.Last() != nil || !strings.Contains(v.Error.Get(), "no such case") {
		t.Errorf("error %q", v.Error.Get())
	}
	v.CaseIndex.Set(-1)
	if !strings.Contains(v.Error.Get(), "no such case") {
		t.Errorf("error %q", v.Error.Get())
	}
}

func gs(x float64) string { return strings.TrimSpace(formatG(x)) }
