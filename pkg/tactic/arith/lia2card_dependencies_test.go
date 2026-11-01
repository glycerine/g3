package arith

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestLIA2CardTracksBoundsInUnsatCoreDependencies(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprintf("proofs=%v", proofs), func(t *testing.T) {
			testLIA2CardDependencies(t, proofs)
		})
	}
}

func testLIA2CardDependencies(t *testing.T, proofs bool) {
	base, m := fixDLFixture(t, proofs)
	port, err := NewLIA2CardTactic(m, base.context, base.globals, nil)
	equationMust(t, err)
	defer func() { equationMust(t, port.Close()) }()
	x := fixDLConst(t, m, true, "core-card-x")
	zero := fixDLNumber(t, m, true, "0")
	one := fixDLNumber(t, m, true, "1")
	two := fixDLNumber(t, m, true, "2")
	lower := fixDLApp(t, m, 3, x, zero)
	upper := fixDLApp(t, m, 2, x, one)
	sum := fixDLApp(t, m, 6, x, one)
	formula := collectorEq(t, m, sum, two)
	labels := []ast.ASTHandle{cardinalityBool(t, m, "card-lower"), cardinalityBool(t, m, "card-upper"), cardinalityBool(t, m, "card-formula")}
	goal, err := tactic.NewGoal(m, proofs, false, true)
	equationMust(t, err)
	defer func() { equationMust(t, goal.Close()) }()
	for i, form := range []ast.ASTHandle{lower, upper, formula} {
		dep, e := m.DependencyLeaf(labels[i])
		equationMust(t, e)
		var proof ast.ASTHandle
		if proofs {
			proof, e = m.MkProofAsserted(form)
			equationMust(t, e)
		}
		equationMust(t, goal.AssertWithProof(form, proof, dep))
	}
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	equationMust(t, port.Apply(goal, &result))
	dep, err := goal.Dependency(2)
	equationMust(t, err)
	leaves, err := m.DependencyLinearize(dep, nil)
	equationMust(t, err)
	seen := make(map[ast.ASTHandle]bool)
	for _, label := range leaves {
		seen[label] = true
	}
	for _, label := range labels {
		if !seen[label] {
			t.Fatal("rewritten formula omitted an indicator dependency", labels, leaves)
		}
	}
	count, err := goal.Size()
	equationMust(t, err)
	if count != 5 {
		t.Fatal("expected two reasserted bounds", count)
	}
	if proofs {
		for i := uint32(0); i < count; i++ {
			form, e := goal.Form(i)
			equationMust(t, e)
			proof, e := goal.Proof(i)
			equationMust(t, e)
			fact, e := m.GetProofFact(proof)
			equationMust(t, e)
			if fact != form {
				t.Fatal("incorrect proof conclusion", i)
			}
		}
	}
	for index, want := range []ast.ASTHandle{labels[1], labels[0]} {
		dep, e := goal.Dependency(uint32(index + 3))
		equationMust(t, e)
		leaves, e := m.DependencyLinearize(dep, nil)
		equationMust(t, e)
		if len(leaves) != 1 || leaves[0] != want {
			t.Fatal("reasserted bound lost its origin", index, leaves)
		}
	}
}

func TestLIA2CardDependenciesExcludeUnrelatedVariable(t *testing.T) {
	base, m := fixDLFixture(t, false)
	port, err := NewLIA2CardTactic(m, base.context, base.globals, nil)
	equationMust(t, err)
	defer func() { equationMust(t, port.Close()) }()
	x := fixDLConst(t, m, true, "card-core-x")
	y := fixDLConst(t, m, true, "card-core-y")
	zero := fixDLNumber(t, m, true, "0")
	one := fixDLNumber(t, m, true, "1")
	two := fixDLNumber(t, m, true, "2")
	forms := []ast.ASTHandle{
		fixDLApp(t, m, 3, x, zero), fixDLApp(t, m, 2, x, one),
		fixDLApp(t, m, 3, y, zero), fixDLApp(t, m, 2, y, one),
		collectorEq(t, m, fixDLApp(t, m, 6, x, one), two),
	}
	labels := make([]ast.ASTHandle, len(forms))
	goal, err := tactic.NewGoal(m, false, false, true)
	equationMust(t, err)
	defer func() { equationMust(t, goal.Close()) }()
	for i, form := range forms {
		labels[i] = cardinalityBool(t, m, fmt.Sprintf("card-core-label-%d", i))
		dep, e := m.DependencyLeaf(labels[i])
		equationMust(t, e)
		equationMust(t, goal.AssertWithProof(form, ast.ASTHandle{}, dep))
	}
	var result tactic.TacticResult
	defer func() { equationMust(t, result.Close()) }()
	equationMust(t, port.Apply(goal, &result))
	dep, err := goal.Dependency(4)
	equationMust(t, err)
	leaves, err := m.DependencyLinearize(dep, nil)
	equationMust(t, err)
	seen := make(map[ast.ASTHandle]bool)
	for _, value := range leaves {
		seen[value] = true
	}
	if !seen[labels[0]] || !seen[labels[1]] || !seen[labels[4]] || seen[labels[2]] || seen[labels[3]] {
		t.Fatal("unexpected formula dependencies", leaves)
	}
}
