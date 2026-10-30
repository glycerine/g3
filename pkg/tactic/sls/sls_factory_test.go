package sls

// Source: src/tactic/sls/sls_tactic.cpp:93-97.

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func TestSLSTacticGuardsQFBVAndCleansCoreBetweenApplications(t *testing.T) {
	_, manager, _, _, context, globals, local, _ := slsTrackerFixture(t)
	strategy, err := NewSLSTactic(manager, context, globals, local)
	if err != nil {
		t.Fatal(err)
	}
	if err = strategy.IncRef(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := strategy.DecRef(); err != nil {
			t.Error(err)
		}
	})
	for _, name := range []string{"sls-factory-first", "sls-factory-second"} {
		atom := slsBoolConst(t, manager, name)
		slsPin(t, manager, atom)
		goal, err := tactic.NewGoal(manager, false, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if err = goal.Assert(atom); err != nil {
			t.Fatal(err)
		}
		result := &tactic.TacticResult{}
		if err = strategy.Apply(goal, result); err != nil {
			t.Fatal(err)
		}
		if size, err := goal.Size(); err != nil || size != 0 || len(result.Goals) != 1 {
			t.Fatal("QF_BV strategy result", size, len(result.Goals), err)
		}
		if err = result.Close(); err != nil {
			t.Fatal(err)
		}
		if err = goal.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSLSTacticRejectsNonQFBVBeforeCore(t *testing.T) {
	_, manager, _, _, context, globals, local, _ := slsTrackerFixture(t)
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	strategy, err := NewSLSTactic(manager, context, globals, local)
	if err != nil {
		t.Fatal(err)
	}
	if err = strategy.IncRef(); err != nil {
		t.Fatal(err)
	}
	defer strategy.DecRef()
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		t.Fatal(err)
	}
	sort, err := manager.GetSort(zero)
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.Symbols.Intern("sls-factory-integer")
	if err != nil {
		t.Fatal(err)
	}
	x, err := manager.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	formula, err := manager.MkEq(x, zero)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, zero, x, formula)
	goal, err := tactic.NewGoal(manager, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer goal.Close()
	if err = goal.Assert(formula); err != nil {
		t.Fatal(err)
	}
	result := &tactic.TacticResult{}
	defer result.Close()
	if err = strategy.Apply(goal, result); err == nil {
		t.Fatal("non-QF_BV goal passed the source fail_if_not guard")
	}
}
