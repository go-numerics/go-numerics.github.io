// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

//go:build !js

package engine

import (
	"context"
	"fmt"
	"math"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// printed parses the "name = value" lines a program printed.
func printed(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(line, " = "); ok {
			m[k] = v
		}
	}
	return m
}

// numbers reads every number in a printed value ("[1.0, 2.5]", "3", "NaN").
func numbers(s string) ([]float64, bool) {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' || r == '[' || r == ']' })
	var out []float64
	for _, x := range f {
		v, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return nil, false
		}
		out = append(out, v)
	}
	return out, len(out) > 0
}

// agree reports whether a program's printed value is the page's, to abs + rel·|page|
// when both are numbers, exactly otherwise.
func agree(page, got string, abs, rel float64) bool {
	p, pok := numbers(page)
	g, gok := numbers(got)
	if !pok || !gok {
		return strings.TrimSpace(page) == strings.TrimSpace(got)
	}
	if len(p) != len(g) {
		return false
	}
	for i := range p {
		switch {
		case math.IsNaN(p[i]) && math.IsNaN(g[i]):
		case p[i] == g[i]:
		case math.Abs(p[i]-g[i]) <= abs+rel*math.Abs(p[i]):
		default:
			return false
		}
	}
	return true
}

// fusedSensitive are the rows that depend on the last bit of the function.
var fusedSensitive = map[string]bool{
	"f(root)": true, "f(x)": true, "iterations": true, "function calls": true, "function evaluations": true,
	"gradient evaluations": true, "subintervals": true, "right-hand side evaluations": true,
	"accepted steps": true, "perr": true, "popt": false,
}

type program struct {
	name string
	res  *Result
	code string
}

// programs runs every case and method and returns the code the page offers in
// one language (pick), the ones that are programs, and those that are reasons.
func programs(t *testing.T, pick func(*Result) string) (progs []program, reasons []string) {
	t.Helper()
	for fi, f := range Families {
		for ci, c := range f.Cases {
			for mi, m := range c.Methods {
				variants := []string{DefaultArgs(&c, m)}
				if f.Name == "Minimisation" {
					// The analytic gradient has no Ruby program; the finite-difference one does.
					variants = append(variants, strings.Replace(variants[0], "grad=analytic", "grad=fd", 1))
				}
				for _, args := range variants {
					r, err := Execute(fi, ci, mi, args)
					if err != nil {
						t.Fatal(err)
					}
					code := pick(r)
					name := fmt.Sprintf("p%03d %s/%s/%s", len(progs)+len(reasons), f.Name, c.Name, m)
					if HasProgram(code) {
						progs = append(progs, program{name, r, code})
					} else {
						reasons = append(reasons, name+": "+code)
					}
				}
			}
		}
	}
	return progs, reasons
}

func TestRubyPrintsWhatThePageShows(t *testing.T) {
	if testing.Short() {
		t.Skip("builds go-embedded-ruby")
	}
	dir := t.TempDir()
	rbgo := filepath.Join(dir, "rbgo")
	build := osexec.Command("go", "build", "-tags", "rbgo_core", "-o", rbgo, "github.com/go-embedded-ruby/ruby/cmd/rbgo")
	build.Dir = "../rubycheck"
	build.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go-embedded-ruby does not build: %v\n%s", err, out)
	}
	progs, reasons := programs(t, func(r *Result) string { return r.Ruby })
	// Which families have a Ruby program: everything but the ODEs, and within
	// each family only the options go-embedded-ruby binds.
	if len(progs) < 50 {
		t.Fatalf("only %d Ruby programs", len(progs))
	}
	for _, r := range reasons {
		if !strings.Contains(r, ": # No program: ") || len(r) < 60 {
			t.Errorf("a reason that says nothing: %q", r)
		}
	}
	for _, p := range progs {
		file := filepath.Join(dir, strings.Fields(p.name)[0]+".rb")
		if err := os.WriteFile(file, []byte(p.code), 0o644); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		out, err := osexec.CommandContext(ctx, rbgo, file).CombinedOutput()
		cancel()
		if p.res.Err != "" {
			// The library refuses; Ruby raises its message, not a number.
			if err == nil || !strings.Contains(string(out), p.res.Err) {
				t.Errorf("%s: the page shows the library's error %q, Ruby gave %v: %s", p.name, p.res.Err, err, out)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v\n%s\n%s", p.name, err, out, p.code)
			continue
		}
		got := printed(string(out))
		if runtime.GOARCH != "amd64" && strings.Contains(p.name, "Minimisation/Rosenbrock") {
			// A hundred fused multiply-adds per evaluation: the path of a
			// finite-difference minimiser is not the interpreter's. It ran.
			continue
		}
		for _, r := range p.res.Summary {
			// The same library, so the same numbers, the counts included. Not
			// on arm64: there the Go compiler fuses x*y + z into one rounding,
			// which the interpreter's separate operations do not, so the test
			// binary's own closures differ from Ruby's in the last bit, and a
			// path may take one step more. The page itself is not affected (the
			// wasm build does not fuse); amd64, which CI is, is exact.
			abs, rel := 0.0, 1e-9
			if runtime.GOARCH != "amd64" {
				if fusedSensitive[r.Name] {
					continue
				}
				rel = 1e-6
			}
			if g, ok := got[r.Name]; !ok || !agree(r.Value, g, abs, rel) {
				t.Errorf("%s: row %q: the page shows %q, Ruby prints %q", p.name, r.Name, r.Value, g)
			}
		}
	}
}

// TestJuliaAgreesWithThePage runs every Julia program on julia 1.12.6 with the
// packages testdata/juliaenv pins and compares the answer with the page's, to
// the tolerance of the problem: the packages' algorithms are not go-numerics'.
// PLAYGROUND_JULIA is the command (it may have arguments); JULIA_DEPOT_PATH
// names the depot with the packages installed.
func TestJuliaAgreesWithThePage(t *testing.T) {
	cmdline := strings.Fields(os.Getenv("PLAYGROUND_JULIA"))
	if len(cmdline) == 0 {
		if os.Getenv("PLAYGROUND_REQUIRE_JULIA") == "1" {
			t.Fatal("the Julia check is required but PLAYGROUND_JULIA is not set")
		}
		t.Skip("PLAYGROUND_JULIA is not set; set PLAYGROUND_REQUIRE_JULIA=1 to make this a failure")
	}
	progs, reasons := programs(t, func(r *Result) string { return r.Julia })
	if len(progs) < 80 {
		t.Fatalf("only %d Julia programs", len(progs))
	}
	for _, r := range reasons {
		if !strings.Contains(r, ": # No program: ") {
			t.Errorf("a reason that says nothing: %q", r)
		}
	}
	env, err := filepath.Abs("testdata/juliaenv")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var files []string
	for _, p := range progs {
		file := filepath.Join(dir, strings.Fields(p.name)[0]+".jl")
		if err := os.WriteFile(file, []byte(p.code), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	// One Julia process runs every program, each in its own module so that
	// their definitions do not meet.
	driver := `for f in ARGS
    println("### ", f)
    try
        m = Module(:Prog)
        Base.include(m, f)
    catch e
        println("ERROR: ", sprint(showerror, e))
    end
end
`
	drv := filepath.Join(dir, "driver.jl")
	if err := os.WriteFile(drv, []byte(driver), 0o644); err != nil {
		t.Fatal(err)
	}
	args := append(cmdline[1:], "--project="+env, drv)
	args = append(args, files...)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	out, err := osexec.CommandContext(ctx, cmdline[0], args...).CombinedOutput()
	if err != nil {
		t.Fatalf("julia: %v\n%s", err, out)
	}
	sections := map[string]string{}
	var cur string
	for _, line := range strings.Split(string(out), "\n") {
		if f, ok := strings.CutPrefix(line, "### "); ok {
			cur = filepath.Base(f)
			continue
		}
		sections[cur] += line + "\n"
	}
	// How closely each printed row must agree: abs, rel.
	tol := map[string][2]float64{
		"root": {1e-8, 0}, "x": {1e-3, 0}, "popt": {1e-6, 1e-4}, "perr": {0, 5e-2}, "value": {1e-6, 1e-8},
		"t at the end": {1e-12, 0}, "y at the end": {2e-3, 0}, "points": {0, 0},
	}
	for _, p := range progs {
		sec := sections[strings.Fields(p.name)[0]+".jl"]
		if strings.Contains(sec, "ERROR:") || sec == "" {
			t.Errorf("%s: %s\n%s", p.name, sec, p.code)
			continue
		}
		got := printed(sec)
		compared := 0
		for _, r := range p.res.Summary {
			g, ok := got[r.Name]
			if !ok {
				continue
			}
			abs, rel := 0.0, 1e-9
			if x, known := tol[r.Name]; known {
				abs, rel = x[0], x[1]
			} else if strings.Contains(r.Name, "f") && strings.HasPrefix(p.name[5:], "Special") {
				abs, rel = 1e-12, 1e-9
			}
			if r.Name == "f(root)" || r.Name == "f(x)" || r.Name == "estimated error" || r.Name == "accepted steps" {
				continue // not the same algorithm: the answer is compared, not its by-products
			}
			compared++
			if !agree(r.Value, g, abs, rel) {
				t.Errorf("%s: row %q: the page shows %q, Julia prints %q", p.name, r.Name, r.Value, g)
			}
		}
		if compared == 0 {
			t.Errorf("%s: Julia printed nothing comparable:\n%s", p.name, sec)
		}
	}
}
