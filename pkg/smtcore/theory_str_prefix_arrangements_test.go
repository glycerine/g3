package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSMTStringPrefixConcatCoreArrangement(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
		x := stringContainmentWord(t, theory, "prefix-core-x")
		y := stringContainmentWord(t, theory, "prefix-core-y")
		n := stringContainmentWord(t, theory, "prefix-core-n")
		left := stringEqualityConcat(t, m, x, y)
		right := stringEqualityConcat(t, m, astTestString(t, m, "ab"), n)
		if reverse {
			left, right = right, left
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
		if state.Inconsistent() || len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 4 {
			t.Fatal("literal-prefix equality did not reach its three byte splits and cut", reverse, state.Inconsistent(), state.theoryCaseSplitSets)
		}
		astTestMust(t, state.PopScopes(1))
		if len(state.theoryCaseSplitSets) != 0 || state.context.enodes.get(left).Root() == state.context.enodes.get(right).Root() {
			t.Fatal("prefix arrangement registration or equality survived pop", reverse)
		}
	}
}

func TestSMTStringPrefixConcatUnknownBytesAndOrder(t *testing.T) {
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
				x := stringContainmentWord(t, theory, "prefix-truth-x")
				y := stringContainmentWord(t, theory, "prefix-truth-y")
				n := stringContainmentWord(t, theory, "prefix-truth-n")
				left := stringEqualityConcat(t, m, x, y)
				right := stringEqualityConcat(t, m, astTestString(t, m, text), n)
				if reverse {
					left, right = right, left
				}
				astTestMust(t, state.Internalize(left, false))
				astTestMust(t, state.Internalize(right, false))
				astTestMust(t, theory.simplifyStringConcatEquality(left, right))
				variables := theory.arrangementVars[[2]ast.ASTHandle{left, right}]
				if variables[0].IsNull() || variables[1].IsNull() || !variables[2].IsNull() || theory.freshID != 2 {
					t.Fatal("prefix residual/flag allocation diverged", variables, theory.freshID)
				}
				if len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != len(text)+2 {
					t.Fatal("prefix byte split enumeration was incomplete", strong, reverse, text)
				}
				for i, literal := range state.theoryCaseSplitSets[0] {
					want := 0.1
					if i == len(text) {
						want = 0.5
					}
					if state.theoryAwareBranching.priorities[literal.Var()] != want {
						t.Fatal("native prefix split order/priority changed", i)
					}
					if i <= len(text) {
						option := state.boolExpr[literal.Var()]
						lengthEquality, err := m.GetArg(option, 2)
						astTestMust(t, err)
						actual, err := m.GetArg(lengthEquality, 1)
						astTestMust(t, err)
						want, err := m.MkSequenceLength(astTestString(t, m, text[:i]))
						astTestMust(t, err)
						if actual != want {
							t.Fatal("source prefix length expression was replaced by a numeral", i)
						}
					}
				}
				axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
				astTestMust(t, err)
				kind, err := m.GetDeclKind(axiom)
				astTestMust(t, err)
				if (kind == ast.ASTOpIff) != strong {
					t.Fatal("prefix strong/weak arrangement parameter was ignored")
				}
				rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
				astTestMust(t, err)
				words := []string{"", "a", "b", "ab", "ba", "\x00\xff"}
				for _, xv := range words {
					for _, yv := range words {
						for _, nv := range words {
							witness := "w"
							if len(xv) > len(text) {
								witness = xv[len(text):]
							}
							values := map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, xv), y: astTestString(t, m, yv), n: astTestString(t, m, nv), variables[0]: astTestString(t, m, witness)}
							if !stringAxiomTruth(t, m, rw, axiom, values) {
								t.Fatal("literal-prefix arrangements excluded a concrete byte valuation", strong, reverse, text, xv, yv, nv)
							}
						}
					}
				}
				astTestMust(t, rw.Close())
			}
		}
	}
}

func TestSMTStringPrefixConcatKnownLengthGuards(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for mode := 0; mode < 4; mode++ {
			m := astTestManager(t)
			theory, state, _ := stringMixedEqualityFixture(t, m)
			x := stringContainmentWord(t, theory, "prefix-length-x")
			y := stringContainmentWord(t, theory, "prefix-length-y")
			n := stringContainmentWord(t, theory, "prefix-length-n")
			text := astTestString(t, m, "\x00\xff")
			left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, text, n)
			if reverse {
				left, right = right, left
			}
			if mode == 0 {
				stringSetLength(t, m, state, x, "1")
			} else if mode == 1 {
				stringSetLength(t, m, state, y, "3")
				stringSetLength(t, m, state, n, "2")
			} else if mode == 2 {
				stringSetLength(t, m, state, x, "3")
			} else {
				stringSetLength(t, m, state, y, "1")
				stringSetLength(t, m, state, n, "2")
			}
			astTestMust(t, theory.simplifyStringConcatEquality(left, right))
			variables := theory.arrangementVars[[2]ast.ASTHandle{left, right}]
			if variables[0].IsNull() || theory.freshID != 2 {
				t.Fatal("known prefix arrangement omitted native witness/flag creation", reverse, mode)
			}
			guards := smtStringLengthPremises{theory: theory}
			items := smtStringLengthPremises{theory: theory}
			parent, err := m.MkEq(left, right)
			astTestMust(t, err)
			astTestMust(t, guards.add(parent))
			switch mode {
			case 0:
				astTestMust(t, guards.length(x, big.NewInt(1)))
			case 1:
				astTestMust(t, guards.length(y, big.NewInt(3)))
				astTestMust(t, guards.length(n, big.NewInt(2)))
			case 2:
				astTestMust(t, guards.length(x, big.NewInt(3)))
			case 3:
				astTestMust(t, guards.length(y, big.NewInt(1)))
				astTestMust(t, guards.length(n, big.NewInt(2)))
			}
			if mode < 2 {
				astTestMust(t, items.equate(x, astTestString(t, m, "\x00")))
				tail := stringEqualityConcat(t, m, astTestString(t, m, "\xff"), n)
				astTestMust(t, items.equate(y, tail))
			} else {
				prefix := stringEqualityConcat(t, m, text, variables[0])
				tail := stringEqualityConcat(t, m, variables[0], y)
				astTestMust(t, items.equate(x, prefix))
				astTestMust(t, items.equate(n, tail))
				astTestMust(t, items.length(variables[0], big.NewInt(1)))
			}
			premise, err := m.MkAndArray(guards.values)
			astTestMust(t, err)
			astTestPin(t, m, premise)
			conclusion, err := m.MkAndArray(items.values)
			astTestMust(t, err)
			astTestPin(t, m, conclusion)
			want, err := m.MkEq(premise, conclusion)
			astTestMust(t, err)
			astTestPin(t, m, want)
			stringConcatPairHasAxiom(t, theory, 0, want)
			astTestMust(t, guards.close())
			astTestMust(t, items.close())
		}
	}
}

func TestSMTStringPrefixConcatWitnessReuseAndFiltering(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	z := stringContainmentWord(t, theory, "prefix-reuse-z")
	y := stringContainmentWord(t, theory, "prefix-reuse-y")
	n := stringContainmentWord(t, theory, "prefix-reuse-n")
	x := stringEqualityConcat(t, m, astTestString(t, m, "a"), z)
	left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, astTestString(t, m, "ab"), n)
	astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
	astTestMust(t, state.Internalize(left, false))
	astTestMust(t, state.Internalize(right, false))
	astTestMust(t, state.PushScope())
	astTestMust(t, theory.simplifyStringConcatEquality(left, right))
	if len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 3 {
		t.Fatal("prefix feasibility did not omit the impossible empty crop")
	}
	witness := theory.arrangementVars[[2]ast.ASTHandle{left, right}][0]
	astTestMust(t, state.PopScopes(1))
	if _, remains := theory.cutHistory[witness]; remains || state.context.enodes.get(witness) != nil {
		t.Fatal("unseeded prefix residual cut/enode survived pop")
	}
	before := theory.freshID
	astTestMust(t, theory.simplifyStringConcatEquality(right, left))
	if theory.freshID != before || len(theory.arrangementVars) != 1 || state.context.enodes.get(witness) == nil {
		t.Fatal("reverse-key prefix reuse failed to refresh its existing residual")
	}
}

func TestSMTStringPrefixConcatLengthPredecessorRejectsNegativeCrop(t *testing.T) {
	m := astTestManager(t)
	theory, state, _ := stringMixedEqualityFixture(t, m)
	x := stringContainmentWord(t, theory, "prefix-negative-x")
	y := stringContainmentWord(t, theory, "prefix-negative-y")
	n := stringContainmentWord(t, theory, "prefix-negative-n")
	left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, astTestString(t, m, "ab"), n)
	stringSetLength(t, m, state, y, "4")
	stringSetLength(t, m, state, n, "1")
	equality, err := m.MkEq(left, right)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	literal, err := state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if !state.Inconsistent() || len(theory.arrangementVars) != 0 {
		t.Fatal("native length consistency did not reject a negative inferred prefix before arrangement")
	}
}

func TestSMTStringPrefixConcatOverlapKnownAndUnknown(t *testing.T) {
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
			x := stringContainmentWord(t, theory, "prefix-overlap-x")
			y := stringContainmentWord(t, theory, "prefix-overlap-y")
			n := stringContainmentWord(t, theory, "prefix-overlap-n")
			left := stringEqualityConcat(t, m, x, y)
			right := stringEqualityConcat(t, m, astTestString(t, m, "ab"), n)
			if known {
				stringSetLength(t, m, state, x, "3")
			}
			astTestMust(t, theory.initializeStringCut(x))
			astTestMust(t, theory.initializeStringCut(n))
			astTestMust(t, theory.mergeStringCut(x, 0, n))
			astTestMust(t, theory.simplifyStringConcatEquality(left, right))
			if !theory.loopDetected {
				t.Fatal("prefix overlap was not recorded", finite, known)
			}
			var alternative ast.ASTHandle
			if known {
				if len(state.theoryCaseSplitSets) != 0 {
					t.Fatal("known prefix overlap registered an unknown-length split")
				}
				axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
				astTestMust(t, err)
				alternative, err = m.GetArg(axiom, 1)
				astTestMust(t, err)
				guards := smtStringLengthPremises{theory: theory}
				equality, err := m.MkEq(left, right)
				astTestMust(t, err)
				astTestMust(t, guards.add(equality))
				astTestMust(t, guards.length(x, big.NewInt(3)))
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
					t.Fatal("known prefix overlap dropped its exact observed-length premise", finite)
				}
				astTestMust(t, guards.close())
			} else {
				if len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 4 {
					t.Fatal("unknown prefix overlap lost a literal split", finite)
				}
				group := state.theoryCaseSplitSets[0]
				alternative = state.boolExpr[group[len(group)-1].Var()]
			}
			if finite {
				variable, found := state.BoolVar(alternative)
				if !found || state.theoryAwareBranching.priorities[variable] != -0.1 {
					t.Fatal("finite prefix overlap priority was omitted", known)
				}
				yes, err := m.GetArg(alternative, 1)
				astTestMust(t, err)
				text, constant, err := m.SequenceStringValue(yes)
				astTestMust(t, err)
				if !constant || text != "yes" {
					t.Fatal("finite prefix overlap lost its yes tester", known)
				}
			} else {
				term, err := m.RefGet(theory.overlapAssumption)
				astTestMust(t, err)
				if alternative != term {
					t.Fatal("prefix loop did not use the current overlap assumption", known)
				}
			}
			witness := theory.arrangementVars[[2]ast.ASTHandle{left, right}][0]
			if _, added := theory.cutHistory[witness]; added {
				t.Fatal("suppressed prefix cut still merged witness origins", known)
			}
		}
	}
}

func TestSMTStringPrefixConcatAlignmentUsesStrongParameter(t *testing.T) {
	for _, strong := range []bool{false, true} {
		m, config := smtModelGeneratorFixture(t)
		parameters, err := NewSMTStrParams(config.Context, config.Params, config.Globals, m.Budget())
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, parameters.Close()) })
		key := "str.strong_arrangements"
		astTestMust(t, config.Context.SetBoolName(config.Params, &key, strong))
		astTestMust(t, parameters.Update(config.Params))
		theory, state, _ := stringMixedEqualityFixture(t, m)
		theory.parameterOwner = parameters
		x := stringContainmentWord(t, theory, "prefix-align-x")
		y := stringContainmentWord(t, theory, "prefix-align-y")
		n := stringContainmentWord(t, theory, "prefix-align-n")
		text := astTestString(t, m, "ab")
		left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, text, n)
		astTestMust(t, state.Internalize(left, false))
		astTestMust(t, state.Internalize(right, false))
		stringSetLength(t, m, state, x, "2")
		// Exercise the retained handler's own alignment branch directly;
		// earlier simplify_concat_equality quick paths have separate tests.
		astTestMust(t, theory.arrangeLiteralPrefixConcats(left, right))
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		kind, err := m.GetDeclKind(axiom)
		astTestMust(t, err)
		if (kind == ast.ASTOpIff) != strong || theory.freshID != 2 {
			t.Fatal("native prefix alignment lost its strong/weak form or cached witness setup", strong, kind)
		}
		if len(state.theoryCaseSplitSets) != 0 {
			t.Fatal("known prefix alignment incorrectly introduced a split disjunction")
		}
	}
}
