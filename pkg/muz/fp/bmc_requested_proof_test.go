package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBMCRequestedNonlinearProofWithoutGlobalTracing(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			e, m := f.engine, f.manager
			fragmentMust(t, e.SelectEngine(backend))
			body, err := m.MkAnd(f.applications["p"], f.applications["q"])
			fragmentMust(t, err)
			rule, err := m.MkImplies(body, f.applications["u"])
			fragmentMust(t, err)
			fragmentMust(t, e.AddRule(rule, "nonlinear"))
			params, err := e.params.NewValue()
			fragmentMust(t, err)
			defer func() { fragmentMust(t, e.params.Del(&params)) }()
			for key, enabled := range map[string]bool{
				"print_certificate":         true,
				"xform.inline_eager":        false,
				"xform.inline_linear":       false,
				"xform.subsumption_checker": false,
			} {
				name := key
				fragmentMust(t, e.params.SetBoolName(&params, &name, enabled))
			}
			fragmentMust(t, e.SetQueryParameters(&params))
			status, err := e.QueryPredicate(f.declarations["u"])
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("nonlinear query", status)
			}
			if e.forceProofTrace || e.queryClone == nil || e.queryClone.forceProofTrace {
				t.Fatal("temporary proof setting escaped the query")
			}
			proof, err := m.RefGet(e.bmcProof)
			fragmentMust(t, err)
			fact, err := m.GetProofFact(proof)
			fragmentMust(t, err)
			want, err := m.MkApp(e.queryClonePredicate, nil)
			fragmentMust(t, err)
			if fact != want {
				t.Fatal("wrong proof conclusion")
			}
			checker, err := proofs.NewChecker(m)
			fragmentMust(t, err)
			valid, conditions, err := checker.Check(proof)
			fragmentMust(t, err)
			defer func() {
				for _, condition := range conditions {
					fragmentMust(t, m.DecRef(condition))
				}
			}()
			if !valid {
				t.Fatal("invalid requested proof")
			}
			solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
			fragmentMust(t, err)
			defer func() { fragmentMust(t, solver.Close()) }()
			for _, condition := range conditions {
				fragmentMust(t, solver.Push())
				negated, err := m.MkNot(condition)
				fragmentMust(t, err)
				fragmentMust(t, solver.AssertFormula(negated))
				status, err := solver.CheckSat(nil)
				fragmentMust(t, err)
				fragmentMust(t, solver.Pop(1))
				if status != lbool.False {
					t.Fatal("invalid proof side condition", status)
				}
			}
		})
	}
}
