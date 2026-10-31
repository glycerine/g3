package qe

import (
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
			if err = port.Apply(goal, out); err == nil {
				t.Fatal("unimplemented default mode accepted")
			}
			params, err := f.r.config.Context.NewValue()
			fmMust(t, err)
			defer func() { fmMust(t, f.r.config.Context.Del(&params)) }()
			key := "projection_mode"
			fmMust(t, f.r.config.Context.SetBoolName(&params, &key, false))
			fmMust(t, port.UpdateParams(&params))
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
