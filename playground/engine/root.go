// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-numerics/optimize"
)

// rootFn is the function of a scalar root case at one setting of its
// parameters: the Go closures the page runs, the same expressions as source
// text for the generated programs, and the root known in closed form.
type rootFn struct {
	f, df    func(float64) float64
	goF, goD string // expressions in x, Go
	pyF, pyD string // expressions in x, NumPy
	exact    float64
}

type rootCase struct {
	name, doc string
	extra     []Param // parameters of the function itself
	build     func(a *Args) (*rootFn, error)
	a, b, x0  string  // default bracket and start
	vlo, vhi  float64 // the plot's window at least
}

var rootMethods = []string{"Brentq", "Ridder", "Brenth", "Bisect", "Newton", "Secant"}

var rootFamily = Family{Name: "Scalar root", Cases: []Case{
	rootCase{
		name: "cos x = x", doc: "the Dottie number",
		a: "0", b: "1", x0: "1", vlo: -0.5, vhi: 1.5,
		build: func(*Args) (*rootFn, error) {
			return &rootFn{
				f:   func(x float64) float64 { return math.Cos(x) - x },
				df:  func(x float64) float64 { return -math.Sin(x) - 1 },
				goF: "math.Cos(x) - x", goD: "-math.Sin(x) - 1",
				pyF: "np.cos(x) - x", pyD: "-np.sin(x) - 1",
				exact: 0.7390851332151607,
			}, nil
		},
	}.make(),
	rootCase{
		name: "x^3 - 2x - 5 = 0", doc: "Wallis's cubic, the equation Newton solved",
		a: "2", b: "3", x0: "2", vlo: 1.5, vhi: 3.5,
		build: func(*Args) (*rootFn, error) {
			return &rootFn{
				f:   func(x float64) float64 { return x*x*x - 2*x - 5 },
				df:  func(x float64) float64 { return 3*x*x - 2 },
				goF: "x*x*x - 2*x - 5", goD: "3*x*x - 2",
				pyF: "x**3 - 2*x - 5", pyD: "3*x**2 - 2",
				exact: 2.0945514815423265,
			}, nil
		},
	}.make(),
	rootCase{
		name: "x eˣ = 1", doc: "the omega constant, W(1)",
		a: "0", b: "1", x0: "1", vlo: -0.5, vhi: 1.2,
		build: func(*Args) (*rootFn, error) {
			return &rootFn{
				f:   func(x float64) float64 { return x*math.Exp(x) - 1 },
				df:  func(x float64) float64 { return (1 + x) * math.Exp(x) },
				goF: "x*math.Exp(x) - 1", goD: "(1 + x) * math.Exp(x)",
				pyF: "x*np.exp(x) - 1", pyD: "(1 + x)*np.exp(x)",
				exact: 0.5671432904097838,
			}, nil
		},
	}.make(),
	rootCase{
		name: "tan x = x", doc: "first positive root, beside the pole at 3π/2",
		a: "4.4", b: "4.6", x0: "4.5", vlo: 4.3, vhi: 4.65,
		build: func(*Args) (*rootFn, error) {
			return &rootFn{
				f:   func(x float64) float64 { return math.Tan(x) - x },
				df:  func(x float64) float64 { t := math.Tan(x); return t * t },
				goF: "math.Tan(x) - x", goD: "math.Tan(x) * math.Tan(x)",
				pyF: "np.tan(x) - x", pyD: "np.tan(x)**2",
				exact: 4.493409457909064,
			}, nil
		},
	}.make(),
	rootCase{
		name: "x² = c", doc: "the square root of c",
		extra: []Param{{"c", "2", "the number whose root is sought"}},
		a:     "0", b: "2", x0: "1", vlo: -0.5, vhi: 2.5,
		build: func(a *Args) (*rootFn, error) {
			c, err := a.Float("c")
			if err != nil {
				return nil, err
			}
			return &rootFn{
				f:   func(x float64) float64 { return x*x - c },
				df:  func(x float64) float64 { return 2 * x },
				goF: "x*x - " + goNum(c), goD: "2 * x",
				pyF: "x**2 - " + pyNum(c), pyD: "2*x",
				exact: math.Sqrt(c),
			}, nil
		},
	}.make(),
}}

func (rc rootCase) make() Case {
	return Case{Name: rc.name, Doc: rc.doc, Methods: rootMethods, Params: rc.params, solve: rc.solve}
}

func isBracketing(method string) bool { return method != "Newton" && method != "Secant" }

func (rc rootCase) params(method string) []Param {
	ps := append([]Param(nil), rc.extra...)
	if isBracketing(method) {
		return append(ps,
			Param{"a", rc.a, "left end of the bracket"},
			Param{"b", rc.b, "right end; f(a), f(b) must differ in sign"},
			Param{"xtol", "2e-12", "absolute tolerance"},
			Param{"rtol", "8.881784197001252e-16", "relative tolerance"},
			Param{"maxiter", "100", "iteration limit"})
	}
	return append(ps,
		Param{"x0", rc.x0, "starting point"},
		Param{"tol", "1.48e-8", "absolute tolerance on successive iterates"},
		Param{"rtol", "0", "relative tolerance"},
		Param{"maxiter", "50", "iteration limit"})
}

func (rc rootCase) solve(method string, a *Args) (*run, error) {
	fn, err := rc.build(a)
	if err != nil {
		return nil, err
	}
	var lo, hi, x0, tol, rtol float64
	var maxiter int
	if isBracketing(method) {
		if lo, err = a.Float("a"); err != nil {
			return nil, err
		}
		if hi, err = a.Float("b"); err != nil {
			return nil, err
		}
		if tol, err = a.Float("xtol"); err != nil {
			return nil, err
		}
	} else {
		if x0, err = a.Float("x0"); err != nil {
			return nil, err
		}
		if tol, err = a.Float("tol"); err != nil {
			return nil, err
		}
	}
	if rtol, err = a.Float("rtol"); err != nil {
		return nil, err
	}
	if maxiter, err = a.Int("maxiter"); err != nil {
		return nil, err
	}

	call := func(t *tap) (*optimize.RootScalarResult, error) {
		f := t.f1(fn.f)
		switch method {
		case "Brentq":
			return optimize.Brentq(f, lo, hi, &optimize.BracketOptions{Xtol: tol, Rtol: rtol, MaxIter: maxiter})
		case "Ridder":
			return optimize.Ridder(f, lo, hi, &optimize.BracketOptions{Xtol: tol, Rtol: rtol, MaxIter: maxiter})
		case "Brenth":
			return optimize.Brenth(f, lo, hi, &optimize.BracketOptions{Xtol: tol, Rtol: rtol, MaxIter: maxiter})
		case "Bisect":
			return optimize.Bisect(f, lo, hi, &optimize.BracketOptions{Xtol: tol, Rtol: rtol, MaxIter: maxiter})
		case "Newton":
			return optimize.Newton(f, x0, &optimize.NewtonOptions{Fprime: fn.df, Tol: tol, Rtol: rtol, MaxIter: maxiter})
		}
		return optimize.Newton(f, x0, &optimize.NewtonOptions{Tol: tol, Rtol: rtol, MaxIter: maxiter}) // Secant
	}

	t := &tap{}
	var res *optimize.RootScalarResult
	var cerr error
	if err := guard(func() { res, cerr = call(t) }); err != nil {
		return nil, err
	}
	out := &Result{Title: rc.name + " — " + method}
	if cerr != nil {
		out.Err = cerr.Error()
	}
	if res != nil {
		out.Status, out.Converged = res.Flag, res.Converged
		fr := fn.f(res.Root)
		out.Summary = []Row{
			{"root", G(res.Root)}, {"f(root)", G(fr)},
			{"iterations", fmt.Sprint(res.Iterations)}, {"function calls", fmt.Sprint(res.FunctionCalls)},
			{"converged", fmt.Sprint(res.Converged)}, {"flag", res.Flag},
		}
		out.Check = []Row{
			{"reference root", G(fn.exact)},
			{"|root − reference|", G(math.Abs(res.Root - fn.exact))},
			{"|f(root)|", G(math.Abs(fr))},
		}
		out.Plots = rc.plots(fn, method, lo, hi, x0, res.Root, t)
	}
	out.Go, out.Python = rc.code(fn, method, lo, hi, x0, tol, rtol, maxiter)
	out.Ruby, out.Julia = rc.rubyJulia(fn, method, lo, hi, x0, tol, rtol, maxiter)
	return &run{res: out, bare: func() { _, _ = call(nil) }}, nil
}

// plots draws f over a window holding the default view, the bracket or start
// and the root, with the points the solver evaluated, then log|f| at each.
func (rc rootCase) plots(fn *rootFn, method string, lo, hi, x0, root float64, t *tap) []Plot {
	wlo, whi := rc.vlo, rc.vhi
	for _, v := range []float64{lo, hi, x0, root} {
		if finite(v) {
			wlo, whi = math.Min(wlo, v), math.Max(whi, v)
		}
	}
	pad := (whi - wlo) * 0.05
	wlo, whi = wlo-pad, whi+pad
	xs := linspace(wlo, whi, 400)
	ys := make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = fn.f(x)
	}
	curve := Series{Label: "f(x)", X: xs, Y: clip(ys, 1e6), Raw: ys, RawName: "f"}
	var ex, ey []float64
	for i, x := range t.xs {
		if x >= wlo && x <= whi && finite(t.ys[i]) && math.Abs(t.ys[i]) <= 1e6 {
			ex, ey = append(ex, x), append(ey, t.ys[i])
		}
	}
	fp := Plot{Name: "f and the points it was evaluated at", XLabel: "x", YLabel: "f(x)", Series: []Series{
		curve,
		{Label: "evaluations", X: ex, Y: ey, Style: Markers},
		{Label: "root", X: []float64{root}, Y: []float64{fn.f(root)}, Style: Markers},
	}}
	k := make([]float64, 0, len(t.ys))
	lg := make([]float64, 0, len(t.ys))
	raw := make([]float64, 0, len(t.ys))
	for i, y := range t.ys {
		k = append(k, float64(i+1))
		lg = append(lg, math.Log10(math.Max(math.Abs(y), 1e-300)))
		raw = append(raw, y)
	}
	cp := Plot{Name: "log10 |f| at each evaluation", XLabel: "evaluation", YLabel: "log10 |f|", Series: []Series{
		{Label: "log10 |f|", X: k, Y: lg, Raw: raw, RawName: "f"},
	}}
	return []Plot{fp, cp}
}

var rootGoName = map[string]string{"Brentq": "Brentq", "Ridder": "Ridder", "Brenth": "Brenth", "Bisect": "Bisect"}

func (rc rootCase) code(fn *rootFn, method string, lo, hi, x0, tol, rtol float64, maxiter int) (string, string) {
	var g, p strings.Builder
	fmt.Fprintf(&g, "f := func(x float64) float64 { return %s }\n", fn.goF)
	switch method {
	case "Newton", "Secant":
		opts := fmt.Sprintf("Tol: %s, Rtol: %s, MaxIter: %d", goNum(tol), goNum(rtol), maxiter)
		if method == "Newton" {
			fmt.Fprintf(&g, "fprime := func(x float64) float64 { return %s }\n", fn.goD)
			opts = "Fprime: fprime, " + opts
		}
		fmt.Fprintf(&g, "res, err := optimize.Newton(f, %s, &optimize.NewtonOptions{%s})\n", goNum(x0), opts)
	default:
		fmt.Fprintf(&g, "res, err := optimize.%s(f, %s, %s, &optimize.BracketOptions{Xtol: %s, Rtol: %s, MaxIter: %d})\n",
			rootGoName[method], goNum(lo), goNum(hi), goNum(tol), goNum(rtol), maxiter)
	}
	g.WriteString("if err != nil {\n\tfmt.Println(\"error:\", err)\n}\nif res == nil {\n\treturn\n}\n")
	g.WriteString(printRow("root", "res.Root"))
	g.WriteString(printRow("f(root)", "f(res.Root)"))
	g.WriteString(printRow("iterations", "res.Iterations"))
	g.WriteString(printRow("function calls", "res.FunctionCalls"))
	g.WriteString(printRow("converged", "res.Converged"))
	g.WriteString(printRow("flag", "res.Flag"))
	goSrc := goFile([]string{"fmt", "math", "github.com/go-numerics/optimize"}, indent(g.String()))

	p.WriteString("import numpy as np\nfrom scipy import optimize\n\n")
	fmt.Fprintf(&p, "def f(x):\n    return %s\n\n", fn.pyF)
	switch method {
	case "Newton":
		fmt.Fprintf(&p, "def fprime(x):\n    return %s\n\n", fn.pyD)
		fmt.Fprintf(&p, "res = optimize.root_scalar(f, x0=%s, fprime=fprime, method=\"newton\", xtol=%s, rtol=%s, maxiter=%d)\n",
			pyNum(x0), pyNum(tol), pyNum(rtol), maxiter)
	case "Secant":
		fmt.Fprintf(&p, "res = optimize.root_scalar(f, x0=%s, method=\"secant\", xtol=%s, rtol=%s, maxiter=%d)\n",
			pyNum(x0), pyNum(tol), pyNum(rtol), maxiter)
	default:
		fmt.Fprintf(&p, "res = optimize.root_scalar(f, bracket=[%s, %s], method=%q, xtol=%s, rtol=%s, maxiter=%d)\n",
			pyNum(lo), pyNum(hi), strings.ToLower(method), pyNum(tol), pyNum(rtol), maxiter)
	}
	p.WriteString("print(res.root, f(res.root), res.iterations, res.function_calls, res.converged, res.flag)\n")
	return goSrc, p.String()
}
