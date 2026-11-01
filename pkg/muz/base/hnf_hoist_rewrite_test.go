package base_test

// Source: src/muz/base/hnf.cpp mk_horn invokes pull_quantifier with its
// default rewrite_ok=true; quant_hoist.cpp reduces rebuilt Boolean gates.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornNormalizerHoistingRewritesBooleanGates(t *testing.T) {
	for _, withProof := range []bool{false, true} {
		for _, prefix := range []bool{false, true} {
			for _, absorbing := range []bool{false, true} {
				name := "duplicate-and"
				if absorbing {
					name = "absorbing-or"
				}
				if prefix {
					name += "/prefix"
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
					variable := own(m.MkVar(0, boolean))
					nameSymbol, err := m.Symbols.Intern("hoist-rewrite-predicate")
					ruleMust(t, err)
					predicate := own(m.MkFuncDecl(nameSymbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
					body := own(m.MkApp(predicate, []ast.ASTHandle{variable}))
					quantified := own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
					antecedent := own(m.MkAnd(quantified, quantified))
					expected := own(m.MkImplies(quantified, f.app["H"]))
					if absorbing {
						antecedent = own(m.MkOr(quantified, own(m.MkTrue())))
						expected = f.app["H"]
					}
					formula := own(m.MkImplies(antecedent, f.app["H"]))
					wrap := func(body ast.ASTHandle) ast.ASTHandle {
						return own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(1)}, Body: body}))
					}
					if prefix {
						formula = wrap(formula)
						expected = wrap(expected)
					}
					normalizer, err := base.NewHornNormalizer(m, "hoist-rewrite")
					ruleMust(t, err)
					t.Cleanup(func() { ruleMust(t, normalizer.Close()) })
					var result *base.HornNormalization
					if withProof {
						ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
						proof := own(m.MkProofAsserted(formula))
						ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
						result, err = normalizer.NormalizeWithProof(formula, proof)
					} else {
						result, err = normalizer.Normalize(formula)
					}
					if withProof && !prefix && ast.ASTSourceDebug {
						// Source hnf::mk_horn only repairs the premise after
						// hoisting when it collected a prefix; mk_modus_ponens
						// then asserts on this rewritten, unmatched premise.
						if result != nil || !errors.Is(err, ast.ErrASTDomain) {
							t.Fatalf("source debug precondition = %v, %v", result, err)
						}
						mode, modeErr := m.ProofMode()
						ruleMust(t, modeErr)
						if mode != ast.ASTProofDisabled {
							t.Fatal("failed proof normalization changed proof mode")
						}
						return
					}
					ruleMust(t, err)
					defer func() { ruleMust(t, result.Close()) }()
					if len(result.Formulas) != 1 || len(result.FreshPredicates) != 0 {
						t.Fatalf("hoisting result = %d formulas/%d predicates, want 1/0", len(result.Formulas), len(result.FreshPredicates))
					}
					actual, err := m.RefGet(result.Formulas[0])
					ruleMust(t, err)
					if actual != expected {
						t.Fatal("Boolean gate was not reduced during source quantifier hoisting", actual, expected)
					}
					if withProof {
						proof, err := m.RefGet(result.Proofs[0])
						ruleMust(t, err)
						fact, err := m.GetProofFact(proof)
						ruleMust(t, err)
						if fact != actual {
							t.Fatal("hoisted proof does not conclude output formula")
						}
						checker, err := proofs.NewChecker(m)
						ruleMust(t, err)
						valid, side, err := checker.Check(proof)
						ruleMust(t, err)
						// Source attaches pull-quant only when it collected a prefix. With
						// no prefix, gate rewriting changes the premise without that proof.
						if valid != prefix {
							t.Fatal("source hoisting/checker behavior differs", valid, prefix)
						}
						ruleMust(t, checker.ReleaseSideConditions(side))
					}
				})
			}
		}
	}
}
