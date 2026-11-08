//go:build g3trace

package fp

// Source: qe_lite.cpp eq::is_invertible_const reads a numeral value directly.
// Re-parsing its decimal spelling introduces an unrelated mpq parser lifetime.

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func newRuleUpdateNumeralReadFixture(t *testing.T) *contextDisplayCleanupFixture {
	t.Helper()
	f := &contextDisplayCleanupFixture{t: t}
	budget := arena.NewBudget(1 << 28)
	var err error
	f.runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	fragmentMust(t, err)
	f.numbers, err = numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: f.runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	fragmentMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	fragmentMust(t, err)
	parameters, err := uparams.NewParamsContext(symbols, f.numbers, uparams.ParamsConfig{Budget: budget})
	fragmentMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	fragmentMust(t, err)
	globals, err := uparams.NewGlobalParamsService(symbols, parameters, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	fragmentMust(t, err)
	fragmentMust(t, globals.InitializeSource())
	fragmentMust(t, globals.SourceSetName("fixedpoint.xform.fix_unbound_vars", "true"))
	f.m, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: f.numbers, Budget: budget})
	fragmentMust(t, err)
	fragmentMust(t, f.m.RegisterDeclarationPlugins())
	f.e, err = NewPropositionalEngineWithServices(f.m, parameters, globals)
	fragmentMust(t, err)
	t.Cleanup(func() {
		fragmentMust(t, f.e.Close())
		fragmentMust(t, closeASTRefs(f.m, f.roots))
		fragmentMust(t, f.m.Close())
		fragmentMust(t, globals.Close())
		fragmentMust(t, catalog.Close())
		fragmentMust(t, parameters.Close())
		fragmentMust(t, f.numbers.Close())
		fragmentMust(t, symbols.Close())
		fragmentMust(t, f.runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("numeral-read update leaked native owners", budget.Used())
		}
	})
	return f
}

type ruleUpdateNumeralParserWriter struct {
	t                 *testing.T
	runtime           *memory.Runtime
	coefficient       string
	seen, interrupted bool
	lane              memory.Block
}

func (w *ruleUpdateNumeralParserWriter) Write(p []byte) (int, error) {
	if string(p) == w.coefficient {
		w.seen = true
	}
	if !w.seen || w.interrupted || string(p) != "------------------------------------------------\n" {
		return len(p), nil
	}
	w.interrupted = true
	// Match the real parser-scoped deletion refusal in numeral's runtime
	// tests: public allocations move TLS accounting to its lower boundary.
	// The following parser destructor, rather than the writer, returns failure.
	first, err := w.runtime.Allocate(8)
	fragmentMust(w.t, err)
	second, err := w.runtime.Allocate(8)
	fragmentMust(w.t, err)
	delta := int64(-1000000)
	if err = w.runtime.Reallocate(&first, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		w.t.Fatal("initial parser accounting boundary", err)
	}
	fragmentMust(w.t, w.runtime.Deallocate(&first))
	state := w.runtime.State()
	target := int64(math.MinInt64) + 1000 - state.GlobalSize + 16
	if err = w.runtime.Reallocate(&second, uint64(target)+16-8); !errors.Is(err, arena.ErrBounds) {
		w.t.Fatal("parser accounting minimum", err)
	}
	fragmentMust(w.t, w.runtime.Deallocate(&second))
	w.lane, err = w.runtime.Allocate(8)
	fragmentMust(w.t, err)
	delta = int64(-100001) - w.runtime.State().LocalSize
	if err = w.runtime.Reallocate(&w.lane, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		w.t.Fatal("parser destructor accounting boundary", err)
	}
	return len(p), nil
}

func (w *ruleUpdateNumeralParserWriter) recover(f *contextDisplayCleanupFixture, cause error) {
	w.t.Helper()
	if !w.interrupted {
		return
	}
	// Keep the regression clean on the old implementation: recover the real
	// pending parser owner before reporting the unexpected parser invocation.
	recovery, err := w.runtime.Allocate(200000)
	fragmentMust(w.t, err)
	fragmentMust(w.t, f.numbers.Manager.CleanupParserTemporaries())
	var receipt memory.ConstructionFailure
	if errors.As(cause, &receipt) {
		fragmentMust(w.t, receipt.Cleanup())
		fragmentMust(w.t, receipt.Cleanup())
	}
	fragmentMust(w.t, w.runtime.Deallocate(&w.lane))
	state := w.runtime.State()
	delta := int64(-500000) - state.GlobalSize - state.LocalSize
	if err = w.runtime.Reallocate(&recovery, uint64(200000)+uint64(delta)); err == nil || w.runtime.State().GlobalSize != -500000 || w.runtime.State().LocalSize != 0 {
		w.t.Fatal("restore parser accounting range", err, w.runtime.State())
	}
	fragmentMust(w.t, w.runtime.Deallocate(&recovery))
}

func TestFixedpointUpdateReadsWideCoefficientWithoutParsing(t *testing.T) {
	f := newRuleUpdateNumeralReadFixture(t)
	boolean := f.own(f.m.MkBoolSort())
	predicate := func(text string) ast.ASTHandle {
		h := f.own(f.m.MkFreshConst(text, boolean))
		d, err := f.m.GetDecl(h)
		fragmentMust(t, err)
		fragmentMust(t, f.e.RegisterPredicate(d))
		return h
	}
	p, h := predicate("read-P"), predicate("read-H")
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
	rawName, err := f.m.Symbols.Intern("read-pending")
	fragmentMust(t, err)
	fragmentMust(t, f.e.addRuleWithSymbolAndBoundEager(p, rawName, 17))
	rawBefore := append([]rawRuleFormula(nil), f.e.rawRules...)
	persistent := f.e.set
	writer := &ruleUpdateNumeralParserWriter{t: t, runtime: f.runtime, coefficient: text}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("mpq_set")
	t.Cleanup(tags.Finalize)
	fragmentMust(t, f.numbers.Manager.BindTrace(tags))
	cause := f.e.UpdateRuleWithSymbol(formula, name)
	writer.recover(f, cause)
	fragmentMust(t, f.numbers.Manager.BindTrace(nil))
	if writer.seen || writer.interrupted {
		t.Fatal("updating a rule reparsed its existing numeral coefficient", cause)
	}
	var receipt memory.ConstructionFailure
	if cause == nil || !strings.Contains(cause.Error(), "non-trivial body") || errors.Is(cause, numeral.ErrRationalLexicalPending) || errors.As(cause, &receipt) {
		t.Fatal("expected only the source two-clause update rejection", cause)
	}
	forms, names, err := f.e.NamedRulesWithSymbols()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, closeASTRefs(f.m, forms)) }()
	if len(forms) != 3 || !reflect.DeepEqual(names, []symbol.Symbol{name, name, rawName}) {
		t.Fatal("wide coefficient update lost its completed clauses or labels", len(forms), names)
	}
	for i, want := range []ast.ASTHandle{fromP, h, p} {
		got, err := f.m.RefGet(forms[i])
		fragmentMust(t, err)
		if got != want {
			t.Fatal("wide coefficient update changed source clause order or meaning", i, got, want)
		}
	}
	if f.e.set != persistent || len(f.e.rawRules) != len(rawBefore) {
		t.Fatal("wide coefficient update replaced the persistent set or raw archive")
	}
	for i, want := range rawBefore {
		want.first += 2
		want.end += 2
		if f.e.rawRules[i] != want {
			t.Fatal("wide coefficient update changed its pending source input", f.e.rawRules[i], want)
		}
	}
	gotBody, err := f.m.GetQuantifierBody(formula)
	fragmentMust(t, err)
	gotText, _, matched, err := f.m.ArithmeticNumeralText(wide)
	fragmentMust(t, err)
	if gotBody != implication || !matched || gotText != text {
		t.Fatal("update changed the borrowed source formula or numeral")
	}
}
