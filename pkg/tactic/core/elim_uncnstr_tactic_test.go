package core

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/statistics"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestEliminateUnconstrainedProofCoreAndIndependentModel(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	runner, err := NewEliminateUnconstrainedTactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	a, b, _ := propagateAtoms(t, m)
	notA, err := m.MkNot(a)
	simplifyMust(t, err)
	simplifyPin(t, m, notA)
	dep, err := m.DependencyLeaf(b)
	simplifyMust(t, err)
	proof, err := m.MkProofAsserted(notA)
	simplifyMust(t, err)
	goal, err := tactic.NewGoal(m, true, true, true)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.AssertWithProof(notA, proof, dep))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	if runner.eliminated != 1 || result.ModelConverter == nil {
		t.Fatal("missing elimination/converter", runner.eliminated)
	}
	fresh, err := goal.Form(0)
	simplifyMust(t, err)
	freshDecl, err := m.GetDecl(fresh)
	simplifyMust(t, err)
	simplifyMust(t, runner.Cleanup())
	if runner.eliminated != 1 {
		t.Fatal("cleanup lost elimination statistics")
	}
	simplifyMust(t, runner.Close())
	proof, err = goal.Proof(0)
	simplifyMust(t, err)
	fact, err := m.GetProofFact(proof)
	simplifyMust(t, err)
	if fact != fresh {
		t.Fatal("definition proof did not conclude the rewritten assertion")
	}
	dep, err = goal.Dependency(0)
	simplifyMust(t, err)
	leaves, err := m.DependencyLinearize(dep, nil)
	simplifyMust(t, err)
	if len(leaves) != 1 || leaves[0] != b {
		t.Fatal("definition introduction changed the core dependency", leaves)
	}
	md, err := model.NewModel(m)
	simplifyMust(t, err)
	var resultModel tactic.ModelRef
	simplifyMust(t, resultModel.Set(&md.CoreField))
	t.Cleanup(func() { simplifyMust(t, resultModel.Close()) })
	truth, err := m.MkTrue()
	simplifyMust(t, err)
	simplifyMust(t, md.CoreField.RegisterConstant(freshDecl, truth))
	simplifyMust(t, result.ModelConverter.ApplyModel(&resultModel, 0))
	converted, err := resultModel.Get()
	simplifyMust(t, err)
	decl, err := m.GetDecl(a)
	simplifyMust(t, err)
	value, err := converted.ConstantInterpretation(decl)
	simplifyMust(t, err)
	if !m.IsFalse(value) {
		t.Fatal("not-a model was not reconstructed", value)
	}
	retained, err := converted.HasInterpretation(freshDecl)
	simplifyMust(t, err)
	if retained {
		t.Fatal("fresh abstraction leaked through filter converter")
	}
}

func TestEliminateUnconstrainedDepthStatisticsAndRecovery(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	p, err := c.NewValue()
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, c.Del(&p)) })
	runner, err := NewEliminateUnconstrainedTactic(m, c, g, nil, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, runner.Close()) })
	if runner.maxMemory != ^uint64(0) {
		t.Fatal("default max_memory must retain source unlimited sentinel")
	}
	a, _, _ := propagateAtoms(t, m)
	notA, err := m.MkNot(a)
	simplifyMust(t, err)
	simplifyPin(t, m, notA)
	goal, err := tactic.NewGoal(m, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, runner.Apply(goal, &result))
	depth, err := goal.Depth()
	simplifyMust(t, err)
	if depth != 0 {
		t.Fatal("no-variable goal changed depth")
	}
	simplifyMust(t, goal.Assert(notA))
	key := "max_steps"
	simplifyMust(t, c.SetUIntName(&p, &key, 0))
	simplifyMust(t, runner.UpdateParams(&p))
	err = runner.Apply(goal, &result)
	if !rewriter.IsRewriterException(err) || err.Error() != "max. steps exceeded" {
		t.Fatal("source step-limit exception", err)
	}
	simplifyMust(t, runner.Cleanup())
	simplifyMust(t, c.SetUIntName(&p, &key, ^uint32(0)))
	simplifyMust(t, runner.UpdateParams(&p))
	simplifyMust(t, runner.Apply(goal, &result))
	if runner.eliminated != 1 {
		t.Fatal("step-limit cleanup failed to recover")
	}
	depth, err = goal.Depth()
	simplifyMust(t, err)
	if depth != 1 {
		t.Fatal("elimination depth", depth)
	}
	simplifyMust(t, runner.ResetStatistics())
	if len(statistics.Sums(runner.CollectStatistics)) != 0 {
		t.Fatal("statistics reset")
	}
	copy := *runner
	if err := copy.Close(); err == nil {
		t.Fatal("copied owner accepted")
	}
}

func TestEliminateUnconstrainedMemoryGuardAndParameterTranslation(t *testing.T) {
	m, c, g, services := solveEqFixture(t)
	p, err := c.NewValue()
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, c.Del(&p)) })
	key := "max_memory"
	simplifyMust(t, c.SetUIntName(&p, &key, 0))
	source, err := NewEliminateUnconstrainedTactic(m, c, g, &p, services)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, source.Close()) })
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, target.Close()) })
	translated, err := source.Translate(target)
	simplifyMust(t, err)
	simplifyMust(t, translated.IncRef())
	t.Cleanup(func() { simplifyMust(t, translated.DecRef()) })
	simplifyMust(t, c.SetUIntName(&p, &key, ^uint32(0)))
	simplifyMust(t, source.UpdateParams(&p))
	simplifyMust(t, source.Close())
	a, _, _ := propagateAtoms(t, target)
	term, err := target.MkNot(a)
	simplifyMust(t, err)
	simplifyPin(t, target, term)
	goal, err := tactic.NewGoal(target, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	simplifyMust(t, goal.Assert(term))
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	err = translated.Apply(goal, &result)
	var exception tactic.TacticException
	if !errors.As(err, &exception) || err.Error() != tactic.TacticMaxMemoryMessage {
		t.Fatal("translated memory policy or source exception type lost", err)
	}
	simplifyMust(t, translated.UpdateParams(&p))
	// Re-entry retires an interrupted traversal before replacing its owners.
	simplifyMust(t, translated.Apply(goal, &result))
	if statistics.Sums(translated.CollectStatistics)["eliminated applications"] != 1 {
		t.Fatal("memory-limit recovery failed")
	}
}

func TestEliminateUnconstrainedBVProofGuardAndCPPApply(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			m, c, g, services := solveEqFixture(t)
			if proofs {
				simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			runner, err := NewEliminateUnconstrainedTactic(m, c, g, nil, services)
			simplifyMust(t, err)
			t.Cleanup(func() { simplifyMust(t, runner.Close()) })
			sort, err := m.BitVectorSort(4)
			simplifyMust(t, err)
			name, err := m.Symbols.Intern("x")
			simplifyMust(t, err)
			x, err := m.MkConst(name, sort)
			simplifyMust(t, err)
			simplifyPin(t, m, x)
			zero, err := m.BitVectorNumeralUint64(0, 4)
			simplifyMust(t, err)
			simplifyPin(t, m, zero)
			formula, err := m.MkFamilyApp(runner.bitVector, 22, nil, []ast.ASTHandle{x, zero})
			simplifyMust(t, err)
			simplifyPin(t, m, formula)
			goal, err := tactic.NewGoal(m, proofs, true, false)
			simplifyMust(t, err)
			t.Cleanup(func() { simplifyMust(t, goal.Close()) })
			if proofs {
				pr, e := m.MkProofAsserted(formula)
				simplifyMust(t, e)
				simplifyMust(t, goal.AssertWithProof(formula, pr, ast.ASTDependency{}))
			} else {
				simplifyMust(t, goal.Assert(formula))
			}
			var result tactic.TacticResult
			t.Cleanup(func() { simplifyMust(t, result.Close()) })
			simplifyMust(t, runner.Apply(goal, &result))
			if proofs && (runner.eliminated != 0 || result.ModelConverter != nil) {
				t.Fatal("bit-vector inequality eliminated despite missing source proof rule")
			}
			if !proofs && runner.eliminated != 1 {
				t.Fatal("bit-vector inequality not abstracted")
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				cmd := exec.Command(reference, "-in")
				cmd.Stdin = strings.NewReader(fmt.Sprintf("(set-option :produce-proofs %v)(set-option :produce-models true)(set-logic QF_BV)(declare-const x (_ BitVec 4))(assert (bvule x #x0))(apply elim-uncnstr)", proofs))
				native, e := cmd.CombinedOutput()
				if e != nil {
					t.Fatal(e, string(native))
				}
				var output bytes.Buffer
				output.WriteString("(goals (goal ")
				size, e := goal.Size()
				simplifyMust(t, e)
				for i := uint32(0); i < size; i++ {
					term, e := goal.Form(i)
					simplifyMust(t, e)
					simplifyMust(t, runner.writeExpression(&output, term, false))
					output.WriteByte(' ')
				}
				depth, e := goal.Depth()
				simplifyMust(t, e)
				fmt.Fprintf(&output, ":precision precise :depth %d))", depth)
				tokens := func(s string) string {
					return strings.Join(strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(s)), " ")
				}
				if tokens(output.String()) != tokens(string(native)) {
					t.Fatalf("native apply differs\nGo: %s\nC++: %s", output.String(), native)
				}
			}
		})
	}
}
