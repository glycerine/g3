package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2CheckSatRejectsNonBooleanAssumptions(t *testing.T) {
	for _, command := range []string{"(check-sat x)", "(check-sat-assuming (x))"} {
		t.Run(command, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			context.PrintSuccess = false
			var output bytes.Buffer
			err = ParseCommands(context, strings.NewReader("(declare-const x Int)"+command), &output)
			if err == nil || !strings.Contains(err.Error(), "argument must be a Boolean literal") || output.Len() != 0 {
				t.Fatal(err, output.String())
			}
			if closeErr := context.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		})
	}
}
