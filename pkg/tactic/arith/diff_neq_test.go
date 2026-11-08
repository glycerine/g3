package arith

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/statistics"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func diffNeqFixture(t *testing.T) *DiffNeqTactic {
	t.Helper()
	base, m := fixDLFixture(t, false)
	r, err := NewDiffNeqTactic(m, base.context, base.globals, nil)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	return r
}
func diffNeqTestBounds(t *testing.T, m *ast.ASTManager, x ast.ASTHandle, upper string) []ast.ASTHandle {
	t.Helper()
	return []ast.ASTHandle{fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "0")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, true, upper))}
}
func diffNeqTestAtom(t *testing.T, m *ast.ASTManager, x, y ast.ASTHandle, k int, reverse bool) ast.ASTHandle {
	t.Helper()
	var lhs, rhs ast.ASTHandle
	if k == 0 && !reverse {
		lhs, rhs = x, y
	} else {
		negative := fixDLApp(t, m, 9, fixDLNumber(t, m, true, "-1"), y)
		if reverse {
			lhs = fixDLApp(t, m, 6, negative, x)
		} else {
			lhs = fixDLApp(t, m, 6, x, negative)
		}
		rhs = fixDLNumber(t, m, true, fmt.Sprint(k))
	}
	eq, err := m.MkEq(lhs, rhs)
	equationMust(t, err)
	fixDLPin(t, m, eq)
	not, err := m.MkNot(eq)
	equationMust(t, err)
	fixDLPin(t, m, not)
	return not
}

func TestDiffNeqExhaustiveSmallDomains(t *testing.T) {
	base := diffNeqFixture(t)
	m := base.manager
	vars := []ast.ASTHandle{fixDLConst(t, m, true, "x"), fixDLConst(t, m, true, "y"), fixDLConst(t, m, true, "z")}
	var bounds []ast.ASTHandle
	for _, x := range vars {
		bounds = append(bounds, diffNeqTestBounds(t, m, x, "2")...)
	}
	type constraint struct {
		x, y, k int
		term    ast.ASTHandle
	}
	var constraints []constraint
	for x := 0; x < 3; x++ {
		for y := x + 1; y < 3; y++ {
			for k := -1; k <= 1; k++ {
				constraints = append(constraints, constraint{x, y, k, diffNeqTestAtom(t, m, vars[x], vars[y], k, k == 1)})
			}
		}
	}
	for mask := 0; mask < 1<<len(constraints); mask++ {
		r, err := NewDiffNeqTactic(m, base.context, base.globals, nil)
		equationMust(t, err)
		forms := append([]ast.ASTHandle(nil), bounds...)
		for i, c := range constraints {
			if mask&(1<<i) != 0 {
				forms = append(forms, c.term)
			}
		}
		goal, err := tactic.NewGoal(m, false, false, false)
		equationMust(t, err)
		for _, f := range forms {
			equationMust(t, goal.Assert(f))
		}
		var result tactic.TacticResult
		equationMust(t, r.Apply(goal, &result))
		if result.ModelConverter != nil {
			t.Fatal("model generated for disabled model flag")
		}
		valid := func(values []int32) bool {
			for i, c := range constraints {
				if mask&(1<<i) != 0 && values[c.x]-values[c.y] == int32(c.k) {
					return false
				}
			}
			return true
		}
		want := false
		for x := int32(0); x <= 2; x++ {
			for y := int32(0); y <= 2; y++ {
				for z := int32(0); z <= 2; z++ {
					want = want || valid([]int32{x, y, z})
				}
			}
		}
		if goal.IsDecidedSat() != want || goal.IsDecidedUnsat() == want {
			t.Fatalf("oracle disagreement mask %d: sat %v want %v", mask, goal.IsDecidedSat(), want)
		}
		if want {
			if len(r.current.stack) != 3 || !valid(r.current.stack) {
				t.Fatal("invalid witness", mask, r.current.stack)
			}
			for _, v := range r.current.stack {
				if v < 0 || v > 2 {
					t.Fatal("out of domain witness", v)
				}
			}
		}
		for _, marked := range r.current.forbidden {
			if marked {
				t.Fatal("forbidden cache was not cleared", mask)
			}
		}
		equationMust(t, result.Close())
		equationMust(t, goal.Close())
		equationMust(t, r.Close())
	}
}

func TestDiffNeqResourceScopeTranslationAndEmptyGoals(t *testing.T) {
	r := diffNeqFixture(t)
	m := r.manager
	p, err := r.context.NewValue()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.context.Del(&p)) })
	key := "diff_neq_max_k"
	equationMust(t, r.context.SetUIntName(&p, &key, 1))
	equationMust(t, r.UpdateParams(&p))
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, target.Close()) })
	translated, err := r.Translate(target)
	equationMust(t, err)
	equationMust(t, translated.IncRef())
	t.Cleanup(func() { equationMust(t, translated.DecRef()) })
	equationMust(t, r.context.SetUIntName(&p, &key, 2))
	equationMust(t, r.UpdateParams(&p))
	x := fixDLConst(t, target, true, "x")
	goal := fixDLGoal(t, target, false, false, false, diffNeqTestBounds(t, target, x, "2")...)
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	if err = translated.Apply(goal, &result); err == nil || err.Error() != "goal is not diff neq" {
		t.Fatal("translation lost stored parameter", err)
	}
	equationMust(t, translated.UpdateParams(&p))
	equationMust(t, translated.Cleanup())
	equationMust(t, translated.Apply(goal, &result))
	if !goal.IsDecidedSat() {
		t.Fatal("translated updated tactic failed")
	}
	x0 := fixDLConst(t, m, true, "x0")
	x1 := fixDLConst(t, m, true, "x1")
	forms := append(diffNeqTestBounds(t, m, x0, "1"), diffNeqTestBounds(t, m, x1, "1")...)
	limited := fixDLGoal(t, m, false, false, false, forms...)
	var sourceResult tactic.TacticResult
	t.Cleanup(func() { equationMust(t, sourceResult.Close()) })
	scope, err := control.NewLimitScope(m.ResourceLimit(), 1)
	equationMust(t, err)
	err = r.Apply(limited, &sourceResult)
	scope.Close()
	if err == nil || err.Error() != "max. resource limit exceeded" {
		t.Fatal("resource-limit message", err)
	}
	equationMust(t, r.Cleanup())
	equationMust(t, r.Apply(limited, &sourceResult))
	if !limited.IsDecidedSat() {
		t.Fatal("resource scope did not recover")
	}
	for _, inconsistent := range []bool{false, true} {
		equationMust(t, r.Cleanup())
		var forms []ast.ASTHandle
		if inconsistent {
			f, e := m.MkFalse()
			equationMust(t, e)
			forms = append(forms, f)
		}
		g := fixDLGoal(t, m, false, true, false, forms...)
		before := m.ResourceLimit().Count()
		equationMust(t, r.Apply(g, &sourceResult))
		depth, e := g.Depth()
		equationMust(t, e)
		want := uint32(1)
		if inconsistent {
			want = 0
		}
		if depth != want || m.ResourceLimit().Count() != before {
			t.Fatal("empty/inconsistent depth or cancellation point", inconsistent, depth)
		}
		if (sourceResult.ModelConverter == nil) != inconsistent {
			t.Fatal("empty/inconsistent model behavior", inconsistent)
		}
	}
}

func TestDiffNeqAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "sat-model", "unsat", "cleanup"} {
		succeeded := false
		for stop := int64(0); stop < 256; stop++ {
			t.Run(fmt.Sprintf("%s/%d", phase, stop), func(t *testing.T) {
				r := diffNeqFixture(t)
				m := r.manager
				if phase == "constructor" {
					m.Budget().FailAfter(stop)
					fresh, err := NewDiffNeqTactic(m, r.context, r.globals, nil)
					m.Budget().FailAfter(-1)
					if err == nil {
						succeeded = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal(err)
					}
					if fresh != nil {
						equationMust(t, fresh.Close())
					}
					return
				}
				x := fixDLConst(t, m, true, "x")
				y := fixDLConst(t, m, true, "y")
				upper := "1"
				if phase == "unsat" {
					upper = "0"
				}
				forms := append(diffNeqTestBounds(t, m, x, upper), diffNeqTestBounds(t, m, y, "0")...)
				forms = append(forms, diffNeqTestAtom(t, m, x, y, 0, false))
				g := fixDLGoal(t, m, false, true, false, forms...)
				var result tactic.TacticResult
				t.Cleanup(func() { equationMust(t, result.Close()) })
				if phase == "cleanup" {
					equationMust(t, r.Apply(g, &result))
				}
				m.Budget().FailAfter(stop)
				var err error
				if phase == "cleanup" {
					err = r.Cleanup()
				} else {
					err = r.Apply(g, &result)
				}
				m.Budget().FailAfter(-1)
				if err == nil {
					succeeded = true
				} else if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal(err)
				}
				if phase == "cleanup" {
					equationMust(t, r.drainDiffNeq())
					if r.current.conflicts != 1 {
						t.Fatal("cleanup failed to preserve conflict statistics")
					}
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("never reached success", phase)
		}
	}
}

func TestDiffNeqBacktrackingStatisticsAndIndependentModel(t *testing.T) {
	r := diffNeqFixture(t)
	m := r.manager
	x := fixDLConst(t, m, true, "x")
	y := fixDLConst(t, m, true, "y")
	forms := append(diffNeqTestBounds(t, m, x, "1"), diffNeqTestBounds(t, m, y, "0")...)
	forms = append(forms, diffNeqTestAtom(t, m, x, y, 0, false))
	goal := fixDLGoal(t, m, false, true, false, forms...)
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, r.Apply(goal, &result))
	if !goal.IsDecidedSat() || r.current.conflicts != 1 || len(r.current.stack) != 2 || r.current.stack[0] != 1 || r.current.stack[1] != 0 {
		t.Fatal("backtracking witness", r.current.stack, r.current.conflicts)
	}
	depth, err := goal.Depth()
	equationMust(t, err)
	if depth != 1 {
		t.Fatal("depth", depth)
	}
	equationMust(t, r.Cleanup())
	if statistics.Sums(r.CollectStatistics)["conflicts"] != 1 || len(r.current.variables) != 0 {
		t.Fatal("cleanup lost stats or kept search state")
	}
	equationMust(t, r.ResetStatistics())
	if statistics.Sums(r.CollectStatistics)["conflicts"] != 0 {
		t.Fatal("reset statistics")
	}
	equationMust(t, r.Close())
	var md tactic.ModelRef
	t.Cleanup(func() { equationMust(t, md.Close()) })
	equationMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &md))
	model, err := md.Get()
	equationMust(t, err)
	for i, term := range []ast.ASTHandle{x, y} {
		decl, e := m.GetDecl(term)
		equationMust(t, e)
		value, e := model.ConstantInterpretation(decl)
		equationMust(t, e)
		text, isInt, matched, e := m.ArithmeticNumeralText(value)
		equationMust(t, e)
		want := "1"
		if i == 1 {
			want = "0"
		}
		if !matched || !isInt || text != want {
			t.Fatal("stored model lost value after close", text, want)
		}
	}
}

func TestDiffNeqRefusalsLimitsAndCancellation(t *testing.T) {
	r := diffNeqFixture(t)
	m := r.manager
	x := fixDLConst(t, m, true, "x")
	y := fixDLConst(t, m, true, "y")
	real := fixDLConst(t, m, false, "r")
	for _, forms := range [][]ast.ASTHandle{
		{fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "0"))},
		{fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "1")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, true, "2"))},
		{fixDLApp(t, m, 3, real, fixDLNumber(t, m, false, "0")), fixDLApp(t, m, 2, real, fixDLNumber(t, m, false, "2"))},
		{fixDLApp(t, m, 4, x, fixDLNumber(t, m, true, "2"))},
		{diffNeqTestAtom(t, m, x, x, 0, false)},
		{fixDLApp(t, m, 2, x, y)},
	} {
		goal := fixDLGoal(t, m, false, false, false, forms...)
		var result tactic.TacticResult
		err := r.Apply(goal, &result)
		if err == nil || err.Error() != "goal is not diff neq" {
			t.Fatal("unsupported goal", err)
		}
		depth, e := goal.Depth()
		equationMust(t, e)
		if depth != 0 {
			t.Fatal("refusal changed depth")
		}
		equationMust(t, result.Close())
		equationMust(t, r.Cleanup())
	}
	p, err := r.context.NewValue()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.context.Del(&p)) })
	key := "diff_neq_max_k"
	equationMust(t, r.context.SetUIntName(&p, &key, ^uint32(0)))
	equationMust(t, r.UpdateParams(&p))
	if r.current.maxK != 1073741823 || r.current.maxNegK != -4294967295 {
		t.Fatal("source clamp order changed", r.current.maxK, r.current.maxNegK)
	}
	equationMust(t, r.context.SetUIntName(&p, &key, 1))
	equationMust(t, r.UpdateParams(&p))
	goal := fixDLGoal(t, m, false, false, false, diffNeqTestBounds(t, m, x, "2")...)
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	if err = r.Apply(goal, &result); err == nil || err.Error() != "goal is not diff neq" {
		t.Fatal("maximum offset ignored", err)
	}
	equationMust(t, r.context.SetUIntName(&p, &key, 1024))
	equationMust(t, r.UpdateParams(&p))
	equationMust(t, r.Cleanup())
	m.ResourceLimit().Cancel()
	before := m.ResourceLimit().Count()
	err = r.Apply(goal, &result)
	if err == nil || err.Error() != tactic.TacticCanceledMessage || m.ResourceLimit().Count() != before+1 {
		t.Fatal("source cancellation point", err)
	}
	m.ResourceLimit().ResetCancel()
	equationMust(t, r.Cleanup())
	equationMust(t, r.Apply(goal, &result))
	if !goal.IsDecidedSat() {
		t.Fatal("cleanup did not recover cancellation")
	}
}

func TestDiffNeqGuardsAndExactTrace(t *testing.T) {
	r := diffNeqFixture(t)
	m := r.manager
	equationMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	for _, flags := range [][2]bool{{true, false}, {false, true}} {
		g := fixDLGoal(t, m, flags[0], false, flags[1])
		var result tactic.TacticResult
		if err := r.Apply(g, &result); err == nil {
			t.Fatal("proof/core guard ignored")
		}
		equationMust(t, result.Close())
	}
	x := fixDLConst(t, m, true, "x")
	goal := fixDLGoal(t, m, false, false, false, diffNeqTestBounds(t, m, x, "0")...)
	var trace, before bytes.Buffer
	equationMust(t, goal.Display(&before))
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("diff_neq_tactic")
	tags.Enable("diff_neq")
	equationMust(t, r.BindTrace(tags))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, r.Apply(goal, &result))
	if tags.ShouldTrace("diff_neq_tactic") {
		record := func(tag, function string, line int, body string) string {
			return fmt.Sprintf("-------- [%s] %s src/tactic/arith/diff_neq_tactic.cpp:%d ---------\n%s------------------------------------------------\n", tag, function, line, body)
		}
		var after bytes.Buffer
		equationMust(t, goal.Display(&after))
		want := record("diff_neq_tactic", "compile", 171, "processing: (>= x 0)\n") + record("diff_neq_tactic", "compile", 171, "processing: (<= x 0)\n") + record("diff_neq_tactic", "operator()", 331, before.String()+"0 <= x <= 0\n") + record("diff_neq_tactic", "search", 293, "") + record("diff_neq_tactic", "search", 300, "x := 0\n") + record("diff_neq", "operator()", 344, after.String())
		if trace.String() != want {
			t.Fatal("source trace", trace.String(), want)
		}
	} else if trace.Len() != 0 {
		t.Fatal("erased trace wrote output")
	}
	refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	refused.EnableAll(true)
	equationMust(t, r.BindTrace(refused))
	equationMust(t, r.Cleanup())
	equationMust(t, r.Apply(goal, &result))
	copy := *r
	if err := copy.Cleanup(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied owner", err)
	}
	equationMust(t, r.Close())
	if err := r.IncRef(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed owner", err)
	}
}
