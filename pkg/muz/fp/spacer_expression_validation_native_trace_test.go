//go:build g3trace

package fp

// Source: spacer_context.cpp context::solve calls validate after finding an
// inductive model; model_evaluator.cpp:566 traces each validation evaluation.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

type spacerValidationNativeWriter struct {
	native                    ruleUpdateDERCleanupWriter
	context                   *spacer.Context
	digits                    string
	operator, site, wide, hit bool
}

func (w *spacerValidationNativeWriter) Write(p []byte) (int, error) {
	if string(p) == "operator()" {
		w.operator, w.site, w.wide = true, false, false
	}
	if w.operator && string(p) == "566" {
		w.site = true
	}
	if strings.Contains(string(p), w.digits) {
		w.wide = true
	}
	if w.site && w.wide && !w.hit && string(p) == "------------------------------------------------\n" {
		// Solve publishes its internal false result immediately before
		// validation. Earlier search evaluations must not arm this boundary.
		status, err := w.context.LastResult()
		fragmentMust(w.native.boundary.t, err)
		if status == lbool.False {
			w.hit = true
			w.native.boundary.seen = true
			return w.native.boundary.Write(p)
		}
	}
	return len(p), nil
}

func TestSpacerExpressionValidationNativeFailurePreservesTransferredOwner(t *testing.T) {
	f := newRuleUpdateNumeralReadFixture(t)
	for _, key := range []string{
		"fixedpoint.xform.fix_unbound_vars", "fixedpoint.xform.slice",
		"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear",
	} {
		fragmentMust(t, f.e.globals.SourceSetName(key, "false"))
	}
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.pdr.validate_result", "true"))
	fragmentMust(t, f.e.SelectEngine("spacer"))
	boolean := f.own(f.m.MkBoolSort())
	family, err := f.m.ArithmeticFamilyID()
	fragmentMust(t, err)
	integer := f.own(f.m.MkFamilySort(family, 1, nil))
	label, err := f.m.Symbols.Intern("validation-native-chain")
	fragmentMust(t, err)
	declaration := f.own(f.m.MkFuncDecl(label, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(t, f.e.RegisterPredicate(declaration))
	zero := f.own(f.m.ArithmeticIntegerNumeralText("0"))
	one := f.own(f.m.ArithmeticIntegerNumeralText("1"))
	digits := strings.Repeat("9", 600)
	negative := f.own(f.m.ArithmeticIntegerNumeralText("-" + digits))
	fact := f.own(f.m.MkApp(declaration, []ast.ASTHandle{zero}))
	fragmentMust(t, f.e.addRuleEager(fact, "zero"))
	variable := f.own(f.m.MkVar(0, integer))
	current := f.own(f.m.MkApp(declaration, []ast.ASTHandle{variable}))
	nextValue := f.own(f.m.MkArithmeticAdd(variable, one))
	next := f.own(f.m.MkApp(declaration, []ast.ASTHandle{nextValue}))
	edge := f.own(f.m.MkImplies(current, next))
	fragmentMust(t, f.e.addRuleEager(edge, "successor"))
	query := f.own(f.m.MkApp(declaration, []ast.ASTHandle{negative}))
	_, err = f.e.NumLevels(declaration)
	fragmentMust(t, err)
	prequery := f.e.spacer
	if prequery == nil {
		t.Fatal("fixture did not construct the prequery Spacer owner")
	}
	writer := &spacerValidationNativeWriter{
		native:  ruleUpdateDERCleanupWriter{boundary: ruleUpdateNumeralParserWriter{t: t, runtime: f.runtime}},
		context: prequery, digits: digits,
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
		status, cause = f.e.QueryExpression(query)
	}()
	// Only repair native accounting; the public query owns all validation
	// temporaries and the context that produced the failed result.
	writer.native.restoreAccounting(t, f.runtime)
	if !writer.hit || !writer.native.boundary.interrupted || status != lbool.False || !errors.Is(cause, arena.ErrBounds) {
		t.Fatal("query missed the native post-solve validation failure", status, cause, writer.hit)
	}
	message := cause.Error()
	failedClone := f.e.queryClone
	if failedClone == nil || failedClone.spacer != prequery || failedClone.closed || f.e.spacer != nil {
		t.Fatal("validation failure lost the transferred context")
	}
	if _, err = f.e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("validation failure published a successful result", err)
	}
	answer, answerErr := f.e.ExpressionAnswer()
	if answer != (ast.ASTRef{}) {
		fragmentMust(t, f.m.DelRef(&answer))
	}
	if !errors.Is(answerErr, ErrPropositionalEngine) {
		t.Fatal("validation failure published an answer", answerErr)
	}
	status, err = f.e.QueryExpression(query)
	fragmentMust(t, err)
	if status != lbool.False || f.e.queryClone == nil || f.e.queryClone == failedClone || !failedClone.closed {
		t.Fatal("query retry failed to replace the prior owner", status)
	}
	if _, err = prequery.Initialized(); err == nil {
		t.Fatal("query retry left the transferred predecessor open")
	}
	last, err := f.e.LastResult()
	fragmentMust(t, err)
	if last != lbool.False {
		t.Fatal("successful retry did not publish the unreachable result", last)
	}
	fragmentMust(t, f.e.clearQueryResources())
	fragmentMust(t, f.e.clearQueryResources())
	fragmentMust(t, f.e.Close())
	fragmentMust(t, f.e.Close())
	var receipt memory.ConstructionFailure
	if errors.As(cause, &receipt) {
		fragmentMust(t, receipt.Cleanup())
		fragmentMust(t, receipt.Cleanup())
	}
	if cause.Error() != message || !errors.Is(cause, arena.ErrBounds) {
		t.Fatal("query retry changed the original validation failure")
	}
	// The shared native fixture closes all caller roots and services and
	// checks that the final allocation budget is zero.
}
