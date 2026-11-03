//go:build g3trace

package fp

// Source: qe_lite.cpp:304 traces an actual solved arithmetic definition;
// dl_context.cpp:485 keeps clauses completed before a later conversion fails.

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type ruleUpdateDERCleanupWriter struct {
	boundary              ruleUpdateNumeralParserWriter
	functionSeen, matched bool
}

func (w *ruleUpdateDERCleanupWriter) Write(p []byte) (int, error) {
	if string(p) == "solve_arith" {
		w.functionSeen = true
	}
	if w.functionSeen && string(p) == "304" {
		w.matched = true
	}
	if w.matched && string(p) == "------------------------------------------------\n" {
		// Reuse the real native accounting boundary, without enabling or
		// invoking the numeral parser. DER's normal unwind owns the failure.
		w.boundary.seen = true
		return w.boundary.Write(p)
	}
	return len(p), nil
}

// Repair only the accounting boundary. In particular, this does not drain the
// update receipt: the next public operation must own that responsibility.
func (w *ruleUpdateDERCleanupWriter) restoreAccounting(t *testing.T, runtime *memory.Runtime) {
	t.Helper()
	if !w.boundary.interrupted {
		return
	}
	recovery, err := runtime.Allocate(200000)
	fragmentMust(t, err)
	fragmentMust(t, runtime.Deallocate(&w.boundary.lane))
	state := runtime.State()
	delta := int64(-500000) - state.GlobalSize - state.LocalSize
	if err = runtime.Reallocate(&recovery, uint64(200000)+uint64(delta)); err == nil || runtime.State().GlobalSize != -500000 || runtime.State().LocalSize != 0 {
		t.Fatal("restore DER accounting range", err, runtime.State())
	}
	fragmentMust(t, runtime.Deallocate(&recovery))
}

func TestFixedpointDeferredUpdatePublishesCompletedPrefix(t *testing.T) {
	for _, resume := range []string{"cleanup", "rules", "display", "open", "pop", "close"} {
		t.Run(resume, func(t *testing.T) {
			f := newRuleUpdateNumeralReadFixture(t)
			boolean := f.own(f.m.MkBoolSort())
			predicate := func(text string) ast.ASTHandle {
				h := f.own(f.m.MkFreshConst(text, boolean))
				d, err := f.m.GetDecl(h)
				fragmentMust(t, err)
				fragmentMust(t, f.e.RegisterPredicate(d))
				return h
			}
			p, h := predicate("deferred-P"), predicate("deferred-H")
			family, err := f.m.ArithmeticFamilyID()
			fragmentMust(t, err)
			real := f.own(f.m.MkFamilySort(family, 0, nil))
			x := f.own(f.m.MkVar(0, real))
			text := strings.Repeat("9", 600)
			wide := f.own(f.m.ArithmeticRealNumeralText(text))
			one := f.own(f.m.ArithmeticRealNumeralText("1"))
			product := f.own(f.m.MkArithmeticMul(wide, x))
			equality := f.own(f.m.MkEq(product, one))
			body := f.own(f.m.MkOr(equality, p))
			implication := f.own(f.m.MkImplies(body, h))
			formula := f.own(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{real}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: implication}))
			fromP := f.own(f.m.MkImplies(p, h))
			name := symbol.Numerical(7)
			rawName, err := f.m.Symbols.Intern("deferred-pending")
			fragmentMust(t, err)
			writer := &ruleUpdateDERCleanupWriter{boundary: ruleUpdateNumeralParserWriter{t: t, runtime: f.runtime}}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("qe_lite")
			t.Cleanup(tags.Finalize)
			cooperation, err := cooperate.NewCooperation(f.m.Budget())
			fragmentMust(t, err)
			lite, err := qe.NewLite(f.m, nil, false, qe.Config{Context: f.e.params, Globals: f.e.globals, Cooperation: cooperation, Trace: tags})
			fragmentMust(t, err)
			f.e.unboundLite, f.e.unboundCooperation = lite, cooperation
			// Install the traced owner before ingestion lazily constructs one.
			// This predicate-only input cannot reach solve_arith's boundary.
			fragmentMust(t, f.e.AddRuleWithSymbolAndBound(p, rawName, 17))
			if resume == "pop" {
				fragmentMust(t, f.e.Push())
			}
			rawBefore := append([]rawRuleFormula(nil), f.e.rawRules...)
			persistent, revision := f.e.set, f.e.revision
			cause := f.e.UpdateRuleWithSymbol(formula, name)
			writer.restoreAccounting(t, f.runtime)
			var receipt memory.ConstructionFailure
			hasReceipt := errors.As(cause, &receipt)
			if hasReceipt {
				// Also leave pre-fix runs clean before reporting the lost prefix.
				defer func() {
					fragmentMust(t, receipt.Cleanup())
					fragmentMust(t, receipt.Cleanup())
				}()
			}
			if !writer.matched || !writer.boundary.interrupted || !hasReceipt || !errors.Is(cause, arena.ErrBounds) || errors.Is(cause, numeral.ErrRationalLexicalPending) || strings.Contains(cause.Error(), "non-trivial body") {
				t.Fatal("update missed its real DER failure and owning receipt", cause, writer.matched, writer.boundary.interrupted)
			}
			original, message := receipt.OriginalCause(), cause.Error()
			checkForms := func(forms []ast.ASTRef, want []ast.ASTHandle) {
				t.Helper()
				defer func() { fragmentMust(t, closeASTRefs(f.m, forms)) }()
				if len(forms) != len(want) {
					t.Fatal("completed update prefix was lost or published more than once", len(forms), len(want))
				}
				for i, ref := range forms {
					got, err := f.m.RefGet(ref)
					fragmentMust(t, err)
					if got != want[i] {
						t.Fatal("deferred update changed source clause order", i, got, want[i])
					}
				}
			}
			switch resume {
			case "cleanup":
				fragmentMust(t, receipt.Cleanup())
				fragmentMust(t, receipt.Cleanup())
			case "rules":
				forms, err := f.e.Rules()
				fragmentMust(t, err)
				checkForms(forms, []ast.ASTHandle{fromP, p})
			case "display":
				var out bytes.Buffer
				fragmentMust(t, f.e.WriteContextSMT2(&out, nil, nil))
				if strings.Count(out.String(), "(rule ") != 2 || !strings.Contains(out.String(), ":named k!7") {
					t.Fatal("context display omitted the completed update prefix", out.String())
				}
			case "open":
				_, err := f.e.Predicates()
				fragmentMust(t, err)
			case "pop":
				fragmentMust(t, f.e.Pop())
			case "close":
				fragmentMust(t, f.e.Close())
				fragmentMust(t, receipt.Cleanup())
				fragmentMust(t, receipt.Cleanup())
				if cause.Error() != message || receipt.OriginalCause() != original {
					t.Fatal("engine close changed the original update failure")
				}
				return
			}
			// Inspect the persistent storage before another public call can
			// mask a missed open/Pop/receipt continuation.
			wantCount := 2
			if resume == "pop" {
				wantCount = 1
			}
			count, err := f.e.set.Len()
			fragmentMust(t, err)
			if count != wantCount {
				t.Fatal("resume did not publish exactly the completed prefix", count, wantCount)
			}
			fragmentMust(t, receipt.Cleanup())
			fragmentMust(t, receipt.Cleanup())
			if cause.Error() != message || receipt.OriginalCause() != original || !errors.Is(cause, arena.ErrBounds) {
				t.Fatal("deferred cleanup changed the original update failure")
			}
			forms, names, err := f.e.NamedRulesWithSymbols()
			fragmentMust(t, err)
			wantForms, wantNames := []ast.ASTHandle{fromP, p}, []symbol.Symbol{name, rawName}
			if resume == "pop" {
				wantForms, wantNames = []ast.ASTHandle{p}, []symbol.Symbol{rawName}
			}
			checkForms(forms, wantForms)
			if !reflect.DeepEqual(names, wantNames) {
				t.Fatal("deferred update changed typed labels", names, wantNames)
			}
			if len(f.e.rawRules) != len(rawBefore) || (resume != "pop" && (f.e.set != persistent || f.e.revision != revision)) {
				t.Fatal("deferred update changed the raw archive or persistent context identity")
			}
			for i, want := range rawBefore {
				if resume == "pop" {
					gotFormula, err := f.m.RefGet(f.e.rawRules[i].formula)
					fragmentMust(t, err)
					if gotFormula != p {
						t.Fatal("pop changed the archived source formula", gotFormula)
					}
					want.formula = f.e.rawRules[i].formula
				} else {
					want.first++
					want.end++
				}
				if f.e.rawRules[i] != want {
					t.Fatal("deferred update changed raw input metadata", f.e.rawRules[i], want)
				}
			}
			gotBody, err := f.m.GetQuantifierBody(formula)
			fragmentMust(t, err)
			gotText, _, matched, err := f.m.ArithmeticNumeralText(wide)
			fragmentMust(t, err)
			if gotBody != implication || !matched || gotText != text {
				t.Fatal("deferred update changed borrowed source input")
			}
		})
	}
}
