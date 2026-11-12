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
	_, _, canceled := qeTestEliminate(m, quantifier, f.r.config)
	m.ResourceLimit().ResetCancel()
	if canceled == nil {
		t.Fatal("canceled closed inequality succeeded")
	}
	result, complete, err := qeTestEliminate(m, quantifier, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("independent inequality was not eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&result)) }()
	value, err := m.RefGet(result)
	fmMust(t, err)
	// quant_elim_plugin::check splits (and (> x 0) r) by relevance
	// (get_max_relevant), eliminates x from (> x 0) alone and substitutes
	// the result back: expr_quant_elim yields (and r true); qe_tactic's goal
	// update then reduces it to r, as z3 4.7.1 prints for (apply qe).
	truth := f.own(m.MkTrue())
	if value != f.own(m.MkAnd(r, truth)) {
		t.Fatal("closed condition was not preserved")
	}
	nonnegative := f.own(m.MkArithmeticGE(x, zero))
	forall := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: nonnegative}))
	falseResult, complete, err := qeTestEliminate(m, forall, f.r.config)
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
	dependentUniversal := f.own(m.MkArithmeticGT(x, y))
	forallDependent := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: dependentUniversal}))
	// z3 4.7.1 decides both quantifiers over a free bound: (forall x (> x y))
	// is false and (exists x (> x y)) is true under (apply qe).
	universal, done, err := qeTestEliminate(m, forallDependent, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&universal)) }()
	if !done {
		t.Fatal("universal inequality depending on a free symbol was not eliminated")
	}
	if value, err := m.RefGet(universal); err != nil || !m.IsFalse(value) {
		t.Fatal("not every integer exceeds a free bound", err)
	}
	dependent := f.own(m.MkArithmeticGT(x, y))
	existential := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: dependent}))
	witness, complete, err := qeTestEliminate(m, existential, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&witness)) }()
	if !complete {
		t.Fatal("inequality depending on a free symbol was not eliminated")
	}
	if value, err := m.RefGet(witness); err != nil || !m.IsTrue(value) {
		t.Fatal("some integer exceeds every free bound", err)
	}
}
