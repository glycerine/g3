package realclosure

import "testing"

func TestRestrictBinaryIntervalToCertifiedSign(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	for _, sign := range []int32{-1, 1} {
		interval, err := NewBinaryInterval(f.bounds)
		if err != nil {
			t.Fatal(err)
		}
		if err := interval.SetFrom(f.interval); err != nil {
			t.Fatal(err)
		}
		if err := restrictBinaryIntervalToSign(interval, sign); err != nil {
			t.Fatal(err)
		}
		contains, err := interval.ContainsZero()
		if err != nil || contains {
			t.Fatal(sign, contains, err)
		}
		positive, err := interval.IsP()
		if err != nil || positive != (sign > 0) {
			t.Fatal(sign, positive, err)
		}
		if err := interval.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
