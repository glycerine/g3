package spacer

// Source: src/muz/spacer/spacer_virtual_solver.cpp timed SMT-LIB benchmark
// dumping and optional local recheck.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestVirtualSolverTimedBenchmarkDumpAndRecheck(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	backend, err := smtcore.NewSMTSolver(manager, nil, "QF_UF", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	rechecks := 0
	config := VirtualSolverDumpConfig{Enabled: true, MinSeconds: -1, Recheck: true, OutputDirectory: directory}
	config.recheck = func(assertions, assumptions []ast.ASTHandle) (lbool.LBool, error) {
		rechecks++
		if len(assertions) != 1 || len(assumptions) != 1 {
			t.Fatalf("dump recheck inputs: assertions=%d assumptions=%d", len(assertions), len(assumptions))
		}
		return lbool.True, nil
	}
	factory, err := NewVirtualSolverFactoryWithDump(backend, config)
	if err != nil {
		t.Fatal(err)
	}
	virtual, err := factory.Fresh()
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.Symbols.Intern("dump-p")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := manager.MkConst(name, boolean)
	if err != nil {
		t.Fatal(err)
	}
	if err = virtual.Assert(predicate); err != nil {
		t.Fatal(err)
	}
	status, err := virtual.CheckSat(nil)
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	if rechecks != 1 {
		t.Fatal("benchmark was not rechecked", rechecks)
	}
	path := filepath.Join(directory, "virt_solver_vsolver#0_0.smt2")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, required := range []string{"(set-info :status sat)", "dump-p", "vsolver#0", "(check-sat", "(exit)"} {
		if !strings.Contains(text, required) {
			t.Fatalf("benchmark lacks %q:\n%s", required, text)
		}
	}
	if err = virtual.Close(); err != nil {
		t.Fatal(err)
	}
	if err = factory.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestVirtualSolverDumpThresholdParamsAndFailurePrefix(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	if err := globals.SourceSetName("fixedpoint.spacer.vs.dump_benchmarks", "true"); err != nil {
		t.Fatal(err)
	}
	if err := globals.SourceSetName("fixedpoint.spacer.vs.dump_min_time", "-1"); err != nil {
		t.Fatal(err)
	}
	if err := globals.SourceSetName("fixedpoint.spacer.vs.recheck", "true"); err != nil {
		t.Fatal(err)
	}
	contexts, err := NewSMTContextManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	if !contexts.dump.Enabled || contexts.dump.MinSeconds != -1 || !contexts.dump.Recheck {
		t.Fatal("fixedpoint dump parameters", contexts.dump)
	}
	contexts.dump.OutputDirectory = filepath.Join(t.TempDir(), "missing", "child")
	virtual, err := contexts.Fresh()
	if err != nil {
		t.Fatal(err)
	}
	status, err := virtual.CheckSat(nil)
	if err == nil || status != lbool.True || !errors.Is(err, os.ErrNotExist) {
		t.Fatal("dump creation failure prefix", status, err)
	}
	stats := statistics.New()
	if collectErr := contexts.CollectStatistics(stats); collectErr != nil {
		t.Fatal(collectErr)
	}
	foundCheck := false
	for index := 0; index < stats.Size(); index++ {
		if stats.IsUint(index) && stats.Key(index) == "virtual_solver.checks" && stats.UintValue(index) == 1 {
			foundCheck = true
		}
	}
	if !foundCheck {
		t.Fatal("failed dump erased completed solver check")
	}
	if err = virtual.Close(); err != nil {
		t.Fatal(err)
	}
	if err = contexts.Close(); err != nil {
		t.Fatal(err)
	}
}
