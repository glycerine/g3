package main

import (
	"bytes"
	"strings"
	"testing"
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
