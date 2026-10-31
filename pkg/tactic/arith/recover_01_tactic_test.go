package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestRecover01TacticParametersAndReuse(t *testing.T) {
	base, m := fixDLFixture(t, false)
	sort, err := m.MkBoolSort()
	equationMust(t, err)
	symbol, err := m.Symbols.Intern("params-p")
	equationMust(t, err)
	p, err := m.MkConst(symbol, sort)
	equationMust(t, err)
	fixDLPin(t, m, p)
	notP, err := m.MkNot(p)
	equationMust(t, err)
	fixDLPin(t, m, notP)
	x := fixDLConst(t, m, true, "params-x")
	clause := func(lit ast.ASTHandle, n string) ast.ASTHandle {
		eq := collectorEq(t, m, x, fixDLNumber(t, m, true, n))
		h, e := m.MkOrArgs([]ast.ASTHandle{lit, eq})
		equationMust(t, e)
		fixDLPin(t, m, h)
		return h
	}
	goal := fixDLGoal(t, m, false, false, false, clause(p, "0"), clause(notP, "1"))
	params, err := base.context.NewValue()
	equationMust(t, err)
	defer func() { equationMust(t, base.context.Del(&params)) }()
	key := "recover_01_max_bits"
	equationMust(t, base.context.SetUIntName(&params, &key, 2))
	port, err := NewRecover01Tactic(m, base.context, base.globals, &params, base.services)
	equationMust(t, err)
	defer func() { equationMust(t, port.Close()) }()
	var out tactic.TacticResult
	defer func() { equationMust(t, out.Close()) }()
	equationMust(t, port.Apply(goal, &out))
	if len(out.Goals) != 1 || out.Goals[0] != goal {
		t.Fatal("strict size limit not applied")
	}
	equationMust(t, out.Reset())
	equationMust(t, base.context.SetUIntName(&params, &key, 3))
	equationMust(t, port.UpdateParams(&params))
	equationMust(t, port.Cleanup())
	equationMust(t, port.Apply(goal, &out))
	if len(out.Goals) != 1 || out.Goals[0] == goal {
		t.Fatal("updated size limit did not recover the variable")
	}
}
