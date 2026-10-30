package spacer

// Source: src/muz/spacer/spacer_util.cpp normalize.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func normalizeMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func normalizePin(t *testing.T, manager *ast.ASTManager, value ast.ASTHandle) ast.ASTRef {
	t.Helper()
	ref, err := manager.NewRef(value)
	normalizeMust(t, err)
	t.Cleanup(func() { normalizeMust(t, manager.DelRef(&ref)) })
	return ref
}

func normalizeNumeral(t *testing.T, manager *ast.ASTManager, text string) ast.ASTHandle {
	t.Helper()
	service := manager.NumeralService()
	value, err := service.NewValue()
	normalizeMust(t, err)
	defer func() { normalizeMust(t, service.Del(&value)) }()
	normalizeMust(t, service.SetString(&value, text))
	result, err := manager.ArithmeticNumeral(&value, true)
	normalizeMust(t, err)
	normalizePin(t, manager, result)
	return result
}

func TestNormalizeFormulaCanonicalOrderAndBounds(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	var err error
	boolean, err := manager.MkBoolSort()
	normalizeMust(t, err)
	pName, err := manager.Symbols.Intern("normalize-p")
	normalizeMust(t, err)
	qName, err := manager.Symbols.Intern("normalize-q")
	normalizeMust(t, err)
	p, err := manager.MkConst(pName, boolean)
	normalizeMust(t, err)
	q, err := manager.MkConst(qName, boolean)
	normalizeMust(t, err)
	notP, err := manager.MkNot(p)
	normalizeMust(t, err)
	implication, err := manager.MkOr(notP, q)
	normalizeMust(t, err)
	formula, err := manager.MkAndArray([]ast.ASTHandle{implication, p})
	normalizeMust(t, err)
	result, err := normalizeFormula(manager, globals.Context, globals, formula, true, false)
	normalizeMust(t, err)
	defer func() { normalizeMust(t, manager.DelRef(&result)) }()
	handle, err := manager.RefGet(result)
	normalizeMust(t, err)
	parts, err := manager.FlattenAnd([]ast.ASTHandle{handle})
	normalizeMust(t, err)
	if len(parts) != 2 {
		t.Fatal("normalized conjunction size", len(parts))
	}
	foundP, foundQ := false, false
	for _, part := range parts {
		foundP = foundP || part == p
		foundQ = foundQ || part == q
	}
	if !foundP || !foundQ {
		t.Fatal("normalized bounds", foundP, foundQ)
	}
}

func TestNormalizeFormulaIntegerThresholds(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	var err error
	minusOne := normalizeNumeral(t, manager, "-1")
	zero := normalizeNumeral(t, manager, "0")
	integer, err := manager.GetSort(zero)
	normalizeMust(t, err)
	xName, err := manager.Symbols.Intern("normalize-x")
	normalizeMust(t, err)
	x, err := manager.MkConst(xName, integer)
	normalizeMust(t, err)
	family, err := manager.ArithmeticFamilyID()
	normalizeMust(t, err)
	leMinusOne, err := manager.MkFamilyApp(family, 2, nil, []ast.ASTHandle{x, minusOne})
	normalizeMust(t, err)
	result, err := normalizeFormula(manager, globals.Context, globals, leMinusOne, true, false)
	normalizeMust(t, err)
	defer func() { normalizeMust(t, manager.DelRef(&result)) }()
	geZero, err := manager.MkFamilyApp(family, 3, nil, []ast.ASTHandle{x, zero})
	normalizeMust(t, err)
	want, err := manager.MkNot(geZero)
	normalizeMust(t, err)
	got, err := manager.RefGet(result)
	normalizeMust(t, err)
	if got != want {
		t.Fatal("integer threshold normalization", got, want)
	}
}

// Source: spacer_util.cpp adhoc_rewriter_cfg runs through rewriter_tpl.
func TestNormalizeAdhocRewritesChildrenBeforeParentThreshold(t *testing.T) {
	m, _ := newSpacerTestServices(t)
	zero := normalizeNumeral(t, m, "0")
	minusOne := normalizeNumeral(t, m, "-1")
	integer, err := m.GetSort(zero)
	normalizeMust(t, err)
	var terms []ast.ASTHandle
	for _, text := range []string{"adhoc-nested-x", "adhoc-nested-y", "adhoc-nested-z"} {
		name, err := m.Symbols.Intern(text)
		normalizeMust(t, err)
		term, err := m.MkConst(name, integer)
		normalizeMust(t, err)
		terms = append(terms, term)
	}
	inner, err := m.MkArithmeticLE(terms[0], minusOne)
	normalizeMust(t, err)
	choice, err := m.MkIte(inner, terms[1], terms[2])
	normalizeMust(t, err)
	outer, err := m.MkArithmeticLE(choice, minusOne)
	normalizeMust(t, err)
	root, err := m.NewRef(outer)
	normalizeMust(t, err)
	defer func() { normalizeMust(t, m.DelRef(&root)) }()
	ge, err := m.MkArithmeticGE(terms[0], zero)
	normalizeMust(t, err)
	expectedInner, err := m.MkNot(ge)
	normalizeMust(t, err)
	expectedChoice, err := m.MkIte(expectedInner, terms[1], terms[2])
	normalizeMust(t, err)
	expectedGE, err := m.MkArithmeticGE(expectedChoice, zero)
	normalizeMust(t, err)
	expected, err := m.MkNot(expectedGE)
	normalizeMust(t, err)
	expectedRef, err := m.NewRef(expected)
	normalizeMust(t, err)
	defer func() { normalizeMust(t, m.DelRef(&expectedRef)) }()
	name, err := m.Symbols.Intern("adhoc-quantifier-metadata")
	normalizeMust(t, err)
	spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{name}, Weight: 7, QID: name, SkolemID: name, Body: outer, NoPatterns: []ast.ASTHandle{outer}}
	quantified, err := m.MkQuantifier(spec)
	normalizeMust(t, err)
	quantifiedRef, err := m.NewRef(quantified)
	normalizeMust(t, err)
	defer func() { normalizeMust(t, m.DelRef(&quantifiedRef)) }()
	spec.Body = expected
	spec.NoPatterns = []ast.ASTHandle{expected}
	expectedQuantified, err := m.MkQuantifier(spec)
	normalizeMust(t, err)
	expectedQuantifiedRef, err := m.NewRef(expectedQuantified)
	normalizeMust(t, err)
	defer func() { normalizeMust(t, m.DelRef(&expectedQuantifiedRef)) }()
	normalizeMust(t, normalizeAdhoc(m, root))
	actual, err := m.RefGet(root)
	normalizeMust(t, err)
	if actual != expected {
		t.Fatal("parent threshold replacement kept the old child", actual, expected)
	}
	normalizeMust(t, normalizeAdhoc(m, quantifiedRef))
	actualQuantified, err := m.RefGet(quantifiedRef)
	normalizeMust(t, err)
	if actualQuantified != expectedQuantified {
		t.Fatal("quantifier body/annotation rewrite or metadata changed", actualQuantified, expectedQuantified)
	}

}
