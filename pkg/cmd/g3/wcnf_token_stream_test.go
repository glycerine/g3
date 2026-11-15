package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/opt"
)

// AUDIT.md opt_02-F5: wcnf::parse reads a token stream (outputs checked
// against build/z3).
func TestWCNFTokenStreamFrontend(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"multiline-clause", "p wcnf 2 3 10\n1 1\n 0\n1 2 0\n10 -1 -2 0\n", "sat\n   1\n"},
		{"undeclared-variable", "p wcnf 2 2 10\n3 1 0\n10 -1 -3 0\n", "sat\n   0\n"},
		{"missing-top", "p wcnf 2 3\n1 1 0\n1 2 0\n-1 -2 0\n", "sat\n   0\n"},
		{"no-header-all-hard", "c no header\n1 1 0\n2 -1 0\n", "unsat\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := runArguments([]string{"-wcnf", "-in"}, strings.NewReader(tc.input), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if output.String() != tc.want {
				t.Fatalf("wcnf output: %q", output.String())
			}
		})
	}
	var output bytes.Buffer
	err := runArguments([]string{"-wcnf", "-in"}, strings.NewReader("p wcnf 2 2 10\n3 x 0\n"), &output)
	var exit *opt.InputParseExit
	if !errors.As(err, &exit) || exit.Status != 3 || exit.Message != `(error line 1 "unexpected char: x" )` || output.Len() != 0 {
		t.Fatalf("wcnf parse error: %v %q", err, output.String())
	}
}
