package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTSequenceAssumeEqualityRewritesBeforeInternalizing(t *testing.T) {
	for _, tc := range []struct {
		name          string
		equal, proofs bool
	}{
		{name: "inconsistent"},
		{name: "inconsistent-proof", proofs: true},
		{name: "tautology", equal: true},
		{name: "tautology-proof", equal: true, proofs: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSMTSequenceInterfaceFixture(t)
			if tc.proofs {
				astTestMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
			}
			a := astTestString(t, f.m, "a")
			b := astTestString(t, f.m, "b")
			left, err := f.m.MkSequenceConcat(f.x, a)
			astTestMust(t, err)
			right, err := f.m.MkSequenceConcat(f.x, b)
			astTestMust(t, err)
			if tc.equal {
				empty, makeErr := f.m.MkSequenceEmpty(f.sort)
				astTestMust(t, makeErr)
				right, err = f.m.MkSequenceConcat(empty, left)
				astTestMust(t, err)
			}
			before := f.state.searchProgress()
			value, err := f.theory.assumeSequenceEquality(left, right)
			astTestMust(t, err)
			want := lbool.False
			if tc.equal {
				want = lbool.True
			}
			if value != want || f.state.searchProgress() != before || f.state.context.enodes.get(left) != nil || f.state.context.enodes.get(right) != nil {
				t.Fatal("rewritable equality changed the search", value, "want", want)
			}
		})
	}
}

func TestSMTSequenceAssumeEqualityExclusionAndScopedPhase(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	equality, err := f.m.MkEq(f.x, f.y)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(equality, false))
	variable, found := f.state.BoolVar(equality)
	if !found {
		t.Fatal("fixture equality is missing")
	}
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, f.state.PushScope())
		astTestMust(t, f.theory.exclusions.Update(f.x, f.y))
		before := f.state.searchProgress()
		value, err := f.theory.assumeSequenceEquality(f.y, f.x)
		astTestMust(t, err)
		if value != lbool.False || f.state.searchProgress() != before {
			t.Fatal("excluded equality acquired a case split", value)
		}
		astTestMust(t, f.state.PopScopes(1))
		astTestMust(t, f.state.PushScope())
		value, err = f.theory.assumeSequenceEquality(f.y, f.x)
		astTestMust(t, err)
		data, err := f.state.BooleanData(variable)
		astTestMust(t, err)
		if value != lbool.Undef || !data.TryTrueFirst() || !f.state.isRelevant(equality) || !f.state.isRelevant(f.x) || !f.state.isRelevant(f.y) {
			t.Fatal("accepted equality lacks canonical atom, relevance or preferred phase", value)
		}
		reversed, err := f.m.MkEq(f.y, f.x)
		astTestMust(t, err)
		if _, duplicate := f.state.BoolVar(reversed); duplicate {
			t.Fatal("reverse argument order created a second equality atom")
		}
		astTestMust(t, f.state.PopScopes(1))
		data, err = f.state.BooleanData(variable)
		astTestMust(t, err)
		if data.TryTrueFirst() {
			t.Fatal("existing atom retained its child-scope preferred phase")
		}
	}
}

func TestSMTSequenceAssumeEqualityRecognizesMergedRoots(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	equality, err := f.m.MkEq(f.x, f.y)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(equality, false))
	literal, err := f.state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, f.state.assignDecisionLiteral(literal))
	consistent, err := f.state.PropagateCore()
	astTestMust(t, err)
	if !consistent || f.state.context.enodes.get(f.x).Root() != f.state.context.enodes.get(f.y).Root() {
		t.Fatal("fixture roots did not merge")
	}
	before := f.state.searchProgress()
	value, err := f.theory.assumeSequenceEquality(f.y, f.x)
	astTestMust(t, err)
	if value != lbool.True || f.state.searchProgress() != before {
		t.Fatal("merged roots acquired another split", value)
	}
}

func TestSMTSequenceAssumeEqualityHonorsCoreDisequality(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, f.state.PushScope())
		astTestMust(t, f.state.context.SetDisequal(f.state.context.enodes.get(f.x), f.state.context.enodes.get(f.y), true))
		if f.theory.exclusions.Contains(f.x, f.y) {
			t.Fatal("fixture did not isolate the core disequality check")
		}
		before := f.state.searchProgress()
		value, err := f.theory.assumeSequenceEquality(f.y, f.x)
		astTestMust(t, err)
		if value != lbool.False || f.state.searchProgress() != before {
			t.Fatal("core-disequal roots acquired an interface equality", value)
		}
		astTestMust(t, f.state.PopScopes(1))
		if f.state.context.isDisequal(f.state.context.enodes.get(f.x), f.state.context.enodes.get(f.y)) {
			t.Fatal("core disequality survived its scope")
		}
	}
}

func TestSMTSequenceAssumeEqualityRewritesUnitElements(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	one := astTestInteger(t, f.m, 1)
	two := astTestInteger(t, f.m, 2)
	sum, err := f.m.MkArithmeticAdd(one, one)
	astTestMust(t, err)
	left, err := f.m.MkSequenceUnit(sum)
	astTestMust(t, err)
	right, err := f.m.MkSequenceUnit(two)
	astTestMust(t, err)
	before := f.state.searchProgress()
	value, err := f.theory.assumeSequenceEquality(left, right)
	astTestMust(t, err)
	if value != lbool.True || f.state.searchProgress() != before {
		t.Fatal("arithmetic element rewriting was omitted", value)
	}
}

func TestSMTSequenceAssumeEqualityDoesNotBorrowPreprocessorSubstitutions(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	rw := smtQuickTheoryRewriter(t, f.m)
	f.state.theoryRewriter = rw
	literal := astTestString(t, f.m, "substituted")
	substitution, err := f.m.NewExprSubstitution()
	astTestMust(t, err)
	defer func() {
		astTestMust(t, rw.SetSubstitution(nil))
		astTestMust(t, f.m.DelExprSubstitution(&substitution))
	}()
	astTestMust(t, f.m.ExprSubstitutionInsert(substitution, f.x, literal, ast.ASTHandle{}, ast.ASTDependency{}))
	astTestMust(t, rw.SetSubstitution(&substitution))
	value, err := f.theory.assumeSequenceEquality(f.x, literal)
	astTestMust(t, err)
	if value != lbool.Undef || f.state.context.enodes.get(f.x).Root() == f.state.context.enodes.get(literal).Root() {
		t.Fatal("preprocessor substitution was treated as an unconditional sequence equality", value)
	}
}
