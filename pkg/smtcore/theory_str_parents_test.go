package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func stringParentPattern(t *testing.T, m *ast.ASTManager, pattern int, x, y ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	b := astTestString(t, m, "b")
	ab, ba := astTestString(t, m, "ab"), astTestString(t, m, "ba")
	concat := func(left, right ast.ASTHandle) ast.ASTHandle {
		return stringEqualityConcat(t, m, left, right)
	}
	switch pattern {
	case 1:
		return concat(x, b), ab
	case 2:
		return concat(b, x), ba
	case 3:
		return concat(x, concat(b, y)), concat(ab, y)
	case 4:
		return concat(concat(y, b), x), concat(y, ba)
	case 5:
		return concat(concat(y, x), b), concat(y, ab)
	case 6:
		return concat(b, concat(x, y)), concat(ba, y)
	}
	t.Fatal("unknown native parent pattern")
	return ast.ASTHandle{}, ast.ASTHandle{}
}

func TestSMTStringParentSixPatternsGuarded(t *testing.T) {
	for pattern := 1; pattern <= 6; pattern++ {
		m, config := smtModelGeneratorFixture(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "parent-pattern-x")
		y := stringContainmentWord(t, theory, "parent-pattern-y")
		parent, simplified := stringParentPattern(t, m, pattern, x, y)
		astTestMust(t, state.Internalize(parent, false))
		constant := astTestString(t, m, "a")
		astTestMust(t, state.Internalize(constant, false))
		before := len(theory.refs)
		astTestMust(t, theory.simplifyStringParents(x, constant))
		guard, err := m.MkEq(x, constant)
		astTestMust(t, err)
		astTestPin(t, m, guard)
		negative, err := m.MkNot(guard)
		astTestMust(t, err)
		astTestPin(t, m, negative)
		conclusion, err := m.MkEq(parent, simplified)
		astTestMust(t, err)
		astTestPin(t, m, conclusion)
		want, err := m.MkOr(negative, conclusion)
		astTestMust(t, err)
		astTestPin(t, m, want)
		found := false
		for _, ref := range theory.refs[before:] {
			axiom, err := m.RefGet(ref)
			astTestMust(t, err)
			found = found || axiom == want
		}
		if !found {
			t.Fatal("native guarded parent pattern was not emitted", pattern)
		}
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for _, left := range []string{"a", "c", "", "\x00\xff"} {
			for _, right := range []string{"", "a", "b", "\xff"} {
				if !stringAxiomTruth(t, m, rw, want, map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, left), y: astTestString(t, m, right)}) {
					t.Fatal("parent rewrite escaped its equality premise or reversed bytes", pattern, left, right)
				}
			}
		}
		astTestMust(t, rw.Close())
	}
}

func TestSMTStringParentCoreCallbackAndRollback(t *testing.T) {
	// Separate contexts prove the callback itself works, without any earlier
	// helper-generated lemmas making a missing callback appear connected.
	for pattern := 1; pattern <= 6; pattern++ {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "core-parent-x")
		y := stringContainmentWord(t, theory, "core-parent-y")
		parent, simplified := stringParentPattern(t, m, pattern, x, y)
		astTestMust(t, state.Internalize(parent, false))
		astTestMust(t, state.PushScope())
		equality, err := m.MkEq(x, astTestString(t, m, "a"))
		astTestMust(t, err)
		astTestMust(t, state.Internalize(equality, true))
		literal, err := state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		if state.Inconsistent() || state.context.enodes.get(simplified) == nil || state.context.enodes.get(parent).Root() != state.context.enodes.get(simplified).Root() {
			t.Fatal("core callback missed native parent simplification", pattern)
		}
		astTestMust(t, state.PopScopes(1))
		if _, known, err := theory.eqcStringValue(x); err != nil || known || state.Inconsistent() {
			t.Fatal("parent callback value or conflict survived pop", pattern, known, err)
		}
	}
}

func TestSMTStringParentSiblingClassPremiseAndConcatReplacement(t *testing.T) {
	for _, rightSide := range []bool{false, true} {
		m, config := smtModelGeneratorFixture(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "parent-guard-x")
		y := stringContainmentWord(t, theory, "parent-guard-y")
		a, b := astTestString(t, m, "a"), astTestString(t, m, "b")
		first, second, text := x, y, "ab"
		if rightSide {
			first, second, text = y, x, "ba"
		}
		parent := stringEqualityConcat(t, m, first, second)
		for _, expression := range []ast.ASTHandle{parent, a, b} {
			astTestMust(t, state.Internalize(expression, false))
		}
		theory.find.Merge(uint32(state.context.enodes.get(y).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(b).TheoryVar(theory.FamilyID())))
		astTestMust(t, theory.simplifyStringParents(x, a))
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		conclusion, err := m.MkEq(parent, astTestString(t, m, text))
		astTestMust(t, err)
		astTestPin(t, m, conclusion)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for _, xv := range []string{"a", "c"} {
			for _, yv := range []string{"b", "d"} {
				for _, result := range []bool{false, true} {
					bit, err := m.MkBoolVal(result)
					astTestMust(t, err)
					values := map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, xv), y: astTestString(t, m, yv), conclusion: bit}
					if got := stringAxiomTruth(t, m, rw, axiom, values); got != (xv != "a" || yv != "b" || result) {
						t.Fatal("parent evaluation lost a pending/sibling equality premise", rightSide, xv, yv, result)
					}
				}
			}
		}
		astTestMust(t, rw.Close())
	}
	// When eval_concat cannot decide, a concat-valued member can still be
	// replaced in its parent under the pending equality alone.
	for _, rightSide := range []bool{false, true} {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "nested-member-x")
		y := stringContainmentWord(t, theory, "nested-member-y")
		z := stringContainmentWord(t, theory, "nested-member-z")
		member := stringEqualityConcat(t, m, x, z)
		constant := astTestString(t, m, "ab")
		parent := stringEqualityConcat(t, m, member, y)
		simplified := stringEqualityConcat(t, m, constant, y)
		if rightSide {
			parent = stringEqualityConcat(t, m, y, member)
			simplified = stringEqualityConcat(t, m, y, constant)
		}
		for _, expression := range []ast.ASTHandle{parent, constant} {
			astTestMust(t, state.Internalize(expression, false))
		}
		astTestMust(t, theory.simplifyStringParents(member, constant))
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		guard, err := m.MkEq(member, constant)
		astTestMust(t, err)
		astTestPin(t, m, guard)
		negative, err := m.MkNot(guard)
		astTestMust(t, err)
		astTestPin(t, m, negative)
		conclusion, err := m.MkEq(parent, simplified)
		astTestMust(t, err)
		want, err := m.MkOr(negative, conclusion)
		astTestMust(t, err)
		if axiom != want {
			t.Fatal("concat-member fallback omitted or changed its pending guard", rightSide)
		}
	}
}

func TestSMTStringEvalConcatAttachmentAndEmpty(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	a, b := astTestString(t, m, "a"), astTestString(t, m, "b")
	if value, err := theory.evalStringConcat(a, b); err != nil || !value.IsNull() {
		t.Fatal("eval_concat used unattached syntactic literals as class values", value, err)
	}
	for _, expression := range []ast.ASTHandle{a, b} {
		astTestMust(t, state.Internalize(expression, false))
	}
	value, err := theory.evalStringConcat(a, b)
	astTestMust(t, err)
	if text, literal, err := m.SequenceStringValue(value); err != nil || !literal || text != "ab" {
		t.Fatal("attached class evaluation changed bytes", text, literal, err)
	}
	x := stringContainmentWord(t, theory, "eval-unknown")
	empty := astTestString(t, m, "")
	astTestMust(t, state.Internalize(empty, false))
	for _, pair := range [][2]ast.ASTHandle{{x, empty}, {empty, x}} {
		if value, err := theory.evalStringConcat(pair[0], pair[1]); err != nil || value != x {
			t.Fatal("empty class constant did not return the original opposite operand", value, err)
		}
	}
}

func TestSMTStringInitialConcatShortcutPrecedesConsistency(t *testing.T) {
	for _, sharedFirst := range []bool{false, true} {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		x := stringContainmentWord(t, theory, "shortcut-x")
		y := stringContainmentWord(t, theory, "shortcut-y")
		z := stringContainmentWord(t, theory, "shortcut-z")
		left, right := stringEqualityConcat(t, m, x, z), stringEqualityConcat(t, m, y, z)
		if sharedFirst {
			left, right = stringEqualityConcat(t, m, z, x), stringEqualityConcat(t, m, z, y)
		}
		for _, expression := range []ast.ASTHandle{left, right} {
			astTestMust(t, state.Internalize(expression, false))
		}
		shortcut, err := theory.stringConcatEqualityShortcut(left, right)
		astTestMust(t, err)
		if shortcut {
			t.Fatal("syntactically shared operand alone triggered early return")
		}
		theory.find.Merge(uint32(state.context.enodes.get(x).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(y).TheoryVar(theory.FamilyID())))
		// A consistency scan would emit containment congruence here.
		// The native initial shortcut must return before that scan.
		stringContainmentRegister(t, theory, left, astTestString(t, m, "a"))
		stringContainmentRegister(t, theory, right, astTestString(t, m, "a"))
		refs, accesses := len(theory.refs), theory.cacheAccesses
		astTestMust(t, theory.NewEquality(state.context.enodes.get(left).TheoryVar(theory.FamilyID()), state.context.enodes.get(right).TheoryVar(theory.FamilyID())))
		if len(theory.refs) != refs || theory.cacheAccesses != accesses {
			t.Fatal("initial concat shortcut ran the later equality stages")
		}
		if same, err := theory.sameStringClass(left, right); err != nil || !same {
			t.Fatal("source post-shortcut private merge was omitted", same, err)
		}
	}
}
