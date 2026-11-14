package smtcore

import (
	"math/big"
	"math/rand"
	"testing"
)

// The word fast paths must give exactly math/big's normalized results,
// including when the destination aliases an operand.
func TestSMTRatOpsMatchMathBig(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	pick := func() *big.Rat {
		var n, d *big.Int
		switch rng.Intn(6) {
		case 4:
			// big values with shared small factors in the denominators.
			n = new(big.Int).Mul(big.NewInt(rng.Int63()-(1<<62)), big.NewInt(rng.Int63n(1<<20)+1))
			d = new(big.Int).Mul(big.NewInt(int64(rng.Intn(36)+1)), new(big.Int).Lsh(big.NewInt(rng.Int63n(1<<30)+1), uint(rng.Intn(80))))
		case 5:
			n = big.NewInt(int64(rng.Intn(3) - 1))
			if n.Sign() == 0 {
				n.SetInt64(1)
			}
			d = big.NewInt(1)
		case 0:
			n, d = big.NewInt(rng.Int63n(7)-3), big.NewInt(rng.Int63n(4)+1)
		case 1:
			n, d = big.NewInt(rng.Int63n(1<<32)-(1<<31)), big.NewInt(rng.Int63n(1<<31)+1)
		case 2:
			n = new(big.Int).Lsh(big.NewInt(rng.Int63()-(1<<62)), uint(rng.Intn(70)))
			d = big.NewInt(1)
		default:
			n = new(big.Int).Lsh(big.NewInt(rng.Int63()-(1<<62)), uint(rng.Intn(70)))
			d = new(big.Int).Lsh(big.NewInt(rng.Int63n(1<<40)+1), uint(rng.Intn(40)))
		}
		return new(big.Rat).SetFrac(n, d)
	}
	type op struct {
		name string
		fast func(z, x, y *big.Rat) *big.Rat
		ref  func(z, x, y *big.Rat) *big.Rat
	}
	ops := []op{
		{"add", smtRatAdd, (*big.Rat).Add},
		{"sub", smtRatSub, (*big.Rat).Sub},
		{"mul", smtRatMul, (*big.Rat).Mul},
		{"quo", smtRatQuo, (*big.Rat).Quo},
	}
	same := func(a, b *big.Rat) bool {
		return a.Num().Cmp(b.Num()) == 0 && a.Denom().Cmp(b.Denom()) == 0
	}
	for i := 0; i < 20000; i++ {
		x, y := pick(), pick()
		for _, o := range ops {
			if o.name == "quo" && y.Sign() == 0 {
				continue
			}
			want := o.ref(new(big.Rat), x, y)
			var uninit big.Rat
			got := []*big.Rat{
				o.fast(new(big.Rat).Set(pick()), x, y),
				o.fast(&uninit, x, y),
				o.fast(new(big.Rat).Set(x), new(big.Rat).Set(x), y),
			}
			wantSelf := want
			if o.name != "quo" || x.Sign() != 0 {
				xa := new(big.Rat).Set(x)
				got = append(got, o.fast(xa, xa, xa))
				wantSelf = o.ref(new(big.Rat), x, x)
			} else {
				got = append(got, want)
			}
			ya := new(big.Rat).Set(y)
			got = append(got, o.fast(ya, x, ya))
			for k, g := range got {
				w := want
				if k == 3 {
					w = wantSelf
				}
				if !same(g, w) || g.Cmp(w) != 0 {
					t.Fatalf("%s(%v, %v) variant %d = %v, want %v", o.name, x, y, k, g, w)
				}
			}
		}
		a, b := x.Num(), y.Num()
		want := new(big.Int)
		if a.Sign() != 0 && b.Sign() != 0 {
			g := new(big.Int).GCD(nil, nil, a, b)
			want.Abs(want.Mul(new(big.Int).Quo(a, g), b))
		}
		if got := smtIntLCM(new(big.Int), a, b, nil); got.Cmp(want) != 0 {
			t.Fatalf("lcm(%v, %v) = %v, want %v", a, b, got, want)
		}
		aa := new(big.Int).Set(a)
		if got := smtIntLCM(aa, aa, b, new(big.Int)); got.Cmp(want) != 0 {
			t.Fatalf("lcm aliasing a (%v, %v) = %v, want %v", a, b, got, want)
		}
		bb := new(big.Int).Set(b)
		if got := smtIntLCM(bb, a, bb, new(big.Int)); got.Cmp(want) != 0 {
			t.Fatalf("lcm aliasing b (%v, %v) = %v, want %v", a, b, got, want)
		}
	}
}
