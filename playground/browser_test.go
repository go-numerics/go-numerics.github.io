// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

//go:build !js

package playground

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBrowserProof builds the page exactly as the deploy does — the wasm from
// this module, wasm_exec.js from the same toolchain, then Hugo — serves it,
// and runs browsertest/driver.cjs against it in headless Chrome, light and
// dark with the scheme forced, at 1x and 2x (both schemes), and on a phone
// viewport (390x844 CSS px at 3x, mobile, touch). See the driver for what it
// checks. It skips when Chrome, node, puppeteer-core or Hugo is missing,
// unless PLAYGROUND_REQUIRE_BROWSER=1 (CI), where a skip would be a pass that
// proved nothing.
//
// Environment: CHROME (the browser), PLAYGROUND_NODE_PATH (a node_modules with
// puppeteer-core), HUGO (the hugo command, default "hugo"), PLAYGROUND_CAPTURE_DIR
// (where the screenshots go).
func TestBrowserProof(t *testing.T) {
	need := func(what string, ok bool) {
		if ok {
			return
		}
		if os.Getenv("PLAYGROUND_REQUIRE_BROWSER") == "1" {
			t.Fatalf("browser proof required but %s is missing", what)
		}
		t.Skipf("%s is missing; set PLAYGROUND_REQUIRE_BROWSER=1 to make this a failure", what)
	}
	chrome := os.Getenv("CHROME")
	if chrome == "" && runtime.GOOS == "darwin" {
		chrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}
	_, err := os.Stat(chrome)
	need("Chrome (CHROME)", chrome != "" && err == nil)
	node, err := exec.LookPath("node")
	need("node", err == nil)
	nodePath := os.Getenv("PLAYGROUND_NODE_PATH")
	_, err = os.Stat(filepath.Join(nodePath, "puppeteer-core"))
	need("puppeteer-core (PLAYGROUND_NODE_PATH)", nodePath != "" && err == nil)
	hugo := os.Getenv("HUGO")
	if hugo == "" {
		hugo = "hugo"
	}
	hugoArgv := strings.Fields(hugo)
	_, err = exec.LookPath(hugoArgv[0])
	need("Hugo (HUGO)", err == nil)

	site, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	static := filepath.Join(site, "static", "playground")
	if err := os.MkdirAll(static, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, env []string, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out.String())
		}
	}
	run(".", []string{"GOOS=js", "GOARCH=wasm"}, "go", "build", "-trimpath", "-ldflags=-s -w", "-o",
		filepath.Join(static, "playground.wasm"), "./cmd/playground-wasm")
	execJS, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(static, "wasm_exec.js"), execJS, 0o644); err != nil {
		t.Fatal(err)
	}
	public := t.TempDir()
	run(site, nil, hugoArgv[0], append(hugoArgv[1:], "--minify", "--destination", public)...)

	srv := httptest.NewServer(http.FileServer(http.Dir(public)))
	defer srv.Close()

	cmd := exec.Command(node, "browsertest/driver.cjs", srv.URL+"/playground/", chrome, captureDir(t))
	cmd.Env = append(os.Environ(), "NODE_PATH="+nodePath)
	out, err := cmd.CombinedOutput()
	var res struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Results []struct {
			Name, Detail string
			OK           bool
		} `json:"results"`
	}
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "RESULT "); ok {
			_ = json.Unmarshal([]byte(rest), &res)
		}
	}
	for _, r := range res.Results {
		t.Logf("%-5v %s  %s", r.OK, r.Name, r.Detail)
	}
	if err != nil || !res.OK {
		t.Fatalf("browser proof failed: %v %s\n%s", err, res.Error, out)
	}
}

// captureDir is where the proof writes screenshots for a person to look at:
// $PLAYGROUND_CAPTURE_DIR, else <user config dir>/go-numerics-playground/captures.
// It is durable (unlike t.TempDir) and it is refused when it lies inside a git
// work tree, so a capture can never be one `git add` away from publication.
func captureDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("PLAYGROUND_CAPTURE_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			t.Skipf("no user config dir: %v", err)
		}
		dir = filepath.Join(base, "go-numerics-playground", "captures")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if tree := gitTreeOf(abs); tree != "" {
		t.Fatalf("refusing to write captures inside the git work tree %s", tree)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatal(err)
	}
	return abs
}

// gitTreeOf returns the first ancestor of dir (or dir itself) holding a .git,
// or "" when there is none up to the filesystem root.
func gitTreeOf(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}
