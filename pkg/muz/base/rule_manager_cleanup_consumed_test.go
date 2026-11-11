package base

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

// Source: src/util/memory_manager.cpp signed TLS accounting and
// src/ast/ast.cpp declaration-info destruction. Public reallocations put the
// real accounting lane near its signed lower bound; final rational destruction
// physically releases the allocation before reporting the accounting error.
func TestRuleManagerCloseRetriesAfterConsumedNativeRelease(t *testing.T) {
	testHornCleanupConsumedRelease(t, false)
}

func TestHornNormalizerResetFinishesPartiallyConsumedBody(t *testing.T) {
	testHornCleanupConsumedRelease(t, true)
}

func testHornCleanupConsumedRelease(t *testing.T, bodyCleanup bool) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	b := arena.NewBudget(1 << 28)
	rt, err := memory.NewRuntime(memory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64})
	must(err)
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: rt, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	must(err)
	sy, err := symbol.NewManager(symbol.Config{Budget: b})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
	must(err)
	rm, err := NewRuleManager(m)
	must(err)
	scoped, err := numeral.NewScopedInt(q.Manager.Z)
	must(err)
	n, err := scoped.Get()
	must(err)
	digits := make([]uint32, 32768)
	digits[0], digits[len(digits)-1] = 17, 1
	must(q.Manager.Z.SetLimbs(n, digits))
	v, err := q.NewFromScopedInt(scoped)
	must(err)
	p, err := m.NewParameterRational(&v)
	must(err)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{p}, ast.ASTFuncFlags{}, false)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := sy.Intern("consumed-native-rule-manager")
	must(err)
	d, err := m.MkFuncDeclWithInfo(name, nil, boolean, info)
	must(err)
	var remaining ast.ASTRef
	if bodyCleanup {
		large, cause := m.MkApp(d, nil)
		must(cause)
		first, cause := m.MkFreshConst("body-before-large", boolean)
		must(cause)
		last, cause := m.MkFreshConst("body-after-large", boolean)
		must(cause)
		cache, cause := m.MkFreshConst("body-cleanup-cache", boolean)
		must(cause)
		must(rm.normalizer.storeHornBody([]ast.ASTHandle{first, large, last}))
		must(rm.normalizer.rememberHornRoot(cache))
		remaining = rm.normalizer.bodyRefs[0]
	} else {
		must(rm.RegisterPredicate(d))
	}
	must(m.DelFuncDeclInfo(&info))
	must(m.DelParameter(&p))
	must(q.Del(&v))
	must(scoped.Close())
	if count, err := m.GetRefCount(d); err != nil || count != 1 {
		t.Fatal("not sole declaration owner", count, err)
	}
	a, err := rt.Allocate(8)
	must(err)
	c, err := rt.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = rt.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal(err)
	}
	must(rt.Deallocate(&a))
	state := rt.State()
	delta = math.MinInt64 + 1000 - state.GlobalSize + 16
	if err = rt.Reallocate(&c, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal(err)
	}
	must(rt.Deallocate(&c))
	if state = rt.State(); state.GlobalSize != math.MinInt64+1000 || state.LocalSize != 0 {
		t.Fatal("failed real TLS setup", state)
	}
	before := m.Stats().LiveReferences
	blocks := rt.Stats().LiveBlocks
	if bodyCleanup {
		normalizer := rm.normalizer
		err = normalizer.beginHornStep()
		if !errors.Is(err, arena.ErrBounds) || normalizer.closed || normalizer.cleanupPending != hornCleanupBody || len(normalizer.bodyRefs) != 1 || normalizer.bodyRefs[0] != remaining || len(normalizer.workRefs) != 1 || m.Stats().LiveReferences+2 != before || rt.Stats().LiveBlocks >= blocks {
			t.Fatal("partially consumed body lost its remaining owners", err, normalizer.cleanupPending, len(normalizer.bodyRefs), len(normalizer.workRefs), before, m.Stats().LiveReferences)
		}
		if _, cause := m.RefGet(remaining); cause != nil {
			t.Fatal("remaining body owner is not live", cause)
		}
		if result, cause := normalizer.Normalize(ast.ASTHandle{}); !errors.Is(cause, ErrHornNormalizationOwner) || result != nil {
			t.Fatal("partially consumed body accepted new work", cause)
		}
	} else {
		err = rm.Close()
		// m_rwr's three scratch references are released before the
		// predicate table's consumed release.
		if !errors.Is(err, arena.ErrBounds) || rm.closed || !rm.cleanupPending || len(rm.predicates) != 0 || m.Stats().LiveReferences+4 != before || rt.Stats().LiveBlocks >= blocks {
			t.Fatal("consumed RuleManager frontier", err, rm.closed, rm.cleanupPending, len(rm.predicates), before, m.Stats().LiveReferences, blocks, rt.Stats().LiveBlocks)
		}
		if err = rm.RegisterPredicate(d); !errors.Is(err, ErrRuleOwner) {
			t.Fatal("cleanup-pending manager accepted predicate registration", err)
		}
		if rule, err := rm.NewRule(ast.ASTHandle{}, nil, nil, "blocked"); !errors.Is(err, ErrRuleOwner) || rule != nil {
			t.Fatal("cleanup-pending manager accepted rule construction", err)
		}
	}
	recovery, err := rt.Allocate(400000)
	must(err)
	state, blocks = rt.State(), rt.Stats().LiveBlocks
	if bodyCleanup {
		must(rm.normalizer.Reset())
		if rm.normalizer.cleanupPending != hornCleanupNone || len(rm.normalizer.bodyRefs) != 0 || len(rm.normalizer.workRefs) != 0 || m.Stats().LiveReferences+4 != before {
			t.Fatal("reset retry did not finish body and cache owners")
		}
		if _, cause := m.RefGet(remaining); !errors.Is(cause, ast.ErrASTHandle) {
			t.Fatal("reset retry did not consume the remaining owner", cause)
		}
		must(rm.normalizer.Reset())
	}
	must(rm.Close())
	must(rm.Close())
	if !rm.closed || rm.cleanupPending || rt.State() != state || rt.Stats().LiveBlocks != blocks {
		t.Fatal("retry replayed consumed decrement")
	}
	finish := func(close func() error) {
		t.Helper()
		for i := 0; i < 128; i++ {
			before := rt.Stats().LiveBlocks
			err := close()
			if err == nil {
				return
			}
			if !errors.Is(err, arena.ErrBounds) || rt.Stats().LiveBlocks >= before {
				t.Fatal("cleanup no physical progress", err, rt.State())
			}
		}
		t.Fatal("cleanup horizon")
	}
	finish(m.Close)
	finish(q.Close)
	finish(sy.Close)
	if err = rt.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
		t.Fatal(err)
	}
	finish(rt.Close)
	if b.Used() != 0 {
		t.Fatal("budget leak", b.Used())
	}
}
