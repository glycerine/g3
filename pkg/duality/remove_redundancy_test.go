package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"math/big"
	"testing"
)

func TestDualityRemoveRedundancyBooleanOrderAndNativeEquivalence(t *testing.T) {
	ctx, m, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	p, err := ctx.BoolConst("redundancy-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("redundancy-q")
	must(err)
	closeExpr(t, q)
	ph, qh := raw(p), raw(q)
	np := wrap(m.MkNot(ph))
	disj := wrap(m.MkOr(ph, qh))
	conj := wrap(m.MkAnd(ph, qh))
	yes, no := wrap(m.MkTrue()), wrap(m.MkFalse())
	notTrue := wrap(m.MkNot(raw(yes)))
	sourceDisjunct := wrap(m.MkAnd(raw(notTrue), qh))
	sourceDisjunction := wrap(m.MkOr(ph, raw(sourceDisjunct)))
	tests := []struct {
		input *Expr
		want  ast.ASTHandle
	}{
		{wrap(m.MkAnd(ph, raw(disj))), ph},
		{wrap(m.MkOr(ph, raw(conj))), raw(sourceDisjunction)},
		{wrap(m.MkOr(ph, ph)), raw(wrap(m.MkOr(ph, raw(notTrue))))},
		{wrap(m.MkAnd(ph, ph)), ph},
		{wrap(m.MkAnd(ph, raw(np))), raw(no)},
		{wrap(m.MkOr(ph, raw(np))), raw(yes)},
		{wrap(m.MkAnd(raw(disj), ph)), ph},
	}
	for _, test := range tests {
		out, err := ctx.RemoveRedundancy(test.input)
		must(err)
		defer out.Close()
		if raw(out) != test.want {
			t.Fatal("unexpected source redundancy reduction", raw(out), test.want)
		}
		difference := wrap(m.MkNot(raw(wrap(m.MkIff(raw(test.input), raw(out))))))
		must(s.Push())
		must(s.Assert(raw(difference)))
		status, err := s.CheckSat(nil)
		must(err)
		must(s.Pop(1))
		if status != lbool.False {
			t.Fatal("reduction changed Boolean semantics", status)
		}
		repeated, err := ctx.RemoveRedundancy(out)
		must(err)
		defer repeated.Close()
		kind, err := m.GetKind(raw(out))
		must(err)
		atom, _, err := IsLiteral(out)
		must(err)
		if atom != nil {
			must(atom.Close())
		}
		if kind == ast.ASTApp && atom != nil && raw(repeated) != raw(out) {
			t.Fatal("reduced literal/constant changed on repeated pass")
		}
	}
}

func TestDualityRemoveRedundancyNumeralEqualityAndQuantifiedFact(t *testing.T) {
	ctx, m, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	integer, err := m.GetSort(raw(one))
	must(err)
	name, err := m.Symbols.Intern("redundancy-x")
	must(err)
	x := wrap(m.MkConst(name, integer))
	yes := wrap(m.MkTrue())
	boolean, err := m.GetSort(raw(yes))
	must(err)
	predicateName, err := m.Symbols.Intern("redundancy-f")
	must(err)
	decl, err := m.MkFuncDecl(predicateName, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	declRef, err := m.NewRef(decl)
	must(err)
	defer func() { must(m.DelRef(&declRef)) }()
	fx := wrap(m.MkApp(decl, []ast.ASTHandle{raw(x)}))
	fone := wrap(m.MkApp(decl, []ast.ASTHandle{raw(one)}))
	eq := wrap(m.MkEq(raw(x), raw(one)))
	for _, and := range []bool{false, true} {
		literal := eq
		if !and {
			literal = wrap(m.MkNot(raw(eq)))
		}
		var input *Expr
		if and {
			input = wrap(m.MkAnd(raw(literal), raw(fx)))
		} else {
			input = wrap(m.MkOr(raw(literal), raw(fx)))
		}
		out, err := ctx.RemoveRedundancy(input)
		must(err)
		defer out.Close()
		last, err := m.GetArg(raw(out), 1)
		must(err)
		if last != raw(fone) {
			t.Fatal("numeral equality did not substitute sibling application")
		}
		difference := wrap(m.MkNot(raw(wrap(m.MkIff(raw(input), raw(out))))))
		must(s.Push())
		must(s.Assert(raw(difference)))
		status, err := s.CheckSat(nil)
		must(err)
		must(s.Pop(1))
		if status != lbool.False {
			t.Fatal("numeral substitution changed semantics", status)
		}
	}
	// Closed quantified facts are treated as literals when eliminating siblings.
	v0 := wrap(m.MkVar(0, boolean))
	quantified := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(v0), Weight: 5}))
	choice := wrap(m.MkOr(raw(quantified), raw(fx)))
	input := wrap(m.MkAnd(raw(quantified), raw(choice)))
	out, err := ctx.RemoveRedundancy(input)
	must(err)
	defer out.Close()
	if raw(out) != raw(quantified) {
		t.Fatal("quantified literal failed to eliminate redundant sibling")
	}
	// Recursing into the quantifier also applies unused-binder deletion.
	unused := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(fx)}))
	dropped, err := ctx.RemoveRedundancy(unused)
	must(err)
	defer dropped.Close()
	if raw(dropped) != raw(fx) {
		t.Fatal("unused quantified binder not removed")
	}
	// Equality argument order is canonicalized by source AST id.
	reversed := wrap(m.MkEq(raw(x), raw(one)))
	canonical, err := ctx.RemoveRedundancy(reversed)
	must(err)
	defer canonical.Close()
	left, err := m.GetArg(raw(canonical), 0)
	must(err)
	right, err := m.GetArg(raw(canonical), 1)
	must(err)
	li, err := m.GetASTID(left)
	must(err)
	ri, err := m.GetASTID(right)
	must(err)
	if li > ri {
		t.Fatal("equality AST-id order not canonicalized")
	}
}

func TestDualityRemoveRedundancyAllocationFailureRetry(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	integer, err := m.GetSort(raw(one))
	must(err)
	name, err := m.Symbols.Intern("redundancy-retry-x")
	must(err)
	x := wrap(m.MkConst(name, integer))
	truth := wrap(m.MkTrue())
	boolean, err := m.GetSort(raw(truth))
	must(err)
	predicate, err := m.Symbols.Intern("redundancy-retry-p")
	must(err)
	decl, err := m.MkFuncDecl(predicate, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	declRef, err := m.NewRef(decl)
	must(err)
	defer func() { must(m.DelRef(&declRef)) }()
	fx := wrap(m.MkApp(decl, []ast.ASTHandle{raw(x)}))
	equality := wrap(m.MkEq(raw(x), raw(one)))
	input := wrap(m.MkAnd(raw(equality), raw(fx)))
	want, err := ctx.RemoveRedundancy(input)
	must(err)
	defer want.Close()
	baseline := m.Stats().LiveReferences
	complete := false
	for allowance := int64(0); allowance < 256; allowance++ {
		m.Budget().FailAfter(allowance)
		out, failure := ctx.RemoveRedundancy(input)
		m.Budget().FailAfter(-1)
		if failure == nil {
			if raw(out) != raw(want) {
				t.Fatal("redundancy result changed", allowance)
			}
			must(out.Close())
			complete = true
		} else {
			if out != nil {
				t.Fatal("partial redundancy result", allowance, failure)
			}
			retry, err := ctx.RemoveRedundancy(input)
			must(err)
			if raw(retry) != raw(want) {
				t.Fatal("retry changed redundancy result", allowance)
			}
			must(retry.Close())
		}
		if got := m.Stats().LiveReferences; got != baseline {
			t.Fatal("redundancy failure/retry leaked references", allowance, got, baseline)
		}
		if complete {
			break
		}
	}
	if !complete {
		t.Fatal("allocation campaign did not reach successful redundancy pass")
	}
}
