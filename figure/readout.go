package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/go-numerics/integrate"
	"github.com/go-plot/plot"
)

// readoutPath is where -readout writes the values the landing shows under
// the pointer (static/img/figure.json); empty writes nothing, so
// `go run . > figure.svg` alone still draws only the figure.
var readoutPath = flag.String("readout", "", "write the pointer readout's values (JSON) to this file")

// readoutSamples is how many times the dense output is evaluated over
// [0, 15] for the readout: one sample every 0.005 in t, about six per
// device pixel of the plot area at the page's widest. Sample i is at
// t = 15*i/(readoutSamples-1), computed in that order on both sides, so the
// page recomputes the exact float64 t from the index.
const readoutSamples = 3001

// readout is static/img/figure.json. Every number is a float64 written by
// encoding/json, which prints the shortest decimal that reads back to the
// same float64: the page shows the solver's values, not a rounding of them.
type readout struct {
	Doc      string     `json:"doc"`
	ViewBox  [2]int     `json:"viewBox"`
	Box      [4]int     `json:"box"` // plot area in viewBox units: x0, y0, x1, y1
	XLim     [2]float64 `json:"xlim"`
	YLim     [2]float64 `json:"ylim"`
	T1       float64    `json:"t1"`
	N        int        `json:"n"`
	Prey     []float64  `json:"prey"`
	Pred     []float64  `json:"pred"`
	StepT    []float64  `json:"stepT"`
	StepPrey []float64  `json:"stepPrey"`
	StepPred []float64  `json:"stepPred"`
}

// clipRect is the plot area go-plot clips the curves to: the first
// clipPath of the SVG it draws.
var clipRect = regexp.MustCompile(`<clipPath id="c1"><rect x="(\d+)" y="(\d+)" width="(\d+)" height="(\d+)"/>`)

// writeReadout writes the readout when -readout names a file. It draws the
// figure a second time into memory to read the plot area, so the page maps
// the pointer with the geometry of the SVG it actually shows.
func writeReadout(fig *plot.Figure, ax *plot.Axes, sol *integrate.Solution) {
	flag.Parse()
	if *readoutPath == "" {
		return
	}
	if err := buildReadout(fig, ax, sol, *readoutPath); err != nil {
		fmt.Fprintln(os.Stderr, "readout:", err)
		os.Exit(1)
	}
}

func buildReadout(fig *plot.Figure, ax *plot.Axes, sol *integrate.Solution, path string) error {
	var svg bytes.Buffer
	if err := fig.SaveSVG(&svg); err != nil {
		return err
	}
	m := clipRect.FindSubmatch(svg.Bytes())
	if m == nil {
		return fmt.Errorf("no plot-area clipPath in the SVG go-plot drew")
	}
	var r [4]int
	for i := range r {
		r[i], _ = strconv.Atoi(string(m[i+1]))
	}
	vb := regexp.MustCompile(`viewBox="0 0 (\d+) (\d+)"`).FindSubmatch(svg.Bytes())
	if vb == nil {
		return fmt.Errorf("no viewBox in the SVG go-plot drew")
	}
	w, _ := strconv.Atoi(string(vb[1]))
	h, _ := strconv.Atoi(string(vb[2]))

	out := readout{
		Doc:     "Written by figure/main.go -readout; values of integrate.SolveIVP's DOP853 dense output (prey, pred) at t = t1*i/(n-1), and its accepted steps. Never edit by hand.",
		ViewBox: [2]int{w, h},
		Box:     [4]int{r[0], r[1], r[0] + r[2], r[1] + r[3]},
		T1:      15,
		N:       readoutSamples,
	}
	out.XLim[0], out.XLim[1] = ax.XLim()
	out.YLim[0], out.YLim[1] = ax.YLim()
	out.Prey = make([]float64, readoutSamples)
	out.Pred = make([]float64, readoutSamples)
	for i := range readoutSamples {
		y := sol.Sol.Eval(out.T1 * float64(i) / float64(readoutSamples-1))
		out.Prey[i], out.Pred[i] = y[0], y[1]
	}
	out.StepT = sol.T
	for _, y := range sol.Y {
		out.StepPrey = append(out.StepPrey, y[0])
		out.StepPred = append(out.StepPred, y[1])
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
