//go:build !xtrace_off

// Package xtracer provides execution tracing for parallel conformance
// testing between Go goivy and Python ivy. Trace() prints timestamped
// messages to stdout by default, matching Python's `if __debug__:` guards.
//
// To disable at build time: go build -tags xtrace_off ./cmd/goivy_check/
// To disable at runtime:    XTRACE_OFF=1 go test ./...
//
// Both Go and Python emit the same format:
//
//	XTRACE: <category>.<function> <ENTER|EXIT> [<detail>]
//
// so that `diff` on the two outputs reveals the first divergence.
package xtrace

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
)

// Enabled is true when xtracer is active (the default).
// Set XTRACE_OFF=1 to suppress output at runtime.
const Enabled = true

// HashVerbose causes HASH trace lines to include
// the full canonical string.
const HashVerbose bool = true

// Suppressed is set by XTRACE_OFF=1 to silence output without
// changing Enabled (so `if xtracer.Enabled` guards still compile away).
var Suppressed bool

func init() {
	if os.Getenv("XTRACE_OFF") == "1" {
		Suppressed = true
	}
}

func Trace1(format string, args ...interface{}) {
	trace(format, args...)
}

// Trace prints an execution trace line to stdout.
// Format: "XTRACE: " + fmt.Sprintf(format, args...) + "\n"
func Trace(format string, args ...interface{}) {
	trace(format, args...)
}

var globalXtraceCounter int64
var mems = &runtime.MemStats{}

// Trace prints an execution trace line to stdout.
// Format: "XTRACE: " + fmt.Sprintf(format, args...) + "\n"
func trace(format string, args ...interface{}) {
	if Suppressed {
		return
	}

	if false {
		if globalXtraceCounter%50 == 0 {
			runtime.ReadMemStats(mems)
			fmt.Printf("[at trace %v] mems.HeapAlloc = %0.3f MB; HeapInuse = %0.3f MB\n", globalXtraceCounter, float64(mems.HeapAlloc)/(1<<20), float64(mems.HeapInuse)/(1<<20))
			//maybePaceGC(globalXtraceCounter, mems)
		}
		//maybeWriteHeapProfile(globalXtraceCounter)
		globalXtraceCounter++
	}

	// replace true/false with True/False to match python
	// and avoid spurious diffs.
	for i, a := range args {
		switch b := a.(type) {
		case bool:
			if b {
				args[i] = "True"
			} else {
				args[i] = "False"
			}
		case string:
			args[i] = b // NormalizeLine(b)
		}
	}

	if strings.Contains(format, "\n") {
		splt := strings.SplitN(format, "\n", 2)
		if len(splt) == 2 {
			format = splt[0] + "\n" + fileLine(3) + ":" + splt[1]
		}
	}
	fmt.Printf("XTRACE: "+format+"\n", args...)
}

func fileLine(depth int) string {
	_, fileName, fileLine, ok := runtime.Caller(depth)
	var s string
	if ok {
		s = fmt.Sprintf("%s:%d", path.Base(fileName), fileLine)
	} else {
		s = ""
	}
	return s
}
