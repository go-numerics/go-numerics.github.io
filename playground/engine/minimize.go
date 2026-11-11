// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-numerics/optimize"
)

// minProblem is a test function of a vector: the closures the page runs, the
// same code as source for the generated programs, and its known minimisers.
type minProblem struct {
	name, doc string
	dim       int // 0: any dimension of at least 2
	x0        string
	f         func(x []float64) float64
	grad      func(x, g []float64)
	goF, goG  string // function literals' bodies, Go
	pyDefs    string // NumPy definitions of f and grad (or an import)
	pyF, pyG  string // the names to pass
	minima    [][]float64
}

var minMethods = []string{"BFGS", "L-BFGS-B", "CG", "Powell", "Nelder-Mead"}

var optMethod = map[string]optimize.Method{
	"BFGS": optimize.BFGS, "L-BFGS-B": optimize.LBFGSB, "CG": optimize.CG,
	"Powell": optimize.Powell, "Nelder-Mead": optimize.NelderMead,
}

var minFamily = Family{Name: "Minimisation", Cases: []Case{
	rosenbrock.make(), himmelblau.make(), booth.make(), beale.make(),
}}

var rosenbrock = minProblem{
	name: "Rosenbrock", doc: "the banana valley, any dimension: minimum 0 at (1, …, 1)",
	x0: "-1.2;1",
	f: func(x []float64) float64 {
		s := 0.0
		for i := 0; i+1 < len(x); i++ {
			s += 100*(x[i+1]-x[i]*x[i])*(x[i+1]-x[i]*x[i]) + (1-x[i])*(1-x[i])
		}
		return s
	},
	grad: func(x, g []float64) {
		for i := range g {
			g[i] = 0
		}
		for i := 0; i+1 < len(x); i++ {
			g[i] += -400*x[i]*(x[i+1]-x[i]*x[i]) - 2*(1-x[i])
			g[i+1] += 200 * (x[i+1] - x[i]*x[i])
		}
	},
	goF: `s := 0.0
for i := 0; i+1 < len(x); i++ {
	s += 100*(x[i+1]-x[i]*x[i])*(x[i+1]-x[i]*x[i]) + (1-x[i])*(1-x[i])
}
return s`,
	goG: `for i := range g {
	g[i] = 0
}
for i := 0; i+1 < len(x); i++ {
	g[i] += -400*x[i]*(x[i+1]-x[i]*x[i]) - 2*(1-x[i])
	g[i+1] += 200 * (x[i+1] - x[i]*x[i])
}`,
	pyDefs: "from scipy.optimize import rosen, rosen_der\n",
	pyF:    "rosen", pyG: "rosen_der",
	// minima depend on the dimension: filled by reference().
}

var himmelblau = minProblem{
	name: "Himmelblau", doc: "four equal minima of 0; the start decides which one is found",
	dim: 2, x0: "0;0",
	f: func(x []float64) float64 {
		a, b := x[0]*x[0]+x[1]-11, x[0]+x[1]*x[1]-7
		return a*a + b*b
	},
	grad: func(x, g []float64) {
		a, b := x[0]*x[0]+x[1]-11, x[0]+x[1]*x[1]-7
		g[0] = 4*x[0]*a + 2*b
		g[1] = 2*a + 4*x[1]*b
	},
	goF: `a, b := x[0]*x[0]+x[1]-11, x[0]+x[1]*x[1]-7
return a*a + b*b`,
	goG: `a, b := x[0]*x[0]+x[1]-11, x[0]+x[1]*x[1]-7
g[0] = 4*x[0]*a + 2*b
g[1] = 2*a + 4*x[1]*b`,
	pyDefs: `def f(x):
    a, b = x[0]**2 + x[1] - 11, x[0] + x[1]**2 - 7
    return a*a + b*b

def grad(x):
    a, b = x[0]**2 + x[1] - 11, x[0] + x[1]**2 - 7
    return np.array([4*x[0]*a + 2*b, 2*a + 4*x[1]*b])
`,
	pyF: "f", pyG: "grad",
	minima: himmelblauMinima(),
}

var booth = minProblem{
	name: "Booth", doc: "a plain quadratic bowl: minimum 0 at (1, 3)",
	dim: 2, x0: "-5;5",
	f: func(x []float64) float64 {
		a, b := x[0]+2*x[1]-7, 2*x[0]+x[1]-5
		return a*a + b*b
	},
	grad: func(x, g []float64) {
		a, b := x[0]+2*x[1]-7, 2*x[0]+x[1]-5
		g[0] = 2*a + 4*b
		g[1] = 4*a + 2*b
	},
	goF: `a, b := x[0]+2*x[1]-7, 2*x[0]+x[1]-5
return a*a + b*b`,
	goG: `a, b := x[0]+2*x[1]-7, 2*x[0]+x[1]-5
g[0] = 2*a + 4*b
g[1] = 4*a + 2*b`,
	pyDefs: `def f(x):
    a, b = x[0] + 2*x[1] - 7, 2*x[0] + x[1] - 5
    return a*a + b*b

def grad(x):
    a, b = x[0] + 2*x[1] - 7, 2*x[0] + x[1] - 5
    return np.array([2*a + 4*b, 4*a + 2*b])
`,
	pyF: "f", pyG: "grad",
	minima: [][]float64{{1, 3}},
}

var beale = minProblem{
	name: "Beale", doc: "a flat valley: minimum 0 at (3, 0.5)",
	dim: 2, x0: "1;1",
	f: func(x []float64) float64 {
		t1, t2, t3 := 1.5-x[0]+x[0]*x[1], 2.25-x[0]+x[0]*x[1]*x[1], 2.625-x[0]+x[0]*x[1]*x[1]*x[1]
		return t1*t1 + t2*t2 + t3*t3
	},
	grad: func(x, g []float64) {
		t1, t2, t3 := 1.5-x[0]+x[0]*x[1], 2.25-x[0]+x[0]*x[1]*x[1], 2.625-x[0]+x[0]*x[1]*x[1]*x[1]
		g[0] = 2*t1*(x[1]-1) + 2*t2*(x[1]*x[1]-1) + 2*t3*(x[1]*x[1]*x[1]-1)
		g[1] = 2*t1*x[0] + 2*t2*(2*x[0]*x[1]) + 2*t3*(3*x[0]*x[1]*x[1])
	},
	goF: `t1, t2, t3 := 1.5-x[0]+x[0]*x[1], 2.25-x[0]+x[0]*x[1]*x[1], 2.625-x[0]+x[0]*x[1]*x[1]*x[1]
return t1*t1 + t2*t2 + t3*t3`,
	goG: `t1, t2, t3 := 1.5-x[0]+x[0]*x[1], 2.25-x[0]+x[0]*x[1]*x[1], 2.625-x[0]+x[0]*x[1]*x[1]*x[1]
g[0] = 2*t1*(x[1]-1) + 2*t2*(x[1]*x[1]-1) + 2*t3*(x[1]*x[1]*x[1]-1)
g[1] = 2*t1*x[0] + 2*t2*(2*x[0]*x[1]) + 2*t3*(3*x[0]*x[1]*x[1])`,
	pyDefs: `def f(x):
    t1, t2, t3 = 1.5 - x[0] + x[0]*x[1], 2.25 - x[0] + x[0]*x[1]**2, 2.625 - x[0] + x[0]*x[1]**3
    return t1*t1 + t2*t2 + t3*t3

def grad(x):
    t1, t2, t3 = 1.5 - x[0] + x[0]*x[1], 2.25 - x[0] + x[0]*x[1]**2, 2.625 - x[0] + x[0]*x[1]**3
    return np.array([2*t1*(x[1] - 1) + 2*t2*(x[1]**2 - 1) + 2*t3*(x[1]**3 - 1),
                     2*t1*x[0] + 2*t2*(2*x[0]*x[1]) + 2*t3*(3*x[0]*x[1]**2)])
`,
	pyF: "f", pyG: "grad",
	minima: [][]float64{{3, 0.5}},
}

// himmelblauMinima polishes the four minima, known to a few digits, by Newton's
// method on the gradient with the analytic Hessian: the references the page
// compares with come from this arithmetic, not from the library under test.
func himmelblauMinima() [][]float64 {
	start := [][]float64{{3, 2}, {-2.8, 3.1}, {-3.8, -3.3}, {3.6, -1.8}}
	out := make([][]float64, len(start))
	for i, p := range start {
		x, y := p[0], p[1]
		for k := 0; k < 50; k++ {
			a, b := x*x+y-11, x+y*y-7
			gx, gy := 4*x*a+2*b, 2*a+4*y*b
			hxx, hxy, hyy := 12*x*x+4*y-42, 4*x+4*y, 4*x+12*y*y-26
			det := hxx*hyy - hxy*hxy
			dx, dy := (hyy*gx-hxy*gy)/det, (hxx*gy-hxy*gx)/det
			x, y = x-dx, y-dy
			if math.Abs(dx)+math.Abs(dy) < 1e-17 {
				break
			}
		}
		out[i] = []float64{x, y}
	}
	return out
}

func (mp minProblem) make() Case {
	return Case{Name: mp.name, Doc: mp.doc, Methods: minMethods, Params: mp.params, solve: mp.solve}
}

func (mp minProblem) params(method string) []Param {
	return []Param{
		{"x0", mp.x0, "start, values separated by ;"},
		{"grad", "analytic", "analytic, or fd for finite differences"},
		{"tol", "0", "the method's main tolerance (0: its default)"},
		{"maxiter", "0", "iteration limit (0: the method's default)"},
	}
}

// reference is the known minimiser nearest to x, and its objective value.
func (mp minProblem) reference(x []float64) ([]float64, float64) {
	if mp.minima == nil { // Rosenbrock: all ones
		ones := make([]float64, len(x))
		for i := range ones {
			ones[i] = 1
		}
		return ones, 0
	}
	best, bd := mp.minima[0], math.Inf(1)
	for _, m := range mp.minima {
		if d := dist(x, m); d < bd {
			best, bd = m, d
		}
	}
	return best, 0
}

func dist(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		s += (a[i] - b[i]) * (a[i] - b[i])
	}
	return math.Sqrt(s)
}

func (mp minProblem) solve(method string, a *Args) (*run, error) {
	x0, err := a.Floats("x0")
	if err != nil {
		return nil, err
	}
	switch {
	case len(x0) < 2 || len(x0) > 200:
		return nil, fmt.Errorf("parameter x0: need between 2 and 200 values, got %d", len(x0))
	case mp.dim != 0 && len(x0) != mp.dim:
		return nil, fmt.Errorf("parameter x0: %s takes %d values, got %d", mp.name, mp.dim, len(x0))
	}
	mode := a.Raw("grad")
	if mode != "analytic" && mode != "fd" {
		return nil, fmt.Errorf("parameter grad: %q is neither analytic nor fd", mode)
	}
	tol, err := a.Float("tol")
	if err != nil {
		return nil, err
	}
	maxiter, err := a.Int("maxiter")
	if err != nil {
		return nil, err
	}

	prob := func(t *tap) optimize.Problem {
		p := optimize.Problem{Func: t.fv(mp.f)}
		if mode == "analytic" {
			p.Grad = mp.grad
		}
		return p
	}
	var iters [][]float64
	var fvals []float64
	call := func(t *tap, record bool) (*optimize.Result, error) {
		o := &optimize.Options{Tol: tol, MaxIter: maxiter}
		if record {
			o.Callback = func(x []float64, f float64) bool {
				if len(iters) < maxRecorded {
					iters, fvals = append(iters, append([]float64(nil), x...)), append(fvals, f)
				}
				return false
			}
		}
		return optimize.Minimize(prob(t), append([]float64(nil), x0...), optMethod[method], o)
	}

	var res *optimize.Result
	var cerr error
	if err := guard(func() { res, cerr = call(&tap{}, true) }); err != nil {
		return nil, err
	}
	out := &Result{Title: mp.name + " — " + method, Err: errText(cerr)}
	if res != nil {
		out.Status, out.Converged = res.Message, res.Success
		out.Summary = []Row{
			{"x", Gs(res.X)}, {"f(x)", G(res.Fun)},
			{"iterations", fmt.Sprint(res.Nit)}, {"function evaluations", fmt.Sprint(res.Nfev)},
			{"gradient evaluations", fmt.Sprint(res.Njev)},
			{"success", fmt.Sprint(res.Success)}, {"status", fmt.Sprint(res.Status)}, {"message", res.Message},
		}
		ref, fmin := mp.reference(res.X)
		g := make([]float64, len(res.X))
		mp.grad(res.X, g)
		gmax := 0.0
		for _, v := range g {
			gmax = math.Max(gmax, math.Abs(v))
		}
		out.Check = []Row{
			{"nearest known minimiser", Gs(ref)},
			{"‖x − x*‖", G(dist(res.X, ref))},
			{"f(x) − f(x*)", G(mp.f(res.X) - fmin)},
			{"max |∇f(x)| (analytic)", G(gmax)},
		}
		out.Plots = mp.plots(x0, res.X, ref, iters, fvals)
	}
	out.Go, out.Python = mp.code(method, x0, mode, tol, maxiter)
	out.Ruby, out.Julia = mp.rubyJulia(method, x0, mode, tol, maxiter)
	return &run{res: out, bare: func() { _, _ = call(nil, false) }}, nil
}

func (mp minProblem) plots(x0, xs, ref []float64, iters [][]float64, fvals []float64) []Plot {
	k := make([]float64, 0, len(fvals)+1)
	lg := make([]float64, 0, len(fvals)+1)
	raw := make([]float64, 0, len(fvals)+1)
	push := func(i int, f float64) {
		k = append(k, float64(i))
		lg = append(lg, math.Log10(math.Max(math.Abs(f), 1e-300)))
		raw = append(raw, f)
	}
	push(0, mp.f(x0))
	for i, f := range fvals {
		push(i+1, f)
	}
	conv := Plot{Name: "log10 f at each iteration", XLabel: "iteration", YLabel: "log10 f",
		Series: []Series{{Label: "log10 f", X: k, Y: lg, Raw: raw, RawName: "f"}}}

	px, py := []float64{x0[0]}, []float64{x0[1]}
	for _, it := range iters {
		px, py = append(px, it[0]), append(py, it[1])
	}
	name := "path of the iterates in (x0, x1)"
	if len(x0) > 2 {
		name = "path of the iterates in (x0, x1) of " + fmt.Sprint(len(x0))
	}
	path := Plot{Name: name, XLabel: "x0", YLabel: "x1", Series: []Series{
		{Label: "iterates", X: px, Y: py},
		{Label: "start", X: x0[:1], Y: x0[1:2], Style: Markers},
		{Label: "minimiser", X: ref[:1], Y: ref[1:2], Style: Markers},
		{Label: "result", X: xs[:1], Y: xs[1:2], Style: Markers},
	}}
	return []Plot{conv, path}
}

var goMethodName = map[string]string{
	"BFGS": "BFGS", "L-BFGS-B": "LBFGSB", "CG": "CG", "Powell": "Powell", "Nelder-Mead": "NelderMead",
}

func (mp minProblem) code(method string, x0 []float64, mode string, tol float64, maxiter int) (string, string) {
	var g strings.Builder
	fmt.Fprintf(&g, "f := func(x []float64) float64 {\n%s\n}\n", indent(mp.goF))
	prob := "optimize.Problem{Func: f}"
	if mode == "analytic" {
		fmt.Fprintf(&g, "grad := func(x, g []float64) {\n%s\n}\n", indent(mp.goG))
		prob = "optimize.Problem{Func: f, Grad: grad}"
	}
	fmt.Fprintf(&g, "res, err := optimize.Minimize(%s, %s, optimize.%s,\n\t&optimize.Options{Tol: %s, MaxIter: %d})\n",
		prob, goNums(x0), goMethodName[method], goNum(tol), maxiter)
	g.WriteString("if err != nil {\n\tfmt.Println(\"error:\", err)\n}\nif res == nil {\n\treturn\n}\n")
	g.WriteString(printRow("x", "res.X"))
	g.WriteString(printRow("f(x)", "res.Fun"))
	g.WriteString(printRow("iterations", "res.Nit"))
	g.WriteString(printRow("function evaluations", "res.Nfev"))
	g.WriteString(printRow("gradient evaluations", "res.Njev"))
	g.WriteString(printRow("success", "res.Success"))
	g.WriteString(printRow("status", "res.Status"))
	g.WriteString(printRow("message", "res.Message"))
	goSrc := goFile([]string{"fmt", "github.com/go-numerics/optimize"}, indent(g.String()))

	var p strings.Builder
	p.WriteString("import numpy as np\nfrom scipy import optimize\n")
	p.WriteString(mp.pyDefs + "\n")
	jac := ""
	if mode == "analytic" {
		jac = ", jac=" + mp.pyG
	}
	t := "None"
	if tol != 0 {
		t = pyNum(tol)
	}
	opts := ""
	if maxiter > 0 {
		opts = fmt.Sprintf(", options={\"maxiter\": %d}", maxiter)
	}
	fmt.Fprintf(&p, "res = optimize.minimize(%s, np.array(%s)%s, method=%q, tol=%s%s)\n",
		mp.pyF, pyNums(x0), jac, method, t, opts)
	p.WriteString("print(res.x, res.fun, res.nit, res.nfev, res.get(\"njev\"), res.success, res.status, res.message)\n")
	return goSrc, p.String()
}
