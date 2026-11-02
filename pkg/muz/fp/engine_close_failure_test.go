package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newAdmittedCloseFailureFixture(t *testing.T) (*spacerGeneralizerExpressionFixture, ast.ASTHandle) {
	t.Helper()
	f := newSpacerGeneralizerExpressionFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("close-unreachable-goal")
	must(err)
	declaration, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(declaration))
	goal, err := m.MkApp(declaration, nil)
	must(err)
	rule, err := m.MkImplies(f.query, goal)
	must(err)
	must(e.AddRule(rule, "negative-goal"))
	status, err := e.QueryPredicate(declaration)
	must(err)
	if status != lbool.False || e.spacer == nil || e.queryClone != nil {
		t.Fatal("expected direct unreachable Spacer query", status)
	}
	if result, err := e.LastResult(); err != nil || result != status {
		t.Fatal("missing initial result", result, err)
	}

	return f, declaration
}

func TestEngineAdmittedCloseFailurePreservesBackendAndInvalidatesResults(t *testing.T) {
	f, declaration := newAdmittedCloseFailureFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	var err error
	backend := e.spacer
	for i := 0; i < 2; i++ {
		m.Budget().FailAfter(0)
		err = e.Close()
		m.Budget().FailAfter(-1)
		if !errors.Is(err, arena.ErrInjectedFailure) || e.closed || e.spacer != backend {
			t.Fatal("refused backend close lost its owner", i, err)
		}
		if _, err = e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
			t.Fatal("disposing engine published a query result", err)
		}
		if _, err = e.NumLevels(declaration); !errors.Is(err, ErrPropositionalEngine) {
			t.Fatal("disposing engine published frames", err)
		}
		if _, err = e.QueryPredicate(declaration); !errors.Is(err, ErrPropositionalEngine) {
			t.Fatal("disposing engine accepted a query", err)
		}
	}
	must(e.Close())
	must(e.Close())
	if !e.closed || e.spacer != nil || e.spacerConversions != nil {
		t.Fatal("engine close retry left backend owners")
	}
}

func TestEngineAdmittedCloseAllocationFailureAndRetry(t *testing.T) {
	complete, failures := false, 0
	for stop := int64(0); stop < 128 && !complete; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f, declaration := newAdmittedCloseFailureFixture(t)
			e, m := f.engine, f.manager
			m.Budget().FailAfter(stop)
			err := e.Close()
			m.Budget().FailAfter(-1)
			if err != nil {
				failures++
				if !errors.Is(err, arena.ErrInjectedFailure) || e.closed {
					t.Fatal("admitted cleanup failure was suppressed or marked complete", stop, err)
				}
				if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("disposing engine exposed result", err)
				}
				if _, err := e.QueryPredicate(declaration); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("disposing engine accepted work", err)
				}
			} else {
				complete = true
			}
			fragmentMust(t, e.Close())
			fragmentMust(t, e.Close())
		})
		if t.Failed() {
			return
		}
	}
	if !complete || failures == 0 {
		t.Fatal("missing disposal allocation failure/success frontier", failures, complete)
	}
}
