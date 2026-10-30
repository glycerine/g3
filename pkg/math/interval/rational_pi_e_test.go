package interval

import "testing"

func TestRationalTranscendentalPiAndCache(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	pi := f.interval()
	if err := f.owner.Pi(4, pi); err != nil {
		t.Fatal(err)
	}
	if !f.encloses(pi, "3141592653589793/1000000000000000") {
		t.Fatal("BBP interval does not enclose pi")
	}
	if err := f.owner.SetPiPrecision(3); err != nil {
		t.Fatal(err)
	}
	old := f.owner.pi
	if err := f.owner.SetPiAtLeastPrecision(2); err != nil || f.owner.pi != old || f.owner.PiPrecision() != 3 {
		t.Fatal("at-least cache unexpectedly refreshed", err)
	}
	if err := f.owner.SetPiAtLeastPrecision(5); err != nil || f.owner.pi == old || f.owner.PiPrecision() != 5 {
		t.Fatal("at-least cache did not refresh", err)
	}
	if !f.encloses(f.owner.piHalf, "15707963267948966/10000000000000000") || !f.encloses(f.owner.twoPi, "6283185307179586/1000000000000000") {
		t.Fatal("scaled pi caches")
	}
}

func TestRationalTranscendentalE(t *testing.T) {
	f := newRationalTranscendentalFixture(t)
	defer f.close()
	e := f.interval()
	if err := f.owner.E(10, e); err != nil {
		t.Fatal(err)
	}
	if !f.encloses(e, "2718281828459045/1000000000000000") {
		t.Fatal("factorial interval does not enclose e")
	}
}
