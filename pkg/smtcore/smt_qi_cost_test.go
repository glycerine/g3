package smtcore

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtQICostQuantifier(t testing.TB, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
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

func TestSMTQICostDefaultsMatchParsedExpressions(t *testing.T) {
	m := astTestManager(t)
	q, pat := smtQICostQuantifier(t, m)
	direct, err := NewSMTQICostModel(m, "(+ weight generation)", "cost", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, direct.Close()) })
	parsed, err := NewSMTQICostModel(m, "(+ generation weight)", "(+ cost 0)", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parsed.Close()) })

	var a, b SMTQuantifierStat
	for _, n := range []uint32{0, 1, 27, 1<<24 - 1, 1<<24 + 3} {
		got, err := direct.Cost(q, pat, &a, n, 0, 3, 4)
		astTestMust(t, err)
		want, err := parsed.Cost(q, pat, &b, n, 0, 3, 4)
		astTestMust(t, err)
		if got != want || a.MaxCost() != b.MaxCost() {
			t.Fatalf("generation %d produced cost %v, want %v", n, got, want)
		}
		g, err := direct.NewGeneration(q, &a, n, 4, got+0.5)
		astTestMust(t, err)
		w, err := parsed.NewGeneration(q, &b, n, 4, want+0.5)
		astTestMust(t, err)
		if g != w {
			t.Fatalf("cost %v produced generation %d, want %d", got+0.5, g, w)
		}
	}
	if _, err := direct.Cost(q, q, &a, 1, 0, 0, 0); err != ast.ErrASTDomain {
		t.Fatalf("quantifier used as a pattern returned %v", err)
	}
	if _, err := direct.NewGeneration(pat, &a, 1, 0, 3); err != ast.ErrASTDomain {
		t.Fatalf("application used as a quantifier returned %v", err)
	}
}

func BenchmarkSMTQICostDefaults(b *testing.B) {
	m := astTestManager(b)
	q, pat := smtQICostQuantifier(b, m)
	model, err := NewSMTQICostModel(m, "(+ weight generation)", "cost", nil)
	benchmarkMust(b, err)
	b.Cleanup(func() { benchmarkMust(b, model.Close()) })
	var stat SMTQuantifierStat
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := uint32(i & 255)
		cost, err := model.Cost(q, pat, &stat, n, 0, n, 0)
		if err != nil {
			b.Fatal(err)
		}
		g, err := model.NewGeneration(q, &stat, n, 0, cost)
		if err != nil || g != n+7 {
			b.Fatal(n, g, err)
		}
	}
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
