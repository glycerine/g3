package smtlogics_test

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestQFBVTacticTranslatedProofAndCoreRoutes(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			option := "produce-unsat-cores"
			if proofs {
				option = "produce-proofs"
			}
			c := qfliaContext(t, "(set-logic QF_BV)(set-option :"+option+" true)(declare-const x (_ BitVec 4))(assert (! (= (bvmul x x) #x2) :named impossible))")
			m, err := c.Manager()
			qfaufliaMust(t, err)
			r, err := c.NewBuiltinTactic("qfbv")
			qfaufliaMust(t, err)
			qfaufliaMust(t, r.IncRef())
			mode := ast.ASTProofDisabled
			if proofs {
				mode = ast.ASTProofEnabled
			}
			target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget(), ProofMode: mode})
			qfaufliaMust(t, err)
			// The source copy-constructs the target (ast_manager(m, ...)), which
			// keeps the source family ids.
			qfaufliaMust(t, target.CopyFamiliesPlugins(m))
			t.Cleanup(func() { qfaufliaMust(t, target.Close()) })
			translated, err := r.Translate(target)
			qfaufliaMust(t, err)
			qfaufliaMust(t, translated.IncRef())
			qfaufliaMust(t, r.DecRef())
			t.Cleanup(func() { qfaufliaMust(t, translated.DecRef()) })
			g, err := tactic.NewGoal(m, proofs, false, !proofs)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, g.Close()) })
			qfaufliaMust(t, c.AssertGoal(g))
			translation, err := ast.NewASTTranslation(m, target, true)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, translation.Close()) })
			copy, err := g.Translate(translation)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, copy.Close()) })
			var model tactic.ModelRef
			t.Cleanup(func() { qfaufliaMust(t, model.Close()) })
			var labels tactic.ModelLabels
			t.Cleanup(labels.Close)
			pr, err := target.NewRef(ast.ASTHandle{})
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, target.DelRef(&pr)) })
			var core tactic.DependencyRef
			t.Cleanup(func() { qfaufliaMust(t, core.Close()) })
			status, reason, err := tactic.CheckSat(translated, copy, &model, &labels, pr, &core)
			qfaufliaMust(t, err)
			if status != lbool.False {
				t.Fatal(status, reason)
			}
			qfaufliaMust(t, translated.Cleanup())
			if proofs {
				proof, err := target.RefGet(pr)
				qfaufliaMust(t, err)
				fact, err := target.GetProofFact(proof)
				qfaufliaMust(t, err)
				if !target.IsFalse(fact) {
					t.Fatal("SMT proof route did not preserve a contradiction proof")
				}
			} else {
				dependency := core.Get()
				entries, err := target.DependencyLinearize(dependency, nil)
				qfaufliaMust(t, err)
				if len(entries) != 1 {
					t.Fatal("core route lost original labels", entries)
				}
			}
		})
	}
}
