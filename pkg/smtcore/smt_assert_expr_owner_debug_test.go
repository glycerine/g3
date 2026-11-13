//go:build g3debug

package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// A fresh input that the assert_expr rewrite replaces dies when the rewriter
// publishes over expr_ref r(e); the debug check names the unowned caller.
func TestSMTAssertExprUnownedInputFailsInDebugBuilds(t *testing.T) {
	saved := debugAssertExprExemptFixtures
	debugAssertExprExemptFixtures = false
	defer func() { debugAssertExprExemptFixtures = saved }()
	m, solver := smtAssertedNNFFixture(t, false)
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "assert-owner-a"), b)
	astTestMust(t, err)
	aRef, err := m.NewRef(a)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&aRef)) }()
	truth, err := m.MkTrue()
	astTestMust(t, err)
	fresh := func() ast.ASTHandle {
		h, err := m.MkAnd(truth, a)
		astTestMust(t, err)
		return h
	}
	err = solver.Assert(fresh())
	if !errors.Is(err, ErrAssertExprInputUnowned) {
		t.Fatal("unowned rewritten input was not reported:", err)
	}
	owned := fresh()
	ref, err := m.NewRef(owned)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(owned))
	astTestMust(t, m.DelRef(&ref))
}
