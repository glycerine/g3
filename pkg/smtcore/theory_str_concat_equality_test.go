package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSMTStringConcatEqualityCoreCancellationAndRollback(t *testing.T) {
	for _, suffix := range []bool{false, true} {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "concat-pair-shared")
		y := stringContainmentWord(t, theory, "concat-pair-left")
		z := stringContainmentWord(t, theory, "concat-pair-right")
		left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, x, z)
		if suffix {
			left, right = stringEqualityConcat(t, m, y, x), stringEqualityConcat(t, m, z, x)
		}
		astTestMust(t, state.Internalize(left, false))
		astTestMust(t, state.Internalize(right, false))
		astTestMust(t, state.PushScope())
		equality, err := m.MkEq(left, right)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(equality, true))
		literal, err := state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		if state.Inconsistent() || state.context.enodes.get(y).Root() != state.context.enodes.get(z).Root() {
			t.Fatal("concat pair did not cancel its shared operand through the core", suffix)
		}
		astTestMust(t, state.PopScopes(1))
		if state.context.enodes.get(y).Root() == state.context.enodes.get(z).Root() {
			t.Fatal("concat cancellation survived scope rollback", suffix)
		}
	}
}

func stringConcatPairHasAxiom(t *testing.T, theory *SMTStringTheory, start int, want ast.ASTHandle) {
	t.Helper()
	for _, ref := range theory.refs[start:] {
		value, err := theory.state.manager.RefGet(ref)
		astTestMust(t, err)
		if value == want {
			return
		}
	}
	t.Fatal("native concat-pair guarded axiom missing")
}

func TestSMTStringConcatEqualityClassCancellationGuards(t *testing.T) {
	for _, suffix := range []bool{false, true} {
		m, config := smtModelGeneratorFixture(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "class-cancel-x")
		y := stringContainmentWord(t, theory, "class-cancel-y")
		a := stringContainmentWord(t, theory, "class-cancel-a")
		b := stringContainmentWord(t, theory, "class-cancel-b")
		left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
		for _, expression := range []ast.ASTHandle{left, right} {
			astTestMust(t, state.Internalize(expression, false))
		}
		first, second, remaining, other := x, a, y, b
		if suffix {
			first, second, remaining, other = y, b, x, a
		}
		// Private equality is deliberately not a core equality. The lemma
		// must retain its premise instead of treating that equality as true.
		theory.find.Merge(uint32(state.context.enodes.get(first).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(second).TheoryVar(theory.FamilyID())))
		start := len(theory.refs)
		astTestMust(t, theory.simplifyStringConcatEquality(left, right))
		parentEq, err := m.MkEq(left, right)
		astTestMust(t, err)
		astTestPin(t, m, parentEq)
		classEq, err := m.MkEq(first, second)
		astTestMust(t, err)
		astTestPin(t, m, classEq)
		resultEq, err := m.MkEq(remaining, other)
		astTestMust(t, err)
		astTestPin(t, m, resultEq)
		l, err := m.MkSequenceLength(remaining)
		astTestMust(t, err)
		astTestPin(t, m, l)
		r, err := m.MkSequenceLength(other)
		astTestMust(t, err)
		lengthEq, err := m.MkEq(l, r)
		astTestMust(t, err)
		astTestPin(t, m, lengthEq)
		conclusion, err := m.MkAnd(resultEq, lengthEq)
		astTestMust(t, err)
		astTestPin(t, m, conclusion)
		guard, err := m.MkAnd(parentEq, classEq)
		astTestMust(t, err)
		astTestPin(t, m, guard)
		negative, err := m.MkNot(guard)
		astTestMust(t, err)
		astTestPin(t, m, negative)
		want, err := m.MkOr(negative, conclusion)
		astTestMust(t, err)
		astTestPin(t, m, want)
		stringConcatPairHasAxiom(t, theory, start, want)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for bits := 0; bits < 16; bits++ {
			values := make(map[ast.ASTHandle]ast.ASTHandle)
			for i, expression := range []ast.ASTHandle{parentEq, classEq, resultEq, lengthEq} {
				value, err := m.MkBoolVal(bits&(1<<i) != 0)
				astTestMust(t, err)
				values[expression] = value
			}
			if got := stringAxiomTruth(t, m, rw, want, values); got != (bits&1 == 0 || bits&2 == 0 || bits&12 == 12) {
				t.Fatal("class cancellation dropped an equality premise or conclusion", suffix, bits)
			}
		}
		astTestMust(t, rw.Close())
		// With both sides already equivalent, the quick path is a no-op.
		theory.find.Merge(uint32(state.context.enodes.get(remaining).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(other).TheoryVar(theory.FamilyID())))
		start = len(theory.refs)
		astTestMust(t, theory.simplifyStringConcatEquality(left, right))
		if len(theory.refs) != start {
			t.Fatal("already equivalent concat operands emitted another cancellation")
		}
	}
}

func TestSMTStringConcatEqualityKnownLengthGuards(t *testing.T) {
	for _, suffix := range []bool{false, true} {
		m, config := smtModelGeneratorFixture(t)
		theory, state, _ := stringMixedEqualityFixture(t, m)
		x := stringContainmentWord(t, theory, "equal-len-x")
		y := stringContainmentWord(t, theory, "equal-len-y")
		a := stringContainmentWord(t, theory, "equal-len-a")
		b := stringContainmentWord(t, theory, "equal-len-b")
		left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
		first, second := x, a
		if suffix {
			first, second = y, b
		}
		l := stringSetLength(t, m, state, first, "2")
		r := stringSetLength(t, m, state, second, "2")
		start := len(theory.refs)
		astTestMust(t, theory.simplifyStringConcatEquality(left, right))
		parentEq, err := m.MkEq(left, right)
		astTestMust(t, err)
		astTestPin(t, m, parentEq)
		lengthEq, err := m.MkEq(l, r)
		astTestMust(t, err)
		astTestPin(t, m, lengthEq)
		xa, err := m.MkEq(x, a)
		astTestMust(t, err)
		astTestPin(t, m, xa)
		yb, err := m.MkEq(y, b)
		astTestMust(t, err)
		astTestPin(t, m, yb)
		guard, err := m.MkAnd(parentEq, lengthEq)
		astTestMust(t, err)
		astTestPin(t, m, guard)
		negative, err := m.MkNot(guard)
		astTestMust(t, err)
		astTestPin(t, m, negative)
		conclusion, err := m.MkAnd(xa, yb)
		astTestMust(t, err)
		astTestPin(t, m, conclusion)
		want, err := m.MkOr(negative, conclusion)
		astTestMust(t, err)
		astTestPin(t, m, want)
		stringConcatPairHasAxiom(t, theory, start, want)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		words := []string{"", "a", "b", "ab", "ba", "\x00\xff"}
		for _, xv := range words {
			for _, yv := range words {
				for _, av := range words {
					for _, bv := range words {
						values := map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, xv), y: astTestString(t, m, yv), a: astTestString(t, m, av), b: astTestString(t, m, bv)}
						if !stringAxiomTruth(t, m, rw, want, values) {
							t.Fatal("equal-length cancellation excluded a concrete byte valuation", suffix, xv, yv, av, bv)
						}
					}
				}
			}
		}
		astTestMust(t, rw.Close())
	}
}

func TestSMTStringConcatArgumentInferenceGuardsAndRefusals(t *testing.T) {
	for _, suffix := range []bool{false, true} {
		m := astTestManager(t)
		theory, state, _ := stringMixedEqualityFixture(t, m)
		x := stringContainmentWord(t, theory, "infer-child-x")
		y := stringContainmentWord(t, theory, "infer-child-y")
		concat := stringEqualityConcat(t, m, x, y)
		known, missing := x, y
		if suffix {
			known, missing = y, x
		}
		stringSetLength(t, m, state, known, "2")
		start := len(theory.refs)
		for _, number := range []int64{-1, 1} {
			astTestMust(t, theory.inferStringConcatArgument(concat, big.NewInt(number)))
		}
		if len(theory.refs) != start {
			t.Fatal("negative total/remainder emitted a length assertion", suffix)
		}
		astTestMust(t, theory.inferStringConcatArgument(concat, big.NewInt(5)))
		premises := smtStringLengthPremises{theory: theory}
		astTestMust(t, premises.length(concat, big.NewInt(5)))
		astTestMust(t, premises.length(known, big.NewInt(2)))
		conclusion := smtStringLengthPremises{theory: theory}
		astTestMust(t, conclusion.length(missing, big.NewInt(3)))
		guard, err := m.MkAndArray(premises.values)
		astTestMust(t, err)
		astTestPin(t, m, guard)
		negative, err := m.MkNot(guard)
		astTestMust(t, err)
		astTestPin(t, m, negative)
		want, err := m.MkOr(negative, conclusion.values[0])
		astTestMust(t, err)
		astTestPin(t, m, want)
		stringConcatPairHasAxiom(t, theory, start, want)
		astTestMust(t, premises.close())
		astTestMust(t, conclusion.close())
	}
	// A known total from the opposite concat is used in either orientation.
	for _, reverse := range []bool{false, true} {
		m := astTestManager(t)
		theory, state, _ := stringMixedEqualityFixture(t, m)
		x := stringContainmentWord(t, theory, "infer-equality-x")
		y := stringContainmentWord(t, theory, "infer-equality-y")
		left := stringEqualityConcat(t, m, x, y)
		right := stringEqualityConcat(t, m, astTestString(t, m, "ab"), astTestString(t, m, "cde"))
		stringSetLength(t, m, state, x, "2")
		start := len(theory.refs)
		if reverse {
			left, right = right, left
		}
		astTestMust(t, theory.inferStringConcatEquality(left, right))
		if len(theory.refs) != start+1 {
			t.Fatal("opposite concat total did not infer one missing child", reverse)
		}
		stringSetLength(t, m, state, y, "3")
		start = len(theory.refs)
		astTestMust(t, theory.inferStringConcatEquality(left, right))
		if len(theory.refs) != start {
			t.Fatal("fully known concat lengths emitted redundant inference", reverse)
		}
	}
}
