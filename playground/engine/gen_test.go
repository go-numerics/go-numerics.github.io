// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

//go:build !js

package engine

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeneratedGoPrintsWhatThePageShows is the proof behind "the equivalent Go
// program": every case, every method, its generated program is gofmt-clean,
// compiles against the module versions this playground pins, and prints, row
// by row, exactly the text the page shows for the same parameters.
func TestGeneratedGoPrintsWhatThePageShows(t *testing.T) {
	if testing.Short() {
		t.Skip("builds about a hundred programs")
	}
	root := t.TempDir()
	mod, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile("../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	gomod := strings.Replace(string(mod), "module github.com/go-numerics/go-numerics.github.io/playground", "module gen", 1)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), sum, 0o644); err != nil {
		t.Fatal(err)
	}

	type prog struct {
		name string
		res  *Result
	}
	var progs []prog
	for fi, f := range Families {
		for ci, c := range f.Cases {
			for mi, m := range c.Methods {
				r, err := Execute(fi, ci, mi, DefaultArgs(&c, m))
				if err != nil {
					t.Fatal(err)
				}
				name := fmt.Sprintf("p%03d", len(progs))
				progs = append(progs, prog{name + " " + f.Name + "/" + c.Name + "/" + m, r})
				dir := filepath.Join(root, name)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				formatted, ferr := format.Source([]byte(r.Go))
				if ferr != nil {
					t.Errorf("%s: does not parse: %v\n%s", progs[len(progs)-1].name, ferr, r.Go)
				} else if !bytes.Equal(formatted, []byte(r.Go)) {
					t.Errorf("%s: not gofmt-clean:\n%s", progs[len(progs)-1].name, r.Go)
				}
				if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(r.Go), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	build := osexec.Command("go", "build", "-o", bin, "./...")
	build.Dir = root
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the generated programs do not build: %v\n%s", err, out)
	}
	for _, p := range progs {
		exe := filepath.Join(bin, strings.Fields(p.name)[0])
		out, err := osexec.Command(exe).CombinedOutput()
		if err != nil {
			t.Errorf("%s: %v\n%s", p.name, err, out)
			continue
		}
		// A row's name may itself hold " = " (f(x = 4)), so each line is
		// matched whole: "name = value" as the page shows it.
		lines := map[string]bool{}
		for _, line := range strings.Split(string(out), "\n") {
			lines[line] = true
		}
		for _, r := range p.res.Summary {
			if want := r.Name + " = " + r.Value; !lines[want] {
				t.Errorf("%s: the page shows %q, the program prints:\n%s", p.name, want, out)
			}
		}
	}
}
