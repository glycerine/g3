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

// gaps_misc2: a resource limit inside qfnra-nlsat's polynomial code is a
// default_exception "canceled" (expr2polynomial's checkpoint, the
// multivariate polynomial manager's checkpoint); the factor and nlsat
// tactics turn it into a tactic_exception, so check-sat-using answers
// unknown with that reason, not Go sentinel text. The source debug build
// counts the resource limit differently, so the limits apply to the
// release build only.
func TestNLSatPolynomialCancellationReason(t *testing.T) {
	if ast.ASTSourceDebug {
		t.Skip("rlimit trip points are those of the release build")
	}
	path := filepath.Join("test_vectors", "nlsat_polynomial_cancel_reason.smt2")
	want := "unknown\n(:reason-unknown \"canceled\")\n"
	for _, rlimit := range []int{300, 320} {
		var out bytes.Buffer
		if err := runArguments([]string{"rlimit=" + strconv.Itoa(rlimit), path}, strings.NewReader(""), &out); err != nil {
			t.Fatal(rlimit, err, out.String())
		}
		if out.String() != want {
			t.Fatalf("rlimit=%d: %q, want %q", rlimit, out.String(), want)
		}
		if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
			native, err := exec.Command(reference, "rlimit="+strconv.Itoa(rlimit), path).Output()
			if err != nil || string(native) != want {
				t.Fatalf("C++ rlimit=%d: %q %v", rlimit, native, err)
			}
		}
	}
}
