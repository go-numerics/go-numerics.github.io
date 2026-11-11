//go:build tools

// Package rubycheck pins the go-embedded-ruby the engine's tests run the
// generated Ruby programs on: go.mod and go.sum say which version, and
// Renovate moves it. Nothing here is built into the playground.
package rubycheck

import _ "github.com/go-embedded-ruby/ruby/cmd/rbgo"
