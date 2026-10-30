package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2EchoRequiresExactlyOneString(t *testing.T) {
	for _, command := range []string{"(echo)", "(echo symbol)", "(echo \"a\" \"b\")"} {
		t.Run(command, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			context.PrintSuccess = false
			var output bytes.Buffer
			err = ParseCommands(context, strings.NewReader(command), &output)
			if err == nil || output.Len() != 0 {
				t.Fatal(err, output.String())
			}
			if closeErr := context.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		})
	}
}
