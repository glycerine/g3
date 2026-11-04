package base

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// A live cost recorder causes a genuine refusal of the last rule release.
// Context cleanup must retain every later owner until Close can resume.
func TestDatalogContextCloseRetriesRuleRelease(t *testing.T) {
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
	recorder := NewCostRecorder()
	defer func() {
		must(recorder.Close())
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
	head, err := m.MkFreshConst("close-retry-rule", boolean)
	must(err)
	declaration, err := m.GetDecl(head)
	must(err)
	must(c.RegisterPredicate(declaration, false))
	rm, err := c.RuleManager()
	must(err)
	rule, err := rm.NewRule(head, nil, nil, "close-retry")
	must(err)
	must(c.AddRule(rule))
	rules, err := c.Rules()
	must(err)
	must(recorder.Start(rule.state.accounting))
	must(rule.Close())
	retained := rules.rules[0]
	refCount := len(c.references)
	err = c.Close()
	if err == nil || !strings.HasPrefix(err.Error(), "muz/base: close datalog context: ") {
		t.Fatal("close error lost context", err)
	}
	if !errors.Is(err, ErrCosts) || !c.closing || c.closed || c.manager != m || c.ruleManager != rm || c.binder.closed || len(c.references) != refCount || rules.closed || len(rules.rules) != 1 || rules.rules[0] != retained || retained.closed {
		t.Fatal("failed close discarded owned state", err)
	}
	if _, err = c.Rules(); !errors.Is(err, ErrDatalogContextOwner) {
		t.Fatal("closing context exposed rules", err)
	}
	if err = c.Pop(0); !errors.Is(err, ErrDatalogContextOwner) {
		t.Fatal("closing context accepted pop", err)
	}
	if _, err = rules.Len(); !errors.Is(err, ErrRuleSetOwner) {
		t.Fatal("borrowed set exposed partial cleanup", err)
	}
	if err = rules.Reset(); !errors.Is(err, ErrRuleSetOwner) {
		t.Fatal("borrowed set accepted reset", err)
	}
	if err = rules.Close(); !errors.Is(err, ErrRuleSetOwner) {
		t.Fatal("borrowed set accepted close", err)
	}
	must(recorder.Finish())
	must(c.Close())
	must(c.Close())
	if !c.closed || c.closing || c.manager != nil || !rules.closed || !retained.closed {
		t.Fatal("close retry did not release owners")
	}
}

// Destroying a rational declaration can consume the AST reference before the
// real TLS accounting error is reported. Retry must not repeat that release.
func TestDatalogContextCloseRetriesConsumedBackground(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	rt, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	nums, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: rt, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: nums, Budget: budget})
	must(err)
	c, err := NewDatalogContext(m)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	first, err := m.MkFreshConst("close-background-first", boolean)
	must(err)
	must(c.AssertExpression(first))
	remaining := c.background[0]
	scoped, err := numeral.NewScopedInt(nums.Manager.Z)
	must(err)
	integer, err := scoped.Get()
	must(err)
	digits := make([]uint32, 32768)
	digits[0], digits[len(digits)-1] = 17, 1
	must(nums.Manager.Z.SetLimbs(integer, digits))
	value, err := nums.NewFromScopedInt(scoped)
	must(err)
	parameter, err := m.NewParameterRational(&value)
	must(err)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
	must(err)
	name, err := symbols.Intern("close-background-consumed")
	must(err)
	declaration, err := m.MkFuncDeclWithInfo(name, nil, boolean, info)
	must(err)
	last, err := m.MkApp(declaration, nil)
	must(err)
	must(c.AssertExpression(last))
	consumed := c.background[1]
	must(m.DelFuncDeclInfo(&info))
	must(m.DelParameter(&parameter))
	must(nums.Del(&value))
	must(scoped.Close())
	if count, e := m.GetRefCount(declaration); e != nil || count != 1 {
		t.Fatal("background does not own declaration", count, e)
	}
	a, err := rt.Allocate(8)
	must(err)
	b, err := rt.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = rt.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal(err)
	}
	must(rt.Deallocate(&a))
	delta = math.MinInt64 + 1000 - rt.State().GlobalSize + 16
	if err = rt.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal(err)
	}
	must(rt.Deallocate(&b))
	if state := rt.State(); state.GlobalSize != math.MinInt64+1000 || state.LocalSize != 0 {
		t.Fatal("accounting setup", state)
	}
	before, blocks := m.Stats().LiveReferences, rt.Stats().LiveBlocks
	err = c.Close()
	if err == nil || !strings.HasPrefix(err.Error(), "muz/base: close datalog context: ") {
		t.Fatal("close error lost context", err)
	}
	if !errors.Is(err, arena.ErrBounds) || !c.closing || c.closed || len(c.background) != 1 || c.background[0] != remaining || m.Stats().LiveReferences+1 != before || rt.Stats().LiveBlocks >= blocks {
		t.Fatal("lost background release frontier", err)
	}
	if _, err = m.RefGet(consumed); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("consumed reference survived", err)
	}
	if h, err := m.RefGet(remaining); err != nil || h != first {
		t.Fatal("remaining reference lost", h, err)
	}
	recovery, err := rt.Allocate(400000)
	must(err)
	state, blocks := rt.State(), rt.Stats().LiveBlocks
	must(c.Close())
	must(c.Close())
	if rt.State() != state || rt.Stats().LiveBlocks != blocks {
		t.Fatal("retry replayed native release")
	}
	finish := func(close func() error) {
		t.Helper()
		for i := 0; i < 128; i++ {
			before := rt.Stats().LiveBlocks
			e := close()
			if e == nil {
				return
			}
			if !errors.Is(e, arena.ErrBounds) || rt.Stats().LiveBlocks >= before {
				t.Fatal("cleanup made no physical progress", e)
			}
		}
		t.Fatal("cleanup retry limit")
	}
	finish(m.Close)
	finish(nums.Close)
	finish(symbols.Close)
	if err = rt.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
		t.Fatal(err)
	}
	finish(rt.Close)
	if budget.Used() != 0 {
		t.Fatal("leaked bytes", budget.Used())
	}
}
