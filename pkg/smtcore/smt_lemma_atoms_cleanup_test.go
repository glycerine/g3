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

type smtNativeCleanupFixture struct {
	manager *ast.ASTManager
	state   *SMTInternalizerState
	numbers *numeral.RationalService
	runtime *memory.Runtime
	budget  *arena.Budget
	action  func()
	called  bool
}

func newSMTNativeCleanupFixture(t *testing.T) *smtNativeCleanupFixture {
	t.Helper()
	f := &smtNativeCleanupFixture{budget: arena.NewBudget(1 << 28)}
	var err error
	f.runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: f.budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if f.action != nil && !f.called {
			f.called = true
			f.action()
		}
		return source
	}})
	astTestMust(t, err)
	f.numbers, err = numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: f.runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: f.budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: f.budget}}}})
	astTestMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: f.budget})
	astTestMust(t, err)
	f.manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: f.numbers, Budget: f.budget})
	astTestMust(t, err)
	t.Cleanup(func() {
		if f.state != nil {
			astTestMust(t, f.state.Close())
		}
		astTestMust(t, f.manager.Close())
		astTestMust(t, f.numbers.Close())
		astTestMust(t, symbols.Close())
		astTestMust(t, f.runtime.Close())
		if f.budget.Used() != 0 {
			t.Fatal("SMT native cleanup fixture leaked bytes", f.budget.Used())
		}
	})
	_, err = f.manager.RegisterArithmeticPlugin()
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(f.manager)
	astTestMust(t, err)
	f.state, err = NewSMTInternalizerState(f.manager, context, nil)
	astTestMust(t, err)
	return f
}

// refuse runs the test action through the real numeral transport while its
// AST invocation owns the manager. Ordinary AST cleanup must then refuse.
func (f *smtNativeCleanupFixture) refuse(t *testing.T, action func()) error {
	t.Helper()
	value, err := f.numbers.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.numbers.Del(&value)) }()
	astTestMust(t, f.numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	astTestMust(t, f.runtime.SetMaxCount(uint64(f.runtime.State().GlobalCount)+1))
	var invocation ast.ASTArithmeticNumeralInvocation
	f.action, f.called = action, false
	_, body := f.manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	f.action = nil
	astTestMust(t, f.runtime.SetMaxCount(0))
	astTestMust(t, f.manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !f.called {
		t.Fatal("native allocation refusal did not invoke cleanup action", body, f.called)
	}
	return body
}

func TestSMTLemmaAtomCaptureTransfersPartialPrefixAndReleaseIsIdempotent(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	expression := smtRelevancyBool(t, m, "lemma-partial-prefix")
	astTestMust(t, state.Internalize(expression, true))
	astTestMust(t, state.relevancy.MarkRelevant(expression))
	literal, err := state.Literal(expression)
	astTestMust(t, err)
	before, err := m.GetRefCount(expression)
	astTestMust(t, err)
	literals := []SMTLiteral{literal.Negated(), NewSMTLiteral(int32(len(state.boolExpr)+1), false)}
	atoms, err := state.captureLemmaAtoms(literals)
	if !errors.Is(err, ast.ErrASTDomain) || len(atoms) != 1 || atoms[0].expression != expression || !atoms[0].sign || !atoms[0].relevant {
		t.Fatal("failed capture did not transfer its owned prefix and metadata", atoms, err)
	}
	after, err := m.GetRefCount(expression)
	astTestMust(t, err)
	if after != before+1 {
		t.Fatal("partial capture did not own its atom", before, after)
	}
	astTestMust(t, state.releaseLemmaAtoms(atoms))
	astTestMust(t, state.releaseLemmaAtoms(atoms))
	if !atoms[0].expression.IsNull() {
		t.Fatal("release did not retire its consumed handle")
	}
	atoms, err = state.retainLemmaAtoms(literals)
	if !errors.Is(err, ast.ErrASTDomain) || atoms != nil {
		t.Fatal("failed owning capture published an incomplete lemma", atoms, err)
	}
	after, err = m.GetRefCount(expression)
	astTestMust(t, err)
	if after != before {
		t.Fatal("partial acquisition cleanup changed caller ownership", before, after)
	}
}

func TestSMTLemmaAtomCleanupRetriesNativeRefusal(t *testing.T) {
	for _, stage := range []string{"direct", "nested", "consumed-tail", "closed-state"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newSMTNativeCleanupFixture(t)
			m, state, runtime := fixture.manager, fixture.state, fixture.runtime
			var atoms, childAtoms []smtRetainedLemmaAtom
			var failure error
			operationCause := errors.New("learned-clause backjump failed")
			var callerRefs []ast.ASTRef
			t.Cleanup(func() {
				for i := len(callerRefs) - 1; i >= 0; i-- {
					astTestMust(t, m.DelRef(&callerRefs[i]))
				}
			})
			var expressions []ast.ASTHandle
			var literals []SMTLiteral
			for _, name := range []string{"lemma-cleanup-first", "lemma-cleanup-last"} {
				expression := smtRelevancyBool(t, m, name)
				ref, err := m.NewRef(expression)
				astTestMust(t, err)
				callerRefs = append(callerRefs, ref)
				astTestMust(t, state.Internalize(expression, true))
				literal, err := state.Literal(expression)
				astTestMust(t, err)
				expressions = append(expressions, expression)
				literals = append(literals, literal)
			}
			var err error
			atoms, err = state.retainLemmaAtoms(literals)
			astTestMust(t, err)
			if stage == "nested" {
				childAtoms, err = state.retainLemmaAtoms(literals[:1])
				astTestMust(t, err)
			}
			if stage == "consumed-tail" {
				// Model an already completed reverse-order release before the
				// older atom encounters a native cleanup refusal.
				astTestMust(t, state.releaseLemmaAtoms(atoms[1:]))
			}
			beforeAtoms := append([]smtRetainedLemmaAtom(nil), atoms...)
			beforeChild := append([]smtRetainedLemmaAtom(nil), childAtoms...)
			beforeCounts := make([]uint32, len(expressions))
			for i, expression := range expressions {
				beforeCounts[i], err = m.GetRefCount(expression)
				astTestMust(t, err)
			}
			body := fixture.refuse(t, func() {
				var cause error
				if stage != "direct" {
					cause = operationCause
				}
				if stage == "nested" {
					cause = state.disposeLemmaAtoms(childAtoms, cause)
				}
				failure = state.disposeLemmaAtoms(atoms, cause)
			})
			if body == nil || !errors.Is(failure, ast.ErrASTPending) || stage != "direct" && !errors.Is(failure, operationCause) {
				t.Fatal("native refusal lost its cleanup or originating cause", body, failure)
			}
			for i := range atoms {
				if atoms[i] != beforeAtoms[i] {
					t.Fatal("refusal consumed a lemma atom", i)
				}
			}
			for i := range childAtoms {
				if childAtoms[i] != beforeChild[i] {
					t.Fatal("refusal consumed a nested lemma atom", i)
				}
			}
			for i, expression := range expressions {
				count, err := m.GetRefCount(expression)
				astTestMust(t, err)
				if count != beforeCounts[i] {
					t.Fatal("native refusal changed atom ownership", i, beforeCounts[i], count)
				}
			}
			var cleanup memory.ConstructionFailure
			if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
				t.Fatal("missing concrete lemma cleanup receipt", failure)
			}
			original, exposed, message := cleanup.OriginalCause(), cleanup.Unwrap(), cleanup.Error()
			if stage == "closed-state" {
				astTestMust(t, state.Close())
				for i, expression := range expressions {
					beforeCounts[i], err = m.GetRefCount(expression)
					astTestMust(t, err)
				}
				if cleanup.RuntimeOwner() != runtime {
					t.Fatal("closing internalizer detached escaped atom cleanup")
				}
			}
			astTestMust(t, cleanup.Cleanup())
			astTestMust(t, cleanup.Cleanup())
			for i, expression := range expressions {
				want := beforeCounts[i]
				if !beforeAtoms[i].expression.IsNull() {
					want--
				}
				if stage == "nested" && i == 0 {
					want--
				}
				count, err := m.GetRefCount(expression)
				astTestMust(t, err)
				if !atoms[i].expression.IsNull() || count != want {
					t.Fatal("cleanup retry released the wrong atom ownership", i, want, count, atoms[i])
				}
			}
			for _, atom := range childAtoms {
				if !atom.expression.IsNull() {
					t.Fatal("cleanup retry left nested atom ownership")
				}
			}
			if cleanup.OriginalCause() != original || cleanup.Unwrap() != exposed || cleanup.Error() != message || cleanup.CleanupCause() != nil {
				t.Fatal("cleanup retry changed stable failure diagnostics")
			}
		})
	}
}
