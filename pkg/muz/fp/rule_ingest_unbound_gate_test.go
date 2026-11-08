package fp

// Source: dl_rule.cpp:218 gates the complete fix_unbound_vars call, including
// its initial QE-lite reduction. Z3 4.7.1 preserves (= B 0) when disabled and
// reduces the same pending rule to P(A) when enabled at display/query time.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRuleIngestionUnboundGateUsesCurrentPhaseParameters(t *testing.T) {
	for _, phase := range []string{"open-display", "closed-query"} {
		for _, initial := range []bool{false, true} {
			for _, current := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/initial=%t/current=%t", phase, initial, current), func(t *testing.T) {
					m, globals := newPDRLifecycleServices(t)
					fragmentMust(t, globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", fmt.Sprint(initial)))
					e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
					fragmentMust(t, err)
					f := &fpRuleNameFixture{t: t, m: m, e: e}
					var snapshots []ast.ASTRef
					t.Cleanup(func() {
						fragmentMust(t, e.Close())
						fragmentMust(t, closeASTRefs(m, snapshots))
						fragmentMust(t, closeASTRefs(m, f.roots))
					})
					fragmentMust(t, e.SelectEngine("spacer"))
					zero := f.own(m.ArithmeticIntegerNumeralText("0"))
					seven := f.own(m.ArithmeticIntegerNumeralText("7"))
					integer, err := m.GetSort(zero)
					fragmentMust(t, err)
					x := f.own(m.MkVar(0, integer))
					y := f.own(m.MkVar(1, integer))
					predicate, head := f.predicate("unbound-gate-P", x)
					equality := f.own(m.MkEq(y, zero))
					implication := f.own(m.MkImplies(equality, head))
					input := implication
					if phase == "closed-query" {
						input = f.own(m.MkQuantifier(ast.ASTQuantifierSpec{
							Forall: true, Sorts: []ast.ASTHandle{integer, integer},
							Names: []symbol.Symbol{symbol.Numerical(1), symbol.Numerical(0)}, Body: implication,
						}))
					}
					label := symbol.Numerical(7)
					fragmentMust(t, e.addRuleWithSymbolEager(input, label))
					if !initial && (e.unboundLite != nil || e.unboundCooperation != nil) {
						t.Fatal("disabled ingestion created QE-lite services")
					}
					liteBefore := e.unboundLite
					persistent := e.set
					setCurrent := func(enabled bool) {
						t.Helper()
						local := globals.Context.EmptyValue()
						key := "xform.fix_unbound_vars"
						fragmentMust(t, globals.Context.SetBoolName(&local, &key, enabled))
						fragmentMust(t, e.SetQueryParameters(&local))
						fragmentMust(t, globals.Context.Del(&local))
					}
					setCurrent(current)
					checkNormalized := func() ast.ASTHandle {
						t.Helper()
						if e.set != persistent {
							t.Fatal("normalization replaced persistent rule-set identity")
						}
						rule, err := e.set.Rule(0)
						fragmentMust(t, err)
						defer func() { fragmentMust(t, rule.Close()) }()
						name, err := rule.NameSymbol()
						fragmentMust(t, err)
						if name != label {
							t.Fatal("normalization changed typed label", name, label)
						}
						size, err := rule.TailSize()
						fragmentMust(t, err)
						wantSize := 1
						if current {
							wantSize = 0
						}
						if size != wantSize {
							t.Fatal("current option did not gate equality reduction", size, wantSize)
						}
						if !current {
							tail, negative, err := rule.Tail(0)
							fragmentMust(t, err)
							if negative || tail != equality {
								t.Fatal("disabled normalization changed the equality body", tail, equality)
							}
						}
						forms, names, err := e.NamedRulesWithSymbols()
						snapshots = append(snapshots, forms...)
						fragmentMust(t, err)
						if len(forms) != 1 || len(names) != 1 || names[0] != label {
							t.Fatal("normalized enumeration changed typed clause", len(forms), names)
						}
						h, err := m.RefGet(forms[0])
						fragmentMust(t, err)
						body, err := m.GetQuantifierBody(h)
						fragmentMust(t, err)
						want := implication
						if current {
							want = head
						}
						if body != want {
							t.Fatal("normalized formula differs from source parameter gate", body, want)
						}
						return h
					}
					var beforeQuery ast.ASTHandle
					if phase == "open-display" {
						var display bytes.Buffer
						fragmentMust(t, e.WriteContextSMT2(&display, nil, nil))
						wantRule := "(rule (! (=> (= B 0) (unbound-gate-P A)) :named k!7))"
						if current {
							wantRule = "(rule (! (unbound-gate-P A) :named k!7))"
						}
						if !strings.Contains(display.String(), wantRule) {
							t.Fatal("display differs from Z3 4.7.1 source oracle", display.String(), wantRule)
						}
						beforeQuery = checkNormalized()
					} else {
						pending, err := e.RuleFormulas()
						snapshots = append(snapshots, pending...)
						fragmentMust(t, err)
						if len(pending) != 1 {
							t.Fatal("pending closed rule count", len(pending))
						}
						h, err := m.RefGet(pending[0])
						fragmentMust(t, err)
						if h != input {
							t.Fatal("enumeration prematurely normalized closed input")
						}
					}
					query := f.own(m.MkApp(predicate, []ast.ASTHandle{seven}))
					status, err := e.QueryExpression(query)
					fragmentMust(t, err)
					if status != lbool.True {
						t.Fatal("source clause did not prove P(7)", status)
					}
					afterQuery := checkNormalized()
					if !beforeQuery.IsNull() && afterQuery != beforeQuery {
						t.Fatal("query changed an already normalized persistent clause")
					}
					if !current && e.unboundLite != liteBefore {
						t.Fatal("disabled pending normalization created QE-lite services")
					}
					setCurrent(!current)
					if checkNormalized() != afterQuery {
						t.Fatal("later parameter change rebuilt a normalized clause")
					}
					fragmentMust(t, e.Close())
					for _, ref := range snapshots {
						_, err := m.RefGet(ref)
						fragmentMust(t, err)
					}
				})
			}
		}
	}
}
