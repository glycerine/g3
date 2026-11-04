package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestFastContextTacticVariableGrowthBeforeRoot(t *testing.T) {
	budget := arena.NewBudget(1 << 28)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}})
	cfg.InitialCapacity = 4
	m, err := numeral.NewFastFloatManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewFastContext(m, &control.Reslimit{Limit: 10000}, nil, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := numeral.NewScopedFastFloat(m)
	if err != nil {
		t.Fatal(err)
	}
	value, err := owner.Get()
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetInt32(value, 1); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
		if err := m.CloseChecked(); err != nil {
			t.Error(err)
		}
		if budget.Used() != 0 {
			t.Error("retained fast context owners", budget.Used())
		}
	})
	if c.root != nil {
		t.Fatal("C++ context constructor leaves root absent")
	}
	x, err := c.NewVariable(true)
	if err != nil || x != 0 {
		t.Fatal("first integer variable", x, err)
	}
	y, err := c.NewVariable(false)
	if err != nil || y != 1 {
		t.Fatal("second real variable", y, err)
	}
	if len(c.isInt) != 2 || !c.isInt[x] || c.isInt[y] || len(c.watchers) != 2 || len(c.watchOrder) != 2 || c.root != nil {
		t.Fatal("source variable and watch registration before root creation")
	}
	sum, err := c.DefinePolynomial(value, []FastPolynomialTerm{{Variable: x, Coefficient: value}, {Variable: y, Coefficient: value}})
	if err != nil || sum != 2 || c.isInt[sum] {
		t.Fatal("real polynomial output", sum, err)
	}
	product, err := c.DefineMonomial([]FastPower{{Variable: x, Degree: 2}})
	if err != nil || product != 3 || !c.isInt[product] {
		t.Fatal("integer monomial output", product, err)
	}
	if c.root != nil {
		t.Fatal("definition registration allocated root early")
	}
	root, err := c.Root()
	if err != nil || len(root.lower) != 4 || len(root.upper) != 4 {
		t.Fatal("source root created with all registered variables", root, err)
	}
	if err := c.InitializeRoot(); err != nil {
		t.Fatal("root with tactic-created variables", err)
	}
	if _, err := c.NewVariable(true); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("late variable must not invalidate initialized root", err)
	}
}
