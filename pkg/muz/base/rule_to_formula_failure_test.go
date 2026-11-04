package base_test

// Source: dl_rule.cpp to_formula scopes its negated body, conjunction and
// quantified result. A failed conversion must release the temporary owners.
import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleToFormulaAllocationFailureKeepsRuleReusable(t *testing.T) {
	failures, complete := 0, false
	for limit := int64(0); limit < 80; limit++ {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			f := newRuleAnalysisFixture(t)
			m := f.manager
			zero, err := m.ArithmeticIntegerNumeralText("0")
			ruleMust(t, err)
			constraint, err := m.MkArithmeticGE(f.x, zero)
			ruleMust(t, err)
			rule, err := f.rules.NewRule(f.head, []ast.ASTHandle{f.head, constraint}, []bool{true, false}, "formula-failure")
			ruleMust(t, err)
			defer func() { ruleMust(t, rule.Close()) }()
			before := m.Stats()
			m.Budget().FailAfter(limit)
			formula, cause := rule.ToFormula()
			m.Budget().FailAfter(-1)
			if cause != nil {
				if formula != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("conversion failure", formula, cause)
				}
				failures++
			} else {
				complete = true
				ruleMust(t, m.DelRef(&formula))
			}
			after := m.Stats()
			if after.LiveReferences != before.LiveReferences {
				t.Fatal("temporary formula references leaked", before, after)
			}
			formula, err = rule.ToFormula()
			ruleMust(t, err)
			handle, err := m.RefGet(formula)
			ruleMust(t, err)
			kind, err := m.GetKind(handle)
			ruleMust(t, err)
			if kind != ast.ASTQuantifier {
				t.Fatal("expected quantified rule", kind)
			}
			body, err := m.GetQuantifierBody(handle)
			ruleMust(t, err)
			op, err := m.GetDeclKind(body)
			ruleMust(t, err)
			if op != ast.ASTOpImplies {
				t.Fatal("expected implication", op)
			}
			antecedent, err := m.GetArg(body, 0)
			ruleMust(t, err)
			op, err = m.GetDeclKind(antecedent)
			ruleMust(t, err)
			if op != ast.ASTOpAnd {
				t.Fatal("expected conjunction", op)
			}
			neg, err := m.GetArg(antecedent, 0)
			ruleMust(t, err)
			op, err = m.GetDeclKind(neg)
			ruleMust(t, err)
			if op != ast.ASTOpNot {
				t.Fatal("expected negative predicate", op)
			}
			ruleMust(t, m.DelRef(&formula))
		})
		if complete {
			break
		}
	}
	if !complete || failures == 0 {
		t.Fatal("failure sweep did not reach both failure and success", failures, complete)
	}
	t.Logf("injected %d failures before successful conversion", failures)
}
