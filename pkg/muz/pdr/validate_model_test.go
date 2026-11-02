package pdr

// Source: pdr_context.cpp context::validate_model. These tests deliberately
// distinguish inductivity from a stronger, separate query-falsity obligation.
import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestValidateInductiveModelRuleViolationsAndFiniteFrame(t *testing.T) {
	m, owner, spare, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, spare.Close()); closureMust(t, owner.Close()); closeBase() }()
	c, err := NewContext(m, nil, 2, ReachableConstraintCache, owner.contexts.config)
	closureMust(t, err)
	defer func() { closureMust(t, c.Close()) }()
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	name, err := m.Symbols.Intern("validation_P")
	closureMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	name, err = m.Symbols.Intern("validation_Goal")
	closureMust(t, err)
	q, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	rm, err := base.NewRuleManager(m)
	closureMust(t, err)
	defer func() { closureMust(t, rm.Close()) }()
	closureMust(t, rm.RegisterPredicate(p))
	closureMust(t, rm.RegisterPredicate(q))
	rs, err := base.NewRuleSet(rm)
	closureMust(t, err)
	defer func() { closureMust(t, rs.Close()) }()
	truth, err := m.MkTrue()
	closureMust(t, err)
	falsity, err := m.MkFalse()
	closureMust(t, err)
	// Index two exercises source Boolean defaults for unused bound-variable slots.
	x, err := m.MkVar(2, boolean)
	closureMust(t, err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	closureMust(t, err)
	pt, err := m.MkApp(p, []ast.ASTHandle{truth})
	closureMust(t, err)
	goal, err := m.MkApp(q, nil)
	closureMust(t, err)
	notx, err := m.MkNot(x)
	closureMust(t, err)
	add := func(h ast.ASTHandle, tail []ast.ASTHandle, name string) {
		r, e := rm.NewRule(h, tail, nil, name)
		closureMust(t, e)
		closureMust(t, rs.Add(r))
		closureMust(t, r.Close())
	}
	add(pt, nil, "validation-fact")
	add(goal, []ast.ASTHandle{px, notx}, "validation-goal")
	closureMust(t, c.Initialize(rs))
	variable, err := m.MkVar(0, boolean)
	closureMust(t, err)
	for _, tc := range []struct {
		name                 string
		body                 ast.ASTHandle
		queryTrue, wantError bool
	}{
		{"inductive", variable, false, false},
		{"missing-fact", falsity, false, true},
		{"reachable-query", truth, false, true},
		{"source-does-not-separately-refute-query", truth, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			md, e := model.NewModel(m)
			closureMust(t, e)
			defer func() { closureMust(t, md.Close()) }()
			fi, e := model.NewFunctionInterpretation(m, 1)
			closureMust(t, e)
			closureMust(t, fi.SetElse(tc.body))
			closureMust(t, md.Core().RegisterFunction(p, fi))
			if tc.queryTrue {
				closureMust(t, md.Core().RegisterConstant(q, truth))
			}
			e = c.validateInductiveModel(md)
			hasQuery, inspectErr := md.Core().HasInterpretation(q)
			closureMust(t, inspectErr)
			if hasQuery != tc.queryTrue {
				t.Fatal("validation completed or changed the borrowed model")
			}
			if tc.wantError {
				if e == nil || !strings.Contains(e.Error(), "rule validation failed when checking:") {
					t.Fatalf("missing semantic rejection: %v", e)
				}
			} else {
				closureMust(t, e)
			}
		})
	}
	c.queryDecl = q
	c.inductive = 2
	closureMust(t, c.AddCover(2, p, variable))
	closureMust(t, c.validateUNSATResult())
	c.inductive = 3
	if err := c.validateUNSATResult(); err == nil || !strings.Contains(err.Error(), "rule validation failed when checking:") {
		t.Fatalf("validation ignored finite frame boundary: %v", err)
	}
	falsity, err = m.MkFalse()
	closureMust(t, err)
	closureMust(t, c.owner.SetBackground(falsity))
	if err = c.validateUNSATResult(); err == nil || !strings.Contains(err.Error(), "rule validation failed when checking:") {
		t.Fatalf("false background hid corrupt inductive model: %v", err)
	}
}
