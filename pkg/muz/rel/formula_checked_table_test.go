package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestFormulaCheckedTableFactsAndReset(t *testing.T) {
	f := newTableRelationFixture(t)
	context, err := uparams.NewParamsContext(f.ast.Symbols, f.ast.NumeralService(), uparams.ParamsConfig{Budget: f.ast.Budget()})
	astTestMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(f.ast.Symbols, uparams.GlobalParamsCatalogConfig{Budget: f.ast.Budget()})
	astTestMust(t, err)
	globals, err := uparams.NewGlobalParamsService(f.ast.Symbols, context, uparams.GlobalParamsConfig{Budget: f.ast.Budget(), Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, globals.InitializeSource())
	t.Cleanup(func() {
		astTestMust(t, globals.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, context.Close())
	})
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(table, smtcore.SMTSolverConfig{Params: context, Globals: globals})
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
	// An underlying mutation without the wrapper makes its recorded formula
	// disagree with emptiness; the solver must find that counterexample.
	astTestMust(t, table.Reset())
	if _, err = checked.Empty(); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed inconsistent empty relation", err)
	}
	astTestMust(t, checked.Reset())
	if empty, err := checked.Empty(); err != nil || !empty {
		t.Fatal("reset", empty, err)
	}
}
