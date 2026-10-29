package qe_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticPluginClassificationWeightAndClose(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	two := f.integerNumeral(t, "2")
	zero := f.integerNumeral(t, "0")
	linear := f.pin(f.manager.MkArithmeticMul(two, f.x))
	opaque := f.pin(f.manager.MkArithmeticMul(f.x, f.y))
	add := f.pin(f.manager.MkArithmeticAdd(f.x, f.y))
	if got, err := f.plugin.IsUninterpreted(linear); err != nil || got {
		t.Fatal("linear product", got, err)
	}
	if got, err := f.plugin.IsUninterpreted(opaque); err != nil || !got {
		t.Fatal("nonlinear product", got, err)
	}
	if got, err := f.plugin.IsUninterpreted(add); err != nil || got {
		t.Fatal("addition", got, err)
	}
	if weight, err := f.plugin.Weight(f.contains, add); err != nil || weight != 2 {
		t.Fatal("weight", weight, err)
	}
	atom := f.pin(f.manager.MkArithmeticLE(add, zero))
	normalized, handled, err := f.plugin.MkAtom(atom, false)
	pluginMust(t, err)
	if !handled {
		t.Fatal("arithmetic atom normalization declined")
	}
	if _, err = f.manager.GetKind(normalized); err != nil {
		t.Fatal("normalized atom lifetime", err)
	}
}

func TestArithmeticPluginRejectsUnrecognizedContainingAtomWithoutCaching(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	opaque := f.pin(f.manager.MkArithmeticMul(f.x, f.y))
	zero := f.integerNumeral(t, "0")
	formula := f.pin(f.manager.MkEq(opaque, zero))
	ok, err := f.plugin.Prepare(f.contains, formula, []ast.ASTHandle{formula}, nil)
	pluginMust(t, err)
	if ok {
		t.Fatal("nonlinear equality accepted")
	}
	_, err = f.plugin.BranchPolicy(f.contains, formula)
	if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("failed prepare cached", err)
	}
	pluginMust(t, f.plugin.Close())
	if _, err = f.plugin.Weight(f.contains, formula); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed plugin", err)
	}
}
