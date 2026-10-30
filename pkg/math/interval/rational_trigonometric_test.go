package interval

import "testing"

func TestRationalTranscendentalSine(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	zero, half, lo, hi := f.rational("0"), f.rational("1/2"), f.rational("0"), f.rational("0")
	if err := f.owner.Sine(zero, 2, lo, hi); err != nil {
		t.Fatal(err)
	}
	if cmp, _ := f.number.Compare(lo, zero); cmp != 0 {
		t.Fatal("sin(0) lower")
	}
	if err := f.owner.Sine(half, 8, lo, hi); err != nil {
		t.Fatal(err)
	}
	want := f.rational("479425538604203/1000000000000000")
	if left, _ := f.number.Compare(lo, want); left > 0 {
		t.Fatal("sine lower exceeds reference")
	}
	if right, _ := f.number.Compare(want, hi); right > 0 {
		t.Fatal("sine upper below reference")
	}
}

func TestRationalTranscendentalCosine(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	zero, half, lo, hi := f.rational("0"), f.rational("1/2"), f.rational("0"), f.rational("0")
	if err := f.owner.Cosine(zero, 3, lo, hi); err != nil {
		t.Fatal(err)
	}
	one := f.rational("1")
	if cmp, _ := f.number.Compare(lo, one); cmp != 0 {
		t.Fatal("cos(0) lower")
	}
	if err := f.owner.Cosine(half, 7, lo, hi); err != nil {
		t.Fatal(err)
	}
	want := f.rational("877582561890373/1000000000000000")
	if left, _ := f.number.Compare(lo, want); left > 0 {
		t.Fatal("cosine lower exceeds reference")
	}
	if right, _ := f.number.Compare(want, hi); right > 0 {
		t.Fatal("cosine upper below reference")
	}
}
