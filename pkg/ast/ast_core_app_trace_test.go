package ast

import (
	"bytes"
	"errors"
	"testing"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestASTChainableTraceOnlyOnLongNonflatBranch(t *testing.T) {
	symbols, numerals, budget := astTestOwners(t)
	var output bytes.Buffer
	tags := udiag.NewTraceTagsWithWriter(&output)
	m, err := NewASTManager(ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget, TraceTags: tags})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	args := make([]ASTHandle, 3)
	for i := range args {
		args[i], err = m.MkVar(uint32(i), sort)
		astTestMust(t, err)
		astTestPin(t, m, args[i])
	}
	pair, err := m.MkIff(args[0], args[1])
	astTestMust(t, err)
	astTestPin(t, m, pair)
	decl, err := m.GetDecl(pair)
	astTestMust(t, err)
	if _, err := m.MkApp(decl, args); err != nil || output.Len() != 0 {
		t.Fatal("disabled chainable trace", err, output.String())
	}
	tags.Enable("chainable")
	if _, err := m.MkApp(decl, args[:1]); !errors.Is(err, ErrASTDomain) || output.Len() != 0 {
		t.Fatal("source basic-family unary precondition", err, output.String())
	}
	if _, err := m.MkApp(decl, args[:2]); err != nil || output.Len() != 0 {
		t.Fatal("binary application emitted chain trace", err, output.String())
	}
	result, err := m.MkApp(decl, args)
	astTestMust(t, err)
	astTestPin(t, m, result)
	if tags.ShouldTrace("chainable") {
		const want = "-------- [chainable] mk_app src/ast/ast.cpp:2195 ---------\n" +
			"chainable...\n" +
			"------------------------------------------------\n"
		if got := output.String(); got != want {
			t.Fatalf("source chainable record: got %q, want %q", got, want)
		}
	} else if output.Len() != 0 {
		t.Fatal("erased chainable trace emitted", output.String())
	}
}
