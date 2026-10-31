package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func formulaCheckerConfig(t *testing.T, m *ast.ASTManager) smtcore.SMTSolverConfig {
	t.Helper()
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	astTestMust(t, err)
	globals, err := uparams.NewGlobalParamsService(m.Symbols, context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, globals.InitializeSource())
	t.Cleanup(func() {
		astTestMust(t, globals.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, context.Close())
	})
	return smtcore.SMTSolverConfig{Params: context, Globals: globals}
}

func TestFormulaCheckedTableFactsAndReset(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(table, config)
	astTestMust(t, err)
	defer func() { astTestMust(t, checked.Close()) }()
	if empty, err := checked.Empty(); err != nil || !empty {
		t.Fatal("initial empty", empty, err)
	}
	added, err := checked.AddFact([]ast.ASTHandle{f.truth})
	if err != nil || !added {
		t.Fatal("first fact", added, err)
	}
	added, err = checked.AddFact([]ast.ASTHandle{f.truth})
	if err != nil || added {
		t.Fatal("duplicate fact", added, err)
	}
	if empty, err := checked.Empty(); err != nil || empty {
		t.Fatal("populated relation", empty, err)
	}
	if found, err := checked.ContainsFact([]ast.ASTHandle{f.truth}); err != nil || !found {
		t.Fatal("checked membership", found, err)
	}
	if found, err := checked.ContainsFact([]ast.ASTHandle{f.falsehood}); err != nil || found {
		t.Fatal("checked non-membership", found, err)
	}
	clone, err := checked.Clone()
	astTestMust(t, err)
	defer func() { astTestMust(t, clone.Close()) }()
	// An underlying mutation without the wrapper makes its recorded formula
	// disagree with emptiness; the solver must find that counterexample.
	astTestMust(t, table.Reset())
	if _, err = checked.ContainsFact([]ast.ASTHandle{f.truth}); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed inconsistent membership", err)
	}
	if found, err := clone.ContainsFact([]ast.ASTHandle{f.truth}); err != nil || !found {
		t.Fatal("clone changed with source", found, err)
	}
	if _, err = checked.Empty(); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed inconsistent empty relation", err)
	}
	astTestMust(t, checked.Reset())
	if empty, err := checked.Empty(); err != nil || !empty {
		t.Fatal("reset", empty, err)
	}
}

func TestFormulaCheckedTableFilters(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(table, config)
	astTestMust(t, err)
	defer func() { astTestMust(t, checked.Close()) }()
	for _, fact := range [][]ast.ASTHandle{
		{f.truth, f.truth}, {f.truth, f.falsehood}, {f.falsehood, f.falsehood},
	} {
		_, err := checked.AddFact(fact)
		astTestMust(t, err)
	}
	astTestMust(t, checked.FilterIdentical([]int{0, 1}))
	if table.NumRows() != 2 {
		t.Fatal("identical filter", table.NumRows())
	}
	astTestMust(t, checked.FilterEqual(f.truth, 0))
	if table.NumRows() != 1 {
		t.Fatal("equal filter", table.NumRows())
	}
	variable, err := f.ast.MkVar(1, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, variable)
	condition, err := f.ast.MkEq(variable, f.truth)
	astTestMust(t, err)
	astTestPin(t, f.ast, condition)
	theory := tableRelationTheoryRewriter(t, f.ast)
	astTestMust(t, checked.FilterInterpreted(theory, condition))
	if table.NumRows() != 1 {
		t.Fatal("interpreted filter", table.NumRows())
	}
	astTestMust(t, table.Reset())
	if err = checked.FilterIdentical([]int{0, 1}); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed filter counterexample", err)
	}
}
