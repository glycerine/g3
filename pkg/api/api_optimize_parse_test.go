package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeParseStringAndFile(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	script := "(declare-const y Int)(assert (>= y 0))(assert (<= y 3))(maximize y)(check-sat)"
	astTestMust(t, f.opt.ParseString(script))
	if n, err := f.opt.NumObjectives(); err != nil || n != 1 {
		t.Fatal("parsed objective count", n, err)
	}
	status, err := f.opt.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("parsed optimize status", status)
	}
	text, err := f.opt.SMT2()
	astTestMust(t, err)
	if !strings.Contains(text, "(maximize y)") || !strings.Contains(text, "(declare-fun y () Int)") {
		t.Fatal("optimize replay", text)
	}

	other := newAPIOptimizeFixture(t)
	path := filepath.Join(t.TempDir(), "objective.smt2")
	astTestMust(t, os.WriteFile(path, []byte("(assert true)(minimize 1)"), 0o600))
	astTestMust(t, other.opt.ParseFile(path))
	if n, err := other.opt.NumObjectives(); err != nil || n != 1 {
		t.Fatal("file objective count", n, err)
	}
}
