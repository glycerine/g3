package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestAPIOptimizeEmptyModelObservesParentContextClose(t *testing.T) {
	context, err := NewSMTContext()
	if err != nil {
		t.Fatal(err)
	}
	optimize, err := context.NewOptimize()
	if err != nil {
		t.Fatal(err)
	}
	model, err := optimize.Model()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = model.NumConstants(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
	// API objects are invalid once their parent native context is closed.
	if err = model.Close(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}
