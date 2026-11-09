package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func stringContainmentWord(t *testing.T, theory *SMTStringTheory, name string) ast.ASTHandle {
	t.Helper()
	m := theory.state.manager
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	word, err := m.MkConst(astTestSymbol(t, m, name), sort)
	astTestMust(t, err)
	astTestMust(t, theory.state.Internalize(word, false))
	// set_up_axioms gives a string variable its theory var (mk_var) at
	// init_search_eh; internalization alone does not.
	_, err = theory.mkVar(theory.state.context.enodes.get(word))
	astTestMust(t, err)
	return word
}

func stringContainmentRegister(t *testing.T, theory *SMTStringTheory, haystack, needle ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	relation, err := theory.state.manager.MkSequenceContains(haystack, needle)
	astTestMust(t, err)
	astTestMust(t, theory.state.Internalize(relation, true))
	astTestMust(t, theory.instantiateContains(theory.state.context.enodes.get(relation)))
	return relation
}

func TestSMTStringContainmentLiteralTraversalSkipsBinders(t *testing.T) {
	m := astTestManager(t)
	theory, _ := stringTheoryFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, m, variable)
	body, err := m.MkEq(variable, astTestString(t, m, "hidden"))
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "literal-bound")}, Body: body})
	astTestMust(t, err)
	astTestPin(t, m, quantifier)
	a, b := astTestString(t, m, "a"), astTestString(t, m, "b")
	choice, err := m.MkIte(quantifier, a, variable)
	astTestMust(t, err)
	expression := stringEqualityConcat(t, m, choice, b, a)
	constants, err := theory.stringConstantsInNode(expression)
	astTestMust(t, err)
	if len(constants) != 3 || constants[0] != a || constants[1] != b || constants[2] != a {
		t.Fatal("literal traversal crossed a binder, lost duplicates or changed left-first order")
	}
}

func TestSMTStringContainmentPairConstantDirections(t *testing.T) {
	for _, equalHaystacks := range []bool{false, true} {
		for _, texts := range [][2]string{{"ab", "a"}, {"a", "ab"}, {"a", "a"}, {"a", "b"}} {
			m, config := smtModelGeneratorFixture(t)
			theory, _ := stringTheoryFixture(t, m)
			x := stringContainmentWord(t, theory, "pair-common-left")
			y := stringContainmentWord(t, theory, "pair-common-right")
			a, b := astTestString(t, m, texts[0]), astTestString(t, m, texts[1])
			var first, second ast.ASTHandle
			if equalHaystacks {
				first = stringContainmentRegister(t, theory, x, a)
				second = stringContainmentRegister(t, theory, y, b)
			} else {
				first = stringContainmentRegister(t, theory, a, x)
				second = stringContainmentRegister(t, theory, b, y)
			}
			before := len(theory.refs)
			astTestMust(t, theory.checkContainByEqualNodes(x, y))
			if texts == [2]string{"a", "b"} {
				if len(theory.refs) != before {
					t.Fatal("incomparable constants produced a containment implication")
				}
				continue
			}
			if len(theory.refs) != before+1 {
				t.Fatal("constant pair did not produce its unique native rule")
			}
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
			astTestMust(t, err)
			for _, same := range []bool{false, true} {
				for _, f := range []bool{false, true} {
					for _, s := range []bool{false, true} {
						fv, err := m.MkBoolVal(f)
						astTestMust(t, err)
						sv, err := m.MkBoolVal(s)
						astTestMust(t, err)
						yText := "y"
						if same {
							yText = "x"
						}
						forward := texts[0] == "ab"
						if !equalHaystacks {
							forward = !forward
						}
						conclusion := !f || s
						if !forward {
							conclusion = !s || f
						}
						if texts[0] == texts[1] {
							conclusion = f == s
						}
						got := stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, "x"), y: astTestString(t, m, yText), first: fv, second: sv})
						if got != (!same || conclusion) {
							t.Fatal("constant pair direction/guard changed", equalHaystacks, texts, same, f, s)
						}
					}
				}
			}
			astTestMust(t, rw.Close())
		}
	}
}

func TestSMTStringContainmentPairSymbolicClassGuards(t *testing.T) {
	for _, equalHaystacks := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			m, config := smtModelGeneratorFixture(t)
			theory, state := stringTheoryFixture(t, m)
			x := stringContainmentWord(t, theory, "pair-symbolic-x")
			y := stringContainmentWord(t, theory, "pair-symbolic-y")
			a := stringContainmentWord(t, theory, "pair-symbolic-a")
			b := stringContainmentWord(t, theory, "pair-symbolic-b")
			aa := stringContainmentWord(t, theory, "pair-symbolic-aa")
			bb := stringContainmentWord(t, theory, "pair-symbolic-bb")
			var first, second ast.ASTHandle
			if equalHaystacks {
				first = stringContainmentRegister(t, theory, x, a)
				second = stringContainmentRegister(t, theory, y, b)
			} else {
				first = stringContainmentRegister(t, theory, a, x)
				second = stringContainmentRegister(t, theory, b, y)
			}
			var condition ast.ASTHandle
			if reverse {
				condition = stringContainmentRegister(t, theory, bb, aa)
			} else {
				condition = stringContainmentRegister(t, theory, aa, bb)
			}
			// Establish only private-class membership. The emitted theorem
			// must still guard every alias equality explicitly.
			for _, pair := range [][2]ast.ASTHandle{{a, aa}, {b, bb}} {
				theory.find.Merge(uint32(state.context.enodes.get(pair[0]).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(pair[1]).TheoryVar(theory.FamilyID())))
			}
			before := len(theory.refs)
			astTestMust(t, theory.checkContainByEqualNodes(x, y))
			if len(theory.refs) != before+1 {
				t.Fatal("registered class bridge did not produce one implication")
			}
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
			astTestMust(t, err)
			// Independently falsify each of the three equality guards and
			// the registered relation, then enumerate both conclusion bits.
			for disabled := -1; disabled < 4; disabled++ {
				for _, f := range []bool{false, true} {
					for _, s := range []bool{false, true} {
						fv, err := m.MkBoolVal(f)
						astTestMust(t, err)
						sv, err := m.MkBoolVal(s)
						astTestMust(t, err)
						cv, err := m.MkBoolVal(disabled != 3)
						astTestMust(t, err)
						values := map[ast.ASTHandle]ast.ASTHandle{first: fv, second: sv, condition: cv}
						for i, pair := range [][2]ast.ASTHandle{{x, y}, {a, aa}, {b, bb}} {
							values[pair[0]] = astTestString(t, m, "same")
							text := "same"
							if disabled == i {
								text = "different"
							}
							values[pair[1]] = astTestString(t, m, text)
						}
						forward := equalHaystacks != reverse
						conclusion := !f || s
						if !forward {
							conclusion = !s || f
						}
						if got := stringAxiomTruth(t, m, rw, axiom, values); got != (disabled >= 0 || conclusion) {
							t.Fatal("symbolic relation direction or equality evidence lost", equalHaystacks, reverse, disabled, f, s)
						}
					}
				}
			}
			astTestMust(t, rw.Close())
		}
	}
}

func TestSMTStringContainmentNonconstantSubstringAndPremiseLifetime(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "substring-x")
	y := stringContainmentWord(t, theory, "substring-y")
	needle := stringContainmentWord(t, theory, "substring-needle")
	tail := stringContainmentWord(t, theory, "substring-tail")
	first := stringContainmentRegister(t, theory, x, needle)
	second := stringContainmentRegister(t, theory, y, needle)
	constant := astTestString(t, m, "aaa")
	astTestMust(t, state.Internalize(constant, false))
	for _, variable := range []ast.ASTHandle{x, y} {
		theory.find.Merge(uint32(state.context.enodes.get(variable).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(constant).TheoryVar(theory.FamilyID())))
	}
	concat := stringEqualityConcat(t, m, astTestString(t, m, "b"), tail)
	astTestMust(t, state.Internalize(concat, false))
	before := len(theory.refs)
	astTestMust(t, theory.checkContainBySubstring(needle, []ast.ASTHandle{needle, concat}))
	if len(theory.refs) != before+2 {
		t.Fatal("nonconstant needle did not inspect both registered haystacks")
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	// Native check_contain_by_substr intentionally keeps its vector across
	// entries, so the second refusal retains the first haystack's premise.
	for disabled := -1; disabled < 3; disabled++ {
		for _, assigned := range []bool{false, true} {
			bit, err := m.MkBoolVal(assigned)
			astTestMust(t, err)
			values := map[ast.ASTHandle]ast.ASTHandle{first: bit, second: bit, tail: astTestString(t, m, "")}
			for i, word := range []ast.ASTHandle{x, y, needle} {
				text := "aaa"
				if i == 2 {
					text = "b"
				}
				if disabled == i {
					text = "different"
				}
				values[word] = astTestString(t, m, text)
			}
			if got := stringAxiomTruth(t, m, rw, axiom, values); got != (disabled >= 0 || !assigned) {
				t.Fatal("nonconstant substring premise accumulation changed", disabled, assigned)
			}
		}
	}
	// The full pre-merge path must reach the no-constant branch, too.
	before = len(theory.refs)
	astTestMust(t, theory.checkContainNewEquality(needle, concat))
	if len(theory.refs) <= before {
		t.Fatal("nonconstant substring stage was not connected to the merge checker")
	}
}

func TestSMTStringContainmentEqualClassAndReflexiveRules(t *testing.T) {
	for _, equalHaystacks := range []bool{false, true} {
		m, config := smtModelGeneratorFixture(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "congruence-x")
		y := stringContainmentWord(t, theory, "congruence-y")
		a := stringContainmentWord(t, theory, "congruence-a")
		b := stringContainmentWord(t, theory, "congruence-b")
		var first, second ast.ASTHandle
		if equalHaystacks {
			first = stringContainmentRegister(t, theory, x, a)
			second = stringContainmentRegister(t, theory, y, b)
		} else {
			first = stringContainmentRegister(t, theory, a, x)
			second = stringContainmentRegister(t, theory, b, y)
		}
		theory.find.Merge(uint32(state.context.enodes.get(a).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(b).TheoryVar(theory.FamilyID())))
		before := len(theory.refs)
		astTestMust(t, theory.checkContainByEqualNodes(x, y))
		if len(theory.refs) != before+1 {
			t.Fatal("private opposite-argument equality did not produce congruence")
		}
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for disabled := -1; disabled < 2; disabled++ {
			for _, f := range []bool{false, true} {
				for _, s := range []bool{false, true} {
					fv, err := m.MkBoolVal(f)
					astTestMust(t, err)
					sv, err := m.MkBoolVal(s)
					astTestMust(t, err)
					values := map[ast.ASTHandle]ast.ASTHandle{first: fv, second: sv}
					for i, pair := range [][2]ast.ASTHandle{{x, y}, {a, b}} {
						values[pair[0]] = astTestString(t, m, "same")
						text := "same"
						if disabled == i {
							text = "different"
						}
						values[pair[1]] = astTestString(t, m, text)
					}
					if got := stringAxiomTruth(t, m, rw, axiom, values); got != (disabled >= 0 || f == s) {
						t.Fatal("class congruence lost a guard")
					}
				}
			}
		}
		astTestMust(t, rw.Close())
	}
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "reflexive-x")
	y := stringContainmentWord(t, theory, "reflexive-y")
	relation := stringContainmentRegister(t, theory, x, y)
	astTestMust(t, theory.checkContainByEqualNodes(x, y))
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	kind, err := m.GetDeclKind(axiom)
	astTestMust(t, err)
	if kind != ast.ASTOpOr {
		t.Fatal("native implication must be represented as not-premise or conclusion")
	}
	// A separately registered self relation is a unit, without equality.
	self := stringContainmentRegister(t, theory, y, y)
	astTestMust(t, theory.containmentReflexive(y, y, self))
	literal, err := state.Literal(self)
	astTestMust(t, err)
	assigned, err := state.Assignment(literal)
	astTestMust(t, err)
	if assigned != lbool.True {
		t.Fatal("reflexive containment was not asserted", relation)
	}
}

func TestSMTStringContainmentMonotonicityCoreMergeAndPop(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "monotonicity-x")
	y := stringContainmentWord(t, theory, "monotonicity-y")
	first := stringContainmentRegister(t, theory, x, astTestString(t, m, "ab"))
	second := stringContainmentRegister(t, theory, y, astTestString(t, m, "a"))
	for _, assignment := range []struct {
		expression ast.ASTHandle
		positive   bool
	}{{first, true}, {second, false}} {
		literal, err := state.Literal(assignment.expression)
		astTestMust(t, err)
		if !assignment.positive {
			literal.Negate()
		}
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	}
	astTestMust(t, state.PushScope())
	equality, err := m.MkEq(x, y)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	literal, err := state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if !state.Inconsistent() {
		t.Fatal("equal haystacks failed to propagate containment from a larger needle to its substring")
	}
	astTestMust(t, state.PopScopes(1))
	if state.Inconsistent() {
		t.Fatal("monotonicity conflict survived pop")
	}
	astTestMust(t, state.PushScope())
	// The parent assignments are compatible when the words differ.
	for _, pair := range [][2]ast.ASTHandle{{x, astTestString(t, m, "ab")}, {y, astTestString(t, m, "b")}} {
		equality, err := m.MkEq(pair[0], pair[1])
		astTestMust(t, err)
		astTestMust(t, state.Internalize(equality, true))
		literal, err := state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
	}
	if state.Inconsistent() {
		t.Fatal("guarded monotonicity theorem poisoned a compatible later branch")
	}
	astTestMust(t, state.PopScopes(1))
}
