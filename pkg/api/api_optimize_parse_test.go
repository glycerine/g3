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

func TestAPIOptimizeParsedCommandsUseSuppliedOptimizer(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	other := newAPIOptimizeFixture(t)
	_, err := f.opt.Minimize(f.x)
	astTestMust(t, err)
	astTestMust(t, f.opt.ParseString("(declare-const p Bool)(assert-soft p :weight 2 :id group)(maximize 3)"))
	if n, err := f.opt.NumObjectives(); err != nil || n != 3 {
		t.Fatal("direct and parsed objectives", n, err)
	}
	if n, err := other.opt.NumObjectives(); err != nil || n != 0 {
		t.Fatal("another optimizer changed", n, err)
	}
	if objective, err := f.opt.Objective(1); err != nil || !objective.Soft || objective.ID != "group" {
		t.Fatal("parsed soft objective", objective, err)
	}
	if objective, err := f.opt.Objective(2); err != nil || !objective.Maximize || objective.Soft {
		t.Fatal("parsed maximize objective", objective, err)
	}
}
