package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestExactSingleRootSignRejectsSharedSignExtension(t *testing.T) {
	f := newBoundedSignFixture(t, 0, 4)
	one, minusSix, eleven := f.rational(1), f.rational(-6), f.rational(11)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	values, err := owner.isolateRationalRootsBySigns([]*realValue{minusSix, eleven, minusSix, one}, 3, f.interval, f.interval, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.exactSingleRootPolynomialSign(values[0], true); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("multi-root sign extension accepted", err)
	}
	releaseIsolatedValues(t, f.store, values)
}
