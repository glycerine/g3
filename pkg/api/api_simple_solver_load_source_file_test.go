package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISimpleSolverLoadFileDispatchesDIMACSAndSMT2(t *testing.T) {
	for _, row := range []struct{ name, suffix, input string }{
		{"dimacs", ".dimacs", "p cnf 1 2\n1 0\n-1 0\n"},
		{"smt2", ".smt2", "(assert false)"},
	} {
		t.Run(row.name, func(t *testing.T) {
			f := newSMTAPIFixture(t, "QF_UF", nil)
			s, err := f.c.NewSimpleSolver()
			astTestMust(t, err)
			path := filepath.Join(t.TempDir(), "input"+row.suffix)
			astTestMust(t, os.WriteFile(path, []byte(row.input), 0o600))
			astTestMust(t, s.LoadFile(path))
			if status, checkErr := s.Check(); checkErr != nil || status != lbool.False {
				t.Fatal("status", status, checkErr)
			}
			astTestMust(t, s.Close())
		})
	}
}
