package base_test

// Source: src/muz/base/hnf.cpp contains_predicate uses quick_for_each_expr
// including annotations; mk_fresh_head uses body-only expr_free_vars.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornNormalizerUsesSourceAnnotationTraversalAndFreeVariableScope(t *testing.T) {
	for _, withProof := range []bool{false, true} {
		for _, noPattern := range []bool{false, true} {
			for _, scopeOnly := range []bool{false, true} {
				name := "annotation-predicate"
				if scopeOnly {
					name = "body-only-fresh-arguments"
				}
				if noPattern {
					name += "/no-pattern"
				} else {
					name += "/pattern"
				}
				if withProof {
					name += "/proof"
				}
				t.Run(name, func(t *testing.T) {
					f := newRuleSetFixture(t, "H")
					m := f.manager
					var pins []ast.ASTRef
					defer func() {
						for i := len(pins) - 1; i >= 0; i-- {
							ruleMust(t, m.DelRef(&pins[i]))
						}
					}()
					own := func(h ast.ASTHandle, err error) ast.ASTHandle {
						t.Helper()
						ruleMust(t, err)
						ref, err := m.NewRef(h)
						ruleMust(t, err)
						pins = append(pins, ref)
						return h
					}
					boolean := own(m.MkBoolSort())
					bound := own(m.MkVar(0, boolean))
					declaration := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
						t.Helper()
						s, err := m.Symbols.Intern(name)
						ruleMust(t, err)
						return own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
					}
					var annotation, body ast.ASTHandle
					if scopeOnly {
						property := declaration("hnf-body-property", []ast.ASTHandle{boolean})
						body = own(m.MkApp(property, []ast.ASTHandle{bound}))
						annotation = own(m.MkApp(declaration("hnf-annotation-pair", []ast.ASTHandle{boolean, boolean}), []ast.ASTHandle{bound, own(m.MkVar(3, boolean))}))
					} else {
						body = bound
						annotation = own(m.MkApp(declaration("hnf-annotation-only-predicate", []ast.ASTHandle{boolean}), []ast.ASTHandle{bound}))
					}
					spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}
					if noPattern {
						spec.NoPatterns = []ast.ASTHandle{annotation}
					} else {
						spec.Patterns = []ast.ASTHandle{own(m.MkPattern([]ast.ASTHandle{annotation}))}
					}
					quantified := own(m.MkQuantifier(spec))
					var antecedent ast.ASTHandle
					expectedFormulas := 3
					if scopeOnly {
						// The outer body's variable zero is free here. The nested quantifier's
						// annotation-only free variable must not add a second fresh argument.
						outerBody := own(m.MkAnd(body, quantified))
						antecedent = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(1)}, Body: outerBody}))
						expectedFormulas = 2
					} else {
						zero := own(m.ArithmeticIntegerNumeralText("0"))
						integer, err := m.GetSort(zero)
						ruleMust(t, err)
						term := own(m.MkVar(0, integer))
						condition := own(m.MkArithmeticGT(term, zero))
						antecedent = own(m.MkOr(quantified, condition))
					}
					formula := own(m.MkImplies(antecedent, f.app["H"]))
					normalizer, err := base.NewHornNormalizer(m, "hnf-annotations")
					ruleMust(t, err)
					var result *base.HornNormalization
					if withProof {
						ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
						p := own(m.MkProofAsserted(formula))
						ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
						result, err = normalizer.NormalizeWithProof(formula, p)
					} else {
						result, err = normalizer.Normalize(formula)
					}
					ruleMust(t, err)
					defer func() { ruleMust(t, result.Close()) }()
					if len(result.FreshPredicates) != 1 || len(result.Formulas) != expectedFormulas {
						t.Fatalf("fresh predicates/formulas = %d/%d, want 1/%d", len(result.FreshPredicates), len(result.Formulas), expectedFormulas)
					}
					fresh, err := m.RefGet(result.FreshPredicates[0])
					ruleMust(t, err)
					arity, err := m.GetArity(fresh)
					ruleMust(t, err)
					if arity != 1 {
						t.Fatal("annotation-only free variable entered fresh predicate signature", arity)
					}
					for i, ref := range result.Proofs {
						p, err := m.RefGet(ref)
						ruleMust(t, err)
						fact, err := m.GetProofFact(p)
						ruleMust(t, err)
						formula, err := m.RefGet(result.Formulas[i])
						ruleMust(t, err)
						if fact != formula {
							t.Fatal("annotation normalization proof fact", i)
						}
					}
				})
			}
		}
	}
}
