package ast

import (
	"errors"
	"testing"
)

func TestASTDependencyDiagnosticAddressAndOwnership(t *testing.T) {
	m := astTestManager(t)
	if text, err := m.DependencyAddressText(ASTDependency{}); err != nil || text != "0" {
		t.Fatal(text, err)
	}
	leaf, err := m.DependencyLeaf(astDependencyExpr(t, m, 1950))
	astTestMust(t, err)
	astTestMust(t, m.DependencyInc(leaf))
	text, err := m.DependencyAddressText(leaf)
	astTestMust(t, err)
	other, err := m.DependencyLeaf(astDependencyExpr(t, m, 1951))
	astTestMust(t, err)
	astTestMust(t, m.DependencyInc(other))
	defer func() { astTestMust(t, m.DependencyDec(other)) }()
	next, err := m.DependencyAddressText(other)
	astTestMust(t, err)
	if text == "0" || text == next {
		t.Fatal("missing actual allocation identity", text, next)
	}
	same, err := m.DependencyJoin(leaf, leaf)
	astTestMust(t, err)
	if address, err := m.DependencyAddressText(same); err != nil || address != text {
		t.Fatal("alias address changed", address, text, err)
	}
	foreign := astTestManager(t)
	if _, err = foreign.DependencyAddressText(leaf); !errors.Is(err, ErrASTHandle) {
		t.Fatal("foreign handle", err)
	}
	astTestMust(t, m.DependencyDec(leaf))
	if _, err = m.DependencyAddressText(leaf); !errors.Is(err, ErrASTHandle) {
		t.Fatal("dead allocation", err)
	}
}
