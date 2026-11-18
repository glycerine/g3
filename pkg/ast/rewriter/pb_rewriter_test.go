package rewriter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestPseudoBooleanValidationTranslationAndSMT2Dump(t *testing.T) {
	m := astTestManager(t)
	r, err := NewPseudoBooleanRewriter(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "pb-validation-a"), b)
	astTestMust(t, err)
	astTestPin(t, m, a)
	c, err := m.MkConst(astTestSymbol(t, m, "pb-validation-c"), b)
	astTestMust(t, err)
	astTestPin(t, m, c)
	original, err := m.MkPseudoBooleanAtLeast([]ast.ASTHandle{a, c}, 1)
	astTestMust(t, err)
	rewritten, err := m.MkOr(a, c)
	astTestMust(t, err)

	// A basic-family equality has an operator number in the PB range; family
	// recognition must still leave it unchanged.
	equality, err := m.MkEq(a, c)
	astTestMust(t, err)
	translated, err := r.TranslatePBToLIA(nil, equality)
	astTestMust(t, err)
	if translated != equality {
		t.Fatal("non-PB formula translated", translated, equality)
	}
	arithmetic, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "pb-validation-x"), integer)
	astTestMust(t, err)
	values := pbTestRationals(t, m, "2", "3")
	weighted, err := m.MkPseudoBooleanGE(values[:1], []ast.ASTHandle{a}, &values[1])
	astTestMust(t, err)
	linear, err := r.TranslatePBToLIA(map[ast.ASTHandle]ast.ASTHandle{a: x}, weighted)
	astTestMust(t, err)
	bound, err := m.GetArg(linear, 1)
	astTestMust(t, err)
	if text, isInteger, matched, e := m.ArithmeticNumeralText(bound); e != nil || !matched || isInteger || text != "3" {
		t.Fatal("PB bound must use the source Real numeral", text, isInteger, matched, e)
	}
	sum, err := m.GetArg(linear, 0)
	astTestMust(t, err)
	product, err := m.GetArg(sum, 0)
	astTestMust(t, err)
	coefficient, err := m.GetArg(product, 0)
	astTestMust(t, err)
	if text, isInteger, matched, e := m.ArithmeticNumeralText(coefficient); e != nil || !matched || isInteger || text != "2" {
		t.Fatal("PB coefficient must use the source Real numeral", text, isInteger, matched, e)
	}

	validation, err := r.MakeValidationFormula(original, rewritten)
	astTestMust(t, err)
	astTestPin(t, m, validation)
	kind, err := m.GetDeclKind(validation)
	astTestMust(t, err)
	count, err := m.GetNumArgs(validation)
	astTestMust(t, err)
	if kind != ast.ASTOpAnd || count != 5 {
		t.Fatal("validation conjunction", kind, count)
	}

	directory := t.TempDir()
	name, err := r.dumpPBRewriteAt(directory, validation)
	astTestMust(t, err)
	if filepath.Dir(name) != directory || !strings.HasPrefix(filepath.Base(name), "pb_rewrite_") || filepath.Ext(name) != ".smt2" {
		t.Fatal("source validation filename", name)
	}
	body, err := os.ReadFile(name)
	astTestMust(t, err)
	text := string(body)
	for _, fragment := range []string{"(declare-fun x0 () Int)", "(declare-fun x1 () Int)", "(assert", "(check-sat)"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("validation dump missing %q:\n%s", fragment, text)
		}
	}
}

func pbTestRationals(t *testing.T, m *ast.ASTManager, values ...string) []unumeral.OwnedRational {
	t.Helper()
	out := make([]unumeral.OwnedRational, len(values))
	for i, value := range values {
		var err error
		out[i], err = m.NumeralService().NewValue()
		astTestMust(t, err)
		astTestMust(t, m.NumeralService().SetString(&out[i], value))
	}
	t.Cleanup(func() {
		for i := range out {
			astTestMust(t, m.NumeralService().Del(&out[i]))
		}
	})
	return out
}

func pbTestRewrite(t *testing.T, r *PseudoBooleanRewriter, expression ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	m := r.manager
	decl, err := m.GetDecl(expression)
	astTestMust(t, err)
	count, err := m.GetNumArgs(expression)
	astTestMust(t, err)
	args := make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = m.GetArg(expression, uint32(i))
		astTestMust(t, err)
	}
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	status, err := r.RewriteApp(decl, ast.SliceArguments(args), out)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatal("PB rewrite status", status)
	}
	got, err := m.RefGet(out)
	astTestMust(t, err)
	astTestPin(t, m, got)
	return got
}

func TestPseudoBooleanRewriterCardinalityAndConstants(t *testing.T) {
	m := astTestManager(t)
	r, err := NewPseudoBooleanRewriter(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "pb-a"), b)
	astTestMust(t, err)
	astTestPin(t, m, a)
	c, err := m.MkConst(astTestSymbol(t, m, "pb-c"), b)
	astTestMust(t, err)
	astTestPin(t, m, c)

	atLeastOne, err := m.MkPseudoBooleanAtLeast([]ast.ASTHandle{a, c}, 1)
	astTestMust(t, err)
	want, err := m.MkOr(a, c)
	astTestMust(t, err)
	if got := pbTestRewrite(t, r, atLeastOne); got != want {
		t.Fatal("at-least one", got, want)
	}
	atLeastAll, err := m.MkPseudoBooleanAtLeast([]ast.ASTHandle{a, c}, 2)
	astTestMust(t, err)
	want, err = m.MkAnd(a, c)
	astTestMust(t, err)
	if got := pbTestRewrite(t, r, atLeastAll); got != want {
		t.Fatal("at-least all", got, want)
	}
	atMostOne, err := m.MkPseudoBooleanAtMost([]ast.ASTHandle{a, c}, 1)
	astTestMust(t, err)
	na, err := m.MkNot(a)
	astTestMust(t, err)
	nc, err := m.MkNot(c)
	astTestMust(t, err)
	want, err = m.MkOr(na, nc)
	astTestMust(t, err)
	if got := pbTestRewrite(t, r, atMostOne); got != want {
		t.Fatal("at-most one", got, want)
	}

	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	constant, err := m.MkPseudoBooleanAtLeast([]ast.ASTHandle{truth, falsehood}, 1)
	astTestMust(t, err)
	if got := pbTestRewrite(t, r, constant); got != truth {
		t.Fatal("constant reduction", got, truth)
	}
}

func TestPseudoBooleanRewriterUniqueNormalizeAndPrune(t *testing.T) {
	m := astTestManager(t)
	r, err := NewPseudoBooleanRewriter(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "pb-u-a"), b)
	astTestMust(t, err)
	astTestPin(t, m, a)
	c, err := m.MkConst(astTestSymbol(t, m, "pb-u-c"), b)
	astTestMust(t, err)
	astTestPin(t, m, c)
	na, err := m.MkNot(a)
	astTestMust(t, err)

	coefficients := pbTestRationals(t, m, "1/2", "1/2", "1")
	bound := pbTestRationals(t, m, "1")[0]
	constraint, err := m.MkPseudoBooleanGE(coefficients, []ast.ASTHandle{a, a, c}, &bound)
	astTestMust(t, err)
	want, err := m.MkOr(a, c)
	astTestMust(t, err)
	if got := pbTestRewrite(t, r, constraint); got != want {
		t.Fatal("coalesced rational coefficients", got, want)
	}

	coefficients = pbTestRationals(t, m, "2", "2", "1")
	bound = pbTestRationals(t, m, "4")[0]
	constraint, err = m.MkPseudoBooleanGE(coefficients, []ast.ASTHandle{a, na, c}, &bound)
	astTestMust(t, err)
	falseValue, err := m.MkFalse()
	astTestMust(t, err)
	if got := pbTestRewrite(t, r, constraint); got != falseValue {
		t.Fatal("literal cancellation and infeasibility", got, falseValue)
	}

	coefficients = pbTestRationals(t, m, "5", "5", "2", "2")
	bound = pbTestRationals(t, m, "5")[0]
	d, err := m.MkConst(astTestSymbol(t, m, "pb-u-d"), b)
	astTestMust(t, err)
	astTestPin(t, m, d)
	e, err := m.MkConst(astTestSymbol(t, m, "pb-u-e"), b)
	astTestMust(t, err)
	astTestPin(t, m, e)
	constraint, err = m.MkPseudoBooleanGE(coefficients, []ast.ASTHandle{a, c, d, e}, &bound)
	astTestMust(t, err)
	got := pbTestRewrite(t, r, constraint)
	want, err = m.MkOr(a, c)
	astTestMust(t, err)
	if got != want {
		t.Fatal("cutting-plane pruning", got, want)
	}
}

func TestPseudoBooleanRewriterAuxiliaryRefusalAndEquality(t *testing.T) {
	m := astTestManager(t)
	r, err := NewPseudoBooleanRewriter(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	aux, err := m.MkFreshPseudoBooleanAux()
	astTestMust(t, err)
	decl, err := m.GetDecl(aux)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	if status, err := r.RewriteApp(decl, ast.AppArguments{}, out); err != nil || status != RewriteFailed {
		t.Fatal("auxiliary refusal", status, err)
	}
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "pb-eq-a"), b)
	astTestMust(t, err)
	astTestPin(t, m, a)
	coefficients := pbTestRationals(t, m, "1")
	bound := pbTestRationals(t, m, "1")[0]
	equality, err := m.MkPseudoBooleanEQ(coefficients, []ast.ASTHandle{a}, &bound)
	astTestMust(t, err)
	got := pbTestRewrite(t, r, equality)
	if yes, err := m.IsPseudoBooleanEQ(got); err != nil || !yes {
		t.Fatal("equality result", yes, err)
	}
}

// mk_validate_rewrite's vars.insert(e, x) overwrites, so the last
// occurrence of a repeated atom names its integer variable.
func TestPseudoBooleanValidationLastOccurrenceWins(t *testing.T) {
	m := astTestManager(t)
	r, err := NewPseudoBooleanRewriter(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "pb-last-a"), b)
	astTestMust(t, err)
	astTestPin(t, m, a)
	original, err := m.MkPseudoBooleanAtLeast([]ast.ASTHandle{a, a}, 1)
	astTestMust(t, err)
	astTestPin(t, m, original)
	validation, err := r.MakeValidationFormula(original, a)
	astTestMust(t, err)
	astTestPin(t, m, validation)
	n, err := m.GetNumArgs(validation)
	astTestMust(t, err)
	different, err := m.GetArg(validation, n-1)
	astTestMust(t, err)
	equal, err := m.GetArg(different, 0)
	astTestMust(t, err)
	left, err := m.GetArg(equal, 0)
	astTestMust(t, err)
	// (>= (to_real (+ x1 x1)) 1.0)
	toReal, err := m.GetArg(left, 0)
	astTestMust(t, err)
	sum, err := m.GetArg(toReal, 0)
	astTestMust(t, err)
	arithmetic, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	x1, err := m.MkConst(astTestSymbol(t, m, "x1"), integer)
	astTestMust(t, err)
	for i := uint32(0); i < 2; i++ {
		if arg, e := m.GetArg(sum, i); e != nil || arg != x1 {
			t.Fatal("repeated atom must map to the last variable x1", i, arg, x1, e)
		}
	}
}
