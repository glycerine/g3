package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func docASTManager(t *testing.T) *ast.ASTManager {
	t.Helper()
	budget := arena.NewBudget(1 << 26)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
		if err := numerals.Close(); err != nil {
			t.Error(err)
		}
		if err := symbols.Close(); err != nil {
			t.Error(err)
		}
		if used := budget.Used(); used != 0 {
			t.Errorf("DOC AST services retained %d bytes", used)
		}
	})
	return manager
}

func TestDOCSourceSMTEmptinessOracleAndFormula(t *testing.T) {
	docManager, _ := newTestDocManager(t, 2)
	manager := docASTManager(t)
	empty := docPattern(t, docManager, "xx", "0x", "1x")
	formula, err := docManager.ToFormula(manager, empty)
	if err != nil || formula == (ast.ASTHandle{}) {
		t.Fatal("formula", formula, err)
	}
	isEmpty, err := docManager.IsEmptyComplete(manager, empty)
	if err != nil || !isEmpty {
		t.Fatal("covered cube must be SMT-empty", isEmpty, err)
	}
	nonempty := docPattern(t, docManager, "xx", "00", "11")
	isEmpty, err = docManager.IsEmptyComplete(manager, nonempty)
	if err != nil || isEmpty {
		t.Fatal("remaining assignments must be satisfiable", isEmpty, err)
	}
	full := docPattern(t, docManager, "xx")
	isEmpty, err = docManager.IsEmptyComplete(manager, full)
	if err != nil || isEmpty {
		t.Fatal("negative-free source shortcut", isEmpty, err)
	}
}

func TestDOCUnionCompleteEmptiness(t *testing.T) {
	docManager, _ := newTestDocManager(t, 1)
	manager := docASTManager(t)
	union, _ := NewDocUnion(docManager)
	if empty, err := union.EmptyComplete(manager); err != nil || !empty {
		t.Fatal("empty union", empty, err)
	}
	covered := docPattern(t, docManager, "x", "0", "1")
	if err := union.PushOwned(covered); err != nil {
		t.Fatal(err)
	}
	if empty, err := union.EmptyComplete(manager); err != nil || !empty {
		t.Fatal("union containing only empty DOC", empty, err)
	}
	live := docPattern(t, docManager, "0")
	if err := union.PushOwned(live); err != nil {
		t.Fatal(err)
	}
	if empty, err := union.EmptyComplete(manager); err != nil || empty {
		t.Fatal("live union", empty, err)
	}
}
