package spacer

// Source: src/muz/spacer/spacer_context.cpp:2330-2384,2436-2568.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerSharedUnevenGroundWitnessAndRuleTrace(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c, m := f.context, f.context.manager
	set, err := base.NewRuleSet(f.manager)
	depthMust(t, err)
	defer func() { depthMust(t, set.Close()) }()
	var pins []ast.ASTRef
	defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		depthMust(t, err)
		r, err := m.NewRef(h)
		depthMust(t, err)
		pins = append(pins, r)
		return h
	}
	boolean := own(m.MkBoolSort())
	zero := own(m.ArithmeticIntegerNumeralText("0"))
	integer, err := m.GetSort(zero)
	depthMust(t, err)
	query := own(m.MkApp(f.query, nil))
	heads := []ast.ASTHandle{query}
	for _, name := range []string{"witness-root", "witness-left", "witness-right", "witness-mid", "witness-leaf"} {
		symbol, err := m.Symbols.Intern(name)
		depthMust(t, err)
		var domain []ast.ASTHandle
		if name == "witness-leaf" {
			domain = []ast.ASTHandle{integer}
		}
		decl := own(m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{}))
		depthMust(t, f.manager.RegisterPredicate(decl))
		var args []ast.ASTHandle
		if len(domain) != 0 {
			args = []ast.ASTHandle{zero}
		}
		heads = append(heads, own(m.MkApp(decl, args)))
	}
	// query -> root -> (left, right); left -> leaf(0), right -> mid -> leaf(0).
	tails := [][]int{{1}, {2, 3}, {5}, {4}, {5}, nil}
	names := []string{"witness-query-rule", "witness-root-rule", "witness-left-rule", "witness-right-rule", "witness-mid-rule", "witness-leaf-rule"}
	for i, indices := range tails {
		body := make([]ast.ASTHandle, len(indices))
		for j, index := range indices {
			body[j] = heads[index]
		}
		rule, err := f.manager.NewRule(heads[i], body, nil, names[i])
		depthMust(t, err)
		depthMust(t, set.Add(rule))
		depthMust(t, rule.Close())
	}
	depthMust(t, c.UpdateRules(set))
	status, err := c.Solve(0, 8)
	depthMust(t, err)
	if status != lbool.True {
		t.Fatal("shared derivation not reachable", status)
	}
	answer, err := c.GroundSatAnswer(f.query)
	depthMust(t, err)
	defer func() { depthMust(t, m.DelRef(&answer)) }()
	root, err := m.RefGet(answer)
	depthMust(t, err)
	witness := []ast.ASTHandle{heads[1], heads[2], heads[3], heads[5], heads[4], heads[5]}
	count, err := m.GetNumArgs(root)
	depthMust(t, err)
	if int(count) != len(witness) {
		t.Fatal("witness occurrence count", count, len(witness))
	}
	for i, want := range witness {
		got, err := m.GetArg(root, uint32(i))
		depthMust(t, err)
		if got != want {
			t.Fatal("source queue order/ground value", i, got, want)
		}
	}
	trace, err := c.RulesAlongTrace()
	depthMust(t, err)
	defer func() {
		for _, rule := range trace {
			if rule != nil {
				depthMust(t, rule.Close())
			}
		}
	}()
	order := []int{0, 1, 2, 3, 5, 4, 5}
	if len(trace) != len(order) {
		t.Fatal("trace occurrence count", len(trace), len(order))
	}
	for i, index := range order {
		name, err := trace[i].Name()
		depthMust(t, err)
		if name != names[index] {
			t.Fatal("trace source queue order", i, name, names[index])
		}
	}
	// Independently check each ground Horn derivation step, including both
	// occurrences of the shared leaf, against its actual exported rule.
	backend, err := smtcore.NewSMTSolver(m, nil, "QF_UFLIA", c.owner.contexts[0].config)
	depthMust(t, err)
	defer func() { depthMust(t, backend.Close()) }()
	for i, index := range order {
		depthMust(t, backend.Push())
		formula, err := trace[i].ToFormula()
		depthMust(t, err)
		handle, err := m.RefGet(formula)
		depthMust(t, err)
		depthMust(t, backend.Assert(handle))
		depthMust(t, m.DelRef(&formula))
		for _, child := range tails[index] {
			depthMust(t, backend.Assert(heads[child]))
		}
		depthMust(t, backend.Push())
		negated := own(m.MkNot(heads[index]))
		depthMust(t, backend.Assert(negated))
		status, err := backend.CheckSat(nil)
		depthMust(t, err)
		if status != lbool.False {
			t.Fatal("exported rule does not justify witness node", i, status)
		}
		depthMust(t, backend.Pop(1))
		depthMust(t, backend.Assert(heads[index]))
		status, err = backend.CheckSat(nil)
		depthMust(t, err)
		if status != lbool.True {
			t.Fatal("witness rule/body is inconsistent", i, status)
		}
		depthMust(t, backend.Pop(1))
	}
	depthMust(t, c.Close())
	// Releasing one repeated-rule owner cannot invalidate the other occurrence.
	depthMust(t, trace[4].Close())
	trace[4] = nil
	name, err := trace[6].Name()
	depthMust(t, err)
	if name != names[5] {
		t.Fatal("shared trace rule ownership was aliased")
	}
	actual, err := m.RefGet(answer)
	depthMust(t, err)
	if actual != root {
		t.Fatal("context disposal invalidated the ground witness")
	}
}
