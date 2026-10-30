package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestBitVectorPluginContextDispatchAndClose(t *testing.T) {
	f := newBVPluginFixture(t, 5)
	if f.plugin.FamilyID() < 0 {
		t.Fatal("null family")
	}
	if plugin, err := f.context.PluginFor(f.variable); err != nil || plugin != f.plugin {
		t.Fatalf("plugin=%T err=%v", plugin, err)
	}
	fmMust(t, f.context.Close())
	if _, err := f.plugin.Weight(f.contains, f.formula); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed plugin err=%v", err)
	}
}
