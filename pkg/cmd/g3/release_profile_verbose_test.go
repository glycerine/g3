package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// AUDIT.md FUZZ-F7: g3 follows the _EXTERNAL_RELEASE build profile of the
// reference binary, so verbose lines guarded by #ifndef _EXTERNAL_RELEASE
// must not appear in the verbose log.
func TestReleaseProfileVerboseLinesAbsent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		level  uint32
		forbid string
	}{
		// sat_solver.cpp:1276 solver::mk_model.
		{"release_profile_sat_model.smt2", 10, "checking model"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", tc.name))
			if err != nil {
				t.Fatal(err)
			}
			level, err := diagnostic.ProcessVerbosityLevel()
			if err != nil {
				t.Fatal(err)
			}
			previous, err := diagnostic.BorrowProcessVerboseStream()
			if err != nil {
				t.Fatal(err)
			}
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				for _, err := range []error{diagnostic.SetProcessVerbosityLevel(level), diagnostic.SetProcessVerboseStream(previous), stream.Close()} {
					if err != nil {
						t.Error(err)
					}
				}
			}()
			if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
				t.Fatal(err)
			}
			if err = diagnostic.SetProcessVerbosityLevel(tc.level); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err = run(bytes.NewReader(script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if canonicalOutput(output.String()) != canonicalOutput(scriptResults[tc.name]) {
				t.Fatal("verbosity changed the result", output.String())
			}
			text, err := stream.String()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(text, tc.forbid) {
				t.Fatalf("Go verbose log has internal-profile line %q\n%s", tc.forbid, text)
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				var native, log bytes.Buffer
				cmd := exec.Command(reference, fmt.Sprintf("-v:%d", tc.level), "-in")
				cmd.Stdin = bytes.NewReader(script)
				cmd.Stdout = &native
				cmd.Stderr = &log
				if err := cmd.Run(); err != nil {
					t.Fatal(err, native.String(), log.String())
				}
				if strings.Contains(log.String(), tc.forbid) {
					t.Fatalf("C++ verbose log has %q; the reference is not a release build", tc.forbid)
				}
				if canonicalOutput(native.String()) != canonicalOutput(output.String()) {
					t.Fatal("source result mismatch", native.String(), output.String())
				}
			}
		})
	}
}
