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

// AUDIT api_02-F2: Z3_optimize_from_file dispatches on get_extension and
// loads .opb and .wcnf files with parse_opb / parse_wcnf. The counts are
// what build/libz3.so reports for the same files.
func TestAPIOptimizeParseFileDispatchesOPBAndWCNF(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		astTestMust(t, os.WriteFile(path, []byte(text), 0o600))
		return path
	}
	counts := func(o *APIOptimize, label string, assertions int, objectives uint32) {
		t.Helper()
		a, err := o.Assertions()
		astTestMust(t, err)
		n, err := o.NumObjectives()
		astTestMust(t, err)
		if len(a) != assertions || n != objectives {
			t.Fatalf("%s: assertions=%d objectives=%d, want %d and %d", label, len(a), n, assertions, objectives)
		}
	}
	opb := write("p.opb", "min: 1 x1 +2 x2;\n1 x1 +1 x2 >= 1;\n")
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.ParseFile(opb))
	counts(f.opt, "opb", 1, 1)
	if status, err := f.opt.Check(); err != nil || status != lbool.True {
		t.Fatal("opb check", status, err)
	}

	hard := newAPIOptimizeFixture(t)
	astTestMust(t, hard.opt.ParseFile(write("w.wcnf", "p wcnf 1 1 5\n5 1 0\n")))
	counts(hard.opt, "wcnf top-weight clause", 1, 0)

	soft := newAPIOptimizeFixture(t)
	astTestMust(t, soft.opt.ParseFile(write("w2.wcnf", "p wcnf 2 3 10\n10 1 2 0\n3 -1 0\n4 -2 0\n")))
	counts(soft.opt, "wcnf", 1, 1)
	if objective, err := soft.opt.Objective(0); err != nil || !objective.Soft {
		t.Fatal("wcnf soft objective", objective, err)
	}
	if status, err := soft.opt.Check(); err != nil || status != lbool.True {
		t.Fatal("wcnf check", status, err)
	}
}
