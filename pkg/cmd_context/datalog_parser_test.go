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

	numeric := contextFixture(t)
	contextMust(t, numeric.ParseDatalogString("Tiny 1\n\nT(x:Tiny)\nQ(x:Tiny)\nT(999).\nQ(x) :- T(x), x = 999.\n"))
	declaration, err := numeric.FindFunctionDeclaration("Q", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	status, err := numeric.QueryFixedpoint(declaration)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("numeric symbolic finite fact", status)
	}

	overflow := contextFixture(t)
	if err := overflow.ParseDatalogString("Tiny 1\n\nT(x:Tiny)\nT(2).\nT(3).\n"); err == nil {
		t.Fatal("accepted two distinct numeric symbols in a one-element domain")
	}

	wide := contextFixture(t)
	if err := wide.ParseDatalogString("Tiny 1\n\nT(x:Tiny)\nT(18446744073709551616).\n"); err == nil {
		t.Fatal("accepted a finite-domain numeric token outside uint64")
	}
}

func TestDatalogParserCommentOnlyLinesDoNotEndDomains(t *testing.T) {
	context := contextFixture(t)
	input := "# header\nN 2\n# between domains\nM 3\n\nP(x:N,y:M)\nP(\"n\",\"m\").\n"
	contextMust(t, context.ParseDatalogString(input))
	declaration, err := context.FindFunctionDeclaration("P", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	manager, err := context.Manager()
	contextMust(t, err)
	for i, want := range []uint64{2, 3} {
		sort, err := manager.GetDomain(declaration, uint32(i))
		contextMust(t, err)
		size, found, err := manager.DatalogFiniteSortSize(sort)
		contextMust(t, err)
		if !found || size != want {
			t.Fatalf("domain %d size %d found %v, want %d", i, size, found, want)
		}
	}
}
