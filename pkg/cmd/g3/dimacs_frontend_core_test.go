package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDIMACSFrontendTrackedClauseCore(t *testing.T) {
	var out bytes.Buffer
	input := "p cnf 2 4\n1 2 0\n-1 2 0\n1 -2 0\n-1 -2 0\n"
	if err := runArguments([]string{"sat.dimacs.core=true", "-dimacs", "-in"}, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "unsat\ncore\n") || !strings.Contains(out.String(), "1 ") || !strings.Contains(out.String(), "-2 ") {
		t.Fatalf("tracked source core %q", out.String())
	}
}
