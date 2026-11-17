package smtcore

import (
	"math/big"
	"sync"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var (
	testQOnce sync.Once
	testQM    *numeral.RationalManager
)

// testQ is a synchronized rational manager for unit tests that build
// theory numerals without an AST manager.
func testQ() *numeral.RationalManager {
	testQOnce.Do(func() {
		budget := arena.NewBudget(^uint64(0))
		m, err := numeral.NewRationalManager(numeral.RationalConfig{Synchronized: true, Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
		if err != nil {
			panic(err)
		}
		testQM = m
	})
	return testQM
}

// testQR is a new rational of testQ with value r.
func testQR(r *big.Rat) *numeral.Rational {
	out := smtQZero()
	if err := testQ().SetBigRat(&out, r); err != nil {
		panic(err)
	}
	return &out
}

// testBig reads a rational of testQ (or of an AST manager's rational
// manager, through q) as math/big.
func testBig(r *numeral.Rational) *big.Rat {
	b, err := smtQBig(testQ(), r)
	if err != nil {
		panic(err)
	}
	return b
}

// testArithValue is inf_numeral(rational, infinitesimal); nil is zero.
func testArithValue(rational, infinitesimal *big.Rat) smtArithmeticValue {
	v := newSMTArithmeticValue()
	if rational != nil {
		v.rational = *testQR(rational)
	}
	if infinitesimal != nil {
		v.infinitesimal = *testQR(infinitesimal)
	}
	return v
}

func TestSMTArithmeticValueOperations(t *testing.T) {
	q := testQ()
	a := testArithValue(big.NewRat(3, 2), big.NewRat(-1, 1))
	b := testArithValue(big.NewRat(1, 3), big.NewRat(2, 1))
	if err := a.add(q, &b); err != nil {
		t.Fatal(err)
	}
	if testBig(&a.rational).Cmp(big.NewRat(11, 6)) != 0 || testBig(&a.infinitesimal).Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("add", testBig(&a.rational), testBig(&a.infinitesimal))
	}
	if err := a.subMul(q, testQR(big.NewRat(2, 1)), &b); err != nil {
		t.Fatal(err)
	}
	if testBig(&a.rational).Cmp(big.NewRat(7, 6)) != 0 || testBig(&a.infinitesimal).Cmp(big.NewRat(-3, 1)) != 0 {
		t.Fatal("submul", testBig(&a.rational), testBig(&a.infinitesimal))
	}
	if c, err := a.cmp(q, &b); err != nil || c <= 0 {
		t.Fatal("cmp", c, err)
	}
	f := smtQZero()
	if err := smtInfFloor(q, &a, &f); err != nil || testBig(&f).Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("floor", err)
	}
	k := testArithValue(big.NewRat(2, 1), big.NewRat(-1, 1))
	if err := smtInfFloor(q, &k, &f); err != nil || testBig(&f).Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("floor of 2 - eps", err)
	}
	if err := smtInfCeil(q, &k, &f); err != nil || testBig(&f).Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("ceil of 2 - eps", err)
	}
	// Equal values have equal map keys, whatever their representation.
	big1 := testQR(new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 70)))
	k1, _ := smtQKey(q, big1)
	k2, _ := smtQKey(q, testQR(new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 70))))
	small1, _ := smtQKey(q, testQR(big.NewRat(-7, 3)))
	small2 := smtQMapKey{n: -7, d: 3}
	if k1 != k2 || small1 != small2 || k1 == small1 {
		t.Fatal("map keys", k1, k2, small1)
	}
	for _, v := range []*smtArithmeticValue{&a, &b, &k} {
		if err := v.del(q); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []*numeral.Rational{&f, big1} {
		if err := q.Del(r); err != nil {
			t.Fatal(err)
		}
	}
}
