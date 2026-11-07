package base_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornQuantifiedClauseReferenceFailuresLeaveNoASTRoots(t *testing.T) {
	// These reference-page boundaries previously stranded quantified
	// clauses while building fresh Horn definitions.
	// Every allocation frontier must fail cleanly, up to the first budget that
	// succeeds; where that lies depends on capacity already grown in the fixture.
	for _, free := range []int{10, 12, 15} {
		completed := false
		for budget := int64(0); !completed; budget++ {
			if budget > 4096 {
				t.Fatal("normalization never completed", free)
			}
			t.Run(fmt.Sprintf("%d/%d", budget, free), func(t *testing.T) {
				f := newRuleSetFixture(t, "P", "Q", "R", "H")
				m := f.manager
				boolean, err := m.MkBoolSort()
				ruleMust(t, err)
				x, err := m.MkVar(0, boolean)
				ruleMust(t, err)
				sym, err := m.Symbols.Intern("quantified-hnf-p")
				ruleMust(t, err)
				decl, err := m.MkFuncDecl(sym, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				ruleMust(t, err)
				app, err := m.MkApp(decl, []ast.ASTHandle{x})
				ruleMust(t, err)
				disjunction, err := m.MkOr(app, f.app["Q"])
				ruleMust(t, err)
				body, err := m.MkAnd(disjunction, f.app["R"])
				ruleMust(t, err)
				core, err := m.MkImplies(body, f.app["H"])
				ruleMust(t, err)
				label, err := m.Symbols.Intern("quantified-x")
				ruleMust(t, err)
				formula, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{label}, Body: core})
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
				if cause == nil {
					if budget == 0 {
						t.Fatal("normalization allocated nothing, so no frontier was tested")
					}
					completed = true
					return
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
				for _, ref := range retried.Formulas {
					clause, e := m.RefGet(ref)
					ruleMust(t, e)
					kind, e := m.GetKind(clause)
					ruleMust(t, e)
					if kind != ast.ASTQuantifier {
						t.Fatal("retry lost universal binder", kind)
					}
					forall, e := m.IsForall(clause)
					ruleMust(t, e)
					if !forall {
						t.Fatal("retry produced existential clause")
					}
				}
				ruleMust(t, retried.Close())
				ruleMust(t, retry.Close())
			})
		}
	}
}
