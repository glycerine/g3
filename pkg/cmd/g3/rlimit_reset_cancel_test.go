package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// leak_factor round 2: at a small rlimit these scripts stop inside nlqsat or
// the qfnra portfolio and then (reset). g3 left the interrupted nlsat search
// owning its literal vectors, so the nlsat solver and its algebraic numbers
// stayed live, (reset) failed with "mpnum: live caller algebraic owners" and
// the next error report panicked on the closed resource limit. nlqsat and
// nla2bv also turned exceptions into tactic_exceptions where the C++ lets
// them escape, so check-sat-using printed unknown where z3 prints
// (error "tactic failed: ..."). Each case must print exactly what z3
// prints. The source debug build counts the resource limit differently, so
// the limits apply to the release build only.
func TestRlimitCancelAcrossReset(t *testing.T) {
	if ast.ASTSourceDebug {
		t.Skip("rlimit trip points are those of the release build")
	}
	cases := []struct {
		vector string
		rlimit int
		exit   int
		want   string
	}{
		{"nra_portfolio.smt2", 50, 0, "(error \"tactic failed: max. resource limit exceeded\")\n(error \"tactic failed: canceled\")\n(error \"tactic failed: canceled\")\nunknown\n"},
		{"nra_portfolio.smt2", 100, 0, "(error \"tactic failed: canceled\")\nsat\nunsat\nunknown\n"},
		{"nra_portfolio.smt2", 150, 0, "(error \"tactic failed: canceled\")\nsat\nunsat\nsat\n"},
		{"nra_solver_quantifiers.smt2", 50, 0, "unknown\nunknown\nunknown\nunknown\n"},
		{"nra_solver_quantifiers.smt2", 100, 0, "unknown\nsat\nunsat\nunknown\n"},
		{"nra_solver_quantifiers.smt2", 150, 0, "unknown\nsat\nunsat\nsat\n"},
		{"nra_default_portfolio.smt2", 100, 0, "unknown\nsat\nunknown\n"},
		{"nra_default_portfolio.smt2", 150, 0, "unknown\nsat\nunknown\n"},
		{"nlqsat_nonlinear_quantifiers.smt2", 50, 0, "unknown\nunknown\n(error \"tactic failed: canceled\")\nunsat\nunknown\nunknown\n"},
		{"nlqsat_models.smt2", 50, 1, "(error \"tactic failed: canceled\")\n(error \"line 7 column 14: model is not available\")\nunknown\n(error \"line 15 column 14: model is not available\")\n"},
		{"nlqsat_models.smt2", 100, 1, "(error \"tactic failed: canceled\")\n(error \"line 7 column 14: model is not available\")\nunknown\n(error \"line 15 column 14: model is not available\")\n"},
		{"z3_bug_nlsat_signed_infinity_sign.smt2", 100, 0, "(error \"tactic failed: canceled\")\n"},
		{"z3_bug_nlsat_signed_infinity_sign.smt2", 300, 0, "(error \"tactic failed: max. resource limit exceeded\")\n"},
		{"qfnra_nlsat.smt2", 50, 0, "unknown\n(error \"tactic failed: max. resource limit exceeded\")\n(error \"tactic failed: max. resource limit exceeded\")\n(error \"tactic failed: max. resource limit exceeded\")\n(error \"tactic failed: max. resource limit exceeded\")\nsat\nunsat\nsat\n"},
		{"qfnra_nlsat.smt2", 100, 0, "unsat\n(error \"tactic failed: max. resource limit exceeded\")\n(error \"tactic failed: max. resource limit exceeded\")\n(error \"tactic failed: max. resource limit exceeded\")\n(error \"tactic failed: max. resource limit exceeded\")\nsat\nunsat\nsat\n"},
	}
	for _, c := range cases {
		path := filepath.Join("test_vectors", c.vector)
		var out bytes.Buffer
		err := runArguments([]string{"rlimit=" + strconv.Itoa(c.rlimit), path}, strings.NewReader(""), &out)
		if (err != nil) != (c.exit != 0) {
			t.Fatal(c.vector, c.rlimit, err, out.String())
		}
		if out.String() != c.want {
			t.Fatalf("%s rlimit=%d: %q, want %q", c.vector, c.rlimit, out.String(), c.want)
		}
		if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
			native, err := exec.Command(reference, "rlimit="+strconv.Itoa(c.rlimit), path).Output()
			code := 0
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(c.vector, c.rlimit, err)
			}
			if code != c.exit || string(native) != c.want {
				t.Fatalf("C++ %s rlimit=%d: exit %d %q", c.vector, c.rlimit, code, native)
			}
		}
	}
}
