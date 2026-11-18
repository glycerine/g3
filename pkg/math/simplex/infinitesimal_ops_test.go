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
	// Beyond int64, so a small-value path cannot silently truncate.
	huge := new(big.Int).Lsh(big.NewInt(int64(r.Intn(9)-4)), 90)
	return new(big.Rat).SetFrac(huge, big.NewInt(int64(1+r.Intn(30))))
}

type bigInfinitesimal struct{ infinity, main, epsilon *big.Rat }

func randomInfinitesimal(r *rand.Rand) (*Infinitesimal, bigInfinitesimal) {
	b := bigInfinitesimal{randomRat(r), randomRat(r), randomRat(r)}
	return testNII(bigRat(b.infinity), bigRat(b.main), bigRat(b.epsilon)), b
}

func sameInfinitesimal(a *Infinitesimal, infinity, main, epsilon *big.Rat) bool {
	return ratBig(&a.Infinity).Cmp(infinity) == 0 && ratBig(&a.Main).Cmp(main) == 0 && ratBig(&a.Epsilon).Cmp(epsilon) == 0
}

func slowCmp(a, b bigInfinitesimal) int {
	if n := a.infinity.Cmp(b.infinity); n != 0 {
		return n
	}
	if n := a.main.Cmp(b.main); n != 0 {
		return n
	}
	return a.epsilon.Cmp(b.epsilon)
}

// Every operation must agree with plain big.Rat arithmetic, including zero
// operands, values past int64, and repeated use of the same receiver.
func TestInfinitesimalOperationsMatchBigRat(t *testing.T) {
	q := optQ()
	r := rand.New(rand.NewSource(11))
	for round := 0; round < 2000; round++ {
		a, ab := randomInfinitesimal(r)
		b, bb := randomInfinitesimal(r)
		m := randomRat(r)
		mq := bigRat(m)
		wi, wm, we := new(big.Rat), new(big.Rat), new(big.Rat)

		sum := iclone(a)
		mustOK(t, sum.Add(q, b))
		wi.Add(ab.infinity, bb.infinity)
		wm.Add(ab.main, bb.main)
		we.Add(ab.epsilon, bb.epsilon)
		if !sameInfinitesimal(sum, wi, wm, we) {
			t.Fatal("add", itext(a), itext(b), itext(sum))
		}
		diff := iclone(a)
		mustOK(t, diff.Sub(q, b))
		wi.Sub(ab.infinity, bb.infinity)
		wm.Sub(ab.main, bb.main)
		we.Sub(ab.epsilon, bb.epsilon)
		if !sameInfinitesimal(diff, wi, wm, we) {
			t.Fatal("sub", itext(a), itext(b), itext(diff))
		}
		negated := iclone(a)
		mustOK(t, negated.Neg(q))
		wi.Neg(ab.infinity)
		wm.Neg(ab.main)
		we.Neg(ab.epsilon)
		if !sameInfinitesimal(negated, wi, wm, we) {
			t.Fatal("neg", itext(a))
		}
		product := iclone(a)
		mustOK(t, product.Mul(q, mq))
		wi.Mul(ab.infinity, m)
		wm.Mul(ab.main, m)
		we.Mul(ab.epsilon, m)
		if !sameInfinitesimal(product, wi, wm, we) {
			t.Fatal("mul", itext(a), m)
		}
		if m.Sign() != 0 {
			quotient := iclone(a)
			mustOK(t, quotient.Quo(q, mq))
			wi.Quo(ab.infinity, m)
			wm.Quo(ab.main, m)
			we.Quo(ab.epsilon, m)
			if !sameInfinitesimal(quotient, wi, wm, we) {
				t.Fatal("quo", itext(a), m)
			}
			mustOK(t, quotient.Del(q))
		}
		if got, want := icmp(a, b), slowCmp(ab, bb); got != want {
			t.Fatal("cmp", itext(a), itext(b), got, want)
		}
		// Overwriting a nonzero receiver with zero must really clear it.
		overwrite, _ := randomInfinitesimal(r)
		mustOK(t, overwrite.Set(q, a))
		if icmp(overwrite, a) != 0 {
			t.Fatal("set", itext(a), itext(overwrite))
		}
		for _, v := range []*Infinitesimal{a, b, sum, diff, negated, product, overwrite} {
			mustOK(t, v.Del(q))
		}
		mustOK(t, q.Del(mq))
	}
}

// A clone must not share storage with its source.
func TestInfinitesimalCloneIsIndependent(t *testing.T) {
	q := optQ()
	a := testNII(rat(3, 2), rat(-7, 3), rat(5, 1))
	clone := iclone(a)
	mustOK(t, clone.Add(q, a))
	mustOK(t, clone.Mul(q, rat(9, 4)))
	if !sameInfinitesimal(a, big.NewRat(3, 2), big.NewRat(-7, 3), big.NewRat(5, 1)) {
		t.Fatal("source changed through its clone", itext(a))
	}
}

func TestInfinitesimalQuoByZeroFails(t *testing.T) {
	zero := ZeroInfinitesimal()
	if err := zero.Quo(optQ(), rat(0, 1)); err == nil {
		t.Fatal("division by zero did not fail")
	}
}
