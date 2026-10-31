package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestRPFPTransformerOwnershipAndBooleanState(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	parameter, err := ctx.BoolConst("transformer-param")
	must(err)
	closeExpr(t, parameter)
	formula, err := ctx.BoolConst("transformer-formula")
	must(err)
	closeExpr(t, formula)
	decl, err := parameter.Decl()
	must(err)
	closeDecl(t, decl)
	transformer, err := NewTransformer(ctx, []*FuncDecl{decl}, []*Expr{parameter}, formula)
	must(err)
	clone, err := transformer.Clone()
	must(err)
	must(transformer.Close())
	copy, err := clone.Formula()
	must(err)
	closeExpr(t, copy)
	got, err := copy.Raw()
	must(err)
	want, err := formula.Raw()
	must(err)
	if got != want {
		t.Fatal("copied formula", got, want)
	}
	must(clone.SetFull())
	yes, err := clone.IsFull()
	must(err)
	if !yes {
		t.Fatal("expected full")
	}
	must(clone.SetEmpty())
	yes, err = clone.IsEmpty()
	must(err)
	if !yes {
		t.Fatal("expected empty")
	}
	must(clone.Complement())
	complemented, err := clone.Formula()
	must(err)
	closeExpr(t, complemented)
	h, err := complemented.Raw()
	must(err)
	op, err := m.GetDeclKind(h)
	must(err)
	if op != ast.ASTOpNot {
		t.Fatal("complement operator", op)
	}
	must(clone.Close())
	if _, err = clone.Formula(); err == nil {
		t.Fatal("closed transformer usable")
	}
}

func TestRPFPTransformerParameterSubstitution(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	x, err := ctx.BoolConst("transformer-x")
	must(err)
	closeExpr(t, x)
	y, err := ctx.BoolConst("transformer-y")
	must(err)
	closeExpr(t, y)
	notY, err := y.Not()
	must(err)
	closeExpr(t, notY)
	target, err := NewTransformer(ctx, nil, []*Expr{x}, x)
	must(err)
	defer func() { must(target.Close()) }()
	source, err := NewTransformer(ctx, nil, []*Expr{y}, notY)
	must(err)
	defer func() { must(source.Close()) }()
	must(target.UnionWith(source))
	formula, err := target.Formula()
	must(err)
	closeExpr(t, formula)
	root, err := formula.Raw()
	must(err)
	op, err := m.GetDeclKind(root)
	must(err)
	if op != ast.ASTOpOr {
		t.Fatal("union operator", op)
	}
	first, err := m.GetArg(root, 0)
	must(err)
	second, err := m.GetArg(root, 1)
	must(err)
	xh, err := x.Raw()
	must(err)
	sub, err := m.GetArg(second, 0)
	must(err)
	if first != xh || sub != xh {
		t.Fatal("parameter substitution", first, sub, xh)
	}
	sourceFormula, err := source.Formula()
	must(err)
	closeExpr(t, sourceFormula)
	unchanged, err := sourceFormula.Raw()
	must(err)
	want, err := notY.Raw()
	must(err)
	if unchanged != want {
		t.Fatal("source was changed")
	}
	must(target.IntersectWith(source))
	formula2, err := target.Formula()
	must(err)
	closeExpr(t, formula2)
	h, err := formula2.Raw()
	must(err)
	op, err = m.GetDeclKind(h)
	must(err)
	if op != ast.ASTOpAnd {
		t.Fatal("intersection operator", op)
	}

	// The source replaces terms beneath binders without shifting them.
	boolean, err := m.MkBoolSort()
	must(err)
	variable, err := m.MkVar(0, boolean)
	must(err)
	bound, err := NewExpr(ctx, variable)
	must(err)
	closeExpr(t, bound)
	yh, err := y.Raw()
	must(err)
	body, err := m.MkOr(yh, variable)
	must(err)
	name, err := m.Symbols.Intern("transformer-bound")
	must(err)
	quantified, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{name}, Body: body})
	must(err)
	qexpr, err := NewExpr(ctx, quantified)
	must(err)
	closeExpr(t, qexpr)
	quantifiedSource, err := NewTransformer(ctx, nil, []*Expr{y}, qexpr)
	must(err)
	defer func() { must(quantifiedSource.Close()) }()
	boundTarget, err := NewTransformer(ctx, nil, []*Expr{bound}, bound)
	must(err)
	defer func() { must(boundTarget.Close()) }()
	replaced, err := boundTarget.substituteParams(quantifiedSource)
	must(err)
	defer func() { must(m.DelRef(&replaced)) }()
	h, err = m.RefGet(replaced)
	must(err)
	body, err = m.GetQuantifierBody(h)
	must(err)
	first, err = m.GetArg(body, 0)
	must(err)
	second, err = m.GetArg(body, 1)
	must(err)
	if first != variable || second != variable {
		t.Fatal("source binder replacement", first, second, variable)
	}
}
