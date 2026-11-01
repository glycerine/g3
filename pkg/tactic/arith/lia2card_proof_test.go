package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestLIA2CardProofCarriesOriginalGoalPremises(t *testing.T) {
	base, m := fixDLFixture(t, true)
	equationMust(t, m.RegisterDeclarationPlugins())
	port, err := NewLIA2CardTactic(m, base.context, base.globals, nil)
	equationMust(t, err)
	defer func() { equationMust(t, port.Close()) }()
	x := fixDLConst(t, m, true, "card-proof-x")
	zero := fixDLNumber(t, m, true, "0")
	one := fixDLNumber(t, m, true, "1")
	lower := fixDLApp(t, m, 3, x, zero)
	upper := fixDLApp(t, m, 2, x, one)
	formula := collectorEq(t, m, fixDLApp(t, m, 6, x, one), one)
	goal := fixDLGoal(t, m, true, false, false, lower, upper, formula)
	original := make([]ast.ASTHandle, 3)
	for i := range original {
		original[i], err = goal.Proof(uint32(i))
		equationMust(t, err)
	}
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	equationMust(t, port.Apply(goal, &result))
	checker, err := proofs.NewChecker(m)
	equationMust(t, err)
	size, err := goal.Size()
	equationMust(t, err)
	if size < 5 {
		t.Fatal("expected reasserted bounds", size)
	}
	// At x=2 the original equality is false but its indicator rewrite is
	// true; the 0/1 bounds are therefore essential proof premises.
	converted, err := goal.Form(2)
	equationMust(t, err)
	md, err := model.NewModel(m)
	equationMust(t, err)
	decl, err := m.GetDecl(x)
	equationMust(t, err)
	two := fixDLNumber(t, m, true, "2")
	equationMust(t, md.Core().RegisterConstant(decl, two))
	evaluator, err := model.NewModelEvaluator(md.Core(), base.services)
	equationMust(t, err)
	for _, item := range []struct {
		term ast.ASTHandle
		want bool
	}{{formula, false}, {converted, true}} {
		valueRef, e := evaluator.EvaluateValue(item.term)
		equationMust(t, e)
		value, e := m.RefGet(valueRef)
		equationMust(t, e)
		if (item.want && !m.IsTrue(value)) || (!item.want && !m.IsFalse(value)) {
			t.Fatal("expected counterexample to unconditional rewrite", value)
		}
		equationMust(t, m.DelRef(&valueRef))
	}
	equationMust(t, evaluator.Close())
	equationMust(t, md.Close())
	for i := uint32(0); i < size; i++ {
		form, e := goal.Form(i)
		equationMust(t, e)
		proof, e := goal.Proof(i)
		equationMust(t, e)
		fact, e := m.GetProofFact(proof)
		equationMust(t, e)
		if fact != form {
			t.Fatal("proof conclusion", i)
		}
		todo := []ast.ASTHandle{proof}
		found := false
		for len(todo) > 0 {
			p := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			isLemma, e := m.IsProofKind(p, ast.ASTPrTheoryLemma)
			equationMust(t, e)
			count, e := m.GetProofNumParents(p)
			equationMust(t, e)
			if isLemma {
				if int(count) != len(original) {
					t.Fatal("missing original premises", i, count)
				}
				for j, want := range original {
					parent, e := m.GetProofParent(p, uint32(j))
					equationMust(t, e)
					if parent != want {
						t.Fatal("incorrect original premise", i, j)
					}
				}
				found = true
				break
			}
			for j := uint32(0); j < count; j++ {
				parent, e := m.GetProofParent(p, j)
				equationMust(t, e)
				todo = append(todo, parent)
			}
		}
		if !found {
			t.Fatal("missing conditional theory lemma", i)
		}
		valid, side, e := checker.Check(proof)
		equationMust(t, e)
		if !valid || len(side) != 0 {
			t.Fatal("invalid proof or unconditional rewrite side condition", i, valid, len(side))
		}
		equationMust(t, checker.ReleaseSideConditions(side))
	}
}

func TestLIA2CardKeepsProofForUnchangedFormula(t *testing.T) {
	base, m := fixDLFixture(t, true)
	port, err := NewLIA2CardTactic(m, base.context, base.globals, nil)
	equationMust(t, err)
	defer func() { equationMust(t, port.Close()) }()
	p := cardinalityBool(t, m, "card-proof-unchanged")
	goal := fixDLGoal(t, m, true, false, false, p)
	original, err := goal.Proof(0)
	equationMust(t, err)
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	equationMust(t, port.Apply(goal, &result))
	proof, err := goal.Proof(0)
	equationMust(t, err)
	if proof != original {
		t.Fatal("unchanged formula acquired a replacement proof")
	}
}
