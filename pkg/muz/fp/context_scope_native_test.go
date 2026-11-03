package fp

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type contextScopeNativeFixture struct {
	t        *testing.T
	budget   *arena.Budget
	runtime  *memory.Runtime
	numbers  *numeral.RationalService
	symbols  *symbol.Manager
	manager  *ast.ASTManager
	engine   *PropositionalEngine
	recovery memory.Block
}

func newContextScopeNativeFixture(t *testing.T, transport memory.ErrorTransport) *contextScopeNativeFixture {
	f := &contextScopeNativeFixture{t: t, budget: arena.NewBudget(1 << 28)}
	var err error
	f.runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: f.budget, ThreadLocal: transport == nil, SizeTBits: 64, Transport: transport})
	f.must(err)
	f.numbers, err = numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: f.runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: f.budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: f.budget}}}})
	f.must(err)
	f.symbols, err = symbol.NewManager(symbol.Config{Budget: f.budget})
	f.must(err)
	f.manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: f.symbols, Numerals: f.numbers, Budget: f.budget})
	f.must(err)
	_, err = f.manager.RegisterArithmeticPlugin()
	f.must(err)
	f.engine, err = NewPropositionalEngine(f.manager)
	f.must(err)
	t.Cleanup(f.close)
	return f
}

func (f *contextScopeNativeFixture) must(err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *contextScopeNativeFixture) close() {
	f.must(f.runtime.SetMaxCount(0))
	finish := func(close func() error) {
		f.t.Helper()
		for i := 0; i < 128; i++ {
			before := f.runtime.Stats().LiveBlocks
			err := close()
			if err == nil {
				return
			}
			if !errors.Is(err, arena.ErrBounds) || f.runtime.Stats().LiveBlocks >= before {
				f.t.Fatal("cleanup made no physical progress", err)
			}
		}
		f.t.Fatal("cleanup horizon")
	}
	finish(f.engine.Close)
	finish(f.manager.Close)
	finish(f.numbers.Close)
	finish(f.symbols.Close)
	if f.recovery != (memory.Block{}) {
		if err := f.runtime.Deallocate(&f.recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || f.recovery != (memory.Block{})) {
			f.t.Fatal(err)
		}
	}
	finish(f.runtime.Close)
	if f.budget.Used() != 0 {
		f.t.Fatal("scope fixture leaked bytes", f.budget.Used())
	}
}

func (f *contextScopeNativeFixture) addRule(name string) {
	f.t.Helper()
	b, err := f.manager.MkBoolSort()
	f.must(err)
	h, err := f.manager.MkFreshConst(name, b)
	f.must(err)
	d, err := f.manager.GetDecl(h)
	f.must(err)
	f.must(f.engine.RegisterPredicate(d))
	f.must(f.engine.AddRule(h, name))
}

func TestFixedpointPopRefusesNativeInvocationBeforeMutation(t *testing.T) {
	var action func()
	fired := false
	f := newContextScopeNativeFixture(t, func(failure memory.Failure) error {
		if action != nil && !fired {
			fired = true
			action()
		}
		return failure
	})
	e, m := f.engine, f.manager
	f.addRule("scope-before")
	f.must(e.Push())
	f.addRule("scope-after")
	assertion, err := m.MkTrue()
	f.must(err)
	f.must(e.AssertBackground(assertion))
	saved, assertionRef, revision := e.scopes[0], e.assertions[0], e.revision
	var refusal error
	intact := true
	action = func() {
		for i := 0; i < 2; i++ {
			refusal = e.Pop()
			intact = intact && errors.Is(refusal, ast.ErrASTPending) && e.popPhase == contextPopNone && len(e.scopes) == 1 && e.scopes[0] == saved && len(e.rawRules) == 2 && len(e.assertions) == 1 && e.assertions[0] == assertionRef && e.revision == revision
		}
	}
	value, err := f.numbers.NewValue()
	f.must(err)
	defer func() { f.must(f.numbers.Del(&value)) }()
	f.must(f.numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	f.must(f.runtime.SetMaxCount(uint64(f.runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	_, failure := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	action = nil
	f.must(f.runtime.SetMaxCount(0))
	f.must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if failure == nil || !fired || !intact {
		t.Fatal("native callback changed the scope", failure, fired, refusal, intact)
	}
	f.must(e.Pop())
	if len(e.scopes) != 0 || len(e.rawRules) != 1 || len(e.assertions) != 0 || e.revision != revision+1 {
		t.Fatal("scope retry did not restore the original program")
	}
}

func TestFixedpointPopResumesConsumedAssertionInSourceOrder(t *testing.T) {
	f := newContextScopeNativeFixture(t, nil)
	e, m, numbers, runtime := f.engine, f.manager, f.numbers, f.runtime
	f.addRule("consumed-scope-before")
	f.must(e.Push())
	f.addRule("consumed-scope-after")
	scoped, err := numeral.NewScopedInt(numbers.Manager.Z)
	f.must(err)
	n, err := scoped.Get()
	f.must(err)
	digits := make([]uint32, 32768)
	digits[0], digits[len(digits)-1] = 17, 1
	f.must(numbers.Manager.Z.SetLimbs(n, digits))
	value, err := numbers.NewFromScopedInt(scoped)
	f.must(err)
	parameter, err := m.NewParameterRational(&value)
	f.must(err)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
	f.must(err)
	boolean, err := m.MkBoolSort()
	f.must(err)
	name, err := f.symbols.Intern("consumed-scope-metadata")
	f.must(err)
	declaration, err := m.MkFuncDeclWithInfo(name, nil, boolean, info)
	f.must(err)
	large, err := m.MkApp(declaration, nil)
	f.must(err)
	first, err := m.MkFreshConst("assertion-before-large", boolean)
	f.must(err)
	last, err := m.MkFreshConst("assertion-after-large", boolean)
	f.must(err)
	for _, assertion := range []ast.ASTHandle{first, large, last} {
		f.must(e.AssertBackground(assertion))
	}
	f.must(m.DelFuncDeclInfo(&info))
	f.must(m.DelParameter(&parameter))
	f.must(numbers.Del(&value))
	f.must(scoped.Close())
	remaining, revision := e.assertions[0], e.revision
	if count, err := m.GetRefCount(declaration); err != nil || count != 1 {
		t.Fatal("not the sole metadata owner", count, err)
	}
	// Real signed TLS accounting refusal occurs after physically releasing
	// the large rational, consuming its ASTRef while leaving the first owner.
	a, err := runtime.Allocate(8)
	f.must(err)
	c, err := runtime.Allocate(8)
	f.must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal(err)
	}
	f.must(runtime.Deallocate(&a))
	delta = math.MinInt64 + 1000 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&c, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal(err)
	}
	f.must(runtime.Deallocate(&c))
	before, blocks := m.Stats().LiveReferences, runtime.Stats().LiveBlocks
	if err = e.Pop(); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("missing consumed assertion refusal", err)
	}
	count, err := e.set.Len()
	f.must(err)
	if e.popPhase != contextPopAssertions || len(e.assertions) != 1 || e.assertions[0] != remaining || len(e.scopes) != 1 || len(e.rawRules) != 2 || len(e.rawRuleScopes[0]) != 1 || count != 2 || m.Stats().LiveReferences+2 != before || runtime.Stats().LiveBlocks >= blocks || e.revision != revision+1 {
		t.Fatal("interrupted background cleanup advanced a later source trail", e.popPhase, len(e.assertions), len(e.rawRules), count, before, m.Stats().LiveReferences)
	}
	if _, err = m.RefGet(remaining); err != nil {
		t.Fatal("untouched assertion owner lost", err)
	}
	if _, err = e.BackgroundAssertions(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("partial background escaped", err)
	}
	f.recovery, err = runtime.Allocate(400000)
	f.must(err)
	f.must(e.Pop())
	count, err = e.set.Len()
	f.must(err)
	if e.popPhase != contextPopNone || len(e.scopes) != 0 || len(e.assertions) != 0 || len(e.rawRules) != 1 || count != 1 || e.revision != revision+1 {
		t.Fatal("retry did not finish the same source scope")
	}
	if _, err = m.RefGet(remaining); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("retry did not release the remaining assertion", err)
	}
}
