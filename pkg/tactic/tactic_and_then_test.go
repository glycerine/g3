package tactic

// Source: src/tactic/tactical.cpp:104-271.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type countingSkipTactic struct {
	*primitiveTactic
	applications uint32
}

type converterSkipTactic struct {
	*primitiveTactic
	converter ModelConverter
}

type replacingGoalTactic struct{ *primitiveTactic }

func (t *replacingGoalTactic) Apply(g *Goal, out *TacticResult) error {
	replacement, err := NewGoalConfigCopy(g)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			_ = replacement.Close()
		}
	}()
	if err = g.CopyTo(replacement); err != nil {
		return err
	}
	if err = out.Prepare(g.manager); err != nil {
		return err
	}
	if err = out.TakeGoal(replacement); err != nil {
		return err
	}
	published = true
	return g.Close()
}

func (t *converterSkipTactic) Apply(g *Goal, out *TacticResult) error {
	if err := t.primitiveTactic.Apply(g, out); err != nil {
		return err
	}
	return out.SetModelConverter(t.converter)
}

func (t *countingSkipTactic) Apply(g *Goal, out *TacticResult) error {
	t.applications++
	return t.primitiveTactic.Apply(g, out)
}

func TestAndThenSingleGoalAndDecidedShortcut(t *testing.T) {
	m := astTestManager(t)
	first := &countingSkipTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)}
	second := &countingSkipTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)}
	sequence, err := NewAndThenTactic(first, second)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)

	nondecided := goalTestNew(t, m, false, false)
	p := goalTestAtom(t, m, "and-then-p")
	astTestMust(t, nondecided.Assert(p))
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(nondecided, out))
	if first.applications != 1 || second.applications != 1 || len(out.Goals) != 1 || out.Goals[0] != nondecided {
		t.Fatal("single-goal sequence", first.applications, second.applications, len(out.Goals))
	}
	astTestMust(t, out.Close())

	decided, err := NewGoal(m, false, true, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, decided.Close()) })
	out = &TacticResult{}
	astTestMust(t, sequence.Apply(decided, out))
	if first.applications != 2 || second.applications != 1 || len(out.Goals) != 1 || out.Goals[0] != decided {
		t.Fatal("decided shortcut", first.applications, second.applications, len(out.Goals))
	}
	astTestMust(t, out.Close())
}

func TestAndThenSnapshotsModesBeforeOwnedGoalReplacement(t *testing.T) {
	m := astTestManager(t)
	g, err := NewGoal(m, false, true, true)
	astTestMust(t, err)
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-replaced")))
	first := &replacingGoalTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)}
	second := &countingSkipTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)}
	sequence, err := NewAndThenTactic(first, second)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	if second.applications != 1 || len(out.Goals) != 1 || out.Goals[0] == g {
		t.Fatal("replacement was not composed", second.applications, out.Goals)
	}
	models, err := out.Goals[0].ModelsEnabled()
	astTestMust(t, err)
	cores, err := out.Goals[0].UnsatCoresEnabled()
	astTestMust(t, err)
	if !models || !cores {
		t.Fatal("replacement modes", models, cores)
	}
	astTestMust(t, out.Close())
}

func TestAndThenNarySourceOrder(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-order")))
	children := []*countingSkipTactic{
		{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
		{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
		{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
	}
	sequence, err := NewAndThenTactic(children[0], children[1], children[2])
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	for i, child := range children {
		if child.applications != 1 {
			t.Fatalf("child %d applications %d", i, child.applications)
		}
	}
	astTestMust(t, out.Close())
}

func TestAndThenComposesSingleGoalModelConverters(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-model")))
	calls := []string{}
	firstConverter := &modelConverterTestLeaf{name: "first", calls: &calls}
	secondConverter := &modelConverterTestLeaf{name: "second", calls: &calls}
	first := &converterSkipTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0), converter: firstConverter}
	second := &converterSkipTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0), converter: secondConverter}
	sequence, err := NewAndThenTactic(first, second)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	if _, ok := out.ModelConverter.(*tacticConcatModelConverter); !ok {
		t.Fatalf("converter composition %T", out.ModelConverter)
	}
	if firstConverter.refs != 1 || secondConverter.refs != 1 {
		t.Fatal("composite converter ownership", firstConverter.refs, secondConverter.refs)
	}
	astTestMust(t, out.Close())
	if !firstConverter.closed || !secondConverter.closed {
		t.Fatal("result did not release converter composition")
	}
}

func TestAndThenFansOutAcrossMultipleSubgoals(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-fanout")))
	first := &branchingTestTactic{newPrimitiveTactic(primitiveSkip, "", 0)}
	second := &countingSkipTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)}
	sequence, err := NewAndThenTactic(first, second)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	if second.applications != 2 || len(out.Goals) != 2 || out.Goals[0] != g || out.Goals[1] != g {
		t.Fatal("multi-subgoal fanout", second.applications, len(out.Goals))
	}
	astTestMust(t, out.Close())
}

func TestAndThenPartitionsMultipleSubgoalModelConverters(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-star-model")))
	calls := []string{}
	firstConverter := &modelConverterTestLeaf{name: "star-first", calls: &calls}
	secondConverter := &modelConverterTestLeaf{name: "star-second", calls: &calls}
	first := &converterBranchingTactic{
		branchingTestTactic: branchingTestTactic{newPrimitiveTactic(primitiveSkip, "", 0)},
		converter:           firstConverter,
	}
	second := &converterSkipTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0), converter: secondConverter}
	sequence, err := NewAndThenTactic(first, second)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	if _, ok := out.ModelConverter.(*tacticConcatStarModelConverter); !ok {
		t.Fatalf("star converter composition %T", out.ModelConverter)
	}
	if firstConverter.refs != 1 || secondConverter.refs != 2 {
		t.Fatal("star converter ownership", firstConverter.refs, secondConverter.refs)
	}
	astTestMust(t, out.Close())
	if !firstConverter.closed || !secondConverter.closed {
		t.Fatal("star result did not release converters")
	}
}

type converterBranchingTactic struct {
	branchingTestTactic
	converter ModelConverter
}

func (t *converterBranchingTactic) Apply(g *Goal, out *TacticResult) error {
	if err := t.branchingTestTactic.Apply(g, out); err != nil {
		return err
	}
	return out.SetModelConverter(t.converter)
}

type splitOwnedTactic struct {
	*primitiveTactic
	proofConverter ProofConverter
}

func (t *splitOwnedTactic) Apply(g *Goal, out *TacticResult) error {
	if err := out.prepare(g.manager); err != nil {
		return err
	}
	for i := 0; i < 2; i++ {
		copy, err := NewGoalConfigCopy(g)
		if err != nil {
			return err
		}
		if err = g.CopyTo(copy); err != nil {
			_ = copy.Close()
			return err
		}
		if err = out.TakeGoal(copy); err != nil {
			_ = copy.Close()
			return err
		}
	}
	return out.SetProofConverter(t.proofConverter)
}

type decideTactic struct {
	*primitiveTactic
	dependency ast.ASTDependency
	sat        bool
}

func (t *decideTactic) Apply(g *Goal, out *TacticResult) error {
	if t.sat {
		if err := g.ResetAll(); err != nil {
			return err
		}
	} else {
		falsity, err := g.manager.MkFalse()
		if err != nil {
			return err
		}
		if t.dependency.IsNull() {
			if err = g.Assert(falsity); err != nil {
				return err
			}
		} else if err = g.AssertWithProof(falsity, ast.ASTHandle{}, t.dependency); err != nil {
			return err
		}
	}
	if err := out.prepare(g.manager); err != nil {
		return err
	}
	return out.AddGoal(g)
}

func TestAndThenCollapsesAllUnsatSubgoals(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-all-unsat")))
	sequence, err := NewAndThenTactic(
		&splitOwnedTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
		&decideTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
	)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	if len(out.Goals) != 1 || out.Goals[0] != g || !g.IsDecidedUnsat() {
		t.Fatal("all-unsat collapse", len(out.Goals), g.IsDecidedUnsat())
	}
	astTestMust(t, out.Close())
}

func TestAndThenReturnsFirstDecidedSatSubgoal(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-sat")))
	sequence, err := NewAndThenTactic(
		&splitOwnedTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
		&decideTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0), sat: true},
	)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	if len(out.Goals) != 1 || !out.Goals[0].IsDecidedSat() || out.ModelConverter == nil {
		t.Fatal("decided SAT branch", len(out.Goals), out.ModelConverter)
	}
	astTestMust(t, out.Close())
}

func TestAndThenBuildsProofForAllUnsatSubgoals(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	g, err := NewGoal(m, true, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-proof")))
	firstProof := newSelectingProofConverter(t, m, 0)
	sequence, err := NewAndThenTactic(
		&splitOwnedTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0), proofConverter: firstProof},
		&decideTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
	)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	if len(out.Goals) != 1 || !g.IsDecidedUnsat() {
		t.Fatal("proof all-unsat collapse")
	}
	proof, err := g.Proof(0)
	astTestMust(t, err)
	if proof.IsNull() {
		t.Fatal("all-unsat collapse omitted proof")
	}
	astTestMust(t, out.Close())
}

func TestAndThenJoinsAllUnsatDependencies(t *testing.T) {
	m := astTestManager(t)
	g, err := NewGoal(m, false, false, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	label := goalTestAtom(t, m, "and-then-core-label")
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-core")))
	dependency, err := m.DependencyLeaf(label)
	astTestMust(t, err)
	sequence, err := NewAndThenTactic(
		&splitOwnedTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
		&decideTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0), dependency: dependency},
	)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	got, err := g.Dependency(0)
	astTestMust(t, err)
	contains, err := m.DependencyContains(got, label)
	astTestMust(t, err)
	if !contains {
		t.Fatal("all-unsat collapse omitted dependency")
	}
	astTestMust(t, out.Close())
}

// scriptedSplitSatTactic splits on its first application when split is set
// (after changing the goal, so repeat does not stop at a fixpoint), leaves the
// next goal undecided and decides every later goal sat.
type scriptedSplitSatTactic struct {
	*primitiveTactic
	split        bool
	marker       ast.ASTHandle
	applications uint32
}

func (t *scriptedSplitSatTactic) Apply(g *Goal, out *TacticResult) error {
	t.applications++
	if t.split && t.applications == 1 {
		if err := g.Assert(t.marker); err != nil {
			return err
		}
		return (&splitOwnedTactic{primitiveTactic: t.primitiveTactic}).Apply(g, out)
	}
	if (t.split && t.applications == 2) || (!t.split && t.applications == 1) {
		return t.primitiveTactic.Apply(g, out)
	}
	return (&decideTactic{primitiveTactic: t.primitiveTactic, sat: true}).Apply(g, out)
}

// AUDIT.md tactic_top_01-F1: C++ and_then_tactical appends the decided-sat
// goal to the undecided subgoals already in result.
func TestAndThenSatSubgoalKeepsEarlierUndecidedGoals(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(goalTestAtom(t, m, "and-then-sat-keeps")))
	sequence, err := NewAndThenTactic(
		&splitOwnedTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
		&scriptedSplitSatTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0)},
	)
	astTestMust(t, err)
	tacticOwnForTest(t, sequence)
	out := &TacticResult{}
	astTestMust(t, sequence.Apply(g, out))
	if len(out.Goals) != 2 || out.DecidedSat() || out.ModelConverter == nil {
		t.Fatal("sat branch dropped earlier goals", len(out.Goals), out.DecidedSat(), out.ModelConverter)
	}
	if out.Goals[0].IsDecided() || !out.Goals[1].IsDecidedSat() {
		t.Fatal("goal order", out.Goals[0].IsDecided(), out.Goals[1].IsDecidedSat())
	}
	astTestMust(t, out.Close())
}

// AUDIT.md tactic_top_01-F1: repeat_tactical shares the multi-subgoal path.
func TestRepeatSatSubgoalKeepsEarlierUndecidedGoals(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(goalTestAtom(t, m, "repeat-sat-keeps")))
	child := &scriptedSplitSatTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0), split: true, marker: goalTestAtom(t, m, "repeat-sat-marker")}
	repeat, err := NewRepeatTactic(child, ^uint32(0))
	astTestMust(t, err)
	tacticOwnForTest(t, repeat)
	out := &TacticResult{}
	astTestMust(t, repeat.Apply(g, out))
	if len(out.Goals) != 2 || out.DecidedSat() || out.ModelConverter == nil {
		t.Fatal("repeat sat branch dropped earlier goals", len(out.Goals), out.DecidedSat(), out.ModelConverter)
	}
	astTestMust(t, out.Close())
}
