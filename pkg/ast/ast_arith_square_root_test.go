package ast

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func squareRootManager(t *testing.T) *ASTManager {
	t.Helper()
	c, g, _, budget := dynAckSourceFixture(t)
	m, err := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Params: c, Globals: g, Budget: budget})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	_, err = m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	return m
}

func TestArithmeticSquareRootNumeralBoundsAndOwnership(t *testing.T) {
	m := squareRootManager(t)
	if _, err := m.ArithmeticSquareRootNumeral(-1, false); !errors.Is(err, ErrASTDomain) {
		t.Fatal(err)
	}
	am, err := m.ArithmeticAlgebraicManager()
	astTestMust(t, err)
	for _, tc := range []struct{ n, expected int32 }{{0, 0}, {1, 1}, {4, 2}, {2147395600, 46340}, {2, -1}, {2147395601, -1}, {2147483647, -1}} {
		for _, negative := range []bool{false, true} {
			h, err := m.ArithmeticSquareRootNumeral(tc.n, negative)
			astTestMust(t, err)
			astTestMust(t, m.IncRef(h))
			if tc.expected >= 0 {
				want := tc.expected
				if negative {
					want = -want
				}
				n, matched, err := m.ArithmeticIntegralInt32(h)
				astTestMust(t, err)
				if !matched || n != want {
					t.Fatalf("sqrt(%d), negative %v = %d, matched %v", tc.n, negative, n, matched)
				}
			} else {
				var value, integer, reference polynomial.Algebraic
				matched, err := m.ArithmeticAlgebraicNumeralValue(h, &value)
				astTestMust(t, err)
				if !matched {
					t.Fatal("expected an irrational numeral")
				}
				number := unumeral.NewInt(tc.n)
				astTestMust(t, am.SetInteger(&integer, &number))
				astTestMust(t, am.Root(&integer, 2, &reference))
				if negative {
					astTestMust(t, am.Neg(&reference))
				}
				order, err := am.Compare(&value, &reference)
				astTestMust(t, err)
				if order != 0 {
					t.Fatalf("root differs from independent construction for %d, negative %v", tc.n, negative)
				}
				for _, v := range []*polynomial.Algebraic{&integer, &reference, &value} {
					astTestMust(t, am.Del(v))
				}
			}
			astTestMust(t, m.DecRef(h))
		}
	}
	w := m.arithmeticPlugin.algebraic
	if w.nextID != 1 || len(w.freeIDs) != 1 {
		t.Fatalf("root slots %d, free %v", w.nextID, w.freeIDs)
	}
	astTestMust(t, m.beginFormatInfo(false, w.owner.family, 1, nil))
	_, err = m.ArithmeticSquareRootNumeral(11, false)
	m.endFormatInfo()
	if !errors.Is(err, ErrASTPending) || len(w.freeIDs) != 1 {
		t.Fatalf("unpublished root: %v, free %v", err, w.freeIDs)
	}
	h, err := m.ArithmeticSquareRootNumeral(11, true)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(h))
	astTestMust(t, m.DecRef(h))
}

func TestArithmeticSquareRootNumeralReservationFailures(t *testing.T) {
	done := false
	for ordinal := int64(0); !done && ordinal < 128; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m := squareRootManager(t)
			_, err := m.ArithmeticAlgebraicManager()
			astTestMust(t, err)
			m.Budget().FailAfter(ordinal)
			h, err := m.ArithmeticSquareRootNumeral(19, false)
			m.Budget().FailAfter(-1)
			if err == nil {
				done = true
				astTestMust(t, m.IncRef(h))
				astTestMust(t, m.DecRef(h))
				return
			}
			if !h.IsNull() {
				t.Fatal("published a root after reservation failure")
			}
			retried, err := m.ArithmeticSquareRootNumeral(19, false)
			astTestMust(t, err)
			astTestMust(t, m.IncRef(retried))
			astTestMust(t, m.DecRef(retried))
		})
	}
	if !done {
		t.Fatal("did not reach a successful construction")
	}
}

func TestCompactSquareRootPrintReadAndRecycle(t *testing.T) {
	m := squareRootManager(t)
	if !m.unboundedAlgebraicReads() {
		t.Skip("compact values require the ordinary arithmetic configuration")
	}
	for _, tc := range []struct {
		negative bool
		index    int
	}{{false, 2}, {true, 1}} {
		h, err := m.ArithmeticSquareRootNumeral(2, tc.negative)
		astTestMust(t, err)
		astTestMust(t, m.IncRef(h))
		w := m.arithmeticPlugin.algebraic
		for range 2 {
			format, ok, err := m.SMT2PPArithLiteral(h, false, 5)
			astTestMust(t, err)
			want, err := m.FormatString(fmt.Sprintf("(root-obj (+ (^ x 2) (- 2)) %d)", tc.index))
			astTestMust(t, err)
			if !ok || format != want || w.values[0] != (polynomial.Algebraic{}) {
				t.Fatal("exact printing did not preserve the compact root")
			}
		}
		decimal, ok, err := m.SMT2PPArithLiteral(h, true, 3)
		astTestMust(t, err)
		want, err := m.FormatString("1.414?")
		astTestMust(t, err)
		if tc.negative {
			want, err = m.FormatSeq1("-", []ASTHandle{want})
			astTestMust(t, err)
		}
		if !ok || decimal != want || w.values[0] == (polynomial.Algebraic{}) {
			t.Fatal("decimal printing did not use the exact numeric value")
		}
		astTestMust(t, m.DecRef(h))
		if w.signedRadicand(0) != 0 || len(w.freeIDs) != 1 {
			t.Fatal("recycled root retained its signed radicand")
		}
	}
}

func TestCompactSquareRootRetriesFailedNumericRead(t *testing.T) {
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		m := squareRootManager(t)
		if !m.unboundedAlgebraicReads() {
			t.Skip("compact values require the ordinary arithmetic configuration")
		}
		h, err := m.ArithmeticSquareRootNumeral(19, true)
		astTestMust(t, err)
		astTestMust(t, m.IncRef(h))
		w := m.arithmeticPlugin.algebraic
		m.Budget().FailAfter(ordinal)
		_, readErr := w.value(0)
		m.Budget().FailAfter(-1)
		if readErr != nil {
			if w.values[0] != (polynomial.Algebraic{}) || w.signedRadicand(0) != -19 {
				t.Fatal("failed read published an incomplete cell", ordinal)
			}
			_, err = w.value(0)
			astTestMust(t, err)
		}
		negative, err := w.manager.IsNeg(&w.values[0])
		astTestMust(t, err)
		if !negative {
			t.Fatal("lost root sign on numeric read", ordinal)
		}
		astTestMust(t, m.DecRef(h))
		if readErr == nil {
			return
		}
	}
	t.Fatal("did not reach a successful numeric read")
}

func TestTranslatedCompactRootSurvivesSourceReuse(t *testing.T) {
	t.Run("IntegerRadicand", func(t *testing.T) {
		checkTranslatedCompactRoot(t, 1, "(root-obj (+ (^ x 2) (- 17)) 1)")
	})
	t.Run("FractionalRadicand", func(t *testing.T) {
		checkTranslatedCompactRoot(t, 3, "(root-obj (+ (* 3 (^ x 2)) (- 17)) 1)")
	})
}

func checkTranslatedCompactRoot(t *testing.T, denominator int32, expected string) {
	t.Helper()
	from := squareRootManager(t)
	to, err := NewASTManager(ASTManagerConfig{Symbols: from.Symbols, Numerals: from.numerals,
		Params: from.params, Globals: from.globals, Budget: from.BudgetField})
	astTestMust(t, err)
	defer func() { astTestMust(t, to.Close()) }()
	_, err = to.RegisterArithmeticPlugin()
	astTestMust(t, err)
	tr, err := NewASTTranslation(from, to, false)
	astTestMust(t, err)
	h, err := from.ArithmeticSquareRootFraction(17, denominator, true)
	astTestMust(t, err)
	astTestMust(t, from.IncRef(h))
	copy, err := tr.Translate(h)
	astTestMust(t, err)
	astTestMust(t, to.IncRef(copy))
	astTestMust(t, tr.Close())
	astTestMust(t, from.DecRef(h))
	h, err = from.ArithmeticSquareRootNumeral(23, false)
	astTestMust(t, err)
	astTestMust(t, from.IncRef(h))
	defer func() { astTestMust(t, from.DecRef(h)) }()
	format, ok, err := to.SMT2PPArithLiteral(copy, false, 8)
	astTestMust(t, err)
	want, err := to.FormatString(expected)
	astTestMust(t, err)
	if !ok || format != want {
		t.Fatal("translated number changed after source slot reuse")
	}
	astTestMust(t, to.DecRef(copy))
}
