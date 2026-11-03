package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalIntervalDatalogCartesianConditionInstallationRetryAndReuse(t *testing.T) {
	testExternalIntervalCartesianPlanInstallation(t, false, false)
}

func TestExternalIntervalDatalogCartesianConstantPinningRetryAndReuse(t *testing.T) {
	testExternalIntervalCartesianPlanInstallation(t, true, false)
}

func TestExternalIntervalDatalogCartesianHeadConstantPinningRetryAndReuse(t *testing.T) {
	testExternalIntervalCartesianPlanInstallation(t, true, true)
}

func testExternalIntervalCartesianPlanInstallation(t *testing.T, withConstants, withHeadConstants bool) {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("Cartesian condition fixture leaked bytes", budget.Used())
		}
	})
	manager, engine, _ := newExternalIntervalDatalogFixture(t)
	budget = manager.Budget()
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	tailSorts := []ast.ASTHandle{integer}
	if withConstants {
		tailSorts = append(tailSorts, integer)
	}
	p := declareExternalIntervalPredicate(t, manager, engine, "cartesian-condition-p", tailSorts, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "cartesian-condition-q", tailSorts, kinds)
	headSorts := []ast.ASTHandle{integer, integer}
	if withHeadConstants {
		headSorts = append(headSorts, integer, integer)
	}
	h := declareExternalIntervalPredicate(t, manager, engine, "cartesian-condition-h", headSorts, kinds)
	x, err := manager.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := manager.MkVar(1, integer)
	fragmentMust(t, err)
	guard, err := manager.MkArithmeticLT(x, y)
	fragmentMust(t, err)
	guardRoot, err := manager.NewRef(guard)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, manager.DelRef(&guardRoot)) })
	var values []ast.ASTHandle
	if withConstants {
		for _, text := range []string{"7", "11"} {
			value, cause := manager.ArithmeticIntegerNumeralText(text)
			fragmentMust(t, cause)
			root, cause := manager.NewRef(value)
			fragmentMust(t, cause)
			t.Cleanup(func() { fragmentMust(t, manager.DelRef(&root)) })
			values = append(values, value)
		}
	}
	selected, err := engine.ensureExternalIntervalDatalog()
	fragmentMust(t, err)
	if !selected {
		t.Fatal("Cartesian condition activation not selected")
	}
	activation := engine.externalIntervalDatalog
	newPlan := func() *externalIntervalDatalogCartesianRule {
		plan := &externalIntervalDatalogCartesianRule{
			head: h, left: p, right: q,
			condition: &externalIntervalUnaryCondition{
				source: []ast.ASTHandle{guard},
				bindings: []externalIntervalUnaryConditionBinding{
					{variable: 0, column: 0, sort: integer},
					{variable: 1, column: 1, sort: integer},
				},
			},
		}
		if withConstants {
			plan.constants = []externalIntervalCartesianConstant{{column: 1, value: values[0]}, {column: 3, value: values[1]}}
			plan.headRemoved = []uint32{1, 3}
			plan.condition.bindings[1].column = 2
		}
		if withHeadConstants {
			plan.headConstants = []externalIntervalCartesianHeadConstant{{sort: integer, value: values[0]}, {sort: integer, value: values[1]}}
		}
		return plan
	}
	assertConstantRoots := func(plan *externalIntervalDatalogCartesianRule, pinned bool) {
		t.Helper()
		for _, constant := range plan.constants {
			if !pinned {
				if constant.root != (ast.ASTRef{}) {
					t.Fatal("constant root was not released", constant.column)
				}
				continue
			}
			value, cause := manager.RefGet(constant.root)
			fragmentMust(t, cause)
			if value != constant.value {
				t.Fatal("constant root changed its numeral", constant.column)
			}
		}
		for column, constant := range plan.headConstants {
			if !pinned {
				if constant.root != (ast.ASTRef{}) {
					t.Fatal("head constant root was not released", column)
				}
				continue
			}
			value, cause := manager.RefGet(constant.root)
			fragmentMust(t, cause)
			if value != constant.value {
				t.Fatal("head constant root changed its numeral", column)
			}
		}
	}
	// Reach a real reference-page allocation so initial condition pinning
	// fails after the activation has adopted the complete immutable plan.
	var pins []ast.ASTRef
	t.Cleanup(func() {
		for i := range pins {
			if pins[i] != (ast.ASTRef{}) {
				fragmentMust(t, manager.DelRef(&pins[i]))
			}
		}
	})
	free := 0
	if withConstants {
		free = 1
	}
	if withHeadConstants {
		free = 3 // Both tail constants and the first head constant.
	}
	// Seed enough references without failure injection to free the desired
	// prefix even when the fixture starts at a page boundary.
	for i := 0; i <= free; i++ {
		seed, cause := manager.NewRef(ast.ASTHandle{})
		fragmentMust(t, cause)
		pins = append(pins, seed)
	}
	for i := 0; i < 4096; i++ {
		budget.FailAfter(0)
		pin, cause := manager.NewRef(ast.ASTHandle{})
		budget.FailAfter(-1)
		if errors.Is(cause, arena.ErrInjectedFailure) {
			break
		}
		fragmentMust(t, cause)
		pins = append(pins, pin)
		if i == 4095 {
			t.Fatal("Cartesian condition reference-page frontier not reached")
		}
	}
	for i := len(pins) - free; i < len(pins); i++ {
		fragmentMust(t, manager.DelRef(&pins[i]))
	}
	first := newPlan()
	initial := publicUDocProductLiveCounts(manager.Stats())
	budget.FailAfter(0)
	cause := activation.installExternalIntervalRulePlans(nil, nil, first)
	budget.FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) || activation.cartesian != first || first.condition.mapped != (ast.ASTRef{}) {
		t.Fatal("failed condition installation lost adopted plan", cause)
	}
	var firstConstantRoot ast.ASTRef
	var firstHeadConstantRoot ast.ASTRef
	if withConstants {
		firstConstantRoot = first.constants[0].root
		if firstConstantRoot == (ast.ASTRef{}) || len(first.condition.sourceRoots) != 0 {
			t.Fatal("constant pin failure lost its completed prefix")
		}
		if withHeadConstants {
			firstHeadConstantRoot = first.headConstants[0].root
			if first.constants[1].root == (ast.ASTRef{}) || firstHeadConstantRoot == (ast.ASTRef{}) || first.headConstants[1].root != (ast.ASTRef{}) {
				t.Fatal("head constant pin failure lost the three-root prefix")
			}
		} else if first.constants[1].root != (ast.ASTRef{}) {
			t.Fatal("constant pin failure did not stop after the first numeral root")
		}
		initial[3] += uint64(free)
	}
	if actual := publicUDocProductLiveCounts(manager.Stats()); actual != initial {
		t.Fatal("partial installation lost AST ownership", initial, actual)
	}
	retry := newPlan()
	fragmentMust(t, activation.installExternalIntervalRulePlans(nil, nil, retry))
	if activation.cartesian != first || first.condition.mapped == (ast.ASTRef{}) || retry.condition.mapped != (ast.ASTRef{}) || len(retry.condition.sourceRoots) != 0 {
		t.Fatal("condition retry replaced its owning plan")
	}
	assertConstantRoots(first, true)
	assertConstantRoots(retry, false)
	if withConstants && first.constants[0].root != firstConstantRoot {
		t.Fatal("constant pin retry replaced the completed first pin")
	}
	if withHeadConstants && first.headConstants[0].root != firstHeadConstantRoot {
		t.Fatal("head constant pin retry replaced the completed first head pin")
	}
	mapped := first.condition.mapped
	before := publicUDocProductLiveCounts(manager.Stats())
	fragmentMust(t, activation.installExternalIntervalRulePlans(nil, nil, newPlan()))
	if activation.cartesian != first || first.condition.mapped != mapped || publicUDocProductLiveCounts(manager.Stats()) != before {
		t.Fatal("equivalent installation duplicated condition ownership")
	}
	fragmentMust(t, activation.installExternalIntervalRulePlans(nil, nil, nil))
	if first.condition.mapped != (ast.ASTRef{}) || len(first.condition.sourceRoots) != 0 {
		t.Fatal("removing standalone plan did not close its condition")
	}
	assertConstantRoots(first, false)
	standalone := newPlan()
	fragmentMust(t, activation.installExternalIntervalRulePlans(nil, nil, standalone))
	left, right := newPlan(), newPlan()
	program := &externalIntervalDatalogProgram{rules: []externalIntervalDatalogProgramRule{{cartesian: left}, {cartesian: right}}}
	fragmentMust(t, activation.installExternalIntervalProgram(program))
	if standalone.condition.mapped != (ast.ASTRef{}) || len(standalone.condition.sourceRoots) != 0 || left.condition.mapped == (ast.ASTRef{}) || right.condition.mapped == (ast.ASTRef{}) {
		t.Fatal("standalone-to-program transition lost condition roots")
	}
	assertConstantRoots(standalone, false)
	assertConstantRoots(left, true)
	assertConstantRoots(right, true)
	before = publicUDocProductLiveCounts(manager.Stats())
	fragmentMust(t, activation.installExternalIntervalProgram(&externalIntervalDatalogProgram{rules: []externalIntervalDatalogProgramRule{{cartesian: newPlan()}, {cartesian: newPlan()}}}))
	if activation.program != program || publicUDocProductLiveCounts(manager.Stats()) != before {
		t.Fatal("equivalent program installation replaced condition owners")
	}
	fragmentMust(t, engine.Close())
	for _, plan := range []*externalIntervalDatalogCartesianRule{left, right} {
		if plan.condition.mapped != (ast.ASTRef{}) || len(plan.condition.sourceRoots) != 0 {
			t.Fatal("program teardown left a condition root")
		}
		assertConstantRoots(plan, false)
	}
}
