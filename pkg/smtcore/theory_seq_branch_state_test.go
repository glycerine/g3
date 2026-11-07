package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTSequenceBranchesOrdinaryVariableWords(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	z, err := f.m.MkConst(astTestSymbol(t, f.m, "ordinary-left-tail"), f.sort)
	astTestMust(t, err)
	w, err := f.m.MkConst(astTestSymbol(t, f.m, "ordinary-right-tail"), f.sort)
	astTestMust(t, err)
	progress, err := f.theory.branchVariablePrefixEquality(1, []ast.ASTHandle{f.x, z}, []ast.ASTHandle{f.y, w}, nil)
	astTestMust(t, err)
	empty, err := f.m.MkSequenceEmpty(f.sort)
	astTestMust(t, err)
	literal := f.theory.equalityLiteral(f.x, empty)
	if !progress || literal == SMTNullLiteral {
		t.Fatal("ordinary variable words did not propose the source empty-prefix split")
	}
	data, err := f.state.BooleanData(literal.Var())
	astTestMust(t, err)
	if !data.TryTrueFirst() || f.state.context.enodes.get(f.x).Root() == f.state.context.enodes.get(empty).Root() {
		t.Fatal("ordinary prefix split was not an unassigned preferred equality")
	}
}

func TestSMTSequencePrefixCursorAdvancesAndRestartsAfterPop(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	zero := astTestInteger(t, f.m, 0)
	integerSort, err := f.m.GetSort(zero)
	astTestMust(t, err)
	n, err := f.m.MkConst(astTestSymbol(t, f.m, "cursor-integer"), integerSort)
	astTestMust(t, err)
	prefix, err := f.m.MkIntToString(n)
	astTestMust(t, err)
	astTestPin(t, f.m, prefix)
	z, err := f.m.MkConst(astTestSymbol(t, f.m, "cursor-left-tail"), f.sort)
	astTestMust(t, err)
	astTestPin(t, f.m, z)
	w, err := f.m.MkConst(astTestSymbol(t, f.m, "cursor-right-tail"), f.sort)
	astTestMust(t, err)
	astTestPin(t, f.m, w)
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, f.state.PushScope())
		left, right := []ast.ASTHandle{f.x, z}, []ast.ASTHandle{prefix, w}
		progress, err := f.theory.branchVariablePrefixEquality(7, left, right, nil)
		astTestMust(t, err)
		empty, err := f.m.MkSequenceEmpty(f.sort)
		astTestMust(t, err)
		emptyLiteral := f.theory.equalityLiteral(f.x, empty)
		if !progress || emptyLiteral == SMTNullLiteral {
			t.Fatal("initial empty-prefix split is missing")
		}
		astTestMust(t, f.state.assignDecisionLiteral(emptyLiteral.Negated()))
		consistent, err := f.state.PropagateCore()
		astTestMust(t, err)
		if !consistent {
			t.Fatal("empty-prefix refusal conflicted")
		}
		progress, err = f.theory.branchVariablePrefixEquality(7, left, right, nil)
		astTestMust(t, err)
		prefixLiteral := f.theory.equalityLiteral(f.x, prefix)
		if !progress || prefixLiteral == SMTNullLiteral {
			t.Fatal("first nonempty prefix is missing after cursor rollback")
		}
		astTestMust(t, f.state.assignDecisionLiteral(prefixLiteral))
		consistent, err = f.state.PropagateCore()
		astTestMust(t, err)
		if !consistent {
			t.Fatal("first nonempty prefix conflicted")
		}
		progress, err = f.theory.branchVariablePrefixEquality(7, left, right, nil)
		astTestMust(t, err)
		word, err := f.m.MkSequenceConcat(prefix, w)
		astTestMust(t, err)
		fullLiteral := f.theory.equalityLiteral(f.x, word)
		if !progress || fullLiteral == SMTNullLiteral {
			t.Fatal("cursor repeated a merged prefix instead of advancing")
		}
		value, err := f.state.Assignment(fullLiteral)
		astTestMust(t, err)
		if value != lbool.Undef {
			t.Fatal("next prefix was asserted", value)
		}
		astTestMust(t, f.state.PopScopes(1))
	}
}

func TestSMTSequencePrefixExhaustionRetainsEarlierCanonicalGuards(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	a, err := f.m.BitVectorNumeralUnsigned('a', 8)
	astTestMust(t, err)
	b, err := f.m.BitVectorNumeralUnsigned('b', 8)
	astTestMust(t, err)
	unitA, err := f.m.MkSequenceUnit(a)
	astTestMust(t, err)
	astTestPin(t, f.m, unitA)
	unitB, err := f.m.MkSequenceUnit(b)
	astTestMust(t, err)
	astTestPin(t, f.m, unitB)
	x, err := f.m.MkConst(astTestSymbol(t, f.m, "exhaustion-later-variable"), f.sort)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(x, false))
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, f.state.PushScope())
		left, right := []ast.ASTHandle{x, unitB}, []ast.ASTHandle{unitA, unitB}
		progress, err := f.theory.branchVariablePrefixEquality(9, left, right, nil)
		astTestMust(t, err)
		canonical, err := f.m.MkEq(unitA, x)
		astTestMust(t, err)
		prefixLiteral, err := f.state.Literal(canonical)
		astTestMust(t, err)
		if !progress {
			t.Fatal("nonempty candidate was not proposed")
		}
		astTestMust(t, f.state.assignDecisionLiteral(prefixLiteral.Negated()))
		consistent, err := f.state.PropagateCore()
		astTestMust(t, err)
		if !consistent {
			t.Fatal("prefix refusal conflicted before exhaustion")
		}
		progress, err = f.theory.branchVariablePrefixEquality(9, left, right, nil)
		astTestMust(t, err)
		empty, err := f.m.MkSequenceEmpty(f.sort)
		astTestMust(t, err)
		emptyLiteral := f.theory.equalityLiteral(x, empty)
		if !progress || emptyLiteral == SMTNullLiteral || f.state.Inconsistent() {
			t.Fatal("exhaustion did not revisit its undefined empty choice")
		}
		data, err := f.state.BooleanData(emptyLiteral.Var())
		astTestMust(t, err)
		if !data.PhaseAvailable || !data.Phase {
			t.Fatal("exhaustion did not force the source empty phase")
		}
		astTestMust(t, f.state.assignDecisionLiteral(emptyLiteral.Negated()))
		consistent, err = f.state.PropagateCore()
		astTestMust(t, err)
		if !consistent {
			t.Fatal("empty refusal conflicted before exhaustion")
		}
		progress, err = f.theory.branchVariablePrefixEquality(9, left, right, nil)
		astTestMust(t, err)
		conflict, found := f.state.Conflict()
		owner, _ := conflict.Justification()
		theoryConflict, ok := owner.(*SMTTheoryConflictJustification)
		if !progress || !found || !ok || len(theoryConflict.Literals) != 2 || theoryConflict.Literals[0] != emptyLiteral.Negated() || theoryConflict.Literals[1] != prefixLiteral.Negated() {
			t.Fatal("prefix exhaustion lost an earlier canonical refusal", owner)
		}
		astTestMust(t, f.state.PopScopes(1))
		if f.state.Inconsistent() {
			t.Fatal("exhaustion conflict survived its scope")
		}
	}
}

func TestSMTSequenceCanonicalResidualGetsFreshPrefixSearch(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	zero := astTestInteger(t, f.m, 0)
	integerSort, err := f.m.GetSort(zero)
	astTestMust(t, err)
	n, err := f.m.MkConst(astTestSymbol(t, f.m, "residual-prefix-integer"), integerSort)
	astTestMust(t, err)
	prefix, err := f.m.MkIntToString(n)
	astTestMust(t, err)
	z, err := f.m.MkConst(astTestSymbol(t, f.m, "residual-prefix-left-tail"), f.sort)
	astTestMust(t, err)
	w, err := f.m.MkConst(astTestSymbol(t, f.m, "residual-prefix-right-tail"), f.sort)
	astTestMust(t, err)
	astTestMust(t, f.theory.equations.Push(0, []ast.ASTHandle{f.x, z}, []ast.ASTHandle{f.y, w}, nil))
	f.theory.nextEquation = 1
	f.theory.branchStart[0] = 2
	astTestMust(t, f.theory.solutions.Update(f.y, prefix, nil))
	empty, err := f.m.MkSequenceEmpty(f.sort)
	astTestMust(t, err)
	astTestPin(t, f.m, empty)
	for step := 0; step < 16 && f.theory.equalityLiteral(f.x, empty) == SMTNullLiteral; step++ {
		_, err = f.theory.solveWordEquations()
		astTestMust(t, err)
	}
	emptyLiteral := f.theory.equalityLiteral(f.x, empty)
	if emptyLiteral == SMTNullLiteral {
		t.Fatal("canonical residual did not propose its empty prefix")
	}
	astTestMust(t, f.state.assignDecisionLiteral(emptyLiteral.Negated()))
	consistent, err := f.state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("residual empty-prefix refusal conflicted")
	}
	for step := 0; step < 16 && f.theory.equalityLiteral(f.x, prefix) == SMTNullLiteral; step++ {
		_, err = f.theory.solveWordEquations()
		astTestMust(t, err)
	}
	if f.theory.equalityLiteral(f.x, prefix) == SMTNullLiteral {
		t.Fatal("old equation's cursor skipped the new residual prefix")
	}
}

func TestSMTSequencePrefixFeasibilityDoesNotConsumeEgraphEqualities(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	equality, err := f.m.MkEq(f.x, f.y)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(equality, false))
	literal, err := f.state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, f.state.assignDecisionLiteral(literal))
	consistent, err := f.state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("fixture equality conflicted")
	}
	character, err := f.m.BitVectorNumeralUnsigned('a', 8)
	astTestMust(t, err)
	unit, err := f.m.MkSequenceUnit(character)
	astTestMust(t, err)
	for _, reverse := range []bool{false, true} {
		left, right := []ast.ASTHandle{f.x}, []ast.ASTHandle{f.y, unit}
		if reverse {
			left, right = right, left
		}
		possible, err := f.theory.sequenceWordsCanBeEqual(left, right)
		astTestMust(t, err)
		if !possible {
			t.Fatal("context-free prefix feasibility consumed an unexplained egraph equality")
		}
	}
}
