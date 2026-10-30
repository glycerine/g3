package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextRunOPBFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "problem.opb")
	astTestMust(t, os.WriteFile(path, []byte("1 x1 >= 1; 1 ~x1 >= 1;"), 0600))
	f := newSMTAPIFixture(t, "ALL", nil)
	result, err := f.c.RunOPBFile(path, nil)
	astTestMust(t, err)
	if result.Status != lbool.False || result.Output != "unsat\n" {
		t.Fatal("OPB file result", result.Status, result.Output)
	}
}
