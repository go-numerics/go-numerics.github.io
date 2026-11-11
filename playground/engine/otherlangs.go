// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"strings"
)

// The Ruby and Julia programs. Ruby is go-embedded-ruby, which binds
// go-numerics itself: its Optimize, Integrate and Special modules call the
// same functions as the page, so a Ruby program prints the page's numbers. It
// has no ODE module and no Jacobian, method or bounds options where the page
// has them; those cases say so instead of offering a program nobody ran.
//
// Julia is the language's own packages (Roots, Optim, LsqFit, QuadGK,
// OrdinaryDiffEq, SpecialFunctions, Elliptic): go-embedded-julia does not run
// Base yet, so no program here runs on it; they run on julia 1.12.6, and
// engine's tests run every one of them there and compare the answer. Their
// algorithms are the packages' own, so the answer agrees to the problem's
// tolerance, not to the last digit, and the counts differ.
//
// A program that does not exist is a comment beginning with unavailable.

// unavailable is the text of a panel with no program, and why.
const unavailable = "# No program: "

func none(why string) string { return unavailable + why + "\n" }

// headers says, on each program, where it was run: a reader of the page must
// not take a Julia program for one that go-embedded-julia ran.
func headers(ruby, julia string) (string, string) {
	if HasProgram(ruby) {
		ruby = "# On go-embedded-ruby: Optimize, Integrate and Special are go-numerics itself, so this prints the page's numbers.\n\n" + ruby
	}
	if HasProgram(julia) {
		julia = "# On julia 1.12 with these packages (go-embedded-julia does not run Base yet). Their algorithms are\n" +
			"# the packages' own: the answer agrees to the problem's tolerance, the counts differ.\n\n" + julia
	}
	return ruby, julia
}

// HasProgram reports whether a code panel holds a program rather than the
// reason there is none.
func HasProgram(code string) bool { return !strings.HasPrefix(code, unavailable) }

// rbNum and jlNum spell a float64 as a float literal.
func rbNum(v float64) string {
	switch {
	case math.IsNaN(v):
		return "Float::NAN"
	case math.IsInf(v, 1):
		return "Float::INFINITY"
	case math.IsInf(v, -1):
		return "-Float::INFINITY"
	}
	return floatLit(v)
}

func jlNum(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Inf"
	case math.IsInf(v, -1):
		return "-Inf"
	}
	return floatLit(v)
}

// floatLit is G(v) with ".0" added when it would read as an integer.
func floatLit(v float64) string {
	s := G(v)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}

func rbNums(v []float64) string { return "[" + joinMap(v, rbNum) + "]" }
func jlNums(v []float64) string { return "[" + joinMap(v, jlNum) + "]" }

func joinMap(v []float64, f func(float64) string) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = f(x)
	}
	return strings.Join(parts, ", ")
}

// rbExpr and jlExpr translate the Go spelling of an expression in x (and t)
// into Ruby and Julia: the math functions the catalogue uses.
var rbMath = strings.NewReplacer("math.Cos", "Math.cos", "math.Sin", "Math.sin", "math.Exp", "Math.exp",
	"math.Tan", "Math.tan", "math.Sqrt", "Math.sqrt", "math.Log", "Math.log")
var jlMath = strings.NewReplacer("math.Cos", "cos", "math.Sin", "sin", "math.Exp", "exp",
	"math.Tan", "tan", "math.Sqrt", "sqrt", "math.Log", "log")

// jlIndex shifts the 0-based indexes of a Go expression to Julia's 1-based.
var jlIndex = strings.NewReplacer("[3]", "[4]", "[2]", "[3]", "[1]", "[2]", "[0]", "[1]")

// ---- scalar roots -----------------------------------------------------------

func (rc rootCase) rubyJulia(fn *rootFn, method string, lo, hi, x0, tol, rtol float64, maxiter int) (string, string) {
	var r, j strings.Builder
	r.WriteString("f = ->(x) { " + rbMath.Replace(fn.goF) + " }\n")
	switch method {
	case "Newton", "Secant":
		args := rbNum(x0) + ", "
		if method == "Newton" {
			r.WriteString("fprime = ->(x) { " + rbMath.Replace(fn.goD) + " }\n")
			args += "fprime: fprime, "
		}
		fmt.Fprintf(&r, "r = Optimize.newton(%stol: %s, rtol: %s, maxiter: %d, full_output: true, &f)\n", args, rbNum(tol), rbNum(rtol), maxiter)
	default:
		fmt.Fprintf(&r, "r = Optimize.%s(%s, %s, xtol: %s, rtol: %s, maxiter: %d, full_output: true, &f)\n",
			strings.ToLower(method), rbNum(lo), rbNum(hi), rbNum(tol), rbNum(rtol), maxiter)
	}
	for _, p := range [][2]string{{"root", "r[:root]"}, {"f(root)", "f.call(r[:root])"}, {"iterations", "r[:iterations]"},
		{"function calls", "r[:function_calls]"}, {"converged", "r[:converged]"}, {"flag", "r[:flag]"}} {
		fmt.Fprintf(&r, "puts \"%s = #{%s}\"\n", p[0], p[1])
	}
	ruby := r.String()

	if method == "Brenth" {
		return ruby, none("Roots.jl has no Brent–Haggerty (brenth) method.")
	}
	j.WriteString("using Roots\n\n")
	j.WriteString("f(x) = " + jlMath.Replace(fn.goF) + "\n")
	opts := fmt.Sprintf("xatol = %s, xrtol = %s, maxiters = %d", jlNum(tol), jlNum(rtol), maxiter)
	switch method {
	case "Newton":
		j.WriteString("fprime(x) = " + jlMath.Replace(fn.goD) + "\n")
		fmt.Fprintf(&j, "r = find_zero((f, fprime), %s, Roots.Newton(); %s)\n", jlNum(x0), opts)
	case "Secant":
		fmt.Fprintf(&j, "r = find_zero(f, %s, Roots.Secant(); %s)\n", jlNum(x0), opts)
	default:
		alg := map[string]string{"Brentq": "Roots.Brent()", "Ridder": "Roots.Ridders()", "Bisect": "Roots.Bisection()"}[method]
		fmt.Fprintf(&j, "r = find_zero(f, (%s, %s), %s; %s)\n", jlNum(lo), jlNum(hi), alg, opts)
	}
	j.WriteString("println(\"root = \", r)\nprintln(\"f(root) = \", f(r))\n")
	return ruby, j.String()
}

// ---- minimisation -----------------------------------------------------------

var minRubyF = map[string]string{
	"Rosenbrock": `s = 0.0
(0...x.size - 1).each do |i|
  s += 100*(x[i+1]-x[i]*x[i])*(x[i+1]-x[i]*x[i]) + (1-x[i])*(1-x[i])
end
s`,
	"Himmelblau": "a, b = x[0]*x[0]+x[1]-11, x[0]+x[1]*x[1]-7\na*a + b*b",
	"Booth":      "a, b = x[0]+2*x[1]-7, 2*x[0]+x[1]-5\na*a + b*b",
	"Beale": `t1, t2, t3 = 1.5-x[0]+x[0]*x[1], 2.25-x[0]+x[0]*x[1]*x[1], 2.625-x[0]+x[0]*x[1]*x[1]*x[1]
t1*t1 + t2*t2 + t3*t3`,
}

var minJuliaF = map[string]string{
	"Rosenbrock": "f(x) = sum(100*(x[i+1]-x[i]*x[i])^2 + (1-x[i])^2 for i in 1:length(x)-1)",
	"Himmelblau": "f(x) = (x[1]^2 + x[2] - 11)^2 + (x[1] + x[2]^2 - 7)^2",
	"Booth":      "f(x) = (x[1] + 2*x[2] - 7)^2 + (2*x[1] + x[2] - 5)^2",
	"Beale":      "f(x) = (1.5 - x[1] + x[1]*x[2])^2 + (2.25 - x[1] + x[1]*x[2]^2)^2 + (2.625 - x[1] + x[1]*x[2]^3)^2",
}

var minJuliaG = map[string]string{
	"Rosenbrock": `function g!(G, x)
    fill!(G, 0)
    for i in 1:length(x)-1
        G[i] += -400*x[i]*(x[i+1]-x[i]*x[i]) - 2*(1-x[i])
        G[i+1] += 200*(x[i+1]-x[i]*x[i])
    end
end`,
	"Himmelblau": `function g!(G, x)
    a, b = x[1]^2 + x[2] - 11, x[1] + x[2]^2 - 7
    G[1] = 4*x[1]*a + 2*b
    G[2] = 2*a + 4*x[2]*b
end`,
	"Booth": `function g!(G, x)
    a, b = x[1] + 2*x[2] - 7, 2*x[1] + x[2] - 5
    G[1] = 2*a + 4*b
    G[2] = 4*a + 2*b
end`,
	"Beale": `function g!(G, x)
    t1, t2, t3 = 1.5 - x[1] + x[1]*x[2], 2.25 - x[1] + x[1]*x[2]^2, 2.625 - x[1] + x[1]*x[2]^3
    G[1] = 2*t1*(x[2]-1) + 2*t2*(x[2]^2-1) + 2*t3*(x[2]^3-1)
    G[2] = 2*t1*x[1] + 2*t2*(2*x[1]*x[2]) + 2*t3*(3*x[1]*x[2]^2)
end`,
}

var juliaOptim = map[string]string{"BFGS": "BFGS()", "L-BFGS-B": "LBFGS()", "CG": "ConjugateGradient()", "Nelder-Mead": "NelderMead()"}

func (mp minProblem) rubyJulia(method string, x0 []float64, mode string, tol float64, maxiter int) (string, string) {
	var ruby string
	if mode != "fd" {
		ruby = none("Optimize.minimize takes no jac: — it differentiates by finite differences. Set grad=fd to get the Ruby program.")
	} else {
		var r strings.Builder
		opts := fmt.Sprintf("method: %q", method)
		if tol != 0 {
			opts += ", tol: " + rbNum(tol)
		}
		if maxiter > 0 {
			opts += fmt.Sprintf(", maxiter: %d", maxiter)
		}
		fmt.Fprintf(&r, "f = ->(x) do\n%s\nend\n", indent(minRubyF[mp.name]))
		fmt.Fprintf(&r, "r = Optimize.minimize(%s, %s, &f)\n", rbNums(x0), opts)
		for _, p := range [][2]string{{"x", "r[:x].inspect"}, {"f(x)", "r[:fun]"}, {"iterations", "r[:nit]"},
			{"function evaluations", "r[:nfev]"}, {"gradient evaluations", "r[:njev]"}, {"success", "r[:success]"},
			{"status", "r[:status]"}, {"message", "r[:message]"}} {
			fmt.Fprintf(&r, "puts \"%s = #{%s}\"\n", p[0], p[1])
		}
		ruby = r.String()
	}

	alg, ok := juliaOptim[method]
	if !ok {
		return ruby, none("Optim.jl has no Powell method.")
	}
	var j strings.Builder
	if method != "Nelder-Mead" && mode == "fd" {
		j.WriteString("using Optim, ADTypes, FiniteDiff\n\n")
	} else {
		j.WriteString("using Optim\n\n")
	}
	j.WriteString(minJuliaF[mp.name] + "\n")
	var opts []string
	if tol != 0 && method != "Nelder-Mead" {
		opts = append(opts, "g_tol = "+jlNum(tol))
	}
	if maxiter > 0 {
		opts = append(opts, fmt.Sprintf("iterations = %d", maxiter))
	}
	o := "Optim.Options(" + strings.Join(opts, ", ") + ")"
	switch {
	case method == "Nelder-Mead":
		fmt.Fprintf(&j, "res = optimize(f, %s, %s, %s)\n", jlNums(x0), alg, o)
	case mode == "analytic":
		j.WriteString(minJuliaG[mp.name] + "\n")
		fmt.Fprintf(&j, "res = optimize(f, g!, %s, %s, %s)\n", jlNums(x0), alg, o)
	default:
		fmt.Fprintf(&j, "res = optimize(f, %s, %s, %s; autodiff = AutoFiniteDiff())\n", jlNums(x0), alg, o)
	}
	j.WriteString("println(\"x = \", Optim.minimizer(res))\nprintln(\"f(x) = \", Optim.minimum(res))\n")
	return ruby, j.String()
}

// ---- curve fit --------------------------------------------------------------

func (m fitModel) rubyJulia(method string, p0 []float64, maxfev int, xs, ys []float64) (string, string) {
	var ruby string
	switch {
	case method != "LM":
		ruby = none("Optimize.curve_fit is Levenberg–Marquardt only: it takes no method:.")
	case maxfev != 0:
		ruby = none("Optimize.curve_fit takes no maxfev: (set maxfev=0).")
	default:
		var r strings.Builder
		fmt.Fprintf(&r, "# The data the page drew (the model at %s plus Gaussian noise).\n", Gs(m.truth))
		fmt.Fprintf(&r, "xs = %s\nys = %s\n\n", rbNums(xs), rbNums(ys))
		fmt.Fprintf(&r, "r = Optimize.curve_fit(xs, ys, %s, full_output: true) { |x, p| %s }\n", rbNums(p0), rbMath.Replace(m.goF))
		r.WriteString("n = r[:popt].size\nperr = (0...n).map { |i| Math.sqrt(r[:pcov][i * n + i]) }\n")
		for _, p := range [][2]string{{"popt", "r[:popt].inspect"}, {"perr", "perr.inspect"}, {"function evaluations", "r[:nfev]"},
			{"covariance estimated", "r[:cov_ok]"}, {"status", "r[:status]"}, {"message", "r[:message].split.join(\" \")"}} {
			fmt.Fprintf(&r, "puts \"%s = #{%s}\"\n", p[0], p[1])
		}
		ruby = r.String()
	}
	if method != "LM" {
		return ruby, none("LsqFit.jl is Levenberg–Marquardt only: it has no trust-region reflective method.")
	}
	var j strings.Builder
	j.WriteString("using LsqFit\n\n")
	fmt.Fprintf(&j, "model(x, p) = @. %s\n", jlIndex.Replace(jlMath.Replace(m.goF)))
	fmt.Fprintf(&j, "xs = %s\nys = %s\n\n", jlNums(xs), jlNums(ys))
	extra := ""
	if maxfev > 0 {
		extra = fmt.Sprintf("; maxIter = %d", maxfev)
	}
	fmt.Fprintf(&j, "fit = curve_fit(model, xs, ys, %s%s)\n", jlNums(p0), extra)
	j.WriteString("println(\"popt = \", coef(fit))\nprintln(\"perr = \", stderror(fit))\n")
	return ruby, j.String()
}

// ---- quadrature -------------------------------------------------------------

var quadRubyBody = map[string]string{"step at 1/3": "x < 1.0/3 ? 1.0 : 2.0"}
var quadJuliaBody = map[string]string{"step at 1/3": "x < 1/3 ? 1.0 : 2.0"}

func (q quadCase) rubyJulia(method string, lo, hi, epsabs, epsrel float64, limit int, points []float64, tol, rtol float64, divmax, n int) (string, string) {
	rbBody, jlBody := rbMath.Replace(q.goF), jlMath.Replace(q.goF)
	if b, ok := quadRubyBody[q.name]; ok {
		rbBody, jlBody = b, quadJuliaBody[q.name]
	}
	var r strings.Builder
	r.WriteString("calls = 0\n")
	fmt.Fprintf(&r, "f = ->(x) { calls += 1; %s }\n", rbBody)
	switch method {
	case "Quad":
		opts := fmt.Sprintf("epsabs: %s, epsrel: %s, limit: %d", rbNum(epsabs), rbNum(epsrel), limit)
		if points != nil {
			opts += ", points: " + rbNums(points)
		}
		fmt.Fprintf(&r, "r = Integrate.quad(%s, %s, %s, full_output: true, &f)\n", rbNum(lo), rbNum(hi), opts)
		for _, p := range [][2]string{{"value", "r[:value]"}, {"estimated error", "r[:abserr]"},
			{"function evaluations", "r[:neval]"}, {"subintervals", "r[:nintervals]"}} {
			fmt.Fprintf(&r, "puts \"%s = #{%s}\"\n", p[0], p[1])
		}
	case "Romberg":
		fmt.Fprintf(&r, "value = Integrate.romberg(%s, %s, tol: %s, rtol: %s, divmax: %d, &f)\n", rbNum(lo), rbNum(hi), rbNum(tol), rbNum(rtol), divmax)
		r.WriteString("puts \"value = #{value}\"\nputs \"function evaluations = #{calls}\"\n")
	default:
		fmt.Fprintf(&r, "value = Integrate.fixed_quad(%s, %s, %d, &f)\n", rbNum(lo), rbNum(hi), n)
		r.WriteString("puts \"value = #{value}\"\nputs \"function evaluations = #{calls}\"\n")
	}
	ruby := r.String()

	if method == "Romberg" {
		return ruby, none("QuadGK.jl is adaptive Gauss–Kronrod; Romberg's rule is in Romberg.jl, which this page does not run.")
	}
	var j strings.Builder
	j.WriteString("using QuadGK\n\n")
	fmt.Fprintf(&j, "f(x) = %s\n", jlBody)
	if method == "FixedQuad" {
		fmt.Fprintf(&j, "x, w = gauss(%d, %s, %s)\nvalue = sum(w .* f.(x))\nprintln(\"value = \", value)\n", n, jlNum(lo), jlNum(hi))
		return ruby, j.String()
	}
	ends := jlNum(lo) + ", " + jlNum(hi)
	if points != nil {
		ends = jlNum(lo) + ", " + joinMap(points, jlNum) + ", " + jlNum(hi)
	}
	fmt.Fprintf(&j, "value, err = quadgk(f, %s; atol = %s, rtol = %s)\n", ends, jlNum(epsabs), jlNum(epsrel))
	j.WriteString("println(\"value = \", value)\nprintln(\"estimated error = \", err)\n")
	return ruby, j.String()
}

// ---- ODEs -------------------------------------------------------------------

var juliaODE = map[string]string{
	"RK45": "DP5()", "DOP853": "DP8()", "RK23": "BS3()", "Radau": "RadauIIA5()", "BDF": "QNDF()",
	"LSODA": "AutoTsit5(Rosenbrock23())",
}

// juliaODEPkg is the package of OrdinaryDiffEq's family that holds each solver.
var juliaODEPkg = map[string]string{
	"RK45": "OrdinaryDiffEqLowOrderRK", "RK23": "OrdinaryDiffEqLowOrderRK", "DOP853": "OrdinaryDiffEqHighOrderRK",
	"Radau": "OrdinaryDiffEqFIRK", "BDF": "OrdinaryDiffEqBDF",
	"LSODA": "OrdinaryDiffEqTsit5, OrdinaryDiffEqRosenbrock",
}

func (c odeCase) rubyJulia(p *odeProb, method string, rtol, atol float64) (string, string) {
	ruby := none("go-embedded-ruby has no ODE module (Integrate holds quadrature only).")
	var j strings.Builder
	fmt.Fprintf(&j, "using %s\n\n", juliaODEPkg[method])
	body := jlIndex.Replace(jlMath.Replace(p.goBody))
	j.WriteString("function f!(dydt, y, p, t)\n" + indent(body) + "end\n\n")
	fmt.Fprintf(&j, "prob = ODEProblem(f!, %s, (0.0, %s))\n", jlNums(p.y0), jlNum(p.tend))
	if method == "LSODA" {
		j.WriteString("# LSODA switches between a non-stiff and a stiff method by itself; so does this pair.\n")
	}
	fmt.Fprintf(&j, "sol = solve(prob, %s; reltol = %s, abstol = %s)\n", juliaODE[method], jlNum(rtol), jlNum(atol))
	j.WriteString("println(\"t at the end = \", sol.t[end])\nprintln(\"y at the end = \", sol.u[end])\n")
	return ruby, j.String()
}

// ---- special functions ------------------------------------------------------

var juliaSpecial = map[string][2]string{
	"Gamma":    {"gamma(x)", "SpecialFunctions"},
	"LogGamma": {"first(logabsgamma(x))", "SpecialFunctions"},
	"Erf":      {"erf(x)", "SpecialFunctions"},
	// Julia's digamma is Inf at the poles; go-numerics' and SciPy's is NaN.
	"Digamma": {"let y = digamma(x); isinf(y) ? NaN : y end", "SpecialFunctions"},
	"Ndtr":    {"erfc(-x / sqrt(2)) / 2", "SpecialFunctions"},
	"J0":      {"besselj0(x)", "SpecialFunctions"},
	"Y0":      {"bessely0(x)", "SpecialFunctions"},
	"I0":      {"besseli(0, x)", "SpecialFunctions"},
	"K0":      {"besselk(0, x)", "SpecialFunctions"},
	"EllipK":  {"Elliptic.K(x)", "Elliptic"},
}

func (s specialCase) rubyJulia(vector bool, lo, hi float64, n int) (string, string) {
	var ruby string
	if vector {
		ruby = none("Special's functions are ufuncs over an NDArray, but there is no …To form to call: the scalar program is the Ruby one.")
	} else {
		var r strings.Builder
		fmt.Fprintf(&r, "n = %d\nxs = (0...n).map { |i| %s + (%s - %s) * i / (n - 1) }\n", n, rbNum(lo), rbNum(hi), rbNum(lo))
		fmt.Fprintf(&r, "ys = xs.map { |x| Special.%s(x) }\n", s.pyName)
		r.WriteString("fin = ys.select { |y| y.finite? }\n")
		for _, p := range [][2]string{{"points", "n"}, {"f(first x)", "ys.first"}, {"f(last x)", "ys.last"},
			{"min f (finite points)", "fin.min"}, {"max f (finite points)", "fin.max"},
			{"sum of f (finite points)", "fin.inject(0.0) { |a, y| a + y }"}} {
			fmt.Fprintf(&r, "puts \"%s = #{%s}\"\n", p[0], p[1])
		}
		ruby = r.String()
	}
	spec := juliaSpecial[s.goName]
	var j strings.Builder
	fmt.Fprintf(&j, "using %s\n\n", spec[1])
	fmt.Fprintf(&j, "# Julia raises a DomainError where the function has no value; the page shows NaN.\ng(x) = try %s catch; NaN end\n\n", spec[0])
	fmt.Fprintf(&j, "n = %d\nxs = [%s + (%s - %s) * (i - 1) / (n - 1) for i in 1:n]\n", n, jlNum(lo), jlNum(hi), jlNum(lo))
	if vector {
		j.WriteString("ys = g.(xs)\n")
	} else {
		j.WriteString("ys = [g(x) for x in xs]\n")
	}
	j.WriteString("fin = filter(isfinite, ys)\n")
	for _, p := range [][2]string{{"points", "n"}, {"f(first x)", "ys[1]"}, {"f(last x)", "ys[end]"},
		{"min f (finite points)", "minimum(fin)"}, {"max f (finite points)", "maximum(fin)"}, {"sum of f (finite points)", "sum(fin)"}} {
		fmt.Fprintf(&j, "println(\"%s = \", %s)\n", p[0], p[1])
	}
	return ruby, j.String()
}
