package opt

// Source: src/opt/pb_sls.cpp/.h.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func pbSLSFixture(t *testing.T) (*ast.ASTManager, PBSLSConfig) {
	t.Helper()
	budget := arena.NewBudget(1 << 29)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	numbers, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.RegisterPseudoBooleanPlugin(); err != nil {
		t.Fatal(err)
	}
	context, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	if err != nil {
		t.Fatal(err)
	}
	if err = globals.InitializeSource(); err != nil {
		t.Fatal(err)
	}
	cooperation, err := cooperate.NewCooperation(budget)
	if err != nil {
		t.Fatal(err)
	}
	local := context.EmptyValue()
	config := PBSLSConfig{Context: context, Globals: globals, Model: model.ModelEvaluatorConfig{Context: context, Globals: globals, Params: &local, Memory: numbers.RuntimeOwner(), Cooperation: cooperation}}
	t.Cleanup(func() {
		if err := cooperation.Close(); err != nil {
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
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
		if err := numbers.Close(); err != nil {
			t.Error(err)
		}
		if err := symbols.Close(); err != nil {
			t.Error(err)
		}
	})
	return manager, config
}

func pbSLSBool(t *testing.T, manager *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	sort, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	symbol, err := manager.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	value, err := manager.MkConst(symbol, sort)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func pbSLSModel(t *testing.T, manager *ast.ASTManager, values map[ast.ASTHandle]bool) *model.Model {
	t.Helper()
	answer, err := model.NewModel(manager)
	if err != nil {
		t.Fatal(err)
	}
	truth, _ := manager.MkTrue()
	falsity, _ := manager.MkFalse()
	for expression, value := range values {
		decl, err := manager.GetDecl(expression)
		if err != nil {
			t.Fatal(err)
		}
		interpretation := falsity
		if value {
			interpretation = truth
		}
		if err = answer.Core().RegisterConstant(decl, interpretation); err != nil {
			t.Fatal(err)
		}
	}
	return answer
}

func TestPBSLSBooleanHardSoftSearchStatisticsAndModel(t *testing.T) {
	manager, config := pbSLSFixture(t)
	p, q := pbSLSBool(t, manager, "pb-sls-p"), pbSLSBool(t, manager, "pb-sls-q")
	hard, err := manager.MkOr(p, q)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []ast.ASTHandle{p, q, hard} {
		if err = manager.IncRef(value); err != nil {
			t.Fatal(err)
		}
		defer manager.DecRef(value)
	}
	initial := pbSLSModel(t, manager, map[ast.ASTHandle]bool{p: false, q: true})
	defer initial.Close()
	solver, err := NewPBSLS(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.SetModel(initial); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddHard(hard); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoft(p, big.NewRat(2, 1)); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoft(q, big.NewRat(1, 1)); err != nil {
		t.Fatal(err)
	}
	if holds, err := solver.SoftHolds(0); err != nil || holds {
		t.Fatal("initial soft p", holds, err)
	}
	status, err := solver.Check()
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	penalty, err := solver.Penalty()
	if err != nil || penalty.Sign() != 0 {
		t.Fatal("best penalty", penalty, err)
	}
	stats, err := solver.Statistics()
	if err != nil || stats["sls.num_flips"] == 0 || stats["sls.num_improvements"] == 0 {
		t.Fatal("source statistics", stats, err)
	}
}

func TestPBSLSPseudoBooleanCompileNegationAndOwnedModel(t *testing.T) {
	manager, config := pbSLSFixture(t)
	p, q := pbSLSBool(t, manager, "pb-ge-p"), pbSLSBool(t, manager, "pb-ge-q")
	service := manager.NumeralService()
	coefficientA, _ := service.NewValue()
	coefficientB, _ := service.NewValue()
	bound, _ := service.NewValue()
	if err := service.SetInt64(&coefficientA, 2); err != nil {
		t.Fatal(err)
	}
	if err := service.SetInt64(&coefficientB, 3); err != nil {
		t.Fatal(err)
	}
	if err := service.SetInt64(&bound, 3); err != nil {
		t.Fatal(err)
	}
	ge, err := manager.MkPseudoBooleanGE([]unumeral.OwnedRational{coefficientA, coefficientB}, []ast.ASTHandle{p, q}, &bound)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Del(&coefficientA); err != nil {
		t.Fatal(err)
	}
	if err = service.Del(&coefficientB); err != nil {
		t.Fatal(err)
	}
	if err = service.Del(&bound); err != nil {
		t.Fatal(err)
	}
	notGE, err := manager.MkNot(ge)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []ast.ASTHandle{p, q, ge, notGE} {
		if err = manager.IncRef(value); err != nil {
			t.Fatal(err)
		}
		defer manager.DecRef(value)
	}
	initial := pbSLSModel(t, manager, map[ast.ASTHandle]bool{p: false, q: false})
	defer initial.Close()
	solver, err := NewPBSLS(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.SetModel(initial); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddHard(notGE); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(); err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	got, err := solver.GetModel()
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	for _, expression := range []ast.ASTHandle{p, q} {
		decl, err := manager.GetDecl(expression)
		if err != nil {
			t.Fatal(err)
		}
		value, err := got.Core().ConstantInterpretation(decl)
		if err != nil || !manager.IsFalse(value) {
			t.Fatal("owned model", value, err)
		}
	}
}
