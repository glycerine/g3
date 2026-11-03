package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Native allocation transport runs while the AST manager owns its execution
// guard. Cleanup from that callback must preserve each refused owner for retry.
func smtQuantifierRefusalFixture(t *testing.T, refuse func()) (*ast.ASTManager, *SMTQuantifierRegistry, *SMTQuantifierManager, func()) {
	t.Helper()
	budget := arena.NewBudget(1 << 28)
	callbacks := 0
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		callbacks++
		refuse()
		return f
	}})
	astTestMust(t, err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	astTestMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	astTestMust(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, m.Close())
		astTestMust(t, numbers.Close())
		astTestMust(t, symbols.Close())
		astTestMust(t, runtime.Close())
		if used := budget.Used(); used != 0 {
			t.Fatal("quantifier scope cleanup leaked bytes", used)
		}
	})
	_, err = m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	costs, err := NewSMTQICostModel(m, "0", "0", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, costs.Close()) })
	registry, err := NewSMTQuantifierRegistry(m, NewSMTQIQueueSchedule(m, 10, 20, false), costs, 100)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, registry)
	astTestMust(t, err)
	manager, err := NewSMTQuantifierManager(state, smtQuickTheoryRewriter(t, m), SMTQuantifierManagerConfig{})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	trigger := func() {
		value, err := numbers.NewValue()
		astTestMust(t, err)
		defer func() { astTestMust(t, numbers.Del(&value)) }()
		astTestMust(t, numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
		astTestMust(t, runtime.SetMaxCount(uint64(runtime.State().GlobalCount)+1))
		var invocation ast.ASTArithmeticNumeralInvocation
		_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
		astTestMust(t, runtime.SetMaxCount(0))
		astTestMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
		if body == nil || callbacks == 0 {
			t.Fatal("native allocation transport was not reached", body, callbacks)
		}
	}
	return m, registry, manager, trigger
}

func TestSMTQuantifierPopResumesAfterNativeCleanupRefusal(t *testing.T) {
	for _, component := range []string{"manager", "registry", "mam"} {
		t.Run(component, func(t *testing.T) {
			var pop func(uint32) error
			var refused error
			var guardedQuantifier ast.ASTHandle
			m, registry, manager, trigger := smtQuantifierRefusalFixture(t, func() { refused = pop(1) })
			switch component {
			case "manager":
				astTestMust(t, manager.PushScope())
				quantifier, _, sort := smtManagedQuantifier(t, m, "scope-refusal")
				astTestMust(t, manager.modelFinder.RegisterQuantifier(quantifier))
				term, err := m.MkConst(astTestSymbol(t, m, "scope-refusal-term"), sort)
				astTestMust(t, err)
				_, err = manager.modelFinder.InsertInstantiationTerm(quantifier, 0, term, 0)
				astTestMust(t, err)
				pop = manager.PopScopes
			case "registry":
				guardedQuantifier, _, _ = smtManagedQuantifier(t, m, "registry-refusal")
				astTestMust(t, registry.Add(guardedQuantifier, 0))
				astTestMust(t, registry.PushScope())
				truth, err := m.MkTrue()
				astTestMust(t, err)
				astTestMust(t, registry.schedule.RecordInstance(truth))
				pop = registry.PopScopes
			case "mam":
				quantifier, predicate, sort := smtManagedQuantifier(t, m, "mam-refusal")
				variable, err := m.MkVar(0, sort)
				astTestMust(t, err)
				body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
				astTestMust(t, err)
				pattern, err := m.MkPattern([]ast.ASTHandle{body})
				astTestMust(t, err)
				astTestMust(t, manager.mam.AddPattern(quantifier, pattern))
				astTestMust(t, manager.mam.PushScope())
				pop = manager.mam.PopScopes
			}
			trigger()
			if !errors.Is(refused, ast.ErrASTPending) {
				t.Fatal("scope cleanup did not report native refusal", refused)
			}
			if component == "registry" {
				if _, err := registry.AddInstance(guardedQuantifier, ast.ASTHandle{}, nil, 0, 0, 0); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("registry accepted an instance during cleanup", err)
				}
				if err := registry.Add(guardedQuantifier, 0); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("registry accepted a quantifier during cleanup", err)
				}
				if err := registry.InitSearch(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("registry started a search during cleanup", err)
				}
			}
			if component == "mam" {
				entry := manager.mam.patterns[0]
				if err := manager.mam.AddPattern(entry.quantifier, entry.pattern); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("MAM accepted a pattern during cleanup", err)
				}
				if err := manager.mam.Match(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("MAM matched patterns during cleanup", err)
				}
				if err := manager.mam.Rematch(false); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("MAM rematched patterns during cleanup", err)
				}
			}
			if component == "manager" && (manager.pendingPop == nil || manager.pendingPop.stage != 2 || len(manager.mam.scopes) != 0 || len(manager.lazyMAM.scopes) != 0 || len(manager.modelFinder.scopes) != 1) {
				t.Fatal("manager lost completed child stages", manager.pendingPop, len(manager.mam.scopes), len(manager.lazyMAM.scopes), len(manager.modelFinder.scopes))
			}
			if err := pop(2); !errors.Is(err, ast.ErrASTPending) {
				t.Fatal("pending cleanup accepted a different scope count", err)
			}
			astTestMust(t, pop(1))
			if component == "manager" && (manager.pendingPop != nil || len(manager.activationScopes) != 0 || len(manager.modelFinder.quantifiers) != 0) {
				t.Fatal("manager retry left scopes or quantifiers")
			}
			if component == "registry" && (registry.scopeLevel != 0 || len(registry.schedule.instances) != 0 || len(registry.fingerprints.scopes) != 0) {
				t.Fatal("registry retry left scope owners")
			}
			if component == "mam" && (manager.mam.pendingPop != 0 || len(manager.mam.scopes) != 0 || len(manager.mam.patterns) != 1) {
				t.Fatal("MAM retry lost surviving patterns or kept its scope")
			}
		})
	}
}
