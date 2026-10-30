package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestAlgebraicRegistryDeletedRootLosesIndex(t *testing.T) {
	f := newRankedValueFixture(t, true)
	value := newRegistryTestValue(t, f)
	root := value.root
	if err := f.base.store.release(value); err != nil {
		t.Fatal(err)
	}
	if _, err := root.registryIndex(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal(err)
	}
}
