package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverLoadSMT2File(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	path := filepath.Join(t.TempDir(), "solver-input.smt2")
	astTestMust(t, os.WriteFile(path, []byte("(assert false)"), 0o600))
	astTestMust(t, s.LoadSMT2File(path))
	if status, checkErr := s.Check(); checkErr != nil || status != lbool.False {
		t.Fatal("status", status, checkErr)
	}
	if err = s.LoadSMT2File(filepath.Join(t.TempDir(), "missing.smt2")); err == nil {
		t.Fatal("missing file accepted")
	}
	astTestMust(t, s.Close())
}
