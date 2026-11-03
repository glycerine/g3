package pdr

// Source pdr_smt_context_manager.cpp::mk_fresh stores each shared-kernel guard
// in an owning app_ref. Reserve the Go reference before constructing that app.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type freshContextGuardFailureWriter struct {
	manager *ast.ASTManager
	line    strings.Builder
	fired   bool
}

func (w *freshContextGuardFailureWriter) Write(p []byte) (int, error) {
	for _, ch := range p {
		w.line.WriteByte(ch)
		if ch != '\n' {
			continue
		}
		line := w.line.String()
		w.line.Reset()
		if !w.fired && strings.HasPrefix(line, "[mk-app] #") && strings.HasSuffix(line, " #context2\n") {
			w.fired = true
			w.manager.TraceStream = nil
			w.manager.Budget().FailAfter(0)
		}
	}
	return len(p), nil
}

func TestSMTContextFreshGuardOwnsAppBeforeReferenceAllocationCanFail(t *testing.T) {
	for _, boundary := range []string{"before-guard", "after-guard"} {
		t.Run(boundary, func(t *testing.T) {
			m, owner, transformer, _, closeBase := closureFixture(t)
			pool, err := NewSMTContextManager(m, nil, 1, owner.contexts.config)
			closureMust(t, err)
			first, err := pool.Fresh()
			closureMust(t, err)
			contexts := []*SMTContext{first}
			var padding []ast.ASTRef
			defer func() {
				m.TraceStream = nil
				m.Budget().FailAfter(-1)
				for _, context := range contexts {
					closureMust(t, context.Close())
				}
				for i := range padding {
					closureMust(t, m.DelRef(&padding[i]))
				}
				closureMust(t, pool.Close())
				closureMust(t, transformer.Close())
				closureMust(t, owner.Close())
				closeBase()
			}()
			writer := &freshContextGuardFailureWriter{manager: m}
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget(), BorrowedWriter: writer})
			closureMust(t, err)
			defer func() { closureMust(t, stream.Close()) }()
			_, err = m.Symbols.Intern("#context2")
			closureMust(t, err)
			boolean, err := m.MkBoolSort()
			closureMust(t, err)
			// Fill real reference storage with ordinary caller roots. The next
			// root now needs an allocation; no internal arena state is changed.
			reached := false
			for i := 0; i < 1024; i++ {
				m.Budget().FailAfter(0)
				ref, cause := m.NewRef(ast.ASTHandle{})
				m.Budget().FailAfter(-1)
				if errors.Is(cause, arena.ErrInjectedFailure) {
					reached = true
					break
				}
				closureMust(t, cause)
				padding = append(padding, ref)
			}
			if !reached {
				t.Fatal("reference allocation boundary was not reached")
			}
			counts := func() [4]uint32 {
				s := m.Stats()
				return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
			}
			before := counts()
			firstGuard, err := first.Predicate()
			closureMust(t, err)
			if boundary == "before-guard" {
				m.Budget().FailAfter(0)
			} else {
				m.TraceStream = stream
			}
			second, cause := pool.Fresh()
			m.TraceStream = nil
			m.Budget().FailAfter(-1)
			if second != nil {
				contexts = append(contexts, second)
			}
			if pool.created != 2 || len(pool.solvers) != 1 || first.solver != pool.solvers[0] || first.predicate != firstGuard {
				t.Fatal("failed guard allocation changed the source counter or shared kernel")
			}
			if boundary == "before-guard" {
				if second != nil || !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("guard reservation did not reject before publication", second, cause)
				}
				if after := counts(); after != before || pool.active != 1 || len(pool.predicates) != 1 {
					t.Fatal("failed guard creation leaked AST ownership or changed live contexts", before, after, pool.active, len(pool.predicates))
				}
			} else {
				// The completed source mk-app event arms allocation failure.
				// Fresh already owns its reserved reference, so no later reference
				// allocation may turn the registered guard into an orphan.
				if !writer.fired || cause != nil || second == nil {
					t.Fatal("guard lacked an owner at its completed construction event", writer.fired, second, cause)
				}
				want := before
				want[0] += 2 // the guard declaration and its nullary application
				want[3]++
				if after := counts(); after != want || pool.active != 2 || len(pool.predicates) != 2 {
					t.Fatal("successful guard publication has incorrect ownership", want, after, pool.active, len(pool.predicates))
				}
				guard, err := second.Predicate()
				closureMust(t, err)
				rc, err := m.GetRefCount(guard)
				closureMust(t, err)
				if rc != 1 {
					t.Fatal("guard is not owned by the shared context manager", rc)
				}
			}
			// Both the failed and successful second attempts advance the source
			// creation counter. The next actual context must use a distinct guard.
			retry, err := pool.Fresh()
			closureMust(t, err)
			contexts = append(contexts, retry)
			guard, err := retry.Predicate()
			closureMust(t, err)
			declaration, err := m.GetDecl(guard)
			closureMust(t, err)
			name, err := m.GetASTName(declaration)
			closureMust(t, err)
			text, err := m.Symbols.String(name)
			closureMust(t, err)
			auxiliary, err := retry.IsAuxPredicate(declaration)
			closureMust(t, err)
			if text != "#context3" || !auxiliary || retry.solver != first.solver || guard == firstGuard {
				t.Fatal("retry lost source guard identity or shared solver selection", text, auxiliary)
			}
			p, err := m.MkFreshConst("fresh-guard-isolation", boolean)
			closureMust(t, err)
			ref, err := m.NewRef(p)
			closureMust(t, err)
			padding = append(padding, ref)
			notP, err := m.MkNot(p)
			closureMust(t, err)
			ref, err = m.NewRef(notP)
			closureMust(t, err)
			padding = append(padding, ref)
			closureMust(t, first.Assert(p))
			closureMust(t, retry.Assert(notP))
			for _, check := range []struct {
				context     *SMTContext
				assumptions []ast.ASTHandle
				want        lbool.LBool
			}{
				{first, nil, lbool.True},
				{retry, nil, lbool.True},
				{first, []ast.ASTHandle{notP}, lbool.False},
				{retry, []ast.ASTHandle{p}, lbool.False},
			} {
				status, err := check.context.Check(check.assumptions)
				closureMust(t, err)
				if status != check.want {
					t.Fatal("guard retry changed shared-kernel isolation", status, check.want)
				}
			}
		})
	}
}
