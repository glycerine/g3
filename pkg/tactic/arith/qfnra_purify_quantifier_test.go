package arith

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestPurifyQuantifierScopesFreshVariablesAndProof(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprintf("proofs=%v", proofs), func(t *testing.T) {
			runner, m := qfnraPurifier(t, proofs, true)
			constant := fixDLConst(t, m, false, "quantifier-sort")
			sort, err := m.GetSort(constant)
			equationMust(t, err)
			x, err := m.MkVar(0, sort)
			equationMust(t, err)
			fixDLPin(t, m, x)
			integer := fixDLApp(t, m, qfnraPurifyToInt, x)
			zero := fixDLNumber(t, m, true, "0")
			body, err := m.MkEq(integer, zero)
			equationMust(t, err)
			fixDLPin(t, m, body)
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
			equationMust(t, err)
			fixDLPin(t, m, q)
			goal, err := tactic.NewGoal(m, proofs, true, false)
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, goal.Close()) })
			var p ast.ASTHandle
			if proofs {
				p, err = m.MkProofAsserted(q)
				equationMust(t, err)
			}
			equationMust(t, goal.AssertWithProof(q, p, ast.ASTDependency{}))
			var result tactic.TacticResult
			t.Cleanup(func() { equationMust(t, result.Close()) })
			equationMust(t, runner.Apply(goal, &result))
			size, err := goal.Size()
			equationMust(t, err)
			if size != 1 {
				t.Fatalf("quantified constraints escaped into %d goal formulas", size)
			}
			outer, err := goal.Form(0)
			equationMust(t, err)
			isForall, err := m.IsForall(outer)
			equationMust(t, err)
			if !isForall {
				t.Fatal("original universal quantifier changed")
			}
			inner, err := m.GetQuantifierBody(outer)
			equationMust(t, err)
			isForall, err = m.IsForall(inner)
			equationMust(t, err)
			if isForall {
				t.Fatal("fresh variable was not existentially bound")
			}
			count, err := m.GetNumDeclarations(inner)
			equationMust(t, err)
			if count != 1 {
				t.Fatalf("fresh binder has %d variables", count)
			}
			conjunction, err := m.GetQuantifierBody(inner)
			equationMust(t, err)
			count, err = m.GetNumArgs(conjunction)
			equationMust(t, err)
			if count != 3 {
				t.Fatalf("conjunction has %d arguments", count)
			}
			lower, err := m.GetArg(conjunction, 0)
			equationMust(t, err)
			diff, err := m.GetArg(lower, 0)
			equationMust(t, err)
			outerVar, err := m.GetArg(diff, 0)
			equationMust(t, err)
			index, err := m.GetVarIndex(outerVar)
			equationMust(t, err)
			if index != 1 {
				t.Fatalf("outer variable index = %d", index)
			}
			rewritten, err := m.GetArg(conjunction, 2)
			equationMust(t, err)
			freshVar, err := m.GetArg(rewritten, 0)
			equationMust(t, err)
			index, err = m.GetVarIndex(freshVar)
			equationMust(t, err)
			if index != 0 {
				t.Fatalf("fresh variable index = %d", index)
			}
			assertEmptyPurifyFilter(t, result.ModelConverter)
			if proofs {
				p, e := goal.Proof(0)
				equationMust(t, e)
				fact, e := m.GetProofFact(p)
				equationMust(t, e)
				if fact != outer {
					t.Fatal("quantified proof has wrong conclusion")
				}
			}
		})
	}
}

func TestPurifyNestedQuantifiersShiftOuterVariables(t *testing.T) {
	runner, m := qfnraPurifier(t, false, true)
	c := fixDLConst(t, m, false, "nested-quantifier-sort")
	sort, err := m.GetSort(c)
	equationMust(t, err)
	xOuter, err := m.MkVar(0, sort)
	equationMust(t, err)
	fixDLPin(t, m, xOuter)
	xInner, err := m.MkVar(1, sort)
	equationMust(t, err)
	fixDLPin(t, m, xInner)
	yInner, err := m.MkVar(0, sort)
	equationMust(t, err)
	fixDLPin(t, m, yInner)
	zero := fixDLNumber(t, m, true, "0")
	outerTerm := fixDLApp(t, m, qfnraPurifyToInt, xOuter)
	outerEq, err := m.MkEq(outerTerm, zero)
	equationMust(t, err)
	fixDLPin(t, m, outerEq)
	sum, err := m.MkArithmeticAdd(xInner, yInner)
	equationMust(t, err)
	fixDLPin(t, m, sum)
	innerTerm := fixDLApp(t, m, qfnraPurifyToInt, sum)
	innerEq, err := m.MkEq(innerTerm, zero)
	equationMust(t, err)
	fixDLPin(t, m, innerEq)
	inner, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(1)}, Body: innerEq})
	equationMust(t, err)
	fixDLPin(t, m, inner)
	body, err := m.MkAnd(outerEq, inner)
	equationMust(t, err)
	fixDLPin(t, m, body)
	outer, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	equationMust(t, err)
	fixDLPin(t, m, outer)
	goal, err := tactic.NewGoal(m, false, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.Assert(outer))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	size, err := goal.Size()
	equationMust(t, err)
	if size != 1 {
		t.Fatalf("got %d formulas", size)
	}
	outer, err = goal.Form(0)
	equationMust(t, err)
	outerFresh, err := m.GetQuantifierBody(outer)
	equationMust(t, err)
	outerConj, err := m.GetQuantifierBody(outerFresh)
	equationMust(t, err)
	outerBody, err := m.GetArg(outerConj, 2)
	equationMust(t, err)
	inner, err = m.GetArg(outerBody, 1)
	equationMust(t, err)
	innerFresh, err := m.GetQuantifierBody(inner)
	equationMust(t, err)
	innerConj, err := m.GetQuantifierBody(innerFresh)
	equationMust(t, err)
	lower, err := m.GetArg(innerConj, 0)
	equationMust(t, err)
	diff, err := m.GetArg(lower, 0)
	equationMust(t, err)
	sum, err = m.GetArg(diff, 0)
	equationMust(t, err)
	xInner, err = m.GetArg(sum, 0)
	equationMust(t, err)
	yInner, err = m.GetArg(sum, 1)
	equationMust(t, err)
	xIndex, err := m.GetVarIndex(xInner)
	equationMust(t, err)
	yIndex, err := m.GetVarIndex(yInner)
	equationMust(t, err)
	if xIndex != 3 || yIndex != 1 {
		t.Fatalf("nested indices x=%d y=%d", xIndex, yIndex)
	}
}

func TestPurifyUnchangedQuantifierWithProof(t *testing.T) {
	runner, m := qfnraPurifier(t, true, true)
	c := fixDLConst(t, m, false, "unchanged-quantifier-sort")
	sort, err := m.GetSort(c)
	equationMust(t, err)
	x, err := m.MkVar(0, sort)
	equationMust(t, err)
	fixDLPin(t, m, x)
	zero := fixDLNumber(t, m, false, "0")
	body, err := m.MkArithmeticGE(x, zero)
	equationMust(t, err)
	fixDLPin(t, m, body)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	equationMust(t, err)
	fixDLPin(t, m, q)
	p, err := m.MkProofAsserted(q)
	equationMust(t, err)
	goal, err := tactic.NewGoal(m, true, false, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.AssertWithProof(q, p, ast.ASTDependency{}))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	got, err := goal.Form(0)
	equationMust(t, err)
	if got != q {
		t.Fatal("unchanged quantifier was modified")
	}
	proof, err := goal.Proof(0)
	equationMust(t, err)
	fact, err := m.GetProofFact(proof)
	equationMust(t, err)
	if fact != q {
		t.Fatal("unchanged quantified proof has wrong conclusion")
	}
}

func TestPurifyQuantifiedModulusBindsBothAuxiliaries(t *testing.T) {
	runner, m := qfnraPurifier(t, true, true)
	c := fixDLConst(t, m, true, "quantified-modulus-sort")
	sort, err := m.GetSort(c)
	equationMust(t, err)
	x, err := m.MkVar(0, sort)
	equationMust(t, err)
	fixDLPin(t, m, x)
	three := fixDLNumber(t, m, true, "3")
	mod := fixDLApp(t, m, qfnraPurifyMod, x, three)
	body, err := m.MkArithmeticLT(mod, three)
	equationMust(t, err)
	fixDLPin(t, m, body)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	equationMust(t, err)
	fixDLPin(t, m, q)
	p, err := m.MkProofAsserted(q)
	equationMust(t, err)
	goal, err := tactic.NewGoal(m, true, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.AssertWithProof(q, p, ast.ASTDependency{}))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	size, err := goal.Size()
	equationMust(t, err)
	if size != 1 {
		t.Fatalf("got %d formulas", size)
	}
	outer, err := goal.Form(0)
	equationMust(t, err)
	inner, err := m.GetQuantifierBody(outer)
	equationMust(t, err)
	count, err := m.GetNumDeclarations(inner)
	equationMust(t, err)
	if count != 2 {
		t.Fatalf("got %d existential variables", count)
	}
	conjunction, err := m.GetQuantifierBody(inner)
	equationMust(t, err)
	count, err = m.GetNumArgs(conjunction)
	equationMust(t, err)
	if count != 5 {
		t.Fatalf("got %d quantified conjuncts", count)
	}
	first, err := m.GetArg(conjunction, 0)
	equationMust(t, err)
	equality, err := m.GetArg(first, 1)
	equationMust(t, err)
	x, err = m.GetArg(equality, 0)
	equationMust(t, err)
	index, err := m.GetVarIndex(x)
	equationMust(t, err)
	if index != 2 {
		t.Fatalf("original variable index = %d", index)
	}
	assertEmptyPurifyFilter(t, result.ModelConverter)
	proof, err := goal.Proof(0)
	equationMust(t, err)
	fact, err := m.GetProofFact(proof)
	equationMust(t, err)
	if fact != outer {
		t.Fatal("quantified modulus proof has wrong conclusion")
	}
}
