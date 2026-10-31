package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
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
