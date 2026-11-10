// Copyright (c) the go-numerics authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Args is a parsed arguments line: every key of the case's parameter list, at
// the visitor's value or at its default.
type Args struct {
	text map[string]string
}

// ParseArgs reads "key=value, key=value" against the parameters a case takes.
// An unknown key, a repeated key or a missing "=" is an error that names what
// is accepted. Values are numbers ("1e-6", "inf", "pi", "pi/2", "2pi") or, for
// vectors, numbers separated by ";".
func ParseArgs(s string, params []Param) (*Args, error) {
	a := &Args{text: map[string]string{}}
	known := make([]string, len(params))
	for i, p := range params {
		a.text[p.Key] = p.Default
		known[i] = p.Key
	}
	seen := map[string]bool{}
	for _, field := range strings.Split(s, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		k, v, ok := strings.Cut(field, "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || k == "" {
			return nil, fmt.Errorf("%q is not key=value (parameters: %s)", field, strings.Join(known, ", "))
		}
		if _, ok := a.text[k]; !ok {
			return nil, fmt.Errorf("unknown parameter %q (parameters: %s)", k, strings.Join(known, ", "))
		}
		if seen[k] {
			return nil, fmt.Errorf("parameter %q given twice", k)
		}
		seen[k] = true
		a.text[k] = v
	}
	return a, nil
}

// Float reads a number parameter.
func (a *Args) Float(key string) (float64, error) {
	v, err := parseNumber(a.text[key])
	if err != nil {
		return 0, fmt.Errorf("parameter %s: %w", key, err)
	}
	return v, nil
}

// Int reads a whole-number parameter.
func (a *Args) Int(key string) (int, error) {
	v, err := a.Float(key)
	if err != nil {
		return 0, err
	}
	if v != math.Trunc(v) || math.Abs(v) > 1<<31 {
		return 0, fmt.Errorf("parameter %s: %s is not a whole number", key, a.text[key])
	}
	return int(v), nil
}

// Floats reads a vector parameter, numbers separated by ";".
func (a *Args) Floats(key string) ([]float64, error) {
	var out []float64
	for _, f := range strings.Split(a.text[key], ";") {
		v, err := parseNumber(f)
		if err != nil {
			return nil, fmt.Errorf("parameter %s: %w", key, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// Raw is the text of a parameter, for the words ("analytic", "fd") some take.
func (a *Args) Raw(key string) string { return a.text[key] }

// parseNumber reads a float64, or [k]pi[/d] for the multiples of π a range of
// integration or a plot wants.
func parseNumber(s string) (float64, error) {
	s = strings.TrimSpace(s)
	num, den, hasDen := strings.Cut(s, "/")
	d := 1.0
	if hasDen {
		v, err := strconv.ParseFloat(strings.TrimSpace(den), 64)
		if err != nil || v == 0 {
			return 0, fmt.Errorf("%q is not a number", s)
		}
		d = v
	}
	num = strings.TrimSpace(num)
	if k, ok := strings.CutSuffix(num, "pi"); ok {
		k = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(k), "*"))
		c := 1.0
		switch k {
		case "":
		case "-":
			c = -1
		default:
			v, err := strconv.ParseFloat(k, 64)
			if err != nil {
				return 0, fmt.Errorf("%q is not a number", s)
			}
			c = v
		}
		return c * math.Pi / d, nil
	}
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", s)
	}
	return v / d, nil
}
