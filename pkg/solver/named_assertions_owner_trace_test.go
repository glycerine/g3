//go:build g3trace

package solver

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type namedOwnerStopWriter struct {
	action func()
	called bool
}

func (w *namedOwnerStopWriter) Write(p []byte) (int, error) {
	if !w.called {
		w.called = true
		w.action()
	}
	return len(p), nil
}
func TestNamedAssertionStoppedLocalPrecedesMemberRetirement(t *testing.T) {
	base := solverTestManager(t)
	writer := &namedOwnerStopWriter{}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags})
	solverMust(t, e)
	t.Cleanup(func() { solverMust(t, m.Close()) })
	solverMust(t, m.RegisterDeclarationPlugins())
	n, e := NewNamedAssertions(m, &namedCoreFixture{})
	solverMust(t, e)
	t.Cleanup(func() { solverMust(t, n.Close()) })
	formula := namedOwnerConst(t, m, "stopped-na-formula")
	tag := namedOwnerConst(t, m, "stopped-na-tag")
	_, e = n.appendTemporary([]ast.ASTHandle{tag})
	solverMust(t, e)
	h, e := m.MkImplies(tag, formula)
	solverMust(t, e)
	ref, e := m.NewRef(h)
	solverMust(t, e)
	local := &namedAssertionScope{manager: m, implies: ref}
	var lexical, closeErr error
	sort, e := m.FloatingPointSort(3, 4)
	solverMust(t, e)
	tags.Enable("ast")
	if !tags.ShouldTrace("ast") {
		solverMust(t, local.close())
		t.Fatal("g3trace build did not enable the source trace")
	}
	writer.action = func() { n.finishAssertion(local, &lexical); closeErr = n.Close() }
	_, e = m.MkFPAZeroOfSort(sort, false)
	solverMust(t, e)
	if !writer.called || !errors.Is(lexical, ast.ErrASTPending) || !errors.Is(closeErr, ast.ErrASTPending) {
		t.Fatal("fixture did not reach actual numerical mutation guard", writer.called, lexical, closeErr)
	}
	if len(n.pending) != 1 || n.pending[0] != local || local.implies != ref || len(n.assumptions) != 1 || n.assumptions[0] == (ast.ASTRef{}) || n.closed {
		t.Fatal("stopped local entered member destruction")
	}
	if _, err := n.NumAssumptions(); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("stopped owner still executable", err)
	}
	refs, e := m.GetRefCount(tag)
	solverMust(t, e)
	if refs != 3 {
		t.Fatal("failed cleanup consumed an unrelated member", refs)
	}
	memberAlive := false
	writer.called = false
	writer.action = func() { memberAlive = len(n.assumptions) == 1 && n.assumptions[0] != (ast.ASTRef{}) }
	solverMust(t, n.Close())
	if !writer.called || !memberAlive {
		t.Fatal("member assumptions retired before the stopped local")
	}
	solverMust(t, n.Close())
	if len(n.pending) != 0 || local.implies != (ast.ASTRef{}) {
		t.Fatal("retry kept local owner")
	}
	refs, e = m.GetRefCount(tag)
	solverMust(t, e)
	if refs != 1 {
		t.Fatal("retry replayed or retained pin", refs)
	}
}

func TestNamedAssertionInterruptedRangeRetirementRequiresClose(t *testing.T) {
	for _, mode := range []string{"check", "pop"} {
		t.Run(mode, func(t *testing.T) {
			base := solverTestManager(t)
			writer := &namedOwnerStopWriter{}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags})
			solverMust(t, e)
			t.Cleanup(func() { solverMust(t, m.Close()) })
			solverMust(t, m.RegisterDeclarationPlugins())
			core := &namedCoreFixture{}
			n, e := NewNamedAssertions(m, core)
			solverMust(t, e)
			t.Cleanup(func() { solverMust(t, n.Close()) })
			tag := namedOwnerConst(t, m, "interrupted-na-tag-"+mode)
			// Stage the source boundary after the core operation, before its
			// assumption range retires. Entering the whole public operation
			// inside the AST callback would stop earlier at n.open().
			if mode == "check" {
				_, e = n.appendTemporary([]ast.ASTHandle{tag, tag})
				solverMust(t, e)
				values, err := n.assumptionHandles()
				solverMust(t, err)
				_, e = core.CheckSatCore(values)
				solverMust(t, e)
			} else {
				solverMust(t, n.Push())
				solverMust(t, n.AssertTracked(tag, tag))
				solverMust(t, n.AssertTracked(tag, tag))
				solverMust(t, core.PopCore(1))
			}
			// Retire a real first owner, then interrupt the remaining suffix.
			// Retry must skip this consumed token and preserve the live one.
			solverMust(t, m.DelRef(&n.assumptions[0]))
			sort, e := m.FloatingPointSort(3, 4)
			solverMust(t, e)
			tags.Enable("ast")
			if !tags.ShouldTrace("ast") {
				t.Fatal("g3trace build did not enable the source trace")
			}
			var operationErr error
			writer.action = func() { operationErr = n.restore(0) }
			_, e = m.MkFPAZeroOfSort(sort, false)
			solverMust(t, e)
			if !writer.called || !errors.Is(operationErr, ast.ErrASTPending) || !n.closing {
				t.Fatal("retirement interruption did not close operation gate", writer.called, operationErr, n.closing)
			}
			if len(n.assumptions) != 2 || n.assumptions[0] != (ast.ASTRef{}) || n.assumptions[1] == (ast.ASTRef{}) {
				t.Fatal("retirement interruption lost its consumed prefix or live suffix", n.assumptions)
			}
			refs, e := m.GetRefCount(tag)
			solverMust(t, e)
			if refs != 2 {
				t.Fatal("interrupted retirement changed the remaining pin", refs)
			}
			if _, e = n.NumAssumptions(); !errors.Is(e, ast.ErrASTPending) {
				t.Fatal("partially retired adapter remained callable", e)
			}
			if mode == "pop" && core.pops != 1 {
				t.Fatal("core pop was not completed exactly once", core.pops)
			}
			solverMust(t, n.Close())
			solverMust(t, n.Close())
			if mode == "pop" && core.pops != 1 {
				t.Fatal("cleanup replayed core pop", core.pops)
			}
			refs, e = m.GetRefCount(tag)
			solverMust(t, e)
			if refs != 1 {
				t.Fatal("cleanup retained or replayed assumption", refs)
			}
		})
	}
}
