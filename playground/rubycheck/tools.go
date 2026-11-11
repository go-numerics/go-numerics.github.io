//go:build tools

// Package rubycheck pins the go-embedded-ruby the engine's tests run the
// generated Ruby programs on: go.mod and go.sum say which version, and
// Renovate moves it. Nothing here is built into the playground.
//
// go.mod also requires go-numerics' optimize, integrate and special at the
// versions playground/go.mod pins: go-embedded-ruby v0.8.5 pins older ones
// (optimize v0.3.1, whose BFGS update is the dense one), and with those a Ruby
// BFGS takes another path than the page. The two must move together.
package rubycheck

import _ "github.com/go-embedded-ruby/ruby/cmd/rbgo"
