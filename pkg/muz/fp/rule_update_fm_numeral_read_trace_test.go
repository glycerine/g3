//go:build g3trace

package fp

// Source: qe_lite.cpp:1734,1755 reads inequality constants and coefficients
// directly. The source does not reparse their existing decimal spellings.

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestFixedpointUpdateReadsWideInequalityCoefficientWithoutParsing(t *testing.T) {
	f := newRuleUpdateNumeralReadFixture(t)
	boolean := f.own(f.m.MkBoolSort())
	predicate := func(text string) ast.ASTHandle {
		h := f.own(f.m.MkFreshConst(text, boolean))
		d, err := f.m.GetDecl(h)
		fragmentMust(t, err)
		fragmentMust(t, f.e.RegisterPredicate(d))
		return h
	}
	p, h := predicate("read-inequality-P"), predicate("read-inequality-H")
	family, err := f.m.ArithmeticFamilyID()
	fragmentMust(t, err)
	real := f.own(f.m.MkFamilySort(family, 0, nil))
	x := f.own(f.m.MkVar(0, real))
	text := strings.Repeat("9", 600)
	wide := f.own(f.m.ArithmeticRealNumeralText(text))
	one := f.own(f.m.ArithmeticRealNumeralText("1"))
	product := f.own(f.m.MkArithmeticMul(wide, x))
	inequality := f.own(f.m.MkArithmeticLE(product, one))
	body := f.own(f.m.MkOr(inequality, p))
	implication := f.own(f.m.MkImplies(body, h))
	formula := f.own(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{real}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: implication}))
	fromP := f.own(f.m.MkImplies(p, h))
	name := symbol.Numerical(7)
	rawName, err := f.m.Symbols.Intern("read-inequality-pending")
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
		t.Fatal("updating an inequality rule reparsed its existing numeral coefficient", cause)
	}
	var receipt memory.ConstructionFailure
	if cause == nil || !strings.Contains(cause.Error(), "non-trivial body") || errors.Is(cause, numeral.ErrRationalLexicalPending) || errors.As(cause, &receipt) {
		t.Fatal("expected only the source two-clause inequality update rejection", cause)
	}
	forms, names, err := f.e.NamedRulesWithSymbols()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, closeASTRefs(f.m, forms)) }()
	if len(forms) != 3 || !reflect.DeepEqual(names, []symbol.Symbol{name, name, rawName}) {
		t.Fatal("wide inequality coefficient update lost its completed clauses or labels", len(forms), names)
	}
	// HNF pops P first. Eliminating the unbound real x makes WIDE*x <= 1
	// satisfiable, so its second Horn clause reduces to the fact H.
	for i, want := range []ast.ASTHandle{fromP, h, p} {
		got, err := f.m.RefGet(forms[i])
		fragmentMust(t, err)
		if got != want {
			t.Fatal("wide inequality update changed source clause order or meaning", i, got, want)
		}
	}
	if f.e.set != persistent || len(f.e.rawRules) != len(rawBefore) {
		t.Fatal("wide inequality update replaced the persistent set or raw archive")
	}
	for i, want := range rawBefore {
		want.first += 2
		want.end += 2
		if f.e.rawRules[i] != want {
			t.Fatal("wide inequality update changed its pending source input", f.e.rawRules[i], want)
		}
	}
	gotBody, err := f.m.GetQuantifierBody(formula)
	fragmentMust(t, err)
	gotText, _, matched, err := f.m.ArithmeticNumeralText(wide)
	fragmentMust(t, err)
	if gotBody != implication || !matched || gotText != text {
		t.Fatal("inequality update changed the borrowed source formula or numeral")
	}
}
