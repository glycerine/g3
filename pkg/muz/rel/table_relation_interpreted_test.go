package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func tableRelationTheoryRewriter(t *testing.T, manager *ast.ASTManager) *astrewriter.TheoryRewriter {
	t.Helper()
	context, err := uparams.NewParamsContext(manager.Symbols, manager.NumeralService(), uparams.ParamsConfig{Budget: manager.Budget()})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := uparams.NewGlobalParamsCatalog(manager.Symbols, uparams.GlobalParamsCatalogConfig{Budget: manager.Budget()})
	if err != nil {
		t.Fatal(err)
	}
	globals, err := uparams.NewGlobalParamsService(manager.Symbols, context, uparams.GlobalParamsConfig{Budget: manager.Budget(), Catalog: catalog})
	if err != nil {
		t.Fatal(err)
	}
	if err = globals.InitializeSource(); err != nil {
		t.Fatal(err)
	}
	rewriter, err := astrewriter.NewTheoryRewriter(manager, context, globals, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rewriter.Close(); err != nil {
			t.Error(err)
		}
		if err := globals.Close(); err != nil {
			t.Error(err)
		}
		if err := catalog.Close(); err != nil {
			t.Error(err)
		}
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	return rewriter
}

func TestTableRelationInterpretedBooleanFilter(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	relation, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, relation)
	_, _ = relation.AddTableFact(Fact{0, 1})
	_, _ = relation.AddTableFact(Fact{1, 2})
	variable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	if err = relation.FilterInterpreted(rewriter, []ast.ASTHandle{variable}, map[uint32]int{0: 0}, map[uint32]ast.ASTHandle{0: f.boolean}); err != nil {
		t.Fatal(err)
	}
	requireTableFact(t, relation, Fact{1, 2}, true)
	if relation.NumRows() != 1 {
		t.Fatal("rows", relation.NumRows())
	}
}
