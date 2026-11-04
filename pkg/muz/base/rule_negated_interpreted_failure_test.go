package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// dl_rule.cpp mk keeps the synthesized NOT in its owning rule as it builds
// the interpreted tail. Go must also clean it up if its reference slot fails.
func TestRuleNegatedInterpretedTailReferenceFailure(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	m := f.manager
	headRef, err := m.NewRef(f.head)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&headRef)) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	constraint, err := m.MkArithmeticGE(f.x, zero)
	ruleMust(t, err)
	constraintRef, err := m.NewRef(constraint)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&constraintRef)) }()
	warm, err := m.MkNot(constraint)
	ruleMust(t, err)
	warmRef, err := m.NewRef(warm)
	ruleMust(t, err)
	ruleMust(t, m.DelRef(&warmRef))
	var pressure []ast.ASTRef
	defer func() {
		for i := range pressure {
			ruleMust(t, m.DelRef(&pressure[i]))
		}
	}()
	for {
		m.Budget().FailAfter(0)
		pin, cause := m.NewRef(ast.ASTHandle{})
		m.Budget().FailAfter(-1)
		if cause != nil {
			if !errors.Is(cause, arena.ErrInjectedFailure) {
				t.Fatal(cause)
			}
			break
		}
		pressure = append(pressure, pin)
		if len(pressure) > 4096 {
			t.Fatal("reference page did not fill")
		}
	}
	last := len(pressure) - 1
	ruleMust(t, m.DelRef(&pressure[last]))
	pressure = pressure[:last]
	_, err = m.Symbols.Intern("negated-failure")
	ruleMust(t, err)
	before := m.Stats()
	m.Budget().FailAfter(0)
	rule, cause := f.rules.NewRule(f.head, []ast.ASTHandle{constraint}, []bool{true}, "negated-failure")
	m.Budget().FailAfter(-1)
	if rule != nil || !errors.Is(cause, arena.ErrInjectedFailure) {
		t.Fatal("expected allocation failure", rule, cause)
	}
	after := m.Stats()
	if after.LiveReferences != before.LiveReferences || after.LiveNodes != before.LiveNodes {
		t.Fatal("failed rule leaked temporary negation", before, after)
	}
	rule, err = f.rules.NewRule(f.head, []ast.ASTHandle{constraint}, []bool{true}, "negated-failure")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	tail, negative, err := rule.Tail(0)
	ruleMust(t, err)
	operator, err := m.GetDeclKind(tail)
	ruleMust(t, err)
	argument, err := m.GetArg(tail, 0)
	ruleMust(t, err)
	if negative || operator != ast.ASTOpNot || argument != constraint {
		t.Fatal("retry did not preserve interpreted negation", tail, negative, operator, argument)
	}
}
