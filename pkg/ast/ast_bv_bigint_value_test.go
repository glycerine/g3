package ast

import (
	"math/big"
	"testing"
)

func TestBitVectorNumeralBigIntExactWideRead(t *testing.T) {
	m := astTestManager(t)
	if _, err := m.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.RegisterBitVectorPlugin(); err != nil {
		t.Fatal(err)
	}
	wide, err := m.BitVectorNumeralAllOnes(70)
	if err != nil {
		t.Fatal(err)
	}
	got, width, matched, err := m.BitVectorNumeralBigInt(wide)
	want := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 70), big.NewInt(1))
	if err != nil || !matched || width != 70 || got.Cmp(want) != 0 {
		t.Fatalf("wide numeral: %v width=%d matched=%v err=%v", got, width, matched, err)
	}
	boolean, err := m.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	if value, _, found, e := m.BitVectorNumeralBigInt(boolean); e != nil || found || value != nil {
		t.Fatalf("non-numeral: %v %v %v", value, found, e)
	}
}
