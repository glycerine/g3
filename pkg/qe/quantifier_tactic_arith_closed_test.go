package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierArithmeticClosedInequality(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.integer))
	y := f.constant("qe-closed-y", f.integer)
	r := f.constant("qe-closed-condition", f.boolean)
	zero := f.number("0")
	positive := f.own(m.MkArithmeticGT(x, zero))
	body := f.own(m.MkAnd(positive, r))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	m.ResourceLimit().Cancel()
	_, _, canceled := eliminateQuantifierArithmeticClosed(m, quantifier, f.r.config)
	m.ResourceLimit().ResetCancel()
	if canceled == nil {
		t.Fatal("canceled closed inequality succeeded")
	}
	result, complete, err := eliminateQuantifierArithmeticClosed(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("independent inequality was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	if value != r {
		t.Fatal("closed condition was not preserved")
	}
	nonnegative := f.own(m.MkArithmeticGE(x, zero))
	forall := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: nonnegative}))
	falseResult, complete, err := eliminateQuantifierArithmeticClosed(m, forall, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("universal inequality was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&falseResult)) }()
	falseValue, err := m.RefGet(falseResult)
	fmMust(t, err)
	if !m.IsFalse(falseValue) {
		t.Fatal("not every integer is nonnegative")
	}
	dependent := f.own(m.MkArithmeticGT(x, y))
	unsupported := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: dependent}))
	unexpected, complete, err := eliminateQuantifierArithmeticClosed(m, unsupported, f.r.config)
	fmMust(t, err)
	if complete || unexpected != (ast.ASTRef{}) {
		if unexpected != (ast.ASTRef{}) {
			fmMust(t, m.DelRef(&unexpected))
		}
		t.Fatal("inequality depending on a free symbol was prematurely eliminated")
	}
}
