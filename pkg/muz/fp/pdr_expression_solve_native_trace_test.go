//go:build g3trace

package fp

// Source pdr_context.cpp:1888 enters solve after pdr_dl_interface has installed
// its query-local rules. The source context owns the solver through failure.

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

type pdrSolveNativeFailureWriter struct {
	engine      *PropositionalEngine
	cleanup     ruleUpdateDERCleanupWriter
	transferred bool
}

func (w *pdrSolveNativeFailureWriter) Write(p []byte) (int, error) {
	if string(p) == "1888" {
		w.cleanup.matched = true
	}
	if w.cleanup.matched && !w.cleanup.boundary.interrupted && string(p) == "------------------------------------------------\n" {
		w.transferred = w.engine.pdr == nil
		w.cleanup.boundary.seen = true
		return w.cleanup.boundary.Write(p)
	}
	return len(p), nil
}

func TestPDRExpressionTransferredOwnerSurvivesNativeSolveFailure(t *testing.T) {
	f := newRuleUpdateNumeralReadFixture(t)
	fragmentMust(t, f.e.SelectEngine("pdr"))
	for _, key := range []string{"fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
		fragmentMust(t, f.e.globals.SourceSetName(key, "false"))
	}
	boolean := f.own(f.m.MkBoolSort())
	family, err := f.m.ArithmeticFamilyID()
	fragmentMust(t, err)
	real := f.own(f.m.MkFamilySort(family, 0, nil))
	name, err := f.m.Symbols.Intern("transferred-wide-P")
	fragmentMust(t, err)
	declaration := f.own(f.m.MkFuncDecl(name, []ast.ASTHandle{real}, boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(t, f.e.RegisterPredicate(declaration))
	wide := f.own(f.m.ArithmeticRealNumeralText(strings.Repeat("9", 600)))
	fact := f.own(f.m.MkApp(declaration, []ast.ASTHandle{wide}))
	label := symbol.Numerical(7)
	fragmentMust(t, f.e.UpdateRuleWithSymbol(fact, label))
	variable := f.own(f.m.MkVar(0, real))
	query := f.own(f.m.MkApp(declaration, []ast.ASTHandle{variable}))
	_, err = f.e.NumLevels(declaration)
	fragmentMust(t, err)
	prequery := f.e.pdr
	if prequery == nil {
		t.Fatal("missing public prequery owner")
	}
	persistent, revision := f.e.set, f.e.revision
	proofMode, err := f.m.ProofMode()
	fragmentMust(t, err)
	writer := &pdrSolveNativeFailureWriter{engine: f.e, cleanup: ruleUpdateDERCleanupWriter{boundary: ruleUpdateNumeralParserWriter{t: t, runtime: f.runtime}}}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("pdr")
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
	// Repair only native accounting; the engine must finish its own solver
	// state before attempting the next public expression query.
	writer.cleanup.restoreAccounting(t, f.runtime)
	if !writer.cleanup.matched || !writer.cleanup.boundary.interrupted || !writer.transferred || status != lbool.Undef || !errors.Is(cause, arena.ErrBounds) || !strings.Contains(cause.Error(), "smt kernel configure asserted bounds") {
		t.Fatal("query missed the native solve failure after owner transfer", status, cause)
	}
	clone := f.e.queryClone
	if clone == nil || clone.pdr != prequery || clone.pdrRules == nil || f.e.pdr != nil {
		t.Fatal("native failure lost the initialized transferred context")
	}
	initialized, err := prequery.Initialized()
	fragmentMust(t, err)
	if !initialized {
		t.Fatal("native solve failure lost inspectable installed rules")
	}
	if _, err = f.e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("failed solve published a query result", err)
	}
	if _, err = f.e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("failed solve published an expression answer", err)
	}
	currentMode, err := f.m.ProofMode()
	fragmentMust(t, err)
	if currentMode != proofMode {
		t.Fatal("native solve failure changed the caller proof mode")
	}
	message := cause.Error()
	status, err = f.e.QueryExpression(query)
	fragmentMust(t, err)
	if status != lbool.True || f.e.queryClone == nil || f.e.queryClone == clone || f.e.queryClone.pdr == prequery || !clone.closed {
		t.Fatal("public retry did not retire the failed owner and solve its rule", status)
	}
	if _, err = prequery.Initialized(); err == nil {
		t.Fatal("query retry left the prior context open")
	}
	count, err := persistent.Len()
	fragmentMust(t, err)
	if f.e.set != persistent || f.e.revision != revision || count != 1 || len(f.e.rawRules) != 0 {
		t.Fatal("query failure or retry changed the persistent source program")
	}
	rule, err := persistent.Rule(0)
	fragmentMust(t, err)
	gotLabel, err := rule.NameSymbol()
	fragmentMust(t, err)
	fragmentMust(t, rule.Close())
	if gotLabel != label {
		t.Fatal("query retry changed the source label")
	}
	fragmentMust(t, f.e.Close())
	fragmentMust(t, f.e.Close())
	var receipt memory.ConstructionFailure
	if errors.As(cause, &receipt) {
		fragmentMust(t, receipt.Cleanup())
		fragmentMust(t, receipt.Cleanup())
	}
	if cause.Error() != message || !errors.Is(cause, arena.ErrBounds) {
		t.Fatal("query retry changed the original native failure")
	}
	// The shared native fixture verifies zero budget after all caller roots
	// and services are closed, including the failed and successful contexts.
}
