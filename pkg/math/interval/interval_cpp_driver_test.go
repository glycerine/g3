package interval

// Source: src/test/interval.cpp mk_random_interval and arithmetic workloads.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/clibrand"
)

type sourceRandomInterval struct {
	lower, upper                             int64
	lowerInf, upperInf, lowerOpen, upperOpen bool
}

func sourceIntervalRandom(rng *clibrand.GLibc31, magnitude uint32) sourceRandomInterval {
	var a sourceRandomInterval
	switch rng.Next() % 3 {
	case 0:
		if rng.Next()%4 == 0 {
			a.lowerInf, a.lowerOpen = true, true
			a.upperOpen = rng.Next()%2 == 0
			a.upper = -int64(rng.Next() % magnitude)
		} else {
			a.upperOpen = rng.Next()%2 == 0
			a.upper = -int64(rng.Next() % magnitude)
			a.lowerOpen = rng.Next()%2 == 0
			a.lower = a.upper - int64(rng.Next()%magnitude)
			if a.lowerOpen || a.upperOpen {
				a.lower--
			}
		}
	case 1:
		if rng.Next()%4 == 0 {
			a.lowerInf, a.lowerOpen = true, true
		} else {
			a.lowerOpen = rng.Next()%2 == 0
			a.lower = -int64(rng.Next()%magnitude) - 1
		}
		if rng.Next()%4 == 0 {
			a.upperInf, a.upperOpen = true, true
		} else {
			a.upperOpen = rng.Next()%2 == 0
			a.upper = int64(rng.Next()%magnitude) + 1
		}
	default:
		if rng.Next()%4 == 0 {
			a.upperInf, a.upperOpen = true, true
			a.lowerOpen = rng.Next()%2 == 0
			a.lower = int64(rng.Next() % magnitude)
		} else {
			a.lowerOpen = rng.Next()%2 == 0
			a.lower = int64(rng.Next() % magnitude)
			a.upperOpen = rng.Next()%2 == 0
			a.upper = a.lower + int64(rng.Next()%magnitude)
			if a.lowerOpen || a.upperOpen {
				a.upper++
			}
		}
	}
	return a
}

func (a sourceRandomInterval) hasZero() bool {
	return (a.lowerInf || a.lower < 0 || a.lower == 0 && !a.lowerOpen) && (a.upperInf || a.upper > 0 || a.upper == 0 && !a.upperOpen)
}
func (a sourceRandomInterval) point() *big.Rat {
	if a.lowerInf && a.upperInf {
		return new(big.Rat)
	}
	if a.lowerInf {
		return new(big.Rat).SetInt64(a.upper - 1)
	}
	if a.upperInf {
		return new(big.Rat).SetInt64(a.lower + 1)
	}
	return new(big.Rat).SetFrac(big.NewInt(a.lower+a.upper), big.NewInt(2))
}

func (a sourceRandomInterval) points() []*big.Rat {
	points := []*big.Rat{a.point()}
	if !a.lowerInf {
		v := new(big.Rat).SetInt64(a.lower)
		if a.lowerOpen {
			v.Add(v, big.NewRat(1, 2))
		}
		points = append(points, v)
	}
	if !a.upperInf {
		v := new(big.Rat).SetInt64(a.upper)
		if a.upperOpen {
			v.Sub(v, big.NewRat(1, 2))
		}
		points = append(points, v)
	}
	return points
}

func TestIntervalSourceSeededArithmetic(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	a, b, result := f.interval(), f.interval(), f.interval()
	point := f.rational("0")
	set := func(dst *RationalInterval, src sourceRandomInterval) {
		t.Helper()
		if err := f.number.SetInt64(&dst.lower, src.lower); err != nil {
			t.Fatal(err)
		}
		if err := f.number.SetInt64(&dst.upper, src.upper); err != nil {
			t.Fatal(err)
		}
		dst.lowerInf, dst.upperInf, dst.lowerOpen, dst.upperOpen = src.lowerInf, src.upperInf, src.lowerOpen, src.upperOpen
		if err := dst.CheckInvariant(); err != nil {
			t.Fatal("generated interval", src, err)
		}
	}
	var rng clibrand.GLibc31
	// Reproduce the root passes before the source's arithmetic passes.
	for count := 0; count < 1000; {
		v := sourceIntervalRandom(&rng, 10)
		if !v.lowerInf && v.lower >= 0 {
			count++
		}
	}
	for i := 0; i < 1000; i++ {
		sourceIntervalRandom(&rng, 10)
	}
	for _, op := range []string{"div", "inv", "pow2", "pow3", "neg", "sub", "mul", "add"} {
		for i := 0; i < 1000; i++ {
			x := sourceIntervalRandom(&rng, 3)
			var y sourceRandomInterval
			if op == "div" {
				for {
					y = sourceIntervalRandom(&rng, 3)
					if !y.hasZero() {
						break
					}
				}
			} else if op == "inv" {
				for x.hasZero() {
					x = sourceIntervalRandom(&rng, 3)
				}
			} else if op == "sub" || op == "mul" || op == "add" {
				y = sourceIntervalRandom(&rng, 3)
			}
			set(a, x)
			var err error
			switch op {
			case "div":
				set(b, y)
				err = result.Div(a, b)
			case "inv":
				err = result.Inv(a)
			case "pow2":
				err = result.Power(a, 2)
			case "pow3":
				err = result.Power(a, 3)
			case "neg":
				err = result.Neg(a)
			case "sub":
				set(b, y)
				err = result.Sub(a, b)
			case "mul":
				set(b, y)
				err = result.Mul(a, b)
			case "add":
				set(b, y)
				err = result.Add(a, b)
			}
			if err != nil {
				t.Fatal(op, i, x, y, err)
			}
			yPoints := []*big.Rat{new(big.Rat)}
			if op == "div" || op == "sub" || op == "mul" || op == "add" {
				yPoints = y.points()
			}
			for _, xp := range x.points() {
				for _, yp := range yPoints {
					var want *big.Rat
					switch op {
					case "div":
						want = new(big.Rat).Quo(xp, yp)
					case "inv":
						want = new(big.Rat).Inv(xp)
					case "pow2":
						want = new(big.Rat).Mul(xp, xp)
					case "pow3":
						want = new(big.Rat).Mul(new(big.Rat).Mul(xp, xp), xp)
					case "neg":
						want = new(big.Rat).Neg(xp)
					case "sub":
						want = new(big.Rat).Sub(xp, yp)
					case "mul":
						want = new(big.Rat).Mul(xp, yp)
					case "add":
						want = new(big.Rat).Add(xp, yp)
					}
					if err = f.number.SetString(point, want.RatString()); err != nil {
						t.Fatal(err)
					}
					if ok, err := result.Contains(point); err != nil || !ok {
						t.Fatal("interval lost source point", op, i, x, y, xp, yp, want, err)
					}
				}
			}
		}
	}
}
