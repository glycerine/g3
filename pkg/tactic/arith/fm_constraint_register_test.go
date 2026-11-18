package arith

import "testing"

func TestFMConstraintRegisterBranches(t *testing.T) {
	v := NewFMVariableTable()
	x := v.Add(false, false)
	y := v.Add(false, true)
	indexed := fmSubsumptionConstraint(nil, []uint32{x}, []int64{2}, 4, false)
	indexed.ID = 1
	if got := fmRegister(t, v, indexed); got != FMRegistrationIndexed || len(v.Index.Upper[x]) != 1 || fmText(&indexed.Coefficients[0]) != "1" || fmText(&indexed.Constant) != "2" {
		t.Fatalf("indexed registration = %v", got)
	}
	deferred := fmSubsumptionConstraint(nil, []uint32{y}, []int64{1}, 0, false)
	if got := fmRegister(t, v, deferred); got != FMRegistrationDeferred {
		t.Fatalf("forbidden registration = %v", got)
	}
	contradiction := &FMConstraint{numerals: fmTestNumerals, Constant: fmCRat(-1, 1)}
	if got := fmRegister(t, v, contradiction); got != FMRegistrationFalse || !contradiction.Dead {
		t.Fatalf("constant contradiction = %v", got)
	}
}

func TestFMConstraintFalseStrictZero(t *testing.T) {
	if !FMConstraintIsFalse(&FMConstraint{numerals: fmTestNumerals, Constant: fmCRat(0, 1), Strict: true}) || FMConstraintIsFalse(&FMConstraint{numerals: fmTestNumerals, Constant: fmCRat(0, 1)}) {
		t.Fatal("strict zero truth classification is wrong")
	}
}
