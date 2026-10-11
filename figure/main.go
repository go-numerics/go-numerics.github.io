// Command figure draws the landing page's figure, static/img/figure.svg:
//
//	cd figure && go run . > ../static/img/figure.svg
//
// The page's terminal shows this file from func main on, read at build
// time, and the CI draws the figure again and fails if a byte differs.
package main

import (
	"fmt"
	"os"

	"github.com/go-numerics/integrate"
	"github.com/go-plot/plot"
)

func main() {
	// Lotka–Volterra: prey y0 and predators y1.
	lotka := func(t float64, y, dydt []float64) {
		dydt[0] = 1.5*y[0] - y[0]*y[1]
		dydt[1] = -3*y[1] + y[0]*y[1]
	}
	sol, err := integrate.SolveIVP(lotka, [2]float64{0, 15}, []float64{10, 5},
		&integrate.IVPOptions{Method: integrate.DOP853, RTol: 1e-10, DenseOutput: true})
	if err != nil {
		panic(err)
	}

	// The dense output between the steps, and the steps themselves.
	t := make([]float64, 600)
	prey, pred := make([]float64, len(t)), make([]float64, len(t))
	for i := range t {
		t[i] = 15 * float64(i) / float64(len(t)-1)
		y := sol.Sol.Eval(t[i])
		prey[i], pred[i] = y[0], y[1]
	}
	stepPrey := make([]float64, len(sol.T))
	for k, y := range sol.Y {
		stepPrey[k] = y[0]
	}

	fig, axs := plot.Subplots(1, 1, plot.FigSize(6.4, 4))
	ax := axs[0][0]
	ax.Plot(t, prey, plot.Label("prey"))
	ax.Plot(t, pred, plot.Label("predators"), plot.LineStyle("--"))
	ax.Plot(sol.T, stepPrey, plot.Marker("o"), plot.LineStyle("None"),
		plot.MarkerSize(3), plot.Color("C0"), plot.Label("accepted steps"))
	ax.SetTitle(fmt.Sprintf("DOP853, rtol 1e-10: %d steps, %d evaluations", sol.NSteps, sol.NFev))
	ax.SetYLim(-0.5, 19)
	ax.SetXLabel(`time $t$`)
	ax.Legend(plot.Loc("upper left"))
	if err := fig.SaveSVG(os.Stdout); err != nil {
		panic(err)
	}
}
