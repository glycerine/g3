//go:build xtrace_off

// When the xtrace_off build tag IS set, all trace calls are no-ops.
// The compiler inlines empty functions, so there is zero runtime cost.
package xtrace

var Suppressed bool

// Enabled is false when xtracer is disabled via build tag.
const Enabled = false

// HashVerbose is unused when xtracer is disabled.
const HashVerbose = false

// Trace is a no-op when xtracer is disabled.
func Trace(format string, args ...interface{}) {}
