package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func addExternalIntervalGroundRule(t *testing.T, manager *ast.ASTManager, engine *PropositionalEngine, head ast.ASTHandle, value ast.ASTHandle, name string) {
	t.Helper()
	application, err := manager.MkApp(head, []ast.ASTHandle{value})
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(application, name))
}

func TestExternalIntervalDatalogUnaryGroundRuleAndRepeat(t *testing.T) {
	for _, numeral := range []struct {
		name string
		sort func(*ast.ASTManager, ast.FamilyID) (ast.ASTHandle, error)
		make func(*ast.ASTManager) (ast.ASTHandle, error)
	}{
		{name: "int", sort: func(m *ast.ASTManager, family ast.FamilyID) (ast.ASTHandle, error) {
			return m.MkFamilySort(family, 1, nil)
		}, make: func(m *ast.ASTManager) (ast.ASTHandle, error) { return m.ArithmeticIntegerNumeralText("7") }},
		{name: "real", sort: func(m *ast.ASTManager, family ast.FamilyID) (ast.ASTHandle, error) {
			return m.MkFamilySort(family, 0, nil)
		}, make: func(m *ast.ASTManager) (ast.ASTHandle, error) { return m.ArithmeticRealNumeralText("7/2") }},
	} {
		for _, kinds := range [][]string{
			{"external_relation", "interval_relation"},
			{"interval_relation", "external_relation"},
		} {
			t.Run(numeral.name+"/"+kinds[0]+"-first", func(t *testing.T) {
				manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
				family, err := manager.RegisterArithmeticPlugin()
				fragmentMust(t, err)
				sort, err := numeral.sort(manager, family)
				fragmentMust(t, err)
				q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-ground-"+numeral.name+"-"+kinds[0], []ast.ASTHandle{sort}, kinds)
				value, err := numeral.make(manager)
				fragmentMust(t, err)
				addExternalIntervalGroundRule(t, manager, engine, q, value, "external-interval-ground")
				selected, err := engine.ensureExternalIntervalDatalog()
				fragmentMust(t, err)
				if !selected {
					t.Fatal("ground product was not selected")
				}
				plan := engine.externalIntervalDatalog.ground
				if plan == nil || plan.valueRoot == (ast.ASTRef{}) {
					t.Fatal("ground plan did not root its numeral")
				}
				rooted, err := manager.RefGet(plan.valueRoot)
				fragmentMust(t, err)
				if rooted != value {
					t.Fatal("ground plan rooted a different numeral")
				}

				callbacks.operations = nil
				status, err := engine.QueryPredicate(q)
				fragmentMust(t, err)
				if status != lbool.Undef {
					t.Fatal("ground rule result", status)
				}
				want := []int32{
					ast.DatalogOpIsEmpty, ast.DatalogOpEmpty, ast.DatalogOpStore,
					ast.DatalogOpEmpty, ast.DatalogOpUnion, ast.DatalogOpIsEmpty,
				}
				if kinds[0] != "external_relation" {
					want = []int32{ast.DatalogOpEmpty, ast.DatalogOpStore, ast.DatalogOpEmpty, ast.DatalogOpUnion, ast.DatalogOpIsEmpty}
				}
				assertExternalIntervalRegisterOperations(t, callbacks.operations, want)

				callbacks.operations = nil
				status, err = engine.QueryPredicate(q)
				fragmentMust(t, err)
				if status != lbool.Undef {
					t.Fatal("repeated ground rule result", status)
				}
				assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
					ast.DatalogOpIsEmpty, ast.DatalogOpClone, ast.DatalogOpEmpty,
					ast.DatalogOpStore, ast.DatalogOpUnion, ast.DatalogOpIsEmpty,
				})
				if engine.externalIntervalDatalog.ground != plan || engine.externalIntervalDatalog.ground.valueRoot != plan.valueRoot {
					t.Fatal("repeated query churned the rooted ground plan")
				}
			})
		}
	}
}

func TestExternalIntervalDatalogGroundSingletonStoreRefusalRollsBack(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-ground-store-refusal-"+kinds[0], []ast.ASTHandle{integer}, kinds)
			one, err := manager.ArithmeticIntegerNumeralText("1")
			fragmentMust(t, err)
			addExternalIntervalGroundRule(t, manager, engine, q, one, "external-interval-ground-store-refusal")
			_, err = engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			old := engine.externalIntervalDatalog.entries[q].product

			refusal := errors.New("ground singleton STORE refused")
			callbacks.operations = nil
			callbacks.refusalOperation = ast.DatalogOpStore
			callbacks.refusalAt = 1
			callbacks.refusal = refusal
			status, queryErr := engine.QueryPredicate(q)
			if status != lbool.Undef || !errors.Is(queryErr, refusal) {
				t.Fatal("singleton STORE refusal did not fail query", status, queryErr)
			}
			want := []int32{ast.DatalogOpIsEmpty, ast.DatalogOpEmpty, ast.DatalogOpStore}
			if kinds[0] != "external_relation" {
				want = []int32{ast.DatalogOpEmpty, ast.DatalogOpStore}
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, want)
			if operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
				t.Fatal("singleton STORE refusal reached UNION")
			}

			callbacks.refusal = nil
			if engine.externalIntervalDatalog.entries[q].product != old {
				t.Fatal("singleton STORE refusal replaced persistent head")
			}
			empty, err := old.Empty()
			fragmentMust(t, err)
			if !empty {
				t.Fatal("singleton STORE refusal changed persistent head")
			}
			callbacks.operations = nil
			status, err = engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("singleton STORE retry did not recover", status)
			}
		})
	}
}

func TestExternalIntervalDatalogGroundUnionRefusalPublishesNothing(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-ground-union-refusal", []ast.ASTHandle{integer}, kinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	addExternalIntervalGroundRule(t, manager, engine, q, one, "external-interval-ground-union-refusal")
	_, err = engine.ensureExternalIntervalDatalog()
	fragmentMust(t, err)
	old := engine.externalIntervalDatalog.entries[q].product

	refusal := errors.New("ground UNION refused")
	callbacks.operations = nil
	callbacks.refusalOperation = ast.DatalogOpUnion
	callbacks.refusalAt = 1
	callbacks.refusal = refusal
	status, queryErr := engine.QueryPredicate(q)
	if status != lbool.Undef || !errors.Is(queryErr, refusal) {
		t.Fatal("ground UNION refusal did not fail query", status, queryErr)
	}
	assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
		ast.DatalogOpIsEmpty, ast.DatalogOpEmpty, ast.DatalogOpStore,
		ast.DatalogOpEmpty, ast.DatalogOpUnion,
	})
	if engine.externalIntervalDatalog.entries[q].product != old {
		t.Fatal("ground UNION refusal replaced persistent head")
	}
	callbacks.refusal = nil
	empty, err := old.Empty()
	fragmentMust(t, err)
	if !empty {
		t.Fatal("ground UNION refusal changed persistent head")
	}
	callbacks.operations = nil
	status, err = engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("ground UNION retry did not recover", status)
	}
}

func TestExternalIntervalDatalogGroundCommittedContinuationStoresWithoutReplay(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-ground-continuation", []ast.ASTHandle{integer}, kinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	addExternalIntervalGroundRule(t, manager, engine, q, one, "external-interval-ground-continuation")
	_, err = engine.ensureExternalIntervalDatalog()
	fragmentMust(t, err)
	activation := engine.externalIntervalDatalog
	old := activation.entries[q].product

	callbacks.operations = nil
	registers, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{q})
	fragmentMust(t, err)
	temporary, selected, err := rel.NewSelectedExternalIntervalProduct(activation.plugin, activation.entries[q].sorts, activation.entries[q].kinds)
	fragmentMust(t, err)
	if !selected {
		t.Fatal("ground singleton product not selected")
	}
	fragmentMust(t, temporary.AddFact([]ast.ASTHandle{one}))
	target, err := registers.ensureRelation(q)
	fragmentMust(t, err)
	fragmentMust(t, target.Union(temporary, nil, false))
	refusal := errors.New("ground committed cleanup pending")
	nested := &externalIntervalNestedConstructionFailure{runtime: manager.NumeralService().RuntimeOwner(), refusal: refusal}
	owner := &externalIntervalDatalogGroundContinuation{
		registers: registers, temporary: temporary, nested: nested,
		runtime: manager.NumeralService().RuntimeOwner(), publish: true,
	}
	pending := newExternalIntervalDatalogGroundFailure(nested, owner)
	activation.pending = pending
	activation.pendingCompletesRule = true
	activation.pendingGround = activation.ground
	if err = pending.Cleanup(); !errors.Is(err, refusal) || activation.entries[q].product != old {
		t.Fatal("ground continuation published before cleanup", err)
	}
	beforeRetry := len(callbacks.operations)
	status, err := engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef || activation.entries[q].product == old || nested.calls != 2 {
		t.Fatal("ground continuation did not publish on retry", status, nested.calls)
	}
	if operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 || len(callbacks.operations) != beforeRetry+1 {
		t.Fatal("ground continuation replayed callbacks", callbacks.operations)
	}
}

func TestExternalIntervalDatalogGroundRejectsUnsupportedBeforeCallbacks(t *testing.T) {
	for _, shape := range []string{"nullary", "arity-two", "variable", "body", "multi-rule"} {
		t.Run(shape, func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			sorts := []ast.ASTHandle{integer}
			if shape == "nullary" {
				sorts = nil
			}
			if shape == "arity-two" {
				sorts = []ast.ASTHandle{integer, integer}
			}
			kinds := []string{"external_relation", "interval_relation"}
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-ground-invalid-q-"+shape, sorts, kinds)
			one, err := manager.ArithmeticIntegerNumeralText("1")
			fragmentMust(t, err)
			arguments := []ast.ASTHandle{one}
			if shape == "nullary" {
				arguments = nil
			}
			if shape == "arity-two" {
				arguments = []ast.ASTHandle{one, one}
			}
			if shape == "variable" {
				variable, variableErr := manager.MkVar(0, integer)
				fragmentMust(t, variableErr)
				arguments = []ast.ASTHandle{variable}
			}
			head, err := manager.MkApp(q, arguments)
			fragmentMust(t, err)
			formula := head
			if shape == "body" {
				p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-ground-invalid-p", []ast.ASTHandle{integer}, kinds)
				variable, variableErr := manager.MkVar(0, integer)
				fragmentMust(t, variableErr)
				tail, tailErr := manager.MkApp(p, []ast.ASTHandle{variable})
				fragmentMust(t, tailErr)
				formula, err = manager.MkImplies(tail, head)
				fragmentMust(t, err)
			}
			fragmentMust(t, engine.AddRule(formula, "external-interval-ground-invalid-"+shape))
			if shape == "multi-rule" {
				r := declareExternalIntervalPredicate(t, manager, engine, "external-interval-ground-invalid-r", []ast.ASTHandle{integer}, kinds)
				second, secondErr := manager.MkApp(r, []ast.ASTHandle{one})
				fragmentMust(t, secondErr)
				fragmentMust(t, engine.AddRule(second, "external-interval-ground-invalid-second"))
			}

			status, queryErr := engine.QueryPredicate(q)
			if status != lbool.Undef || !errors.Is(queryErr, ErrExternalIntervalDatalogFragment) || engine.externalIntervalDatalog != nil || len(callbacks.operations) != 0 {
				t.Fatal("unsupported ground shape reached callbacks", status, queryErr, callbacks.operations)
			}
		})
	}
}
