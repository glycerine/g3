package duality

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityNormalizeClauseHeadParametersAndConstraints(t *testing.T) {
	ctx, m, native := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
	one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	sort, err := m.GetSort(raw(one))
	must(err)
	constant := func(text string) *Expr {
		name, err := m.Symbols.Intern(text)
		must(err)
		return wrap(m.MkConst(name, sort))
	}
	x := constant("head-x")
	v := wrap(m.MkVar(0, sort))
	compound := wrap(m.MkArithmeticAdd(raw(x), raw(one)))
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("head-R")
	must(err)
	domain := []ast.ASTHandle{sort, sort, sort, sort, sort, sort}
	decl, err := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	head := wrap(m.MkApp(decl, []ast.ASTHandle{raw(x), raw(x), raw(compound), raw(one), raw(v), raw(v)}))
	body := wrap(m.MkArithmeticGT(raw(x), raw(one)))
	nodes, err := normalizeClauseHead(head, nil)
	must(err)
	defer nodes.close()
	edges, err := normalizeClauseHead(head, body)
	must(err)
	defer edges.close()
	if nodes.Body != nil || len(nodes.Parameters) != 6 || len(edges.Parameters) != 6 {
		t.Fatal("head normalization arity mismatch")
	}
	wants := []*Expr{x, constant("@a_1"), constant("@a_2"), constant("@a_3"), v, constant("@a_5")}
	for i, want := range wants {
		if raw(nodes.Parameters[i]) != raw(want) || raw(edges.Parameters[i]) != raw(want) {
			t.Fatalf("parameter %d mismatch", i)
		}
	}
	// Constraints are appended in source order to the existing body.
	cursor := raw(edges.Body)
	for _, i := range []int{5, 3, 2, 1} {
		op, err := m.GetDeclKind(cursor)
		must(err)
		if op != ast.ASTOpAnd {
			t.Fatal("constraint conjunction shape changed")
		}
		equality, err := m.GetArg(cursor, 1)
		must(err)
		left, err := m.GetArg(equality, 0)
		must(err)
		original, err := m.GetArg(raw(head), uint32(i))
		must(err)
		right, err := m.GetArg(equality, 1)
		must(err)
		if left != original || right != raw(wants[i]) {
			t.Fatal("head equality order/orientation mismatch")
		}
		cursor, err = m.GetArg(cursor, 0)
		must(err)
	}
	if cursor != raw(body) {
		t.Fatal("original body changed")
	}
	// Instantiate the remaining bound variable, then check that the generated
	// equalities enforce the original head arguments in their new positions.
	replacement := constant("head-instantiation")
	g, err := NewRPFP(ctx)
	must(err)
	defer g.Close()
	instantiated, err := g.SubstBound(map[uint32]*Expr{0: replacement}, edges.Body)
	must(err)
	defer instantiated.Close()
	mismatch := wrap(m.MkNot(raw(wrap(m.MkEq(raw(wants[2]), raw(compound))))))
	violation := wrap(m.MkAnd(raw(instantiated), raw(mismatch)))
	must(native.Push())
	must(native.Assert(raw(violation)))
	status, err := native.CheckSat(nil)
	must(err)
	must(native.Pop(1))
	if status != lbool.False {
		t.Fatal("compound head parameter lost its defining constraint")
	}
	// Query heads are nullary and preserve their bodies.
	no := wrap(m.MkFalse())
	query, err := normalizeClauseHead(no, body)
	must(err)
	defer query.close()
	if len(query.Parameters) != 0 || raw(query.Body) != raw(body) {
		t.Fatal("query gained head parameters")
	}
	// Outputs own the rewritten body and parameters independently.
	saved, err := edges.Body.Clone()
	must(err)
	defer saved.Close()
	must(edges.close())
	must(edges.close())
	if _, err := saved.Raw(); err != nil {
		t.Fatal("head result ownership lost", err)
	}
}
