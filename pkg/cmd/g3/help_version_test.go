package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpWithoutDebuggingSupport drops display_usage's _TRACE/Z3DEBUG lines,
// which the g3trace/g3debug builds print as the trace and debug z3 builds
// do; the checked-in text is the release build's.
func helpWithoutDebuggingSupport(text string) string {
	text = strings.Replace(text, "  -tr:tag     enable trace messages tagged with <tag>.\n", "", 1)
	text = strings.Replace(text, "  -dbg:tag    enable assertions tagged with <tag>.\n", "", 1)
	return strings.Replace(text, "Debugging support:\n\n", "", 1)
}

// AUDIT.md solver_misc_02-F1: -h, -? and -version print the source's
// display_usage and display_version text (64-bit build).
func TestHelpAndVersionMatchSource(t *testing.T) {
	var out bytes.Buffer
	if err := runArguments([]string{"-version"}, strings.NewReader(""), &out); err != nil || out.String() != "Z3 version 4.7.1 - 64 bit\n" {
		t.Fatalf("-version: %q %v", out.String(), err)
	}
	want, err := os.ReadFile(filepath.Join("test_vectors", "z3_help_output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-h", "-?"} {
		out.Reset()
		if err := runArguments([]string{flag}, strings.NewReader(""), &out); err != nil || helpWithoutDebuggingSupport(out.String()) != string(want) {
			t.Fatalf("%s: %v\n%s", flag, err, out.String())
		}
	}
}
