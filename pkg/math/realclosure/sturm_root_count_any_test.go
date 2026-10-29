package realclosure

import "testing"

func TestAnySturmUnboundedRootCount(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnySturmFixture(t)
			count, err := f.base.store.rootCountUnboundedAny(f.polynomial, usePrem)
			if err != nil || count != 2 {
				t.Fatal("root count", count, err)
			}
			bounded, err := f.base.store.rootCountBinaryIntervalAny(f.polynomial, f.base.interval, usePrem)
			if err != nil || bounded != 2 {
				t.Fatal("bounded root count", bounded, err)
			}
		})
	}
}
