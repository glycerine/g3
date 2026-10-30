package interval

import (
	"reflect"
	"testing"
)

func TestDependentIntervalMulScalarUsesSourceDependencyMasks(t *testing.T) {
	f := newDependentIntervalFixture(t)
	a := f.interval("1", "2", false, false, 11, 22)
	result := f.interval("7", "8", false, false, 77, 88)

	positive := f.rational("3")
	if err := result.MulScalar(&positive, a, true); err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, result); got != "[3,6] ld 11 ud 22" {
		t.Fatal("positive scalar:", got)
	}

	negative := f.rational("-3")
	if err := result.MulScalar(&negative, a, true); err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, result); got != "[-6,-3] ld 22 ud 11" {
		t.Fatal("negative scalar:", got)
	}

	zero := f.rational("0")
	if err := result.MulScalar(&zero, a, true); err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, result); got != "(-oo,oo)" {
		t.Fatal("zero scalar:", got)
	}
	if result.LowerDep != nil || result.UpperDep != nil {
		t.Fatal("zero scalar retained dependencies", result.LowerDependencies(), result.UpperDependencies())
	}
}

func TestDependentIntervalMulScalarSourceMasksSurviveAliasing(t *testing.T) {
	f := newDependentIntervalFixture(t)
	a := f.interval("-2", "5", true, false, 3, 4)

	negative := f.rational("-2")
	if err := a.MulScalar(&negative, a, true); err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, a); got != "[-10,4) ld 4 ud 3" {
		t.Fatal("aliased negative scalar:", got)
	}

	zero := f.rational("0")
	if err := a.MulScalar(&zero, a, true); err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, a); got != "(-oo,oo)" {
		t.Fatal("aliased zero scalar:", got)
	}
	if a.LowerDep != nil || a.UpperDep != nil {
		t.Fatal("aliased zero retained dependencies")
	}
}

func TestDependentIntervalMulScalarWithoutDependenciesLeavesPublishedState(t *testing.T) {
	f := newDependentIntervalFixture(t)
	a := f.interval("1", "2", false, false, 1, 2)
	result := f.interval("7", "8", false, false, 77, 88)
	beforeLower, beforeUpper := result.LowerDependencies(), result.UpperDependencies()

	zero := f.rational("0")
	if err := result.MulScalar(&zero, a, false); err != nil {
		t.Fatal(err)
	}
	if got := resultText(t, result); got != "(-oo,oo) ld 77 ud 88" {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(result.LowerDependencies(), beforeLower) || !reflect.DeepEqual(result.UpperDependencies(), beforeUpper) {
		t.Fatal("dependency-disabled multiplication changed published state")
	}
}
