// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-numerics/integrate"
)

// odeProb is an initial value problem at one setting of its parameters.
type odeProb struct {
	f      integrate.ODEFunc
	y0     []float64
	tend   float64
	names  []string
	goBody string // the statements of the right-hand side, Go (t, y, dydt)
	pyRet  string // the Python return expression, a list (t, y)
	// check compares the solution with a mathematical reference.
	check func(s *integrate.Solution) []Row
	// extra plots after the time series and before the step sizes.
	extra func(s *integrate.Solution) []Plot
	// exact, when non-nil, is a closed-form solution drawn beside the first
	// component.
	exact func(t float64) float64
}

type odeCase struct {
	name, doc string
	extra     []Param
	build     func(a *Args) (*odeProb, error)
}

var odeMethods = []string{"RK45", "DOP853", "RK23", "Radau", "BDF", "LSODA"}

var odeMethod = map[string]integrate.Method{
	"RK45": integrate.RK45, "DOP853": integrate.DOP853, "RK23": integrate.RK23,
	"Radau": integrate.Radau, "BDF": integrate.BDF, "LSODA": integrate.LSODA,
}

var odeFamily = Family{Name: "ODE", Cases: []Case{
	odeCase{
		name: "Lotka–Volterra", doc: "predator and prey; V = d·x − c·ln x + b·y − a·ln y is constant along every orbit",
		extra: []Param{{"a", "1.5", "prey growth"}, {"b", "1", "predation"}, {"c", "3", "predator death"}, {"d", "1", "predator gain"},
			{"x0", "10", "initial prey"}, {"y0", "5", "initial predators"}, {"tend", "10", "end of the span"}},
		build: buildLV,
	}.make(),
	odeCase{
		name: "Van der Pol", doc: "y″ = μ(1 − y²)y′ − y: stiff for large μ (try mu=1000, tend=3000 with RK45, then Radau)",
		extra: []Param{{"mu", "5", "μ"}, {"tend", "20", "end of the span"}},
		build: buildVdP,
	}.make(),
	odeCase{
		name: "stiff relaxation", doc: "y′ = −λ(y − cos t) − sin t, y(0) = 1: the exact solution is cos t for every λ",
		extra: []Param{{"lambda", "1000", "λ, the stiffness"}, {"tend", "10", "end of the span"}},
		build: buildStiff,
	}.make(),
}}

func (c odeCase) make() Case {
	return Case{Name: c.name, Doc: c.doc, Methods: odeMethods, solve: c.solve, Params: func(string) []Param {
		return append(append([]Param(nil), c.extra...), Param{"rtol", "1e-6", "relative tolerance"}, Param{"atol", "1e-9", "absolute tolerance"})
	}}
}

func buildLV(a *Args) (*odeProb, error) {
	v, err := floats(a, "a", "b", "c", "d", "x0", "y0", "tend")
	if err != nil {
		return nil, err
	}
	pa, pb, pc, pd, x0, y0, tend := v[0], v[1], v[2], v[3], v[4], v[5], v[6]
	inv := func(y []float64) float64 { return pd*y[0] - pc*math.Log(y[0]) + pb*y[1] - pa*math.Log(y[1]) }
	return &odeProb{
		f: func(t float64, y, dydt []float64) {
			dydt[0] = pa*y[0] - pb*y[0]*y[1]
			dydt[1] = -pc*y[1] + pd*y[0]*y[1]
		},
		y0: []float64{x0, y0}, tend: tend, names: []string{"prey x", "predators y"},
		goBody: fmt.Sprintf("dydt[0] = %s*y[0] - %s*y[0]*y[1]\ndydt[1] = %s*y[1] + %s*y[0]*y[1]", goNum(pa), goNum(pb), goNum(-pc), goNum(pd)),
		pyRet:  fmt.Sprintf("[%s*y[0] - %s*y[0]*y[1], %s*y[1] + %s*y[0]*y[1]]", pyNum(pa), pyNum(pb), pyNum(-pc), pyNum(pd)),
		check: func(s *integrate.Solution) []Row {
			v0, worst := inv(s.Y[0]), 0.0
			for _, y := range s.Y {
				worst = math.Max(worst, math.Abs(inv(y)-v0)/math.Abs(v0))
			}
			return []Row{{"V(0), V = d·x − c·ln x + b·y − a·ln y", G(v0)}, {"max |V(t) − V(0)| / |V(0)| over the steps", G(worst)}}
		},
		extra: func(s *integrate.Solution) []Plot {
			x, y := column(s, 0), column(s, 1)
			v0 := inv(s.Y[0])
			d, raw := make([]float64, len(s.T)), make([]float64, len(s.T))
			for i, yy := range s.Y {
				raw[i] = inv(yy) - v0
				d[i] = math.Log10(math.Max(math.Abs(raw[i])/math.Abs(v0), 1e-17))
			}
			return []Plot{
				{Name: "phase portrait", XLabel: "prey x", YLabel: "predators y", Series: []Series{{Label: "orbit", X: x, Y: y}}},
				{Name: "drift of the invariant V", XLabel: "t", YLabel: "log10 |V − V(0)| / |V(0)|",
					Series: []Series{{Label: "drift", X: s.T, Y: d, Raw: raw, RawName: "V − V(0)"}}},
			}
		},
	}, nil
}

func buildVdP(a *Args) (*odeProb, error) {
	v, err := floats(a, "mu", "tend")
	if err != nil {
		return nil, err
	}
	mu, tend := v[0], v[1]
	p := &odeProb{
		f: func(t float64, y, dydt []float64) {
			dydt[0] = y[1]
			dydt[1] = mu*(1-y[0]*y[0])*y[1] - y[0]
		},
		y0: []float64{2, 0}, tend: tend, names: []string{"y", "y′"},
		goBody: fmt.Sprintf("dydt[0] = y[1]\ndydt[1] = %s*(1-y[0]*y[0])*y[1] - y[0]", goNum(mu)),
		pyRet:  fmt.Sprintf("[y[1], %s*(1 - y[0]**2)*y[1] - y[0]]", pyNum(mu)),
	}
	p.check = func(s *integrate.Solution) []Row {
		method, opts := integrate.DOP853, &integrate.IVPOptions{Method: integrate.DOP853, RTol: 1e-13, ATol: 1e-15}
		if math.Abs(mu) >= 50 {
			method, opts = integrate.Radau, &integrate.IVPOptions{Method: integrate.Radau, RTol: 1e-10, ATol: 1e-12}
		}
		name := method.String() + fmt.Sprintf(", rtol %s", G(opts.RTol))
		var ref *integrate.Solution
		var rerr error
		t := &tap{}
		f := integrate.ODEFunc(func(tt float64, y, dydt []float64) { t.count(); p.f(tt, y, dydt) })
		if err := guard(func() { ref, rerr = integrate.SolveIVP(f, [2]float64{0, tend}, p.y0, opts) }); err != nil {
			return []Row{{"reference at the end (" + name + ")", "not computed: " + err.Error()}}
		}
		if rerr != nil {
			return []Row{{"reference at the end (" + name + ")", "not computed: " + rerr.Error()}}
		}
		ye, re := s.Y[len(s.Y)-1], ref.Y[len(ref.Y)-1]
		return []Row{
			{"reference y at the end (" + name + ")", Gs(re)},
			{"max |y − reference| at the end", G(math.Max(math.Abs(ye[0]-re[0]), math.Abs(ye[1]-re[1])))},
		}
	}
	p.extra = func(s *integrate.Solution) []Plot {
		return []Plot{{Name: "phase portrait", XLabel: "y", YLabel: "y′", Series: []Series{{Label: "orbit", X: column(s, 0), Y: column(s, 1)}}}}
	}
	return p, nil
}

func buildStiff(a *Args) (*odeProb, error) {
	v, err := floats(a, "lambda", "tend")
	if err != nil {
		return nil, err
	}
	lam, tend := v[0], v[1]
	return &odeProb{
		f: func(t float64, y, dydt []float64) {
			dydt[0] = -lam*(y[0]-math.Cos(t)) - math.Sin(t)
		},
		y0: []float64{1}, tend: tend, names: []string{"y"},
		goBody: fmt.Sprintf("dydt[0] = %s*(y[0]-math.Cos(t)) - math.Sin(t)", goNum(-lam)),
		pyRet:  fmt.Sprintf("[%s*(y[0] - np.cos(t)) - np.sin(t)]", pyNum(-lam)),
		exact:  math.Cos,
		check: func(s *integrate.Solution) []Row {
			worst := 0.0
			for i, t := range s.T {
				worst = math.Max(worst, math.Abs(s.Y[i][0]-math.Cos(t)))
			}
			end := len(s.T) - 1
			return []Row{{"exact y at the end, cos t", G(math.Cos(s.T[end]))},
				{"max |y − cos t| over the steps", G(worst)}, {"|y − cos t| at the end", G(math.Abs(s.Y[end][0] - math.Cos(s.T[end])))}}
		},
	}, nil
}

func floats(a *Args, keys ...string) ([]float64, error) {
	out := make([]float64, len(keys))
	for i, k := range keys {
		v, err := a.Float(k)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// column is component i of every stored state.
func column(s *integrate.Solution, i int) []float64 {
	out := make([]float64, len(s.Y))
	for k, y := range s.Y {
		out[k] = y[i]
	}
	return out
}

func (c odeCase) solve(method string, a *Args) (*run, error) {
	p, err := c.build(a)
	if err != nil {
		return nil, err
	}
	rtol, err := a.Float("rtol")
	if err != nil {
		return nil, err
	}
	atol, err := a.Float("atol")
	if err != nil {
		return nil, err
	}
	opts := &integrate.IVPOptions{Method: odeMethod[method], RTol: rtol, ATol: atol}
	t := &tap{}
	f := integrate.ODEFunc(func(tt float64, y, dydt []float64) { t.count(); p.f(tt, y, dydt) })
	var sol *integrate.Solution
	var cerr error
	if err := guard(func() { sol, cerr = integrate.SolveIVP(f, [2]float64{0, p.tend}, p.y0, opts) }); err != nil {
		return nil, err
	}
	out := &Result{Title: c.name + " — " + method}
	if cerr != nil {
		out.Err = cerr.Error()
	}
	if sol != nil && len(sol.T) > 0 {
		end := len(sol.T) - 1
		out.Status, out.Converged = sol.Message, sol.Success
		out.Summary = []Row{
			{"t at the end", G(sol.T[end])}, {"y at the end", Gs(sol.Y[end])},
			{"accepted steps", fmt.Sprint(sol.NSteps)}, {"right-hand side evaluations", fmt.Sprint(sol.NFev)},
			{"Jacobian evaluations", fmt.Sprint(sol.NJev)}, {"LU decompositions", fmt.Sprint(sol.NLU)},
			{"success", fmt.Sprint(sol.Success)}, {"status", fmt.Sprint(sol.Status)}, {"message", sol.Message},
		}
		out.Check = p.check(sol)
		out.Plots = c.plots(p, sol)
	}
	out.Go, out.Python = c.code(p, method, rtol, atol)
	return &run{res: out, bare: func() { _, _ = integrate.SolveIVP(p.f, [2]float64{0, p.tend}, p.y0, opts) }}, nil
}

func (c odeCase) plots(p *odeProb, s *integrate.Solution) []Plot {
	var series []Series
	for i, name := range p.names {
		series = append(series, Series{Label: name, X: s.T, Y: column(s, i)})
	}
	if p.exact != nil {
		ex := make([]float64, len(s.T))
		for i, t := range s.T {
			ex[i] = p.exact(t)
		}
		series = append(series, Series{Label: "exact", X: s.T, Y: ex})
	}
	plots := []Plot{{Name: "the solution at the accepted steps", XLabel: "t", YLabel: "y", Series: series}}
	if p.extra != nil {
		plots = append(plots, p.extra(s)...)
	}
	if len(s.T) > 1 {
		h := make([]float64, len(s.T)-1)
		lg := make([]float64, len(h))
		for i := range h {
			h[i] = s.T[i+1] - s.T[i]
			lg[i] = math.Log10(math.Abs(h[i]))
		}
		plots = append(plots, Plot{Name: "step size", XLabel: "t", YLabel: "log10 h",
			Series: []Series{{Label: "log10 h", X: s.T[1:], Y: lg, Raw: h, RawName: "h"}}})
	}
	return plots
}

var goODEMethod = map[string]string{"RK45": "RK45", "DOP853": "DOP853", "RK23": "RK23", "Radau": "Radau", "BDF": "BDF", "LSODA": "LSODA"}

func (c odeCase) code(p *odeProb, method string, rtol, atol float64) (string, string) {
	var g strings.Builder
	g.WriteString("f := func(t float64, y, dydt []float64) {\n" + indent(p.goBody) + "}\n")
	fmt.Fprintf(&g, "sol, err := integrate.SolveIVP(f, [2]float64{0, %s}, %s,\n\t&integrate.IVPOptions{Method: integrate.%s, RTol: %s, ATol: %s})\n",
		goNum(p.tend), goNums(p.y0), goODEMethod[method], goNum(rtol), goNum(atol))
	g.WriteString("if err != nil {\n\tfmt.Println(\"error:\", err)\n}\nif sol == nil || len(sol.T) == 0 {\n\treturn\n}\nend := len(sol.T) - 1\n")
	g.WriteString(printRow("t at the end", "sol.T[end]"))
	g.WriteString(printRow("y at the end", "sol.Y[end]"))
	g.WriteString(printRow("accepted steps", "sol.NSteps"))
	g.WriteString(printRow("right-hand side evaluations", "sol.NFev"))
	g.WriteString(printRow("Jacobian evaluations", "sol.NJev"))
	g.WriteString(printRow("LU decompositions", "sol.NLU"))
	g.WriteString(printRow("success", "sol.Success"))
	g.WriteString(printRow("status", "sol.Status"))
	g.WriteString(printRow("message", "sol.Message"))
	goSrc := goFile([]string{"fmt", "math", "github.com/go-numerics/integrate"}, indent(g.String()))

	var q strings.Builder
	q.WriteString("import numpy as np\nfrom scipy.integrate import solve_ivp\n\n")
	fmt.Fprintf(&q, "def f(t, y):\n    return %s\n\n", p.pyRet)
	fmt.Fprintf(&q, "sol = solve_ivp(f, (0, %s), %s, method=%q, rtol=%s, atol=%s)\n", pyNum(p.tend), pyNums(p.y0), method, pyNum(rtol), pyNum(atol))
	q.WriteString("print(sol.t[-1], sol.y[:, -1], len(sol.t) - 1, sol.nfev, sol.njev, sol.nlu, sol.success, sol.status, sol.message)\n")
	return goSrc, q.String()
}
