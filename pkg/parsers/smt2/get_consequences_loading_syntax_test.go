package smt2

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestParseAssertionsStillRejectsMalformedConsequences(t *testing.T) {
	for _, script := range []string{
		`(get-consequences ())`,
		`(get-consequences () () trailing)`,
		`(get-consequences true ())`,
	} {
		c, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		err = ParseAssertions(c, strings.NewReader(script))
		_ = c.Close()
		if err == nil {
			t.Fatal("accepted malformed get-consequences", script)
		}
	}
}
