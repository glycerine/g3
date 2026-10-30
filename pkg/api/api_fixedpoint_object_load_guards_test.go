package api

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAPIFixedpointObjectLoadSMT2Guards(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	if err := f.fp.LoadSMT2(nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("nil reader", err)
	}
	if err := f.fp.LoadSMT2File(filepath.Join(t.TempDir(), "missing.smt2")); err == nil {
		t.Fatal("missing file loaded")
	}
	astTestMust(t, f.fp.Close())
	if err := f.fp.LoadSMT2String(""); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed string load", err)
	}
}
