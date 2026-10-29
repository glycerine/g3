package api

import "testing"

func TestAPIAlgebraicRootAndPower(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	root := f.sqrt2(t)
	squared, err := f.c.AlgebraicPower(root, 2)
	astTestMust(t, err)
	squared = f.pin(squared, nil)
	if text, textErr := f.c.NumeralString(squared); textErr != nil || text != "2" {
		t.Fatal("algebraic square", text, textErr)
	}
	cubeRoot, err := f.c.AlgebraicRoot(f.three, 3)
	astTestMust(t, err)
	cubeRoot = f.pin(cubeRoot, nil)
	cubed, err := f.c.AlgebraicPower(cubeRoot, 3)
	astTestMust(t, err)
	cubed = f.pin(cubed, nil)
	if equal, compareErr := f.c.AlgebraicEqual(cubed, f.three); compareErr != nil || !equal {
		t.Fatal("odd root/power", equal, compareErr)
	}
	identity, err := f.c.AlgebraicPower(root, 0)
	astTestMust(t, err)
	identity = f.pin(identity, nil)
	if text, textErr := f.c.NumeralString(identity); textErr != nil || text != "1" {
		t.Fatal("zero power", text, textErr)
	}
}
