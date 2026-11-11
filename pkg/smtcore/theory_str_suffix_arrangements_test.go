package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSMTStringSuffixConcatCoreArrangement(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
		x := stringContainmentWord(t, theory, "suffix-core-x")
		y := stringContainmentWord(t, theory, "suffix-core-y")
		a := stringContainmentWord(t, theory, "suffix-core-a")
		left := stringEqualityConcat(t, m, x, y)
		right := stringEqualityConcat(t, m, a, astTestString(t, m, "ab"))
		if reverse {
			left, right = right, left
		}
		astTestMust(t, state.Internalize(left, false))
		astTestMust(t, state.Internalize(right, false))
		astTestMust(t, state.PushScope())
		equality, err := stringEqAtom(m, left, right)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(equality, true))
		literal, err := state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		if state.Inconsistent() || len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 4 {
			t.Fatal("literal-suffix equality did not reach its cut and three byte splits", reverse, state.Inconsistent(), state.theoryCaseSplitSets)
		}
		astTestMust(t, state.PopScopes(1))
		if len(state.theoryCaseSplitSets) != 0 || state.context.enodes.get(left).Root() == state.context.enodes.get(right).Root() {
			t.Fatal("suffix arrangement registration or equality survived pop", reverse)
		}
	}
}

func TestSMTStringSuffixConcatUnknownBytesAndPriorities(t *testing.T) {
	for _, strong := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			for _, text := range []string{"ab", "\x00\xff", ""} {
				m, config := smtModelGeneratorFixture(t)
				parameters, err := NewSMTStrParams(config.Context, config.Params, config.Globals, m.Budget())
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, parameters.Close()) })
				key := "str.strong_arrangements"
				astTestMust(t, config.Context.SetBoolName(config.Params, &key, strong))
				astTestMust(t, parameters.Update(config.Params))
				theory, state := stringTheoryFixture(t, m)
				theory.parameterOwner = parameters
				astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
				x := stringContainmentWord(t, theory, "suffix-truth-x")
				y := stringContainmentWord(t, theory, "suffix-truth-y")
				a := stringContainmentWord(t, theory, "suffix-truth-a")
				left := stringEqualityConcat(t, m, x, y)
				right := stringEqualityConcat(t, m, a, astTestString(t, m, text))
				if reverse {
					left, right = right, left
				}
				astTestMust(t, state.Internalize(left, false))
				astTestMust(t, state.Internalize(right, false))
				astTestMust(t, theory.simplifyStringConcatEquality(left, right))
				variables := theory.arrangementVars[[2]ast.ASTHandle{left, right}]
				if variables[0].IsNull() || variables[1].IsNull() || !variables[2].IsNull() || theory.freshID != 2 || theory.nonemptyID != 1 {
					t.Fatal("suffix arrangement did not allocate one residual and the native flag", variables, theory.freshID)
				}
				if len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != len(text)+2 {
					t.Fatal("literal suffix byte split enumeration was incomplete", strong, reverse, text)
				}
				for i, literal := range state.theoryCaseSplitSets[0] {
					want := 0.1
					if i == 1 {
						want = 0.5
					}
					if state.theoryAwareBranching.priorities[literal.Var()] != want {
						t.Fatal("native suffix arrangement ordering/priority changed", i)
					}
				}
				history := theory.cutHistory[variables[0]]
				if len(history) != 1 || history[0].level != 0 {
					t.Fatal("boundary witness acquired a native-forbidden initial -1 frame", history)
				}
				if _, seeded := history[0].vars[variables[0]]; seeded {
					t.Fatal("boundary witness was initialized as a type-1 self cut")
				}
				axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
				astTestMust(t, err)
				kind, err := m.GetDeclKind(axiom)
				astTestMust(t, err)
				if (kind == ast.ASTOpIff) != strong {
					t.Fatal("suffix arrangement lost the strong/weak parameter", kind, strong)
				}
				rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
				astTestMust(t, err)
				words := []string{"", "a", "b", "ab", "ba", "\x00\xff"}
				for _, xv := range words {
					for _, yv := range words {
						for _, av := range words {
							witness := "w"
							if len(av) > len(xv) {
								witness = av[len(xv):]
							}
							values := map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, xv), y: astTestString(t, m, yv), a: astTestString(t, m, av), variables[0]: astTestString(t, m, witness)}
							if !stringAxiomTruth(t, m, rw, axiom, values) {
								t.Fatal("literal-suffix arrangements excluded a concrete byte valuation", strong, reverse, text, xv, yv, av)
							}
						}
					}
				}
				astTestMust(t, rw.Close())
			}
		}
	}
}

func TestSMTStringSuffixConcatKnownLengthGuards(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for mode := 0; mode < 4; mode++ {
			m := astTestManager(t)
			theory, state, _ := stringMixedEqualityFixture(t, m)
			x := stringContainmentWord(t, theory, "suffix-length-x")
			y := stringContainmentWord(t, theory, "suffix-length-y")
			a := stringContainmentWord(t, theory, "suffix-length-a")
			text := astTestString(t, m, "\x00\xff")
			left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, text)
			if reverse {
				left, right = right, left
			}
			if mode == 0 {
				stringSetLength(t, m, state, x, "1")
				stringSetLength(t, m, state, a, "2")
			} else if mode == 1 {
				stringSetLength(t, m, state, y, "3")
			} else if mode == 2 {
				stringSetLength(t, m, state, x, "2")
				stringSetLength(t, m, state, a, "1")
			} else {
				stringSetLength(t, m, state, y, "1")
			}
			astTestMust(t, theory.simplifyStringConcatEquality(left, right))
			variables := theory.arrangementVars[[2]ast.ASTHandle{left, right}]
			if variables[0].IsNull() || theory.freshID != 2 {
				t.Fatal("known suffix arrangement skipped native witness/flag creation", reverse, mode)
			}
			guards := smtStringLengthPremises{theory: theory}
			items := smtStringLengthPremises{theory: theory}
			parent, err := stringEqAtom(m, left, right)
			astTestMust(t, err)
			astTestMust(t, guards.add(parent))
			switch mode {
			case 0:
				astTestMust(t, guards.length(x, big.NewInt(1)))
				astTestMust(t, guards.length(a, big.NewInt(2)))
			case 1:
				astTestMust(t, guards.length(y, big.NewInt(3)))
				astTestMust(t, guards.length(text, big.NewInt(2)))
			case 2:
				astTestMust(t, guards.length(x, big.NewInt(2)))
				astTestMust(t, guards.length(a, big.NewInt(1)))
			case 3:
				// Native fixed cropping uses only the observed y length;
				// the literal length is syntactically fixed, not a guard.
				astTestMust(t, guards.length(y, big.NewInt(1)))
			}
			if mode < 2 {
				prefix := stringEqualityConcat(t, m, x, variables[0])
				tail := stringEqualityConcat(t, m, variables[0], text)
				astTestMust(t, items.equate(a, prefix))
				astTestMust(t, items.equate(y, tail))
				astTestMust(t, items.length(variables[0], big.NewInt(1)))
			} else {
				prefix := stringEqualityConcat(t, m, a, astTestString(t, m, "\x00"))
				astTestMust(t, items.equate(x, prefix))
				astTestMust(t, items.equate(y, astTestString(t, m, "\xff")))
			}
			premise, err := m.MkAndArray(guards.values)
			astTestMust(t, err)
			astTestPin(t, m, premise)
			conclusion, err := m.MkAndArray(items.values)
			astTestMust(t, err)
			astTestPin(t, m, conclusion)
			want, err := stringEqAtom(m, premise, conclusion)
			astTestMust(t, err)
			astTestPin(t, m, want)
			stringConcatPairHasAxiom(t, theory, 0, want)
			astTestMust(t, guards.close())
			astTestMust(t, items.close())
		}
	}
}

func TestSMTStringSuffixConcatWitnessReuseAndFiltering(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "suffix-reuse-x")
	z := stringContainmentWord(t, theory, "suffix-reuse-z")
	a := stringContainmentWord(t, theory, "suffix-reuse-a")
	y := stringEqualityConcat(t, m, z, astTestString(t, m, "b"))
	left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, astTestString(t, m, "ab"))
	astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
	astTestMust(t, state.Internalize(left, false))
	astTestMust(t, state.Internalize(right, false))
	astTestMust(t, state.PushScope())
	astTestMust(t, theory.simplifyStringConcatEquality(left, right))
	if len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 3 {
		t.Fatal("suffix feasibility did not omit the impossible empty crop")
	}
	witness := theory.arrangementVars[[2]ast.ASTHandle{left, right}][0]
	astTestMust(t, state.PopScopes(1))
	if _, remains := theory.cutHistory[witness]; remains || state.context.enodes.get(witness) != nil {
		t.Fatal("unseeded suffix witness cut/enode survived pop")
	}
	before := theory.freshID
	astTestMust(t, theory.simplifyStringConcatEquality(right, left))
	if theory.freshID != before || len(theory.arrangementVars) != 1 || state.context.enodes.get(witness) == nil {
		t.Fatal("reverse suffix witness reuse failed to refresh its existing residual")
	}
}

func TestSMTStringSuffixConcatLengthPredecessorRejectsOversizedCrop(t *testing.T) {
	m := astTestManager(t)
	theory, state, _ := stringMixedEqualityFixture(t, m)
	x := stringContainmentWord(t, theory, "suffix-oversized-x")
	y := stringContainmentWord(t, theory, "suffix-oversized-y")
	a := stringContainmentWord(t, theory, "suffix-oversized-a")
	left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, astTestString(t, m, "ab"))
	stringSetLength(t, m, state, x, "4")
	stringSetLength(t, m, state, a, "1")
	equality, err := stringEqAtom(m, left, right)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	literal, err := state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if !state.Inconsistent() || len(theory.arrangementVars) != 0 {
		t.Fatal("native length consistency did not reject an oversized literal crop before arrangement")
	}
}

func TestSMTStringSuffixConcatOverlapKnownAndUnknown(t *testing.T) {
	for _, finite := range []bool{false, true} {
		for _, known := range []bool{false, true} {
			m, config := smtModelGeneratorFixture(t)
			parameters, err := NewSMTStrParams(config.Context, config.Params, config.Globals, m.Budget())
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, parameters.Close()) })
			key := "str.finite_overlap_models"
			astTestMust(t, config.Context.SetBoolName(config.Params, &key, finite))
			astTestMust(t, parameters.Update(config.Params))
			theory, state, _ := stringMixedEqualityFixture(t, m)
			theory.parameterOwner = parameters
			assumptions, err := theory.AddTheoryAssumptions(nil)
			astTestMust(t, err)
			astTestPin(t, m, assumptions[0])
			astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
			x := stringContainmentWord(t, theory, "suffix-overlap-x")
			y := stringContainmentWord(t, theory, "suffix-overlap-y")
			a := stringContainmentWord(t, theory, "suffix-overlap-a")
			left := stringEqualityConcat(t, m, x, y)
			right := stringEqualityConcat(t, m, a, astTestString(t, m, "ab"))
			if known {
				stringSetLength(t, m, state, x, "1")
				stringSetLength(t, m, state, a, "2")
			}
			astTestMust(t, theory.initializeStringCut(y))
			astTestMust(t, theory.initializeStringCut(a))
			astTestMust(t, theory.mergeStringCut(a, 0, y))
			astTestMust(t, theory.simplifyStringConcatEquality(left, right))
			if !theory.loopDetected {
				t.Fatal("suffix overlap was not recorded", finite, known)
			}
			var alternative ast.ASTHandle
			if known {
				if len(state.theoryCaseSplitSets) != 0 {
					t.Fatal("known overlap incorrectly registered an unknown-length arrangement")
				}
				axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
				astTestMust(t, err)
				alternative, err = m.GetArg(axiom, 1)
				astTestMust(t, err)
				guards := smtStringLengthPremises{theory: theory}
				equality, err := stringEqAtom(m, left, right)
				astTestMust(t, err)
				astTestMust(t, guards.add(equality))
				astTestMust(t, guards.length(x, big.NewInt(1)))
				astTestMust(t, guards.length(a, big.NewInt(2)))
				premise, err := m.MkAndArray(guards.values)
				astTestMust(t, err)
				astTestPin(t, m, premise)
				negative, err := m.MkNot(premise)
				astTestMust(t, err)
				astTestPin(t, m, negative)
				want, err := m.MkOr(negative, alternative)
				astTestMust(t, err)
				astTestPin(t, m, want)
				if axiom != want {
					t.Fatal("known suffix overlap dropped its length/equality premise", finite)
				}
				astTestMust(t, guards.close())
			} else {
				if len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 4 {
					t.Fatal("unknown suffix overlap lost a finite literal split", finite)
				}
				alternative = state.boolExpr[state.theoryCaseSplitSets[0][0].Var()]
			}
			if finite {
				variable, found := state.BoolVar(alternative)
				if !found || state.theoryAwareBranching.priorities[variable] != -0.1 {
					t.Fatal("finite suffix overlap priority was omitted", known)
				}
				yes, err := m.GetArg(alternative, 1)
				astTestMust(t, err)
				text, constant, err := m.SequenceStringValue(yes)
				astTestMust(t, err)
				if !constant || text != "yes" {
					t.Fatal("finite suffix overlap lost its yes tester", known)
				}
			} else {
				term, err := m.RefGet(theory.overlapAssumption)
				astTestMust(t, err)
				if alternative != term {
					t.Fatal("suffix loop did not use the current overlap assumption", known)
				}
			}
			witness := theory.arrangementVars[[2]ast.ASTHandle{left, right}][0]
			if _, added := theory.cutHistory[witness]; added {
				t.Fatal("a suppressed cut still merged origins into its witness", known)
			}
		}
	}
}
