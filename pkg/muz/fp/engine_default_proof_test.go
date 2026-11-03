package fp

// Source: src/muz/base/dl_engine_base.h engine_base::get_proof constructs
// asserted(true) directly into the caller's owning proof_ref.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestEngineDefaultProofReferenceAllocationFailureAndRetry(t *testing.T) {
	for _, backend := range []string{"datalog", "bmc", "clp"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			e, m := f.engine, f.manager
			fragmentMust(t, e.SelectEngine(backend))
			status, err := e.QueryPredicate(f.declarations["p"])
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("missing successful query snapshot", status)
			}
			fragmentMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			// Warm only the asserted-kind declaration using a different fact.
			// The production asserted(true) application must still be fresh.
			falsehood, err := m.MkFalse()
			fragmentMust(t, err)
			warm, err := m.MkProofAsserted(falsehood)
			fragmentMust(t, err)
			warmRef, err := m.NewRef(warm)
			fragmentMust(t, err)
			fragmentMust(t, m.DelRef(&warmRef))
			var pressure []ast.ASTRef
			defer func() {
				m.Budget().FailAfter(-1)
				fragmentMust(t, closeASTRefs(m, pressure))
			}()
			for {
				m.Budget().FailAfter(0)
				ref, cause := m.NewRef(ast.ASTHandle{})
				m.Budget().FailAfter(-1)
				if cause != nil {
					if ref != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
						t.Fatal("reference allocation frontier", ref, cause)
					}
					break
				}
				pressure = append(pressure, ref)
				if len(pressure) > 4096 {
					t.Fatal("did not reach a physical reference allocation")
				}
			}
			before := m.Stats()
			checkOwners := func(label string) {
				t.Helper()
				after := m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
					t.Fatal(label, "changed live AST ownership", before, after)
				}
			}
			m.Budget().FailAfter(0)
			result, cause := e.Proof(f.declarations["p"])
			m.Budget().FailAfter(-1)
			if result != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
				t.Fatal("proof did not refuse the result-reference allocation", result, cause)
			}
			checkOwners("failed publication")
			for i := 0; i < 2; i++ {
				result, err = e.Proof(f.declarations["p"])
				fragmentMust(t, err)
				proof, err := m.RefGet(result)
				fragmentMust(t, err)
				asserted, err := m.IsProofKind(proof, ast.ASTPrAsserted)
				fragmentMust(t, err)
				fact, err := m.GetProofFact(proof)
				fragmentMust(t, err)
				if !asserted || !m.IsTrue(fact) {
					t.Fatal("default proof is not asserted(true)")
				}
				fragmentMust(t, m.DelRef(&result))
				checkOwners("completed retry")
			}
		})
	}
}
