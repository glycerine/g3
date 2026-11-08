package fp

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerExpressionQueryUsesPrequeryCoverOwner(t *testing.T) {
	for _, backend := range []string{"spacer"} {
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
			name, err := m.Symbols.Intern("expression-covered-integer")
			must(err)
			declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(declaration))
			zero, err := m.ArithmeticIntegerNumeralText("0")
			must(err)
			fact, err := m.MkApp(declaration, []ast.ASTHandle{zero})
			must(err)
			must(e.addRuleEager(fact, "expression-zero-fact"))
			parameters, err := e.params.NewValue()
			must(err)
			defer func() { must(e.params.Del(&parameters)) }()
			for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
				must(e.params.SetBoolName(&parameters, &name, false))
			}
			must(e.SetQueryParameters(&parameters))
			variable, err := m.MkVar(0, integer)
			must(err)
			property, err := m.MkEq(variable, zero)
			must(err)
			propertyRef, err := m.NewRef(property)
			must(err)
			defer func() { must(m.DelRef(&propertyRef)) }()
			query, err := m.MkApp(declaration, []ast.ASTHandle{variable})
			must(err)
			queryRef, err := m.NewRef(query)
			must(err)
			defer func() { must(m.DelRef(&queryRef)) }()
			must(e.AddCover(2, declaration, property))
			context := e.spacer
			status, err := e.QueryExpression(query)
			must(err)
			if status != lbool.True {
				t.Fatal("zero relation is unreachable", status)
			}
			if e.spacer != nil || e.queryClone == nil || e.queryClone.spacer != context {
				t.Fatal("expression query did not transfer the pre-query context owner")
			}
			cover, err := context.CoverDelta(2, e.queryClone.spacerPredicate(declaration))
			must(err)
			handle, err := m.RefGet(cover)
			must(err)
			if handle != property {
				t.Fatal("expression query lost the raw pre-query cover", handle, property)
			}
			must(m.DelRef(&cover))
			cover, err = e.CoverDelta(2, declaration)
			must(err)
			defer func() { must(m.DelRef(&cover)) }()
			handle, err = m.RefGet(cover)
			must(err)
			if handle != property {
				printer, err := ast.NewASTPPUtil(m)
				must(err)
				var text strings.Builder
				must(printer.DisplayAssertions(&text, []ast.ASTHandle{handle, property}, true))
				t.Fatal("public cover did not use the active query owner", text.String())
			}
			must(e.AddCover(3, declaration, property))
			levels, err := e.NumLevels(declaration)
			must(err)
			if levels != 4 || e.spacer != nil {
				t.Fatal("property access created a disconnected context", levels)
			}
			answer, err := e.ExpressionAnswer()
			must(err)
			defer func() { must(m.DelRef(&answer)) }()
			status, err = e.QueryExpression(query)
			must(err)
			if status != lbool.True {
				t.Fatal("repeated zero query", status)
			}
			if _, err := context.NumLevels(declaration); err == nil {
				t.Fatal("superseded expression context stayed open")
			}
			if _, err := m.RefGet(answer); err != nil {
				t.Fatal("query replacement invalidated the caller's owning answer", err)
			}
			if _, err := m.RefGet(cover); err != nil {
				t.Fatal("query replacement invalidated the caller's owning cover", err)
			}
			raw, err := e.queryClone.spacer.CoverDelta(3, e.queryClone.spacerPredicate(declaration))
			must(err)
			handle, err = m.RefGet(raw)
			must(err)
			if !m.IsTrue(handle) {
				t.Fatal("new synthetic query carried an old frame across reset")
			}
			must(m.DelRef(&raw))
		})
	}
}
