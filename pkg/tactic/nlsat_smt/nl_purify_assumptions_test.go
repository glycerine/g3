package nlsatsmt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/params"
)

func nlModelEvaluatorFixture(t *testing.T, m *ast.ASTManager) (*model.Model, *model.ModelEvaluator, model.ModelEvaluatorConfig) {
	t.Helper()
	context, err := params.NewParamsContext(m.Symbols, m.NumeralService(), params.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	catalog, err := params.NewGlobalParamsCatalog(m.Symbols, params.GlobalParamsCatalogConfig{Budget: m.Budget()})
	astTestMust(t, err)
	globals, err := params.NewGlobalParamsService(m.Symbols, context, params.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, globals.InitializeSource())
	previous, err := params.BindProcessGlobalParamsService(globals)
	astTestMust(t, err)
	mem, err := memory.NewRuntime(memory.RuntimeConfig{Budget: m.Budget()})
	astTestMust(t, err)
	coop, err := cooperate.NewCooperation(m.Budget())
	astTestMust(t, err)
	md, err := model.NewModel(m)
	astTestMust(t, err)
	config := model.ModelEvaluatorConfig{Context: context, Globals: globals, Memory: mem, Cooperation: coop}
	ev, err := model.NewModelEvaluator(md.Core(), config)
	astTestMust(t, err)
	t.Cleanup(func() {
		_, err := params.BindProcessGlobalParamsService(previous)
		astTestMust(t, err)
		astTestMust(t, ev.Close())
		astTestMust(t, md.Close())
		astTestMust(t, coop.Close())
		astTestMust(t, mem.Close())
		astTestMust(t, globals.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, context.Close())
	})
	return md, ev, config
}

func TestNLPurifyModelAssumptionsRespectPolarity(t *testing.T) {
	m := astTestManager(t)
	md, ev, _ := nlModelEvaluatorFixture(t, m)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	preds := make([]ast.ASTHandle, 7)
	for i, name := range []string{"pos-false", "pos-true", "neg-true", "neg-false", "dual-true", "dual-false", "unknown"} {
		preds[i] = analysisConst(t, m, boolean, name)
		if i < 6 {
			decl, err := m.GetDecl(preds[i])
			astTestMust(t, err)
			value := falsehood
			if i == 1 || i == 2 || i == 4 {
				value = truth
			}
			astTestMust(t, md.Core().RegisterConstant(decl, value))
		}
	}
	pols := map[ast.ASTHandle]polarity{preds[0]: positive, preds[1]: positive, preds[2]: negative, preds[3]: negative, preds[4]: dual, preds[5]: dual, preds[6]: dual}
	values, err := updateNLEqualityValues(m, ev, []ast.ASTHandle{preds[1], preds[0], preds[6]})
	astTestMust(t, err)
	if values[0] != lbool.True || values[1] != lbool.False || values[2] != lbool.Undef {
		t.Fatal("equality values", values)
	}
	ctx := analysisConst(t, m, boolean, "context-assumption")
	asms, err := makeNLAssumptions(m, ev, []ast.ASTHandle{ctx}, preds, pols, []ast.ASTHandle{preds[1], preds[0], preds[6]}, values)
	astTestMust(t, err)
	defer func() { astTestMust(t, asms.Close()) }()
	n0, err := m.MkNot(preds[0])
	astTestMust(t, err)
	n5, err := m.MkNot(preds[5])
	astTestMust(t, err)
	want := []ast.ASTHandle{ctx, n0, preds[2], preds[4], n5, preds[1], n0}
	if len(asms.literals) != len(want) {
		t.Fatal("assumption count", len(asms.literals), len(want))
	}
	for i := range want {
		if asms.literals[i] != want[i] {
			t.Fatal("assumption", i, asms.literals[i], want[i])
		}
	}
}
