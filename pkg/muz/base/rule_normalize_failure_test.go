package base_test

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestRuleNormalizeFailureOwnsSparseVariableSubstitution(t *testing.T) {
	failures, done := 0, false
	for limit := int64(0); limit < 80; limit++ {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			f := newRuleAnalysisFixture(t)
			m := f.manager
			fixtureRef, err := m.NewRef(f.head)
			ruleMust(t, err)
			defer func() { ruleMust(t, m.DelRef(&fixtureRef)) }()
			x, err := m.MkVar(3, f.integer)
			ruleMust(t, err)
			y, err := m.MkVar(5, f.integer)
			ruleMust(t, err)
			head, err := m.MkApp(f.pred, []ast.ASTHandle{x})
			ruleMust(t, err)
			tail, err := m.MkApp(f.pred, []ast.ASTHandle{y})
			ruleMust(t, err)
			headRef, err := m.NewRef(head)
			ruleMust(t, err)
			defer func() { ruleMust(t, m.DelRef(&headRef)) }()
			tailRef, err := m.NewRef(tail)
			ruleMust(t, err)
			defer func() { ruleMust(t, m.DelRef(&tailRef)) }()
			_, err = m.Symbols.Intern("sparse-failure")
			ruleMust(t, err)
			before := m.Stats()
			m.Budget().FailAfter(limit)
			rule, cause := f.rules.NewRule(head, []ast.ASTHandle{tail}, nil, "sparse-failure")
			m.Budget().FailAfter(-1)
			if cause != nil {
				if rule != nil || !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal(rule, cause)
				}
				failures++
			} else {
				done = true
				ruleMust(t, rule.Close())
			}
			after := m.Stats()
			if after.LiveReferences != before.LiveReferences || after.LiveNodes != before.LiveNodes {
				t.Fatal("normalization leaked", before, after, cause)
			}
			if cause != nil {
				rule, err = f.rules.NewRule(head, []ast.ASTHandle{tail}, nil, "sparse-failure")
				ruleMust(t, err)
				h, e := rule.Head()
				ruleMust(t, e)
				b, _, e := rule.Tail(0)
				ruleMust(t, e)
				for i, application := range []ast.ASTHandle{h, b} {
					argument, e := m.GetArg(application, 0)
					ruleMust(t, e)
					index, e := m.GetVarIndex(argument)
					ruleMust(t, e)
					if index != uint32(i) {
						t.Fatal("retry changed normalized index", i, index)
					}
				}
				ruleMust(t, rule.Close())
			}
		})
		if done {
			break
		}
	}
	if failures == 0 || !done {
		t.Fatal("incomplete failure sweep", failures, done)
	}
}
