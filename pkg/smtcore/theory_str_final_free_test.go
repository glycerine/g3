package smtcore

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTStringFreeFinalCheckCreatesAndReplaysActualTesters(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "free-final-word")
	status, err := theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || len(theory.testers.lengthIndicators[word]) != 1 {
		t.Fatal("actual final checker did not create free-variable length options", status)
	}
	first := theory.testers.lengthIndicators[word][0]
	fresh := theory.freshID
	status, err = theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || theory.freshID != fresh {
		t.Fatal("unassigned final-check replay created another tester", status)
	}
	stringTesterNewEquality(t, theory, first, astTestString(t, m, "1"))
	if len(theory.testers.valueIndicators[word][1]) != 1 {
		t.Fatal("actual final-check tester did not enter equality callback generation")
	}
	stringTesterNewEquality(t, theory, word, astTestString(t, m, "a"))
	// Source allows prefix length work to finish before the assigned branch.
	for i := 0; i < 8; i++ {
		status, err = theory.FinalCheck()
		astTestMust(t, err)
		if status == SMTFinalCheckDone {
			return
		}
	}
	t.Fatal("assigned free variable did not reach source completion", status)
}

func TestSMTStringFreeDispatchPrivateUnrollRouting(t *testing.T) {
	for _, general := range []bool{false, true} {
		t.Run(map[bool]string{false: "literal", true: "general"}[general], func(t *testing.T) {
			m, theory, state, _ := stringLibraryFixture(t)
			word := stringGroundingWord(t, theory, "free-private-unroll")
			count := stringLibraryInteger(t, m, state, "free-private-count")
			regex := stringRegexLiteral(t, m, "ab")
			if general {
				var err error
				regex, err = m.MkRegexUnion(regex, stringRegexLiteral(t, m, "cd"))
				astTestMust(t, err)
			}
			unroll := stringUnrollTerm(t, theory, regex, count)
			stringDependencyPrivateMerge(t, theory, word, unroll)
			a := &smtStringDependencyAnalysis{freeVariables: map[ast.ASTHandle]int{word: 1}}
			astTestMust(t, theory.assignStringFreeVariables(a))
			if len(a.freeVariables) != 0 || len(theory.testers.lengthIndicators[word]) != 0 {
				t.Fatal("unroll-bounded variable entered ordinary length testing")
			}
			if general {
				if theory.testers.unrollDefinitions[unroll].IsNull() || len(theory.testers.unrollHistories[word]) != 0 {
					t.Fatal("general unroll did not use regex assignment")
				}
			} else {
				if len(theory.testers.unrollHistories[word]) != 1 || len(theory.testers.unrollDefinitions) != 0 {
					t.Fatal("simple unroll did not use literal-period assignment")
				}
			}
		})
	}
}

func TestSMTStringFreeDispatchUsesCoreCyclesAndRemovesSkippedArguments(t *testing.T) {
	for _, otherFree := range []bool{false, true} {
		t.Run(map[bool]string{false: "other-argument-bounded", true: "both-free"}[otherFree], func(t *testing.T) {
			m, theory, state, _ := stringLibraryFixture(t)
			left := stringGroundingWord(t, theory, "free-core-left")
			right := stringGroundingWord(t, theory, "free-core-right")
			concat := stringDependencyConcat(t, theory, left, right)
			count := stringLibraryInteger(t, m, state, "free-core-count")
			unroll := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, "x"), count)
			astTestMust(t, state.addEquality(state.context.enodes.get(concat), state.context.enodes.get(unroll), NewSMTEqualityAxiom()))
			if same, err := theory.sameStringClass(concat, unroll); err != nil || same {
				t.Fatal("test requires merged core cycle before private theory callback", same, err)
			}
			free := map[ast.ASTHandle]int{left: 1}
			if otherFree {
				free[right] = 1
			}
			a := &smtStringDependencyAnalysis{freeVariables: free, unrollGroups: map[ast.ASTHandle]map[ast.ASTHandle]int{unroll: {unroll: 1}}}
			astTestMust(t, theory.assignStringFreeVariables(a))
			if len(a.freeVariables) != 0 || len(theory.testers.lengthIndicators) != 0 {
				t.Fatal("concat arguments were not removed before ordinary free assignment")
			}
			_, built := theory.testers.concatUnrollDefinitions[[2]ast.ASTHandle{concat, unroll}]
			if built != otherFree {
				t.Fatal("core concat eligibility ignored other argument boundedness", built, otherFree)
			}
		})
	}
}

func TestSMTStringFreeDispatchNestedConcatAndLiteralBoundedness(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	left := stringGroundingWord(t, theory, "free-nested-left")
	right := stringGroundingWord(t, theory, "free-nested-right")
	inner := stringDependencyConcat(t, theory, left, right)
	outer := stringDependencyConcat(t, theory, inner, astTestString(t, m, "suffix"))
	count := stringLibraryInteger(t, m, state, "free-nested-count")
	regex := stringRegexLiteral(t, m, "x")
	u1 := stringUnrollTerm(t, theory, regex, count)
	secondCount := stringLibraryInteger(t, m, state, "free-nested-count2")
	u2 := stringUnrollTerm(t, theory, regex, secondCount)
	astTestMust(t, state.addEquality(state.context.enodes.get(inner), state.context.enodes.get(u1), NewSMTEqualityAxiom()))
	astTestMust(t, state.addEquality(state.context.enodes.get(outer), state.context.enodes.get(u2), NewSMTEqualityAxiom()))
	a := &smtStringDependencyAnalysis{freeVariables: map[ast.ASTHandle]int{left: 1, right: 1}, unrollGroups: map[ast.ASTHandle]map[ast.ASTHandle]int{u1: {u1: 1}, u2: {u2: 1}}}
	astTestMust(t, theory.assignStringFreeVariables(a))
	if len(theory.testers.concatUnrollDefinitions) != 2 {
		t.Fatal("nested concat with its own unroll group or literal argument incorrectly marked bounded")
	}
}

func TestSMTStringFreeKernelModelAndCPPReference(t *testing.T) {
	for _, target := range []int{0, 1, 3, -1} {
		t.Run(strconv.Itoa(target), func(t *testing.T) {
			m, parent, theory := stringExpressionSolverFixture(t)
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			arithmetic, err := NewSMTMixedLinearArithmeticTheory(family)
			astTestMust(t, err)
			astTestMust(t, parent.internalizer.RegisterTheory(arithmetic))
			word := stringGroundingWord(t, theory, "free-model-word")
			length, err := m.MkSequenceLength(word)
			astTestMust(t, err)
			number, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(target))
			astTestMust(t, err)
			equal, err := m.MkEq(length, number)
			astTestMust(t, err)
			cppConstraint := "(= (str.len s) " + strconv.Itoa(target) + ")"
			cppQuery := "(str.len s)"
			cppValue := strconv.Itoa(target)
			if target < 0 {
				empty := astTestString(t, m, "")
				equal, err = m.MkEq(word, empty)
				astTestMust(t, err)
				equal, err = m.MkNot(equal)
				astTestMust(t, err)
				cppConstraint = "(not (= s \"\"))"
				cppQuery = "(= s \"\")"
				cppValue = "false"
			}
			astTestMust(t, parent.Assert(equal, ast.ASTHandle{}))
			parent.produceModel = true
			status, err := parent.Check()
			astTestMust(t, err)
			if status != lbool.True || parent.generated == nil {
				t.Fatal("free-variable String kernel did not solve/build a model", status)
			}
			decl, err := m.GetDecl(word)
			astTestMust(t, err)
			value, err := parent.GeneratedModel().ConstantInterpretation(decl)
			astTestMust(t, err)
			text, literal, err := m.SequenceStringValue(value)
			astTestMust(t, err)
			if !literal || target >= 0 && len(text) != target || target < 0 && len(text) == 0 {
				t.Fatal("free-variable model violates asserted length", text, literal)
			}
			// Zero length may be grounded by native basic/coherence axioms
			// before the free-variable branch is needed.
			if target != 0 && (len(theory.testers.lengthIndicators[word]) == 0 || len(theory.testers.valueIndicators[word][int32(len(text))]) == 0) {
				t.Fatal("model bypassed actual length/value tester pipeline")
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, reference, "-in")
				command.Stdin = strings.NewReader("(set-option :smt.string_solver z3str3)\n(set-logic QF_S)\n(declare-const s String)\n(assert " + cppConstraint + ")\n(check-sat)\n(get-value (" + cppQuery + "))\n")
				output, err := command.CombinedOutput()
				if err != nil || strings.TrimSpace(string(output)) != "sat\n(("+cppQuery+" "+cppValue+"))" {
					t.Fatal("free-variable length result differs from prebuilt legacy C++", err, string(output))
				}
			}
		})
	}
}
