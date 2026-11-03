package fp

// Source: spacer_context.cpp get_rules_along_trace and dl_interface forwarding.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"reflect"
	"testing"
)

func TestSpacerExpressionTraceFailurePublicationAndRetry(t *testing.T) {
	for _, spare := range []int{0, 1} {
		t.Run(fmt.Sprintf("spare=%d", spare), func(t *testing.T) {
			testSpacerExpressionTraceFailure(t, spare)
		})
	}
}

func testSpacerExpressionTraceFailure(t *testing.T, spare int) {
	t.Helper()
	success, failures := false, 0
	for stop := int64(0); stop < 512 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newExpressionFixture(t, "spacer", false)
			e, m := f.engine, f.manager
			fragmentMust(t, e.globals.SourceSetName("fixedpoint.xform.subsumption_checker", "false"))
			status, err := e.QueryExpression(f.query)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("query", status)
			}
			// Fill the current reference page so trace publication crosses an
			// actual allocation boundary instead of reusing its spare slots.
			var held []ast.ASTRef
			defer func() {
				for i := range held {
					fragmentMust(t, m.DelRef(&held[i]))
				}
			}()
			frontier := false
			for i := 0; i < 4096; i++ {
				m.Budget().FailAfter(0)
				ref, cause := m.NewRef(ast.ASTHandle{})
				m.Budget().FailAfter(-1)
				if errors.Is(cause, arena.ErrInjectedFailure) {
					frontier = true
					break
				}
				fragmentMust(t, cause)
				held = append(held, ref)
			}
			if !frontier {
				t.Fatal("reference allocation boundary not reached")
			}
			for i := 0; i < spare; i++ {
				last := len(held) - 1
				fragmentMust(t, m.DelRef(&held[last]))
				held = held[:last]
			}
			before := m.Stats().LiveReferences
			release := func(forms []ast.ASTRef) {
				for i := range forms {
					fragmentMust(t, m.DelRef(&forms[i]))
				}
			}
			// The synthetic query has a null symbol, whose source str() is
			// "<null>"; an explicitly empty rule label would print "".
			want := []string{"<null>", "failed-expression-zero"}
			m.Budget().FailAfter(stop)
			forms, names, failure := e.RulesAlongTrace()
			m.Budget().FailAfter(-1)
			if failure != nil {
				failures++
				if forms != nil || names != nil {
					t.Fatal("failed trace published partial arrays", len(forms), names)
				}
				var cleanup memory.ConstructionFailure
				if errors.As(failure, &cleanup) {
					fragmentMust(t, cleanup.Cleanup())
				}
				var resource memory.Failure
				if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &resource) || resource.Kind != memory.FailureMemout) {
					t.Fatal("unexpected trace failure", failure)
				}
			} else {
				success = true
				if !reflect.DeepEqual(names, want) {
					t.Fatal("trace order", names, want)
				}
				release(forms)
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("trace construction leaked references", before, got, failure)
			}
			forms, names, err = e.RulesAlongTrace()
			fragmentMust(t, err)
			if !reflect.DeepEqual(names, want) || len(forms) != 2 {
				t.Fatal("retry trace", names)
			}
			fact, err := m.RefGet(forms[1])
			fragmentMust(t, err)
			if fact != f.fact {
				t.Fatal("trace leaf is not the original numeric fact", fact, f.fact)
			}
			typedForms, labels, err := e.RulesAlongTraceWithSymbols()
			fragmentMust(t, err)
			leafLabel, err := m.Symbols.Intern("failed-expression-zero")
			fragmentMust(t, err)
			if len(typedForms) != 2 || len(labels) != 2 || !labels[0].IsNull() || labels[1] != leafLabel {
				t.Fatal("retry trace lost the synthetic null or original leaf label", labels)
			}
			for i, ref := range typedForms {
				typed, err := m.RefGet(ref)
				fragmentMust(t, err)
				text, err := m.RefGet(forms[i])
				fragmentMust(t, err)
				if typed != text {
					t.Fatal("typed retry trace changed formula order", i)
				}
			}
			release(typedForms)
			fragmentMust(t, e.AddRule(f.fact, "retire-trace-query"))
			m.Budget().FailAfter(0)
			stale, staleNames, staleErr := e.RulesAlongTrace()
			m.Budget().FailAfter(-1)
			if stale != nil || staleNames != nil || !errors.Is(staleErr, ErrPropositionalEngine) {
				t.Fatal("retired trace", staleNames, staleErr)
			}
			fragmentMust(t, e.clearQueryResources())
			got, err := m.RefGet(forms[1])
			fragmentMust(t, err)
			if got != fact {
				t.Fatal("caller trace lost original leaf after query disposal")
			}
			release(forms)
		})
		if t.Failed() {
			break
		}
	}
	if failures == 0 || !success {
		t.Fatal("incomplete trace frontier", failures, success)
	}
}
