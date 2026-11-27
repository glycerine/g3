package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestModelTableIntegerOperations(t *testing.T) {
	m, _, md, _, _ := smtAuxiliaryQueryFixture(t, false)
	f := tableTest{t, m, md}
	boolean := f.term(m.MkBoolSort())
	x := f.term(m.MkVar(0, boolean))
	no, yes := f.term(m.MkFalse()), f.term(m.MkTrue())
	// Six Boolean binders give 64 assignments, the minimum domain product
	// for integer-valued tables. The last declaration binds variable zero.
	sorts := []ast.ASTHandle{boolean, boolean, boolean, boolean, boolean, boolean}
	witness := func(last ast.ASTHandle) []ast.ASTHandle {
		return []ast.ASTHandle{no, no, no, no, no, last}
	}
	num := func(s string) ast.ASTHandle { return f.term(m.ArithmeticIntegerNumeralText(s)) }
	minusTwo, one, two, three := num("-2"), num("1"), num("2"), num("3")
	// On the two possible bindings, v is 1 or 3. Every counterexample
	// below is also checked by ordinary instantiation and model evaluation.
	v := f.term(m.MkIte(x, three, one))
	sum := f.term(m.MkArithmeticAdd(v, two))
	diff := f.term(m.MkArithmeticSub(sum, three))
	product := f.term(m.MkArithmeticMul(diff, minusTwo))
	neg := f.term(m.MkArithmeticNeg(product))
	for _, tc := range []struct {
		name    string
		formula ast.ASTHandle
		witness []ast.ASTHandle
	}{
		{"computed equality", f.term(m.MkEq(sum, three)), witness(yes)},
		{"less", f.term(m.MkArithmeticLT(neg, three)), witness(yes)},
		{"less equal", f.term(m.MkArithmeticLE(diff, two)), nil},
		{"greater", f.term(m.MkArithmeticGT(sum, three)), witness(no)},
		{"greater equal", f.term(m.MkArithmeticGE(product, num("-4"))), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tableTest{t, m, md}
			status := lbool.False
			if len(tc.witness) != 0 {
				status = lbool.True
			}
			f.check(f.forall(tc.formula, sorts...), status, tc.witness...)
		})
	}
	// A computed key and the same numeral in a function interpretation
	// must select the same entry. The fallback covers the other binding.
	integer := f.term(m.GetSort(one))
	fn := f.term(m.MkFuncDecl(astTestSymbol(t, m, "is-three"), []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
	tab, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterFunction(fn, tab))
	astTestMust(t, tab.SetElse(no))
	astTestMust(t, tab.InsertNewEntry([]ast.ASTHandle{three}, yes))
	lookup := f.term(m.MkApp(fn, []ast.ASTHandle{sum}))
	f.check(f.forall(lookup, sorts...), lbool.True, witness(yes)...)
	f.check(f.forall(lookup, sorts[:5]...), lbool.Undef)

	// Exceeding the supported range at any intermediate node delegates
	// the whole check, even when the final result would fit.
	maximum, minimum := num("2147483647"), num("-2147483648")
	overflow := f.term(m.MkArithmeticAdd(maximum, one))
	for _, term := range []ast.ASTHandle{
		overflow, f.term(m.MkArithmeticSub(overflow, one)),
		f.term(m.MkArithmeticNeg(minimum)), f.term(m.MkArithmeticMul(maximum, maximum)),
		num("2147483648"), f.term(m.MkArithmeticDiv(three, two)),
	} {
		f.check(f.forall(f.term(m.MkEq(term, v)), sorts...), lbool.Undef)
	}
}
