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

func TestDatalogContextRestrictPredicatesStagesClassifiersAndRetries(t *testing.T) {
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
	var headRefs []ast.ASTRef
	defer func() {
		budget.FailAfter(-1)
		for i := range padding {
			must(m.DelRef(&padding[i]))
		}
		for i := range headRefs {
			must(m.DelRef(&headRefs[i]))
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
	var heads, declarations []ast.ASTHandle
	for _, name := range []string{"restrict-old", "restrict-new-a", "restrict-new-b"} {
		head, e := m.MkFreshConst(name, boolean)
		must(e)
		declaration, e := m.GetDecl(head)
		must(e)
		ref, e := m.NewRef(head)
		must(e)
		headRefs = append(headRefs, ref)
		heads = append(heads, head)
		declarations = append(declarations, declaration)
	}
	must(c.RegisterPredicate(declarations[0], true))
	// Reserve three existing slots before refusing new pages. One staged
	// predicate gets a pin and classifier, and the second gets only its pin.
	for i := 0; i < 3; i++ {
		ref, e := m.NewRef(ast.ASTHandle{})
		must(e)
		padding = append(padding, ref)
	}
	budget.FailAfter(0)
	for i := 0; ; i++ {
		ref, e := m.NewRef(ast.ASTHandle{})
		if errors.Is(e, arena.ErrInjectedFailure) {
			break
		}
		must(e)
		padding = append(padding, ref)
		if i >= 256 {
			t.Fatal("reference page exhaustion not reached")
		}
	}
	for i := 0; i < 3; i++ {
		last := len(padding) - 1
		must(m.DelRef(&padding[last]))
		padding = padding[:last]
	}
	keep := map[ast.ASTHandle]bool{declarations[1]: true, declarations[2]: true}
	if err = c.RestrictPredicates(keep); !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected staged admission failure", err)
	}
	if len(c.inactivePredicates) != 1 {
		t.Fatal("first classifier was not retained for retry", len(c.inactivePredicates))
	}
	check := func(want ...bool) {
		t.Helper()
		for i, head := range heads {
			member, e := c.IsPredicate(declarations[i])
			must(e)
			classified, e := c.ruleManager.isPredicate(head)
			must(e)
			if member != want[i] || classified != want[i] {
				t.Fatalf("predicate %d membership=%v classifier=%v want=%v", i, member, classified, want[i])
			}
		}
	}
	check(true, false, false)
	budget.FailAfter(-1)
	must(c.RestrictPredicates(keep))
	check(false, true, true)
	rule, err := c.ruleManager.NewRule(heads[1], []ast.ASTHandle{heads[0], heads[2]}, nil, "restricted")
	must(err)
	count, err := rule.UninterpretedTailSize()
	must(err)
	if count != 1 {
		t.Fatal("rule construction used stale predicate classification", count)
	}
	must(rule.Close())
	// Restricting back must restore the old classifier and remove both new
	// ones; re-registering a removed predicate reuses its parked owner.
	must(c.RestrictPredicates(map[ast.ASTHandle]bool{declarations[0]: true}))
	check(true, false, false)
	must(c.RegisterPredicate(declarations[1], false))
	check(true, true, false)
	must(c.Close())
}
