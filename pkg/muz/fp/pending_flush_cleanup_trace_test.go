//go:build g3trace

package fp

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source: dl_context.cpp flush_add_rules publishes a clause before processing
// the next one. The second clause reaches QE's numeric cleanup boundary.
func TestPendingFlushRecoversReducerAfterNativeCleanupFailure(t *testing.T) {
	f := newRuleUpdateNumeralReadFixture(t)
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", "false"))
	boolean := f.own(f.m.MkBoolSort())
	predicate := func(name string) ast.ASTHandle {
		h := f.own(f.m.MkFreshConst(name, boolean))
		d, err := f.m.GetDecl(h)
		fragmentMust(t, err)
		fragmentMust(t, f.e.RegisterPredicate(d))
		return h
	}
	p, head := predicate("pending-cleanup-P"), predicate("pending-cleanup-H")
	family, err := f.m.ArithmeticFamilyID()
	fragmentMust(t, err)
	real := f.own(f.m.MkFamilySort(family, 0, nil))
	x := f.own(f.m.MkVar(0, real))
	wide := f.own(f.m.ArithmeticRealNumeralText(strings.Repeat("9", 600)))
	one := f.own(f.m.ArithmeticRealNumeralText("1"))
	product := f.own(f.m.MkArithmeticMul(wide, x))
	equality := f.own(f.m.MkEq(product, one))
	body := f.own(f.m.MkOr(equality, p))
	implication := f.own(f.m.MkImplies(body, head))
	formula := f.own(f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{real},
		Names: []symbol.Symbol{symbol.Numerical(0)}, Body: implication,
	}))
	fragmentMust(t, f.e.AddRule(formula, "pending-cleanup"))
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", "true"))

	writer := &ruleUpdateDERCleanupWriter{boundary: ruleUpdateNumeralParserWriter{t: t, runtime: f.runtime}}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("qe_lite")
	t.Cleanup(tags.Finalize)
	cooperation, err := cooperate.NewCooperation(f.m.Budget())
	fragmentMust(t, err)
	lite, err := qe.NewLite(f.m, nil, false, qe.Config{Context: f.e.params, Globals: f.e.globals, Cooperation: cooperation, Trace: tags})
	fragmentMust(t, err)
	f.e.unboundLite, f.e.unboundCooperation = lite, cooperation

	cause := f.e.flushPendingRules()
	writer.restoreAccounting(t, f.runtime)
	var receipt memory.ConstructionFailure
	if !writer.matched || !writer.boundary.interrupted || !errors.Is(cause, arena.ErrBounds) || !errors.As(cause, &receipt) {
		t.Fatalf("flush missed real QE cleanup failure: %v, matched=%t interrupted=%t", cause, writer.matched, writer.boundary.interrupted)
	}
	defer func() { fragmentMust(t, receipt.Cleanup()) }()
	count, err := f.e.set.Len()
	fragmentMust(t, err)
	if count != 1 || len(f.e.rawRules) != 1 || !f.e.rawRules[0].pending || !f.e.unboundLiteDirty {
		t.Fatalf("after failure: %d clauses, raw=%+v dirty=%t", count, f.e.rawRules, f.e.unboundLiteDirty)
	}
	first, err := f.e.set.Rule(0)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, first.Close()) }()
	message := cause.Error()
	// Retry through the engine without manually draining its QE owner; the
	// error receipt remains safe to clean up after the successful retry.
	fragmentMust(t, f.e.flushPendingRules())
	count, err = f.e.set.Len()
	fragmentMust(t, err)
	if count != 3 || f.e.rawRules[0].pending || f.e.unboundLite == lite || f.e.unboundLiteDirty {
		t.Fatalf("after retry: %d clauses, raw=%+v dirty=%t", count, f.e.rawRules, f.e.unboundLiteDirty)
	}
	published, err := f.e.set.Rule(0)
	fragmentMust(t, err)
	same, err := first.Same(published)
	fragmentMust(t, err)
	fragmentMust(t, published.Close())
	if !same {
		t.Fatal("retry replaced the published prefix")
	}
	fragmentMust(t, receipt.Cleanup())
	fragmentMust(t, receipt.Cleanup())
	if cause.Error() != message || !errors.Is(cause, arena.ErrBounds) {
		t.Fatal("cleanup changed the original failure", cause)
	}
}
