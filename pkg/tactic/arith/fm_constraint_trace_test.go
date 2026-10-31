package arith

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func fmConstraintRecord(tag, function string, line int, body string) string {
	return fmt.Sprintf("-------- [%s] %s src/tactic/arith/fm_tactic.cpp:%d ---------\n%s------------------------------------------------\n", tag, function, line, body)
}
func TestFMConstraintTraceContradictionSourceSequence(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("fm")
	tags.Enable("fm_bug")
	equationMust(t, fm.BindTrace(tags))
	x := fixDLConst(t, m, false, "x")
	g := fixDLGoal(t, m, false, false, false, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "1")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "0")))
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	want := fmConstraintRecord("fm", "operator()", 1577, "x\n  (-1*x <= -1)\n  (x <= 0)\n") +
		fmConstraintRecord("fm", "sort_candidates", 1173, "(x 1) \n") +
		fmConstraintRecord("fm_bug", "try_eliminate", 1479, "eliminating x\nlowers:\n  (-1*x <= -1)\nuppers:\n  (x <= 0)\n") +
		fmConstraintRecord("fm", "resolve", 1420, "resolution 0 inconsistent: \n(-1*x <= -1)\n(x <= 0)\n") +
		fmConstraintRecord("fm", "try_eliminate", 1514, "variables was eliminated old: 2 new_constraints: 0\n") +
		fmConstraintRecord("fm", "operator()", 1611, "(goal\n  false)\n")
	if tags.ShouldTrace("fm") {
		if out.String() != want {
			t.Fatalf("source trace\ngot %q\nwant %q", out.String(), want)
		}
	} else if out.Len() != 0 {
		t.Fatal("erased trace output", out.String())
	}
}
func TestFMConstraintTraceSubsumptionAndTrivialElimination(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("fm")
	tags.Enable("fm_subsumption")
	equationMust(t, fm.BindTrace(tags))
	x := fixDLConst(t, m, false, "x")
	g := fixDLGoal(t, m, false, false, false, fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "2")))
	equationMust(t, fm.Apply(g, fmResult(t)))
	if tags.ShouldTrace("fm") {
		for _, record := range []string{fmConstraintRecord("fm_subsumption", "backward_subsumption", 749, "(x <= 2)\nsubsumed:\n(x <= 4)\n"), fmConstraintRecord("fm", "sort_candidates", 1173, "(x 0) \n"), fmConstraintRecord("fm", "try_eliminate", 1461, "variables was eliminated (trivial case)\n")} {
			if !strings.Contains(out.String(), record) {
				t.Fatal("missing source record", record, out.String())
			}
		}
	}
}
func TestFMConstraintTraceDeferredResolventAndDependencyAddress(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("fm")
	equationMust(t, fm.BindTrace(tags))
	x := fixDLConst(t, m, false, "x")
	y := fixDLConst(t, m, false, "y")
	pred := fmPredicate(t, m, y)
	g := fixDLGoal(t, m, false, false, true, pred, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "0")), fixDLApp(t, m, 2, fixDLApp(t, m, 6, x, y), fixDLNumber(t, m, false, "3")))
	boolean, err := m.MkBoolSort()
	equationMust(t, err)
	name, err := m.Symbols.Intern("premise")
	equationMust(t, err)
	label, err := m.MkConst(name, boolean)
	equationMust(t, err)
	fixDLPin(t, m, label)
	dep, err := m.DependencyLeaf(label)
	equationMust(t, err)
	form, err := g.Form(1)
	equationMust(t, err)
	equationMust(t, g.Update(1, form, ast.ASTHandle{}, dep))
	address, err := m.DependencyAddressText(dep)
	equationMust(t, err)
	equationMust(t, fm.Apply(g, fmResult(t)))
	if tags.ShouldTrace("fm") {
		record := fmConstraintRecord("fm", "resolve", 1438, "resolution 0\n(-1*x <= 0)\n(x + y <= 3)\n---->\n(y <= 3)\nnew_dep: "+address+"\n")
		if !strings.Contains(out.String(), record) {
			t.Fatal("missing resolution/address record", record, out.String())
		}
	}
}

func TestFMConstraintTraceResolutionTautologyOutcomes(t *testing.T) {
	for _, booleanTautology := range []bool{false, true} {
		t.Run(fmt.Sprint(booleanTautology), func(t *testing.T) {
			fm, m := fmTacticFixture(t, false)
			var out bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&out)
			tags.Enable("fm")
			equationMust(t, fm.BindTrace(tags))
			x := fixDLConst(t, m, false, "x")
			lower := fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "2"))
			upper := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4"))
			line := 1366
			body := "resolution 0 consequent literal is always true: \n(-1*x <= -2)\n(x <= 4)\n"
			if booleanTautology {
				name := "fm_occ"
				equationMust(t, fm.context.SetBoolName(&fm.params, &name, true))
				equationMust(t, fm.UpdateParams(&fm.params))
				boolean, err := m.MkBoolSort()
				equationMust(t, err)
				sym, err := m.Symbols.Intern("p")
				equationMust(t, err)
				p, err := m.MkConst(sym, boolean)
				equationMust(t, err)
				fixDLPin(t, m, p)
				neg, err := m.MkNot(p)
				equationMust(t, err)
				fixDLPin(t, m, neg)
				lower, err = m.MkOrArgs([]ast.ASTHandle{p, lower})
				equationMust(t, err)
				fixDLPin(t, m, lower)
				upper, err = m.MkOrArgs([]ast.ASTHandle{neg, fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "1"))})
				equationMust(t, err)
				fixDLPin(t, m, upper)
				line = 1410
				body = "resolution 0 tautology: \np (-1*x <= -2)\n~p (x <= 1)\n"
			}
			g := fixDLGoal(t, m, false, false, false, lower, upper)
			result := fmResult(t)
			equationMust(t, fm.Apply(g, result))
			size, err := result.Goals[0].Size()
			equationMust(t, err)
			if size != 0 {
				t.Fatal("tautological projection", size)
			}
			if tags.ShouldTrace("fm") && !strings.Contains(out.String(), fmConstraintRecord("fm", "resolve", line, body)) {
				t.Fatal("missing source resolution branch", out.String())
			}
		})
	}
}

type fmFinalGoalFailWriter struct {
	out    bytes.Buffer
	budget *arena.Budget
}

func (w *fmFinalGoalFailWriter) Write(p []byte) (int, error) {
	n, err := w.out.Write(p)
	if strings.Contains(w.out.String(), "fm_tactic.cpp:1611 ---------\n") {
		w.budget.FailAfter(0)
	}
	return n, err
}
func TestFMConstraintFinalTraceFailureKeepsPublishedGoal(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	writer := &fmFinalGoalFailWriter{budget: m.Budget()}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("fm")
	equationMust(t, fm.BindTrace(tags))
	// Default real-only policy copies this integer bound into the final goal.
	x := fixDLConst(t, m, true, "x")
	g := fixDLGoal(t, m, false, false, false, fixDLApp(t, m, 2, x, fixDLNumber(t, m, true, "4")))
	result := fmResult(t)
	err := fm.Apply(g, result)
	m.Budget().FailAfter(-1)
	if tags.ShouldTrace("fm") {
		if !errors.Is(err, arena.ErrInjectedFailure) || len(result.Goals) != 1 || fm.run.goal != nil {
			t.Fatal("final source trace publication prefix", len(result.Goals), err)
		}
	} else {
		equationMust(t, err)
	}
	size, err := result.Goals[0].Size()
	equationMust(t, err)
	if size != 1 {
		t.Fatal("published goal lost", size)
	}
	equationMust(t, fm.Cleanup())
}

func TestFMConstraintTraceGrowthAbortAndRefusedStream(t *testing.T) {
	base, m := fixDLFixture(t, false)
	store, err := NewFMConstraintStore(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, store.Close()) })
	vars := NewFMVariableTable()
	var terms []ast.ASTHandle
	for i, name := range []string{"x", "a", "b", "c", "d", "e", "f"} {
		vars.Add(false, i != 0)
		terms = append(terms, fixDLConst(t, m, false, name))
	}
	expressions, err := NewFMExpressionTable(m, vars)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, expressions.Close()) })
	for i, term := range terms {
		equationMust(t, expressions.BindVariable(uint32(i), term))
	}
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("fm")
	tags.Enable("fm_bug")
	services := base.services
	services.Trace = tags
	diagnostics := &fmConstraintDiagnostics{manager: m, services: services, expressions: expressions}
	store.diagnostics = diagnostics
	vars.Index.diagnostics = diagnostics
	var original []*FMConstraint
	for i := uint32(1); i <= 3; i++ {
		lower, err := store.New(nil, []uint32{0, i}, []big.Rat{fmCRat(-1, 1), fmCRat(1, 1)}, big.NewRat(0, 1), false, ast.ASTDependency{})
		equationMust(t, err)
		upper, err := store.New(nil, []uint32{0, i + 3}, []big.Rat{fmCRat(1, 1), fmCRat(1, 1)}, big.NewRat(0, 1), false, ast.ASTDependency{})
		equationMust(t, err)
		vars.Register(lower)
		vars.Register(upper)
		original = append(original, lower, upper)
	}
	eliminator, err := NewFMEliminator(store, vars, DefaultFMEliminationConfig())
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, eliminator.Close()) })
	eliminated, err := eliminator.Try(0)
	equationMust(t, err)
	if eliminated {
		t.Fatal("growth cutoff did not abort")
	}
	for _, c := range original {
		if c.Dead {
			t.Fatal("growth abort deleted original constraint")
		}
	}
	if tags.ShouldTrace("fm") && !strings.Contains(out.String(), fmConstraintRecord("fm", "try_eliminate", 1489, "too many new constraints: 7\n")) {
		t.Fatal("missing strict greater-than growth guard", out.String())
	}
	// A failed ostream still evaluates operands and completes the same algorithm.
	services.Trace = diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	services.Trace.Enable("fm")
	services.Trace.Enable("fm_bug")
	diagnostics.services = services
	eliminated, err = eliminator.Try(0)
	equationMust(t, err)
	if eliminated {
		t.Fatal("stream refusal changed cutoff")
	}
}

func TestFMConstraintTraceAllocationFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 1024; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			fm, m := fmTacticFixture(t, false)
			var out bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&out)
			for _, tag := range []string{"fm", "fm_bug", "fm_subsumption"} {
				tags.Enable(tag)
			}
			equationMust(t, fm.BindTrace(tags))
			x := fixDLConst(t, m, false, "x")
			g := fixDLGoal(t, m, false, true, true, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "2")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4")))
			result := fmResult(t)
			m.Budget().FailAfter(stop)
			err := fm.Apply(g, result)
			m.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
			equationMust(t, fm.Cleanup())
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("never completed traced application")
	}
}
