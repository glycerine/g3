package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSMTStringVariableConcatCoreArrangement(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	astTestMust(t, state.configureBooleanHeuristics(SMTParamsState{CaseSplitStrategy: 6, TheoryCaseSplit: true, InvDecay: 1.05}))
	x := stringContainmentWord(t, theory, "type1-core-x")
	y := stringContainmentWord(t, theory, "type1-core-y")
	a := stringContainmentWord(t, theory, "type1-core-a")
	b := stringContainmentWord(t, theory, "type1-core-b")
	left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
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
	if state.Inconsistent() || len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 3 {
		t.Fatal("variable-concat equality did not reach its native three arrangements", state.Inconsistent(), state.theoryCaseSplitSets)
	}
	astTestMust(t, state.PopScopes(1))
	if len(state.theoryCaseSplitSets) != 0 || state.context.enodes.get(left).Root() == state.context.enodes.get(right).Root() {
		t.Fatal("arrangement registration or equality survived pop")
	}
}

func TestSMTStringVariableConcatUnknownTruthAndPriorities(t *testing.T) {
	for _, strong := range []bool{false, true} {
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
		x := stringContainmentWord(t, theory, "type1-truth-x")
		y := stringContainmentWord(t, theory, "type1-truth-y")
		a := stringContainmentWord(t, theory, "type1-truth-a")
		b := stringContainmentWord(t, theory, "type1-truth-b")
		left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
		astTestMust(t, theory.simplifyStringConcatEquality(left, right))
		variables := theory.arrangementVars[[2]ast.ASTHandle{left, right}]
		if variables[0].IsNull() || variables[1].IsNull() || variables[2].IsNull() || theory.freshID != 3 || theory.nonemptyID != 2 {
			t.Fatal("native residual strings/integer flag were not allocated exactly once", variables, theory.freshID)
		}
		if len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 3 {
			t.Fatal("unknown arrangement did not register three mutually exclusive options")
		}
		for i, literal := range state.theoryCaseSplitSets[0] {
			want := 0.1
			if i == 2 {
				want = 0.5
			}
			if state.theoryAwareBranching.priorities[literal.Var()] != want {
				t.Fatal("arrangement priority did not reach the actual theory-aware queue", i)
			}
		}
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		kind, err := m.GetDeclKind(axiom)
		astTestMust(t, err)
		if (kind == ast.ASTOpIff) != strong {
			t.Fatal("strong-arrangements parameter did not select equivalence", strong, kind)
		}
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		words := []string{"", "a", "b", "ab", "ba", "\x00\xff"}
		for _, xv := range words {
			for _, yv := range words {
				for _, av := range words {
					for _, bv := range words {
						first, second := "w", "w"
						if len(av) > len(xv) {
							first = av[len(xv):]
						}
						if len(xv) > len(av) {
							second = xv[len(av):]
						}
						values := map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, xv), y: astTestString(t, m, yv), a: astTestString(t, m, av), b: astTestString(t, m, bv), variables[0]: astTestString(t, m, first), variables[1]: astTestString(t, m, second)}
						if !stringAxiomTruth(t, m, rw, axiom, values) {
							t.Fatal("three-way arrangement excluded independent concrete strings", strong, xv, yv, av, bv)
						}
					}
				}
			}
		}
		astTestMust(t, rw.Close())
	}
}

func TestSMTStringVariableConcatKnownLengthOrientations(t *testing.T) {
	for _, suffix := range []bool{false, true} {
		for _, longerLeft := range []bool{false, true} {
			m := astTestManager(t)
			theory, state, _ := stringMixedEqualityFixture(t, m)
			x := stringContainmentWord(t, theory, "type1-length-x")
			y := stringContainmentWord(t, theory, "type1-length-y")
			a := stringContainmentWord(t, theory, "type1-length-a")
			b := stringContainmentWord(t, theory, "type1-length-b")
			left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
			first, second := x, a
			if suffix {
				first, second = y, b
			}
			lv, rv := int64(1), int64(2)
			if longerLeft {
				lv, rv = rv, lv
			}
			stringSetLength(t, m, state, first, big.NewInt(lv).String())
			stringSetLength(t, m, state, second, big.NewInt(rv).String())
			astTestMust(t, theory.simplifyStringConcatEquality(left, right))
			variables := theory.arrangementVars[[2]ast.ASTHandle{left, right}]
			cutLeft := longerLeft != suffix
			witness, prefix, cutter, cut, tail := variables[0], x, a, y, b
			if cutLeft {
				witness, prefix, cutter, cut, tail = variables[1], a, x, b, y
			}
			parentEq, err := stringEqAtom(m, left, right)
			astTestMust(t, err)
			guards := smtStringLengthPremises{theory: theory}
			astTestMust(t, guards.add(parentEq))
			astTestMust(t, guards.length(first, big.NewInt(lv)))
			astTestMust(t, guards.length(second, big.NewInt(rv)))
			conclusions := smtStringLengthPremises{theory: theory}
			one := stringEqualityConcat(t, m, prefix, witness)
			two := stringEqualityConcat(t, m, witness, tail)
			astTestMust(t, conclusions.equate(cutter, one))
			if cutLeft {
				astTestMust(t, conclusions.equate(two, cut))
			} else {
				astTestMust(t, conclusions.equate(cut, two))
			}
			astTestMust(t, conclusions.length(witness, big.NewInt(1)))
			premise, err := m.MkAndArray(guards.values)
			astTestMust(t, err)
			astTestPin(t, m, premise)
			conclusion, err := m.MkAndArray(conclusions.values)
			astTestMust(t, err)
			astTestPin(t, m, conclusion)
			want, err := stringEqAtom(m, premise, conclusion)
			astTestMust(t, err)
			astTestPin(t, m, want)
			stringConcatPairHasAxiom(t, theory, 0, want)
			if !theory.hasSelfCut(witness, cutter) || !theory.hasSelfCut(witness, cut) {
				t.Fatal("length-selected residual lost its cut origins", suffix, longerLeft)
			}
			astTestMust(t, guards.close())
			astTestMust(t, conclusions.close())
		}
	}
}

func TestSMTStringVariableConcatWitnessReuseAfterPop(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "type1-reuse-x")
	y := stringContainmentWord(t, theory, "type1-reuse-y")
	a := stringContainmentWord(t, theory, "type1-reuse-a")
	b := stringContainmentWord(t, theory, "type1-reuse-b")
	left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
	astTestMust(t, state.PushScope())
	astTestMust(t, theory.simplifyStringConcatEquality(left, right))
	variables := theory.arrangementVars[[2]ast.ASTHandle{left, right}]
	astTestMust(t, state.PopScopes(1))
	if state.context.enodes.get(variables[0]) != nil || theory.hasSelfCut(variables[0], a) {
		t.Fatal("popped residual enode or scoped cut origins survived")
	}
	before := theory.freshID
	astTestMust(t, theory.simplifyStringConcatEquality(right, left))
	if theory.freshID != before || len(theory.arrangementVars) != 1 || state.context.enodes.get(variables[0]) == nil || state.context.enodes.get(variables[1]) == nil {
		t.Fatal("reverse-key reuse failed to refresh the native retained witnesses", theory.freshID, before)
	}
	for _, variable := range variables[:2] {
		if _, retained := theory.internalVariables[variable]; !retained {
			t.Fatal("native witness membership was incorrectly removed on pop")
		}
	}
}

func TestSMTStringVariableConcatOverlapAlternatives(t *testing.T) {
	for _, finite := range []bool{false, true} {
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
		x := stringContainmentWord(t, theory, "type1-overlap-x")
		y := stringContainmentWord(t, theory, "type1-overlap-y")
		a := stringContainmentWord(t, theory, "type1-overlap-a")
		b := stringContainmentWord(t, theory, "type1-overlap-b")
		for _, variable := range []ast.ASTHandle{x, y, a, b} {
			astTestMust(t, theory.initializeStringCut(variable))
		}
		astTestMust(t, theory.mergeStringCut(a, 0, y))
		left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
		astTestMust(t, theory.simplifyStringConcatEquality(left, right))
		if !theory.loopDetected || len(state.theoryCaseSplitSets) != 1 || len(state.theoryCaseSplitSets[0]) != 3 {
			t.Fatal("overlap did not replace just the looping cut option", finite)
		}
		literal := state.theoryCaseSplitSets[0][0]
		term := state.boolExpr[literal.Var()]
		if finite {
			if state.theoryAwareBranching.priorities[literal.Var()] != -0.1 {
				t.Fatal("finite overlap priority was not retained")
			}
			variable, err := m.GetArg(term, 0)
			astTestMust(t, err)
			yes, err := m.GetArg(term, 1)
			astTestMust(t, err)
			text, constant, err := m.SequenceStringValue(yes)
			astTestMust(t, err)
			if !constant || text != "yes" {
				t.Fatal("finite overlap did not select its yes tester")
			}
			emptyEq, err := stringEqAtom(m, variable, astTestString(t, m, ""))
			astTestMust(t, err)
			astTestPin(t, m, emptyEq)
			choice, err := m.MkOr(emptyEq, term)
			astTestMust(t, err)
			astTestPin(t, m, choice)
			stringConcatPairHasAxiom(t, theory, 0, choice)
		} else {
			overlap, err := m.RefGet(theory.overlapAssumption)
			astTestMust(t, err)
			if term != overlap {
				t.Fatal("looping option lost the actual per-check overlap assumption")
			}
		}
	}
}

func TestSMTStringCutHistoryNativeBoundaryAndRoots(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "cut-boundary-x")
	y := stringContainmentWord(t, theory, "cut-boundary-y")
	z := stringContainmentWord(t, theory, "cut-boundary-z")
	for _, variable := range []ast.ASTHandle{x, y, z} {
		astTestMust(t, theory.initializeStringCut(variable))
	}
	astTestMust(t, state.PushScope())
	astTestMust(t, theory.mergeStringCut(x, 1, y))
	astTestMust(t, state.PushScope())
	astTestMust(t, theory.mergeStringCut(x, 2, z))
	if !theory.hasSelfCut(x, y) || !theory.hasSelfCut(x, z) {
		t.Fatal("higher-level cut union failed")
	}
	astTestMust(t, state.PopScopes(1))
	// Source removes frames with level >= the new level, not just frames
	// introduced strictly above it. Both levels 2 and 1 must disappear.
	if theory.hasSelfCut(x, y) || theory.hasSelfCut(x, z) || !theory.hasSelfCut(x, x) {
		t.Fatal("cut rollback did not retain only the -1 seed at the source boundary")
	}
	astTestMust(t, state.PopScopes(1))
	astTestMust(t, theory.mergeStringCut(x, 0, y))
	astTestMust(t, state.PushScope())
	astTestMust(t, theory.mergeStringCut(x, 1, z))
	astTestMust(t, state.PopScopes(1))
	if !theory.hasSelfCut(x, y) || theory.hasSelfCut(x, z) {
		t.Fatal("native root-level cut frame failed to survive pop")
	}
}

func TestSMTStringOverlapClassificationAndPairSelection(t *testing.T) {
	for family := 1; family <= 6; family++ {
		m := astTestManager(t)
		theory, _ := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "overlap-family-x")
		y := stringContainmentWord(t, theory, "overlap-family-y")
		a := stringContainmentWord(t, theory, "overlap-family-a")
		b := stringContainmentWord(t, theory, "overlap-family-b")
		s, text := astTestString(t, m, "s"), astTestString(t, m, "st")
		left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
		origin, cut := a, y
		switch family {
		case 2:
			right = stringEqualityConcat(t, m, a, s)
		case 3:
			right = stringEqualityConcat(t, m, s, b)
			origin, cut = x, b
		case 4:
			left, right = stringEqualityConcat(t, m, s, y), stringEqualityConcat(t, m, text, b)
			origin, cut = y, b
		case 5:
			left, right = stringEqualityConcat(t, m, x, s), stringEqualityConcat(t, m, a, text)
			origin, cut = x, a
		case 6:
			left, right = stringEqualityConcat(t, m, s, y), stringEqualityConcat(t, m, a, text)
		}
		for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
			overlap, err := theory.stringConcatOverlap(pair[0], pair[1])
			astTestMust(t, err)
			if overlap {
				t.Fatal("fresh independent concat variables were marked overlapping", family)
			}
		}
		astTestMust(t, theory.mergeStringCut(origin, 0, cut))
		for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
			overlap, err := theory.stringConcatOverlap(pair[0], pair[1])
			astTestMust(t, err)
			if overlap != (family != 4 && family != 5) {
				t.Fatal("source overlap classification lost an orientation or constant-boundary exemption", family, overlap)
			}
		}
	}
	m := astTestManager(t)
	theory, _ := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "pair-selection-x")
	y := stringContainmentWord(t, theory, "pair-selection-y")
	a := stringContainmentWord(t, theory, "pair-selection-a")
	b := stringContainmentWord(t, theory, "pair-selection-b")
	u := stringContainmentWord(t, theory, "pair-selection-u")
	v := stringContainmentWord(t, theory, "pair-selection-v")
	first := stringEqualityConcat(t, m, x, y)
	second := stringEqualityConcat(t, m, u, v)
	right := stringEqualityConcat(t, m, a, b)
	astTestMust(t, theory.initializeStringCut(a))
	astTestMust(t, theory.initializeStringCut(y))
	astTestMust(t, theory.mergeStringCut(a, 0, y))
	astTestMust(t, theory.simplifyStringConcatGroups(smtStringClassGroups{concats: []ast.ASTHandle{first, second}}, smtStringClassGroups{concats: []ast.ASTHandle{first, right}}))
	if len(theory.arrangementVars) != 0 {
		t.Fatal("shared concat groups should skip the whole step")
	}
	astTestMust(t, theory.simplifyStringConcatGroups(smtStringClassGroups{concats: []ast.ASTHandle{first, second}}, smtStringClassGroups{concats: []ast.ASTHandle{right}}))
	if _, selected := theory.arrangementVars[[2]ast.ASTHandle{second, right}]; !selected || len(theory.arrangementVars) != 1 {
		t.Fatal("pair selection did not skip the earlier overlapping concat")
	}
}

func TestSMTStringKnownConcatOverlapRetainsLengthPremise(t *testing.T) {
	for _, cutLeft := range []bool{false, true} {
		m := astTestManager(t)
		theory, state, _ := stringMixedEqualityFixture(t, m)
		assumptions, err := theory.AddTheoryAssumptions(nil)
		astTestMust(t, err)
		astTestPin(t, m, assumptions[0])
		x := stringContainmentWord(t, theory, "known-overlap-x")
		y := stringContainmentWord(t, theory, "known-overlap-y")
		a := stringContainmentWord(t, theory, "known-overlap-a")
		b := stringContainmentWord(t, theory, "known-overlap-b")
		left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
		lx, la := int64(1), int64(2)
		cutter, cut := a, y
		if cutLeft {
			lx, la, cutter, cut = la, lx, x, b
		}
		stringSetLength(t, m, state, x, big.NewInt(lx).String())
		stringSetLength(t, m, state, a, big.NewInt(la).String())
		for _, variable := range []ast.ASTHandle{x, y, a, b} {
			astTestMust(t, theory.initializeStringCut(variable))
		}
		astTestMust(t, theory.mergeStringCut(cutter, 0, cut))
		astTestMust(t, theory.simplifyStringConcatEquality(left, right))
		guard := smtStringLengthPremises{theory: theory}
		equality, err := stringEqAtom(m, left, right)
		astTestMust(t, err)
		astTestMust(t, guard.add(equality))
		astTestMust(t, guard.length(x, big.NewInt(lx)))
		astTestMust(t, guard.length(a, big.NewInt(la)))
		premise, err := m.MkAndArray(guard.values)
		astTestMust(t, err)
		astTestPin(t, m, premise)
		negative, err := m.MkNot(premise)
		astTestMust(t, err)
		astTestPin(t, m, negative)
		overlap, err := m.RefGet(theory.overlapAssumption)
		astTestMust(t, err)
		want, err := m.MkOr(negative, overlap)
		astTestMust(t, err)
		astTestPin(t, m, want)
		stringConcatPairHasAxiom(t, theory, 0, want)
		if !theory.loopDetected || len(state.theoryCaseSplitSets) != 0 {
			t.Fatal("known looping arrangement was turned into an unconditional split", cutLeft)
		}
		astTestMust(t, guard.close())
	}
}

func TestSMTStringConcatResimplificationBridgeGuards(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "resimplify-bridge-x")
	y := stringContainmentWord(t, theory, "resimplify-bridge-y")
	a := stringContainmentWord(t, theory, "resimplify-bridge-a")
	b := stringContainmentWord(t, theory, "resimplify-bridge-b")
	p, pq := astTestString(t, m, "p"), astTestString(t, m, "pq")
	// An unattached empty seed is intentionally preserved by native
	// mk_concat. Attach it so both rebuilds have these canonical shapes.
	empty := astTestString(t, m, "")
	for _, expression := range []ast.ASTHandle{p, pq, empty} {
		astTestMust(t, state.Internalize(expression, false))
	}
	theory.find.Merge(uint32(state.context.enodes.get(x).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(p).TheoryVar(theory.FamilyID())))
	theory.find.Merge(uint32(state.context.enodes.get(a).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(pq).TheoryVar(theory.FamilyID())))
	left, right := stringEqualityConcat(t, m, x, y), stringEqualityConcat(t, m, a, b)
	astTestMust(t, theory.simplifyStringConcatEquality(left, right))
	newLeft, newRight := stringEqualityConcat(t, m, p, y), stringEqualityConcat(t, m, pq, b)
	guards := smtStringLengthPremises{theory: theory}
	for _, pair := range [][2]ast.ASTHandle{{left, newLeft}, {right, newRight}, {left, right}} {
		astTestMust(t, guards.equate(pair[0], pair[1]))
	}
	guard, err := m.MkAndArray(guards.values)
	astTestMust(t, err)
	astTestPin(t, m, guard)
	negative, err := m.MkNot(guard)
	astTestMust(t, err)
	astTestPin(t, m, negative)
	equality, err := stringEqAtom(m, newLeft, newRight)
	astTestMust(t, err)
	astTestPin(t, m, equality)
	want, err := m.MkOr(negative, equality)
	astTestMust(t, err)
	astTestPin(t, m, want)
	stringConcatPairHasAxiom(t, theory, 0, want)
	astTestMust(t, guards.close())
}
