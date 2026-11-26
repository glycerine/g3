package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestDIMACSFrontendUNSATWithoutCore(t *testing.T) {
	var out bytes.Buffer
	if err := runArguments([]string{"-dimacs", "-in"}, strings.NewReader("p cnf 1 2\n1 0\n-1 0\n"), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "unsat\n" {
		t.Fatalf("source UNSAT output %q", out.String())
	}
}

func TestDIMACSFrontendClauseCapacity(t *testing.T) {
	var input strings.Builder
	input.WriteString("p cnf 12 33\n1 2 3 4 0\n5 6 7 8 0\n9 10 11 12 0\n")
	for _, group := range [2][6]int{{1, 2, 5, 6, 9, 10}, {3, 4, 7, 8, 11, 12}} {
		for i, v := range group {
			for _, u := range group[:i] {
				fmt.Fprintf(&input, "-%d -%d 0\n", u, v)
			}
		}
	}
	var out bytes.Buffer
	if err := runArguments([]string{"-st", "-dimacs", "-in"}, strings.NewReader(input.String()), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "unsat\n") || strings.Contains(out.String(), ":decisions ") != diagnostic.TraceCompiled() {
		t.Fatalf("capacity proof or trace-build fallback failed:\n%s", out.String())
	}
}
