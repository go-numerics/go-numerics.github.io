// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

// Command covergate reads a Go coverage profile and exits 1 unless every
// statement in it was run: the playground's 100% gate, as a program with a
// test rather than shell arithmetic.
//
//	go test -coverprofile=coverage.out . ./engine ./vm && go run ./internal/covergate coverage.out
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// uncovered returns the profile's blocks that no test ran and how many
// statements the profile holds. A profile with no statement at all is an
// error: a gate over nothing passes for the wrong reason.
func uncovered(r io.Reader) (blocks []string, statements int, err error) {
	sc := bufio.NewScanner(r)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			if !strings.HasPrefix(line, "mode:") {
				return nil, 0, fmt.Errorf("not a coverage profile: %q", line)
			}
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 {
			return nil, 0, fmt.Errorf("malformed line %q", line)
		}
		n, err1 := strconv.Atoi(f[1])
		c, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil {
			return nil, 0, fmt.Errorf("malformed line %q", line)
		}
		statements += n
		if c == 0 {
			blocks = append(blocks, f[0])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, 0, err
	}
	if statements == 0 {
		return nil, 0, fmt.Errorf("the profile holds no statement")
	}
	return blocks, statements, nil
}

func run(args []string, out io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(out, "usage: covergate coverage.out")
		return 2
	}
	f, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	defer f.Close()
	blocks, n, err := uncovered(f)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	if len(blocks) > 0 {
		fmt.Fprintf(out, "not run (%d of %d statements are in blocks no test ran):\n", len(blocks), n)
		for _, b := range blocks {
			fmt.Fprintln(out, "  "+b)
		}
		return 1
	}
	fmt.Fprintf(out, "every one of %d statements ran\n", n)
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }
