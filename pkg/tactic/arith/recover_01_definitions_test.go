package arith

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRecover01DefinitionsShareBooleansAndHandleSigns(t *testing.T) {
	_, m := fixDLFixture(t, false)
	boolSort, err := m.MkBoolSort()
	equationMust(t, err)
	boolean := func(name string) ast.ASTHandle {
		sym, e := m.Symbols.Intern(name)
		equationMust(t, e)
		h, e := m.MkConst(sym, boolSort)
		equationMust(t, e)
		fixDLPin(t, m, h)
		return h
	}
	p, q := boolean("recover-p"), boolean("recover-q")
	notQ, err := m.MkNot(q)
	equationMust(t, err)
	fixDLPin(t, m, notQ)
	x := fixDLConst(t, m, true, "recover-x")
	y := fixDLConst(t, m, false, "recover-y")
	goal := fixDLGoal(t, m, false, false, false)
	sub, err := m.NewExprSubstitution()
	equationMust(t, err)
	defer func() { equationMust(t, m.DelExprSubstitution(&sub)) }()
	d := &recover01Definitions{manager: m, goal: goal, sub: sub}
	defer func() { equationMust(t, d.Close()) }()
	equationMust(t, d.build(x, []ast.ASTHandle{p, notQ}, []big.Rat{*big.NewRat(2, 1), *big.NewRat(3, 1)}))
	equationMust(t, d.build(y, []ast.ASTHandle{p}, []big.Rat{*big.NewRat(1, 2)}))
	if len(d.booleans) != 2 {
		t.Fatalf("created %d fresh variables, want two", len(d.booleans))
	}
	size, err := goal.Size()
	equationMust(t, err)
	if size != 4 {
		t.Fatalf("created %d bounds, want four", size)
	}
	find := func(key ast.ASTHandle) ast.ASTHandle {
		var value, proof ast.ASTHandle
		ok, e := m.ExprSubstitutionFindProof(sub, key, &value, &proof)
		equationMust(t, e)
		if !ok {
			t.Fatal("missing substitution")
		}
		return value
	}
	freshP, err := m.RefGet(d.booleans[d.lookup[p]].fresh)
	equationMust(t, err)
	freshQ, err := m.RefGet(d.booleans[d.lookup[q]].fresh)
	equationMust(t, err)
	pEq, err := m.MkEq(freshP, fixDLNumber(t, m, true, "1"))
	equationMust(t, err)
	if find(p) != pEq {
		t.Fatal("incorrect Boolean definition")
	}
	xDef := find(x)
	first, err := m.GetArg(xDef, 0)
	equationMust(t, err)
	second, err := m.GetArg(xDef, 1)
	equationMust(t, err)
	wantFirst := fixDLApp(t, m, 9, fixDLNumber(t, m, true, "2"), freshP)
	sign := fixDLApp(t, m, 7, fixDLNumber(t, m, true, "1"), freshQ)
	wantSecond := fixDLApp(t, m, 9, fixDLNumber(t, m, true, "3"), sign)
	if first != wantFirst || second != wantSecond {
		t.Fatal("integer definition has wrong coefficients or sign")
	}
	realP := fixDLApp(t, m, 14, freshP)
	wantReal := fixDLApp(t, m, 9, fixDLNumber(t, m, false, "1/2"), realP)
	if find(y) != wantReal {
		t.Fatal("real definition has wrong coercion or coefficient")
	}
}
