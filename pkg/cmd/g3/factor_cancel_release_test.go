package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// leak_factor (gaps_misc2 finding): a resource limit that trips inside the
// factor tactic's m_pm.factor call unwinds factor's scoped locals in C++.
// g3 left the factor frame active, the per-call polynomial manager refused
// to close, and the run ended with "ERROR: g3: live raw memory blocks" after
// the correct answer. Each case must print exactly what z3 prints and end
// without an error. The source debug build counts the resource limit
// differently, so the limits apply to the release build only.
func TestFactorCancellationReleasesPolynomialMemory(t *testing.T) {
	if ast.ASTSourceDebug {
		t.Skip("rlimit trip points are those of the release build")
	}
	cases := []struct {
		vector string
		rlimit int
		want   string
	}{
		{"nra_root_sort_order_quartic.smt2", 150, "unknown\n"},
		{"nra_root_sort_order_quartic.smt2", 200, "unknown\n"},
		{"factor_eq_var_order.smt2", 50, "(error \"tactic failed: canceled\")\n"},
		{"nlsat_polynomial_cancel_reason.smt2", 150, "unknown\n(:reason-unknown \"canceled\")\n"},
		{"nlsat_polynomial_cancel_reason.smt2", 200, "unknown\n(:reason-unknown \"canceled\")\n"},
		{"nonlinear_horner_bound.smt2", 150, "unknown\n"},
		{"nra_translate_q_trace.smt2", 300, "unknown\n"},
		{"qfnra_nlsat_named.smt2", 50, "unknown\nunknown\nunknown\nsat\nunsat\n"},
		{"qfnra_nlsat_named.smt2", 100, "unknown\nunknown\nunknown\nsat\nunsat\n"},
		{"release_profile_factor_verbose.smt2", 150, "unknown\n"},
		{"release_profile_factor_verbose.smt2", 200, "unknown\n"},
	}
	for _, c := range cases {
		path := filepath.Join("test_vectors", c.vector)
		var out bytes.Buffer
		if err := runArguments([]string{"rlimit=" + strconv.Itoa(c.rlimit), path}, strings.NewReader(""), &out); err != nil {
			t.Fatal(c.vector, c.rlimit, err, out.String())
		}
		if out.String() != c.want {
			t.Fatalf("%s rlimit=%d: %q, want %q", c.vector, c.rlimit, out.String(), c.want)
		}
		if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
			native, err := exec.Command(reference, "rlimit="+strconv.Itoa(c.rlimit), path).Output()
			if err != nil || string(native) != c.want {
				t.Fatalf("C++ %s rlimit=%d: %q %v", c.vector, c.rlimit, native, err)
			}
		}
	}
}
