package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDIMACSFrontendSATModelSourceNumbering(t *testing.T) {
	var out bytes.Buffer
	input := "c model\np cnf 3 3\n1 0\n-2 3 0\n2 3 0\n"
	if err := runArguments([]string{"-dimacs", "-in"}, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "sat\n") || !strings.Contains(out.String(), "1 ") || !strings.Contains(out.String(), "3 ") {
		t.Fatalf("source SAT model output %q", out.String())
	}
}
