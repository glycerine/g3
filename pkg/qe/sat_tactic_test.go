package qe

import (
	"github.com/glycerine/g3/pkg/util/statistics"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQESATTacticPartialGoalsAndModels(t *testing.T) {
	for _, satisfiable := range []bool{false, true} {
		t.Run(map[bool]string{false: "unsat", true: "sat"}[satisfiable], func(t *testing.T) {
			f := qsatKernelFixture(t)
			m := f.r.manager
			v := f.own(m.MkVar(0, f.boolean))
			body := v
			if satisfiable {
				body = f.own(m.MkOr(v, f.own(m.MkNot(v))))
			}
			quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
			p := f.constant("qe-sat-free", f.boolean)
			goal := f.goal(false, false)
			fmMust(t, goal.Assert(quantifier))
			if satisfiable {
				fmMust(t, goal.Assert(p))
			}
			port, err := NewQESATTactic(m, nil, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, port.Close()) }()
			out := f.result()
			fmMust(t, port.Apply(goal, out))
			if len(out.Goals) != 1 || out.Goals[0] != goal {
				t.Fatal("goal not returned")
			}
			size, err := goal.Size()
			fmMust(t, err)
			if satisfiable {
				if size != 0 || out.ModelConverter == nil {
					t.Fatal("satisfiable goal/model")
				}
				var ref tactic.ModelRef
				defer func() { fmMust(t, ref.Close()) }()
				fmMust(t, out.ModelConverter.ApplyUnindexed(&ref))
				md, err := ref.Get()
				fmMust(t, err)
				qsatKernelEvaluate(t, f, md, p)
			} else {
				if size != 1 {
					t.Fatalf("unsatisfiable goal size %d", size)
				}
				h, err := goal.Form(0)
				fmMust(t, err)
				if !m.IsFalse(h) {
					t.Fatal("unsat goal did not contain false")
				}
			}
		})
	}
}

func TestQESATTacticFullProjectionUnsupportedGoalIsUnchanged(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	sort := f.own(m.MkUninterpretedSort(symbol.Numerical(99022), nil))
	v := f.own(m.MkVar(0, sort))
	other := f.constant("full-unsupported-constant", sort)
	body := f.own(m.MkEq(v, other))
	quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(quantifier))
	port, err := NewQESATTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	out := f.result()
	if err = port.Apply(goal, out); err == nil {
		t.Fatal("unsupported full projection succeeded")
	}
	size, err := goal.Size()
	fmMust(t, err)
	if size != 1 {
		t.Fatalf("goal size changed to %d", size)
	}
	got, err := goal.Form(0)
	fmMust(t, err)
	if got != quantifier {
		t.Fatal("unsupported projection changed goal")
	}
}

func TestQESATTacticStatisticsAcrossApplyAndReset(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	port, err := NewQESATTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	makeGoal := func() *tactic.Goal {
		variable := f.own(m.MkVar(0, f.boolean))
		quantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: variable,
		}))
		goal := f.goal(false, false)
		fmMust(t, goal.Assert(quantifier))
		return goal
	}
	apply := func() {
		goal := makeGoal()
		defer func() { fmMust(t, goal.Close()) }()
		out := f.result()
		defer func() { fmMust(t, out.Close()) }()
		fmMust(t, port.Apply(goal, out))
	}
	apply()
	if got := statistics.Sums(port.CollectStatistics); len(got) == 0 {
		t.Fatal("SAT tactic did not report solver statistics")
	}
	before := statistics.Sums(port.CollectStatistics)
	fmMust(t, port.ResetStatistics())
	// The source kernels' reset_statistics is empty.
	if got := statistics.Sums(port.CollectStatistics); len(got) != len(before) {
		t.Fatalf("statistics after reset: %v, before %v", got, before)
	}
	apply()
	if got := statistics.Sums(port.CollectStatistics); len(got) == 0 {
		t.Fatal("statistics were not collected after reset and another application")
	}
}

func TestQESATTacticCancellationBeforePreprocessingAndRetry(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	b := f.own(m.MkVar(0, f.boolean))
	formula := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: b}))
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(formula))
	port, err := NewQESATTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	out := f.result()
	m.ResourceLimit().Cancel()
	err = port.Apply(goal, out)
	m.ResourceLimit().ResetCancel()
	if err == nil {
		t.Fatal("canceled tactic succeeded")
	}
	before, err := goal.Form(0)
	fmMust(t, err)
	if before != formula {
		t.Fatal("cancellation changed the goal")
	}
	fmMust(t, port.Apply(goal, out))
	got, err := goal.Form(0)
	fmMust(t, err)
	if !m.IsFalse(got) {
		t.Fatal("retry did not prove the Boolean universal false")
	}
}
