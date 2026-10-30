package arith

import "testing"

func TestFMConstraintRegisterBranches(t *testing.T) {
	v := NewFMVariableTable()
	x := v.Add(false, false)
	y := v.Add(false, true)
	indexed := fmSubsumptionConstraint(nil, []uint32{x}, []int64{2}, 4, false)
	indexed.ID = 1
	if got := v.Register(indexed); got != FMRegistrationIndexed || len(v.Index.Upper[x]) != 1 || indexed.Coefficients[0].RatString() != "1" || indexed.Constant.RatString() != "2" {
		t.Fatalf("indexed registration = %v", got)
	}
	deferred := fmSubsumptionConstraint(nil, []uint32{y}, []int64{1}, 0, false)
	if got := v.Register(deferred); got != FMRegistrationDeferred {
		t.Fatalf("forbidden registration = %v", got)
	}
	contradiction := &FMConstraint{Constant: fmCRat(-1, 1)}
	if got := v.Register(contradiction); got != FMRegistrationFalse || !contradiction.Dead {
		t.Fatalf("constant contradiction = %v", got)
	}
}

func TestFMConstraintFalseStrictZero(t *testing.T) {
	if !FMConstraintIsFalse(&FMConstraint{Constant: fmCRat(0, 1), Strict: true}) || FMConstraintIsFalse(&FMConstraint{Constant: fmCRat(0, 1)}) {
		t.Fatal("strict zero truth classification is wrong")
	}
}
