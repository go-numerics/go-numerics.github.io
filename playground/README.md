# go-numerics playground

The page at <https://go-numerics.github.io/playground/>: go-numerics itself
(`optimize`, `integrate`, `special`), compiled to WebAssembly, inside a
[go-widgets](https://github.com/go-widgets) canvas application. Nothing is
computed in JavaScript — the page only loads the wasm, forwards input and
follows the landing's theme toggle.

A visitor picks a problem family and a case, a solver, and edits the
parameters (one `key=value  # what it is` per line):

| family | cases | solvers |
|---|---|---|
| Scalar root | cos x = x, x³−2x−5, x eˣ = 1, tan x = x, x² = c | Brentq, Ridder, Brenth, Bisect, Newton, Secant |
| Minimisation | Rosenbrock (any dimension), Himmelblau, Booth, Beale | BFGS, L-BFGS-B, CG, Powell, Nelder–Mead; analytic or finite-difference gradient |
| Curve fit | exponential decay, Gaussian peak, logistic (seeded noise) | LM, TRF |
| Quadrature | sin x, 1/√x, e^(−x²) over the line, ln x, a step | Quad (QUADPACK), Romberg, FixedQuad |
| ODE | Lotka–Volterra, Van der Pol (μ), a stiff relaxation (λ) | RK45, DOP853, RK23, Radau, BDF, LSODA |
| Special functions | Γ, ln\|Γ\|, erf, ψ, Φ, J₀, Y₀, I₀, K₀, K(m) | scalar, and the vectorised `…To` form where the library has one |

and sees: the library's numbers (value, `nfev`/`nit`/`nsteps`, its status and
its own error text), a **check against mathematics** (a closed form, a
conserved quantity, an identity between functions, a reference solution; never
the library against itself), plots with a hover readout of the exact values
the library computed at the pointer, the time of one **uninstrumented** call in
the browser, and the equivalent code, copyable, in four tabs: Go, Ruby, Julia,
SciPy.

## Ruby and Julia

* **Ruby** is [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby),
  which binds go-numerics itself (`Optimize`, `Integrate`, `Special`): a Ruby
  program calls the same functions as the page and prints the same numbers. It
  binds less than the page offers, and where it does not, the tab says so
  instead of showing a program nobody ran: no ODE module at all, no analytic
  gradient (`jac:`) for `minimize`, no `method:` or `maxfev:` for `curve_fit`,
  no `…To` vector form. `TestRubyPrintsWhatThePageShows` builds go-embedded-ruby
  (`rubycheck/` pins its version), runs every Ruby program and compares each row
  with the page's: exactly on amd64, to 1e-6 elsewhere (arm64 fuses
  multiply-adds in the test binary's own closures, not in the interpreter).
* **Julia** is Julia's own packages — Roots, Optim, LsqFit, QuadGK,
  OrdinaryDiffEq, SpecialFunctions, Elliptic — because go-embedded-julia does
  not run Base yet (its README: M3 in progress), so no program here runs on it;
  each says so in its first line. `TestJuliaAgreesWithThePage` runs every one on
  julia 1.12.6 with the packages `engine/testdata/juliaenv` pins and compares the
  answer (root, minimiser, parameters, integral, the state at the end of the
  span, the sweep's statistics) with the page's, to the problem's tolerance: the
  algorithms are the packages', so the counts differ. Where a package has no
  equivalent the tab says so: no Brent–Haggerty `brenth` (Roots), no Powell
  (Optim), LM only (LsqFit), no Romberg (QuadGK). LSODA maps to
  `AutoTsit5(Rosenbrock23())`, BDF to `QNDF`, which are the same idea, not the
  same method. The test needs julia: `PLAYGROUND_JULIA=julia` and, in CI,
  `PLAYGROUND_REQUIRE_JULIA=1`.
* Running either in the page is not done: go-embedded-ruby is a 23 MB wasm
  (go-ndarray's playground fetches it on demand) and go-embedded-julia has no
  wasm yet.

## Layout

| package | role |
|---|---|
| `engine` | the model: the catalogue, argument parsing, running a case (counted and bounded), the checks, the plots, the timing, and the Go and SciPy spellings |
| `vm` | the ViewModel: every piece of state as a `go-widgets/mvvm` Observable, ObservableList or Command; no widget |
| `.` (`playground`) | the View: toolkit widgets (`DropDown`, `TextView`, `Table`, `XYPlot`, `ViewSwitcher`, …) bound to the ViewModel (`mvvmtk`, plus `view_binding.go`); events through the root box |
| `cmd/playground-wasm` | the browser shell: canvas at device pixels, input forwarding, `numericsSetTheme`, `numericsDebug`/`numericsRects` for the proofs |
| `internal/covergate` | the 100% statement gate, a Go program with a test |

No widget was written for this page: the plot is the toolkit's `XYPlot`, whose
own pointer tracking the ViewModel turns into the readout.

## What is proved

* `engine` tests check every case and method against mathematics: closed-form
  roots and integrals, known minimisers (Himmelblau's four, polished here by
  Newton's method on the gradient), recovery of the curve a fit's data were
  drawn from, the Lotka–Volterra invariant, `cos t` as the exact solution of
  the stiff problem for every method, a high-accuracy reference for Van der Pol,
  and for each special function an identity (recurrence, Wronskian, AGM, erf +
  erfc = 1) holding to rounding.
* `TestGeneratedGoPrintsWhatThePageShows` writes the Go program of every case
  and method (about a hundred), checks it is gofmt-clean, builds it against the
  module versions pinned here, runs it, and compares each printed row with the
  page's, to the last digit. The SciPy snippets are text; they are not run in CI
  (`scipy.integrate.romberg` was removed in SciPy 1.15, which the Romberg snippet
  says).
* Work is bounded: a case that needs more than 5 million evaluations of the
  function stops with a message of the page, not a frozen tab.
* The browser proof (`browser_test.go` + `browsertest/driver.cjs`) builds the
  site with Hugo and drives it in headless Chrome, light and dark forced, 1x and
  2x, and a phone (390x844 at 3x): real clicks, typing, menu choices and pointer
  moves, asserted on the app's state and on canvas pixels (the curve is in the
  accent colour, the readout equals `cos x − x` at the hovered x).

## Build and test

```sh
GOWORK=off go test -race ./...                       # 100% statement coverage in CI
GOWORK=off GOOS=js GOARCH=wasm go test \
  -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./engine ./vm
GOOS=js GOARCH=wasm go build -o ../static/playground/playground.wasm ./cmd/playground-wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" ../static/playground/
```

The wasm is about 11.5 MB (3.4 MB gzipped, as Pages serves it). Neither it nor
`wasm_exec.js` is committed: `hugo.yml` builds both from the deployed commit.
Timings are single-threaded (`GOMAXPROCS` is 1 under `GOOS=js`) and without
SIMD kernels, so they are slower than native Go; the page says so.

```sh
CHROME=… PLAYGROUND_NODE_PATH=<dir with puppeteer-core> HUGO=hugo \
  GOWORK=off go test -run TestBrowserProof -v .
```

Screenshots go to `$PLAYGROUND_CAPTURE_DIR`, else the user config directory —
never into a repository (the helper refuses a directory inside a git tree).
