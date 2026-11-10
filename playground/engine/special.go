// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-numerics/special"
)

// specialCase is a special function over a range of x. Its check is an
// identity between functions of the library (or an independent oracle) that
// holds exactly in real arithmetic, so the residual measures the library.
type specialCase struct {
	name, doc string
	f         func(float64) float64
	to        func(dst, x []float64) // the vectorised form, when the library has one
	goName    string                 // special.<goName>
	pyName    string                 // scipy.special.<pyName>
	lo, hi    string
	ylim      string
	identity  string
	residual  func(x float64) (r float64, ok bool)
}

var specialFamily = Family{Name: "Special functions", Cases: []Case{
	specialCase{
		name: "Γ(x)", doc: "the gamma function, poles at 0, −1, −2, …",
		f: special.Gamma, to: special.GammaTo, goName: "Gamma", pyName: "gamma", lo: "-4", hi: "4", ylim: "20",
		identity: "Γ(x+1) = x·Γ(x), relative",
		residual: func(x float64) (float64, bool) {
			g1 := special.Gamma(x + 1)
			return relDiff(g1, x*special.Gamma(x), g1)
		},
	}.make(),
	specialCase{
		name: "ln |Γ(x)|", doc: "log-gamma, finite where Γ is large",
		f: special.LogGamma, to: special.LogGammaTo, goName: "LogGamma", pyName: "gammaln", lo: "0.1", hi: "20", ylim: "1e6",
		identity: "ln|Γ(x+1)| = ln|Γ(x)| + ln|x|, scaled by max(1, |ln|Γ(x+1)||)",
		residual: func(x float64) (float64, bool) {
			a := special.LogGamma(x + 1)
			return relDiff(a, special.LogGamma(x)+math.Log(math.Abs(x)), math.Max(1, math.Abs(a)))
		},
	}.make(),
	specialCase{
		name: "erf(x)", doc: "the error function",
		f: special.Erf, to: special.ErfTo, goName: "Erf", pyName: "erf", lo: "-3", hi: "3", ylim: "1e6",
		identity: "erf(x) + erfc(x) = 1, absolute",
		residual: func(x float64) (float64, bool) {
			return math.Abs(special.Erf(x) + special.Erfc(x) - 1), true
		},
	}.make(),
	specialCase{
		name: "ψ(x)", doc: "digamma, the derivative of ln Γ",
		f: special.Digamma, to: special.DigammaTo, goName: "Digamma", pyName: "digamma", lo: "-4", hi: "4", ylim: "10",
		identity: "ψ(x+1) = ψ(x) + 1/x, scaled by |ψ(x+1)| + |ψ(x)| + |1/x|",
		residual: func(x float64) (float64, bool) {
			a, b := special.Digamma(x+1), special.Digamma(x)
			return relDiff(a, b+1/x, math.Abs(a)+math.Abs(b)+math.Abs(1/x))
		},
	}.make(),
	specialCase{
		name: "Φ(x)", doc: "the standard normal distribution function",
		f: special.Ndtr, to: special.NdtrTo, goName: "Ndtr", pyName: "ndtr", lo: "-5", hi: "5", ylim: "1e6",
		identity: "Φ(x) = ½·erfc(−x/√2) with the Go standard library's math.Erfc, relative",
		residual: func(x float64) (float64, bool) {
			a := special.Ndtr(x)
			return relDiff(a, 0.5*math.Erfc(-x/math.Sqrt2), a)
		},
	}.make(),
	specialCase{
		name: "J₀(x)", doc: "Bessel function of the first kind",
		f: special.J0, goName: "J0", pyName: "j0", lo: "0.1", hi: "30", ylim: "1e6",
		identity: "Wronskian J₁·Y₀ − J₀·Y₁ = 2/(πx), relative",
		residual: func(x float64) (float64, bool) {
			w := 2 / (math.Pi * x)
			return relDiff(special.J1(x)*special.Y0(x)-special.J0(x)*special.Y1(x), w, w)
		},
	}.make(),
	specialCase{
		name: "Y₀(x)", doc: "Bessel function of the second kind, logarithmic at 0",
		f: special.Y0, goName: "Y0", pyName: "y0", lo: "0.1", hi: "30", ylim: "2",
		identity: "Wronskian J₁·Y₀ − J₀·Y₁ = 2/(πx), relative",
		residual: func(x float64) (float64, bool) {
			w := 2 / (math.Pi * x)
			return relDiff(special.J1(x)*special.Y0(x)-special.J0(x)*special.Y1(x), w, w)
		},
	}.make(),
	specialCase{
		name: "I₀(x)", doc: "modified Bessel function of the first kind, grows like eˣ",
		f: special.I0, goName: "I0", pyName: "i0", lo: "0", hi: "5", ylim: "1e6",
		identity: "Wronskian I₀·K₁ + I₁·K₀ = 1/x, relative",
		residual: func(x float64) (float64, bool) {
			w := 1 / x
			return relDiff(special.I0(x)*special.K1(x)+special.I1(x)*special.K0(x), w, w)
		},
	}.make(),
	specialCase{
		name: "K₀(x)", doc: "modified Bessel function of the second kind, decays like e⁻ˣ",
		f: special.K0, goName: "K0", pyName: "k0", lo: "0.05", hi: "5", ylim: "5",
		identity: "Wronskian I₀·K₁ + I₁·K₀ = 1/x, relative",
		residual: func(x float64) (float64, bool) {
			w := 1 / x
			return relDiff(special.I0(x)*special.K1(x)+special.I1(x)*special.K0(x), w, w)
		},
	}.make(),
	specialCase{
		name: "K(m)", doc: "the complete elliptic integral of the first kind, parameter m = k²",
		f: special.EllipK, goName: "EllipK", pyName: "ellipk", lo: "0", hi: "0.99", ylim: "1e6",
		identity: "K(m) = π / (2·AGM(1, √(1−m))), arithmetic–geometric mean computed here, relative",
		residual: func(m float64) (float64, bool) {
			a := special.EllipK(m)
			return relDiff(a, math.Pi/(2*agm(1, math.Sqrt(1-m))), a)
		},
	}.make(),
}}

// agm is the arithmetic–geometric mean, to the last digit.
func agm(a, b float64) float64 {
	for i := 0; i < 100 && math.Abs(a-b) > 1e-16*math.Abs(a); i++ {
		a, b = (a+b)/2, math.Sqrt(a*b)
	}
	return a
}

// relDiff is |a − b| / scale, with ok false where the identity cannot be
// tested (a pole, a value that is not a finite number, a zero scale).
func relDiff(a, b, scale float64) (float64, bool) {
	if !finite(a) || !finite(b) || !finite(scale) || scale == 0 {
		return 0, false
	}
	return math.Abs(a-b) / math.Abs(scale), true
}

func (s specialCase) make() Case {
	methods := []string{"scalar"}
	if s.to != nil {
		methods = append(methods, "vector (…To)")
	}
	return Case{Name: s.name, Doc: s.doc, Methods: methods, Params: s.params, solve: s.solve}
}

func (s specialCase) params(string) []Param {
	return []Param{
		{"lo", s.lo, "first x"}, {"hi", s.hi, "last x"}, {"n", "400", "number of points"},
		{"ylim", s.ylim, "values beyond ±ylim are left out of the plot"},
	}
}

func (s specialCase) solve(method string, a *Args) (*run, error) {
	lo, err := a.Float("lo")
	if err != nil {
		return nil, err
	}
	hi, err := a.Float("hi")
	if err != nil {
		return nil, err
	}
	n, err := a.Int("n")
	if err != nil {
		return nil, err
	}
	if n < 2 || n > 200000 {
		return nil, fmt.Errorf("parameter n: %d is outside 2 to 200000", n)
	}
	ylim, err := a.Float("ylim")
	if err != nil {
		return nil, err
	}
	vector := strings.HasPrefix(method, "vector")
	xs := make([]float64, n)
	for i := range xs {
		xs[i] = lo + (hi-lo)*float64(i)/float64(n-1)
	}
	ys := make([]float64, n)
	scalar := make([]float64, n)
	for i, x := range xs {
		scalar[i] = s.f(x)
	}
	sweep := func() {
		if vector {
			s.to(ys, xs)
			return
		}
		for i, x := range xs {
			ys[i] = s.f(x)
		}
	}
	sweep()

	out := &Result{Title: s.name + " — " + method, Converged: true, Status: "evaluated", Points: n}
	ymin, ymax, sum := math.Inf(1), math.Inf(-1), 0.0
	for _, y := range ys {
		if finite(y) {
			ymin, ymax, sum = math.Min(ymin, y), math.Max(ymax, y), sum+y
		}
	}
	out.Summary = []Row{
		{"points", fmt.Sprint(n)}, {"f(x = " + G(xs[0]) + ")", G(ys[0])}, {"f(x = " + G(xs[n-1]) + ")", G(ys[n-1])},
		{"min f (finite points)", G(ymin)}, {"max f (finite points)", G(ymax)}, {"sum of f (finite points)", G(sum)},
	}
	res := make([]float64, n)
	lg := make([]float64, n)
	worst, tested := 0.0, 0
	for i, x := range xs {
		r, ok := s.residual(x)
		if !ok {
			res[i], lg[i] = math.NaN(), math.NaN()
			continue
		}
		res[i], lg[i] = r, math.Log10(math.Max(r, 1e-17))
		worst, tested = math.Max(worst, r), tested+1
	}
	out.Check = []Row{
		{"identity", s.identity},
		{"max residual over " + fmt.Sprint(tested) + " testable points", G(worst)},
	}
	if vector {
		d := 0.0
		for i := range ys {
			if finite(ys[i]) || finite(scalar[i]) {
				d = math.Max(d, math.Abs(ys[i]-scalar[i]))
			}
		}
		out.Check = append(out.Check, Row{"max |vector − scalar|", G(d)})
	}
	out.Plots = []Plot{
		{Name: "f(x)", XLabel: "x", YLabel: "f(x)", Series: []Series{{Label: s.name, X: xs, Y: clip(ys, ylim), Raw: ys, RawName: "f"}}},
		{Name: "log10 of the identity residual (floor −17)", XLabel: "x", YLabel: "log10 residual",
			Series: []Series{{Label: "residual", X: xs, Y: lg, Raw: res, RawName: "residual"}}},
	}
	out.Go, out.Python = s.code(vector, lo, hi, n)
	return &run{res: out, bare: sweep}, nil
}

func (s specialCase) code(vector bool, lo, hi float64, n int) (string, string) {
	var g strings.Builder
	fmt.Fprintf(&g, "n := %d\nxs := make([]float64, n)\nfor i := range xs {\n\txs[i] = %s + (%s)*float64(i)/float64(n-1)\n}\nys := make([]float64, n)\n",
		n, goNum(lo), goSub(hi, lo))
	if vector {
		fmt.Fprintf(&g, "special.%sTo(ys, xs)\n", s.goName)
	} else {
		fmt.Fprintf(&g, "for i, x := range xs {\n\tys[i] = special.%s(x)\n}\n", s.goName)
	}
	g.WriteString("ymin, ymax, sum := math.Inf(1), math.Inf(-1), 0.0\n")
	g.WriteString("for _, y := range ys {\n\tif math.IsNaN(y) || math.IsInf(y, 0) {\n\t\tcontinue\n\t}\n\tymin, ymax, sum = math.Min(ymin, y), math.Max(ymax, y), sum+y\n}\n")
	g.WriteString(printRow("points", "n"))
	g.WriteString(printRow("f(x = "+G(lo)+")", "ys[0]"))
	g.WriteString(printRow("f(x = "+G(lo+(hi-lo)*float64(n-1)/float64(n-1))+")", "ys[n-1]"))
	g.WriteString(printRow("min f (finite points)", "ymin"))
	g.WriteString(printRow("max f (finite points)", "ymax"))
	g.WriteString(printRow("sum of f (finite points)", "sum"))
	goSrc := goFile([]string{"fmt", "math", "github.com/go-numerics/special"}, indent(g.String()))

	var p strings.Builder
	p.WriteString("import numpy as np\nfrom scipy import special\n\n")
	fmt.Fprintf(&p, "n = %d\nxs = %s + (%s - %s)*np.arange(n)/(n - 1)\nys = special.%s(xs)\n", n, pyNum(lo), pyNum(hi), pyNum(lo), s.pyName)
	p.WriteString("fin = np.isfinite(ys)\nprint(n, ys[0], ys[-1], ys[fin].min(), ys[fin].max(), ys[fin].sum())\n")
	return goSrc, p.String()
}
