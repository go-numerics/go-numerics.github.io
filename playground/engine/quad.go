// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-numerics/integrate"
)

// quadCase is an integrand with an antiderivative known in closed form, which
// is what the page compares the quadrature with.
type quadCase struct {
	name, doc string
	f         func(float64) float64
	anti      func(float64) float64 // an antiderivative, NaN where it is not defined
	goF, pyF  string
	goBody    string // the integrand as statements, when it is not one expression
	a, b      string
	vlo, vhi  float64 // the window drawn when a limit is infinite
	ylim      float64 // |y| beyond this is left out of the plot
}

var quadMethods = []string{"Quad", "Romberg", "FixedQuad"}

var quadFamily = Family{Name: "Quadrature", Cases: []Case{
	quadCase{
		name: "sin x", doc: "smooth: ∫₀^π sin x dx = 2",
		f: math.Sin, anti: func(x float64) float64 { return -math.Cos(x) },
		goF: "math.Sin(x)", pyF: "np.sin(x)", a: "0", b: "pi", vlo: -1, vhi: 4, ylim: 1e6,
	}.make(),
	quadCase{
		name: "1/√x", doc: "an integrable singularity at 0: ∫₀¹ x^(−1/2) dx = 2",
		f:    func(x float64) float64 { return 1 / math.Sqrt(x) },
		anti: func(x float64) float64 { return 2 * math.Sqrt(x) },
		goF:  "1 / math.Sqrt(x)", pyF: "1/np.sqrt(x)", a: "0", b: "1", vlo: 0, vhi: 1, ylim: 20,
	}.make(),
	quadCase{
		name: "e^(−x²) over the line", doc: "an infinite range (QUADPACK's QAGI): ∫ e^(−x²) dx = √π",
		f:    func(x float64) float64 { return math.Exp(-x * x) },
		anti: func(x float64) float64 { return math.Sqrt(math.Pi) / 2 * math.Erf(x) },
		goF:  "math.Exp(-x * x)", pyF: "np.exp(-x**2)", a: "-inf", b: "inf", vlo: -4, vhi: 4, ylim: 1e6,
	}.make(),
	quadCase{
		name: "ln x", doc: "a logarithmic singularity at 0: ∫₀¹ ln x dx = −1",
		f: math.Log,
		anti: func(x float64) float64 {
			if x == 0 {
				return 0
			}
			return x*math.Log(x) - x
		},
		goF: "math.Log(x)", pyF: "np.log(x)", a: "0", b: "1", vlo: 0, vhi: 1, ylim: 10,
	}.make(),
	quadCase{
		name: "step at 1/3", doc: "a jump inside the range, 1 then 2: try points=0.3333333333333333 and compare the evaluation counts",
		f: func(x float64) float64 {
			if x < 1.0/3 {
				return 1
			}
			return 2
		},
		anti:   func(x float64) float64 { return x + math.Max(0, x-1.0/3) },
		goBody: "if x < 1.0/3 {\n\treturn 1\n}\nreturn 2",
		pyF:    "np.where(x < 1/3, 1.0, 2.0)", a: "0", b: "1", vlo: 0, vhi: 1, ylim: 1e6,
	}.make(),
}}

func (q quadCase) make() Case {
	return Case{Name: q.name, Doc: q.doc, Methods: quadMethods, Params: q.params, solve: q.solve}
}

func (q quadCase) params(method string) []Param {
	ab := []Param{{"a", q.a, "lower limit (-inf, pi, pi/2 allowed)"}, {"b", q.b, "upper limit"}}
	switch method {
	case "Romberg":
		return append(ab, Param{"tol", "1.48e-8", "absolute tolerance"}, Param{"rtol", "1.48e-8", "relative tolerance"},
			Param{"divmax", "10", "maximum order of extrapolation"})
	case "FixedQuad":
		return append(ab, Param{"n", "5", "number of Gauss–Legendre nodes"})
	}
	return append(ab, Param{"epsabs", "1.49e-8", "absolute error target"}, Param{"epsrel", "1.49e-8", "relative error target"},
		Param{"limit", "50", "most subintervals"}, Param{"points", "none", "interior break points separated by ;, or none"})
}

// reference is the integral by the antiderivative, NaN when it is not defined
// at a limit (an infinite limit of an integrand that does not decay).
func (q quadCase) reference(a, b float64) float64 { return q.anti(b) - q.anti(a) }

func (q quadCase) solve(method string, a *Args) (*run, error) {
	lo, err := a.Float("a")
	if err != nil {
		return nil, err
	}
	hi, err := a.Float("b")
	if err != nil {
		return nil, err
	}
	var epsabs, epsrel, tol, rtol float64
	var limit, divmax, n int
	var points []float64
	switch method {
	case "Romberg":
		if tol, err = a.Float("tol"); err != nil {
			return nil, err
		}
		if rtol, err = a.Float("rtol"); err != nil {
			return nil, err
		}
		if divmax, err = a.Int("divmax"); err != nil {
			return nil, err
		}
	case "FixedQuad":
		if n, err = a.Int("n"); err != nil {
			return nil, err
		}
	default:
		if epsabs, err = a.Float("epsabs"); err != nil {
			return nil, err
		}
		if epsrel, err = a.Float("epsrel"); err != nil {
			return nil, err
		}
		if limit, err = a.Int("limit"); err != nil {
			return nil, err
		}
		if a.Raw("points") != "none" {
			if points, err = a.Floats("points"); err != nil {
				return nil, err
			}
		}
	}

	wlo, whi := q.vlo, q.vhi
	if finite(lo) {
		wlo = lo
	}
	if finite(hi) {
		whi = hi
	}
	if wlo > whi {
		wlo, whi = whi, wlo
	}
	if whi == wlo {
		whi = wlo + 1
	}
	t := &tap{keep: func(x float64) bool { return x >= wlo && x <= whi }}

	var value, abserr float64
	var neval, nint int
	call := func(t *tap) error {
		f := t.f1(q.f)
		switch method {
		case "Romberg":
			v, err := integrate.Romberg(f, lo, hi, &integrate.RombergOptions{Tol: tol, RTol: rtol, DivMax: divmax})
			value = v
			return err
		case "FixedQuad":
			v, err := integrate.FixedQuad(f, lo, hi, n)
			value = v
			return err
		}
		r, err := integrate.Quad(f, lo, hi, &integrate.QuadOptions{EpsAbs: epsabs, EpsRel: epsrel, Limit: limit, Points: points})
		value, abserr, neval, nint = r.Value, r.AbsErr, r.NEval, r.NIntervals
		return err
	}
	var cerr error
	if err := guard(func() { cerr = call(t) }); err != nil {
		return nil, err
	}
	out := &Result{Title: q.name + " — " + method, Converged: cerr == nil, Status: "converged"}
	if cerr != nil {
		out.Err, out.Status = cerr.Error(), cerr.Error()
	}
	out.Summary = []Row{{"value", G(value)}}
	if method == "Quad" {
		out.Summary = append(out.Summary, Row{"estimated error", G(abserr)},
			Row{"function evaluations", fmt.Sprint(neval)}, Row{"subintervals", fmt.Sprint(nint)})
	} else {
		out.Summary = append(out.Summary, Row{"function evaluations", fmt.Sprint(t.n)})
	}
	ref := q.reference(lo, hi)
	if finite(ref) {
		d := math.Abs(value - ref)
		out.Check = []Row{{"reference (antiderivative)", G(ref)}, {"|value − reference|", G(d)}}
		if method == "Quad" {
			out.Check = append(out.Check, Row{"true error within the estimate", boolText(d <= abserr)})
		}
	} else {
		out.Check = []Row{{"reference (antiderivative)", "not defined at these limits"}}
	}
	out.Plots = q.plots(wlo, whi, t)
	out.Go, out.Python = q.code(method, lo, hi, epsabs, epsrel, limit, points, tol, rtol, divmax, n)
	return &run{res: out, bare: func() { _ = call(nil) }}, nil
}

func (q quadCase) plots(wlo, whi float64, t *tap) []Plot {
	xs := linspace(wlo, whi, 400)
	ys := make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = q.f(x)
	}
	var ex, ey []float64
	for i, x := range t.xs {
		if finite(t.ys[i]) && math.Abs(t.ys[i]) <= q.ylim {
			ex, ey = append(ex, x), append(ey, t.ys[i])
		}
	}
	return []Plot{{Name: "the integrand and the points the rule evaluated it at", XLabel: "x", YLabel: "f(x)", Series: []Series{
		{Label: "f(x)", X: xs, Y: clip(ys, q.ylim), Raw: ys, RawName: "f"},
		{Label: "evaluations", X: ex, Y: ey, Style: Markers},
	}}}
}

func (q quadCase) code(method string, lo, hi, epsabs, epsrel float64, limit int, points []float64, tol, rtol float64, divmax, n int) (string, string) {
	var g, p strings.Builder
	// calls counts the evaluations of the rules that do not report them.
	fdef := fmt.Sprintf("f := func(x float64) float64 { return %s }\n", q.goF)
	cdef := fmt.Sprintf("calls := 0\nf := func(x float64) float64 {\n\tcalls++\n\treturn %s\n}\n", q.goF)
	if q.goBody != "" {
		fdef = "f := func(x float64) float64 {\n" + indent(q.goBody) + "}\n"
		cdef = "calls := 0\nf := func(x float64) float64 {\n\tcalls++\n" + indent(q.goBody) + "}\n"
	}
	if method == "Quad" {
		g.WriteString(fdef)
		opts := fmt.Sprintf("EpsAbs: %s, EpsRel: %s, Limit: %d", goNum(epsabs), goNum(epsrel), limit)
		if points != nil {
			opts += ", Points: " + goNums(points)
		}
		fmt.Fprintf(&g, "res, err := integrate.Quad(f, %s, %s, &integrate.QuadOptions{%s})\n", goNum(lo), goNum(hi), opts)
		g.WriteString("if err != nil {\n\tfmt.Println(\"error:\", err)\n}\n")
		g.WriteString(printRow("value", "res.Value"))
		g.WriteString(printRow("estimated error", "res.AbsErr"))
		g.WriteString(printRow("function evaluations", "res.NEval"))
		g.WriteString(printRow("subintervals", "res.NIntervals"))
	} else {
		g.WriteString(cdef)
		if method == "Romberg" {
			fmt.Fprintf(&g, "value, err := integrate.Romberg(f, %s, %s, &integrate.RombergOptions{Tol: %s, RTol: %s, DivMax: %d})\n",
				goNum(lo), goNum(hi), goNum(tol), goNum(rtol), divmax)
		} else {
			fmt.Fprintf(&g, "value, err := integrate.FixedQuad(f, %s, %s, %d)\n", goNum(lo), goNum(hi), n)
		}
		g.WriteString("if err != nil {\n\tfmt.Println(\"error:\", err)\n}\n")
		g.WriteString(printRow("value", "value"))
		g.WriteString(printRow("function evaluations", "calls"))
	}
	goSrc := goFile([]string{"fmt", "math", "github.com/go-numerics/integrate"}, indent(g.String()))

	p.WriteString("import numpy as np\nfrom scipy import integrate\n\n")
	fmt.Fprintf(&p, "def f(x):\n    return %s\n\n", q.pyF)
	switch method {
	case "Romberg":
		fmt.Fprintf(&p, "# scipy.integrate.romberg was removed in SciPy 1.15; this is SciPy <= 1.14.\n")
		fmt.Fprintf(&p, "value = integrate.romberg(f, %s, %s, tol=%s, rtol=%s, divmax=%d)\nprint(value)\n",
			pyNum(lo), pyNum(hi), pyNum(tol), pyNum(rtol), divmax)
	case "FixedQuad":
		fmt.Fprintf(&p, "value, _ = integrate.fixed_quad(f, %s, %s, n=%d)\nprint(value)\n", pyNum(lo), pyNum(hi), n)
	default:
		extra := ""
		if points != nil {
			extra = ", points=" + pyNums(points)
		}
		fmt.Fprintf(&p, "value, abserr, info = integrate.quad(f, %s, %s, epsabs=%s, epsrel=%s, limit=%d%s, full_output=1)[:3]\n",
			pyNum(lo), pyNum(hi), pyNum(epsabs), pyNum(epsrel), limit, extra)
		p.WriteString("print(value, abserr, info[\"neval\"], info[\"last\"])\n")
	}
	return goSrc, p.String()
}
