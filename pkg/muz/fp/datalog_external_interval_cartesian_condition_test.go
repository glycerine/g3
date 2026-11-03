package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalIntervalDatalogCartesianConditionInstallationRetryAndReuse(t *testing.T) {
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
	p := declareExternalIntervalPredicate(t, manager, engine, "cartesian-condition-p", []ast.ASTHandle{integer}, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "cartesian-condition-q", []ast.ASTHandle{integer}, kinds)
	h := declareExternalIntervalPredicate(t, manager, engine, "cartesian-condition-h", []ast.ASTHandle{integer, integer}, kinds)
	x, err := manager.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := manager.MkVar(1, integer)
	fragmentMust(t, err)
	guard, err := manager.MkArithmeticLT(x, y)
	fragmentMust(t, err)
	guardRoot, err := manager.NewRef(guard)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, manager.DelRef(&guardRoot)) })
	selected, err := engine.ensureExternalIntervalDatalog()
	fragmentMust(t, err)
	if !selected {
		t.Fatal("Cartesian condition activation not selected")
	}
	activation := engine.externalIntervalDatalog
	newPlan := func() *externalIntervalDatalogCartesianRule {
		return &externalIntervalDatalogCartesianRule{
			head: h, left: p, right: q,
			condition: &externalIntervalUnaryCondition{
				source: []ast.ASTHandle{guard},
				bindings: []externalIntervalUnaryConditionBinding{
					{variable: 0, column: 0, sort: integer},
					{variable: 1, column: 1, sort: integer},
				},
			},
		}
	}
	// Reach a real reference-page allocation so initial condition pinning
	// fails after the activation has adopted the complete immutable plan.
	var pins []ast.ASTRef
	t.Cleanup(func() {
		for i := range pins {
			fragmentMust(t, manager.DelRef(&pins[i]))
		}
	})
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
	first := newPlan()
	budget.FailAfter(0)
	cause := activation.installExternalIntervalRulePlans(nil, nil, first)
	budget.FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) || activation.cartesian != first || first.condition.mapped != (ast.ASTRef{}) {
		t.Fatal("failed condition installation lost adopted plan", cause)
	}
	retry := newPlan()
	fragmentMust(t, activation.installExternalIntervalRulePlans(nil, nil, retry))
	if activation.cartesian != first || first.condition.mapped == (ast.ASTRef{}) || retry.condition.mapped != (ast.ASTRef{}) || len(retry.condition.sourceRoots) != 0 {
		t.Fatal("condition retry replaced its owning plan")
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
	standalone := newPlan()
	fragmentMust(t, activation.installExternalIntervalRulePlans(nil, nil, standalone))
	left, right := newPlan(), newPlan()
	program := &externalIntervalDatalogProgram{rules: []externalIntervalDatalogProgramRule{{cartesian: left}, {cartesian: right}}}
	fragmentMust(t, activation.installExternalIntervalProgram(program))
	if standalone.condition.mapped != (ast.ASTRef{}) || len(standalone.condition.sourceRoots) != 0 || left.condition.mapped == (ast.ASTRef{}) || right.condition.mapped == (ast.ASTRef{}) {
		t.Fatal("standalone-to-program transition lost condition roots")
	}
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
	}
}
