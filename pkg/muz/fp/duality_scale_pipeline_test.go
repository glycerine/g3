package fp

// Source: duality_dl_interface.cpp's quantify_arrays query branch and
// dl_mk_scale.cpp's default transformation.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityScaleRealQueries(t *testing.T) {
	for _, arrays := range []bool{false, true} {
		for _, scale := range []bool{false, true} {
			name := "raw"
			if arrays {
				name = "transformed"
			}
			if scale {
				name += "-scale"
			}
			t.Run(name, func(t *testing.T) {
				m, globals := newPDRLifecycleServices(t)
				for _, key := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
					fragmentMust(t, globals.SourceSetName("fixedpoint.xform."+key, "false"))
				}
				if arrays {
					fragmentMust(t, globals.SourceSetName("fixedpoint.xform.quantify_arrays", "true"))
				}
				if scale {
					fragmentMust(t, globals.SourceSetName("fixedpoint.xform.scale", "true"))
				}
				e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
				fragmentMust(t, err)
				defer func() { fragmentMust(t, e.Close()) }()
				fragmentMust(t, e.SelectEngine("duality"))
				var roots []ast.ASTRef
				defer func() {
					for i := len(roots) - 1; i >= 0; i-- {
						fragmentMust(t, m.DelRef(&roots[i]))
					}
				}()
				own := func(h ast.ASTHandle, err error) ast.ASTHandle {
					fragmentMust(t, err)
					ref, err := m.NewRef(h)
					fragmentMust(t, err)
					roots = append(roots, ref)
					return h
				}
				one := own(m.ArithmeticRealNumeralText("1"))
				two := own(m.ArithmeticRealNumeralText("2"))
				realSort, err := m.GetSort(one)
				fragmentMust(t, err)
				boolean := own(m.MkBoolSort())
				name, err := m.Symbols.Intern("duality-scale-real")
				fragmentMust(t, err)
				predicate := own(m.MkFuncDecl(name, []ast.ASTHandle{realSort}, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(predicate))
				fact := own(m.MkApp(predicate, []ast.ASTHandle{one}))
				absent := own(m.MkApp(predicate, []ast.ASTHandle{two}))
				fragmentMust(t, e.AddRule(fact, "scale-fact"))
				var firstProof ast.ASTRef
				var safeModel *model.Model
				defer func() {
					if firstProof != (ast.ASTRef{}) {
						fragmentMust(t, m.DelRef(&firstProof))
					}
					if safeModel != nil {
						fragmentMust(t, safeModel.Close())
					}
				}()
				for index, query := range []struct {
					term ast.ASTHandle
					want lbool.LBool
				}{{fact, lbool.True}, {absent, lbool.False}, {fact, lbool.True}} {
					status, err := e.QueryExpression(query.term)
					fragmentMust(t, err)
					if status != query.want {
						t.Fatalf("query status = %v, want %v", status, query.want)
					}
					proof, err := e.Proof(predicate)
					fragmentMust(t, err)
					proofTerm, err := m.RefGet(proof)
					fragmentMust(t, err)
					if query.want == lbool.True {
						if proofTerm.IsNull() {
							t.Fatal("reachable query returned no proof")
						}
						conclusion, err := m.GetProofFact(proofTerm)
						fragmentMust(t, err)
						if !m.IsFalse(conclusion) {
							t.Fatal("query proof does not conclude false")
						}
						if index == 0 {
							firstProof = proof
						} else {
							fragmentMust(t, m.DelRef(&proof))
						}
					} else {
						if !proofTerm.IsNull() {
							t.Fatal("safe query returned a refutation proof")
						}
						fragmentMust(t, m.DelRef(&proof))
						safeModel, err = e.Model(predicate)
						fragmentMust(t, err)
					}
				}
				// The next query replaces the backend result, but previously
				// returned proof and model owners remain valid.
				_, err = m.RefGet(firstProof)
				fragmentMust(t, err)
				_, err = safeModel.Core().HasInterpretation(predicate)
				fragmentMust(t, err)
				if arrays {
					// Check the same default transform pipeline used by the Duality
					// query branch; status alone cannot show that scaling ran.
					prepared, err := e.preprocessHornRulesWithQueryRule(predicate, nil, true, true, false, true)
					fragmentMust(t, err)
					derived := prepared.scaledPredicates[predicate]
					if scale {
						if derived.IsNull() {
							t.Fatal("scale did not produce a predicate")
						}
						arity, err := m.GetArity(derived)
						fragmentMust(t, err)
						if arity != 2 {
							t.Fatalf("scaled predicate arity = %d, want 2", arity)
						}
					} else if !derived.IsNull() || len(prepared.scaledPredicates) != 0 {
						t.Fatal("unexpected scale transform")
					}
					fragmentMust(t, prepared.Close())
				}
			})
		}
	}
}
