package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp add_cover/check_reset/query and
// pdr_context.cpp context::update_rules.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRPrequeryCoverRejectsEnabledSlicing(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(f.engine.SelectEngine(backend))
			falsity, err := f.manager.MkFalse()
			must(err)
			err = f.engine.AddCover(2, f.declarations["u"], falsity)
			if err == nil || !strings.Contains(err.Error(), "Covers are incompatible with slicing. Disable slicing before using covers") {
				t.Fatal("pre-query cover did not enforce source slicing restriction", err)
			}
			levels, err := f.engine.NumLevels(f.declarations["u"])
			must(err)
			if levels != 0 {
				t.Fatal("rejected cover created predicate frames", levels)
			}
		})
	}
}

func TestPDRCoverBeforeFirstQuerySurvivesRuleInstallation(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(e.SelectEngine(backend))
			arithmetic, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(arithmetic, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("prequery-covered-integer")
			must(err)
			query, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(query))
			zero, err := m.ArithmeticIntegerNumeralText("0")
			must(err)
			fact, err := m.MkApp(query, []ast.ASTHandle{zero})
			must(err)
			must(e.AddRule(fact, "covered-zero-fact"))
			parameters, err := e.params.NewValue()
			must(err)
			defer func() { must(e.params.Del(&parameters)) }()
			for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
				must(e.params.SetBoolName(&parameters, &name, false))
			}
			must(e.SetQueryParameters(&parameters))
			levels, err := e.NumLevels(query)
			must(err)
			if levels != 0 {
				t.Fatal("unknown predicate has frames", levels)
			}
			cover, err := e.CoverDelta(2, query)
			must(err)
			handle, err := m.RefGet(cover)
			must(err)
			if !m.IsTrue(handle) {
				t.Fatal("unknown predicate cover is not true")
			}
			must(m.DelRef(&cover))
			variable, err := m.MkVar(0, integer)
			must(err)
			marker, err := m.MkEq(variable, zero)
			must(err)
			markerRef, err := m.NewRef(marker)
			must(err)
			defer func() { must(m.DelRef(&markerRef)) }()
			must(e.AddCover(2, query, marker))
			context := e.pdr
			assertCover := func() {
				t.Helper()
				cover, err := e.CoverDelta(2, query)
				must(err)
				defer func() { must(m.DelRef(&cover)) }()
				handle, err := m.RefGet(cover)
				must(err)
				if handle != marker {
					t.Fatal("pre-query cover changed", handle, marker)
				}
			}
			assertCover()
			status, err := e.QueryPredicate(query)
			must(err)
			if status != lbool.True {
				t.Fatal("zero fact must be reachable", status)
			}
			if e.pdr != context {
				t.Fatal("initial query replaced the context containing its cover")
			}
			transformer, found, err := e.pdr.Transformer(e.pdrPredicate(query))
			must(err)
			if !found {
				t.Fatal("initial query lost the covered predicate")
			}
			raw, err := transformer.CoverDelta(2)
			must(err)
			rawHandle, err := m.RefGet(raw)
			must(err)
			if rawHandle != marker {
				t.Fatal("initial query lost the supplied raw frame", rawHandle, marker)
			}
			must(m.DelRef(&raw))
			levels, err = e.NumLevels(query)
			must(err)
			if levels != 3 {
				t.Fatal("initial query lost the supplied frame level", levels)
			}
			assertCover()
			one, err := m.ArithmeticIntegerNumeralText("1")
			must(err)
			fact, err = m.MkApp(query, []ast.ASTHandle{one})
			must(err)
			must(e.AddRule(fact, "cover-invalidating-one-fact"))
			status, err = e.QueryPredicate(query)
			must(err)
			if status != lbool.True {
				t.Fatal("new fact is not reachable", status)
			}
			raw, err = e.pdr.CoverDelta(2, e.pdrPredicate(query))
			must(err)
			rawHandle, err = m.RefGet(raw)
			must(err)
			if !m.IsTrue(rawHandle) {
				t.Fatal("new fact did not discard the old raw cover", rawHandle)
			}
			must(m.DelRef(&raw))
		})
	}
}
