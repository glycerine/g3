package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIParserFileUsesSameOneShotPath(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	path := filepath.Join(t.TempDir(), "formula.smt2")
	astTestMust(t, os.WriteFile(path, []byte("(assert false)"), 0o600))
	parsed, err := f.c.ParseSMT2File(path, SMT2Bindings{})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&parsed)) }()
	formula, err := f.m.RefGet(parsed)
	astTestMust(t, err)
	value, err := f.c.BooleanValue(formula)
	astTestMust(t, err)
	if value != lbool.False {
		t.Fatal("file parser result", value)
	}
	if _, err = f.c.ParseSMT2File(filepath.Join(t.TempDir(), "missing.smt2"), SMT2Bindings{}); err == nil {
		t.Fatal("missing parser file accepted")
	}
}
