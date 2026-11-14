package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestEliminateTermITEProofCoreAndModelFilter(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	runner, err := NewEliminateTermITETactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })

	condition, dependencyLeaf, _ := propagateAtoms(t, m)
	one, err := m.ArithmeticRealNumeralText("1")
	simplifyMust(t, err)
	simplifyPin(t, m, one)
	two, err := m.ArithmeticRealNumeralText("2")
	simplifyMust(t, err)
	simplifyPin(t, m, two)
	choice, err := m.MkIte(condition, one, two)
	simplifyMust(t, err)
	simplifyPin(t, m, choice)
	formula, err := m.MkEq(choice, one)
	simplifyMust(t, err)
	simplifyPin(t, m, formula)
	proof, err := m.MkProofAsserted(formula)
	simplifyMust(t, err)
	dependency, err := m.DependencyLeaf(dependencyLeaf)
	simplifyMust(t, err)

	goal, err := tactic.NewGoal(m, true, true, true)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.AssertWithProof(formula, proof, dependency))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))

	if runner.numFresh != 1 || result.ModelConverter == nil {
		t.Fatal("missing term-ITE abstraction", runner.numFresh, result.ModelConverter)
	}
	// AUDIT.md FUZZ_I-F5: the source tactic has no collect_statistics.
	if sums := statistics.Sums(runner.CollectStatistics); len(sums) != 0 {
		t.Fatal("elim_term_ite_tactic has no collect_statistics", sums)
	}
	size, err := goal.Size()
	simplifyMust(t, err)
	if size != 3 {
		t.Fatal("definition was not appended", size)
	}
	rewritten, err := goal.Form(0)
	simplifyMust(t, err)
	name, err := m.GetArg(rewritten, 0)
	simplifyMust(t, err)
	freshDecl, err := m.GetDecl(name)
	simplifyMust(t, err)
	rewrittenProof, err := goal.Proof(0)
	simplifyMust(t, err)
	fact, err := m.GetProofFact(rewrittenProof)
	simplifyMust(t, err)
	if fact != rewritten {
		t.Fatal("modus-ponens proof does not conclude rewritten formula")
	}
	rewrittenDependency, err := goal.Dependency(0)
	simplifyMust(t, err)
	leaves, err := m.DependencyLinearize(rewrittenDependency, nil)
	simplifyMust(t, err)
	if len(leaves) != 1 || leaves[0] != dependencyLeaf {
		t.Fatal("rewriting changed the original core dependency", leaves)
	}
	for i := uint32(1); i < size; i++ {
		definitionDependency, err := goal.Dependency(i)
		simplifyMust(t, err)
		leaves, err = m.DependencyLinearize(definitionDependency, nil)
		simplifyMust(t, err)
		if len(leaves) != 0 {
			t.Fatal("fresh definition acquired a core dependency", i, leaves)
		}
	}

	md, err := model.NewModel(m)
	simplifyMust(t, err)
	simplifyMust(t, md.CoreField.RegisterConstant(freshDecl, one))
	var output tactic.ModelRef
	simplifyMust(t, output.Set(&md.CoreField))
	t.Cleanup(func() { simplifyMust(t, output.Close()) })
	simplifyMust(t, result.ModelConverter.ApplyModel(&output, 0))
	filtered, err := output.Get()
	simplifyMust(t, err)
	present, err := filtered.HasInterpretation(freshDecl)
	simplifyMust(t, err)
	if present {
		t.Fatal("fresh term-ITE declaration leaked into the model")
	}
}

func TestEliminateTermITELeavesBooleanITEAndRecoversAfterCleanup(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	runner, err := NewEliminateTermITETactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	condition, thenPart, elsePart := propagateAtoms(t, m)
	booleanITE, err := m.MkIte(condition, thenPart, elsePart)
	simplifyMust(t, err)
	simplifyPin(t, m, booleanITE)
	goal, err := tactic.NewGoal(m, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(booleanITE))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	got, err := goal.Form(0)
	simplifyMust(t, err)
	if got != booleanITE || runner.numFresh != 0 {
		t.Fatal("Boolean ITE was treated as a term ITE", got, runner.numFresh)
	}
	depth, err := goal.Depth()
	simplifyMust(t, err)
	if depth != 1 {
		t.Fatal("source depth increment missing", depth)
	}
	simplifyMust(t, runner.Cleanup())
	simplifyMust(t, runner.Apply(goal, &result))
	if runner.numFresh != 0 {
		t.Fatal("cleanup retained stale defined names", runner.numFresh)
	}
}
