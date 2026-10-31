package tactic

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"testing"
)

func TestEq2BVModelConverterRestoresIntegerAndCopiesBindings(t *testing.T) {
	m := astTestManager(t)
	family, e := m.RegisterArithmeticPlugin()
	satOK(t, e)
	intSort, e := m.MkFamilySort(family, 1, nil)
	satOK(t, e)
	_, e = m.RegisterBitVectorPlugin()
	satOK(t, e)
	bvSort, e := m.BitVectorSort(4)
	satOK(t, e)
	original, e := m.MkConst(astTestSymbol(t, m, "original"), intSort)
	satOK(t, e)
	originalDecl, e := m.GetDecl(original)
	satOK(t, e)
	fresh, e := m.MkConst(astTestSymbol(t, m, "fresh"), bvSort)
	satOK(t, e)
	freshDecl, e := m.GetDecl(fresh)
	satOK(t, e)
	other := modelTestDecl(t, m, "untouched")
	val, e := m.BitVectorNumeralUnsigned(13, 4)
	satOK(t, e)
	yes, e := m.MkTrue()
	satOK(t, e)
	old, e := model.NewModel(m)
	satOK(t, e)
	satOK(t, old.Core().RegisterConstant(freshDecl, val))
	satOK(t, old.Core().RegisterConstant(other, yes))
	var ref ModelRef
	satOK(t, ref.Set(old.Core()))
	t.Cleanup(func() { satOK(t, ref.Close()) })
	converter, e := NewEq2BVModelConverter(m)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, converter.Close()) })
	satOK(t, converter.Insert(freshDecl, originalDecl))
	satOK(t, converter.ApplyUnindexed(&ref))
	converted, e := ref.Get()
	satOK(t, e)
	restored, e := converted.ConstantInterpretation(originalDecl)
	satOK(t, e)
	text, _, found, e := m.ArithmeticNumeralText(restored)
	satOK(t, e)
	if !found || text != "13" {
		t.Fatalf("restored integer %q", text)
	}
	lost, e := converted.ConstantInterpretation(freshDecl)
	satOK(t, e)
	if !lost.IsNull() {
		t.Fatal("fresh binding was not removed")
	}
	kept, e := converted.ConstantInterpretation(other)
	satOK(t, e)
	if kept != yes {
		t.Fatal("unrelated binding changed")
	}
}

func TestEq2BVModelConverterTranslatesDeclarations(t *testing.T) {
	from := astTestManager(t)
	to, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: from.Symbols, Numerals: from.NumeralService(), Budget: from.Budget()})
	satOK(t, e)
	t.Cleanup(func() { satOK(t, to.Close()) })
	family, e := from.RegisterArithmeticPlugin()
	satOK(t, e)
	intSort, e := from.MkFamilySort(family, 1, nil)
	satOK(t, e)
	_, e = from.RegisterBitVectorPlugin()
	satOK(t, e)
	bvSort, e := from.BitVectorSort(2)
	satOK(t, e)
	x, e := from.MkConst(astTestSymbol(t, from, "translated-x"), intSort)
	satOK(t, e)
	a, e := from.GetDecl(x)
	satOK(t, e)
	y, e := from.MkConst(astTestSymbol(t, from, "translated-y"), bvSort)
	satOK(t, e)
	b, e := from.GetDecl(y)
	satOK(t, e)
	c, e := NewEq2BVModelConverter(from)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, c.Close()) })
	satOK(t, c.Insert(b, a))
	tr, e := ast.NewASTTranslation(from, to, true)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	copied, e := c.Translate(tr)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, copied.Close()) })
	actual := copied.(*Eq2BVModelConverter)
	wantFresh, e := tr.Translate(b)
	satOK(t, e)
	wantOriginal, e := tr.Translate(a)
	satOK(t, e)
	if len(actual.pairs) != 1 || actual.pairs[0] != (eq2bvModelPair{wantFresh, wantOriginal}) {
		t.Fatal("incorrect translated mappings")
	}
}
