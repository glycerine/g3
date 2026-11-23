package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type enumTableFixture struct {
	tableTest
	element, x, color, red, blue, first, second, application ast.ASTHandle
}

func newEnumTableFixture(t testing.TB, f tableTest) enumTableFixture {
	t.Helper()
	m := f.m
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{
		Name: astTestSymbol(t, m, "Color"),
		Constructors: []ast.DatatypeConstructorDefinition{
			{Name: astTestSymbol(t, m, "red"), Recognizer: astTestSymbol(t, m, "is-red")},
			{Name: astTestSymbol(t, m, "blue"), Recognizer: astTestSymbol(t, m, "is-blue")},
		},
	}})
	astTestMust(t, err)
	constructors, err := m.DatatypeConstructors(sorts[0])
	astTestMust(t, err)
	g := enumTableFixture{tableTest: f, color: sorts[0]}
	g.red = f.term(m.MkApp(constructors[0], nil))
	g.blue = f.term(m.MkApp(constructors[1], nil))
	g.element = f.term(m.MkUninterpretedSort(astTestSymbol(t, m, "element"), nil))
	g.first = f.term(m.MkModelValue(0, g.element))
	g.second = f.term(m.MkModelValue(1, g.element))
	astTestMust(t, f.md.RegisterValue(g.first))
	astTestMust(t, f.md.RegisterValue(g.second))
	astTestMust(t, f.md.FreezeUniverse(g.element))
	colorOf := smtQuickPredicate(t, m, "color-of", g.element, g.color)
	tab, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, tab.SetElse(g.red))
	astTestMust(t, tab.InsertNewEntry([]ast.ASTHandle{g.second}, g.blue))
	astTestMust(t, f.md.Core().RegisterFunction(colorOf, tab))
	g.x = f.term(m.MkVar(0, g.element))
	g.application = f.term(m.MkApp(colorOf, []ast.ASTHandle{g.x}))
	return g
}

func TestModelTablesGroundEnumeration(t *testing.T) {
	m, _, md, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := newEnumTableFixture(t, tableTest{t, m, md})
	red := f.term(m.MkEq(f.application, f.red))
	blue := f.term(m.MkEq(f.application, f.blue))
	f.check(f.forall(red, f.element), lbool.True, f.second)
	f.check(f.forall(f.term(m.MkXor(red, blue)), f.element), lbool.False)

	// Exercise an enumeration in the argument position of an explicit row.
	boolean := f.term(m.MkBoolSort())
	isBlue := smtQuickPredicate(t, m, "blue-color", f.color, boolean)
	tab, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, tab.SetElse(f.term(m.MkFalse())))
	astTestMust(t, tab.InsertNewEntry([]ast.ASTHandle{f.blue}, f.term(m.MkTrue())))
	astTestMust(t, md.Core().RegisterFunction(isBlue, tab))
	pred := f.term(m.MkApp(isBlue, []ast.ASTHandle{f.application}))
	f.check(f.forall(f.term(m.MkIff(pred, blue)), f.element), lbool.False)
	// An enum binder needs a complete domain. An interpreted model value
	// has no constructor identity. Both go through the ordinary solver.
	y := f.term(m.MkVar(0, f.color))
	f.check(f.forall(f.term(m.MkEq(y, f.red)), f.color), lbool.Undef)
	placeholder := f.term(m.MkModelValue(0, f.color))
	f.check(f.forall(f.term(m.MkEq(f.application, placeholder)), f.element), lbool.Undef)

	// The datatype family also accepts raw declarations. A constructor
	// outside the definition of Color cannot supply another distinct color.
	family, err := m.DatatypeFamilyID()
	astTestMust(t, err)
	name, err := m.NewParameterSymbol(astTestSymbol(t, m, "third-color"))
	astTestMust(t, err)
	decl := f.term(m.MkFamilyFuncDecl(family, ast.ASTDatatypeConstructor, []ast.ASTParameter{name}, nil, f.color))
	astTestMust(t, m.DelParameter(&name))
	other := f.term(m.MkApp(decl, nil))
	f.check(f.forall(f.term(m.MkEq(f.application, other)), f.element), lbool.Undef)
}

func BenchmarkModelTableGroundEnumeration(b *testing.B) {
	for _, kind := range []string{"Tables", "AuxiliarySolver"} {
		b.Run(kind, func(b *testing.B) {
			m, c, md, _, _ := smtAuxiliaryQueryFixture(b, false)
			f := newEnumTableFixture(b, tableTest{b, m, md})
			isSecond := f.term(m.MkEq(f.x, f.second))
			isBlue := f.term(m.MkEq(f.application, f.blue))
			q := f.forall(f.term(m.MkIff(isSecond, isBlue)), f.element)
			astTestMust(b, c.initAuxiliaryContext())
			c.auxiliaryContext.config.CheckUninterpretedModelTables = kind == "Tables"
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				remaining := uint32(1)
				found, err := c.auxiliaryCounterexamples(md, q, &remaining)
				if err != nil || found || remaining != 1 {
					b.Fatal(found, remaining, err)
				}
			}
			b.StopTimer()
			checks := c.auxiliaryContext.kernel.Statistics().NumChecks
			if kind == "Tables" && checks != 0 || kind == "AuxiliarySolver" && checks == 0 {
				b.Fatal("unexpected auxiliary solver checks", checks)
			}
		})
	}
}
