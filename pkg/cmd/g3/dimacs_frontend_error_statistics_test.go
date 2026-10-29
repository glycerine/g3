package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/sat"
)

func TestDIMACSFrontendMalformedPrefixAndStatistics(t *testing.T) {
	var out bytes.Buffer
	err := runArguments([]string{"-dimacs", "-in"}, strings.NewReader("p cnf 2 2\n1 0\nx\n"), &out)
	if !errors.Is(err, sat.ErrDIMACSUnexpectedChar) || out.Len() != 0 {
		t.Fatalf("malformed source prefix err=%v out=%q", err, out.String())
	}
	out.Reset()
	if err = runArguments([]string{"-st", "-dimacs", "-in"}, strings.NewReader("p cnf 1 1\n1 0\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "sat\n1 \n(") || !strings.Contains(out.String(), ":mk-bool-var") || !strings.HasSuffix(out.String(), ")\n") {
		t.Fatalf("statistics output %q", out.String())
	}
}
