package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserFiniteOrderingUsesDatalogOperator(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "N 3\nI int\n\nP(x:N)\nQ(x:N)\nA(x:I)\nB(x:I)\nQ(X) :- P(X), X < \"b\".\nQ(X) :- P(X), \"a\" > X.\nB(X) :- A(X), X < 2.\n"
	fpMust(t, fp.ParseDatalogString(f.context, input))
	rules, err := f.context.Rules()
	fpMust(t, err)
	manager, err := f.context.Manager()
	fpMust(t, err)
	for i := 0; i < 3; i++ {
		rule, err := rules.Rule(i)
		fpMust(t, err)
		tail, negative, err := rule.Tail(1)
		fpMust(t, err)
		if negative {
			t.Fatal("ordering tail unexpectedly negated", i)
		}
		finite, err := manager.IsDatalogLT(tail)
		fpMust(t, err)
		if finite != (i < 2) {
			t.Fatalf("rule %d finite ordering operator = %v", i, finite)
		}
		if i == 1 {
			left, err := manager.GetArg(tail, 0)
			fpMust(t, err)
			kind, err := manager.GetKind(left)
			fpMust(t, err)
			if kind != ast.ASTVar {
				t.Fatal("greater-than did not reverse finite operands", kind)
			}
		}
		fpMust(t, rule.Close())
	}
}
