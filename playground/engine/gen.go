// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// The generated programs. Each case writes the Go program that makes the same
// call and prints the same rows as the page ("name = value", the numbers with
// fmt's %v, as G does), and the SciPy call it ports. engine's tests build and
// run every Go program and compare what it prints with the page.

// goNum spells a float64 as Go source.
func goNum(v float64) string {
	switch {
	case math.IsNaN(v):
		return "math.NaN()"
	case math.IsInf(v, 1):
		return "math.Inf(1)"
	case math.IsInf(v, -1):
		return "math.Inf(-1)"
	}
	return G(v)
}

// goNums spells a []float64 literal.
func goNums(v []float64) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = goNum(x)
	}
	return "[]float64{" + strings.Join(parts, ", ") + "}"
}

// goSub spells a−b the way gofmt does: "a-b", or "a - -b" for a negative b.
func goSub(a, b float64) string {
	if s := goNum(b); strings.HasPrefix(s, "-") {
		return goNum(a) + " - " + s
	}
	return goNum(a) + "-" + goNum(b)
}

// pyNum spells a float64 as Python source.
func pyNum(v float64) string {
	switch {
	case math.IsNaN(v):
		return "np.nan"
	case math.IsInf(v, 1):
		return "np.inf"
	case math.IsInf(v, -1):
		return "-np.inf"
	}
	return G(v)
}

// pyNums spells a list literal.
func pyNums(v []float64) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = pyNum(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// goFile assembles a main package: the imports (standard library first), and
// the body of main.
func goFile(imports []string, body string) string {
	var std, other []string
	seen := map[string]bool{}
	for _, im := range imports {
		// Only what the program uses: an unused import does not compile.
		if seen[im] || !strings.Contains(body, pkgName(im)+".") {
			continue
		}
		seen[im] = true
		if strings.Contains(strings.SplitN(im, "/", 2)[0], ".") {
			other = append(other, im)
		} else {
			std = append(std, im)
		}
	}
	sort.Strings(std)
	sort.Strings(other)
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n")
	for _, im := range std {
		fmt.Fprintf(&b, "\t%q\n", im)
	}
	if len(std) > 0 && len(other) > 0 {
		b.WriteString("\n")
	}
	for _, im := range other {
		fmt.Fprintf(&b, "\t%q\n", im)
	}
	b.WriteString(")\n\n")
	b.WriteString("func main() {\n")
	b.WriteString(body)
	b.WriteString("}\n")
	return b.String()
}

// pkgName is the name a package is referred to by: the last element of its
// path, or the one before it for a version suffix (math/rand/v2 is rand).
func pkgName(path string) string {
	parts := strings.Split(path, "/")
	last := parts[len(parts)-1]
	if len(parts) > 1 && len(last) > 1 && last[0] == 'v' && strings.Trim(last[1:], "0123456789") == "" {
		return parts[len(parts)-2]
	}
	return last
}

// indent prefixes every non-empty line of s with a tab.
func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = "\t" + l
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// printRow is the Go statement printing one row as the page shows it.
func printRow(name, expr string) string {
	return fmt.Sprintf("fmt.Println(%q, \"=\", %s)\n", name, expr)
}
