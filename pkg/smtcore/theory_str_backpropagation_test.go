package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func stringBackpropMixedFixture(t *testing.T) (*ast.ASTManager, *SMTStringTheory, *SMTInternalizerState, *rewriter.TheoryRewriter) {
	t.Helper()
	m, config := smtModelGeneratorFixture(t)
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	theory, state, _ := stringMixedEqualityFixture(t, m)
	state.theoryRewriter = rw
	return m, theory, state, rw
}

func TestSMTStringFinalCheckBackpropagatesConstantBytesAndGuards(t *testing.T) {
	for _, literalLeft := range []bool{false, true} {
		for _, literalRight := range []bool{false, true} {
			m, theory, state, rw := stringLibraryFixture(t)
			word := stringGroundingWord(t, theory, "backprop-word")
			leftValue, rightValue := astTestString(t, m, "a\x00"), astTestString(t, m, "\xffb")
			astTestMust(t, state.Internalize(leftValue, false))
			astTestMust(t, state.Internalize(rightValue, false))
			left, right := leftValue, rightValue
			if !literalLeft {
				left = stringGroundingWord(t, theory, "backprop-left")
				stringConversionCoreEquality(t, state, left, leftValue)
			}
			if !literalRight {
				right = stringGroundingWord(t, theory, "backprop-right")
				stringConversionCoreEquality(t, state, right, rightValue)
			}
			concat := stringDependencyConcat(t, theory, left, right)
			stringDependencyEquality(t, theory, word, concat, true)
			stringDependencyPrivateMerge(t, theory, word, concat)
			status, err := theory.FinalCheck()
			astTestMust(t, err)
			if status != SMTFinalCheckContinue {
				t.Fatal("actual final-check prefix did not continue after constant backpropagation", status)
			}
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			var guards []ast.ASTHandle
			for _, pair := range [][2]ast.ASTHandle{{left, leftValue}, {right, rightValue}} {
				if pair[0] != pair[1] {
					guard, err := theory.state.MkEqAtom(pair[0], pair[1])
					astTestMust(t, err)
					guards = append(guards, guard)
				}
			}
			for mask := 0; mask < 1<<len(guards); mask++ {
				for _, text := range []string{"a\x00\xffb", "wrong"} {
					values := map[ast.ASTHandle]ast.ASTHandle{concat: astTestString(t, m, text)}
					for i, guard := range guards {
						bit, err := m.MkBoolVal(mask&(1<<i) != 0)
						astTestMust(t, err)
						values[guard] = bit
					}
					want := mask != (1<<len(guards))-1 || text == "a\x00\xffb"
					if truth := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); truth != want {
						t.Fatal("constant backpropagation changed bytes or omitted/added premises", literalLeft, literalRight, mask, text, truth)
					}
				}
			}
			_, err = state.PropagateCore()
			astTestMust(t, err)
			value, known, err := theory.eqcStringValue(word)
			astTestMust(t, err)
			text, literal, err := m.SequenceStringValue(value)
			astTestMust(t, err)
			if !known || !literal || text != "a\x00\xffb" {
				t.Fatal("final-check constant consequence did not reach actual private String classes")
			}
			status, err = theory.FinalCheck()
			astTestMust(t, err)
			if status != SMTFinalCheckDone {
				t.Fatal("fully assigned constant classes did not reach native completion", status)
			}
		}
	}
}

func TestSMTStringLengthBackpropagationPreservesShadowedNativeConclusion(t *testing.T) {
	m, theory, state, rw := stringBackpropMixedFixture(t)
	x := stringGroundingWord(t, theory, "backprop-length-x")
	y := stringGroundingWord(t, theory, "backprop-length-y")
	yLength := stringSetLength(t, m, state, y, "7")
	stringDependencyPrivateMerge(t, theory, x, y)
	added, err := theory.propagateStringLengthWithinClass(x)
	astTestMust(t, err)
	if !added {
		t.Fatal("known private member length did not produce fallback implication")
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	xLength, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	guard, err := m.MkEq(x, y)
	astTestMust(t, err)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	astTestMust(t, err)
	lengthGuard, err := m.MkEq(yLength, seven)
	astTestMust(t, err)
	for mask := 0; mask < 4; mask++ {
		for _, length := range []string{"0", "1", "7"} {
			number, err := m.ArithmeticIntegerNumeralText(length)
			astTestMust(t, err)
			first, err := m.MkBoolVal(mask&1 != 0)
			astTestMust(t, err)
			second, err := m.MkBoolVal(mask&2 != 0)
			astTestMust(t, err)
			values := map[ast.ASTHandle]ast.ASTHandle{guard: first, lengthGuard: second, xLength: number}
			if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); got != (mask != 3 || length == "1") {
				t.Fatal("retained obj_ref-to-bool shadow behavior or fallback premises changed", mask, length, got)
			}
		}
	}
}

func TestSMTStringFinalCheckLengthSumsDuplicateLeavesAndGuards(t *testing.T) {
	m, theory, state, rw := stringBackpropMixedFixture(t)
	x := stringGroundingWord(t, theory, "backprop-sum-x")
	word := stringGroundingWord(t, theory, "backprop-sum-word")
	xLength := stringSetLength(t, m, state, x, "3")
	constant := astTestString(t, m, "ab")
	astTestMust(t, state.Internalize(constant, false))
	inner := stringDependencyConcat(t, theory, x, x)
	outer := stringDependencyConcat(t, theory, inner, constant)
	stringDependencyEquality(t, theory, word, outer, true)
	stringDependencyPrivateMerge(t, theory, word, outer)
	equality, err := m.MkEq(word, outer)
	astTestMust(t, err)
	astTestMust(t, state.markRelevant(equality))
	before := len(theory.refs)
	status, err := theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue {
		t.Fatal("actual final-check prefix did not continue after concat length propagation", status)
	}
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	var axioms []ast.ASTHandle
	for _, ref := range theory.refs[before:] {
		value, err := m.RefGet(ref)
		astTestMust(t, err)
		sort, err := m.GetSort(value)
		astTestMust(t, err)
		if sort == boolean {
			axioms = append(axioms, value)
		}
	}
	if len(axioms) != 2 {
		t.Fatal("concat length pass did not publish both nested sums", len(axioms))
	}
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	xGuard, err := m.MkEq(xLength, three)
	astTestMust(t, err)
	constantLength, err := m.MkSequenceLength(constant)
	astTestMust(t, err)
	astTestPin(t, m, constantLength)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	constantGuard, err := m.MkEq(constantLength, two)
	astTestMust(t, err)
	for i, concat := range []ast.ASTHandle{inner, outer} {
		length, err := m.MkSequenceLength(concat)
		astTestMust(t, err)
		want := []int{6, 8}[i]
		for mask := 0; mask < 4; mask++ {
			for _, output := range []int{want, want + 1} {
				value, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(output))
				astTestMust(t, err)
				first, err := m.MkBoolVal(mask&1 != 0)
				astTestMust(t, err)
				second, err := m.MkBoolVal(mask&2 != 0)
				astTestMust(t, err)
				values := map[ast.ASTHandle]ast.ASTHandle{length: value, xGuard: first, constantGuard: second}
				guardHolds := mask&1 != 0 && (i == 0 || mask&2 != 0)
				if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axioms[i]}, values); got != (!guardHolds || output == want) {
					t.Fatal("duplicate leaf sum or unique leaf premises changed", i, mask, output, got)
				}
			}
		}
	}
}

func TestSMTStringLengthCollectorRolesAndRelevancy(t *testing.T) {
	m, theory, state, _ := stringBackpropMixedFixture(t)
	lengthTester := stringGroundingWord(t, theory, "backprop-collector-length")
	valueTester := stringGroundingWord(t, theory, "backprop-collector-value")
	unrollTester := stringGroundingWord(t, theory, "backprop-collector-unroll")
	theory.lengthTestVariables[lengthTester] = struct{}{}
	theory.valueTestVariables[valueTester] = struct{}{}
	theory.unrollTestVariables[unrollTester] = struct{}{}
	inner := stringDependencyConcat(t, theory, valueTester, unrollTester)
	outer := stringDependencyConcat(t, theory, lengthTester, inner)
	variables, concats := make(map[ast.ASTHandle]int), make(map[ast.ASTHandle]int)
	astTestMust(t, theory.collectStringLengthExpressions(outer, variables, concats))
	if len(variables) != 2 || variables[valueTester] != 1 || variables[unrollTester] != 1 || variables[lengthTester] != 0 || len(concats) != 2 {
		t.Fatal("length collector incorrectly generalized classifier tester exclusions", variables, concats)
	}
	x := stringGroundingWord(t, theory, "backprop-irrelevant-x")
	y := stringGroundingWord(t, theory, "backprop-irrelevant-y")
	stringSetLength(t, m, state, x, "3")
	concat := stringDependencyConcat(t, theory, x, x)
	stringDependencyEquality(t, theory, concat, y, true)
	before := len(theory.refs)
	added, err := theory.propagateStringLengths()
	astTestMust(t, err)
	if added || len(theory.refs) != before {
		t.Fatal("length propagation ignored its native relevancy filter")
	}
}
