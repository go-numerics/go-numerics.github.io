// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "c.out")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGate(t *testing.T) {
	cases := []struct {
		name, profile string
		want          int
		out           string
	}{
		{"all run", "mode: set\na.go:1.1,2.2 3 1\nb.go:1.1,2.2 2 5\n", 0, "every one of 5 statements"},
		{"one block not run", "mode: set\na.go:1.1,2.2 3 1\nb.go:7.1,8.2 2 0\n", 1, "b.go:7.1,8.2"},
		{"empty profile", "mode: set\n", 2, "no statement"},
		{"not a profile", "hello\n", 2, "not a coverage profile"},
		{"short line", "mode: set\na.go:1.1,2.2 3\n", 2, "malformed"},
		{"bad count", "mode: set\na.go:1.1,2.2 x 1\n", 2, "malformed"},
	}
	for _, c := range cases {
		var out bytes.Buffer
		if got := run([]string{write(t, c.profile)}, &out); got != c.want || !strings.Contains(out.String(), c.out) {
			t.Errorf("%s: exit %d (want %d), output %q", c.name, got, c.want, out.String())
		}
	}
	var out bytes.Buffer
	if run(nil, &out) != 2 || !strings.Contains(out.String(), "usage") {
		t.Error("no arguments")
	}
	if run([]string{filepath.Join(t.TempDir(), "missing")}, &out) != 2 {
		t.Error("a missing file")
	}
}

func TestMainExits(t *testing.T) {
	// main is os.Exit(run(...)): exercised by the gate in CI; here only that
	// run's contract holds for the real profile shape of go test.
	p := write(t, "mode: atomic\nx.go:3.2,4.5 1 2\n")
	var out bytes.Buffer
	if run([]string{p}, &out) != 0 {
		t.Error(out.String())
	}
}
