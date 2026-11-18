package numeral

// Added exact format expectations.

import (
	"errors"
	"testing"
)

func TestBinaryFormattingExact(t *testing.T) {
	m, _ := binaryManager(t)
	a := NewBinaryIntPower(-3, 3)
	cases := []struct {
		kind int
		want string
	}{{0, "-3/2^3"}, {1, "-3/2<sup>3</sup>"}, {2, "(/ (- 3) (^ 2 3))"}, {3, "(/ (- 3.0) (^ 2.0 3.0))"}, {4, "-0.375"}, {5, "-0.3?"}}
	for _, c := range cases {
		var got string
		var e error
		switch c.kind {
		case 0:
			got, e = m.String(&a)
		case 1:
			got, e = m.PrettyString(&a)
		case 2:
			got, e = m.SMT2String(&a, false)
		case 3:
			got, e = m.SMT2String(&a, true)
		case 4:
			got, e = m.DecimalString(&a, 8)
		case 5:
			got, e = m.DecimalString(&a, 1)
		}
		if e != nil || got != c.want {
			t.Fatal(c, got, e)
		}
	}
	dst := []byte("xxx")
	n, e := m.FormatInto(&a, dst)
	if !errors.Is(e, ErrShortBuffer) || n != 6 || string(dst) != "xxx" {
		t.Fatal(n, e, string(dst))
	}
}

func TestBinaryFormattingBranchesAndFaultPublication(t *testing.T) {
	m, budget := binaryManager(t)
	a := NewBinaryIntPower(2, 1)
	got, e := m.DecimalString(&a, 8)
	if SourceExtendedGCDDebug {
		if !errors.Is(e, ErrBinaryInvariant) {
			t.Fatal(e)
		}
		// Raw mpz locals do not execute the end label when the source assertion
		// stops this invocation. Audit that prefix, then explicitly abort ownership.
		binaryAudit(t, m, &a)
		serviceMust(t, m.CleanupDisplay())
	} else if e != nil || got != "1.0" {
		t.Fatal(got, e)
	}
	a = NewBinaryIntPower(1, 1)
	got, e = m.DecimalString(&a, 0)
	if e != nil || got != "0.?" {
		t.Fatal(got, e)
	}
	a = NewBinaryInt(3)
	got, e = m.DecimalStringDefault(&a)
	if e != nil || got != "3" {
		t.Fatal(got, e)
	}
	// Input with many digits forces text and formatting scratch growth after
	// short forms above; publication into a caller buffer must remain atomic.
	serviceMust(t, m.Z.SetString(&a.Num, "100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"))
	a.KField = 1000
	dst := []byte("preserve caller bytes")
	budget.FailAfter(0)
	_, e = m.FormatDecimalInto(&a, 1000, dst)
	budget.FailAfter(-1)
	if e == nil || string(dst) != "preserve caller bytes" {
		t.Fatal(e, string(dst))
	}
	binaryAudit(t, m, &a)
	serviceMust(t, m.Del(&a))
}
