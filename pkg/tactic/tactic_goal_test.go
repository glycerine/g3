package tactic

// Source: src/tactic/goal.cpp:26-30,111-190,240-329,498-631,656-707;
// src/tactic/goal.h:39-143.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func goalTestAtom(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	h := proofTestBool(t, m, name)
	astTestPin(t, m, h)
	return h
}
func goalTestApp(t *testing.T, m *ast.ASTManager, op int32, args ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, e := m.MkFamilyApp(0, op, nil, args)
	astTestMust(t, e)
	astTestPin(t, m, h)
	return h
}
func goalTestNew(t *testing.T, m *ast.ASTManager, proofs, cores bool) *Goal {
	t.Helper()
	g, e := NewGoal(m, proofs, true, cores)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	return g
}
func goalTestForms(t *testing.T, g *Goal, want ...ast.ASTHandle) {
	t.Helper()
	n, e := g.Size()
	astTestMust(t, e)
	if int(n) != len(want) {
		t.Fatalf("source formula count %d want %d", n, len(want))
	}
	for i, h := range want {
		got, e := g.Form(uint32(i))
		astTestMust(t, e)
		if got != h {
			t.Fatalf("source formula[%d] %v want %v", i, got, h)
		}
	}
}

func TestTacticGoalConfigCopyKeepsFlagsDepthPrecisionWithoutAssertions(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "config-copy-p")
	source := goalTestNew(t, m, false, true)
	astTestMust(t, source.Assert(p))
	astTestMust(t, source.SetDepth(7))
	astTestMust(t, source.SetPrecision(GoalUnder))
	copy, err := NewGoalConfigCopy(source)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, copy.Close()) })
	if copy == source || copy.manager != m || !copy.modelsEnabled || copy.proofsEnabled || !copy.coreEnabled || copy.depth != 7 || copy.precision != GoalUnder {
		t.Fatal("source configuration was not copied")
	}
	goalTestForms(t, copy)
	goalTestForms(t, source, p)
}

func TestTacticGoalDefaultAndFullCopyConstructors(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	defaultGoal, err := NewDefaultGoal(m, false, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, defaultGoal.Close()) })
	if !defaultGoal.proofsEnabled || defaultGoal.modelsEnabled || !defaultGoal.coreEnabled {
		t.Fatal("default constructor flags")
	}
	p := goalTestAtom(t, m, "full-copy-p")
	astTestMust(t, defaultGoal.Assert(p))
	astTestMust(t, defaultGoal.SetDepth(11))
	astTestMust(t, defaultGoal.SetPrecision(GoalOver))
	copy, err := NewGoalCopy(defaultGoal)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, copy.Close()) })
	goalTestForms(t, copy, p)
	if copy.manager != m || !copy.proofsEnabled || copy.modelsEnabled || !copy.coreEnabled || copy.depth != 11 || copy.precision != GoalOver {
		t.Fatal("full copy constructor configuration")
	}
	astTestMust(t, defaultGoal.ResetAll())
	goalTestForms(t, copy, p)
}

func TestTacticGoalAssertionSplittingAndSourceOrder(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	r := goalTestAtom(t, m, "r")
	np := goalTestApp(t, m, ast.ASTOpNot, p)
	input := goalTestApp(t, m, ast.ASTOpAnd, p, goalTestApp(t, m, ast.ASTOpNot, goalTestApp(t, m, ast.ASTOpOr, q, goalTestApp(t, m, ast.ASTOpNot, r))), p)
	nq := goalTestApp(t, m, ast.ASTOpNot, q)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(input))
	goalTestForms(t, g, p, nq, r, p)
	// A top-level double negation is not rewritten by quick_process's shortcut.
	double := goalTestApp(t, m, ast.ASTOpNot, np)
	astTestMust(t, g.Assert(double))
	goalTestForms(t, g, p, nq, r, p, double)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	astTestMust(t, g.Assert(truth))
	goalTestForms(t, g, p, nq, r, p, double)
}

func TestTacticGoalFalseReplacesAllAndResetAttributes(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(p))
	astTestMust(t, g.SetDepth(17))
	astTestMust(t, g.SetPrecision(GoalOver))
	f, e := m.MkFalse()
	astTestMust(t, e)
	astTestMust(t, g.Assert(f))
	astTestMust(t, g.Assert(p))
	goalTestForms(t, g, f)
	if !g.inconsistent || !g.IsDecidedUnsat() {
		t.Fatal("source false goal must decide UNSAT")
	}
	astTestMust(t, g.Reset())
	goalTestForms(t, g)
	if g.depth != 17 || g.precision != GoalOver || g.IsDecidedSat() {
		t.Fatal("reset must preserve over-approximation and depth")
	}
	astTestMust(t, g.ResetAll())
	if g.depth != 0 || g.precision != GoalPrecise || !g.IsDecidedSat() {
		t.Fatal("reset_all restores precise empty goal")
	}
}

func TestTacticGoalUpdateAndEliminateTrue(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	r := goalTestAtom(t, m, "r")
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(r))
	input := goalTestApp(t, m, ast.ASTOpNot, goalTestApp(t, m, ast.ASTOpOr, p, goalTestApp(t, m, ast.ASTOpNot, q)))
	astTestMust(t, g.Update(0, input, ast.ASTHandle{}, ast.ASTDependency{}))
	np := goalTestApp(t, m, ast.ASTOpNot, p)
	goalTestForms(t, g, np, q)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	astTestMust(t, g.Update(0, truth, ast.ASTHandle{}, ast.ASTDependency{}))
	goalTestForms(t, g, truth, q)
	astTestMust(t, g.ElimTrue())
	goalTestForms(t, g, q)
}

func TestTacticGoalRedundanciesProofAndDependency(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "positive-first", true: "negative-first"}[reverse], func(t *testing.T) {
			m := astTestManager(t)
			astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			p := goalTestAtom(t, m, "p")
			np := goalTestApp(t, m, ast.ASTOpNot, p)
			g := goalTestNew(t, m, true, true)
			a, b := p, np
			if reverse {
				a, b = b, a
			}
			da, e := m.DependencyLeaf(a)
			astTestMust(t, e)
			astTestMust(t, m.DependencyInc(da))
			defer func() { astTestMust(t, m.DependencyDec(da)) }()
			db, e := m.DependencyLeaf(b)
			astTestMust(t, e)
			astTestMust(t, m.DependencyInc(db))
			defer func() { astTestMust(t, m.DependencyDec(db)) }()
			pa, e := m.MkProofAsserted(a)
			astTestMust(t, e)
			astTestPin(t, m, pa)
			pb, e := m.MkProofAsserted(b)
			astTestMust(t, e)
			astTestPin(t, m, pb)
			astTestMust(t, g.AssertWithProof(a, pa, da))
			astTestMust(t, g.AssertWithProof(a, pa, da))
			astTestMust(t, g.AssertWithProof(b, pb, db))
			astTestMust(t, g.ElimRedundancies())
			f, e := m.MkFalse()
			astTestMust(t, e)
			goalTestForms(t, g, f)
			pr, e := g.Proof(0)
			astTestMust(t, e)
			proofTestFact(t, m, pr, f)
			dep, e := g.Dependency(0)
			astTestMust(t, e)
			v, e := m.DependencyLinearize(dep, nil)
			astTestMust(t, e)
			size := len(v)
			if size != 2 {
				t.Fatal("source contradiction core must join both premises", size)
			}
		})
	}
}

func TestTacticGoalProofAssertionSplitting(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	input := goalTestApp(t, m, ast.ASTOpAnd, p, goalTestApp(t, m, ast.ASTOpNot, goalTestApp(t, m, ast.ASTOpOr, q, goalTestApp(t, m, ast.ASTOpNot, p))))
	g := goalTestNew(t, m, true, false)
	astTestMust(t, g.Assert(input))
	nq := goalTestApp(t, m, ast.ASTOpNot, q)
	goalTestForms(t, g, p, nq, p)
	for i, h := range []ast.ASTHandle{p, nq, p} {
		pr, e := g.Proof(uint32(i))
		astTestMust(t, e)
		proofTestFact(t, m, pr, h)
	}
}

func TestTacticGoalCopyAndPrecisionAlgebra(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	g := goalTestNew(t, m, false, false)
	target := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(p))
	astTestMust(t, g.SetDepth(3))
	astTestMust(t, target.SetDepth(7))
	astTestMust(t, g.SetPrecision(GoalUnder))
	astTestMust(t, target.SetPrecision(GoalOver))
	astTestMust(t, g.CopyTo(target))
	goalTestForms(t, target, p)
	if target.depth != 7 || target.precision != GoalUnderOver {
		t.Fatal("source copy attributes", target.depth, target.precision)
	}
	astTestMust(t, g.Reset())
	goalTestForms(t, target, p)
	for a := GoalPrecise; a <= GoalUnderOver; a++ {
		for b := GoalPrecise; b <= GoalUnderOver; b++ {
			want := a
			if a == GoalPrecise {
				want = b
			} else if b != GoalPrecise && a != b {
				want = GoalUnderOver
			}
			if got := tacticGoalPrecisionUnion(a, b); got != want {
				t.Fatal(a, b, got, want)
			}
		}
	}
}

func TestTacticGoalsEqualSourceSetAndSizeSemantics(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "goal-equal-p")
	q := goalTestAtom(t, m, "goal-equal-q")
	first := goalTestNew(t, m, false, false)
	second := goalTestNew(t, m, false, false)
	astTestMust(t, first.Assert(p))
	astTestMust(t, first.Assert(q))
	astTestMust(t, second.Assert(q))
	astTestMust(t, second.Assert(p))
	equal, err := GoalsEqual(first, second)
	astTestMust(t, err)
	if !equal {
		t.Fatal("formula order changed source equality")
	}
	astTestMust(t, second.Assert(p))
	equal, err = GoalsEqual(first, second)
	astTestMust(t, err)
	if equal {
		t.Fatal("different formula counts compared equal")
	}
	third := goalTestNew(t, m, false, false)
	astTestMust(t, third.Assert(p))
	astTestMust(t, third.Assert(p))
	fourth := goalTestNew(t, m, false, false)
	astTestMust(t, fourth.Assert(p))
	astTestMust(t, fourth.Assert(q))
	equal, err = GoalsEqual(third, fourth)
	astTestMust(t, err)
	if equal {
		t.Fatal("different unique formula sets compared equal")
	}
}

func TestTacticGoalDuplicateEliminationAndRetainedReferences(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	g := goalTestNew(t, m, false, false)
	before, e := m.GetRefCount(p)
	astTestMust(t, e)
	for _, h := range []ast.ASTHandle{p, p, q, p} {
		astTestMust(t, g.Assert(h))
	}
	astTestMust(t, g.ElimRedundancies())
	goalTestForms(t, g, p, q)
	after, e := m.GetRefCount(p)
	astTestMust(t, e)
	if after != before+1 {
		t.Fatal("retained goal array must own only surviving formula", before, after)
	}
	astTestMust(t, g.Close())
	after, e = m.GetRefCount(p)
	astTestMust(t, e)
	if after != before {
		t.Fatal("goal close must release retained formula", before, after)
	}
}

func TestTacticGoalUpdateRetainsSavedFirstBeforeLaterFalse(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(q))
	f, e := m.MkFalse()
	astTestMust(t, e)
	input := goalTestApp(t, m, ast.ASTOpAnd, p, f, q)
	before, e := m.GetRefCount(p)
	astTestMust(t, e)
	astTestMust(t, g.Update(0, input, ast.ASTHandle{}, ast.ASTDependency{}))
	goalTestForms(t, g, f)
	after, e := m.GetRefCount(p)
	astTestMust(t, e)
	if after != before {
		t.Fatal("first replacement expr_ref must remain owned through later false", before, after)
	}
}

// goal::update without proofs flattens (and false p) to false. z3 4.7.1
// (goal.cpp:291-292) then pushes the unflattened f, which leaves the goal
// consistent with the stale formula still at the index; g3 pushes false
// (cmd/g3/test_vectors/z3_bug_goal_update_flattened_false.smt2).
func TestTacticGoalUpdateLiteralFalseHeadMakesGoalInconsistent(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(q))
	f, e := m.MkFalse()
	astTestMust(t, e)
	input := goalTestApp(t, m, ast.ASTOpAnd, f, p)
	astTestMust(t, g.Update(0, input, ast.ASTHandle{}, ast.ASTDependency{}))
	goalTestForms(t, g, f)
	if !g.inconsistent || !g.IsDecidedUnsat() {
		t.Fatal("update to a formula that flattens to false must decide UNSAT")
	}
}

func TestTacticGoalProofUpdateKeepsDerivedFacts(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	g := goalTestNew(t, m, true, false)
	astTestMust(t, g.Assert(q))
	input := goalTestApp(t, m, ast.ASTOpAnd, p, goalTestApp(t, m, ast.ASTOpNot, goalTestApp(t, m, ast.ASTOpOr, q, goalTestApp(t, m, ast.ASTOpNot, p))))
	pr, e := m.MkProofAsserted(input)
	astTestMust(t, e)
	astTestPin(t, m, pr)
	astTestMust(t, g.Update(0, input, pr, ast.ASTDependency{}))
	nq := goalTestApp(t, m, ast.ASTOpNot, q)
	goalTestForms(t, g, p, nq, p)
	for i, h := range []ast.ASTHandle{p, nq, p} {
		pr, e := g.Proof(uint32(i))
		astTestMust(t, e)
		proofTestFact(t, m, pr, h)
	}
}

func TestTacticGoalSourceDepthPrecisionCountsAndReferences(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "p")
	q := goalTestAtom(t, m, "q")
	f := goalTestApp(t, m, ast.ASTOpOr, p, q)
	g := goalTestNew(t, m, false, false)
	astTestMust(t, g.Assert(f))
	astTestMust(t, g.Assert(f))
	one, e := ast.NewASTFastMark1(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, one.Close()) }()
	two, e := ast.NewASTFastMark2(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, two.Close()) }()
	for _, h := range []ast.ASTHandle{p, q, f} {
		astTestMust(t, one.Mark(h, true))
		astTestMust(t, two.Mark(h, true))
	}
	count, e := g.NumExprs()
	astTestMust(t, e)
	if count != 3 {
		t.Fatal("source count must visit shared ASTs once", count)
	}
	astTestMust(t, g.ElimRedundancies())
	goalTestForms(t, g, f)
	for _, h := range []ast.ASTHandle{p, q, f} {
		a, e := one.IsMarked(h)
		astTestMust(t, e)
		b, e := two.IsMarked(h)
		astTestMust(t, e)
		if !a || !b {
			t.Fatal("goal operations changed another caller's marks")
		}
	}
	astTestMust(t, one.Close())
	astTestMust(t, two.Close())
	astTestMust(t, g.SetDepth((1<<26)-1))
	astTestMust(t, g.IncDepth())
	if g.depth != 0 {
		t.Fatal("source 26-bit depth wraps", g.depth)
	}
	for precision := GoalPrecise; precision <= GoalUnderOver; precision++ {
		astTestMust(t, g.Reset())
		astTestMust(t, g.SetPrecision(precision))
		if g.IsDecidedSat() != (precision == GoalPrecise || precision == GoalUnder) {
			t.Fatal("SAT preservation", precision)
		}
		f, e := m.MkFalse()
		astTestMust(t, e)
		astTestMust(t, g.Assert(f))
		if g.IsDecidedUnsat() != (precision == GoalPrecise || precision == GoalOver) {
			t.Fatal("UNSAT preservation", precision)
		}
	}
	astTestMust(t, g.IncRef())
	astTestMust(t, g.IncRef())
	consumed, e := g.DecRefConsumed()
	astTestMust(t, e)
	if !consumed || g.RefCount != 1 || g.Closed {
		t.Fatal("source shared goal reference was not consumed", consumed, g.RefCount, g.Closed)
	}
	consumed, e = g.DecRefConsumed()
	astTestMust(t, e)
	if !consumed {
		t.Fatal("source final goal reference was not consumed")
	}
	if _, e = g.Size(); e == nil {
		t.Fatal("source final reference must retire goal")
	}
}
