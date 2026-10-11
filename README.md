# go-numerics.github.io

The go-numerics landing page (Hugo), served at <https://go-numerics.github.io/>.

The layout is the go-* family template (go-fileshare.github.io's, through
`logos/landing-kit`, as go-plot and go-notebook use it): `layouts/index.html`,
the shared stylesheet `layouts/partials/styles.html`, and `[params.brand]` in
`hugo.toml`, read from the logo's gradient.

The figure is real: `figure/main.go` solves Lotka–Volterra with
go-numerics/integrate and draws it with go-plot into `static/img/figure.svg`,
and the page's terminal shows that file, read at build time. The `ci`
workflow draws the figure again and fails if a byte differs.

```sh
cd figure && go run . > ../static/img/figure.svg
hugo server
```

| Workflow | What |
| --- | --- |
| `hugo.yml` | builds on pull requests; builds and deploys to Pages on `main` |
| `ci.yml` | the site builds and names every repository; the figure is what its program draws |
| `links.yml` | every link and image of the built site resolves |

BSD-3-Clause.
