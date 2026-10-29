package sls

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func slsSigned(value *big.Int, width uint32) *big.Int {
	modulus := slsModulus(width)
	half := new(big.Int).Rsh(new(big.Int).Set(modulus), 1)
	out := new(big.Int).Set(value)
	if out.Cmp(half) >= 0 {
		out.Sub(out, modulus)
	}
	return out
}

func slsSignedModOracle(x, y *big.Int, width uint32) *big.Int {
	if y.Sign() == 0 {
		return slsModulo(x, width)
	}
	absX, absY := new(big.Int).Abs(new(big.Int).Set(x)), new(big.Int).Abs(new(big.Int).Set(y))
	r := new(big.Int).Mod(absX, absY)
	switch {
	case r.Sign() == 0 || x.Sign() >= 0 && y.Sign() >= 0:
	case x.Sign() < 0 && y.Sign() >= 0:
		r.Sub(y, r)
	case x.Sign() >= 0 && y.Sign() < 0:
		r.Add(r, y)
	default:
		r.Neg(r)
	}
	return slsModulo(r, width)
}

func TestBVEvaluatorSignedDivisionRemainderModuloOracles(t *testing.T) {
	for _, width := range []uint32{8, 80} {
		manager, budget := powersManager(t, 1<<29)
		p, err := newPowers(manager)
		if err != nil {
			t.Fatal(err)
		}
		evaluator, err := newBVEvaluatorSlice(manager, p)
		if err != nil {
			t.Fatal(err)
		}
		modulus, half := slsModulus(width), new(big.Int).Rsh(slsModulus(width), 1)
		patterns := [][2]*big.Int{
			{big.NewInt(23), big.NewInt(5)},
			{new(big.Int).Sub(modulus, big.NewInt(23)), big.NewInt(5)},
			{big.NewInt(23), new(big.Int).Sub(modulus, big.NewInt(5))},
			{new(big.Int).Sub(modulus, big.NewInt(23)), new(big.Int).Sub(modulus, big.NewInt(5))},
			{new(big.Int).Set(half), new(big.Int).Sub(modulus, big.NewInt(1))}, // signed MIN/-1 overflow
			{new(big.Int).Sub(modulus, big.NewInt(7)), big.NewInt(0)},
			{big.NewInt(7), big.NewInt(0)},
		}
		for i, pair := range patterns {
			left, right := slsSetBig(t, manager, pair[0]), slsSetBig(t, manager, pair[1])
			leftBefore, rightBefore := pair[0].String(), pair[1].String()
			sx, sy := slsSigned(pair[0], width), slsSigned(pair[1], width)
			var result numeral.Int
			if err := evaluator.signedDiv(&left, &right, width, &result); err != nil {
				t.Fatal(i, err)
			}
			var divWant *big.Int
			if sy.Sign() == 0 {
				if sx.Sign() < 0 {
					divWant = big.NewInt(1)
				} else {
					divWant = new(big.Int).Sub(modulus, big.NewInt(1))
				}
			} else {
				divWant = slsModulo(new(big.Int).Quo(sx, sy), width)
			}
			if got := slsIntegerText(t, manager, &result); got != divWant.String() {
				t.Fatalf("width %d case %d sdiv=%s want %s", width, i, got, divWant)
			}
			if err := evaluator.signedRem(&left, &right, width, &result); err != nil {
				t.Fatal(i, err)
			}
			remWant := sx
			if sy.Sign() != 0 {
				remWant = new(big.Int).Rem(sx, sy)
			}
			remWant = slsModulo(remWant, width)
			if got := slsIntegerText(t, manager, &result); got != remWant.String() {
				t.Fatalf("width %d case %d srem=%s want %s", width, i, got, remWant)
			}
			if err := evaluator.signedMod(&left, &right, width, &result); err != nil {
				t.Fatal(i, err)
			}
			modWant := slsSignedModOracle(sx, sy, width)
			if got := slsIntegerText(t, manager, &result); got != modWant.String() {
				t.Fatalf("width %d case %d smod=%s want %s", width, i, got, modWant)
			}
			if slsIntegerText(t, manager, &left) != leftBefore || slsIntegerText(t, manager, &right) != rightBefore {
				t.Fatal("signed operation changed borrowed operands")
			}
			for _, value := range []*numeral.Int{&result, &left, &right} {
				if err := manager.Del(value); err != nil {
					t.Fatal(err)
				}
			}
		}
		if p.byShift[width] == nil || p.byShift[width-1] == nil || len(p.byShift) != 2 {
			t.Fatal("signed family cache", p.byShift)
		}
		if err := p.closeChecked(); err != nil {
			t.Fatal(err)
		}
		if err := manager.CloseChecked(); err != nil {
			t.Fatal(err)
		}
		if budget.Used() != 0 {
			t.Fatal("signed family leak", budget.Used())
		}
	}
}

func TestBVEvaluatorBitwiseAndComparisonOracles(t *testing.T) {
	manager, budget := powersManager(t, 1<<29)
	p, err := newPowers(manager)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := newBVEvaluatorSlice(manager, p)
	if err != nil {
		t.Fatal(err)
	}
	width := uint32(80)
	mask := new(big.Int).Sub(slsModulus(width), big.NewInt(1))
	bigValues := []*big.Int{
		new(big.Int).SetUint64(0xf0f0f0f0),
		new(big.Int).Sub(slsModulus(width), big.NewInt(0x12345)),
		new(big.Int).SetUint64(0xaaaaaaaa),
	}
	values := []numeral.Int{slsSetBig(t, manager, bigValues[0]), slsSetBig(t, manager, bigValues[1]), slsSetBig(t, manager, bigValues[2])}
	args := []*numeral.Int{&values[0], &values[1], &values[2]}
	var result numeral.Int
	for _, kind := range []bvBitwiseKind{bvAnd, bvOr, bvXor, bvNand, bvNor} {
		if err := evaluator.bitwise(args, width, kind, &result); err != nil {
			t.Fatal(kind, err)
		}
		want := new(big.Int).Set(bigValues[0])
		for _, next := range bigValues[1:] {
			switch kind {
			case bvAnd:
				want.And(want, next)
			case bvOr:
				want.Or(want, next)
			case bvXor:
				want.Xor(want, next)
			case bvNand:
				want.And(want, next).Xor(want, mask)
			case bvNor:
				want.Or(want, next).Xor(want, mask)
			}
		}
		if got := slsIntegerText(t, manager, &result); got != want.String() {
			t.Fatalf("bitwise %d=%s want %s", kind, got, want)
		}
	}
	if err := evaluator.bitwiseNot(&values[1], width, &result); err != nil {
		t.Fatal(err)
	}
	wantNot := new(big.Int).Xor(new(big.Int).Set(bigValues[1]), mask)
	if got := slsIntegerText(t, manager, &result); got != wantNot.String() {
		t.Fatal("not", got, wantNot)
	}
	for _, signed := range []bool{false, true} {
		for _, kind := range []bvComparisonKind{bvLess, bvLessEqual, bvGreater, bvGreaterEqual} {
			if signed {
				err = evaluator.signedCompare(&values[0], &values[1], width, kind, &result)
			} else {
				err = evaluator.unsignedCompare(&values[0], &values[1], kind, &result)
			}
			if err != nil {
				t.Fatal(signed, kind, err)
			}
			x, y := bigValues[0], bigValues[1]
			if signed {
				x, y = slsSigned(x, width), slsSigned(y, width)
			}
			cmp := x.Cmp(y)
			truth := cmp < 0 && kind == bvLess || cmp <= 0 && kind == bvLessEqual || cmp > 0 && kind == bvGreater || cmp >= 0 && kind == bvGreaterEqual
			want := "0"
			if truth {
				want = "1"
			}
			if got := slsIntegerText(t, manager, &result); got != want {
				t.Fatal("comparison", signed, kind, got, want)
			}
		}
	}
	if err := evaluator.identity(&values[1], &result); err != nil || slsIntegerText(t, manager, &result) != bigValues[1].String() {
		t.Fatal("identity", err)
	}
	for i := range values {
		if slsIntegerText(t, manager, &values[i]) != bigValues[i].String() {
			t.Fatal("bitwise borrowed input changed", i)
		}
		if err := manager.Del(&values[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := manager.Del(&result); err != nil {
		t.Fatal(err)
	}
	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if budget.Used() != 0 {
		t.Fatal("bitwise/comparison leak", budget.Used())
	}
}

func TestBVEvaluatorShiftOraclesAndOverflow(t *testing.T) {
	for _, width := range []uint32{8, 80} {
		manager, budget := powersManager(t, 1<<29)
		p, err := newPowers(manager)
		if err != nil {
			t.Fatal(err)
		}
		evaluator, err := newBVEvaluatorSlice(manager, p)
		if err != nil {
			t.Fatal(err)
		}
		values := []*big.Int{big.NewInt(13), new(big.Int).Sub(slsModulus(width), big.NewInt(13))}
		for _, valueBig := range values {
			value := slsSetBig(t, manager, valueBig)
			for _, shiftValue := range []uint32{0, 1, width - 1, width, width + 2} {
				shift := numeral.NewInt(int32(shiftValue))
				var result numeral.Int
				if err := evaluator.logicalShiftRight(&value, &shift, &result); err != nil {
					t.Fatal(err)
				}
				want := new(big.Int).Rsh(new(big.Int).Set(valueBig), uint(shiftValue))
				if got := slsIntegerText(t, manager, &result); got != want.String() {
					t.Fatal("lshr", width, shiftValue, got, want)
				}
				if err := evaluator.shiftLeft(&value, &shift, width, &result); err != nil {
					t.Fatal(err)
				}
				want.Lsh(new(big.Int).Set(valueBig), uint(shiftValue)).Mod(want, slsModulus(width))
				if got := slsIntegerText(t, manager, &result); got != want.String() {
					t.Fatal("shl", width, shiftValue, got, want)
				}
				if err := evaluator.arithmeticShiftRight(&value, &shift, width, &result); err != nil {
					t.Fatal(err)
				}
				want.Rsh(slsSigned(valueBig, width), uint(shiftValue)).Mod(want, slsModulus(width))
				if got := slsIntegerText(t, manager, &result); got != want.String() {
					t.Fatal("ashr", width, shiftValue, got, want)
				}
				if err := manager.Del(&result); err != nil {
					t.Fatal(err)
				}
			}
			if slsIntegerText(t, manager, &value) != valueBig.String() {
				t.Fatal("shift changed borrowed value")
			}
			if err := manager.Del(&value); err != nil {
				t.Fatal(err)
			}
		}
		if p.byShift[width] == nil || p.byShift[width-1] == nil {
			t.Fatal("shift power cache", p.byShift)
		}
		if err := p.closeChecked(); err != nil {
			t.Fatal(err)
		}
		if err := manager.CloseChecked(); err != nil {
			t.Fatal(err)
		}
		if budget.Used() != 0 {
			t.Fatal("shift leak", budget.Used())
		}
	}
}

func TestBVEvaluatorShiftLeftPowerFailurePreservesLoopPrefix(t *testing.T) {
	manager, budget := powersManager(t, 1<<29)
	p, err := newPowers(manager)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := newBVEvaluatorSlice(manager, p)
	if err != nil {
		t.Fatal(err)
	}
	value, amount, result := numeral.NewInt(7), numeral.NewInt(3), numeral.NewInt(19)
	budget.FailAfter(0)
	err = evaluator.shiftLeft(&value, &amount, 100000, &result)
	if err == nil {
		t.Fatal("large shift modulus allocation unexpectedly succeeded")
	}
	if got := slsIntegerText(t, manager, &result); got != "56" {
		t.Fatal("left-shift reached prefix", got)
	}
	if value != numeral.NewInt(7) || amount != numeral.NewInt(3) || len(p.byShift) != 0 {
		t.Fatal("failed shift changed borrowed input or published power")
	}
	budget.FailAfter(-1)
	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if budget.Used() != 0 {
		t.Fatal("failed shift leak", budget.Used())
	}
}
