package sat

// Source: src/sat/sat_solver.cpp:2773-2779,3652-3714; src/sat/sat_params.pyg.

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

const satSolverChildDescriptorNames = "elim_blocked_clauses elim_blocked_clauses_at blocked_clause_limit resolution resolution.limit resolution.occ_cutoff resolution.occ_cutoff_range1 resolution.occ_cutoff_range2 resolution.occ_cutoff_range3 resolution.lit_cutoff_range1 resolution.lit_cutoff_range2 resolution.lit_cutoff_range3 resolution.cls_cutoff1 resolution.cls_cutoff2 elim_vars subsumption subsumption.limit asymm_branch asymm_branch.rounds asymm_branch.limit scc"

func satSolverDescriptorNames() []string {
	names := []string{}
	for _, line := range strings.Split(satMainConfigDescriptorFixture, "\n") {
		names = append(names, strings.Split(line, "\t")[0])
	}
	return append(names, strings.Fields(satSolverChildDescriptorNames)...)
}
func TestSATSolverReportParamsFiveActualDescriptorChildren(t *testing.T) {
	sy := symbolManagerTest(t, nil, false)
	d := pdnew(t, sy, nil)
	satOK(t, collectSolverParamDescrs(d))
	names := satSolverDescriptorNames()
	n, e := d.Size()
	satOK(t, e)
	if n != 45 || int(n) != len(names) {
		t.Fatal(n, len(names))
	}
	for i, want := range names {
		k, e := d.ParamName(uint32(i))
		satOK(t, e)
		got, yes, e := sy.BareString(k)
		satOK(t, e)
		if !yes || got != want {
			t.Fatal(i, got, want)
		}
	}
	satOK(t, collectSolverParamDescrs(d))
	n, e = d.Size()
	satOK(t, e)
	if n != 45 {
		t.Fatal("source duplicate insertion", n)
	}
}
func satStatusExpected(vars, elim, lits, assigned, bin, ter, cls, del uint32, inconsistent bool, avg string, mem uint64) string {
	return fmt.Sprintf("(sat-status\n  :inconsistent    %t\n  :vars            %d\n  :elim-vars       %d\n  :lits            %d\n  :assigned        %d\n  :binary-clauses  %d\n  :ternary-clauses %d\n  :clauses         %d\n  :del-clause      %d\n  :avg-clause-size %s\n  :memory          %.2f)\n", inconsistent, vars, elim, lits, assigned, bin, ter, cls, del, avg, float64(mem)/(1024*1024))
}
func TestSATSolverReportStatusActualCountsAndPersistentFormatting(t *testing.T) {
	s := satInspectionFixture(t)
	_, e := s.Lifecycle.MkClauseCore(4, []Literal{0, 2, 4, 6}, true)
	satOK(t, e)
	s.Lifecycle.deleted = 7
	o := satInspectionStream(t, arena.NewBudget(math.MaxUint64))
	mem, e := s.runtime.memory.AllocationSize()
	satOK(t, e)
	satOK(t, s.displayStatus(o))
	satInspectionText(t, o, satStatusExpected(4, 0, 9, 1, 1, 1, 1, 7, false, "3", mem))
	if state := o.State(); state.FloatMode != utext.NumericFixedFloat || state.Precision != 2 {
		t.Fatal("source manipulators persist", state)
	}
	satOK(t, o.ResetText())
	satOK(t, s.displayStatus(o))
	satInspectionText(t, o, satStatusExpected(4, 0, 9, 1, 1, 1, 1, 7, false, "3.00", mem))
	satFullSolverRuntimeAudit(t, s)
}
func TestSATSolverReportStatusIndependentBinaryWorlds(t *testing.T) {
	for mask := uint32(0); mask < 16; mask++ {
		s, _, _ := satSolverCheckFixture(t)
		for i := 0; i < 2; i++ {
			_, e := s.MkVar(false, true)
			satOK(t, e)
		}
		count := uint32(0)
		for i := uint32(0); i < 4; i++ {
			if mask&(1<<i) != 0 {
				count++
				_, e := s.Lifecycle.MkClauseCore(2, []Literal{Literal(i & 1), Literal(2 + (i >> 1))}, i&1 != 0)
				satOK(t, e)
			}
		}
		o := satInspectionStream(t, arena.NewBudget(math.MaxUint64))
		mem, e := s.runtime.memory.AllocationSize()
		satOK(t, e)
		satOK(t, s.displayStatus(o))
		avg := "0"
		if count != 0 {
			avg = "2"
		}
		satInspectionText(t, o, satStatusExpected(2, 0, 2*count, 0, count, 0, 0, 0, false, avg, mem))
		satFullSolverRuntimeAudit(t, s)
	}
}

func TestSATSolverReportStatusUsesActualRuntimeAllocationOwner(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	extra, e := s.runtime.memory.Allocate(120000)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, s.runtime.memory.Deallocate(&extra)) })
	o := satInspectionStream(t, arena.NewBudget(math.MaxUint64))
	mem, e := s.runtime.memory.AllocationSize()
	satOK(t, e)
	if mem < 120000 {
		t.Fatal("actual source Runtime sync not reached", mem)
	}
	satOK(t, s.displayStatus(o))
	satInspectionText(t, o, satStatusExpected(0, 0, 0, 0, 0, 0, 0, 0, false, "0", mem))
	roots := satFullSolverRuntimeRoots(s)
	roots = append(roots, extra)
	satScalarAuditRoots(t, s.Variables, roots...)
}

func TestSATSolverReportStatusActualEliminatedInconsistentAndExternalInspection(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 2; i++ {
		_, e := s.MkVar(false, true)
		satOK(t, e)
	}
	satOK(t, s.Variables.setEliminated(BoolVar(1), true))
	satOK(t, s.MkClause(nil))
	list, e := s.Variables.watches.Get(0)
	satOK(t, e)
	ref, e := list.ref()
	satOK(t, e)
	ext, e := newSATExternalWatch(3)
	satOK(t, e)
	satOK(t, s.Variables.watches.store.Push(ref, ext))
	o := satInspectionStream(t, arena.NewBudget(math.MaxUint64))
	mem, e := s.runtime.memory.AllocationSize()
	satOK(t, e)
	satOK(t, s.displayStatus(o))
	satInspectionText(t, o, satStatusExpected(2, 1, 0, 0, 0, 0, 0, 0, true, "0", mem))
	satFullSolverRuntimeAudit(t, s)
	// This is raw external-record inspection, not a concrete extension subclass or
	// successful EXT algorithm binding. Caller removes its own record before the
	// genuine destructor invariant, which does require a valid solver world.
	satOK(t, s.Variables.watches.store.eraseAt(ref, 0))
}
