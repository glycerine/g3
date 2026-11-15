package tactic

// Source: src/test/horn_subsume_model_converter.cpp.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func hornTestPredicate(t *testing.T, m *ast.ASTManager, name string, arity uint32) ast.ASTHandle {
	t.Helper()
	b, err := m.MkBoolSort()
	satOK(t, err)
	domain := make([]ast.ASTHandle, arity)
	for i := range domain {
		domain[i] = b
	}
	d, err := m.MkFuncDecl(astTestSymbol(t, m, name), domain, b, ast.ASTFuncDeclInfo{})
	satOK(t, err)
	astTestPin(t, m, d)
	return d
}

func TestHornSubsumeMkHornRepeatedVariablesAndClauseRecognition(t *testing.T) {
	m, _, _, config := extensionFixture(t)
	c, err := NewHornSubsumeModelConverter(m, config)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, c.Close()) })
	p := hornTestPredicate(t, m, "horn-p", 2)
	q := hornTestPredicate(t, m, "horn-q", 2)
	b, err := m.MkBoolSort()
	satOK(t, err)
	v0, err := m.MkVar(0, b)
	satOK(t, err)
	v1, err := m.MkVar(1, b)
	satOK(t, err)
	head, err := m.MkApp(p, []ast.ASTHandle{v0, v0})
	satOK(t, err)
	body, err := m.MkApp(q, []ast.ASTHandle{v1, v0})
	satOK(t, err)
	astTestPin(t, m, head)
	astTestPin(t, m, body)
	predicate, converted, ok, err := c.MkHorn(head, body)
	satOK(t, err)
	if !ok || predicate != p {
		t.Fatal("Horn head was not recognized")
	}
	convertedBody, err := m.RefGet(converted)
	satOK(t, err)
	if kind, e := m.GetKind(convertedBody); e != nil || kind != ast.ASTQuantifier {
		t.Fatal("free body variable was not existentially bound", kind, e)
	}
	free, err := m.HasFreeVars(convertedBody)
	satOK(t, err)
	if !free {
		t.Fatal("predicate arguments were not retained as free interpretation variables")
	}
	declarations, err := m.GetNumDeclarations(convertedBody)
	satOK(t, err)
	if declarations != 1 {
		t.Fatal("body-only variable was not the sole existential", declarations)
	}
	satOK(t, m.DelRef(&converted))

	implication, err := m.MkImplies(body, head)
	satOK(t, err)
	clause, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b, b}, Names: []usymbol.Symbol{usymbol.Numerical(1), usymbol.Numerical(0)}, Body: implication})
	satOK(t, err)
	astTestPin(t, m, clause)
	predicate, converted, ok, err = c.MkHornClause(clause)
	satOK(t, err)
	if !ok || predicate != p {
		t.Fatal("closed forall implication was not recognized")
	}
	satOK(t, m.DelRef(&converted))
	yes, err := m.MkTrue()
	satOK(t, err)
	if _, empty, accepted, e := c.MkHornClause(yes); e != nil || accepted || empty != (ast.ASTRef{}) {
		t.Fatal("non-Horn clause accepted", accepted, e)
	}
}

func TestHornSubsumeApplyDefaultsAndReverseDisjunction(t *testing.T) {
	m, md, out, config := extensionFixture(t)
	c, err := NewHornSubsumeModelConverter(m, config)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, c.Close()) })
	p := hornTestPredicate(t, m, "horn-apply-p", 1)
	q := hornTestPredicate(t, m, "horn-apply-q", 0)
	qApp, err := m.MkApp(q, nil)
	satOK(t, err)
	yes, err := m.MkTrue()
	satOK(t, err)
	satOK(t, c.InsertDefinition(p, qApp))
	satOK(t, c.InsertDefinition(p, yes))
	satOK(t, c.ApplyUnindexed(out))
	no, err := m.MkFalse()
	satOK(t, err)
	qValue, err := md.Core().ConstantInterpretation(q)
	satOK(t, err)
	if qValue != no {
		t.Fatal("missing body predicate did not receive false", qValue)
	}
	pValue, err := md.Core().FunctionInterpretation(p)
	satOK(t, err)
	if pValue == nil {
		t.Fatal("predicate graph was not installed")
	}
	fallback, err := pValue.Else()
	satOK(t, err)
	if fallback != yes {
		t.Fatal("reverse updates did not preserve disjunction", fallback)
	}
	if got, err := out.Get(); err != nil || got != md.Core() {
		t.Fatal("converter unexpectedly replaced model", err)
	}
}

func TestHornSubsumeDelayedTranslationAndInheritedOperations(t *testing.T) {
	from, _, _, config := extensionFixture(t)
	c, err := NewHornSubsumeModelConverter(from, config)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, c.Close()) })
	p := hornTestPredicate(t, from, "horn-translate-p", 0)
	q := hornTestPredicate(t, from, "horn-translate-q", 0)
	pApp, err := from.MkApp(p, nil)
	satOK(t, err)
	qApp, err := from.MkApp(q, nil)
	satOK(t, err)
	yes, err := from.MkTrue()
	satOK(t, err)
	satOK(t, c.InsertDefinition(p, yes))
	satOK(t, c.Insert(pApp, qApp))
	var display bytes.Buffer
	satOK(t, c.Display(&display))
	if display.Len() != 0 {
		t.Fatal("source converter unexpectedly displayed text")
	}
	satOK(t, c.ApplyLabels(nil, 9))
	satOK(t, c.Cancel())

	to, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: from.Symbols, Numerals: from.NumeralService(), Budget: from.Budget()})
	satOK(t, err)
	t.Cleanup(func() { satOK(t, to.Close()) })
	translator, err := ast.NewASTTranslation(from, to, true)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, translator.Close()) })
	translatedBase, err := c.Translate(translator)
	satOK(t, err)
	translated := translatedBase.(*HornSubsumeModelConverter)
	t.Cleanup(func() { satOK(t, translated.Close()) })
	if len(translated.funcs) != 1 || len(translated.bodies) != 1 || len(translated.delayedHeads) != 0 || len(translated.delayedBodies) != 0 {
		t.Fatal("translation did not preserve the source immediate-only rule")
	}
	want, err := translator.Translate(p)
	satOK(t, err)
	if translated.funcs[0] != want {
		t.Fatal("translated declaration mismatch")
	}
}

// AUDIT.md tactic_top_01-F7: add_default_false_interpretation uses
// for_each_expr, a post-order walk with arguments left to right.
func TestHornSubsumeDefaultFalseRegistrationOrder(t *testing.T) {
	m, md, _, config := extensionFixture(t)
	c, err := NewHornSubsumeModelConverter(m, config)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, c.Close()) })
	a := hornTestPredicate(t, m, "horn-order-a", 1)
	b := hornTestPredicate(t, m, "horn-order-b", 1)
	boolSort, err := m.MkBoolSort()
	satOK(t, err)
	x, err := m.MkVar(0, boolSort)
	satOK(t, err)
	ax, err := m.MkApp(a, []ast.ASTHandle{x})
	satOK(t, err)
	bx, err := m.MkApp(b, []ast.ASTHandle{x})
	satOK(t, err)
	body, err := m.MkAnd(ax, bx)
	satOK(t, err)
	astTestPin(t, m, body)
	satOK(t, c.addDefaultFalseInterpretations(body, md.Core()))
	n, err := md.Core().NumDeclarations()
	satOK(t, err)
	if n != 2 {
		t.Fatal("default-false declarations", n)
	}
	for i, want := range []ast.ASTHandle{a, b} {
		got, err := md.Core().Declaration(uint32(i))
		satOK(t, err)
		if got != want {
			t.Fatalf("declaration %d: got %v want %v", i, got, want)
		}
	}
}

// AUDIT.md tactic_top_01-F7: mk_horn builds body_expr with the raw
// ast_manager::mk_and, so a conjunctive body is not flattened:
// (and (and q r) (= v true)).
func TestHornSubsumeMkHornUsesRawAnd(t *testing.T) {
	m, _, _, config := extensionFixture(t)
	c, err := NewHornSubsumeModelConverter(m, config)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, c.Close()) })
	p := hornTestPredicate(t, m, "horn-raw-p", 1)
	q := hornTestPredicate(t, m, "horn-raw-q", 0)
	r := hornTestPredicate(t, m, "horn-raw-r", 0)
	yes, err := m.MkTrue()
	satOK(t, err)
	head, err := m.MkApp(p, []ast.ASTHandle{yes})
	satOK(t, err)
	astTestPin(t, m, head)
	qApp, err := m.MkApp(q, nil)
	satOK(t, err)
	rApp, err := m.MkApp(r, nil)
	satOK(t, err)
	body, err := m.MkAnd(qApp, rApp)
	satOK(t, err)
	astTestPin(t, m, body)
	_, converted, ok, err := c.MkHorn(head, body)
	satOK(t, err)
	if !ok {
		t.Fatal("Horn head was not recognized")
	}
	t.Cleanup(func() { satOK(t, m.DelRef(&converted)) })
	got, err := m.RefGet(converted)
	satOK(t, err)
	boolSort, err := m.MkBoolSort()
	satOK(t, err)
	v, err := m.MkVar(0, boolSort)
	satOK(t, err)
	eq, err := m.MkEq(v, yes)
	satOK(t, err)
	want, err := m.MkAnd(body, eq)
	satOK(t, err)
	if got != want {
		t.Fatal("mk_horn body is not the raw conjunction", got, want)
	}
}
