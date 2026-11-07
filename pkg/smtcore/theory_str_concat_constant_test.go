package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSMTStringConcatConstantCoreCancellationAndEmpty(t *testing.T) {
	for _, mode := range []string{"prefix", "suffix", "empty"} {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "concat-cancel-x")
		y := stringContainmentWord(t, theory, "concat-cancel-y")
		first, second, text, want := astTestString(t, m, "a"), x, "ab", "b"
		if mode == "suffix" {
			first, second, want = x, astTestString(t, m, "b"), "a"
		} else if mode == "empty" {
			first, second, text, want = x, y, "", ""
		}
		concat := stringEqualityConcat(t, m, first, second)
		astTestMust(t, state.PushScope())
		equality, err := m.MkEq(concat, astTestString(t, m, text))
		astTestMust(t, err)
		astTestMust(t, state.Internalize(equality, true))
		literal, err := state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		value := astTestString(t, m, want)
		if state.context.enodes.get(value) == nil || state.context.enodes.get(x).Root() != state.context.enodes.get(value).Root() {
			t.Fatal("constant concat cancellation was not propagated through the core", mode)
		}
		if mode == "empty" && state.context.enodes.get(y).Root() != state.context.enodes.get(value).Root() {
			t.Fatal("empty concat did not force both operands empty")
		}
		astTestMust(t, state.PopScopes(1))
		if _, known, err := theory.eqcStringValue(x); err != nil || known {
			t.Fatal("constant cancellation survived pop", known, err)
		}
	}
}

func TestSMTStringConcatConstantArrangementTruthTablesAndScope(t *testing.T) {
	for _, heuristic := range []bool{false, true} {
		m, config := smtModelGeneratorFixture(t)
		parameters, err := NewSMTStrParams(config.Context, config.Params, config.Globals, m.Budget())
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, parameters.Close()) })
		key := "str.strong_arrangements"
		astTestMust(t, config.Context.SetBoolName(config.Params, &key, !heuristic))
		astTestMust(t, parameters.Update(config.Params))
		theory, state := stringTheoryFixture(t, m)
		theory.parameterOwner = parameters
		state.theoryCaseSplit = heuristic
		x := stringContainmentWord(t, theory, "concat-split-x")
		y := stringContainmentWord(t, theory, "concat-split-y")
		concat := stringEqualityConcat(t, m, x, y)
		astTestMust(t, state.Internalize(concat, false))
		constant := astTestString(t, m, "ab")
		astTestMust(t, state.Internalize(constant, false))
		astTestMust(t, state.PushScope())
		before := len(theory.refs)
		astTestMust(t, theory.solveConcatConstant(concat, constant))
		if len(theory.refs) <= before || theory.freshID != 1 || len(theory.concatBreakFlags) != 1 {
			t.Fatal("unknown-length arrangement or native cached xor flag was omitted")
		}
		if heuristic && len(state.theoryCaseSplitSets) != 1 {
			t.Fatal("concat arrangements did not reach the core case-split consumer")
		}
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		kind, err := m.GetDeclKind(axiom)
		astTestMust(t, err)
		if (kind == ast.ASTOpOr) != heuristic {
			t.Fatal("borrowed strong-arrangements parameter did not select equivalence versus implication")
		}
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for _, left := range []string{"", "a", "b", "ab"} {
			for _, right := range []string{"", "a", "b", "ab"} {
				// The complete split disjunction equals the word equality
				// under concrete independent byte-string assignments.
				if !stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, left), y: astTestString(t, m, right)}) {
					t.Fatal("arrangement axiom excluded a concrete valuation", heuristic, left, right)
				}
			}
		}
		astTestMust(t, rw.Close())
		astTestMust(t, state.PopScopes(1))
		if len(state.theoryCaseSplitSets) != 0 || len(theory.concatBreakFlags) != 1 {
			t.Fatal("native scoped case split/persistent flag lifetimes diverged")
		}
		// The source flag lookup reuses either operand order even after pop.
		reverse := stringEqualityConcat(t, m, y, x)
		astTestMust(t, state.Internalize(reverse, false))
		astTestMust(t, theory.solveConcatConstant(reverse, constant))
		if theory.freshID != 1 {
			t.Fatal("reverse concat allocated another native xor flag")
		}
	}
}

func TestSMTStringConcatConstantKnownLengthGuards(t *testing.T) {
	for _, rightKnown := range []bool{false, true} {
		m, config := smtModelGeneratorFixture(t)
		theory, state, _ := stringMixedEqualityFixture(t, m)
		x := stringContainmentWord(t, theory, "known-split-x")
		y := stringContainmentWord(t, theory, "known-split-y")
		concat := stringEqualityConcat(t, m, x, y)
		astTestMust(t, state.Internalize(concat, false))
		chosen := x
		if rightKnown {
			chosen = y
		}
		length := stringSetLength(t, m, state, chosen, "1")
		constant := astTestString(t, m, "ab")
		astTestMust(t, state.Internalize(constant, false))
		astTestMust(t, theory.solveConcatConstant(concat, constant))
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		wordEq, err := m.MkEq(concat, constant)
		astTestMust(t, err)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for _, parent := range []bool{false, true} {
			for _, observed := range []string{"0", "1", "2"} {
				for _, correct := range []bool{false, true} {
					bit, err := m.MkBoolVal(parent)
					astTestMust(t, err)
					numeral, err := m.ArithmeticIntegerNumeralText(observed)
					astTestMust(t, err)
					left := "wrong"
					if correct {
						left = "a"
					}
					values := map[ast.ASTHandle]ast.ASTHandle{wordEq: bit, length: numeral, x: astTestString(t, m, left), y: astTestString(t, m, "b")}
					want := !parent || observed != "1" || correct
					if got := stringAxiomTruth(t, m, rw, axiom, values); got != want {
						t.Fatal("known split lost its parent/length guard or exact pieces", rightKnown, parent, observed, correct)
					}
				}
			}
		}
		astTestMust(t, rw.Close())
	}
}

func TestSMTStringConcatClassSimplificationKeepsAliasGuards(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "simplified-concat-x")
	y := stringContainmentWord(t, theory, "simplified-concat-y")
	concat := stringEqualityConcat(t, m, x, y)
	astTestMust(t, state.Internalize(concat, false))
	constant := astTestString(t, m, "a")
	astTestMust(t, state.Internalize(constant, false))
	astTestMust(t, state.PushScope())
	theory.find.Merge(uint32(state.context.enodes.get(x).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(constant).TheoryVar(theory.FamilyID())))
	simplified, err := theory.simplifyStringConcat(concat)
	astTestMust(t, err)
	leaves, err := theory.concatLeaves(simplified)
	astTestMust(t, err)
	// mk_concat consults attached class values, not syntactic literals.
	// Its newly created, uninternalized empty seed can remain a leaf.
	if len(leaves) != 3 || leaves[1] != constant || leaves[2] != y {
		t.Fatal("private-class concat simplification did not rebuild its leaves")
	}
	if text, literal, err := m.SequenceStringValue(leaves[0]); err != nil || !literal || text != "" {
		t.Fatal("native uninternalized empty seed was not preserved", text, literal, err)
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
	astTestMust(t, err)
	for _, left := range []string{"a", "b"} {
		for _, right := range []string{"", "a", "b"} {
			if !stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, left), y: astTestString(t, m, right)}) {
				t.Fatal("class substitution escaped its equality guard", left, right)
			}
		}
	}
	astTestMust(t, rw.Close())
	astTestMust(t, state.PopScopes(1))
	after, err := theory.simplifyStringConcat(concat)
	astTestMust(t, err)
	if after != concat {
		t.Fatal("concat simplification used an alias from a popped scope")
	}
}

func TestSMTStringConcatConstantRefusalAndLengthBounds(t *testing.T) {
	for _, observed := range []int64{-1, 3} {
		m, config := smtModelGeneratorFixture(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "split-bound-x")
		y := stringContainmentWord(t, theory, "split-bound-y")
		concat := stringEqualityConcat(t, m, x, y)
		astTestMust(t, state.Internalize(concat, false))
		equality, err := m.MkEq(concat, astTestString(t, m, "ab"))
		astTestMust(t, err)
		astTestPin(t, m, equality)
		astTestMust(t, theory.solveConcatKnownLength(equality, x, y, "ab", big.NewInt(observed), true, nil, false))
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		length, err := m.MkSequenceLength(x)
		astTestMust(t, err)
		astTestPin(t, m, length)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for _, parent := range []bool{false, true} {
			for _, value := range []int64{-1, 0, 1, 2, 3} {
				bit, err := m.MkBoolVal(parent)
				astTestMust(t, err)
				numeral, err := m.ArithmeticIntegerNumeralText(big.NewInt(value).String())
				astTestMust(t, err)
				want := value >= 0
				if observed > 0 {
					want = !parent || value <= 2
				}
				if got := stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{equality: bit, length: numeral}); got != want {
					t.Fatal("negative/overlong split bound has the wrong guard", observed, parent, value)
				}
			}
		}
		astTestMust(t, rw.Close())
	}
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "refused-split-x")
	y := stringContainmentWord(t, theory, "refused-split-y")
	left := stringEqualityConcat(t, m, astTestString(t, m, "a"), x)
	right := stringEqualityConcat(t, m, astTestString(t, m, "b"), y)
	concat := stringEqualityConcat(t, m, left, right)
	astTestMust(t, state.Internalize(concat, false))
	constant := astTestString(t, m, "ba")
	astTestMust(t, state.Internalize(constant, false))
	astTestMust(t, theory.solveConcatConstant(concat, constant))
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	equality, err := m.MkEq(concat, constant)
	astTestMust(t, err)
	negative, err := m.MkNot(equality)
	astTestMust(t, err)
	if axiom != negative {
		t.Fatal("an empty set of feasible arrangements did not reject the word equality")
	}
}
