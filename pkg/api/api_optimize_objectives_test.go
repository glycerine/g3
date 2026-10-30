package api

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeArithmeticObjectiveValueAndIsolation(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.Assert(f.lower))
	astTestMust(t, f.opt.Assert(f.upper))
	index, err := f.opt.Maximize(f.x)
	astTestMust(t, err)
	if index != 0 {
		t.Fatal("objective index", index)
	}
	status, err := f.opt.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("optimization status", status)
	}
	objective, err := f.opt.Objective(index)
	astTestMust(t, err)
	if objective.Expression != f.x || !objective.Maximize || objective.Soft {
		t.Fatal("objective metadata", objective)
	}
	value, err := f.opt.ObjectiveValue(index)
	astTestMust(t, err)
	if !value.Available || value.Unbounded || value.Value.Main.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatal("objective value", value)
	}
	if _, err = f.opt.Statistics(); err != nil {
		t.Fatal("optimization statistics", err)
	}
	parentAssertions, err := f.c.Assertions()
	astTestMust(t, err)
	if len(parentAssertions) != 0 {
		t.Fatal("optimize assertions leaked to parent", parentAssertions)
	}
	if _, err = f.opt.Objective(1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("objective bounds", err)
	}
}
