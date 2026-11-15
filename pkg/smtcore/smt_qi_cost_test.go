package smtcore

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtQICostQuantifier(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "cost-x")},
		Body: variable, Weight: 7,
	})
	astTestMust(t, err)
	pattern, err := m.MkAnd(variable, variable)
	astTestMust(t, err)
	return quantifier, pattern
}

func TestSMTQICostModelPublishesSourceVariablesAndNewGeneration(t *testing.T) {
	m := astTestManager(t)
	quantifier, pattern := smtQICostQuantifier(t, m)
	model, err := NewSMTQICostModel(m,
		"(+ (+ (+ weight generation) (+ min_top_generation max_top_generation)) (+ pattern_width vars))",
		"(+ cost quant_generation)", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, model.Close()) })
	stat := &SMTQuantifierStat{generation: 11, size: 13, depth: 4, caseSplitFactor: 9}
	cost, err := model.Cost(quantifier, pattern, stat, 3, 2, 5, 6)
	astTestMust(t, err)
	if cost != 20 || stat.MaxCost() != 20 {
		t.Fatal("cost variable order or max update", cost, stat.MaxCost())
	}
	generation, err := model.NewGeneration(quantifier, stat, 3, 6, cost)
	astTestMust(t, err)
	if generation != 31 {
		t.Fatal("new generation variables", generation)
	}
}

func TestSMTQICostModelInvalidFunctionsUseSourceFallbacks(t *testing.T) {
	m := astTestManager(t)
	quantifier, pattern := smtQICostQuantifier(t, m)
	var warnings bytes.Buffer
	model, err := NewSMTQICostModel(m, "(", "(bad", diagnostic.NewWarningService(&warnings))
	astTestMust(t, err)
	// smtcore_05-F7: qi_queue::setup warns before switching to the defaults.
	for _, want := range []string{
		"WARNING: invalid cost function '(', switching to default one\n",
		"WARNING: invalid new_gen function '(bad', switching to default one\n",
	} {
		if !strings.Contains(warnings.String(), want) {
			t.Fatalf("missing warning %q in %q", want, warnings.String())
		}
	}
	t.Cleanup(func() { astTestMust(t, model.Close()) })
	stat := &SMTQuantifierStat{}
	cost, err := model.Cost(quantifier, pattern, stat, 3, 0, 0, 0)
	astTestMust(t, err)
	if cost != 10 {
		t.Fatal("default weight plus generation cost", cost)
	}
	generation, err := model.NewGeneration(quantifier, stat, 3, 0, cost)
	astTestMust(t, err)
	if generation != 10 {
		t.Fatal("default cost new-generation function", generation)
	}
}
