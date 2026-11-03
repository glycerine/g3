//go:build g3trace

package fp

// Source dl_rule.h:117 owns the qe_lite used by dl_rule.cpp:627. Query-local
// rule copies still own that reducer after an interrupted arithmetic cleanup.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExpressionQueryClonePreservesUnboundReducerAfterNativeCleanupFailure(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		for _, resume := range []string{"cleanup", "query", "close"} {
			t.Run(backend+"/"+resume, func(t *testing.T) {
				f := newRuleUpdateNumeralReadFixture(t)
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", "false"))
				fragmentMust(t, f.e.SelectEngine(backend))
				boolean := f.own(f.m.MkBoolSort())
				head := f.own(f.m.MkFreshConst("clone-unbound-H", boolean))
				declaration, err := f.m.GetDecl(head)
				fragmentMust(t, err)
				fragmentMust(t, f.e.RegisterPredicate(declaration))
				family, err := f.m.ArithmeticFamilyID()
				fragmentMust(t, err)
				real := f.own(f.m.MkFamilySort(family, 0, nil))
				x := f.own(f.m.MkVar(0, real))
				wide := f.own(f.m.ArithmeticRealNumeralText(strings.Repeat("9", 600)))
				one := f.own(f.m.ArithmeticRealNumeralText("1"))
				product := f.own(f.m.MkArithmeticMul(wide, x))
				equality := f.own(f.m.MkEq(product, one))
				implication := f.own(f.m.MkImplies(equality, head))
				formula := f.own(f.m.MkQuantifier(ast.ASTQuantifierSpec{
					Forall: true, Sorts: []ast.ASTHandle{real},
					Names: []symbol.Symbol{symbol.Numerical(0)}, Body: implication,
				}))
				name := symbol.Numerical(7)
				// UpdateRule installs a normalized rule without a pending raw input.
				// Enabling unbound reduction now reaches DER only while the public
				// expression query copies that rule into its private context.
				fragmentMust(t, f.e.UpdateRuleWithSymbol(formula, name))
				persistent, revision := f.e.set, f.e.revision
				count, err := persistent.Len()
				fragmentMust(t, err)
				if count != 1 || len(f.e.rawRules) != 0 || f.e.unboundLite != nil {
					t.Fatal("fixture did not isolate clone-local unbound reduction")
				}
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", "true"))
				writer := &ruleUpdateDERCleanupWriter{boundary: ruleUpdateNumeralParserWriter{t: t, runtime: f.runtime}}
				tags := diagnostic.NewTraceTagsWithWriter(writer)
				tags.Enable("qe_lite")
				t.Cleanup(tags.Finalize)
				var result lbool.LBool
				var cause error
				func() {
					// The lazily constructed clone uses source process trace tags.
					// Bind this single-goroutine test's stream only for the query.
					process := diagnostic.BorrowProcessTraceTags()
					saved := *process
					*process = *tags
					defer func() { *process = saved }()
					result, cause = f.e.QueryExpression(head)
				}()
				writer.restoreAccounting(t, f.runtime)
				var receipt memory.ConstructionFailure
				if result != lbool.Undef || !writer.matched || !writer.boundary.interrupted || !errors.Is(cause, arena.ErrBounds) || !errors.As(cause, &receipt) {
					t.Fatal("query missed its actual source DER cleanup failure", result, cause)
				}
				if len(f.e.queryCleanup) != 1 || f.e.queryClone != nil || f.e.queryReady {
					t.Fatal("failed query did not preserve exactly its discarded clone")
				}
				clone := f.e.queryCleanup[0].clone
				if clone.closed || !clone.closing || clone.unboundLite == nil || clone.unboundCooperation == nil || clone.set == nil || clone.rules == nil || clone.binder == nil || clone.relations == nil {
					t.Fatal("clone abandoned its interrupted reducer or later owners", clone.closed, clone.unboundLite, clone.unboundCooperation)
				}
				original, message := receipt.OriginalCause(), cause.Error()
				switch resume {
				case "cleanup":
					// This receipt drains the reducer's invocation; it does not
					// destroy the persistent reducer and its traversal owners.
					fragmentMust(t, receipt.Cleanup())
					fragmentMust(t, receipt.Cleanup())
					fragmentMust(t, f.e.clearQueryResources())
				case "query":
					result, err = f.e.QueryExpression(head)
					fragmentMust(t, err)
					if result != lbool.True {
						t.Fatal("query retry lost the arithmetic rule", result)
					}
				case "close":
					fragmentMust(t, f.e.Close())
					fragmentMust(t, f.e.Close())
				}
				fragmentMust(t, receipt.Cleanup())
				fragmentMust(t, receipt.Cleanup())
				if receipt.OriginalCause() != original || cause.Error() != message || !errors.Is(cause, arena.ErrBounds) {
					t.Fatal("cleanup changed the original source failure")
				}
				if len(f.e.queryCleanup) != 0 || !clone.closed || clone.unboundLite != nil || clone.unboundCooperation != nil || clone.rules != nil || clone.binder != nil || clone.relations != nil {
					t.Fatal("retry did not finish the queued clone's owners")
				}
				if resume != "close" {
					count, err = persistent.Len()
					fragmentMust(t, err)
					if f.e.set != persistent || f.e.revision != revision || count != 1 || len(f.e.rawRules) != 0 {
						t.Fatal("failed query or retry changed the persistent source program")
					}
					rule, err := persistent.Rule(0)
					fragmentMust(t, err)
					label, err := rule.NameSymbol()
					fragmentMust(t, err)
					fragmentMust(t, rule.Close())
					if label != name {
						t.Fatal("query retry changed the source rule label")
					}
				}
				// The shared native fixture checks zero budget after all caller
				// roots and services close; an abandoned Lite leaves 13 KB live.
			})
		}
	}
}
