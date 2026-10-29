package opt

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type weightedSoftFixture struct {
	budget   *arena.Budget
	symbols  *usymbol.Manager
	numerals *unumeral.RationalService
	manager  *ast.ASTManager
	refs     []ast.ASTRef
	p, q     ast.ASTHandle
}

func newWeightedSoftFixture(t *testing.T) *weightedSoftFixture {
	t.Helper()
	f := &weightedSoftFixture{budget: arena.NewBudget(1 << 26)}
	var err error
	f.symbols, err = usymbol.NewManager(usymbol.Config{Budget: f.budget})
	if err != nil {
		t.Fatal(err)
	}
	f.numerals, err = unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: f.budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: f.budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	f.manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: f.symbols, Numerals: f.numerals, Budget: f.budget})
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := f.manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name string
		out  *ast.ASTHandle
	}{{"p", &f.p}, {"q", &f.q}} {
		symbol, err := f.symbols.Intern(row.name)
		if err != nil {
			t.Fatal(err)
		}
		*row.out, err = f.manager.MkConst(symbol, boolean)
		if err != nil {
			t.Fatal(err)
		}
		ref, err := f.manager.NewRef(*row.out)
		if err != nil {
			t.Fatal(err)
		}
		f.refs = append(f.refs, ref)
	}
	t.Cleanup(func() {
		for i := len(f.refs) - 1; i >= 0; i-- {
			if err := f.manager.DelRef(&f.refs[i]); err != nil {
				t.Error(err)
			}
		}
		if err := f.manager.Close(); err != nil {
			t.Error(err)
		}
		if err := f.numerals.Close(); err != nil {
			t.Error(err)
		}
		if err := f.symbols.Close(); err != nil {
			t.Error(err)
		}
		if f.budget.Used() != 0 {
			t.Errorf("retained budget %d", f.budget.Used())
		}
	})
	return f
}
