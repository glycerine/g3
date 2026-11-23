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
