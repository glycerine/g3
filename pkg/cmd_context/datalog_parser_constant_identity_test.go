package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source parse_arg canonicalizes numeric arguments through mk_symbol_const;
// parse_infix instead passes each constant's original spelling to mk_const.
func TestDatalogParserSourceConstantIdentity(t *testing.T) {
	for _, test := range []struct {
		name, fact, operand string
		want                lbool.LBool
	}{
		{"numeric-argument", "0007", "7", lbool.True},
		{"negative-zero-argument", "-000", "0", lbool.True},
		{"negative-zero-infix-spelling", "-0", "-0", lbool.False},
		{"infix-spelling", "0007", "0007", lbool.False},
		{"quoted-argument", `"0007"`, "0007", lbool.True},
		{"negative-finite-symbol", `"-3"`, "-3", lbool.True},
		{"wide-finite-symbol", `"18446744073709551616"`, "18446744073709551616", lbool.True},
		{"quoted-variable-name", `"a"`, `"X"`, lbool.False},
	} {
		t.Run(test.name, func(t *testing.T) {
			context := contextFixture(t)
			input := "N 2\n\nP(x:N)\nQ(x:N)\nP(" + test.fact + ").\nQ(X) :- P(X), X = " + test.operand + ".\n"
			contextMust(t, context.ParseDatalogString(input))
			query, err := context.FindFunctionDeclaration("Q", nil, nil, ast.ASTHandle{})
			contextMust(t, err)
			status, err := context.QueryFixedpoint(query)
			contextMust(t, err)
			if status != test.want {
				t.Fatal("constant spelling changed the derived relation", status, test.want)
			}
		})
	}
}

func TestDatalogParserInfixRequiresIdentifierVariables(t *testing.T) {
	for _, guard := range []string{`"X" = "a"`, `"a" = "X"`, "X = Missing"} {
		t.Run(guard, func(t *testing.T) {
			context := contextFixture(t)
			input := "N 3\n\nP(x:N)\nQ(x:N)\nQ(X) :- P(X), " + guard + ".\n"
			if err := context.ParseDatalogString(input); err == nil {
				t.Fatal("infix accepted a quoted or unknown variable", guard)
			}
		})
	}
}
