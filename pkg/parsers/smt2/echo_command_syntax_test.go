package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2EchoRequiresExactlyOneString(t *testing.T) {
	// AUDIT.md parsers_shell_01-F8: echo_cmd prints in set_next_arg, so
	// the extra argument is rejected after "a" is printed (as z3 does).
	printed := map[string]string{"(echo \"a\" \"b\")": "a\n"}
	for _, command := range []string{"(echo)", "(echo symbol)", "(echo \"a\" \"b\")"} {
		t.Run(command, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			context.PrintSuccess = false
			var output bytes.Buffer
			err = ParseCommands(context, strings.NewReader(command), &output)
			if err == nil || output.String() != printed[command] {
				t.Fatal(err, output.String())
			}
			if closeErr := context.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		})
	}
}
