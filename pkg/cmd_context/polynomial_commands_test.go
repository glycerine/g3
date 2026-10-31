package cmd_context_test

import (
	"bytes"
	"errors"
	"io"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestWritePolynomialDisplaysSourceDenominator(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	m, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	half, err := m.ArithmeticNumeralRat(big.NewRat(1, 2), false)
	if err != nil {
		t.Fatal(err)
	}
	sort, err := m.GetSort(half)
	if err != nil {
		t.Fatal(err)
	}
	name, err := m.Symbols.Intern("x")
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	plus, err := m.MkArithmeticAdd(x, half)
	if err != nil {
		t.Fatal(err)
	}
	minus, err := m.MkArithmeticSub(x, half)
	if err != nil {
		t.Fatal(err)
	}
	expression, err := m.MkArithmeticMul(plus, minus)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = context.WritePolynomial(&output, expression); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "(* 4 (+ (* 4.0 (^ x 2.0)) (- 1.0)))\n"; got != want {
		t.Fatalf("polynomial output = %q, want %q", got, want)
	}
	output.Reset()
	if err = context.WritePolynomialFactors(&output, expression, polynomial.DefaultUPolynomialFactorParams()); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "(factors\n1/4\n(+ (* 2.0 x) (- 1.0))\n(+ (* 2.0 x) 1.0))\n"; got != want {
		t.Fatalf("factor output = %q, want %q", got, want)
	}
}

func TestWritePolynomialRootsWriterFailureAndRetry(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	m, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	two, err := m.ArithmeticIntegerNumeralText("2")
	if err != nil {
		t.Fatal(err)
	}
	sort, err := m.GetSort(two)
	if err != nil {
		t.Fatal(err)
	}
	name, err := m.Symbols.Intern("roots-writer-x")
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	square, err := m.MkArithmeticMul(x, x)
	if err != nil {
		t.Fatal(err)
	}
	expression, err := m.MkArithmeticSub(square, two)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := m.NewRef(expression)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DelRef(&ref); err != nil {
			t.Error(err)
		}
	}()
	for _, allowance := range []int{0, 7, 15} {
		if err = context.WritePolynomialRoots(&subpavingRefusingWriter{remaining: allowance}, expression, nil); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal("writer error", allowance, err)
		}
		if errors.Is(err, polynomial.ErrAlgebraicPending) {
			t.Fatal("algebraic cleanup left pending work", allowance, err)
		}
		var output bytes.Buffer
		if err = context.WritePolynomialRoots(&output, expression, nil); err != nil {
			t.Fatal("retry", allowance, err)
		}
		if want := "(roots\n(root-obj (+ (^ x 2) (- 2)) 1)\n(root-obj (+ (^ x 2) (- 2)) 2))\n"; output.String() != want {
			t.Fatal("retry output", allowance, output.String())
		}
	}
}

func TestWritePolynomialRootsWithAlgebraicAssignment(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	m, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	integer, err := m.ArithmeticIntegerNumeralText("2")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.GetSort(integer)
	if err != nil {
		t.Fatal(err)
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		t.Fatal(err)
	}
	real, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	nameX, err := m.Symbols.Intern("root-x")
	if err != nil {
		t.Fatal(err)
	}
	nameY, err := m.Symbols.Intern("root-y")
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkConst(nameX, real)
	if err != nil {
		t.Fatal(err)
	}
	y, err := m.MkConst(nameY, real)
	if err != nil {
		t.Fatal(err)
	}
	expression, err := m.MkArithmeticSub(x, y)
	if err != nil {
		t.Fatal(err)
	}
	expressionRef, err := m.NewRef(expression)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DelRef(&expressionRef); err != nil {
			t.Error(err)
		}
	}()
	algebraic, err := m.ArithmeticAlgebraicManager()
	if err != nil {
		t.Fatal(err)
	}
	two := numeral.NewInt(2)
	var value, root polynomial.Algebraic
	if err = algebraic.SetInteger(&value, &two); err != nil {
		t.Fatal(err)
	}
	if err = algebraic.Root(&value, 2, &root); err != nil {
		t.Fatal(err)
	}
	if err = algebraic.Del(&value); err != nil {
		t.Fatal(err)
	}
	rootExpr, err := m.ArithmeticAlgebraicNumeral(&root)
	if err != nil {
		t.Fatal(err)
	}
	if err = algebraic.Del(&root); err != nil {
		t.Fatal(err)
	}
	rootRef, err := m.NewRef(rootExpr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DelRef(&rootRef); err != nil {
			t.Error(err)
		}
	}()
	var output bytes.Buffer
	if err = context.WritePolynomialRoots(&output, expression, []ast.ASTHandle{y, rootExpr}); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "(roots\n(root-obj (+ (^ x 2) (- 2)) 2))\n"; got != want {
		t.Fatalf("algebraic assignment roots = %q, want %q", got, want)
	}
}
