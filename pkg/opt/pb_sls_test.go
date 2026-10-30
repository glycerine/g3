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

func TestPBSLSBooleanEqualityAndIffCompilationMatchSource(t *testing.T) {
	manager, config := pbSLSFixture(t)
	p, q := pbSLSBool(t, manager, "pb-sls-eq-p"), pbSLSBool(t, manager, "pb-sls-eq-q")
	boolEq, err := manager.MkEq(p, q)
	if err != nil {
		t.Fatal(err)
	}
	iff, err := manager.MkIff(p, q)
	if err != nil {
		t.Fatal(err)
	}
	for _, expression := range []ast.ASTHandle{p, q, boolEq, iff} {
		if err = manager.IncRef(expression); err != nil {
			t.Fatal(err)
		}
		defer manager.DecRef(expression)
	}
	initial := pbSLSModel(t, manager, map[ast.ASTHandle]bool{p: true, q: false})
	defer initial.Close()
	solver, err := NewPBSLS(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.SetModel(initial); err != nil {
		t.Fatal(err)
	}
	if _, err = solver.makeLiteral(boolEq); err != nil {
		t.Fatal(err)
	}
	if len(solver.hard) != 4 {
		t.Fatalf("Boolean equality generated %d clauses, want four biconditional clauses", len(solver.hard))
	}
	if _, err = solver.makeLiteral(iff); err != nil {
		t.Fatal(err)
	}
	if len(solver.hard) != 8 {
		t.Fatalf("iff generated %d clauses total, want eight biconditional clauses", len(solver.hard))
	}
}

func TestPBSLSNonBooleanEqualityRemainsOneAtomicLiteral(t *testing.T) {
	manager, config := pbSLSFixture(t)
	valueSortName, err := manager.Symbols.Intern("pb-sls-value")
	if err != nil {
		t.Fatal(err)
	}
	valueSort, err := manager.MkUninterpretedSort(valueSortName, nil)
	if err != nil {
		t.Fatal(err)
	}
	xName, err := manager.Symbols.Intern("pb-sls-value-x")
	if err != nil {
		t.Fatal(err)
	}
	yName, err := manager.Symbols.Intern("pb-sls-value-y")
	if err != nil {
		t.Fatal(err)
	}
	x, err := manager.MkConst(xName, valueSort)
	if err != nil {
		t.Fatal(err)
	}
	y, err := manager.MkConst(yName, valueSort)
	if err != nil {
		t.Fatal(err)
	}
	equality, err := manager.MkEq(x, y)
	if err != nil {
		t.Fatal(err)
	}
	for _, expression := range []ast.ASTHandle{x, y, equality} {
		if err = manager.IncRef(expression); err != nil {
			t.Fatal(err)
		}
		defer manager.DecRef(expression)
	}
	initial, err := model.NewModel(manager)
	if err != nil {
		t.Fatal(err)
	}
	defer initial.Close()
	for _, expression := range []ast.ASTHandle{x, y} {
		decl, e := manager.GetDecl(expression)
		if e != nil {
			t.Fatal(e)
		}
		if e = initial.Core().RegisterConstant(decl, expression); e != nil {
			t.Fatal(e)
		}
	}
	solver, err := NewPBSLS(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.SetModel(initial); err != nil {
		t.Fatal(err)
	}
	if _, err = solver.makeLiteral(equality); err != nil {
		t.Fatal(err)
	}
	if len(solver.hard) != 0 || len(solver.variableExprs) != 2 {
		t.Fatalf("non-Boolean equality decomposition: %d clauses, %d variables", len(solver.hard), len(solver.variableExprs))
	}
}

func TestPBSLSPseudoBooleanRelationsMatchAllAssignments(t *testing.T) {
	manager, config := pbSLSFixture(t)
	p, q := pbSLSBool(t, manager, "pb-sls-rel-p"), pbSLSBool(t, manager, "pb-sls-rel-q")
	for _, expression := range []ast.ASTHandle{p, q} {
		if err := manager.IncRef(expression); err != nil {
			t.Fatal(err)
		}
		defer manager.DecRef(expression)
	}
	service := manager.NumeralService()
	coefficients := make([]unumeral.OwnedRational, 2)
	bound, err := service.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	defer service.Del(&bound)
	for i, value := range []int64{2, 3} {
		coefficients[i], err = service.NewValue()
		if err != nil {
			t.Fatal(err)
		}
		if err = service.SetInt64(&coefficients[i], value); err != nil {
			t.Fatal(err)
		}
		defer service.Del(&coefficients[i])
	}
	if err = service.SetInt64(&bound, 3); err != nil {
		t.Fatal(err)
	}
	args := []ast.ASTHandle{p, q}
	makeRelation := func(kind string) ast.ASTHandle {
		t.Helper()
		var expression ast.ASTHandle
		var makeErr error
		switch kind {
		case "ge":
			expression, makeErr = manager.MkPseudoBooleanGE(coefficients, args, &bound)
		case "eq":
			expression, makeErr = manager.MkPseudoBooleanEQ(coefficients, args, &bound)
		case "le":
			expression, makeErr = manager.MkPseudoBooleanLE(coefficients, args, &bound)
		}
		if makeErr != nil {
			t.Fatal(makeErr)
		}
		return expression
	}
	for _, kind := range []string{"ge", "eq", "le"} {
		relation := makeRelation(kind)
		if err = manager.IncRef(relation); err != nil {
			t.Fatal(err)
		}
		for mask := 0; mask < 4; mask++ {
			pValue, qValue := mask&1 != 0, mask&2 != 0
			value := int64(0)
			if pValue {
				value += 2
			}
			if qValue {
				value += 3
			}
			want := value >= 3
			if kind == "eq" {
				want = value == 3
			} else if kind == "le" {
				want = value <= 3
			}
			initial := pbSLSModel(t, manager, map[ast.ASTHandle]bool{p: pValue, q: qValue})
			solver, createErr := NewPBSLS(manager, config)
			if createErr != nil {
				t.Fatal(createErr)
			}
			if createErr = solver.SetModel(initial); createErr == nil {
				createErr = solver.AddHard(relation)
			}
			if createErr != nil {
				t.Fatal(createErr)
			}
			got := solver.evalClause(&solver.hard[0])
			if got != want {
				t.Errorf("%s at p=%t q=%t: got %t, want %t", kind, pValue, qValue, got, want)
			}
			if closeErr := solver.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if closeErr := initial.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		}
		if err = manager.DecRef(relation); err != nil {
			t.Fatal(err)
		}
	}
}
