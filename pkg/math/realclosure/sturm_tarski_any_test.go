package realclosure

import "testing"

func TestAnySturmTarskiQuery(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnySturmFixture(t)
			query, err := f.base.store.tarskiUnboundedAny(f.polynomial, f.query, usePrem)
			if err != nil || query != -1 {
				t.Fatal("unbounded Tarski query", query, err)
			}
			bounded, err := f.base.store.tarskiBinaryIntervalAny(f.polynomial, f.query, f.base.interval, usePrem)
			if err != nil || bounded != -1 {
				t.Fatal("bounded Tarski query", bounded, err)
			}
		})
	}
}
