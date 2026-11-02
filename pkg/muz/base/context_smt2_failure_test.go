package base_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

func contextDumpFailureFixture(t *testing.T) (*ruleSetFixture, *base.RuleSet) {
	f := newRuleSetFixture(t, "R")
	m := f.manager
	boolean, err := m.MkBoolSort()
	ruleMust(t, err)
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	for _, name := range []string{"dump-P", "dump-Q"} {
		symbol, err := m.Symbols.Intern(name)
		ruleMust(t, err)
		d, err := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, err)
		ruleMust(t, f.rules.RegisterPredicate(d))
		x, err := m.MkVar(0, boolean)
		ruleMust(t, err)
		head, err := m.MkApp(d, []ast.ASTHandle{x})
		ruleMust(t, err)
		var tail []ast.ASTHandle
		if name == "dump-P" {
			tail = []ast.ASTHandle{f.app["R"]}
		} else {
			ruleMust(t, set.SetOutputPredicate(d))
		}
		r, err := f.rules.NewRule(head, tail, nil, name)
		ruleMust(t, err)
		ruleMust(t, set.Add(r))
		ruleMust(t, r.Close())
	}
	return f, set
}

func TestContextSMT2AllocationFailureOwnershipAndRetry(t *testing.T) {
	complete, failures := false, 0
	for allowance := int64(0); allowance < 2048 && !complete; allowance++ {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f, set := contextDumpFailureFixture(t)
			before := f.manager.Stats().LiveReferences
			var out bytes.Buffer
			f.manager.Budget().FailAfter(allowance)
			failure := set.WriteContextSMT2(&out, base.DefaultContextSMT2Config())
			f.manager.Budget().FailAfter(-1)
			if failure != nil {
				failures++
				var cleanup memory.ConstructionFailure
				if errors.As(failure, &cleanup) {
					ruleMust(t, cleanup.Cleanup())
				}
				var resource memory.Failure
				if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &resource) || resource.Kind != memory.FailureMemout) {
					t.Fatal("unexpected context dump failure", failure)
				}
			} else {
				complete = true
			}
			if f.manager.Stats().LiveReferences != before {
				t.Fatal("context dump lost an owner", allowance, failure)
			}
			out.Reset()
			ruleMust(t, set.WriteContextSMT2(&out, base.DefaultContextSMT2Config()))
			if out.Len() == 0 {
				t.Fatal("retry produced no dump")
			}
			if f.manager.Stats().LiveReferences != before {
				t.Fatal("context dump retry leaked references")
			}
		})
	}
	if !complete || failures == 0 {
		t.Fatal("incomplete context dump failure campaign", complete, failures)
	}
}

type contextDumpRefusingWriter struct{ remaining int }

func (w *contextDumpRefusingWriter) Write(body []byte) (int, error) {
	if len(body) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(body)
	return len(body), nil
}
func TestContextSMT2WriterFailureReleasesTemporaryOwners(t *testing.T) {
	f, set := contextDumpFailureFixture(t)
	for _, limit := range []int{0, 30, 80, 160} {
		before := f.manager.Stats().LiveReferences
		failure := set.WriteContextSMT2(&contextDumpRefusingWriter{remaining: limit}, base.DefaultContextSMT2Config())
		if !errors.Is(failure, io.ErrClosedPipe) {
			t.Fatal("writer refusal", limit, failure)
		}
		if f.manager.Stats().LiveReferences != before {
			t.Fatal("writer refusal leaked references", limit)
		}
	}
	var out bytes.Buffer
	ruleMust(t, set.WriteContextSMT2(&out, base.DefaultContextSMT2Config()))
}
