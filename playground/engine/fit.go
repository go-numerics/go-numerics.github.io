// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"

	"github.com/go-ndarray/ndarray"
	"github.com/go-numerics/optimize"
)

// fitModel is a model y = f(x; p) with the parameters the data are drawn from.
type fitModel struct {
	name, doc string
	f         func(x float64, p []float64) float64
	goF, pyF  string // the expression in x and p
	truth     []float64
	lo, hi    float64
	p0        string
}

var fitMethods = []string{"LM", "TRF"}

var fitFamily = Family{Name: "Curve fit", Cases: []Case{
	fitModel{
		name: "exponential decay", doc: "a·e^(−b·x) + c",
		f:   func(x float64, p []float64) float64 { return p[0]*math.Exp(-p[1]*x) + p[2] },
		goF: "p[0]*math.Exp(-p[1]*x) + p[2]", pyF: "p[0]*np.exp(-p[1]*x) + p[2]",
		truth: []float64{2.5, 1.3, 0.5}, lo: 0, hi: 4, p0: "1;1;0",
	}.make(),
	fitModel{
		name: "Gaussian peak", doc: "a·exp(−(x − μ)² / 2σ²)",
		f: func(x float64, p []float64) float64 {
			return p[0] * math.Exp(-(x-p[1])*(x-p[1])/(2*p[2]*p[2]))
		},
		goF: "p[0] * math.Exp(-(x-p[1])*(x-p[1])/(2*p[2]*p[2]))", pyF: "p[0]*np.exp(-(x - p[1])**2/(2*p[2]**2))",
		truth: []float64{3, 1.5, 0.8}, lo: -2, hi: 5, p0: "1;1;1",
	}.make(),
	fitModel{
		name: "logistic curve", doc: "L / (1 + e^(−k·(x − x0)))",
		f:   func(x float64, p []float64) float64 { return p[0] / (1 + math.Exp(-p[1]*(x-p[2]))) },
		goF: "p[0] / (1 + math.Exp(-p[1]*(x-p[2])))", pyF: "p[0]/(1 + np.exp(-p[1]*(x - p[2])))",
		truth: []float64{5, 1.2, 2}, lo: -2, hi: 6, p0: "1;1;1",
	}.make(),
}}

func (m fitModel) make() Case {
	return Case{Name: m.name, Doc: m.doc, Methods: fitMethods, Params: m.params, solve: m.solve}
}

func (m fitModel) params(string) []Param {
	return []Param{
		{"n", "25", "number of samples"},
		{"noise", "0.05", "standard deviation of the Gaussian noise added to each sample"},
		{"seed", "1", "seed of the noise (PCG, math/rand/v2)"},
		{"p0", m.p0, "initial guess, values separated by ;"},
		{"maxfev", "0", "evaluation limit (0: the method's default)"},
	}
}

// sample draws the data: n points from lo to hi, the model at the true
// parameters plus noise·N(0, 1) from PCG(seed, 0), one normal per point.
func (m fitModel) sample(n int, noise float64, seed uint64) (x, y []float64) {
	rng := rand.New(rand.NewPCG(seed, 0))
	x, y = make([]float64, n), make([]float64, n)
	for i := range x {
		x[i] = m.lo + (m.hi-m.lo)*float64(i)/float64(n-1)
		y[i] = m.f(x[i], m.truth) + noise*rng.NormFloat64()
	}
	return x, y
}

func (m fitModel) solve(method string, a *Args) (*run, error) {
	n, err := a.Int("n")
	if err != nil {
		return nil, err
	}
	if n < 5 || n > 5000 {
		return nil, fmt.Errorf("parameter n: %d is outside 5 to 5000", n)
	}
	noise, err := a.Float("noise")
	if err != nil {
		return nil, err
	}
	seed, err := a.Int("seed")
	if err != nil {
		return nil, err
	}
	if seed < 0 {
		return nil, fmt.Errorf("parameter seed: %d is negative", seed)
	}
	p0, err := a.Floats("p0")
	if err != nil {
		return nil, err
	}
	if len(p0) != len(m.truth) {
		return nil, fmt.Errorf("parameter p0: %s takes %d values, got %d", m.name, len(m.truth), len(p0))
	}
	maxfev, err := a.Int("maxfev")
	if err != nil {
		return nil, err
	}
	xs, ys := m.sample(n, noise, uint64(seed))
	// FromData only fails when the shape does not match the data: n elements, n.
	xa, _ := ndarray.FromData(xs, n)
	ya, _ := ndarray.FromData(ys, n)
	lsm := optimize.LM
	if method == "TRF" {
		lsm = optimize.TRF
	}
	t := &tap{}
	model := func(x float64, p []float64) float64 { t.count(); return m.f(x, p) }
	bareModel := m.f
	call := func(f func(float64, []float64) float64) (*optimize.CurveFitResult, error) {
		return optimize.CurveFit(f, xa, ya, append([]float64(nil), p0...),
			&optimize.CurveFitOptions{Method: lsm, MaxFev: maxfev})
	}
	var res *optimize.CurveFitResult
	var cerr error
	if err := guard(func() { res, cerr = call(model) }); err != nil {
		return nil, err
	}
	out := &Result{Title: m.name + " — " + method, Err: errText(cerr)}
	if res != nil {
		msg := strings.Join(strings.Fields(res.Message), " ") // MINPACK's text wraps lines
		out.Status, out.Converged = msg, res.Status > 0 && res.Status < 5
		out.Summary = []Row{{"popt", Gs(res.Popt)}}
		// Perr indexes the covariance, which a fit that gave up leaves empty.
		var perr []float64
		if len(res.Pcov) == len(res.Popt)*len(res.Popt) {
			perr = res.Perr()
			out.Summary = append(out.Summary, Row{"perr", Gs(perr)})
		}
		out.Summary = append(out.Summary,
			Row{"function evaluations", fmt.Sprint(res.Nfev)}, Row{"covariance estimated", fmt.Sprint(res.CovOK)},
			Row{"status", fmt.Sprint(res.Status)}, Row{"message", msg})
		worst, zmax, rss := 0.0, 0.0, 0.0
		for i, p := range res.Popt {
			d := math.Abs(p - m.truth[i])
			worst = math.Max(worst, d)
			if res.CovOK && perr != nil && perr[i] > 0 {
				zmax = math.Max(zmax, d/perr[i])
			}
		}
		for i, x := range xs {
			r := ys[i] - m.f(x, res.Popt)
			rss += r * r
		}
		out.Check = []Row{
			{"true parameters", Gs(m.truth)},
			{"max |popt − true|", G(worst)},
			{"max |popt − true| / perr", G(zmax)},
			{"noise σ put in the data", G(noise)},
			{"rms residual", G(math.Sqrt(rss / float64(n)))},
		}
		out.Plots = m.plots(xs, ys, res.Popt)
	}
	out.Go, out.Python = m.code(method, n, noise, seed, p0, maxfev, xs, ys)
	out.Ruby, out.Julia = m.rubyJulia(method, p0, maxfev, xs, ys)
	return &run{res: out, bare: func() { _, _ = call(bareModel) }}, nil
}

func (m fitModel) plots(xs, ys, popt []float64) []Plot {
	dx := linspace(xs[0], xs[len(xs)-1], 200)
	fit, truth := make([]float64, len(dx)), make([]float64, len(dx))
	for i, x := range dx {
		fit[i], truth[i] = m.f(x, popt), m.f(x, m.truth)
	}
	res := make([]float64, len(xs))
	for i, x := range xs {
		res[i] = ys[i] - m.f(x, popt)
	}
	return []Plot{
		{Name: "data, the fit and the curve the data were drawn from", XLabel: "x", YLabel: "y", Series: []Series{
			{Label: "fit", X: dx, Y: fit},
			{Label: "data", X: xs, Y: ys, Style: Markers},
			{Label: "true curve", X: dx, Y: truth},
		}},
		{Name: "residuals y − f(x; popt)", XLabel: "x", YLabel: "residual", Series: []Series{
			{Label: "residual", X: xs, Y: res, Style: Stem},
		}},
	}
}

func (m fitModel) code(method string, n int, noise float64, seed int, p0 []float64, maxfev int, xs, ys []float64) (string, string) {
	var g strings.Builder
	fmt.Fprintf(&g, "f := func(x float64, p []float64) float64 { return %s }\n\n", m.goF)
	fmt.Fprintf(&g, "// The data: the model at %s plus Gaussian noise.\n", Gs(m.truth))
	fmt.Fprintf(&g, "truth := %s\n", goNums(m.truth))
	fmt.Fprintf(&g, "rng := rand.New(rand.NewPCG(%d, 0))\n", seed)
	fmt.Fprintf(&g, "xs, ys := make([]float64, %d), make([]float64, %d)\n", n, n)
	fmt.Fprintf(&g, "for i := range xs {\n\txs[i] = %s + (%s)*float64(i)/float64(%d)\n", goNum(m.lo), goSub(m.hi, m.lo), n-1)
	fmt.Fprintf(&g, "\tys[i] = f(xs[i], truth) + %s*rng.NormFloat64()\n}\n", goNum(noise))
	g.WriteString("xa, _ := ndarray.FromData(xs, len(xs))\nya, _ := ndarray.FromData(ys, len(ys))\n\n")
	fmt.Fprintf(&g, "res, err := optimize.CurveFit(f, xa, ya, %s,\n\t&optimize.CurveFitOptions{Method: optimize.%s, MaxFev: %d})\n", goNums(p0), method, maxfev)
	g.WriteString("if err != nil {\n\tfmt.Println(\"error:\", err)\n}\nif res == nil {\n\treturn\n}\n")
	g.WriteString(printRow("popt", "res.Popt"))
	g.WriteString("if len(res.Pcov) == len(res.Popt)*len(res.Popt) { // a fit that gave up has no covariance\n" +
		indent(printRow("perr", "res.Perr()")) + "}\n")
	g.WriteString(printRow("function evaluations", "res.Nfev"))
	g.WriteString(printRow("covariance estimated", "res.CovOK"))
	g.WriteString(printRow("status", "res.Status"))
	g.WriteString(printRow("message", `strings.Join(strings.Fields(res.Message), " ")`))
	goSrc := goFile([]string{"fmt", "math", "math/rand/v2", "strings", "github.com/go-ndarray/ndarray", "github.com/go-numerics/optimize"}, indent(g.String()))

	var p strings.Builder
	p.WriteString("import numpy as np\nfrom scipy import optimize\n\n")
	fmt.Fprintf(&p, "def f(x, *p):\n    return %s\n\n", m.pyF)
	fmt.Fprintf(&p, "# The data the page drew (the model at %s plus Gaussian noise, %d points).\n", Gs(m.truth), n)
	fmt.Fprintf(&p, "xs = np.array(%s)\nys = np.array(%s)\n\n", pyNums(xs), pyNums(ys))
	extra := ""
	if maxfev > 0 {
		extra = fmt.Sprintf(", maxfev=%d", maxfev)
	}
	fmt.Fprintf(&p, "popt, pcov = optimize.curve_fit(f, xs, ys, p0=%s, method=%q%s)\n", pyNums(p0), strings.ToLower(method), extra)
	p.WriteString("print(popt, np.sqrt(np.diag(pcov)))\n")
	return goSrc, p.String()
}
