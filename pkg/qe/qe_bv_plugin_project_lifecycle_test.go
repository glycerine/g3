package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestBitVectorPluginProjectModelRejectsClosedPlugin(t *testing.T) {
	f := newBVModelProjectFixture(t, 8, 4)
	fmMust(t, f.plugin.Close())
	if projected, err := f.plugin.ProjectModel(f.contains, f.model, f.formula, f.base.r.config); projected || !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed plugin accepted", projected, err)
	}
}
