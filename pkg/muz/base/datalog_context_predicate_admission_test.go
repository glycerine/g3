package base

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Leave one real AST reference slot so the context pin succeeds but the
// RuleManager's separate classifier owner cannot be admitted.
func TestDatalogContextPredicateAdmissionFailureDoesNotPublishClassifier(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	rt, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64})
	must(err)
	nums, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: rt, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: nums, Budget: budget})
	must(err)
	c, err := NewDatalogContext(m)
	must(err)
	var padding []ast.ASTRef
	defer func() {
		budget.FailAfter(-1)
		for i := range padding {
			must(m.DelRef(&padding[i]))
		}
		must(c.Close())
		must(m.Close())
		must(nums.Close())
		must(symbols.Close())
		must(rt.Close())
		if budget.Used() != 0 {
			t.Fatal("leaked bytes", budget.Used())
		}
	}()
	boolean, err := m.MkBoolSort()
	must(err)
	head, err := m.MkFreshConst("predicate-admission", boolean)
	must(err)
	declaration, err := m.GetDecl(head)
	must(err)
	name, err := m.GetASTName(declaration)
	must(err)
	seed, err := m.NewRef(ast.ASTHandle{})
	must(err)
	padding = append(padding, seed)
	budget.FailAfter(0)
	for i := 0; ; i++ {
		ref, cause := m.NewRef(ast.ASTHandle{})
		if errors.Is(cause, arena.ErrInjectedFailure) {
			break
		}
		must(cause)
		padding = append(padding, ref)
		if i >= 256 {
			t.Fatal("reference page exhaustion not reached")
		}
	}
	if len(padding) == 0 {
		t.Fatal("missing slot to release")
	}
	last := len(padding) - 1
	must(m.DelRef(&padding[last]))
	padding = padding[:last]
	if err = c.RegisterPredicate(declaration, true); !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected classifier allocation refusal", err)
	}
	if _, ok := c.ruleManager.predicates[declaration]; ok {
		t.Fatal("failed admission published classifier")
	}
	if c.predicates[declaration] || len(c.predicateOrder) != 0 || len(c.references) != 1 {
		t.Fatal("failed admission lost its pin or published context membership")
	}
	if _, found := c.named[name]; found {
		t.Fatal("failed admission published name")
	}
	if predicate, cause := c.ruleManager.isPredicate(head); cause != nil || predicate {
		t.Fatal("failed admission classified rule tail", predicate, cause)
	}
	budget.FailAfter(-1)
	must(c.RegisterPredicate(declaration, true))
	if predicate, cause := c.ruleManager.isPredicate(head); cause != nil || !predicate {
		t.Fatal("retry did not classify predicate", predicate, cause)
	}
	if got, found, cause := c.Predicate(name); cause != nil || !found || got != declaration {
		t.Fatal("retry did not publish name", got, found, cause)
	}
	must(c.Close())
}
