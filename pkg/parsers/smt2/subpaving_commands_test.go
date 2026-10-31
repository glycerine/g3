package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2ToSubpavingCommand(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(declare-const x Real)
(declare-const y Real)
(to-subpaving (+ (* x x) y))
(to-subpaving (* 2 x))
(to-subpaving (/ 3.0 2.0))`
	want := "(+ (^ x 2.0) y)\n=======>\nx0 := x\nx2 := y\nx1 = x0^2\nx3 = x1 + x2\n1/1 x3\n(* 2.0 x)\n=======>\nx0 := x\n2/1 x0\n(/ 3.0 2.0)\n=======>\n3/2 x4294967295\n"
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != want {
		t.Fatalf("subpaving output = %q, want %q", got, want)
	}
}
