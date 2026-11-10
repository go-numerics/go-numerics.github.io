package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// TestBrowserReadout drives the built landing page in headless Chrome: it
// points at the figure and requires the readout to print exactly the
// float64 values figure.json holds — a dense-output sample under the
// pointer, an accepted step on its dot, and the arrow keys.
//
//	hugo --minify -d <dir> && SITE_DIR=<dir> go test -run TestBrowserReadout .
//
// CHROME names the browser when chromedp cannot find one. Without SITE_DIR
// the test is skipped: the CI sets it after building the site.
func TestBrowserReadout(t *testing.T) {
	site := os.Getenv("SITE_DIR")
	if site == "" {
		t.Skip("SITE_DIR not set: build the site with hugo first")
	}
	raw, err := os.ReadFile(filepath.Join(site, "img", "figure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var d readout
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(site)))
	defer srv.Close()

	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.WindowSize(1280, 1000))
	// GitHub's Ubuntu 24.04 runners forbid the unprivileged user namespaces
	// Chrome's sandbox needs (AppArmor), and Chrome then never opens its
	// DevTools socket ("websocket url timeout reached"). The page served is
	// this repository's own, from a local server: only there is the sandbox
	// dropped.
	if os.Getenv("CI") != "" {
		opts = append(opts, chromedp.NoSandbox)
	}
	if c := os.Getenv("CHROME"); c != "" {
		opts = append(opts, chromedp.ExecPath(c))
	}
	actx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(actx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	const figRect = `(function(){var r=document.querySelector("#fig-live img").getBoundingClientRect();return {Left:r.left,Top:r.top,Width:r.width}})()`
	var rect struct{ Left, Top, Width float64 }
	if err := chromedp.Run(ctx,
		chromedp.Navigate(srv.URL+"/"),
		chromedp.ScrollIntoView("#fig-live", chromedp.ByQuery),
		// The listeners are attached once figure.json has loaded.
		chromedp.Poll(`document.getElementById("fig-vl").getAttribute("y1") === "`+strconv.Itoa(d.Box[1])+`"`, nil),
		chromedp.Evaluate(figRect, &rect),
	); err != nil {
		t.Fatal(err)
	}
	s := rect.Width / float64(d.ViewBox[0])
	if s <= 0 {
		t.Fatalf("figure width %v", rect.Width)
	}
	px := func(x float64) float64 {
		return float64(d.Box[0]) + (x-d.XLim[0])/(d.XLim[1]-d.XLim[0])*float64(d.Box[2]-d.Box[0])
	}
	py := func(y float64) float64 {
		return float64(d.Box[3]) - (y-d.YLim[0])/(d.YLim[1]-d.YLim[0])*float64(d.Box[3]-d.Box[1])
	}
	// readAt moves the pointer and waits until the readout satisfies ready, a
	// JS expression over its text `s`. Chrome delivers pointermove aligned
	// to animation frames, after the input command has returned, so the
	// text is read only once it answers for this pointer position; on a
	// timeout the text it shows is reported.
	readAt := func(vx, vy float64, ready string) string {
		t.Helper()
		// The figure's place is measured again before each move: the page
		// may still be scrolling from ScrollIntoView, and a stale rectangle
		// puts the pointer outside the plot area.
		var r struct{ Left, Top, Width float64 }
		if err := chromedp.Run(ctx, chromedp.Evaluate(figRect, &r)); err != nil {
			t.Fatal(err)
		}
		k := r.Width / float64(d.ViewBox[0])
		var text string
		err := chromedp.Run(ctx,
			chromedp.MouseEvent(input.MouseMoved, r.Left+vx*k, r.Top+vy*k),
			chromedp.Poll(`(function(s){return `+ready+`})(document.getElementById("fig-read").textContent)`, nil, chromedp.WithPollingTimeout(5*time.Second)),
		)
		_ = chromedp.Run(context.WithoutCancel(ctx), chromedp.Text("#fig-read", &text, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("pointer at (%.2f, %.2f): readout %q never satisfied %s: %v", vx, vy, text, ready, err)
		}
		return text
	}
	// JS String(x) and Go's 'f', -1 agree on these magnitudes (1e-7 to 1e21):
	// both print the shortest decimal that reads back to the same float64.
	f := func(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }
	sample := regexp.MustCompile(`^dense output, sample (\d+) of (\d+): `)
	tOf := func(i int) float64 { return d.T1 * float64(i) / float64(d.N-1) }

	// Points between the dots, along t: the sample under the pointer, its
	// exact values, and an index within a screen pixel of the pointer.
	perPixel := float64(d.N-1) / ((d.T1 / (d.XLim[1] - d.XLim[0])) * float64(d.Box[2]-d.Box[0]) * s)
	for _, target := range []int{37, 1000, 1777, 2500, 2950} {
		vx := px(tOf(target))
		text := readAt(vx, float64(d.Box[3])-2, fmt.Sprintf(`(function(m){return !!m && Math.abs(+m[1]-%d) <= %d})(/^dense output, sample (\d+) of/.exec(s))`, target, int(perPixel)+1))
		m := sample.FindStringSubmatch(text)
		if m == nil {
			t.Fatalf("pointer at sample %d: readout %q is not a dense-output sample", target, text)
		}
		i, _ := strconv.Atoi(m[1])
		if diff := float64(i - target); diff > perPixel+1 || -diff > perPixel+1 {
			t.Errorf("pointer at sample %d (x %.2f) read sample %d (%.1f samples per pixel): %q", target, vx, i, perPixel, text)
		}
		want := fmt.Sprintf("dense output, sample %d of %d: t = %s, prey = %s, predators = %s", i, d.N-1, f(tOf(i)), f(d.Prey[i]), f(d.Pred[i]))
		if text != want {
			t.Errorf("readout\n got %q\nwant %q", text, want)
		}
	}

	// On an accepted step's dot: the step the solver returned.
	for _, k := range []int{3, len(d.StepT) / 2, len(d.StepT) - 2} {
		text := readAt(px(d.StepT[k]), py(d.StepPrey[k]), fmt.Sprintf(`s.indexOf("accepted step %d of ") === 0`, k))
		want := fmt.Sprintf("accepted step %d of %d: t = %s, prey = %s, predators = %s", k, len(d.StepT)-1, f(d.StepT[k]), f(d.StepPrey[k]), f(d.StepPred[k]))
		if text != want {
			t.Errorf("on step %d\n got %q\nwant %q", k, text, want)
		}
	}

	// Keyboard: Home, then two right arrows, is sample 2.
	var text string
	if err := chromedp.Run(ctx,
		chromedp.Focus("#fig-live", chromedp.ByQuery),
		chromedp.KeyEvent(kb.Home),
		chromedp.KeyEvent(kb.ArrowRight), chromedp.KeyEvent(kb.ArrowRight),
		chromedp.Text("#fig-read", &text, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("dense output, sample 2 of %d: t = %s, prey = %s, predators = %s", d.N-1, f(tOf(2)), f(d.Prey[2]), f(d.Pred[2]))
	if text != want {
		t.Errorf("keyboard\n got %q\nwant %q", text, want)
	}
}
