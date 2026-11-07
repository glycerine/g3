package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func stringAxiomTruth(t *testing.T, m *ast.ASTManager, rw *rewriter.TheoryRewriter, axiom ast.ASTHandle, values map[ast.ASTHandle]ast.ASTHandle) bool {
	t.Helper()
	replace, err := rewriter.NewExprSafeReplace(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, replace.Close()) }()
	for source, destination := range values {
		astTestMust(t, replace.Insert(source, destination))
	}
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	astTestMust(t, replace.Apply(axiom, out))
	expression, err := m.RefGet(out)
	astTestMust(t, err)
	astTestMust(t, rw.Rewrite(expression, out))
	expression, err = m.RefGet(out)
	astTestMust(t, err)
	if !m.IsTrue(expression) && !m.IsFalse(expression) {
		t.Fatal("fully concrete native relation axiom did not reduce to Boolean")
	}
	return m.IsTrue(expression)
}

func TestSMTStringBoundaryAxiomsConcretePartitions(t *testing.T) {
	for _, suffix := range []bool{false, true} {
		m, config := smtModelGeneratorFixture(t)
		theory, state := stringTheoryFixture(t, m)
		sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
		astTestMust(t, err)
		boundary, err := m.MkConst(astTestSymbol(t, m, "str-boundary-word"), sort)
		astTestMust(t, err)
		container, err := m.MkConst(astTestSymbol(t, m, "str-container-word"), sort)
		astTestMust(t, err)
		var relation ast.ASTHandle
		if suffix {
			relation, err = m.MkSequenceSuffix(boundary, container)
		} else {
			relation, err = m.MkSequencePrefix(boundary, container)
		}
		astTestMust(t, err)
		astTestMust(t, state.Internalize(relation, true))
		astTestMust(t, theory.instantiateBoundaryRelation(state.context.enodes.get(relation), suffix))
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		first, err := m.MkSequenceSkolem(astTestSymbol(t, m, "ts0!tmp0"), nil, sort)
		astTestMust(t, err)
		second, err := m.MkSequenceSkolem(astTestSymbol(t, m, "ts1!tmp1"), nil, sort)
		astTestMust(t, err)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, rw.Close()) })
		for _, b := range []string{"", "a", "b", "ab", "ba"} {
			for _, c := range []string{"", "a", "b", "ab", "ba"} {
				left, right := "", ""
				if len(b) <= len(c) {
					cut := len(b)
					if suffix {
						cut = len(c) - len(b)
					}
					left, right = c[:cut], c[cut:]
				}
				actual := strings.HasPrefix(c, b)
				if suffix {
					actual = strings.HasSuffix(c, b)
				}
				for _, assigned := range []bool{actual, !actual} {
					truth, err := m.MkBoolVal(assigned)
					astTestMust(t, err)
					values := map[ast.ASTHandle]ast.ASTHandle{boundary: astTestString(t, m, b), container: astTestString(t, m, c), first: astTestString(t, m, left), second: astTestString(t, m, right), relation: truth}
					if stringAxiomTruth(t, m, rw, axiom, values) != (assigned == actual) {
						t.Fatal("boundary axiom disagrees with concrete string semantics", suffix, b, c, assigned)
					}
				}
			}
		}
		before, freshID := len(theory.refs), theory.freshID
		astTestMust(t, theory.instantiateBoundaryRelation(state.context.enodes.get(relation), suffix))
		if len(theory.refs) != before || theory.freshID != freshID {
			t.Fatal("duplicate boundary setup created new axioms or witnesses")
		}
	}
}

func TestSMTStringContainsWitnessesAndIndices(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	theory, state := stringTheoryFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	haystack, err := m.MkConst(astTestSymbol(t, m, "str-contains-haystack"), sort)
	astTestMust(t, err)
	needle, err := m.MkConst(astTestSymbol(t, m, "str-contains-needle"), sort)
	astTestMust(t, err)
	relation, err := m.MkSequenceContains(haystack, needle)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(relation, true))
	astTestMust(t, theory.instantiateContains(state.context.enodes.get(relation)))
	key := [2]ast.ASTHandle{haystack, needle}
	if len(theory.contains) != 1 || theory.containsByPair[key] != relation {
		t.Fatal("symbolic containment was not registered")
	}
	for _, expression := range []ast.ASTHandle{haystack, needle} {
		if _, found := theory.containsIndex[expression][key]; !found {
			t.Fatal("containment pair missing from a reverse index")
		}
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	first, err := m.MkSequenceSkolem(astTestSymbol(t, m, "ts0!tmp0"), nil, sort)
	astTestMust(t, err)
	second, err := m.MkSequenceSkolem(astTestSymbol(t, m, "ts1!tmp1"), nil, sort)
	astTestMust(t, err)
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	for _, h := range []string{"", "a", "b", "aba"} {
		for _, n := range []string{"", "a", "b", "aa"} {
			left, right := "", ""
			actual := strings.Contains(h, n)
			if index := strings.Index(h, n); index >= 0 {
				left, right = h[:index], h[index+len(n):]
			}
			for _, assigned := range []bool{actual, !actual} {
				truth, err := m.MkBoolVal(assigned)
				astTestMust(t, err)
				values := map[ast.ASTHandle]ast.ASTHandle{haystack: astTestString(t, m, h), needle: astTestString(t, m, n), first: astTestString(t, m, left), second: astTestString(t, m, right), relation: truth}
				if stringAxiomTruth(t, m, rw, axiom, values) != (assigned == actual) {
					t.Fatal("containment witness equation disagrees with its concrete partition", h, n, assigned)
				}
			}
		}
	}
	before, freshID := len(theory.refs), theory.freshID
	astTestMust(t, theory.instantiateContains(state.context.enodes.get(relation)))
	if len(theory.refs) != before || theory.freshID != freshID || len(theory.contains) != 1 {
		t.Fatal("duplicate containment setup changed the registry or witnesses")
	}
}
