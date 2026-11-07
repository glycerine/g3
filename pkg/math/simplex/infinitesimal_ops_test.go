package simplex

import (
	"math/big"
	"math/rand"
	"testing"
)

func randomRat(r *rand.Rand) *big.Rat {
	switch r.Intn(6) {
	case 0, 1, 2:
		return new(big.Rat)
	case 3:
		return big.NewRat(int64(r.Intn(11)-5), 1)
	case 4:
		return big.NewRat(int64(r.Intn(201)-100), int64(1+r.Intn(30)))
	}
	// Beyond int64, so a fast path cannot silently truncate.
	huge := new(big.Int).Lsh(big.NewInt(int64(r.Intn(9)-4)), 90)
	return new(big.Rat).SetFrac(huge, big.NewInt(int64(1+r.Intn(30))))
}

func randomInfinitesimal(r *rand.Rand) Infinitesimal {
	return NewInfiniteInfinitesimal(randomRat(r), randomRat(r), randomRat(r))
}

func slowCmp(a, b *Infinitesimal) int {
	if n := a.Infinity.Cmp(&b.Infinity); n != 0 {
		return n
	}
	if n := a.Main.Cmp(&b.Main); n != 0 {
		return n
	}
	return a.Epsilon.Cmp(&b.Epsilon)
}

func sameInfinitesimal(a *Infinitesimal, infinity, main, epsilon *big.Rat) bool {
	return a.Infinity.Cmp(infinity) == 0 && a.Main.Cmp(main) == 0 && a.Epsilon.Cmp(epsilon) == 0
}

// Every fast path must agree with plain big.Rat arithmetic, including zero
// operands, values past int64, and repeated use of the same receiver.
func TestInfinitesimalOperationsMatchBigRat(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for round := 0; round < 20000; round++ {
		a, b := randomInfinitesimal(r), randomInfinitesimal(r)
		q := randomRat(r)
		wi, wm, we := new(big.Rat), new(big.Rat), new(big.Rat)

		sum := a.Clone()
		sum.Add(&b)
		wi.Add(&a.Infinity, &b.Infinity)
		wm.Add(&a.Main, &b.Main)
		we.Add(&a.Epsilon, &b.Epsilon)
		if !sameInfinitesimal(&sum, wi, wm, we) {
			t.Fatal("add", a.String(), b.String(), sum.String())
		}
		diff := a.Clone()
		diff.Sub(&b)
		wi.Sub(&a.Infinity, &b.Infinity)
		wm.Sub(&a.Main, &b.Main)
		we.Sub(&a.Epsilon, &b.Epsilon)
		if !sameInfinitesimal(&diff, wi, wm, we) {
			t.Fatal("sub", a.String(), b.String(), diff.String())
		}
		negated := a.Clone()
		negated.Neg()
		wi.Neg(&a.Infinity)
		wm.Neg(&a.Main)
		we.Neg(&a.Epsilon)
		if !sameInfinitesimal(&negated, wi, wm, we) {
			t.Fatal("neg", a.String())
		}
		product := a.Clone()
		product.Mul(q)
		wi.Mul(&a.Infinity, q)
		wm.Mul(&a.Main, q)
		we.Mul(&a.Epsilon, q)
		if !sameInfinitesimal(&product, wi, wm, we) {
			t.Fatal("mul", a.String(), q)
		}
		if q.Sign() != 0 {
			quotient := a.Clone()
			quotient.Quo(q)
			wi.Quo(&a.Infinity, q)
			wm.Quo(&a.Main, q)
			we.Quo(&a.Epsilon, q)
			if !sameInfinitesimal(&quotient, wi, wm, we) {
				t.Fatal("quo", a.String(), q)
			}
		}
		if got, want := a.Cmp(&b), slowCmp(&a, &b); got != want {
			t.Fatal("cmp", a.String(), b.String(), got, want)
		}
		// Overwriting a nonzero receiver with zero must really clear it.
		overwrite := randomInfinitesimal(r)
		overwrite.Set(&a)
		if overwrite.Cmp(&a) != 0 {
			t.Fatal("set", a.String(), overwrite.String())
		}
	}
}

// A clone must not share storage with its source.
func TestInfinitesimalCloneIsIndependent(t *testing.T) {
	a := NewInfiniteInfinitesimal(big.NewRat(3, 2), big.NewRat(-7, 3), big.NewRat(5, 1))
	clone := a.Clone()
	clone.Add(&a)
	clone.Mul(big.NewRat(9, 4))
	if !sameInfinitesimal(&a, big.NewRat(3, 2), big.NewRat(-7, 3), big.NewRat(5, 1)) {
		t.Fatal("source changed through its clone", a.String())
	}
}

func TestInfinitesimalQuoByZeroStillPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("division by zero did not panic")
		}
	}()
	var zero Infinitesimal
	zero.Quo(new(big.Rat))
}

// A rational-only value must not allocate for its zero components.
func TestInfinitesimalZeroComponentsDoNotAllocate(t *testing.T) {
	var a, b Infinitesimal
	if allocs := testing.AllocsPerRun(100, func() {
		a.Add(&b)
		a.Sub(&b)
		a.Set(&b)
		a.Neg()
		_ = a.Cmp(&b)
	}); allocs != 0 {
		t.Fatalf("%v allocations", allocs)
	}
}
