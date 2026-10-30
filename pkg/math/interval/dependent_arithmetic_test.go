package interval

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDependentIntervalArithmeticCombinesSourceBounds(t *testing.T) {
	f := newDependentIntervalFixture(t)
	a := f.interval("1", "2", false, false, 1, 2)
	b := f.interval("3", "4", false, false, 3, 4)
	result := f.empty()
	if err := result.Add(a, b, true); err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, result); got != "[4,6] ld 1 3 ud 2 4" {
		t.Fatal(got)
	}
	if err := result.Mul(a, b, true); err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, result); got != "[3,8] ld 1 3 ud 4 1 2" {
		t.Fatal(got)
	}
	negative := f.rational("-2")
	if err := result.MulScalar(&negative, a, true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.LowerDependencies(), []uint32{2}) || !reflect.DeepEqual(result.UpperDependencies(), []uint32{1}) {
		t.Fatal(result.LowerDependencies(), result.UpperDependencies())
	}
	if err := result.Power(a, 2, true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.LowerDependencies(), []uint32{1}) || !reflect.DeepEqual(result.UpperDependencies(), []uint32{1, 2}) {
		t.Fatal(result.LowerDependencies(), result.UpperDependencies())
	}
}

func resultText(t *testing.T, v *DependentRationalInterval) string {
	t.Helper()
	var out bytes.Buffer
	if err := v.Display(&out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
