# go-numerics.github.io

The go-numerics landing page (Hugo), served at <https://go-numerics.github.io/>.

The layout is the go-* family template (go-fileshare.github.io's, through
`logos/landing-kit`, as go-plot and go-notebook use it): `layouts/index.html`,
the shared stylesheet `layouts/partials/styles.html`, and `[params.brand]` in
`hugo.toml`, read from the logo's gradient.

The figure is real: `figure/main.go` solves Lotka–Volterra with
go-numerics/integrate and draws it with go-plot into `static/img/figure.svg`,
and the page's terminal shows that file, read at build time. With `-readout`
it also writes `static/img/figure.json`: the dense output at 3001 times and the
accepted steps, as full float64, and the plot area of the SVG. The page reads
it to show, under the pointer (or the arrow keys), the solver's exact values.
The `ci` workflow writes both again and fails if a byte differs — on an arm64
runner, because Go fuses multiply-adds on arm64 and not on amd64, and the JSON's
full float64 values differ in the last bit between the two: regenerate it on an
arm64 machine (a Mac). A headless-Chrome test (`figure/browser_test.go`) then
checks that the built page prints exactly those values.

```sh
cd figure && go run . -readout ../static/img/figure.json > ../static/img/figure.svg
hugo server
```

| Workflow | What |
| --- | --- |
| `hugo.yml` | builds on pull requests; builds and deploys to Pages on `main` |
| `ci.yml` | the site builds and names every repository; the figure is what its program draws |
| `links.yml` | every link and image of the built site resolves |
| `playground.yml` | the playground: gofmt, vet, 100% coverage, the suites run as WebAssembly, and the browser proof in headless Chrome |

BSD-3-Clause.

## The playground

<https://go-numerics.github.io/playground/> is go-numerics itself, compiled to
WebAssembly, inside a go-widgets canvas application: pick a problem (scalar
roots, minimisation, curve fitting, quadrature, ODE solvers, special
functions), a solver and its parameters, and see the library's own numbers, a
check of the answer against mathematics, a plot with the exact computed value
under the pointer, the time of one call, and the equivalent Go and SciPy
programs. It lives in `playground/` (a nested Go module; its README describes
the layout and the proofs). `hugo.yml` builds the wasm at deploy time; it is
never committed.

The generic go-notebook playground, with the example notebooks, moved to
<https://go-numerics.github.io/notebooks/> (`notebooks/`, built by the same
workflow) and is linked from the playground's nav bar.

## The design is not here

It comes from the shared Hugo module
[`go-fleettools/landingkit`](https://github.com/go-fleettools/landingkit),
pinned in `go.mod` (Renovate bumps it); `kit.go` keeps `go mod tidy` from
deleting that requirement. `[params.brand]` in `hugo.toml` is read from the logo by
`landingkit brand`. A file under `layouts/partials/` with the name of one of the
kit's would silently override it.
