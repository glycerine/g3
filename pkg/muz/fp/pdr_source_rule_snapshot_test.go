package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRCoveredAdditionUsesOriginalRuleSnapshot(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	query := f.declarations["q"]
	status, err := e.QueryPredicate(query)
	must(err)
	if status != lbool.True {
		t.Fatal("initial query", status)
	}
	context, source := e.pdr, e.pdrSourceRules
	count, err := source.Len()
	must(err)
	if count != 2 {
		t.Fatal("snapshot does not own both original rules", count)
	}
	body, err := m.MkAnd(f.applications["p"], f.applications["u"])
	must(err)
	edge, err := m.MkImplies(body, f.applications["q"])
	must(err)
	must(e.AddRule(edge, "covered-added-edge"))
	status, err = e.QueryPredicate(query)
	must(err)
	if status != lbool.True || e.pdr != context {
		t.Fatal("covered addition did not preserve the PDR context", status)
	}
	if _, err := source.Len(); err == nil {
		t.Fatal("superseded source snapshot remained open")
	}
	source = e.pdrSourceRules
	count, err = source.Len()
	must(err)
	if count != 3 {
		t.Fatal("source snapshot lost the added original rule", count)
	}
	must(e.AddRule(f.applications["u"], "uncovered-added-fact"))
	status, err = e.QueryPredicate(query)
	must(err)
	if status != lbool.True || e.pdr == context {
		t.Fatal("uncovered addition reused old PDR state", status)
	}
	if _, err := source.Len(); err == nil {
		t.Fatal("replaced source snapshot remained open")
	}
	source = e.pdrSourceRules
	must(e.Close())
	if e.pdrSourceRules != nil {
		t.Fatal("source snapshot survived engine close")
	}
	if _, err := source.Len(); err == nil {
		t.Fatal("closed engine left source snapshot open")
	}
}

func TestPDRCoveredArithmeticAdditionPreservesRawCover(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	arithmetic, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("covered-added-integer")
	must(err)
	query, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(query))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	fact, err := m.MkApp(query, []ast.ASTHandle{zero})
	must(err)
	must(e.AddRule(fact, "zero-fact"))
	parameters, err := e.params.NewValue()
	must(err)
	defer func() { must(e.params.Del(&parameters)) }()
	for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
		must(e.params.SetBoolName(&parameters, &name, false))
	}
	must(e.SetQueryParameters(&parameters))
	status, err := e.QueryPredicate(query)
	must(err)
	if status != lbool.True {
		t.Fatal("zero fact is unreachable", status)
	}
	context := e.pdr
	variable, err := m.MkVar(0, integer)
	must(err)
	property, err := m.MkEq(variable, zero)
	must(err)
	propertyRef, err := m.NewRef(property)
	must(err)
	defer func() { must(m.DelRef(&propertyRef)) }()
	must(e.AddCover(2, query, property))
	edge, err := m.MkImplies(f.applications["p"], fact)
	must(err)
	must(e.AddRule(edge, "covered-zero-rule"))
	proofsBefore, err := m.ProofsEnabled()
	must(err)
	status, err = e.QueryPredicate(query)
	must(err)
	proofsAfter, err := m.ProofsEnabled()
	must(err)
	if proofsBefore != proofsAfter {
		t.Fatal("covered arithmetic addition changed the caller's proof mode")
	}
	if status != lbool.True || e.pdr != context {
		t.Fatal("covered arithmetic addition lost its context", status)
	}
	raw, err := e.pdr.CoverDelta(2, e.pdrPredicate(query))
	must(err)
	defer func() { must(m.DelRef(&raw)) }()
	handle, err := m.RefGet(raw)
	must(err)
	if handle != property {
		t.Fatal("covered addition lost the raw frame property", handle, property)
	}
}

func TestPDRCoveredAdditionDoesNotHideBackgroundRevision(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	query := f.declarations["q"]
	status, err := e.QueryPredicate(query)
	must(err)
	if status != lbool.True {
		t.Fatal("initial query", status)
	}
	prior := e.pdr
	falsity, err := m.MkFalse()
	must(err)
	must(e.SetBackground(falsity))
	body, err := m.MkAnd(f.applications["p"], f.applications["u"])
	must(err)
	edge, err := m.MkImplies(body, f.applications["q"])
	must(err)
	must(e.AddRule(edge, "covered-addition-after-background"))
	status, err = e.QueryPredicate(query)
	must(err)
	if status != lbool.False || e.pdr == prior {
		t.Fatal("covered addition hid a background revision", status)
	}
}
