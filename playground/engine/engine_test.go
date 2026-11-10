// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

// find resolves menu indexes by name.
func find(t *testing.T, family, name, method string) (fi, ci, mi int) {
	t.Helper()
	for i, f := range Families {
		if f.Name != family {
			continue
		}
		for j, c := range f.Cases {
			if c.Name != name {
				continue
			}
			for k, m := range c.Methods {
				if m == method {
					return i, j, k
				}
			}
		}
	}
	t.Fatalf("no %s / %s / %s", family, name, method)
	return
}

// exec runs a case and fails on an error of the page.
func exec(t *testing.T, family, name, method, args string) *Result {
	t.Helper()
	fi, ci, mi := find(t, family, name, method)
	r, err := Execute(fi, ci, mi, args)
	if err != nil {
		t.Fatalf("%s/%s/%s %q: %v", family, name, method, args, err)
	}
	return r
}

func row(rows []Row, name string) string {
	for _, r := range rows {
		if r.Name == name {
			return r.Value
		}
	}
	return "\x00missing " + name
}

func num(t *testing.T, rows []Row, name string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(row(rows, name), 64)
	if err != nil {
		t.Fatalf("row %q: %v (rows %v)", name, err, rows)
	}
	return v
}

func vec(t *testing.T, rows []Row, name string) []float64 {
	t.Helper()
	s := strings.Trim(row(rows, name), "[]")
	var out []float64
	for _, f := range strings.Fields(s) {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			t.Fatalf("row %q: %v", name, err)
		}
		out = append(out, v)
	}
	return out
}

func TestEveryCaseRuns(t *testing.T) {
	n := 0
	for fi, f := range Families {
		for ci, c := range f.Cases {
			for mi, m := range c.Methods {
				r, err := Execute(fi, ci, mi, DefaultArgs(&c, m))
				if err != nil {
					t.Errorf("%s/%s/%s: %v", f.Name, c.Name, m, err)
					continue
				}
				n++
				// Romberg wants a smooth integrand on a finite range: the library
				// says so, in its own words, on the four cases QUADPACK exists for.
				if r.Err != "" && !(m == "Romberg" && c.Name != "sin x") {
					t.Errorf("%s/%s/%s: the library said %s", f.Name, c.Name, m, r.Err)
				}
				if len(r.Summary) == 0 || len(r.Check) == 0 || len(r.Plots) == 0 || r.Go == "" || r.Python == "" || r.Title == "" {
					t.Errorf("%s/%s/%s: incomplete result", f.Name, c.Name, m)
				}
				if r.Per <= 0 || r.Reps < 1 {
					t.Errorf("%s/%s/%s: no timing", f.Name, c.Name, m)
				}
				for _, p := range r.Plots {
					if len(p.Series) == 0 || len(p.Series[0].X) == 0 || len(p.Series[0].X) != len(p.Series[0].Y) {
						t.Errorf("%s/%s/%s: plot %q has no first series", f.Name, c.Name, m, p.Name)
					}
				}
				if !strings.Contains(DefaultArgs(&c, m), "  # ") || c.Doc == "" {
					t.Errorf("%s/%s: no documented defaults", f.Name, c.Name)
				}
			}
		}
	}
	if n < 100 {
		t.Fatalf("only %d runs", n)
	}
}

// The page's answers against mathematics: closed forms, conserved quantities,
// identities. None of them compares the library with itself.

func TestRootsMatchTheirClosedForms(t *testing.T) {
	for _, c := range rootFamily.Cases {
		for _, m := range rootMethods {
			r := exec(t, "Scalar root", c.Name, m, "")
			ref := num(t, r.Check, "reference root")
			tol := 1e-9
			if !isBracketing(m) {
				tol = 1e-6 // Newton's default tolerance is 1.48e-8 on successive iterates
			}
			if d := num(t, r.Check, "|root − reference|"); d > tol {
				t.Errorf("%s %s: root off by %g (reference %g)", c.Name, m, d, ref)
			}
			if row(r.Summary, "converged") != "true" || !r.Converged {
				t.Errorf("%s %s: not converged: %s", c.Name, m, r.Status)
			}
		}
	}
	// The parameter of x² = c is the number whose root is sought.
	r := exec(t, "Scalar root", "x² = c", "Brentq", "c=9, a=0, b=10")
	if v := num(t, r.Summary, "root"); math.Abs(v-3) > 1e-11 {
		t.Errorf("sqrt 9 = %v", v)
	}
	// Brent is faster than bisection on a smooth function, by the counts.
	br := num(t, exec(t, "Scalar root", "cos x = x", "Brentq", "").Summary, "function calls")
	bi := num(t, exec(t, "Scalar root", "cos x = x", "Bisect", "").Summary, "function calls")
	if br >= bi {
		t.Errorf("Brentq took %v calls, Bisect %v", br, bi)
	}
}

func TestRootErrorsAreTheLibrarys(t *testing.T) {
	r := exec(t, "Scalar root", "cos x = x", "Brentq", "a=2, b=3")
	if !strings.Contains(r.Err, "sign") || len(r.Summary) != 0 {
		t.Errorf("no sign change: err %q summary %v", r.Err, r.Summary)
	}
	r = exec(t, "Scalar root", "cos x = x", "Bisect", "maxiter=3")
	if r.Err == "" && r.Converged {
		t.Errorf("3 bisections cannot converge: %+v", r.Summary)
	}
}

func TestMinimisersFindTheKnownMinima(t *testing.T) {
	for _, m := range minMethods {
		r := exec(t, "Minimisation", "Rosenbrock", m, "")
		if d := num(t, r.Check, "‖x − x*‖"); d > 1e-3 {
			t.Errorf("Rosenbrock %s: %g from (1, 1): %v", m, d, r.Summary)
		}
		if f := num(t, r.Check, "f(x) − f(x*)"); f > 1e-6 {
			t.Errorf("Rosenbrock %s: f = %g", m, f)
		}
		for _, c := range []string{"Himmelblau", "Booth", "Beale"} {
			r := exec(t, "Minimisation", c, m, "")
			if f := num(t, r.Check, "f(x) − f(x*)"); f > 1e-4 {
				t.Errorf("%s %s: f = %g, x = %s", c, m, f, row(r.Summary, "x"))
			}
		}
	}
	// Five dimensions, finite differences, and a start near another basin.
	r := exec(t, "Minimisation", "Rosenbrock", "L-BFGS-B", "x0=1.1;1.1;1.1;1.1;1.1, grad=fd")
	if d := num(t, r.Check, "‖x − x*‖"); d > 1e-3 {
		t.Errorf("5-D: %g", d)
	}
	r = exec(t, "Minimisation", "Himmelblau", "BFGS", "x0=-4;-4")
	ref := vec(t, r.Check, "nearest known minimiser")
	if math.Abs(ref[0]+3.779310253377747) > 1e-9 || math.Abs(ref[1]+3.2831859912861695) > 1e-9 {
		t.Errorf("the polished minimum is %v", ref)
	}
	// The analytic gradient saves the evaluations finite differences spend.
	an := num(t, exec(t, "Minimisation", "Rosenbrock", "BFGS", "").Summary, "function evaluations")
	fd := num(t, exec(t, "Minimisation", "Rosenbrock", "BFGS", "grad=fd").Summary, "function evaluations")
	if an >= fd {
		t.Errorf("analytic %v evaluations, finite differences %v", an, fd)
	}
}

func TestHimmelblauReferencesAreStationary(t *testing.T) {
	for _, m := range himmelblau.minima {
		g := make([]float64, 2)
		himmelblau.grad(m, g)
		if math.Hypot(g[0], g[1]) > 1e-12 || himmelblau.f(m) > 1e-20 {
			t.Errorf("%v: gradient %v, f %g", m, g, himmelblau.f(m))
		}
	}
}

func TestGradientsMatchFiniteDifferences(t *testing.T) {
	for _, p := range []minProblem{rosenbrock, himmelblau, booth, beale} {
		x := []float64{0.3, -0.7}
		g := make([]float64, 2)
		p.grad(x, g)
		for i := range x {
			h := 1e-6
			xp, xm := append([]float64(nil), x...), append([]float64(nil), x...)
			xp[i] += h
			xm[i] -= h
			fd := (p.f(xp) - p.f(xm)) / (2 * h)
			if math.Abs(fd-g[i]) > 1e-5*math.Max(1, math.Abs(g[i])) {
				t.Errorf("%s ∂%d: analytic %g, central difference %g", p.name, i, g[i], fd)
			}
		}
	}
}

func TestCurveFitRecoversTheTruth(t *testing.T) {
	for _, c := range fitFamily.Cases {
		for _, m := range fitMethods {
			clean := exec(t, "Curve fit", c.Name, m, "noise=0")
			if d := num(t, clean.Check, "max |popt − true|"); d > 1e-6 {
				t.Errorf("%s %s without noise: off by %g", c.Name, m, d)
			}
			noisy := exec(t, "Curve fit", c.Name, m, "")
			if z := num(t, noisy.Check, "max |popt − true| / perr"); z > 4 {
				t.Errorf("%s %s: %g standard errors from the truth", c.Name, m, z)
			}
			rms := num(t, noisy.Check, "rms residual")
			if rms < 0.02 || rms > 0.08 {
				t.Errorf("%s %s: rms residual %g for noise 0.05", c.Name, m, rms)
			}
		}
	}
	// The same seed is the same data; another seed is not.
	a := exec(t, "Curve fit", "exponential decay", "LM", "seed=7").Summary
	b := exec(t, "Curve fit", "exponential decay", "LM", "seed=7").Summary
	c := exec(t, "Curve fit", "exponential decay", "LM", "seed=8").Summary
	if row(a, "popt") != row(b, "popt") || row(a, "popt") == row(c, "popt") {
		t.Errorf("seeding: %s %s %s", row(a, "popt"), row(b, "popt"), row(c, "popt"))
	}
}

func TestQuadratureAgainstAntiderivatives(t *testing.T) {
	for _, c := range quadFamily.Cases {
		r := exec(t, "Quadrature", c.Name, "Quad", "")
		if d := num(t, r.Check, "|value − reference|"); d > 1e-8 {
			t.Errorf("%s: |value − reference| = %g", c.Name, d)
		}
		if row(r.Check, "error within estimate") != "yes" {
			t.Errorf("%s: the true error exceeds the estimate: %v", c.Name, r.Check)
		}
	}
	if v := num(t, exec(t, "Quadrature", "sin x", "Quad", "").Summary, "value"); math.Abs(v-2) > 1e-14 {
		t.Errorf("∫ sin = %v", v)
	}
	if v := num(t, exec(t, "Quadrature", "e^(−x²) over the line", "Quad", "").Summary, "value"); math.Abs(v-math.Sqrt(math.Pi)) > 1e-8 {
		t.Errorf("Gaussian integral = %v", v)
	}
	// Telling QUADPACK where the kink is saves evaluations.
	without := num(t, exec(t, "Quadrature", "step at 1/3", "Quad", "").Summary, "function evaluations")
	with := num(t, exec(t, "Quadrature", "step at 1/3", "Quad", "points=0.3333333333333333").Summary, "function evaluations")
	if with >= without {
		t.Errorf("with the break point %v evaluations, without %v", with, without)
	}
	// Fixed Gauss–Legendre of 5 nodes is exact for sin on [0, π] to 1e-4 only.
	r := exec(t, "Quadrature", "sin x", "FixedQuad", "")
	if d := num(t, r.Check, "|value − reference|"); d > 1e-3 || d == 0 {
		t.Errorf("FixedQuad(5): %g", d)
	}
	if n := num(t, r.Summary, "function evaluations"); n != 5 {
		t.Errorf("FixedQuad(5) evaluated %v times", n)
	}
	r = exec(t, "Quadrature", "sin x", "Romberg", "")
	if d := num(t, r.Check, "|value − reference|"); d > 1e-7 {
		t.Errorf("Romberg: %g", d)
	}
	// Limits the antiderivative is not defined at.
	r = exec(t, "Quadrature", "sin x", "Quad", "a=-inf, b=inf, limit=5")
	if !strings.Contains(row(r.Check, "reference"), "not defined") {
		t.Errorf("sin over the line: %v", r.Check)
	}
}

func TestQuadratureErrorsAreTheLibrarys(t *testing.T) {
	r := exec(t, "Quadrature", "1/√x", "Romberg", "")
	if r.Err == "" {
		t.Errorf("Romberg on a singularity succeeded: %v", r.Summary)
	}
	r = exec(t, "Quadrature", "step at 1/3", "Quad", "limit=1, epsabs=1e-14, epsrel=1e-14")
	if r.Err == "" || len(r.Summary) == 0 || r.Converged {
		t.Errorf("one subinterval cannot reach 1e-14: err %q, converged %v", r.Err, r.Converged)
	}
}

func TestODEsAgainstInvariantsAndClosedForms(t *testing.T) {
	for _, m := range odeMethods {
		lv := exec(t, "ODE", "Lotka–Volterra", m, "rtol=1e-8, atol=1e-10")
		if d := num(t, lv.Check, "max drift of V"); d > 1e-4 {
			t.Errorf("LV %s: V drifts by %g", m, d)
		}
		st := exec(t, "ODE", "stiff relaxation", m, "rtol=1e-8, atol=1e-10")
		if d := num(t, st.Check, "max |y - cos t|"); d > 1e-4 {
			t.Errorf("stiff %s: off cos t by %g", m, d)
		}
		vp := exec(t, "ODE", "Van der Pol", m, "rtol=1e-9, atol=1e-11")
		if d := num(t, vp.Check, "max |y - ref| at end"); d > 1e-3 {
			t.Errorf("Van der Pol %s: %g from the reference", m, d)
		}
	}
	// What stiffness is: the explicit method needs far more steps than Radau.
	rk := num(t, exec(t, "ODE", "stiff relaxation", "RK45", "lambda=10000").Summary, "accepted steps")
	ra := num(t, exec(t, "ODE", "stiff relaxation", "Radau", "lambda=10000").Summary, "accepted steps")
	if rk < 20*ra {
		t.Errorf("RK45 %v steps, Radau %v", rk, ra)
	}
	// A large μ switches the reference to Radau.
	r := exec(t, "ODE", "Van der Pol", "Radau", "mu=100, tend=200")
	if !strings.Contains(r.Check[0].Name, "Radau") {
		t.Errorf("reference for μ = 100: %v", r.Check)
	}
}

func TestSpecialFunctionIdentities(t *testing.T) {
	for _, c := range specialFamily.Cases {
		for _, m := range c.Methods {
			r := exec(t, "Special functions", c.Name, m, "")
			var worst float64
			for _, cr := range r.Check {
				if cr.Name == "max residual" {
					worst, _ = strconv.ParseFloat(cr.Value, 64)
					if n, _ := strconv.Atoi(row(r.Check, "points tested")); n < 100 {
						t.Errorf("%s: only %s points tested", c.Name, row(r.Check, "points tested"))
					}
				}
			}
			if worst > 1e-12 {
				t.Errorf("%s %s: identity residual %g", c.Name, m, worst)
			}
			if strings.HasPrefix(m, "vector") {
				if d := num(t, r.Check, "max |vector − scalar|"); d > 1e-12*math.Max(1, num(t, r.Summary, "max f (finite points)")) {
					t.Errorf("%s: vector differs from scalar by %g", c.Name, d)
				}
			}
		}
	}
	// Known values.
	r := exec(t, "Special functions", "Γ(x)", "scalar", "lo=0.5, hi=0.5, n=2")
	if v := num(t, r.Summary, "f(first x)"); math.Abs(v-math.Sqrt(math.Pi)) > 1e-15 {
		t.Errorf("Γ(1/2) = %v", v)
	}
	r = exec(t, "Special functions", "K(m)", "scalar", "lo=0, hi=0, n=2")
	if v := num(t, r.Summary, "f(first x)"); math.Abs(v-math.Pi/2) > 1e-15 {
		t.Errorf("K(0) = %v", v)
	}
}

func TestAGM(t *testing.T) {
	// Gauss's constant: 1/AGM(1, √2) = 0.8346268416740731…
	if g := 1 / agm(1, math.Sqrt2); math.Abs(g-0.8346268416740731) > 1e-15 {
		t.Errorf("Gauss's constant %v", g)
	}
}

func TestBudgetStopsARunawayRun(t *testing.T) {
	fi, ci, mi := find(t, "ODE", "stiff relaxation", "RK45")
	start := time.Now()
	_, err := Execute(fi, ci, mi, "lambda=1e7, tend=1000")
	if err == nil || !strings.Contains(err.Error(), "playground limit") {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 30*time.Second {
		t.Errorf("the limit took %v to fire", time.Since(start))
	}
	// Every family's limit is the same guard.
	fi, ci, mi = find(t, "Minimisation", "Rosenbrock", "Nelder-Mead")
	if _, err := Execute(fi, ci, mi, "x0=-1.2;1, tol=1e-300, maxiter=100000000"); err != nil &&
		!strings.Contains(err.Error(), "playground limit") {
		t.Errorf("unexpected error %v", err)
	}
}

func TestTheLimitGuardsEveryFamily(t *testing.T) {
	defer func(old int) { budget = old }(budget)
	budget = 10
	for _, c := range []struct{ fam, name, method string }{
		{"Scalar root", "cos x = x", "Bisect"},
		{"Minimisation", "Rosenbrock", "Nelder-Mead"},
		{"Curve fit", "exponential decay", "LM"},
		{"Quadrature", "sin x", "FixedQuad"},
		{"Quadrature", "sin x", "Quad"},
		{"ODE", "stiff relaxation", "RK45"},
	} {
		fi, ci, mi := find(t, c.fam, c.name, c.method)
		args := ""
		if c.fam == "Quadrature" && c.method == "FixedQuad" {
			args = "n=100"
		}
		if _, err := Execute(fi, ci, mi, args); err == nil || !strings.Contains(err.Error(), "playground limit") {
			t.Errorf("%s/%s/%s: %v", c.fam, c.name, c.method, err)
		}
	}
	// The Van der Pol reference stops at the limit too, and says so in its row.
	budget = 400
	r := exec(t, "ODE", "Van der Pol", "RK45", "mu=1, tend=2, rtol=1e-3")
	if got := r.Check[0].Value; !strings.Contains(got, "not computed: playground limit") {
		t.Errorf("reference row: %q (%d steps)", got, len(r.Summary))
	}
}

func TestLibraryErrorsComeBackWithTheirResults(t *testing.T) {
	r := exec(t, "Minimisation", "Rosenbrock", "BFGS", "maxiter=2")
	if r.Converged || len(r.Summary) == 0 {
		t.Errorf("two iterations cannot minimise Rosenbrock: %v", r.Summary)
	}
	// The ODE solver refuses a negative tolerance; it fails on a NaN right-hand
	// side after some steps, and so does the reference run that follows.
	r = exec(t, "ODE", "stiff relaxation", "RK45", "tend=nan")
	if r.Err == "" || len(r.Summary) != 0 {
		t.Errorf("tend=nan: err %q, summary %v", r.Err, r.Summary)
	}
	r = exec(t, "ODE", "Van der Pol", "RK45", "mu=nan")
	if r.Err == "" || !strings.Contains(r.Check[0].Value, "not computed") {
		t.Errorf("mu=nan: err %q, check %v", r.Err, r.Check)
	}
	// MINPACK gives up after one evaluation, with the fit it has.
	r = exec(t, "Curve fit", "exponential decay", "LM", "maxfev=1")
	if r.Err == "" && r.Converged {
		t.Errorf("one evaluation cannot fit: %v", r.Summary)
	}
	r = exec(t, "Minimisation", "Booth", "BFGS", "x0=nan;1")
	if r.Err == "" && r.Converged {
		t.Errorf("NaN start: %v", r.Summary)
	}
	// Equal limits: an empty range, drawn on a unit window.
	r = exec(t, "Quadrature", "sin x", "Quad", "a=1, b=1")
	if v := num(t, r.Summary, "value"); v != 0 {
		t.Errorf("∫1¹ = %v", v)
	}
	// Limits the wrong way round.
	r = exec(t, "Quadrature", "sin x", "Quad", "a=pi, b=0")
	if v := num(t, r.Summary, "value"); math.Abs(v+2) > 1e-14 {
		t.Errorf("∫π⁰ sin = %v", v)
	}
}

func TestArgumentsTextTakesLinesCommasAndComments(t *testing.T) {
	params := []Param{{"a", "1", "first"}, {"b", "2", "second"}, {"c", "3", "third"}}
	a, err := ParseArgs("a=10  # the first\n   \nb = 20, c=30 # c, with a comma, is a comment\n", params)
	if err != nil {
		t.Fatal(err)
	}
	if x, _ := a.Float("a"); x != 10 {
		t.Errorf("a = %v", x)
	}
	if x, _ := a.Float("b"); x != 20 {
		t.Errorf("b = %v", x)
	}
	if x, _ := a.Float("c"); x != 30 {
		t.Errorf("c = %v", x)
	}
	// The defaults text parses back to the defaults.
	c := &Case{Params: func(string) []Param { return params }}
	d, err := ParseArgs(DefaultArgs(c, ""), params)
	if err != nil {
		t.Fatal(err)
	}
	if x, _ := d.Float("c"); x != 3 {
		t.Errorf("default c = %v", x)
	}
}

func TestPageErrors(t *testing.T) {
	bad := []struct{ fam, name, method, args, want string }{
		{"Scalar root", "cos x = x", "Brentq", "zz=1", "unknown parameter"},
		{"Scalar root", "cos x = x", "Brentq", "a", "not key=value"},
		{"Scalar root", "cos x = x", "Brentq", "a=1, a=2", "twice"},
		{"Scalar root", "cos x = x", "Brentq", "a=x", "not a number"},
		{"Scalar root", "cos x = x", "Brentq", "a=1/0", "not a number"},
		{"Scalar root", "cos x = x", "Brentq", "a=zpi", "not a number"},
		{"Scalar root", "cos x = x", "Brentq", "b=1/x", "not a number"},
		{"Scalar root", "cos x = x", "Brentq", "maxiter=2.5", "whole number"},
		{"Scalar root", "cos x = x", "Brentq", "b=zz", "parameter b"},
		{"Scalar root", "cos x = x", "Brentq", "xtol=zz", "parameter xtol"},
		{"Scalar root", "cos x = x", "Brentq", "rtol=zz", "parameter rtol"},
		{"Scalar root", "cos x = x", "Brentq", "maxiter=zz", "parameter maxiter"},
		{"Scalar root", "cos x = x", "Newton", "x0=zz", "parameter x0"},
		{"Scalar root", "cos x = x", "Newton", "tol=zz", "parameter tol"},
		{"Scalar root", "x² = c", "Brentq", "c=zz", "parameter c"},
		{"Minimisation", "Booth", "BFGS", "x0=1", "need between 2 and 200"},
		{"Minimisation", "Booth", "BFGS", "x0=1;2;3", "takes 2 values"},
		{"Minimisation", "Booth", "BFGS", "x0=1;z", "parameter x0"},
		{"Minimisation", "Booth", "BFGS", "grad=maybe", "neither analytic nor fd"},
		{"Minimisation", "Booth", "BFGS", "tol=z", "parameter tol"},
		{"Minimisation", "Booth", "BFGS", "maxiter=z", "parameter maxiter"},
		{"Curve fit", "Gaussian peak", "LM", "n=2", "outside 5 to 5000"},
		{"Curve fit", "Gaussian peak", "LM", "n=z", "parameter n"},
		{"Curve fit", "Gaussian peak", "LM", "noise=z", "parameter noise"},
		{"Curve fit", "Gaussian peak", "LM", "seed=-1", "negative"},
		{"Curve fit", "Gaussian peak", "LM", "seed=z", "parameter seed"},
		{"Curve fit", "Gaussian peak", "LM", "p0=1;2", "takes 3 values"},
		{"Curve fit", "Gaussian peak", "LM", "p0=1;2;z", "parameter p0"},
		{"Curve fit", "Gaussian peak", "LM", "maxfev=z", "parameter maxfev"},
		{"Quadrature", "sin x", "Quad", "a=z", "parameter a"},
		{"Quadrature", "sin x", "Quad", "b=z", "parameter b"},
		{"Quadrature", "sin x", "Quad", "epsabs=z", "parameter epsabs"},
		{"Quadrature", "sin x", "Quad", "epsrel=z", "parameter epsrel"},
		{"Quadrature", "sin x", "Quad", "limit=z", "parameter limit"},
		{"Quadrature", "sin x", "Quad", "points=z", "parameter points"},
		{"Quadrature", "sin x", "Romberg", "tol=z", "parameter tol"},
		{"Quadrature", "sin x", "Romberg", "rtol=z", "parameter rtol"},
		{"Quadrature", "sin x", "Romberg", "divmax=z", "parameter divmax"},
		{"Quadrature", "sin x", "FixedQuad", "n=z", "parameter n"},
		{"ODE", "Lotka–Volterra", "RK45", "a=z", "parameter a"},
		{"ODE", "Van der Pol", "RK45", "tend=z", "parameter tend"},
		{"ODE", "stiff relaxation", "RK45", "lambda=z", "parameter lambda"},
		{"ODE", "stiff relaxation", "RK45", "rtol=z", "parameter rtol"},
		{"ODE", "stiff relaxation", "RK45", "atol=z", "parameter atol"},
		{"Special functions", "Γ(x)", "scalar", "lo=z", "parameter lo"},
		{"Special functions", "Γ(x)", "scalar", "hi=z", "parameter hi"},
		{"Special functions", "Γ(x)", "scalar", "n=z", "parameter n"},
		{"Special functions", "Γ(x)", "scalar", "n=1", "outside 2 to 200000"},
		{"Special functions", "Γ(x)", "scalar", "ylim=z", "parameter ylim"},
	}
	for _, b := range bad {
		fi, ci, mi := find(t, b.fam, b.name, b.method)
		_, err := Execute(fi, ci, mi, b.args)
		if err == nil || !strings.Contains(err.Error(), b.want) {
			t.Errorf("%s/%s %q: error %v, want %q", b.fam, b.name, b.args, err, b.want)
		}
	}
	for _, idx := range [][3]int{{-1, 0, 0}, {99, 0, 0}, {0, -1, 0}, {0, 99, 0}, {0, 0, -1}, {0, 0, 99}} {
		if _, err := Execute(idx[0], idx[1], idx[2], ""); err == nil {
			t.Errorf("Execute%v succeeded", idx)
		}
	}
}

func TestNumbersAndFormats(t *testing.T) {
	for in, want := range map[string]float64{
		"1": 1, " -2.5 ": -2.5, "1e-3": 1e-3, "pi": math.Pi, "-pi": -math.Pi, "2pi": 2 * math.Pi,
		"2*pi": 2 * math.Pi, "pi/2": math.Pi / 2, "3/4": 0.75, "-pi/4": -math.Pi / 4,
	} {
		if got, err := parseNumber(in); err != nil || got != want {
			t.Errorf("parseNumber(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if v, err := parseNumber("-inf"); err != nil || !math.IsInf(v, -1) {
		t.Errorf("-inf: %v %v", v, err)
	}
	for d, want := range map[time.Duration]string{
		0: "0 ns", 420: "420 ns", 1500: "1.5 µs", 2300 * time.Microsecond: "2.3 ms", 3 * time.Second: "3 s",
	} {
		if got := FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", d, got, want)
		}
	}
	if G(0.1) != "0.1" || Gs([]float64{1, 2.5}) != "[1 2.5]" || boolText(true) != "yes" || boolText(false) != "no" {
		t.Error("formatters")
	}
	if got := goNum(math.NaN()) + goNum(math.Inf(1)) + goNum(math.Inf(-1)) + goNums([]float64{1, 2}); got != "math.NaN()math.Inf(1)math.Inf(-1)[]float64{1, 2}" {
		t.Errorf("goNum: %s", got)
	}
	if got := pyNum(math.NaN()) + pyNum(math.Inf(1)) + pyNum(math.Inf(-1)) + pyNums([]float64{1, 2}); got != "np.nannp.inf-np.inf[1, 2]" {
		t.Errorf("pyNum: %s", got)
	}
	if goSub(4, -4) != "4 - -4" || goSub(4, 1) != "4-1" {
		t.Errorf("goSub: %s %s", goSub(4, -4), goSub(4, 1))
	}
	if x := linspace(0, 1, 1); len(x) != 1 || x[0] != 0 {
		t.Errorf("linspace of one point: %v", x)
	}
	if c := clip([]float64{1, 100, math.Inf(1)}, 10); c[0] != 1 || !math.IsNaN(c[1]) || !math.IsNaN(c[2]) {
		t.Errorf("clip: %v", c)
	}
	if _, ok := relDiff(1, 2, 0); ok {
		t.Error("a zero scale is testable")
	}
	if _, ok := relDiff(math.NaN(), 2, 1); ok {
		t.Error("NaN is testable")
	}
}

func TestTimingFillsItsWindow(t *testing.T) {
	calls := 0
	per, reps := timeCall(func() { calls++ })
	if reps != calls || reps < 1000 || per > time.Millisecond {
		t.Errorf("reps %d calls %d per %v", reps, calls, per)
	}
	per, reps = timeCall(func() { time.Sleep(15 * time.Millisecond) })
	if reps != 1 || per < 15*time.Millisecond {
		t.Errorf("a long call is timed once: %d %v", reps, per)
	}
}

func TestGuardRethrowsOtherPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a panic that is not the budget's was swallowed")
		}
	}()
	_ = guard(func() { panic("boom") })
}

func TestTapPassesThroughWhenNil(t *testing.T) {
	var tp *tap
	f := func(x float64) float64 { return x + 1 }
	if tp.f1(f)(1) != 2 || tp.fv(func(x []float64) float64 { return x[0] })([]float64{3}) != 3 {
		t.Error("nil tap changed the function")
	}
	rec := &tap{}
	g := rec.f1(f)
	for i := 0; i < maxRecorded+5; i++ {
		g(float64(i))
	}
	if len(rec.xs) != maxRecorded || rec.n != maxRecorded+5 {
		t.Errorf("recorded %d of %d", len(rec.xs), rec.n)
	}
}
