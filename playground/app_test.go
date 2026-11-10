// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package playground

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/go-numerics/go-numerics.github.io/playground/engine"
	"github.com/go-numerics/go-numerics.github.io/playground/vm"
	"github.com/go-widgets/toolkit"
)

const testW, testH = 1280, 800

func newState(t *testing.T, w, h int, dark bool) *State {
	t.Helper()
	SetupText(1)
	return NewState(w, h, dark)
}

func frame(s *State) []byte {
	w, h := s.Size()
	buf := make([]byte, 4*w*h)
	s.Draw(buf)
	return buf
}

func center(r toolkit.Rect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

// near counts the pixels of buf within tol of c on every channel, inside r.
func near(buf []byte, w int, r toolkit.Rect, c toolkit.RGBA, tol int) int {
	n := 0
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			i := 4 * (y*w + x)
			if abs(int(buf[i])-int(c.R)) <= tol && abs(int(buf[i+1])-int(c.G)) <= tol && abs(int(buf[i+2])-int(c.B)) <= tol {
				n++
			}
		}
	}
	return n
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestDrawsTheFirstProblem(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s := newState(t, testW, testH, dark)
		if !s.Dirty() || s.Dark() != dark {
			t.Fatalf("dark %v: dirty %v", dark, s.Dirty())
		}
		buf := frame(s)
		if s.Dirty() {
			t.Error("still dirty after a draw")
		}
		rects := s.Rects()
		th := Theme(dark)
		// The curve is drawn in the accent, inside the plot.
		if n := near(buf, testW, rects["plot"], th.Accent, 40); n < 100 {
			t.Errorf("dark %v: %d accent pixels in the plot", dark, n)
		}
		// The table, the code and the plot all took pixels that are not background.
		for _, name := range []string{"table", "code", "plot", "args"} {
			r := rects[name]
			if r.W < 100 || r.H < 50 {
				t.Errorf("%s is %dx%d", name, r.W, r.H)
			}
			if n := near(buf, testW, r, th.Background, 0); n == r.W*r.H {
				t.Errorf("dark %v: %s is blank", dark, name)
			}
		}
		d := s.Debug()
		if d.Family != "Scalar root" || d.Case != "cos x = x" || d.Method != "Brentq" || len(d.Rows) == 0 ||
			len(d.PlotNames) != 2 || len(d.Series) != 3 || d.Series[0].N != 400 || d.Dark != dark || d.Goroutines < 1 {
			t.Errorf("debug: %+v", d)
		}
		if _, err := json.Marshal(d); err != nil {
			t.Error(err)
		}
	}
}

func TestThemeSwitchRecolours(t *testing.T) {
	s := newState(t, testW, testH, false)
	light := frame(s)
	s.SetTheme(true)
	if !s.Dirty() || !s.Dark() {
		t.Fatal("no redraw after the theme changed")
	}
	dark := frame(s)
	if light[0] == dark[0] && light[1] == dark[1] && light[2] == dark[2] {
		t.Error("the background did not change")
	}
}

func TestHoverShowsTheExactComputedValue(t *testing.T) {
	s := newState(t, testW, testH, false)
	frame(s)
	r := s.Rects()["plot"]
	x, y := center(r)
	if !s.HandleMove(x, y) && !s.Dirty() {
		t.Error("a move over the plot changed nothing")
	}
	d := s.Debug()
	if !d.Hover || !strings.HasPrefix(d.Readout, "x = ") || !strings.Contains(d.Readout, "f = ") {
		t.Fatalf("readout %q hover %v", d.Readout, d.Hover)
	}
	// The readout's f is the library's function at that very x, to the bit.
	at := d.HoverAt
	xs, ys := d.Series[0].X, d.Series[0].Y
	fx := math.Cos(xs[at]) - xs[at]
	if !strings.Contains(d.Readout, "f = "+engine.G(fx)) || ys[at] != fx {
		t.Errorf("readout %q, cos x − x = %v at x = %v", d.Readout, fx, xs[at])
	}
	// The pointer leaves.
	s.HandleMove(r.X-5, r.Y-5)
	if d := s.Debug(); d.Hover || d.Readout != vm.Prompt {
		t.Errorf("after leaving: %q", d.Readout)
	}
}

func TestPointerAndKeyboard(t *testing.T) {
	s := newState(t, testW, testH, false)
	frame(s)
	rects := s.Rects()

	// Type into the parameters: click, select all by typing over? Append text.
	ax, ay := center(rects["args"])
	s.HandleClick(ax, ay)
	before := s.VM.Args.Get()
	s.HandleKeyDown("ctrl+End")
	s.HandleChar("#")
	if s.VM.Args.Get() == before {
		t.Error("typing did not reach the parameters")
	}
	s.HandleRelease(ax, ay) // the press captured the args box

	// Run through the button.
	s.VM.Args.Set("a=0\nb=2\nxtol=1e-3")
	rx, ry := center(rects["run"])
	s.HandleClick(rx, ry)
	if !strings.Contains(s.VM.Timing.Get(), "one call") || s.VM.Last() == nil {
		t.Error("the Run button did not run")
	}
	if got := num(s.VM.Table.Get().Rows[0].Value); math.Abs(got-0.7390851332151607) > 1e-3 || got == 0.7390851332151607 {
		t.Errorf("a loose tolerance should give a loose root, got %v", got)
	}

	// Defaults puts the parameters back.
	dx, dy := center(rects["defaults"])
	s.HandleClick(dx, dy)
	if !strings.Contains(s.VM.Args.Get(), "xtol=2e-12") {
		t.Errorf("Defaults: %q", s.VM.Args.Get())
	}

	// A drag that starts in the table is captured by it, whatever it crosses.
	tx, ty := center(rects["table"])
	s.HandleClick(tx, ty)
	if !s.HandleMove(tx+30, ty+30) {
		t.Error("a drag did nothing")
	}
	if !s.HandleRelease(tx+30, ty+30) || s.HandleRelease(tx, ty) {
		t.Error("release: first ends the drag, second has nothing to end")
	}

	// Wheel over the table and the code.
	if !s.HandleScroll(tx, ty, 0, 2) {
		t.Error("scroll")
	}
}

func TestMenusOpenAndChoose(t *testing.T) {
	s := newState(t, testW, testH, false)
	frame(s)
	// Choose the ODE family from the menu with real clicks.
	fx, fy := center(s.Rects()["families"])
	s.HandleClick(fx, fy)
	frame(s)
	pb := s.Rects()["familiesPopover"]
	if pb.W == 0 || pb.H == 0 {
		t.Fatalf("the family menu did not open: %+v", pb)
	}
	n := len(s.VM.FamilyNames)
	item := pb.H / n
	// While open, a wheel over its list scrolls the list, not the page.
	s.HandleScroll(pb.X+5, pb.Y+5, 0, 1)
	s.HandleClick(pb.X+pb.W/2, pb.Y+4*item+item/2) // the fifth: ODE
	if got := s.VM.FamilyIndex.Get(); got != 4 {
		t.Fatalf("family %d after clicking the fifth item", got)
	}
	if d := s.Debug(); d.Family != "ODE" || d.Case != "Lotka–Volterra" || len(d.Series) < 2 {
		t.Errorf("after choosing ODE: %+v", d.Title)
	}
	// Escape closes a menu.
	s.HandleClick(fx, fy)
	frame(s)
	if !s.families.PopoverOpen() {
		t.Fatal("the menu did not reopen")
	}
	s.HandleKeyDown("Escape")
	if s.families.PopoverOpen() {
		t.Error("Escape left the menu open")
	}
	// The plot menu switches to the phase portrait.
	s.VM.PlotIndex.Set(1)
	if d := s.Debug(); d.PlotIndex != 1 || d.Series[0].Label != "orbit" {
		t.Errorf("plot %d series %v", d.PlotIndex, d.Series)
	}
}

func TestNoResultClearsTheTableAndThePlot(t *testing.T) {
	s := newState(t, testW, testH, false)
	s.VM.Args.Set("nonsense=1")
	s.VM.Run.Execute()
	frame(s)
	if len(s.table.Rows) != 0 || len(s.plot.Series().Slice()) != 0 || s.errLabel.Text().Get() == "" {
		t.Errorf("rows %d series %d", len(s.table.Rows), len(s.plot.Series().Slice()))
	}
	if d := s.Debug(); d.Series != nil || d.Rows != nil || !strings.Contains(d.Error, "unknown parameter") {
		t.Errorf("debug after an error: %+v", d.Error)
	}
	s.VM.Defaults.Execute()
	s.VM.Run.Execute()
	if len(s.table.Rows) == 0 || len(s.plot.Series().Slice()) == 0 {
		t.Error("the next run did not fill them again")
	}
}

func TestTheTableHasAHeadingBetweenNumbersAndChecks(t *testing.T) {
	s := newState(t, testW, testH, false)
	var heading int
	for i, r := range s.table.Rows {
		if strings.HasPrefix(r[0], "— against mathematics") {
			heading = i
		}
	}
	if heading != s.VM.Table.Get().CheckFrom || heading == 0 {
		t.Errorf("heading at %d, checks from %d", heading, s.VM.Table.Get().CheckFrom)
	}
}

func TestNarrowLayoutStacksAndAsksForHeight(t *testing.T) {
	s := newState(t, 390, 844, false)
	frame(s)
	r := s.Rects()
	if r["table"].Y <= r["args"].Y || r["plot"].Y <= r["table"].Y {
		t.Errorf("not stacked: %+v", r)
	}
	need := s.MinHeight(390)
	if need <= 844 {
		t.Errorf("a phone needs more than %d px, said %d", 844, need)
	}
	// At that height the plot is at least minPlotH tall.
	s.Resize(390, need)
	if h := s.Rects()["plot"].H; h < minPlotH-2 {
		t.Errorf("plot %d px at the height it asked for", h)
	}
	if s.MinHeight(testW) != 0 {
		t.Error("a wide window needs no extra height")
	}
	if w, h := s.Size(); w != 390 || h != need {
		t.Errorf("size %dx%d", w, h)
	}
	// Back to wide: side by side.
	s.Resize(testW, testH)
	if r := s.Rects(); r["table"].X != r["args"].X || r["plot"].X <= r["table"].X {
		t.Errorf("not side by side: %+v", r)
	}
	s.Resize(0, 0) // never a zero-size scene
	if w, h := s.Size(); w != 1 || h != 1 {
		t.Errorf("size %dx%d", w, h)
	}
}

func TestSetupTextTakesAScale(t *testing.T) {
	SetupText(0) // a missing device pixel ratio is 1
	SetupText(2)
	s := NewState(testW*2, testH*2, false)
	frame(s)
	SetupText(1)
}

func TestCloseUnbindsTheScene(t *testing.T) {
	s := newState(t, testW, testH, false)
	frame(s)
	s.Close()
	s.VM.Args.Set("a=0\nb=2")
	s.VM.Run.Execute()
	if s.Dirty() {
		t.Error("the scene still follows the ViewModel after Close")
	}
	s.Close() // twice is harmless
}

func TestHelpers(t *testing.T) {
	if pick([]string{"a"}, -1) != "" || pick([]string{"a"}, 1) != "" || pick([]string{"a"}, 0) != "a" {
		t.Error("pick")
	}
	if finiteOrNil([]float64{1, math.NaN()}) != nil || finiteOrNil([]float64{1, math.Inf(1)}) != nil || len(finiteOrNil([]float64{1, 2})) != 2 {
		t.Error("finiteOrNil")
	}
	if plotStyle(engine.Line) != toolkit.PlotLine || plotStyle(engine.Stem) != toolkit.PlotStem || plotStyle(engine.Markers) != toolkit.PlotMarkers {
		t.Error("plotStyle")
	}
	// bindText: a value that is already in the widget does not move the caret.
	s := newState(t, testW, testH, false)
	s.VM.Args.Set(s.VM.Args.Get())
	if mutedInk(true) == mutedInk(false) || errorInk(true) == errorInk(false) || brandText(true) == brandText(false) {
		t.Error("theme inks do not differ")
	}
}

func num(s string) float64 {
	var v float64
	_ = json.Unmarshal([]byte(s), &v)
	return v
}
