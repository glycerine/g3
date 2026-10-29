package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func runApplyTacticScript(t *testing.T, script string) (string, error) {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	err = ParseCommands(context, strings.NewReader(script), &output)
	return output.String(), err
}
