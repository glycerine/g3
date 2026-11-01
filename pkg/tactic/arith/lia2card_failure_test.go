package arith

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLIA2CardConstructorAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 256 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			base, m := fixDLFixture(t, false)
			m.Budget().FailAfter(stop)
			port, err := NewLIA2CardTactic(m, base.context, base.globals, nil)
			m.Budget().FailAfter(-1)
			if port != nil {
				equationMust(t, port.Close())
			}
			if err == nil {
				success = true
				return
			}
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
		})
	}
	if !success {
		t.Fatal("constructor did not reach a successful allocation frontier")
	}
}

func TestLIA2CardApplyAllocationFrontiers(t *testing.T) {
	for _, tc := range []struct{ proofs, cores bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		success := false
		for stop := int64(0); stop < 256 && !success; stop++ {
			t.Run(fmt.Sprintf("proofs=%v/cores=%v/%d", tc.proofs, tc.cores, stop), func(t *testing.T) {
				base, m := fixDLFixture(t, tc.proofs)
				port, err := NewLIA2CardTactic(m, base.context, base.globals, nil)
				equationMust(t, err)
				t.Cleanup(func() { equationMust(t, port.Close()) })
				x := fixDLConst(t, m, true, "failure-card-x")
				y := fixDLConst(t, m, true, "failure-card-y")
				zero := fixDLNumber(t, m, true, "0")
				one := fixDLNumber(t, m, true, "1")
				forms := []ast.ASTHandle{fixDLApp(t, m, 3, x, zero), fixDLApp(t, m, 2, x, one), fixDLApp(t, m, 3, y, zero), fixDLApp(t, m, 2, y, one), collectorEq(t, m, fixDLApp(t, m, 6, x, y), one)}
				labels := make([]ast.ASTHandle, len(forms))
				if tc.cores {
					for i := range labels {
						labels[i] = cardinalityBool(t, m, fmt.Sprintf("failure-card-label-%d", i))
					}
				}
				newGoal := func() *tactic.Goal {
					goal, e := tactic.NewGoal(m, tc.proofs, false, tc.cores)
					equationMust(t, e)
					t.Cleanup(func() { equationMust(t, goal.Close()) })
					for i, form := range forms {
						var proof ast.ASTHandle
						if tc.proofs {
							proof, e = m.MkProofAsserted(form)
							equationMust(t, e)
						}
						var dep ast.ASTDependency
						if tc.cores {
							dep, e = m.DependencyLeaf(labels[i])
							equationMust(t, e)
						}
						equationMust(t, goal.AssertWithProof(form, proof, dep))
					}
					return goal
				}
				goal := newGoal()
				check := func(g *tactic.Goal, out *tactic.TacticResult) {
					if len(out.Goals) != 1 || out.Goals[0] != g {
						t.Fatal("result did not publish the goal")
					}
					depth, e := g.Depth()
					equationMust(t, e)
					count, e := g.Size()
					equationMust(t, e)
					converted, e := g.Form(4)
					equationMust(t, e)
					if depth != 1 || count < 7 || converted == forms[4] {
						t.Fatal("retry did not perform the conversion", depth, count)
					}
				}
				var result tactic.TacticResult
				t.Cleanup(func() { equationMust(t, result.Close()) })
				m.Budget().FailAfter(stop)
				err = port.Apply(goal, &result)
				m.Budget().FailAfter(-1)
				if err == nil {
					check(goal, &result)
					success = true
					return
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == memory.FailureMemout && m.NumeralService().RuntimeOwner().State().OutOfMemory) {
					t.Fatal(err)
				}
				retry := newGoal()
				var retried tactic.TacticResult
				t.Cleanup(func() { equationMust(t, retried.Close()) })
				equationMust(t, port.Apply(retry, &retried))
				check(retry, &retried)
			})
		}
		if !success {
			t.Fatalf("no successful allocation frontier: %+v", tc)
		}
	}
}
