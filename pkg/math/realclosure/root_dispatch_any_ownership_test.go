package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestAnyRootDispatchRejectsForeignCoefficientWithoutConsumingInputs(t *testing.T) {
	f := newAnySturmFixture(t)
	foreign := newBoundedSignFixture(t, -1, 1)
	foreignOne := foreign.rational(1)
	owner := &algebraicRootOwner{values: f.base.store, intervals: f.base.bounds}
	before := f.alpha.refs
	if roots, err := owner.isolateAnyRoots([]*realValue{f.alpha, foreignOne}, true); !errors.Is(err, arena.ErrInvalidSlot) || roots != nil {
		t.Fatal(roots, err)
	}
	if f.alpha.refs != before || f.alpha.deleted {
		t.Fatal("failed dispatch consumed source coefficient", before, f.alpha.refs)
	}
}
