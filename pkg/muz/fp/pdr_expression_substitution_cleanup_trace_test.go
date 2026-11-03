//go:build g3trace

package fp

// Source pdr_context.cpp:2294-2305 evaluates later predecessor variables and
// substitutes their values into a literal connecting multiple predecessors.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type pdrSubstitutionNativeFailureWriter struct {
	engine                   *PropositionalEngine
	context                  *pdr.Context
	variable                 string
	cleanup                  ruleUpdateDERCleanupWriter
	operator, site, operand  bool
	transferred, initialized bool
}

func (w *pdrSubstitutionNativeFailureWriter) Write(p []byte) (int, error) {
	text := string(p)
	if text == "operator()" {
		w.operator = true
	}
	if w.operator && text == "566" {
		w.site = true
	}
	if w.site && text == w.variable {
		w.operand = true
	}
	if text != "------------------------------------------------\n" {
		return len(p), nil
	}
	matched := w.operator && w.site && w.operand
	w.operator, w.site, w.operand = false, false, false
	if matched && !w.cleanup.boundary.interrupted {
		initialized, err := w.context.Initialized()
		if err == nil && initialized && w.engine.pdr == nil {
			w.transferred, w.initialized = true, true
			w.cleanup.boundary.seen = true
			return w.cleanup.boundary.Write(p)
		}
	}
	return len(p), nil
}

func TestPDRExpressionSubstitutionRecoversAfterNativeFailure(t *testing.T) {
	for _, resume := range []string{"query", "close"} {
		t.Run(resume, func(t *testing.T) {
			f := newRuleUpdateNumeralReadFixture(t)
			for _, key := range []string{"fixedpoint.xform.fix_unbound_vars", "fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker", "fixedpoint.pdr.use_model_generalizer"} {
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
			const secondName = "substitution-native-Q"
			p, q := predicate("substitution-native-P"), predicate(secondName)
			head := f.own(f.m.MkFreshConst("substitution-native-R", boolean))
			r, err := f.m.GetDecl(head)
			fragmentMust(t, err)
			fragmentMust(t, f.e.RegisterPredicate(r))
			wide := f.own(f.m.ArithmeticRealNumeralText(strings.Repeat("9", 600)))
			labels := []symbol.Symbol{symbol.Numerical(7), symbol.Numerical(8), symbol.Numerical(9)}
			for index, declaration := range []ast.ASTHandle{p, q} {
				fact := f.own(f.m.MkApp(declaration, []ast.ASTHandle{wide}))
				fragmentMust(t, f.e.UpdateRuleWithSymbol(fact, labels[index]))
			}
			x, y := f.own(f.m.MkVar(0, real)), f.own(f.m.MkVar(1, real))
			px := f.own(f.m.MkApp(p, []ast.ASTHandle{x}))
			qy := f.own(f.m.MkApp(q, []ast.ASTHandle{y}))
			equality := f.own(f.m.MkEq(x, y))
			lowerBound := f.own(f.m.MkArithmeticGE(y, wide))
			body := f.own(f.m.MkAndArray([]ast.ASTHandle{px, qy, equality, lowerBound}))
			rule := f.own(f.m.MkImplies(body, head))
			fragmentMust(t, f.e.UpdateRuleWithSymbol(rule, labels[2]))
			_, err = f.e.NumLevels(r)
			fragmentMust(t, err)
			prequery := f.e.pdr
			if prequery == nil {
				t.Fatal("missing public prequery context")
			}
			persistent, revision := f.e.set, f.e.revision
			checkProgram := func() {
				count, err := persistent.Len()
				fragmentMust(t, err)
				if f.e.set != persistent || f.e.revision != revision || count != 3 || len(f.e.rawRules) != 0 {
					t.Fatal("model substitution failure changed the persistent source program")
				}
				for index, declaration := range []ast.ASTHandle{p, q, r} {
					rule, err := persistent.Rule(index)
					fragmentMust(t, err)
					name, err := rule.NameSymbol()
					fragmentMust(t, err)
					head, err := rule.HeadDeclaration()
					fragmentMust(t, err)
					fragmentMust(t, rule.Close())
					if name != labels[index] || head != declaration {
						t.Fatal("model substitution changed the source rule or typed label", index)
					}
				}
			}
			checkProgram()
			proofMode, err := f.m.ProofMode()
			fragmentMust(t, err)
			// The source signature names argument 0 of predecessor 1 Q_0_1.
			// In this fixture, evaluating that bare variable reaches the coupled
			// substitution. The required error context below also verifies the
			// selected operation, without relying on a global event ordinal.
			writer := &pdrSubstitutionNativeFailureWriter{
				engine: f.e, context: prequery, variable: secondName + "_0_1",
				cleanup: ruleUpdateDERCleanupWriter{boundary: ruleUpdateNumeralParserWriter{t: t, runtime: f.runtime}},
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("model_evaluator")
			t.Cleanup(tags.Finalize)
			var status lbool.LBool
			var cause error
			func() {
				process := diagnostic.BorrowProcessTraceTags()
				saved := *process
				*process = *tags
				defer func() { *process = saved }()
				status, cause = f.e.QueryExpression(head)
			}()
			// Repair native accounting only; no error receipt or private
			// cleanup operation is used to recover the failed expression query.
			writer.cleanup.restoreAccounting(t, f.runtime)
			if !writer.cleanup.boundary.interrupted || !writer.transferred || !writer.initialized || status != lbool.Undef || !errors.Is(cause, arena.ErrBounds) || !strings.Contains(cause.Error(), "projected child construction: muz/pdr: coupled-state model substitution") {
				t.Fatal("query missed the native coupled-state substitution failure", status, cause)
			}
			clone := f.e.queryClone
			if clone == nil || clone.pdr != prequery || clone.pdrRules == nil || f.e.pdr != nil {
				t.Fatal("substitution failure lost the initialized transferred context")
			}
			initialized, err := prequery.Initialized()
			fragmentMust(t, err)
			if !initialized {
				t.Fatal("substitution failure lost its installed rules")
			}
			if _, err = f.e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("failed substitution published a query result", err)
			}
			if _, err = f.e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("failed substitution published an expression answer", err)
			}
			currentMode, err := f.m.ProofMode()
			fragmentMust(t, err)
			if currentMode != proofMode {
				t.Fatal("substitution failure changed the caller proof mode")
			}
			checkProgram()
			message := cause.Error()
			if resume == "query" {
				status, err = f.e.QueryExpression(head)
				fragmentMust(t, err)
				if status != lbool.True || f.e.queryClone == nil || f.e.queryClone == clone || f.e.queryClone.pdr == prequery || !clone.closed {
					t.Fatal("public retry did not retire the failed context and derive R", status)
				}
				if _, err = prequery.Initialized(); err == nil {
					t.Fatal("public retry left the old context open")
				}
				checkProgram()
			}
			fragmentMust(t, f.e.Close())
			fragmentMust(t, f.e.Close())
			if !clone.closed {
				t.Fatal("public close left the failed query clone open")
			}
			if _, err = prequery.Initialized(); err == nil {
				t.Fatal("public close left the transferred context open")
			}
			if cause.Error() != message || !errors.Is(cause, arena.ErrBounds) {
				t.Fatal("public recovery changed the original native failure")
			}
			// The shared fixture requires zero budget after closing all caller
			// roots and services, without any later receipt-based salvage.
		})
	}
}
