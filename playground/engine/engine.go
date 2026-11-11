// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package engine is the model of the go-numerics playground: a catalogue of
// problems (families, cases, solver methods, editable parameters), the
// execution of one problem by go-numerics' own optimize, integrate and special
// packages, and everything the page shows about the run: the numbers, checks
// against a mathematical reference, the plots, the time of one call and the
// equivalent Go and SciPy programs. It holds no widget.
package engine

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Param is one editable parameter of a case: its key in the arguments line, its
// default text and a few words on what it is.
type Param struct{ Key, Default, Doc string }

// Style is how a series is drawn.
type Style int

const (
	Line Style = iota
	Stem
	Markers
)

// Series is one curve of a plot. Y is what is drawn; Raw, when non-nil, holds
// the exact values behind it (a plot of log10 f keeps f in Raw) and RawName
// names them in the readout.
type Series struct {
	Label   string
	X, Y    []float64
	Style   Style
	Raw     []float64
	RawName string
}

// Plot is one chart of a result.
type Plot struct {
	Name, XLabel, YLabel string
	Series               []Series
}

// Row is one labelled value of the result table.
type Row struct{ Name, Value string }

// Result is everything one run produced.
type Result struct {
	Title   string
	Summary []Row // the library's own outputs
	Check   []Row // the same answer against a mathematical reference
	Plots   []Plot
	Go      string
	Python  string
	// Ruby and Julia are programs too, or the reason there is none (see
	// HasProgram): go-embedded-ruby binds go-numerics but not all of it, and
	// the Julia ones use Julia's packages.
	Ruby, Julia string
	// Status is the library's own verdict (its flag or message); Converged is
	// its boolean.
	Status    string
	Converged bool
	// Err is the library's own error text. The Summary may still hold the best
	// estimate it returned alongside (QUADPACK does).
	Err string
	// Per is the mean time of one call of the library, measured without any
	// instrumentation; Reps is how many calls were timed.
	Per  time.Duration
	Reps int
	// Points is non-zero for a run that evaluates a function at that many
	// points per call (special functions): Per is then the time of the sweep.
	Points int
}

// Case is one problem of a family.
type Case struct {
	Name    string
	Doc     string
	Methods []string
	// Params lists the parameters a method takes, with their defaults.
	Params func(method string) []Param
	// solve runs the problem with the method on the parsed arguments.
	solve func(method string, a *Args) (*run, error)
}

// Family is a group of cases that share a kind of solver.
type Family struct {
	Name  string
	Cases []Case
}

// Families is the catalogue, in menu order.
var Families = []Family{rootFamily, minFamily, fitFamily, quadFamily, odeFamily, specialFamily}

// run is what a case's solver hands back: the instrumented result and a bare
// closure that makes the same library call with nothing around it, for timing.
type run struct {
	res  *Result
	bare func()
}

// budget bounds the work one run may do: the playground runs in a browser tab
// on one thread, and a parameter typo must not freeze it.
var budget = 5_000_000

type budgetExceeded struct{}

// tap counts the calls of an instrumented function and records the points of a
// scalar one. A nil *tap passes the function through untouched, which is what
// the timed calls use.
type tap struct {
	n      int
	xs, ys []float64
	keep   func(x float64) bool
}

const maxRecorded = 4000

func (t *tap) count() {
	t.n++
	if t.n > budget {
		panic(budgetExceeded{})
	}
}

// f1 wraps a scalar function.
func (t *tap) f1(f func(float64) float64) func(float64) float64 {
	if t == nil {
		return f
	}
	return func(x float64) float64 {
		t.count()
		y := f(x)
		if len(t.xs) < maxRecorded && (t.keep == nil || t.keep(x)) {
			t.xs, t.ys = append(t.xs, x), append(t.ys, y)
		}
		return y
	}
}

// fv wraps an objective of a vector.
func (t *tap) fv(f func([]float64) float64) func([]float64) float64 {
	if t == nil {
		return f
	}
	return func(x []float64) float64 { t.count(); return f(x) }
}

// guard runs fn and turns a budget panic into an error.
func guard(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(budgetExceeded); !ok {
				panic(r)
			}
			err = fmt.Errorf("playground limit: more than %d evaluations of the function; "+
				"loosen the tolerance, shorten the span or pick another method", budget)
		}
	}()
	fn()
	return nil
}

// Execute runs case ci of family fi with method mi on the arguments text and
// returns the result. A malformed arguments line is an error of the page; an
// error of the library is Result.Err, next to whatever it still returned.
func Execute(fi, ci, mi int, args string) (*Result, error) {
	c, method, err := Lookup(fi, ci, mi)
	if err != nil {
		return nil, err
	}
	a, err := ParseArgs(args, c.Params(method))
	if err != nil {
		return nil, err
	}
	r, err := c.solve(method, a)
	if err != nil {
		return nil, err
	}
	r.res.Ruby, r.res.Julia = headers(r.res.Ruby, r.res.Julia)
	r.res.Per, r.res.Reps = timeCall(r.bare)
	return r.res, nil
}

// Lookup resolves menu indexes, rejecting an index out of range.
func Lookup(fi, ci, mi int) (*Case, string, error) {
	if fi < 0 || fi >= len(Families) {
		return nil, "", errors.New("no such family")
	}
	f := &Families[fi]
	if ci < 0 || ci >= len(f.Cases) {
		return nil, "", errors.New("no such case")
	}
	c := &f.Cases[ci]
	if mi < 0 || mi >= len(c.Methods) {
		return nil, "", errors.New("no such method")
	}
	return c, c.Methods[mi], nil
}

// DefaultArgs is the arguments text of a case with a method: one parameter per
// line, at its default, with what it is after a "#" (which ParseArgs ignores).
func DefaultArgs(c *Case, method string) string {
	var lines []string
	for _, p := range c.Params(method) {
		lines = append(lines, p.Key+"="+p.Default+"  # "+p.Doc)
	}
	return strings.Join(lines, "\n")
}

// timeCall times bare. One call first; if it is short, enough more to fill
// about 20 ms, so a microsecond-scale call is not lost in the browser clock's
// granularity. The mean is returned.
func timeCall(bare func()) (time.Duration, int) {
	const window = 20 * time.Millisecond
	start := time.Now()
	bare()
	first := time.Since(start)
	if first >= window/2 {
		return first, 1
	}
	n := 1
	total := first
	for total < window && n < 20000 {
		t0 := time.Now()
		bare()
		total += time.Since(t0)
		n++
	}
	return total / time.Duration(n), n
}

// FormatDuration prints a duration with a unit that keeps 3 significant digits.
func FormatDuration(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return strconv.FormatInt(d.Nanoseconds(), 10) + " ns"
	case d < time.Millisecond:
		return fmt.Sprintf("%.3g µs", float64(d)/float64(time.Microsecond))
	case d < time.Second:
		return fmt.Sprintf("%.3g ms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.3g s", d.Seconds())
}

// G is the one number format of the page and of the generated Go programs:
// fmt's %v, the shortest text that reads back as the same float64.
func G(v float64) string { return fmt.Sprint(v) }

// Gs formats a slice as "[a b c]".
func Gs(v []float64) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = G(x)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// linspace is n evenly spaced points from lo to hi inclusive.
func linspace(lo, hi float64, n int) []float64 {
	if n < 2 {
		return []float64{lo}
	}
	x := make([]float64, n)
	for i := range x {
		x[i] = lo + (hi-lo)*float64(i)/float64(n-1)
	}
	return x
}

// finite reports whether v is a number a plot can place.
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// clip replaces what is beyond ±lim by NaN so the plot leaves a gap there.
func clip(y []float64, lim float64) []float64 {
	out := make([]float64, len(y))
	for i, v := range y {
		if math.Abs(v) > lim {
			v = math.NaN()
		}
		out[i] = v
	}
	return out
}

// errText is the library's own error message, "" for no error.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
