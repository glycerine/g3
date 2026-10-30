package realclosure

import "testing"

func TestManagerDerivedValueRelationsBothRemainderModes(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		f := newAnySturmFixture(t)
		neq, e0 := f.base.store.notEqualAnyDeterminedValues(f.alpha, f.one, usePrem)
		le, e1 := f.base.store.lessEqualAnyDeterminedValues(f.alpha, f.one, usePrem)
		ge, e2 := f.base.store.greaterEqualAnyDeterminedValues(f.alpha, f.one, usePrem)
		selfLE, e3 := f.base.store.lessEqualAnyDeterminedValues(f.alpha, f.alpha, usePrem)
		selfGE, e4 := f.base.store.greaterEqualAnyDeterminedValues(f.alpha, f.alpha, usePrem)
		if e0 != nil || e1 != nil || e2 != nil || e3 != nil || e4 != nil || !neq || le || !ge || !selfLE || !selfGE {
			t.Fatal(usePrem, neq, le, ge, selfLE, selfGE, e0, e1, e2, e3, e4)
		}
	}
}
