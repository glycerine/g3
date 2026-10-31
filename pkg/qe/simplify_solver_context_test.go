package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSimplifyExistsBooleanArithmeticFixedPoint(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	b := f.constant("simplify-exists-b", f.boolean)
	x := f.constant("simplify-exists-x", f.integer)
	z := f.constant("simplify-exists-z", f.integer)
	equation := f.own(m.MkEq(x, f.number("3")))
	inequality := f.own(m.MkArithmeticLE(z, x))
	formula := f.own(m.MkAndArgs([]ast.ASTHandle{b, equation, inequality}))
	result, err := simplifyExists(m, []ast.ASTHandle{b, x, z}, formula)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	if len(result.variables) != 1 {
		t.Fatalf("remaining variables: %d", len(result.variables))
	}
	variable, err := m.RefGet(result.variables[0])
	fmMust(t, err)
	if variable != z {
		t.Fatal("wrong variable survived")
	}
	simplified, err := m.RefGet(result.formula)
	fmMust(t, err)
	for _, tc := range []struct {
		variable ast.ASTHandle
		want     bool
	}{{b, false}, {x, false}, {z, true}} {
		contains, e := ast.NewContainsApp(m, tc.variable)
		fmMust(t, e)
		got, e := contains.Check(simplified)
		fmMust(t, e)
		fmMust(t, contains.Close())
		if got != tc.want {
			t.Fatalf("variable occurrence = %t, want %t", got, tc.want)
		}
	}
}

func TestSimplifyExistsLeavesUnsolvedVariable(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("simplify-unsolved-x", f.integer)
	formula := f.own(m.MkArithmeticLE(x, f.number("1")))
	result, err := simplifyExists(m, []ast.ASTHandle{x}, formula)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	value, err := m.RefGet(result.formula)
	fmMust(t, err)
	if value != formula || len(result.variables) != 1 {
		t.Fatal("unsolved formula or variable changed")
	}
}

func TestSimplifyExistsOwnsFreshArithmeticVariables(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	x := f.constant("simplify-fresh-x", f.integer)
	y := f.constant("simplify-fresh-y", f.integer)
	twoX := f.own(m.MkArithmeticMul(f.number("2"), x))
	threeY := f.own(m.MkArithmeticMul(f.number("3"), y))
	sum := f.own(m.MkArithmeticAdd(twoX, threeY))
	lhs := f.own(m.MkArithmeticAdd(sum, f.number("1")))
	formula := f.own(m.MkEq(lhs, f.number("0")))
	result, err := simplifyExists(m, []ast.ASTHandle{x, y}, formula)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	if len(result.variables) != 1 {
		t.Fatalf("fresh arithmetic solving left %d variables", len(result.variables))
	}
	remaining, err := m.RefGet(result.variables[0])
	fmMust(t, err)
	if remaining == x || remaining == y {
		t.Fatal("expected a fresh variable")
	}
	abstractor, err := ast.NewExprAbstractor(m)
	fmMust(t, err)
	defer func() { fmMust(t, abstractor.Close()) }()
	quantified, err := m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&quantified)) }()
	body, err := m.RefGet(result.formula)
	fmMust(t, err)
	fmMust(t, abstractor.MkExists([]ast.ASTHandle{remaining}, body, quantified))
	q, err := m.RefGet(quantified)
	fmMust(t, err)
	projected, complete, err := rewriteQuantifierScalar(m, q, f.r.config)
	fmMust(t, err)
	if !complete {
		t.Fatal("fresh-variable formula could not be eliminated")
	}
	defer func() { fmMust(t, m.DelRef(&projected)) }()
	h, err := m.RefGet(projected)
	fmMust(t, err)
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	ref, err := m.NewRef(h)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&ref)) }()
	fmMust(t, rw.Rewrite(h, ref))
	h, err = m.RefGet(ref)
	fmMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("integer equation should simplify to true")
	}
}
