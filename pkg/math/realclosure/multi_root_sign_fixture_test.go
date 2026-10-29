package realclosure

import "testing"

type multiRootExactFixture struct {
	f      *boundedSignFixture
	owner  *algebraicRootOwner
	roots  []*realValue
	two    *realValue
	values []*realValue
}

func newMultiRootExactFixture(t *testing.T, usePrem bool) *multiRootExactFixture {
	t.Helper()
	f := newBoundedSignFixture(t, 0, 4)
	one, minusSix, eleven := f.rational(1), f.rational(-6), f.rational(11)
	two := f.rational(2)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRootsBySigns([]*realValue{minusSix, eleven, minusSix, one}, 3, f.interval, f.interval, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &multiRootExactFixture{f: f, owner: owner, roots: roots, two: two}
	for _, rootValue := range roots {
		value, err := f.store.subtractRetainedValues(rootValue, two)
		if err != nil {
			t.Fatal(err)
		}
		fixture.values = append(fixture.values, value)
	}
	t.Cleanup(func() {
		for _, value := range fixture.values {
			if value != nil && !value.deleted {
				if err := f.store.release(value); err != nil {
					t.Error(err)
				}
			}
		}
		releaseIsolatedValues(t, f.store, fixture.roots)
	})
	return fixture
}
