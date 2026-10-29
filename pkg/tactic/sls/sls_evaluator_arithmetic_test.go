package sls

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func slsSetBig(t *testing.T, manager *numeral.IntegerManager, value *big.Int) numeral.Int {
	t.Helper()
	var out numeral.Int
	if err := manager.SetString(&out, value.String()); err != nil {
		t.Fatal(err)
	}
	return out
}

func slsIntegerText(t *testing.T, manager *numeral.IntegerManager, value *numeral.Int) string {
	t.Helper()
	text, err := manager.String(value)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func slsModulus(width uint32) *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), uint(width))
}

func slsModulo(value *big.Int, width uint32) *big.Int {
	return new(big.Int).Mod(new(big.Int).Set(value), slsModulus(width))
}

func TestBVEvaluatorModularArithmeticIndependentOracles(t *testing.T) {
	for _, width := range []uint32{5, 32, 80} {
		t.Run(new(big.Int).SetUint64(uint64(width)).String(), func(t *testing.T) {
			manager, budget := powersManager(t, 1<<29)
			p, err := newPowers(manager)
			if err != nil {
				t.Fatal(err)
			}
			evaluator, err := newBVEvaluatorSlice(manager, p)
			if err != nil {
				t.Fatal(err)
			}

			modulus := slsModulus(width)
			aBig := new(big.Int).Sub(new(big.Int).Set(modulus), big.NewInt(3))
			bBig := new(big.Int).Rsh(new(big.Int).Set(modulus), 1)
			bBig.Add(bBig, big.NewInt(7)).Mod(bBig, modulus)
			cBig := big.NewInt(11)
			values := []numeral.Int{slsSetBig(t, manager, aBig), slsSetBig(t, manager, bBig), slsSetBig(t, manager, cBig)}
			args := []*numeral.Int{&values[0], &values[1], &values[2]}
			before := []string{aBig.String(), bBig.String(), cBig.String()}
			var result numeral.Int

			if err := evaluator.add(args, width, &result); err != nil {
				t.Fatal(err)
			}
			want := new(big.Int).Add(aBig, bBig)
			want.Add(want, cBig).Mod(want, modulus)
			if got := slsIntegerText(t, manager, &result); got != want.String() {
				t.Fatalf("add width %d = %s, want %s", width, got, want)
			}

			if err := evaluator.sub(&values[1], &values[0], width, &result); err != nil {
				t.Fatal(err)
			}
			want.Sub(bBig, aBig).Mod(want, modulus)
			if got := slsIntegerText(t, manager, &result); got != want.String() {
				t.Fatalf("sub width %d = %s, want %s", width, got, want)
			}

			if err := evaluator.mul(args, width, &result); err != nil {
				t.Fatal(err)
			}
			want.Mul(aBig, bBig).Mul(want, cBig).Mod(want, modulus)
			if got := slsIntegerText(t, manager, &result); got != want.String() {
				t.Fatalf("mul width %d = %s, want %s", width, got, want)
			}

			if err := evaluator.neg(&values[0], width, &result); err != nil {
				t.Fatal(err)
			}
			want.Neg(aBig).Mod(want, modulus)
			if got := slsIntegerText(t, manager, &result); got != want.String() {
				t.Fatalf("neg width %d = %s, want %s", width, got, want)
			}

			for i := range values {
				if got := slsIntegerText(t, manager, &values[i]); got != before[i] {
					t.Fatalf("borrowed operand %d changed from %s to %s", i, before[i], got)
				}
			}
			if p.byShift[width] == nil || len(p.byShift) != 1 {
				t.Fatalf("modular family cache = %v", p.byShift)
			}

			if err := manager.Del(&result); err != nil {
				t.Fatal(err)
			}
			for i := range values {
				if err := manager.Del(&values[i]); err != nil {
					t.Fatal(err)
				}
			}
			if err := p.closeChecked(); err != nil {
				t.Fatal(err)
			}
			if err := manager.CloseChecked(); err != nil {
				t.Fatal(err)
			}
			if budget.Used() != 0 {
				t.Fatal("modular family leak", budget.Used())
			}
		})
	}
}

func TestBVEvaluatorUnsignedDivisionRemainderAndZeroSemantics(t *testing.T) {
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
	leftBig := new(big.Int).Sub(slsModulus(width), big.NewInt(12345))
	rightBig := big.NewInt(97)
	left, right := slsSetBig(t, manager, leftBig), slsSetBig(t, manager, rightBig)
	zero := numeral.NewInt(0)
	var result numeral.Int

	if err := evaluator.unsignedDiv(&left, &right, width, &result); err != nil {
		t.Fatal(err)
	}
	want := new(big.Int).Quo(leftBig, rightBig)
	if got := slsIntegerText(t, manager, &result); got != want.String() {
		t.Fatalf("udiv = %s, want %s", got, want)
	}
	if len(p.byShift) != 0 {
		t.Fatal("ordinary unsigned division requested a power", p.byShift)
	}

	if err := evaluator.unsignedRem(&left, &right, &result); err != nil {
		t.Fatal(err)
	}
	want.Rem(leftBig, rightBig)
	if got := slsIntegerText(t, manager, &result); got != want.String() {
		t.Fatalf("urem = %s, want %s", got, want)
	}

	if err := evaluator.unsignedDiv(&left, &zero, width, &result); err != nil {
		t.Fatal(err)
	}
	want.Sub(slsModulus(width), big.NewInt(1))
	if got := slsIntegerText(t, manager, &result); got != want.String() {
		t.Fatalf("udiv by zero = %s, want %s", got, want)
	}
	if p.byShift[width] == nil || len(p.byShift) != 1 {
		t.Fatal("zero divisor did not request exactly its result power", p.byShift)
	}

	if err := evaluator.unsignedRem(&left, &zero, &result); err != nil {
		t.Fatal(err)
	}
	if got := slsIntegerText(t, manager, &result); got != leftBig.String() {
		t.Fatalf("urem by zero = %s, want dividend", got)
	}
	if slsIntegerText(t, manager, &left) != leftBig.String() || slsIntegerText(t, manager, &right) != rightBig.String() {
		t.Fatal("unsigned operation changed borrowed operands")
	}

	for _, value := range []*numeral.Int{&result, &left, &right} {
		if err := manager.Del(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if budget.Used() != 0 {
		t.Fatal("unsigned family leak", budget.Used())
	}
}

func TestBVEvaluatorAddPowerMissFailurePreservesReachedPrefix(t *testing.T) {
	manager, budget := powersManager(t, 1<<29)
	p, err := newPowers(manager)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := newBVEvaluatorSlice(manager, p)
	if err != nil {
		t.Fatal(err)
	}
	a, b, result := numeral.NewInt(5), numeral.NewInt(7), numeral.NewInt(19)
	budget.FailAfter(0)
	err = evaluator.add([]*numeral.Int{&a, &b}, 100000, &result)
	if err == nil {
		t.Fatal("large cached power allocation unexpectedly succeeded")
	}
	if got := slsIntegerText(t, manager, &result); got != "12" {
		t.Fatalf("source prefix before failed power lookup = %s", got)
	}
	if len(p.byShift) != 0 || a != numeral.NewInt(5) || b != numeral.NewInt(7) {
		t.Fatal("failed miss published or changed borrowed inputs")
	}
	budget.FailAfter(-1)
	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if budget.Used() != 0 {
		t.Fatal("failed power miss leak", budget.Used())
	}
}

func TestBVEvaluatorUnsignedDivCopyFailureCleansSourceLocals(t *testing.T) {
	for _, ordinal := range []int64{0, 1} {
		manager, budget := powersManager(t, 1<<29)
		p, err := newPowers(manager)
		if err != nil {
			t.Fatal(err)
		}
		evaluator, err := newBVEvaluatorSlice(manager, p)
		if err != nil {
			t.Fatal(err)
		}
		one := numeral.NewInt(1)
		var left, right, result numeral.Int
		if err := manager.Mul2k(&one, 40000, &left); err != nil {
			t.Fatal(err)
		}
		if err := manager.Mul2k(&one, 39999, &right); err != nil {
			t.Fatal(err)
		}
		budget.FailAfter(ordinal)
		err = evaluator.unsignedDiv(&left, &right, 40001, &result)
		if err == nil {
			t.Fatalf("copy refusal ordinal %d unexpectedly succeeded", ordinal)
		}
		budget.FailAfter(-1)
		leftPower, leftShift, leftErr := manager.IsPowerOfTwo(&left)
		rightPower, rightShift, rightErr := manager.IsPowerOfTwo(&right)
		if leftErr != nil || rightErr != nil || !leftPower || !rightPower || leftShift != 40000 || rightShift != 39999 {
			t.Fatalf("copy refusal ordinal %d changed borrowed inputs", ordinal)
		}
		if err := manager.Del(&result); err != nil {
			t.Fatal(err)
		}
		if err := manager.Del(&left); err != nil {
			t.Fatal(err)
		}
		if err := manager.Del(&right); err != nil {
			t.Fatal(err)
		}
		if err := p.closeChecked(); err != nil {
			t.Fatal(err)
		}
		if err := manager.CloseChecked(); err != nil {
			t.Fatal(err)
		}
		if budget.Used() != 0 {
			t.Fatalf("copy refusal ordinal %d leak: %d", ordinal, budget.Used())
		}
	}
}
