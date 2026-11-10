package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderAUFAnalyzerVisitsBooleanFormulasInsideTerms(t *testing.T) {
	m, _, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	b, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	conditional, err := m.MkIte(b, zero, one)
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "analyzer-nested-p", integer, boolean)
	body, err := m.MkApp(p, []ast.ASTHandle{conditional})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "analyzer-b")}, Body: body})
	astTestMust(t, err)
	graph := newSMTModelFinderAUFGraph(manager.modelFinder)
	t.Cleanup(func() { astTestMust(t, graph.close()) })
	astTestMust(t, aufTestBuild(t, graph, []ast.ASTHandle{q}))
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	set := graph.uvars[smtModelFinderAUFKey{q, 0}].root().set
	if len(set.testElements()) != 2 {
		t.Fatal("nested Boolean condition omitted one polarity", set.testElements())
	}
	for _, value := range []ast.ASTHandle{truth, falsehood} {
		if generation, found := set.testElements()[value]; !found || generation != 0 {
			t.Fatal("nested Boolean condition instantiation", value, set.testElements())
		}
	}
}

func TestSMTModelFinderAUFAnalyzerDeduplicatesSharedQInfo(t *testing.T) {
	m, _, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	left, err := m.MkArithmeticAdd(x, one)
	astTestMust(t, err)
	right, err := m.MkArithmeticAdd(one, x)
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "analyzer-duplicate-p", integer, boolean)
	first, err := m.MkApp(p, []ast.ASTHandle{left})
	astTestMust(t, err)
	second, err := m.MkApp(p, []ast.ASTHandle{right})
	astTestMust(t, err)
	body, err := m.MkAnd(first, second)
	astTestMust(t, err)
	// Reuse the Boolean DAG at multiple depths and polarities.
	for i := 0; i < 12; i++ {
		body, err = m.MkIff(body, body)
		astTestMust(t, err)
	}
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "analyzer-x")}, Body: body})
	astTestMust(t, err)
	graph := newSMTModelFinderAUFGraph(manager.modelFinder)
	t.Cleanup(func() { astTestMust(t, graph.close()) })
	astTestMust(t, graph.collectFVars(q))
	if len(graph.fvars) != 1 || graph.fvars[0].offset.IsNull() {
		t.Fatal("shared normalized offset produced duplicate qinfo", graph.fvars)
	}
	if len(graph.analysisSeen) > 40 {
		t.Fatal("analysis did not cache shared Boolean subgraphs", len(graph.analysisSeen))
	}
}

// aufTestBuild registers each quantifier (register_quantifier runs the
// analyzer that fills its qinfos) before building the auf_solver graph.
func aufTestBuild(t *testing.T, graph *smtModelFinderAUFGraph, quantifiers []ast.ASTHandle) error {
	t.Helper()
	for _, quantifier := range quantifiers {
		if _, found := graph.finder.registered[quantifier]; found {
			continue
		}
		if err := graph.finder.RegisterQuantifier(quantifier); err != nil {
			return err
		}
	}
	return graph.build(quantifiers)
}
