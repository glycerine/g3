package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSMTStringMatchingLiteralBoundariesCoreAndRollback(t *testing.T) {
	for _, suffix := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			m := astTestManager(t)
			theory, state := stringTheoryFixture(t, m)
			x := stringContainmentWord(t, theory, "matching-core-x")
			y := stringContainmentWord(t, theory, "matching-core-y")
			left := stringEqualityConcat(t, m, astTestString(t, m, "ab"), x)
			right := stringEqualityConcat(t, m, astTestString(t, m, "a"), y)
			residual := stringEqualityConcat(t, m, astTestString(t, m, "b"), x)
			if suffix {
				left = stringEqualityConcat(t, m, x, astTestString(t, m, "ba"))
				right = stringEqualityConcat(t, m, y, astTestString(t, m, "a"))
				residual = stringEqualityConcat(t, m, x, astTestString(t, m, "b"))
			}
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
			if state.Inconsistent() || state.context.enodes.get(residual) == nil || state.context.enodes.get(y).Root() != state.context.enodes.get(residual).Root() {
				t.Fatal("matching literal boundaries did not propagate their residual equality", suffix, reverse)
			}
			if theory.freshID != 0 {
				t.Fatal("deterministic boundary cancellation introduced a witness")
			}
			astTestMust(t, state.PopScopes(1))
			if node := state.context.enodes.get(residual); node != nil && node.Root() == state.context.enodes.get(y).Root() {
				t.Fatal("matching boundary consequence survived scope rollback", suffix, reverse)
			}
		}
	}
}

func TestSMTStringMatchingLiteralBoundaryExactFormulasAndBytes(t *testing.T) {
	type boundaryCase struct {
		prefixLeft, suffixLeft, right string
		delta                         string
		destinationRight, equal       bool
		conflict                      bool
	}
	cases := []boundaryCase{
		{"ab", "ba", "a", "b", true, false, false},
		{"a", "a", "ab", "b", false, false, false},
		{"a", "a", "a", "", true, true, false},
		{"", "", "ab", "ab", false, false, false},
		{"\x00\xff", "\xff\x00", "\x00", "\xff", true, false, false},
		{"ab", "ba", "ac", "", false, false, true},
	}
	for _, suffix := range []bool{false, true} {
		for _, strong := range []bool{false, true} {
			for index, test := range cases {
				m, config := smtModelGeneratorFixture(t)
				parameters, err := NewSMTStrParams(config.Context, config.Params, config.Globals, m.Budget())
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, parameters.Close()) })
				key := "str.strong_arrangements"
				astTestMust(t, config.Context.SetBoolName(config.Params, &key, strong))
				astTestMust(t, parameters.Update(config.Params))
				theory, state := stringTheoryFixture(t, m)
				theory.parameterOwner = parameters
				x := stringContainmentWord(t, theory, "matching-formula-x")
				y := stringContainmentWord(t, theory, "matching-formula-y")
				first, second := test.prefixLeft, test.right
				if suffix {
					first = test.suffixLeft
					if index == 1 {
						second = "ba"
					}
				}
				left := stringEqualityConcat(t, m, astTestString(t, m, first), x)
				right := stringEqualityConcat(t, m, astTestString(t, m, second), y)
				if suffix {
					left = stringEqualityConcat(t, m, x, astTestString(t, m, first))
					right = stringEqualityConcat(t, m, y, astTestString(t, m, second))
				}
				astTestMust(t, state.Internalize(left, false))
				astTestMust(t, state.Internalize(right, false))
				astTestMust(t, theory.arrangeMatchingLiteralBoundaries(left, right, suffix))
				axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
				astTestMust(t, err)
				parent, err := stringEqAtom(m, left, right)
				astTestMust(t, err)
				astTestPin(t, m, parent)
				var want ast.ASTHandle
				if test.conflict {
					want, err = m.MkNot(parent)
					astTestMust(t, err)
				} else {
					destination, replacement := x, y
					if test.destinationRight {
						destination, replacement = y, x
					}
					if test.equal && suffix {
						destination, replacement = x, y
					}
					if !test.equal {
						if suffix {
							replacement = stringEqualityConcat(t, m, replacement, astTestString(t, m, test.delta))
						} else {
							replacement = stringEqualityConcat(t, m, astTestString(t, m, test.delta), replacement)
						}
					}
					conclusion, err := stringEqAtom(m, destination, replacement)
					astTestMust(t, err)
					astTestPin(t, m, conclusion)
					if strong {
						want, err = stringEqAtom(m, parent, conclusion)
						astTestMust(t, err)
					} else {
						negative, err := m.MkNot(parent)
						astTestMust(t, err)
						astTestPin(t, m, negative)
						want, err = m.MkOr(negative, conclusion)
						astTestMust(t, err)
					}
				}
				astTestPin(t, m, want)
				if axiom != want {
					t.Fatal("native literal-boundary equality orientation or crop changed", suffix, strong, index)
				}
				rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
				astTestMust(t, err)
				for _, xv := range []string{"", "a", "b", "ab", "ba", "\x00\xff"} {
					for _, yv := range []string{"", "a", "b", "ab", "ba", "\x00\xff"} {
						if !stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, xv), y: astTestString(t, m, yv)}) {
							t.Fatal("literal-boundary cancellation excluded a concrete byte valuation", suffix, strong, index, xv, yv)
						}
					}
				}
				astTestMust(t, rw.Close())
				if !test.conflict {
					// The published conclusion's operands are now attached.
					// Once their private classes agree, source suppresses the
					// duplicate cancellation axiom.
					conclusion, err := m.GetArg(axiom, 1)
					astTestMust(t, err)
					if strong && conclusion == parent {
						// mk_eq_atom orders the strong axiom's sides by id.
						conclusion, err = m.GetArg(axiom, 0)
						astTestMust(t, err)
					}
					first, err := m.GetArg(conclusion, 0)
					astTestMust(t, err)
					second, err := m.GetArg(conclusion, 1)
					astTestMust(t, err)
					theory.find.Merge(uint32(state.context.enodes.get(first).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(second).TheoryVar(theory.FamilyID())))
					before := len(theory.refs)
					astTestMust(t, theory.arrangeMatchingLiteralBoundaries(left, right, suffix))
					if len(theory.refs) != before {
						t.Fatal("already equivalent boundary residuals emitted another axiom", suffix, strong, index)
					}
				}
			}
		}
	}
}

func TestSMTStringOpposingLiteralBoundariesCoreAndRollback(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
		a := stringContainmentWord(t, theory, "opposing-core-a")
		y := stringContainmentWord(t, theory, "opposing-core-y")
		text := astTestString(t, m, "aba")
		left, right := stringEqualityConcat(t, m, text, y), stringEqualityConcat(t, m, a, text)
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
			t.Fatal("opposing literal boundaries did not reach zero/one/full overlap plus gap", reverse, state.Inconsistent(), state.theoryCaseSplitSets)
		}
		astTestMust(t, state.PopScopes(1))
		if len(state.theoryCaseSplitSets) != 0 || state.context.enodes.get(left).Root() == state.context.enodes.get(right).Root() {
			t.Fatal("opposing arrangement registration or equality survived pop", reverse)
		}
	}
}

func TestSMTStringOpposingLiteralBoundaryBytesAndOverlapOrder(t *testing.T) {
	cases := []struct {
		first, second string
		overlaps      []int
	}{
		{"aba", "aba", []int{0, 1, 3}},
		{"ab", "bc", []int{0, 1}},
		{"ab", "cd", []int{0}},
		{"\x00\xff", "\xff\x00", []int{0, 1}},
		{"", "ab", []int{0}},
		{"ab", "", []int{0}},
		{"", "", []int{0}},
	}
	for _, strong := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			for _, test := range cases {
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
				a := stringContainmentWord(t, theory, "opposing-truth-a")
				y := stringContainmentWord(t, theory, "opposing-truth-y")
				left := stringEqualityConcat(t, m, astTestString(t, m, test.first), y)
				right := stringEqualityConcat(t, m, a, astTestString(t, m, test.second))
				if reverse {
					left, right = right, left
				}
				astTestMust(t, state.Internalize(left, false))
				astTestMust(t, state.Internalize(right, false))
				astTestMust(t, theory.simplifyStringConcatEquality(left, right))
				variables := theory.arrangementVars[[2]ast.ASTHandle{left, right}]
				if variables[0].IsNull() || variables[1].IsNull() || !variables[2].IsNull() || theory.freshID != 2 {
					t.Fatal("opposing-boundary witness/flag setup changed", variables, theory.freshID)
				}
				if _, merged := theory.cutHistory[variables[0]]; merged {
					t.Fatal("native family 6 must not merge origins into its common witness")
				}
				if len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != len(test.overlaps)+1 {
					t.Fatal("opposing-boundary overlap list was incomplete", test)
				}
				for i, literal := range state.theoryCaseSplitSets[0] {
					wantPriority := 0.1
					if i == 1 {
						wantPriority = 0.5
					}
					if state.theoryAwareBranching.priorities[literal.Var()] != wantPriority {
						t.Fatal("opposing overlap priority/order changed", i, test)
					}
					if i == 0 {
						continue
					}
					size := test.overlaps[i-1]
					option := state.boolExpr[literal.Var()]
					for position, value := range map[uint32]string{0: test.first[:len(test.first)-size], 2: test.second[size:]} {
						equality, err := m.GetArg(option, position)
						astTestMust(t, err)
						constant, err := m.GetArg(equality, 1)
						astTestMust(t, err)
						text, literal, err := m.SequenceStringValue(constant)
						astTestMust(t, err)
						if !literal || text != value {
							t.Fatal("opposing overlap cropped the wrong byte boundary", i, position, text, value)
						}
					}
					for _, position := range []uint32{1, 3} {
						equality, err := m.GetArg(option, position)
						astTestMust(t, err)
						length, err := m.GetArg(equality, 1)
						astTestMust(t, err)
						kind, err := m.GetNumArgs(length)
						astTestMust(t, err)
						if kind != 1 {
							t.Fatal("opposing overlap literal length expression was replaced by a numeral")
						}
					}
				}
				axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
				astTestMust(t, err)
				kind, err := m.GetDeclKind(axiom)
				astTestMust(t, err)
				if (kind == ast.ASTOpIff) != strong {
					t.Fatal("opposing boundary strong/weak parameter was ignored")
				}
				rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
				astTestMust(t, err)
				for _, av := range []string{"", "a", "b", "ab", "aba", "\x00\xff"} {
					for _, yv := range []string{"", "a", "b", "ab", "aba", "\x00\xff"} {
						witness := "w"
						if len(av) > len(test.first) {
							witness = av[len(test.first):]
						}
						if !stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{a: astTestString(t, m, av), y: astTestString(t, m, yv), variables[0]: astTestString(t, m, witness)}) {
							t.Fatal("opposing-boundary arrangement excluded a concrete byte valuation", strong, reverse, test, av, yv)
						}
					}
				}
				astTestMust(t, rw.Close())
			}
		}
	}
}

func TestSMTStringOpposingBoundaryWitnessReuseAndUnfilteredOverlap(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	z := stringContainmentWord(t, theory, "opposing-reuse-z")
	y := stringContainmentWord(t, theory, "opposing-reuse-y")
	ab := astTestString(t, m, "ab")
	a := stringEqualityConcat(t, m, ab, z)
	left, right := stringEqualityConcat(t, m, ab, y), stringEqualityConcat(t, m, a, astTestString(t, m, "b"))
	astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
	astTestMust(t, state.Internalize(left, false))
	astTestMust(t, state.Internalize(right, false))
	astTestMust(t, state.PushScope())
	astTestMust(t, theory.simplifyStringConcatEquality(left, right))
	if len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 3 {
		t.Fatal("native opposing-boundary overlap enumeration was filtered")
	}
	last := state.boolExpr[state.theoryCaseSplitSets[0][2].Var()]
	equality, err := m.GetArg(last, 0)
	astTestMust(t, err)
	crop, err := m.GetArg(equality, 1)
	astTestMust(t, err)
	possible, err := theory.canTwoNodesEqual(a, crop)
	astTestMust(t, err)
	if possible {
		t.Fatal("unfiltered-overlap regression did not establish an infeasible literal crop")
	}
	witness := theory.arrangementVars[[2]ast.ASTHandle{left, right}][0]
	if _, merged := theory.cutHistory[witness]; merged {
		t.Fatal("opposing residual acquired cut origins")
	}
	astTestMust(t, state.PopScopes(1))
	if state.context.enodes.get(witness) != nil {
		t.Fatal("opposing witness enode survived pop")
	}
	before := theory.freshID
	astTestMust(t, theory.simplifyStringConcatEquality(right, left))
	if theory.freshID != before || len(theory.arrangementVars) != 1 || state.context.enodes.get(witness) == nil {
		t.Fatal("opposing reverse-key witness cache did not refresh its retained residual")
	}
}

func TestSMTStringOpposingBoundaryLoopAlternatives(t *testing.T) {
	for _, finite := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			m, config := smtModelGeneratorFixture(t)
			parameters, err := NewSMTStrParams(config.Context, config.Params, config.Globals, m.Budget())
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, parameters.Close()) })
			key := "str.finite_overlap_models"
			astTestMust(t, config.Context.SetBoolName(config.Params, &key, finite))
			astTestMust(t, parameters.Update(config.Params))
			theory, state := stringTheoryFixture(t, m)
			theory.parameterOwner = parameters
			assumptions, err := theory.AddTheoryAssumptions(nil)
			astTestMust(t, err)
			astTestPin(t, m, assumptions[0])
			astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
			a := stringContainmentWord(t, theory, "opposing-loop-a")
			y := stringContainmentWord(t, theory, "opposing-loop-y")
			text := astTestString(t, m, "aba")
			left, right := stringEqualityConcat(t, m, text, y), stringEqualityConcat(t, m, a, text)
			if reverse {
				left, right = right, left
			}
			astTestMust(t, theory.initializeStringCut(a))
			astTestMust(t, theory.initializeStringCut(y))
			astTestMust(t, theory.mergeStringCut(a, 0, y))
			astTestMust(t, theory.simplifyStringConcatEquality(left, right))
			if !theory.loopDetected || len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 4 {
				t.Fatal("opposing loop lost its finite byte overlaps", finite, reverse)
			}
			literal := state.theoryCaseSplitSets[0][0]
			alternative := state.boolExpr[literal.Var()]
			if finite {
				if state.theoryAwareBranching.priorities[literal.Var()] != -0.1 {
					t.Fatal("opposing finite-tester priority changed", reverse)
				}
				yes, err := m.GetArg(alternative, 1)
				astTestMust(t, err)
				text, constant, err := m.SequenceStringValue(yes)
				astTestMust(t, err)
				if !constant || text != "yes" {
					t.Fatal("opposing finite test did not select yes")
				}
			} else {
				term, err := m.RefGet(theory.overlapAssumption)
				astTestMust(t, err)
				if alternative != term {
					t.Fatal("opposing loop lost the current overlap assumption")
				}
			}
			witness := theory.arrangementVars[[2]ast.ASTHandle{left, right}][0]
			if _, merged := theory.cutHistory[witness]; merged {
				t.Fatal("suppressed opposing cut acquired origins")
			}
		}
	}
}
