package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestSimplifyQEQuantifierPreservesOuterBindings(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.own(m.MkVar(0, f.integer))
	b := f.own(m.MkVar(1, f.boolean))
	outer := f.own(m.MkVar(2, f.integer))
	comparison := f.own(m.MkArithmeticLE(x, outer))
	body := f.own(m.MkAnd(b, comparison))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean, f.integer}, Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body}))
	result, err := SimplifyQuantifiers(m, f.r.config, q)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	got, err := m.RefGet(result)
	fmMust(t, err)
	n, err := m.GetNumDeclarations(got)
	fmMust(t, err)
	if n != 1 {
		t.Fatalf("declarations = %d, want 1", n)
	}
	rewritten, err := m.GetQuantifierBody(got)
	fmMust(t, err)
	test, err := m.GetArg(rewritten, 1)
	fmMust(t, err)
	left, err := m.GetArg(test, 0)
	fmMust(t, err)
	right, err := m.GetArg(test, 1)
	fmMust(t, err)
	li, err := m.GetVarIndex(left)
	fmMust(t, err)
	ri, err := m.GetVarIndex(right)
	fmMust(t, err)
	if li != 0 || ri != 1 {
		t.Fatalf("variable indices = %d, %d", li, ri)
	}
}

// AUDIT.md qe_05-F3: reduce_quantifier negates a forall body with
// ast_util mk_not, which strips the body's not and folds true to false.
func TestSimplifyQEQuantifierForallUsesASTUtilNot(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	y := f.constant("y", f.integer)
	x := f.own(m.MkVar(0, f.integer))
	body := f.own(m.MkNot(f.own(m.MkEq(x, y))))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	result, err := SimplifyQuantifiers(m, f.r.config, q)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	got, err := m.RefGet(result)
	fmMust(t, err)
	if got != f.own(m.MkFalse()) {
		t.Fatal("forall x. x != y did not simplify to false")
	}
}

func TestSimplifyQEQuantifierSkipsPatterns(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	v := f.own(m.MkVar(0, f.integer))
	trigger := f.predicate("simplify-trigger", v)
	pattern := f.own(m.MkPattern([]ast.ASTHandle{trigger}))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: trigger, Patterns: []ast.ASTHandle{pattern}}))
	result, err := SimplifyQuantifiers(m, f.r.config, q)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	got, err := m.RefGet(result)
	fmMust(t, err)
	if got != q {
		t.Fatal("annotated quantifier changed")
	}
}

func TestSimplifyQEQuantifierCancellationAndRetry(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	v := f.own(m.MkVar(0, f.boolean))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: v}))
	m.ResourceLimit().Cancel()
	result, err := SimplifyQuantifiers(m, f.r.config, q)
	m.ResourceLimit().ResetCancel()
	if err == nil || result != (ast.ASTRef{}) {
		t.Fatal("canceled simplification succeeded")
	}
	result, err = SimplifyQuantifiers(m, f.r.config, q)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	got, err := m.RefGet(result)
	fmMust(t, err)
	if !m.IsTrue(got) {
		t.Fatal("retry did not eliminate the existential Boolean unit")
	}
}
