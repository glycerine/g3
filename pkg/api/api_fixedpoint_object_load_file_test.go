package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAPIFixedpointObjectLoadSMT2File(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	path := filepath.Join(t.TempDir(), "fixedpoint.smt2")
	astTestMust(t, os.WriteFile(path, []byte(`(declare-rel file-p ()) (rule (! file-p :named file-rule))`), 0o600))
	astTestMust(t, f.fp.LoadSMT2File(path))
	rules, err := f.fp.Rules()
	astTestMust(t, err)
	if len(rules) != 1 {
		t.Fatal("file rules", len(rules))
	}
	releaseAPIRefs(t, f.m, rules)
}
