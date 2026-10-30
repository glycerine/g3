package cmd_context

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBuiltinTacticDescriptionRejectsUnknownNames(t *testing.T) {
	for _, name := range []string{"", "not-a-retained-tactic", "SKIP"} {
		if _, err := BuiltinTacticDescription(name); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("unknown description", name, err)
		}
	}
}
