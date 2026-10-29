package cmd_context

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogParserDeclarationsFactsRulesAndInfix(t *testing.T) {
	context := contextFixture(t)
	program := "D 3\n\nP(x:D)\nQ(x:D)\nP(\"a\").\nQ(X) :- P(X), X != \"b\".\n"
	contextMust(t, context.ParseDatalogString(program))
	q, err := context.FindFunctionDeclaration("Q", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	answer, err := context.QueryFixedpoint(q)
	contextMust(t, err)
	if answer != lbool.True {
		t.Fatal("parsed program query", answer)
	}
	engine, err := context.fixedpointContext()
	contextMust(t, err)
	rules, err := engine.Rules()
	contextMust(t, err)
	if len(rules) != 2 {
		t.Fatalf("parsed rules %d, want 2", len(rules))
	}
	manager, err := context.Manager()
	contextMust(t, err)
	for i := range rules {
		err = errors.Join(err, manager.DelRef(&rules[i]))
	}
	contextMust(t, err)
}

func TestDatalogParserDomainSuffixWildcardAndBounds(t *testing.T) {
	context := contextFixture(t)
	contextMust(t, context.ParseDatalogString("Node12 2\n\nR(x:Node99)\nR(_).\n"))
	if err := context.ParseDatalogString("Tiny 1\n\nT(x:Tiny)\nT(2).\n"); err == nil {
		t.Fatal("accepted out-of-range finite numeral")
	}
}
