package fp

// Source: dl_context.cpp get_rules_as_formulas/display_smt2 promotes open raw
// rules without replacing Duality's completed engine proof or annotation model.

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type dualityDisplayResultFixture struct {
	*fpRuleNameFixture
	predicate, application, unreachable, unreachableApplication, openRule ast.ASTHandle
}

func newDualityDisplayResultFixture(t *testing.T) *dualityDisplayResultFixture {
	t.Helper()
	f := &dualityDisplayResultFixture{fpRuleNameFixture: newFPRuleNameFixture(t)}
	fragmentMust(t, f.e.SelectEngine("duality"))
	f.predicate, f.application = f.fpRuleNameFixture.predicate("duality-display-P")
	f.unreachable, f.unreachableApplication = f.fpRuleNameFixture.predicate("duality-display-Q")
	zero := f.own(f.m.ArithmeticIntegerNumeralText("0"))
	integer, err := f.m.GetSort(zero)
	fragmentMust(t, err)
	x := f.own(f.m.MkVar(0, integer))
	f.openRule = f.own(f.m.MkImplies(f.own(f.m.MkEq(x, zero)), f.application))
	fragmentMust(t, f.e.addRuleWithSymbolEager(f.openRule, symbol.Numerical(7)))
	return f
}

func (f *dualityDisplayResultFixture) promote(t *testing.T, enumeration bool) {
	t.Helper()
	if enumeration {
		forms, err := f.e.RuleFormulas()
		fragmentMust(t, err)
		defer func() { fragmentMust(t, closeASTRefs(f.m, forms)) }()
		if len(forms) == 0 {
			t.Fatal("promotion lost the source rule")
		}
	} else {
		var out bytes.Buffer
		fragmentMust(t, f.e.WriteContextSMT2(&out, nil, nil))
		for _, part := range []string{"(declare-var A Int)", "(=> (= A 0) duality-display-P)", " :named k!7)"} {
			if !strings.Contains(out.String(), part) {
				t.Fatal("display did not promote the source open rule", part, out.String())
			}
		}
	}
	if len(f.e.rawRules) != 0 {
		t.Fatal("display/enumeration left the open input pending", len(f.e.rawRules))
	}
}

func TestDualityDisplayAndEnumerationPreserveCompletedResults(t *testing.T) {
	for _, expression := range []bool{false, true} {
		for _, reachable := range []bool{false, true} {
			for _, enumeration := range []bool{false, true} {
				t.Run(fmt.Sprintf("expression=%t/reachable=%t/enumeration=%t", expression, reachable, enumeration), func(t *testing.T) {
					f := newDualityDisplayResultFixture(t)
					declaration, application, want := f.unreachable, f.unreachableApplication, lbool.False
					if reachable {
						declaration, application, want = f.predicate, f.application, lbool.True
					}
					var status lbool.LBool
					var err error
					if expression {
						status, err = f.e.QueryExpression(application)
					} else {
						status, err = f.e.QueryPredicate(declaration)
					}
					fragmentMust(t, err)
					if status != want || len(f.e.rawRules) != 1 || !f.e.rawRules[0].pending {
						t.Fatal("ordinary Duality query did not preserve its pending open input", status)
					}
					backend, persistent := f.e.duality, f.e.set
					var heldProof ast.ASTRef
					var heldModel *model.Model
					if reachable {
						heldProof, err = f.e.Proof(declaration)
						fragmentMust(t, err)
						t.Cleanup(func() { fragmentMust(t, f.m.DelRef(&heldProof)) })
					} else {
						heldModel, err = f.e.Model(declaration)
						fragmentMust(t, err)
						t.Cleanup(func() { fragmentMust(t, heldModel.Close()) })
					}
					// The fixture owns these services independently; engine Close
					// clears its borrowed fields without closing the services.
					params, globals := f.e.params, f.e.globals
					modelText := func(md *model.Model) string {
						t.Helper()
						var out bytes.Buffer
						fragmentMust(t, md.Core().WriteSMT2(&out, params, globals, 0))
						return out.String()
					}
					checkProof := func(ref ast.ASTRef) {
						t.Helper()
						h, err := f.m.RefGet(ref)
						fragmentMust(t, err)
						fact, err := f.m.GetProofFact(h)
						fragmentMust(t, err)
						if !f.m.IsFalse(fact) {
							t.Fatal("Duality refutation no longer concludes false")
						}
					}
					var originalModel, originalCertificate string
					if reachable {
						checkProof(heldProof)
					} else {
						originalModel = modelText(heldModel)
						var out bytes.Buffer
						found, err := f.e.WriteCertificate(&out, declaration, nil)
						fragmentMust(t, err)
						if !found || !strings.HasPrefix(out.String(), "(fixedpoint ") {
							t.Fatal("missing source safety certificate", found, out.String())
						}
						originalCertificate = out.String()
					}
					checkResult := func() {
						t.Helper()
						last, err := f.e.LastResult()
						fragmentMust(t, err)
						if last != want || f.e.duality != backend || f.e.set != persistent {
							t.Fatal("display changed completed result or persistent owners", last)
						}
						if _, err := f.e.Answer(declaration); !errors.Is(err, ErrPropositionalEngine) {
							t.Fatal("display enabled unsupported source Answer", err)
						}
						if reachable {
							proof, err := f.e.Proof(declaration)
							fragmentMust(t, err)
							defer func() { fragmentMust(t, f.m.DelRef(&proof)) }()
							checkProof(proof)
							checkProof(heldProof)
						} else {
							md, err := f.e.Model(declaration)
							fragmentMust(t, err)
							defer func() { fragmentMust(t, md.Close()) }()
							if modelText(md) != originalModel || modelText(heldModel) != originalModel {
								t.Fatal("display changed annotation model snapshots")
							}
							var out bytes.Buffer
							found, err := f.e.WriteCertificate(&out, declaration, nil)
							fragmentMust(t, err)
							if !found || out.String() != originalCertificate {
								t.Fatal("display changed safety certificate", found, out.String())
							}
						}
					}
					checkResult()
					f.promote(t, enumeration)
					checkResult()
					f.promote(t, enumeration)
					checkResult()
					fragmentMust(t, f.e.Close())
					if reachable {
						checkProof(heldProof)
					} else if modelText(heldModel) != originalModel {
						t.Fatal("held model lost ownership after source close")
					}
				})
			}
		}
	}
}

func TestDualityDisplayDoesNotResurrectMutatedResults(t *testing.T) {
	for _, mutation := range []string{"rule", "parameters"} {
		for _, enumeration := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/enumeration=%t", mutation, enumeration), func(t *testing.T) {
				f := newDualityDisplayResultFixture(t)
				status, err := f.e.QueryPredicate(f.predicate)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("initial reachable result", status)
				}
				if mutation == "rule" {
					fragmentMust(t, f.e.addRuleEager(f.openRule, "new-rule"))
				} else {
					local := f.e.params.EmptyValue()
					key := "duality.recursion_bound"
					fragmentMust(t, f.e.params.SetUIntName(&local, &key, 3))
					fragmentMust(t, f.e.SetQueryParameters(&local))
					fragmentMust(t, f.e.params.Del(&local))
				}
				checkStale := func() {
					t.Helper()
					if _, err := f.e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
						t.Fatal("mutation left a valid result", err)
					}
					if _, err := f.e.Proof(f.predicate); !errors.Is(err, ErrPropositionalEngine) {
						t.Fatal("stale proof became available", err)
					}
				}
				checkStale()
				f.promote(t, enumeration)
				checkStale()
			})
		}
	}
}
