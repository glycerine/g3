package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/opt"
)

// AUDIT.md opt_02-F4: the OPB reader is opt_parse.cpp's opb over
// opt_stream_buffer. A term without a coefficient reads as 0 times its
// literals, and a malformed input is the reader's own message on stderr
// with exit(3).
func TestOPBFrontendSourceReader(t *testing.T) {
	for _, tc := range []struct{ name, want, stderr string }{
		{"opb_coefficientless.opb", "sat\n   3\n", ""},
		{"opb_interleaved_objectives.opb", "sat\n   0\n   3\n", ""},
		{"opb_unexpected_char.opb", "", `(error line 1 "unexpected char: y" expected "x")`},
		// z3 4.7.1 crashes here; FORK FIX #128 reports the truncation.
		{"opb_constraint_without_relation.opb", "", `(error line 2 "unexpected end of input" expected ">=", "=" or "<=")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("test_vectors", tc.name)
			var out bytes.Buffer
			err := runArguments([]string{path}, strings.NewReader(""), &out)
			var exit *opt.InputParseExit
			if tc.stderr == "" {
				if err != nil || out.String() != tc.want {
					t.Fatalf("output %q, error %v; want %q", out.String(), err, tc.want)
				}
			} else if !errors.As(err, &exit) || exit.Status != 3 || exit.Message != tc.stderr || out.Len() != 0 {
				t.Fatalf("output %q, error %v; want exit(3) %q", out.String(), err, tc.stderr)
			}
			reference := os.Getenv("G3_CPP_REFERENCE")
			if reference == "" {
				return
			}
			var stdout, stderr bytes.Buffer
			command := exec.Command(reference, path)
			command.Stdout, command.Stderr = &stdout, &stderr
			runErr := command.Run()
			if tc.stderr == "" {
				if runErr != nil || stdout.String() != tc.want {
					t.Fatalf("C++ output %q, error %v", stdout.String(), runErr)
				}
				return
			}
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 3 || stderr.String() != tc.stderr+"\n" {
				t.Fatalf("C++ stderr %q, error %v", stderr.String(), runErr)
			}
		})
	}
}
