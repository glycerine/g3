package shell_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/shell"
)

func mpsPair(column, row string, value float64) string {
	return fmt.Sprintf("    %-8s  %-8s  %.12g\n", column, row, value)
}
func mpsBound(kind, data string) string { return fmt.Sprintf(" %-2s %-8s  %s\n", kind, "BND", data) }

func lpFrontendFixture(t *testing.T, input string, values map[string]string) (*shell.LPFrontend, *bytes.Buffer) {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	globals, err := context.GlobalParameters()
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range values {
		if err = globals.SourceSetName(name, value); err != nil {
			t.Fatal(err)
		}
	}
	out := &bytes.Buffer{}
	frontend, err := shell.NewLPFrontend(shell.LPFrontendConfig{Globals: globals, Input: strings.NewReader(input), Output: out, Name: "fixture.mps"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := frontend.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return frontend, out
}

func oneVariableMPS(rowKind string, rhs float64, bounds string) string {
	return "NAME ONE\nROWS\n N COST\n " + rowKind + " LIMIT\nCOLUMNS\n" + mpsPair("x", "COST", 2) + mpsPair("x", "LIMIT", 1) + "RHS\n" + mpsPair("RHS1", "LIMIT", rhs) + bounds + "ENDATA\n"
}
