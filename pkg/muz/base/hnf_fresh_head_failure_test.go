package base_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestHornFreshHeadReferenceFailuresLeaveNoASTRoots(t *testing.T) {
	// These reference-page boundaries previously stranded fresh declarations,
	// applications and Horn implications during definition creation.
	for _, budget := range []int64{0, 1} {
		for _, free := range []int{4, 6, 9, 10, 11, 12} {
			t.Run(fmt.Sprintf("%d/%d", budget, free), func(t *testing.T) {
				f := newRuleSetFixture(t, "P", "Q", "R", "H")
				m := f.manager
				disjunction, err := m.MkOr(f.app["P"], f.app["Q"])
				ruleMust(t, err)
				body, err := m.MkAnd(disjunction, f.app["R"])
				ruleMust(t, err)
				formula, err := m.MkImplies(body, f.app["H"])
				ruleMust(t, err)
				root, err := m.NewRef(formula)
				ruleMust(t, err)
				defer func() { ruleMust(t, m.DelRef(&root)) }()
				normalizer, err := base.NewHornNormalizer(m, "fresh-owner")
				ruleMust(t, err)
				defer func() { ruleMust(t, normalizer.Close()) }()
				var pressure []ast.ASTRef
				defer func() {
					for i := range pressure {
						ruleMust(t, m.DelRef(&pressure[i]))
					}
				}()
				for i := 0; i < 64; i++ {
					pin, err := m.NewRef(ast.ASTHandle{})
					ruleMust(t, err)
					pressure = append(pressure, pin)
				}
				for {
					m.Budget().FailAfter(0)
					pin, cause := m.NewRef(ast.ASTHandle{})
					m.Budget().FailAfter(-1)
					if cause != nil {
						if !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Fatal(cause)
						}
						break
					}
					pressure = append(pressure, pin)
					if len(pressure) > 4096 {
						t.Fatal("reference page did not fill")
					}
				}
				for i := 0; i < free; i++ {
					last := len(pressure) - 1
					ruleMust(t, m.DelRef(&pressure[last]))
					pressure = pressure[:last]
				}
				before := m.Stats()
				m.Budget().FailAfter(budget)
				result, cause := normalizer.Normalize(formula)
				m.Budget().FailAfter(-1)
				if result != nil {
					ruleMust(t, result.Close())
				}
				if !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("expected allocation failure", cause)
				}
				ruleMust(t, normalizer.Close())
				after := m.Stats()
				if after.LiveNodes > before.LiveNodes || after.LiveReferences > before.LiveReferences {
					t.Fatal("fresh HNF roots leaked", before, after, cause)
				}
				retry, err := base.NewHornNormalizer(m, "fresh-owner")
				ruleMust(t, err)
				retried, err := retry.Normalize(formula)
				ruleMust(t, err)
				if len(retried.FreshPredicates) != 1 || len(retried.Formulas) != 3 {
					t.Fatal("retry changed fresh definition", len(retried.FreshPredicates), len(retried.Formulas))
				}
				ruleMust(t, retried.Close())
				ruleMust(t, retry.Close())
			})
		}
	}
}
