package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// Exercise weighted pseudo-Boolean assertions through parsing, scoped solver
// state and the command frontend, with an independent assignment oracle.
func TestWeightedPseudoBooleanAssignmentsThroughSMTLib(t *testing.T) {
	var script, want strings.Builder
	script.WriteString("(set-logic QF_FD)\n(declare-const x Bool)\n(declare-const y Bool)\n(declare-const z Bool)\n")
	script.WriteString("(assert ((_ pbge 4 2 3 2) x (not y) z))\n")
	for assignment := 0; assignment < 8; assignment++ {
		script.WriteString("(push 1)\n")
		total := 0
		for i, name := range []string{"x", "y", "z"} {
			value := assignment&(1<<i) != 0
			if value {
				fmt.Fprintf(&script, "(assert %s)\n", name)
			} else {
				fmt.Fprintf(&script, "(assert (not %s))\n", name)
			}
			if i == 1 {
				value = !value
			}
			if value {
				total += []int{2, 3, 2}[i]
			}
		}
		script.WriteString("(check-sat)\n(pop 1)\n")
		if total >= 4 {
			want.WriteString("sat\n")
		} else {
			want.WriteString("unsat\n")
		}
	}
	script.WriteString("(check-sat)\n")
	want.WriteString("sat\n")
	var output bytes.Buffer
	if err := run(strings.NewReader(script.String()), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if output.String() != want.String() {
		t.Fatalf("SMT-LIB results:\n%s\nwant:\n%s", output.String(), want.String())
	}
}
