package cmd_context_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/math/polynomial"
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
