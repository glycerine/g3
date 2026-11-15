//go:build g3trace

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// read_datalog's IF_VERBOSE(1) lines and the WPA loader's IF_VERBOSE(10)
// lines (AUDIT.md cmd_context_02-F11) are g3trace-only, like the other
// ported progress lines. With G3_CPP_REFERENCE set the C++ lines must agree.
var datalogVerbosePrefixes = []string{"Z3 Datalog Engine", "parsing finished", "running saturation...",
	"Start parsing directory ", "Parsing map file ", "Parsing relation file ", "Done parsing directory "}

func datalogVerboseLines(log string) []string {
	var out []string
	for _, line := range strings.Split(log, "\n") {
		for _, prefix := range datalogVerbosePrefixes {
			if strings.HasPrefix(line, prefix) {
				out = append(out, line)
				break
			}
		}
	}
	return out
}

func TestDatalogFrontendVerboseProgress(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "p.datalog")
	if err = os.WriteFile(program, []byte("V 3\n\ne(x:V) printtuples\ne(\"a\").\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wpa := filepath.Join(wd, "test_vectors", "wpa", "basic")
	reference := os.Getenv("G3_CPP_REFERENCE")
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"file", []string{"-dl", "-v:1", program}, []string{"Z3 Datalog Engine", "parsing finished", "running saturation..."}},
		{"wpa", []string{"-dl", "-v:10", wpa}, []string{"Z3 Datalog Engine", "Start parsing directory " + wpa,
			"Parsing map file " + wpa + "/V.map", "Parsing map file " + wpa + "/nested/W.map",
			"Parsing relation file " + wpa + "/Edge.rel", "Parsing relation file " + wpa + "/nested/Tagged.rel",
			"Done parsing directory " + wpa, "parsing finished", "running saturation..."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := datalogVerboseLines(runVerboseProgressChild(t, tc.args))
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("Go lines\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
			if reference == "" {
				return
			}
			var native, cppLog bytes.Buffer
			cmd := exec.Command(reference, tc.args...)
			cmd.Stdout, cmd.Stderr = &native, &cppLog
			if err := cmd.Run(); err != nil {
				t.Fatal(err, native.String(), cppLog.String())
			}
			if cpp := datalogVerboseLines(cppLog.String()); strings.Join(cpp, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("C++ lines\n%s\nwant\n%s", strings.Join(cpp, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}
