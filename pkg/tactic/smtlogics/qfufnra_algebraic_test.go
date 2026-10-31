package smtlogics_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestQFUFNRATacticWithAlgebraicRoot(t *testing.T) {
	c := qfliaContext(t, "(set-logic ALL)(declare-const x Real)")
	m, err := c.Manager()
	qfaufliaMust(t, err)
	x, err := c.MakeApp("x", nil, nil, ast.ASTHandle{})
	qfaufliaMust(t, err)
	am, err := m.ArithmeticAlgebraicManager()
	qfaufliaMust(t, err)
	two := numeral.NewInt(2)
	var number, root polynomial.Algebraic
	qfaufliaMust(t, am.SetInteger(&number, &two))
	qfaufliaMust(t, am.Root(&number, 2, &root))
	qfaufliaMust(t, am.Del(&number))
	rootTerm, err := m.ArithmeticAlgebraicNumeral(&root)
	qfaufliaMust(t, err)
	qfaufliaMust(t, am.Del(&root))
	rootRef, err := m.NewRef(rootTerm)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, m.DelRef(&rootRef)) })
	one, err := m.ArithmeticRealNumeralText("1")
	qfaufliaMust(t, err)
	equality, err := m.MkEq(x, rootTerm)
	qfaufliaMust(t, err)
	bound, err := m.MkArithmeticLE(x, one)
	qfaufliaMust(t, err)
	goal, err := tactic.NewGoal(m, false, false, false)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
	qfaufliaMust(t, goal.Assert(equality))
	qfaufliaMust(t, goal.Assert(bound))
	strategy, err := c.NewBuiltinTactic("qfufnra")
	qfaufliaMust(t, err)
	qfaufliaMust(t, strategy.IncRef())
	t.Cleanup(func() { qfaufliaMust(t, strategy.DecRef()) })
	var md tactic.ModelRef
	t.Cleanup(func() { qfaufliaMust(t, md.Close()) })
	var labels tactic.ModelLabels
	t.Cleanup(labels.Close)
	var core tactic.DependencyRef
	t.Cleanup(func() { qfaufliaMust(t, core.Close()) })
	proof, err := m.NewRef(ast.ASTHandle{})
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, m.DelRef(&proof)) })
	status, reason, err := tactic.CheckSat(strategy, goal, &md, &labels, proof, &core)
	qfaufliaMust(t, err)
	if status != lbool.False {
		t.Fatalf("sqrt(2) <= 1: status %v, reason %s", status, reason)
	}
}
