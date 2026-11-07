package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTSequenceLengthCoherenceRequiresSolutionRoot(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	family, err := f.m.ArithmeticFamilyID()
	astTestMust(t, err)
	arithmetic, err := NewSMTLinearArithmeticTheory(family)
	astTestMust(t, err)
	astTestMust(t, f.state.RegisterTheory(arithmetic))
	length, err := f.m.MkSequenceLength(f.x)
	astTestMust(t, err)
	minimum, err := f.m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	lower, err := f.m.MkArithmeticGE(length, minimum)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(lower, true))
	variable, found := f.state.BoolVar(lower)
	if !found {
		t.Fatal("length bound was not internalized")
	}
	astTestMust(t, arithmetic.Assign(variable, true))
	astTestMust(t, arithmetic.Propagate())
	astTestMust(t, f.theory.Relevant(length))
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, f.state.PushScope())
		astTestMust(t, f.state.PushScope())
		astTestMust(t, f.theory.solutions.Update(f.x, f.y, nil))
		before := f.state.searchProgress()
		axioms := len(f.theory.axioms.values)
		progress, err := f.theory.addLengthLowerBoundDecomposition()
		astTestMust(t, err)
		if progress || f.state.searchProgress() != before || len(f.theory.axioms.values) != axioms {
			t.Fatal("already solved sequence acquired a lower-bound decomposition")
		}
		astTestMust(t, f.state.PopScopes(1))
		progress, err = f.theory.addLengthLowerBoundDecomposition()
		astTestMust(t, err)
		if !progress || len(f.theory.axioms.values) <= axioms {
			t.Fatal("restored solution root did not acquire a lower-bound decomposition")
		}
		astTestMust(t, f.state.PopScopes(1))
	}
}

func TestSMTSequenceFixedLengthSkolemEligibilityAndReplay(t *testing.T) {
	for _, name := range []string{"seq.tail", "seq.first", "seq.idx.left", "seq.idx.right", "seq.pre", "seq.post", "seq.left", "seq.right"} {
		t.Run(name, func(t *testing.T) {
			f := newSMTSequenceInterfaceFixture(t)
			zero := astTestInteger(t, f.m, 0)
			arguments := []ast.ASTHandle{f.x, zero}
			if name == "seq.first" {
				arguments = []ast.ASTHandle{f.x}
			}
			if name == "seq.idx.left" || name == "seq.idx.right" || name == "seq.left" || name == "seq.right" {
				arguments = []ast.ASTHandle{f.x, f.y}
			}
			symbol, err := f.m.Symbols.Intern(name)
			astTestMust(t, err)
			word, err := f.m.MkSequenceSkolem(symbol, arguments, f.sort)
			astTestMust(t, err)
			family, err := f.m.ArithmeticFamilyID()
			astTestMust(t, err)
			arithmetic, err := NewSMTLinearArithmeticTheory(family)
			astTestMust(t, err)
			astTestMust(t, f.state.RegisterTheory(arithmetic))
			length, err := f.m.MkSequenceLength(word)
			astTestMust(t, err)
			two := astTestInteger(t, f.m, 2)
			lower, err := f.m.MkArithmeticGE(length, two)
			astTestMust(t, err)
			upper, err := f.m.MkArithmeticLE(length, two)
			astTestMust(t, err)
			for _, bound := range []ast.ASTHandle{lower, upper} {
				astTestMust(t, f.state.Internalize(bound, true))
				variable, found := f.state.BoolVar(bound)
				if !found {
					t.Fatal("Skolem length bound was not internalized")
				}
				astTestMust(t, arithmetic.Assign(variable, true))
			}
			astTestMust(t, f.theory.Relevant(length))
			for attempt := 0; attempt < 2; attempt++ {
				astTestMust(t, f.state.PushScope())
				before := len(f.theory.axioms.values)
				progress, err := f.theory.addFixedLengthDecomposition()
				astTestMust(t, err)
				want := name == "seq.pre" || name == "seq.post" || name == "seq.left" || name == "seq.right"
				if progress != want || (len(f.theory.axioms.values) > before) != want {
					t.Fatal("fixed-length Skolem eligibility", progress, "want", want)
				}
				progress, err = f.theory.addFixedLengthDecomposition()
				astTestMust(t, err)
				if progress {
					t.Fatal("fixed-length decomposition repeated in the same scope")
				}
				astTestMust(t, f.state.PopScopes(1))
			}
		})
	}
}

func TestSMTSequenceLengthRelevanceFollowsMergedClassAndScope(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	lengthX, err := f.m.MkSequenceLength(f.x)
	astTestMust(t, err)
	astTestPin(t, f.m, lengthX)
	astTestMust(t, f.state.Internalize(lengthX, false))
	astTestMust(t, f.theory.Relevant(lengthX))
	lengthY, err := f.m.MkSequenceLength(f.y)
	astTestMust(t, err)
	astTestPin(t, f.m, lengthY)
	equality, err := f.m.MkEq(f.x, f.y)
	astTestMust(t, err)
	astTestPin(t, f.m, equality)
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, f.state.PushScope())
		astTestMust(t, f.state.Internalize(equality, false))
		literal, err := f.state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, f.state.assignDecisionLiteral(literal))
		consistent, err := f.state.PropagateCore()
		astTestMust(t, err)
		if !consistent {
			t.Fatal("fixture equality conflicted")
		}
		astTestMust(t, f.theory.Relevant(lengthX))
		if f.state.context.enodes.get(lengthY) == nil {
			t.Fatal("relevant length was not extended to its merged sequence class")
		}
		before := f.state.searchProgress()
		axioms := len(f.theory.axioms.values)
		astTestMust(t, f.theory.Relevant(lengthX))
		if f.state.searchProgress() != before || len(f.theory.axioms.values) != axioms {
			t.Fatal("repeated length relevance changed the search")
		}
		astTestMust(t, f.state.PopScopes(1))
		if f.state.context.enodes.get(lengthY) != nil || f.state.context.enodes.get(f.x).Root() == f.state.context.enodes.get(f.y).Root() {
			t.Fatal("child length application or class merge survived pop")
		}
	}
}

func TestSMTSequenceLengthTermsHaveNoSequenceVariables(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	left, err := f.m.MkSequenceLength(f.x)
	astTestMust(t, err)
	right, err := f.m.MkSequenceLength(f.y)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(left, false))
	astTestMust(t, f.state.Internalize(right, false))
	first := f.state.context.enodes.get(left).TheoryVar(f.theory.FamilyID())
	second := f.state.context.enodes.get(right).TheoryVar(f.theory.FamilyID())
	if first != SMTNullTheoryVar || second != SMTNullTheoryVar {
		t.Fatal("integer length terms acquired sequence theory variables")
	}
	if !f.theory.solutions.IsRoot(left) || !f.theory.solutions.IsRoot(right) {
		t.Fatal("integer-valued length terms entered the sequence solution map")
	}
}

func TestSMTSequenceRelationAtomHasNoSequenceVariable(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	contains, err := f.m.MkSequenceContains(f.x, f.y)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(contains, false))
	node := f.state.context.enodes.get(contains)
	variable, found := f.state.BoolVar(contains)
	if node == nil || !found || node.TheoryVar(f.theory.FamilyID()) != SMTNullTheoryVar {
		t.Fatal("Boolean relation internalization attached a sequence variable")
	}
	data, err := f.state.BooleanData(variable)
	astTestMust(t, err)
	if !data.IsTheoryAtom() {
		t.Fatal("Boolean relation lost assignment callbacks")
	}
}

func TestSMTSequenceNegativeContainsDoesNotExposePositiveWitness(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	contains, err := f.m.MkSequenceContains(f.x, f.y)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(contains, false))
	literal, err := f.state.Literal(contains)
	astTestMust(t, err)
	astTestMust(t, f.state.assignDecisionLiteral(literal.Negated()))
	consistent, err := f.state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("negative containment fixture conflicted")
	}
	leftName, err := f.m.Symbols.Intern("seq.idx.left")
	astTestMust(t, err)
	rightName, err := f.m.Symbols.Intern("seq.idx.right")
	astTestMust(t, err)
	arguments := []ast.ASTHandle{f.x, f.y}
	left, err := f.m.MkSequenceSkolem(leftName, arguments, f.sort)
	astTestMust(t, err)
	right, err := f.m.MkSequenceSkolem(rightName, arguments, f.sort)
	astTestMust(t, err)
	tail, err := f.m.MkSequenceConcat(f.y, right)
	astTestMust(t, err)
	word, err := f.m.MkSequenceConcat(left, tail)
	astTestMust(t, err)
	if f.theory.equalityLiteral(word, f.x) != SMTNullLiteral {
		t.Fatal("false containment exposed the positive witness equation")
	}
}

func TestSMTSequencePrefixDelaysTailUntilNonemptyAndEqualHeads(t *testing.T) {
	f := newSMTSequenceInterfaceFixture(t)
	prefix, err := f.m.MkSequencePrefix(f.x, f.y)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(prefix, false))
	literal, err := f.state.Literal(prefix)
	astTestMust(t, err)
	astTestMust(t, f.state.assignDecisionLiteral(literal.Negated()))
	consistent, err := f.state.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("negative prefix fixture conflicted")
	}
	empty, err := f.m.MkSequenceEmpty(f.sort)
	astTestMust(t, err)
	emptyLiteral := f.theory.equalityLiteral(f.y, empty)
	if emptyLiteral == SMTNullLiteral {
		t.Fatal("container-empty decision is missing")
	}
	data, err := f.state.BooleanData(emptyLiteral.Var())
	astTestMust(t, err)
	if !data.PhaseAvailable || !data.Phase {
		t.Fatal("empty container was not preferred")
	}
	zero := astTestInteger(t, f.m, 0)
	tailName, err := f.m.Symbols.Intern("seq.tail")
	astTestMust(t, err)
	tailX, err := f.m.MkSequenceSkolem(tailName, []ast.ASTHandle{f.x, zero}, f.sort)
	astTestMust(t, err)
	tailY, err := f.m.MkSequenceSkolem(tailName, []ast.ASTHandle{f.y, zero}, f.sort)
	astTestMust(t, err)
	tailPrefix, err := f.m.MkSequencePrefix(tailX, tailY)
	astTestMust(t, err)
	astTestPin(t, f.m, tailPrefix)
	if _, found := f.state.BoolVar(tailPrefix); found {
		t.Fatal("tail predicate was exposed before the empty decision")
	}
	for _, equalHeads := range []bool{false, true} {
		astTestMust(t, f.state.PushScope())
		astTestMust(t, f.state.assignDecisionLiteral(emptyLiteral.Negated()))
		consistent, err = f.state.PropagateCore()
		astTestMust(t, err)
		if !consistent {
			t.Fatal("nonempty container conflicted")
		}
		astTestMust(t, f.theory.addNegativePrefixStep(prefix))
		elementSort, _, err := f.m.SequenceSortElement(f.sort)
		astTestMust(t, err)
		headName, err := f.m.Symbols.Intern("seq.nth")
		astTestMust(t, err)
		headX, err := f.m.MkSequenceSkolem(headName, []ast.ASTHandle{f.x, zero}, elementSort)
		astTestMust(t, err)
		headY, err := f.m.MkSequenceSkolem(headName, []ast.ASTHandle{f.y, zero}, elementSort)
		astTestMust(t, err)
		headLiteral := f.theory.equalityLiteral(headX, headY)
		if headLiteral == SMTNullLiteral {
			t.Fatal("head comparison is missing")
		}
		data, err = f.state.BooleanData(headLiteral.Var())
		astTestMust(t, err)
		if !data.PhaseAvailable || data.Phase {
			t.Fatal("head mismatch was not preferred")
		}
		if _, found := f.state.BoolVar(tailPrefix); found {
			t.Fatal("tail predicate was exposed before head equality")
		}
		decision := headLiteral
		if !equalHeads {
			decision = decision.Negated()
		}
		astTestMust(t, f.state.assignDecisionLiteral(decision))
		consistent, err = f.state.PropagateCore()
		astTestMust(t, err)
		if !consistent {
			t.Fatal("head comparison conflicted")
		}
		astTestMust(t, f.theory.addNegativePrefixStep(prefix))
		consistent, err = f.state.PropagateCore()
		astTestMust(t, err)
		if !consistent {
			t.Fatal("prefix continuation conflicted")
		}
		variable, found := f.state.BoolVar(tailPrefix)
		if found != equalHeads {
			t.Fatal("tail predicate exposure did not follow the head decision", found, equalHeads)
		}
		if found {
			value, err := f.state.Assignment(NewSMTLiteral(variable, false))
			astTestMust(t, err)
			if value != lbool.False {
				t.Fatal("matching-head prefix did not propagate its negative tail", value)
			}
		}
		astTestMust(t, f.state.PopScopes(1))
		if _, found := f.state.BoolVar(tailPrefix); found {
			t.Fatal("child tail predicate survived pop")
		}
	}
	data, err = f.state.BooleanData(emptyLiteral.Var())
	astTestMust(t, err)
	data.PhaseAvailable = false
	before := f.state.searchProgress()
	astTestMust(t, f.theory.addNegativePrefixStep(prefix))
	if f.state.searchProgress() == before || !data.PhaseAvailable || !data.Phase {
		t.Fatal("reactivated empty phase did not reach the kernel progress gate")
	}
	before = f.state.searchProgress()
	astTestMust(t, f.theory.addNegativePrefixStep(prefix))
	if f.state.searchProgress() != before {
		t.Fatal("unchanged prefix phase reported repeated progress")
	}
}
