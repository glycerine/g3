//go:build g3trace

package fp

// Source pdr_context.cpp:2192-2242 projects the current head signature while
// keeping predecessor variables; qe_lite.cpp:304 solves its arithmetic equality.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type pdrProjectionNativeFailureWriter struct {
	engine                   *PropositionalEngine
	context                  *pdr.Context
	coefficient              string
	cleanup                  ruleUpdateDERCleanupWriter
	function, site, wide     bool
	transferred, initialized bool
}

func (w *pdrProjectionNativeFailureWriter) Write(p []byte) (int, error) {
	text := string(p)
	if text == "solve_arith" {
		w.function = true
	}
	if w.function && text == "304" {
		w.site = true
	}
	if w.site && strings.Contains(text, w.coefficient) {
		w.wide = true
	}
	if text != "------------------------------------------------\n" {
		return len(p), nil
	}
	matched := w.function && w.site && w.wide
	w.function, w.site, w.wide = false, false, false
	if matched && !w.cleanup.boundary.interrupted {
		// The copied rule has its predecessor variable positively bound and
		// unbound-variable reduction disabled. Only the installed PDR context
		// projects this head variable after the public ownership transfer.
		initialized, err := w.context.Initialized()
		if err == nil && initialized && w.engine.pdr == nil {
			w.transferred, w.initialized = true, true
			w.cleanup.boundary.seen = true
			return w.cleanup.boundary.Write(p)
		}
	}
	return len(p), nil
}

func TestPDRExpressionProjectionPreservesNativeCleanupReceipt(t *testing.T) {
	for _, resume := range []string{"cleanup", "query", "close"} {
		t.Run(resume, func(t *testing.T) {
			f := newRuleUpdateNumeralReadFixture(t)
			for _, key := range []string{"fixedpoint.xform.fix_unbound_vars", "fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
				fragmentMust(t, f.e.globals.SourceSetName(key, "false"))
			}
			fragmentMust(t, f.e.SelectEngine("pdr"))
			boolean := f.own(f.m.MkBoolSort())
			family, err := f.m.ArithmeticFamilyID()
			fragmentMust(t, err)
			real := f.own(f.m.MkFamilySort(family, 0, nil))
			predicate := func(text string) ast.ASTHandle {
				name, err := f.m.Symbols.Intern(text)
				fragmentMust(t, err)
				declaration := f.own(f.m.MkFuncDecl(name, []ast.ASTHandle{real}, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, f.e.RegisterPredicate(declaration))
				return declaration
			}
			p, q := predicate("projected-cleanup-P"), predicate("projected-cleanup-Q")
			zero := f.own(f.m.ArithmeticRealNumeralText("0"))
			one := f.own(f.m.ArithmeticRealNumeralText("1"))
			coefficient := strings.Repeat("9", 600)
			wide := f.own(f.m.ArithmeticRealNumeralText(coefficient))
			fact := f.own(f.m.MkApp(p, []ast.ASTHandle{one}))
			x, y := f.own(f.m.MkVar(0, real)), f.own(f.m.MkVar(1, real))
			tail := f.own(f.m.MkApp(p, []ast.ASTHandle{x}))
			head := f.own(f.m.MkApp(q, []ast.ASTHandle{y}))
			product := f.own(f.m.MkArithmeticMul(wide, y))
			sum := f.own(f.m.MkArithmeticAdd(product, one))
			equality := f.own(f.m.MkEq(x, sum))
			body := f.own(f.m.MkAnd(tail, equality))
			edge := f.own(f.m.MkImplies(body, head))
			labels := []symbol.Symbol{symbol.Numerical(7), symbol.Numerical(8)}
			fragmentMust(t, f.e.UpdateRuleWithSymbol(fact, labels[0]))
			fragmentMust(t, f.e.UpdateRuleWithSymbol(edge, labels[1]))
			query := f.own(f.m.MkApp(q, []ast.ASTHandle{zero}))
			_, err = f.e.NumLevels(q)
			fragmentMust(t, err)
			prequery := f.e.pdr
			if prequery == nil {
				t.Fatal("missing public prequery context")
			}
			persistent, revision := f.e.set, f.e.revision
			checkProgram := func() {
				count, err := persistent.Len()
				fragmentMust(t, err)
				if f.e.set != persistent || f.e.revision != revision || count != 2 || len(f.e.rawRules) != 0 {
					t.Fatal("projection cleanup changed the persistent source program")
				}
				for index, label := range labels {
					rule, err := persistent.Rule(index)
					fragmentMust(t, err)
					got, err := rule.NameSymbol()
					fragmentMust(t, err)
					fragmentMust(t, rule.Close())
					if got != label {
						t.Fatal("projection cleanup changed the typed source label", index, got, label)
					}
				}
			}
			checkProgram()
			proofMode, err := f.m.ProofMode()
			fragmentMust(t, err)
			writer := &pdrProjectionNativeFailureWriter{
				engine: f.e, context: prequery, coefficient: coefficient,
				cleanup: ruleUpdateDERCleanupWriter{boundary: ruleUpdateNumeralParserWriter{t: t, runtime: f.runtime}},
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("qe_lite")
			t.Cleanup(tags.Finalize)
			var status lbool.LBool
			var cause error
			func() {
				process := diagnostic.BorrowProcessTraceTags()
				saved := *process
				*process = *tags
				defer func() { *process = saved }()
				status, cause = f.e.QueryExpression(query)
			}()
			// This repairs native accounting only. It does not drain the lower
			// QE invocation or retire the context that received the query rules.
			writer.cleanup.restoreAccounting(t, f.runtime)
			var receipt *qe.LiteFailure
			if !writer.cleanup.boundary.interrupted || !writer.transferred || !writer.initialized || status != lbool.Undef || !errors.Is(cause, arena.ErrBounds) || !errors.As(cause, &receipt) || !strings.Contains(cause.Error(), "projected child construction: muz/pdr: QE-lite projection") {
				t.Fatal("query missed its source projected-child cleanup failure", status, cause)
			}
			if receipt.RuntimeOwner() != f.runtime {
				t.Fatal("projection receipt lost its native runtime")
			}
			clone := f.e.queryClone
			if clone == nil || clone.pdr != prequery || clone.pdrRules == nil || f.e.pdr != nil {
				t.Fatal("projection failure lost the transferred initialized context")
			}
			initialized, err := prequery.Initialized()
			fragmentMust(t, err)
			if !initialized {
				t.Fatal("projection failure discarded the installed rules")
			}
			if _, err = f.e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("failed projection published a query result", err)
			}
			if _, err = f.e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("failed projection published an expression answer", err)
			}
			currentMode, err := f.m.ProofMode()
			fragmentMust(t, err)
			if currentMode != proofMode {
				t.Fatal("projection failure changed the caller proof mode")
			}
			checkProgram()
			original, message := receipt.OriginalCause(), cause.Error()
			if resume == "cleanup" {
				fragmentMust(t, receipt.Cleanup())
				fragmentMust(t, receipt.Cleanup())
			}
			if resume == "close" {
				fragmentMust(t, f.e.Close())
				fragmentMust(t, f.e.Close())
			} else {
				status, err = f.e.QueryExpression(query)
				fragmentMust(t, err)
				if status != lbool.True || f.e.queryClone == nil || f.e.queryClone == clone || f.e.queryClone.pdr == prequery {
					t.Fatal("query retry did not derive Q(0) from P(1) and its affine rule", status)
				}
				if !clone.closed {
					t.Fatal("query retry left its predecessor clone open")
				}
				if _, err = prequery.Initialized(); err == nil {
					t.Fatal("query retry left the transferred context open")
				}
				checkProgram()
				fragmentMust(t, f.e.Close())
				fragmentMust(t, f.e.Close())
			}
			if !clone.closed {
				t.Fatal("public recovery left the failed query clone open")
			}
			if _, err = prequery.Initialized(); err == nil {
				t.Fatal("public recovery left the transferred context open")
			}
			counts, budget := contextDisplayLiveCounts(f.m.Stats()), f.m.Budget().Used()
			fragmentMust(t, receipt.Cleanup())
			fragmentMust(t, receipt.Cleanup())
			if contextDisplayLiveCounts(f.m.Stats()) != counts || f.m.Budget().Used() != budget {
				t.Fatal("late receipt cleanup rescued resources missed by public recovery")
			}
			if receipt.OriginalCause() != original || receipt.RuntimeOwner() != f.runtime || cause.Error() != message || !errors.Is(cause, arena.ErrBounds) {
				t.Fatal("public recovery changed the original projection failure")
			}
			// The shared fixture closes caller roots and services and requires
			// zero budget, including the lower invocation and both contexts.
		})
	}
}
