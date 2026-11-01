package sls

// Source: src/tactic/sls/sls_engine.cpp and src/tactic/sls/sls_tracker.h.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func slsEngineFixture(t *testing.T) (*Engine, *SearchTracker, *Evaluator, *Tracker, *ast.ASTManager, *numeral.IntegerManager) {
	t.Helper()
	tracker, manager, numbers, powerCache, context, globals, local, _ := slsTrackerFixture(t)
	wp := "wp"
	randomOffset := "random_offset"
	ucb := "walksat_ucb"
	if err := context.SetUIntName(local, &wp, 0); err != nil {
		t.Fatal(err)
	}
	if err := context.SetBoolName(local, &randomOffset, false); err != nil {
		t.Fatal(err)
	}
	if err := context.SetBoolName(local, &ucb, false); err != nil {
		t.Fatal(err)
	}
	search, err := NewSearchTracker(tracker, 0)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewEvaluator(manager, tracker, numbers, powerCache)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(manager, tracker, search, evaluator, context, globals, local, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		if err := evaluator.Close(); err != nil {
			t.Error(err)
		}
		if err := search.Close(); err != nil {
			t.Error(err)
		}
	})
	return engine, search, evaluator, tracker, manager, numbers
}

func TestSLSEngineSolvesRetainedBooleanConstantEndToEnd(t *testing.T) {
	engine, _, _, tracker, manager, numbers := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "sls-engine-p")
	slsPin(t, manager, p)
	if err := engine.AssertExpression(p); err != nil {
		t.Fatal(err)
	}
	status, err := engine.Solve()
	if err != nil || status != lbool.True {
		t.Fatal("search", status, err)
	}
	value, err := tracker.GetValue(p)
	if err != nil || slsIntText(t, numbers, value) != "1" {
		t.Fatal("winning model", err)
	}
	stats, err := engine.Stats()
	if err != nil || stats.Moves == 0 || stats.IncrementalEvaluations == 0 || stats.FullEvaluations == 0 {
		t.Fatal("source statistics", stats, err)
	}
	if stats.Flips != 0 || stats.Increments != 0 || stats.Decrements != 0 || stats.Inversions != 0 {
		t.Fatal("source leaves move-type counters unchanged", stats)
	}
}

func TestSLSEngineSolvesRetainedBVEqualityByIncrementalFlips(t *testing.T) {
	engine, _, _, tracker, manager, numbers := slsEngineFixture(t)
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(8)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort, "sls-engine-bv-x")
	three := slsBVNumeral(t, manager, 3, 8)
	root, err := manager.MkEq(x, three)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, root)
	if err = engine.AssertExpression(root); err != nil {
		t.Fatal(err)
	}
	status, err := engine.Solve()
	if err != nil || status != lbool.True {
		t.Fatal("BV search", status, err)
	}
	value, err := tracker.GetValue(x)
	if err != nil || slsIntText(t, numbers, value) != "3" {
		t.Fatal("BV winning model", err)
	}
	rootValue, err := tracker.GetValue(root)
	if err != nil || slsIntText(t, numbers, rootValue) != "1" {
		t.Fatal("BV assertion", err)
	}
}

func TestSLSEngineMoveArithmeticWrapsAtBitWidth(t *testing.T) {
	engine, _, _, _, manager, numbers := slsEngineFixture(t)
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(4)
	if err != nil {
		t.Fatal(err)
	}
	zero, fifteen, one := numeral.NewInt(0), numeral.NewInt(15), numeral.NewInt(1)
	out := numeral.NewInt(0)
	defer func() {
		if err := numbers.Del(&out); err != nil {
			t.Fatal(err)
		}
	}()
	if err = engine.MkIncrement(4, &fifteen, &out); err != nil || slsIntText(t, numbers, &out) != "0" {
		t.Fatal("increment wrap", err)
	}
	if err = engine.MkDecrement(4, &zero, &out); err != nil || slsIntText(t, numbers, &out) != "15" {
		t.Fatal("decrement wrap", err)
	}
	if err = engine.MkInvert(4, &one, &out); err != nil || slsIntText(t, numbers, &out) != "14" {
		t.Fatal("invert", err)
	}
	if err = engine.MkFlip(sort, &one, 2, &out); err != nil || slsIntText(t, numbers, &out) != "5" {
		t.Fatal("flip", err)
	}
	if err = engine.MkAdd(4, &fifteen, &one, &out); err != nil || slsIntText(t, numbers, &out) != "0" {
		t.Fatal("masked add", err)
	}
}

func TestSLSRandomWalkUsesSourceMoveBits(t *testing.T) {
	for _, tc := range []struct {
		name string
		bits uint32
		want string
	}{
		{"flip", 0, "1"},
		{"increment", 1 << 5, "6"},
		{"decrement", 1 << 4, "4"},
		{"inversion-becomes-flip", 1<<4 | 1<<5, "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine, search, evaluator, tracker, manager, numbers := slsEngineFixture(t)
			if err := manager.RegisterDeclarationPlugins(); err != nil {
				t.Fatal(err)
			}
			sort, err := manager.BitVectorSort(4)
			if err != nil {
				t.Fatal(err)
			}
			x := slsBVConst(t, manager, sort, "random-walk-x")
			target := slsBVNumeral(t, manager, 6, 4)
			root, err := manager.MkEq(x, target)
			if err != nil {
				t.Fatal(err)
			}
			slsPin(t, manager, root)
			if err = search.Initialize([]ast.ASTHandle{root}); err != nil {
				t.Fatal(err)
			}
			if err = search.Reset(); err != nil {
				t.Fatal(err)
			}
			decl, err := manager.GetDecl(x)
			if err != nil {
				t.Fatal(err)
			}
			initial := numeral.NewInt(5)
			if err = evaluator.SeriousUpdate(search, decl, &initial); err != nil {
				t.Fatal(err)
			}
			// Four bits select the only declaration; the next two select the
			// move, and the next four select bit 2 for the flip cases.
			search.randomBits, search.randomBitsCount = tc.bits|1<<9, 15
			before := engine.stats.IncrementalEvaluations
			if err = engine.RandomMove([]ast.ASTHandle{decl}); err != nil {
				t.Fatal(err)
			}
			value, err := tracker.GetValue(x)
			if err != nil || slsIntText(t, numbers, value) != tc.want {
				t.Fatal("random walk value", err, slsIntText(t, numbers, value), "want", tc.want)
			}
			if engine.stats.IncrementalEvaluations != before {
				t.Fatal("random walk changed incremental evaluation count", engine.stats.IncrementalEvaluations, before)
			}
			rootValue, err := tracker.GetValue(root)
			if err != nil {
				t.Fatal(err)
			}
			wantRoot := "0"
			if tc.want == "6" {
				wantRoot = "1"
			}
			if got := slsIntText(t, numbers, rootValue); got != wantRoot {
				t.Fatal("root value after random walk", got, "want", wantRoot)
			}
		})
	}
}

func TestSLSRandomWalkBooleanUpdatesWithoutScoreEvaluation(t *testing.T) {
	engine, search, _, tracker, manager, numbers := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "random-walk-p")
	slsPin(t, manager, p)
	if err := search.Initialize([]ast.ASTHandle{p}); err != nil {
		t.Fatal(err)
	}
	if err := search.Reset(); err != nil {
		t.Fatal(err)
	}
	decl, err := manager.GetDecl(p)
	if err != nil {
		t.Fatal(err)
	}
	search.randomBits, search.randomBitsCount = 0, 15
	if err = engine.RandomMove([]ast.ASTHandle{decl}); err != nil {
		t.Fatal(err)
	}
	value, err := tracker.GetValue(p)
	if err != nil || slsIntText(t, numbers, value) != "1" {
		t.Fatal("Boolean random walk", value, err)
	}
	if engine.stats.IncrementalEvaluations != 0 || search.randomBitsCount != 11 {
		t.Fatal("Boolean random walk accounting", engine.stats, search.randomBitsCount)
	}
}

func TestSLSSearchSolvesViaRandomWalk(t *testing.T) {
	engine, search, _, tracker, manager, numbers := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "random-walk-search-p")
	slsPin(t, manager, p)
	if err := engine.AssertExpression(p); err != nil {
		t.Fatal(err)
	}
	if err := search.Initialize([]ast.ASTHandle{p}); err != nil {
		t.Fatal(err)
	}
	if err := search.Reset(); err != nil {
		t.Fatal(err)
	}
	engine.wp = 1024
	search.randomBits, search.randomBitsCount = 0, 15
	status, err := engine.Search()
	if err != nil || status != lbool.True {
		t.Fatal("random-walk search", status, err)
	}
	value, err := tracker.GetValue(p)
	if err != nil || slsIntText(t, numbers, value) != "1" {
		t.Fatal("random-walk model", value, err)
	}
	if engine.stats.Moves != 2 || engine.stats.IncrementalEvaluations != 0 || engine.stats.FullEvaluations != 1 {
		t.Fatal("random-walk search accounting", engine.stats)
	}
}

func TestSLSSearchTrackerRandomStreamAndUnsatSelection(t *testing.T) {
	_, search, _, tracker, manager, _ := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "sls-random-p")
	q := slsBoolConst(t, manager, "sls-random-q")
	slsPin(t, manager, p, q)
	if err := search.Initialize([]ast.ASTHandle{p, q}); err != nil {
		t.Fatal(err)
	}
	if err := search.SetRandomSeed(17); err != nil {
		t.Fatal(err)
	}
	first, err := search.RandomUint(16)
	if err != nil {
		t.Fatal(err)
	}
	if first != 62464 {
		t.Fatal("first source random value", first)
	}
	if err = search.SetRandomSeed(17); err != nil {
		t.Fatal(err)
	}
	again, err := search.RandomUint(16)
	if err != nil || again != 90010 {
		t.Fatal("source random stream after reseeding", first, again, err)
	}
	root, found, err := search.UnsatAssertion([]ast.ASTHandle{p, q})
	if err != nil || !found || root != p && root != q {
		t.Fatal("unsatisfied assertion reservoir", root, found, err)
	}
	other, found, err := search.NewUnsatAssertion([]ast.ASTHandle{p, q})
	if err != nil || !found || other == root {
		t.Fatal("repick did not exclude the current assertion", root, other, found, err)
	}
	if tracker.touched == 0 {
		t.Fatal("tracker touched total was not initialized")
	}
}

func TestSLSEngineParameterReseedKeepsBufferedRandomBits(t *testing.T) {
	engine, search, _, _, _, _ := slsEngineFixture(t)
	if err := search.SetRandomSeed(17); err != nil {
		t.Fatal(err)
	}
	first, err := search.RandomUint(16)
	if err != nil || first != 62464 {
		t.Fatal("initial source random value", first, err)
	}
	settings, err := engine.context.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.context.Del(&settings); err != nil {
			t.Error(err)
		}
	}()
	name := "random_seed"
	if err = engine.context.SetUIntName(&settings, &name, 17); err != nil {
		t.Fatal(err)
	}
	if err = engine.UpdateParams(&settings); err != nil {
		t.Fatal(err)
	}
	next, err := search.RandomUint(16)
	if err != nil || next != 90010 {
		t.Fatal("source random value after parameter update", next, err)
	}
}

func TestSLSSingleAssertionSelectionKeepsRandomAndUCBState(t *testing.T) {
	_, search, _, tracker, manager, _ := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "sls-single-p")
	slsPin(t, manager, p)
	if err := search.Initialize([]ast.ASTHandle{p}); err != nil {
		t.Fatal(err)
	}
	if err := search.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := search.SetRandomSeed(17); err != nil {
		t.Fatal(err)
	}
	search.lastPosition, search.hasLastPosition = 9, true
	for _, ucb := range []bool{false, true} {
		tracker.ucb = ucb
		beforeTouched := tracker.touched
		beforeBits, beforeCount := search.randomBits, search.randomBitsCount
		root, found, err := search.UnsatAssertion([]ast.ASTHandle{p})
		if err != nil || !found || root != p {
			t.Fatal("single unsatisfied assertion", root, found, err)
		}
		if tracker.touched != beforeTouched || search.randomBits != beforeBits || search.randomBitsCount != beforeCount || search.lastPosition != 9 || !search.hasLastPosition {
			t.Fatal("single assertion changed source selection state", ucb, tracker.touched, search.randomBitsCount, search.lastPosition)
		}
	}
	one := numeral.NewInt(1)
	if err := tracker.SetValue(p, &one); err != nil {
		t.Fatal(err)
	}
	if _, found, err := search.UnsatAssertion([]ast.ASTHandle{p}); err != nil || found {
		t.Fatal("satisfied single assertion", found, err)
	}
	if search.randomBitsCount != 0 || search.lastPosition != 9 {
		t.Fatal("satisfied assertion changed selection state")
	}
}

func TestSLSGSATSingleAssertionUsesAllTrackedConstants(t *testing.T) {
	_, search, _, tracker, manager, _ := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "sls-single-all-p")
	q := slsBoolConst(t, manager, "sls-single-all-q")
	slsPin(t, manager, p, q)
	if err := search.Initialize([]ast.ASTHandle{p, q}); err != nil {
		t.Fatal(err)
	}
	if err := search.Reset(); err != nil {
		t.Fatal(err)
	}
	tracker.walksat = false
	got, err := search.UnsatConstants([]ast.ASTHandle{p})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatal("source returns the complete constant vector", got)
	}
	pDecl, err := manager.GetDecl(p)
	if err != nil {
		t.Fatal(err)
	}
	qDecl, err := manager.GetDecl(q)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != pDecl || got[1] != qDecl {
		t.Fatal("source constant order", got)
	}
	one := numeral.NewInt(1)
	if err = tracker.SetValue(p, &one); err != nil {
		t.Fatal(err)
	}
	if got, err = search.UnsatConstants([]ast.ASTHandle{p}); err != nil || len(got) != 0 {
		t.Fatal("satisfied single assertion returned constants", got, err)
	}
}

func TestSLSEngineMovePhysicalFailurePrefixIsRetryable(t *testing.T) {
	engine, _, _, _, manager, numbers := slsEngineFixture(t)
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(10001)
	if err != nil {
		t.Fatal(err)
	}
	one, huge, out := numeral.NewInt(1), numeral.NewInt(0), numeral.NewInt(0)
	if err = numbers.Mul2k(&one, 10000, &huge); err != nil {
		t.Fatal(err)
	}
	budget := manager.Budget()
	defer func() {
		budget.FailAfter(-1)
		if err := numbers.Del(&out); err != nil {
			t.Fatal(err)
		}
		if err := numbers.Del(&huge); err != nil {
			t.Fatal(err)
		}
	}()
	budget.FailAfter(0)
	if err = engine.MkFlip(sort, &huge, 9999, &out); err == nil {
		t.Fatal("move did not reach power-cache physical refusal")
	}
	budget.FailAfter(-1)
	if err = engine.MkFlip(sort, &huge, 9999, &out); err != nil {
		t.Fatal("move did not recover after owned prefix cleanup", err)
	}
	if out.Cell == numbers.FailedNullRef {
		t.Fatal("retry retained a failed output sentinel")
	}
	if _, err = numbers.IsNonNeg(&out); err != nil && !errors.Is(err, ErrSLSSearch) {
		t.Fatal(err)
	}
}

func TestSLSEngineScoreCounterFailureOrder(t *testing.T) {
	for _, mode := range []string{"incremental", "pruned", "serious"} {
		t.Run(mode, func(t *testing.T) {
			engine, search, _, _, manager, numbers := slsEngineFixture(t)
			if err := manager.RegisterDeclarationPlugins(); err != nil {
				t.Fatal(err)
			}
			sort, err := manager.BitVectorSort(256)
			if err != nil {
				t.Fatal(err)
			}
			x := slsBVConst(t, manager, sort, "score-failure-"+mode)
			zero := slsBVNumeral(t, manager, 0, 256)
			root, err := manager.MkEq(x, zero)
			if err != nil {
				t.Fatal(err)
			}
			slsPin(t, manager, x, zero, root)
			if err = search.Initialize([]ast.ASTHandle{root}); err != nil {
				t.Fatal(err)
			}
			if err = search.Reset(); err != nil {
				t.Fatal(err)
			}
			if _, err = engine.Rescore(); err != nil {
				t.Fatal(err)
			}
			decl, err := manager.GetDecl(x)
			if err != nil {
				t.Fatal(err)
			}
			one, candidate := numeral.NewInt(1), numeral.NewInt(0)
			if err = numbers.Mul2k(&one, 200, &candidate); err != nil {
				t.Fatal(err)
			}
			defer func() {
				manager.Budget().FailAfter(-1)
				if err := numbers.Del(&candidate); err != nil {
					t.Error(err)
				}
			}()
			engine.earlyPrune = mode == "pruned"
			before := engine.stats.IncrementalEvaluations
			manager.Budget().FailAfter(0)
			if mode == "serious" {
				_, err = engine.seriousScore(decl, &candidate)
			} else if mode == "pruned" {
				_, err = engine.incrementalScorePrune(decl, &candidate)
			} else {
				_, err = engine.incrementalScore(decl, &candidate)
			}
			manager.Budget().FailAfter(-1)
			if err == nil {
				t.Fatal("expected an interrupted numerical update")
			}
			want := before
			if mode == "pruned" {
				want++
			}
			if got := engine.stats.IncrementalEvaluations; got != want {
				t.Fatalf("%s evaluations = %d, want %d", mode, got, want)
			}
		})
	}
}

func TestSLSEngineRestorationUsesUnprunedScore(t *testing.T) {
	engine, search, _, _, manager, _ := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "sls-unpruned-restore")
	slsPin(t, manager, p)
	if err := engine.AssertExpression(p); err != nil {
		t.Fatal(err)
	}
	if err := search.Initialize([]ast.ASTHandle{p}); err != nil {
		t.Fatal(err)
	}
	if err := search.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Rescore(); err != nil {
		t.Fatal(err)
	}
	decl, err := manager.GetDecl(p)
	if err != nil {
		t.Fatal(err)
	}
	engine.earlyPrune = true
	zero := numeral.NewInt(0)
	score, err := engine.incrementalScore(decl, &zero)
	if err != nil || score != 0 {
		t.Fatal("source restoration must use the full score", score, err)
	}
}

func TestSLSGoalOrchestrationResetsSATGoalAndStoresModel(t *testing.T) {
	engine, _, _, _, manager, _ := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "sls-goal-p")
	slsPin(t, manager, p)
	goal, err := tactic.NewGoal(manager, false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := goal.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = goal.Assert(p); err != nil {
		t.Fatal(err)
	}
	result := new(tactic.TacticResult)
	t.Cleanup(func() {
		if err := result.Reset(); err != nil {
			t.Error(err)
		}
	})
	status, err := ApplyGoal(engine, goal, result)
	if err != nil || status != lbool.True {
		t.Fatal("goal search", status, err)
	}
	if size, err := goal.Size(); err != nil || size != 0 || !goal.IsDecidedSat() {
		t.Fatal("SAT goal reset", size, err, goal.IsDecidedSat())
	}
	if depth, err := goal.Depth(); err != nil || depth != 1 {
		t.Fatal("goal depth", depth, err)
	}
	if len(result.Goals) != 1 || result.Goals[0] != goal || result.ModelConverter == nil {
		t.Fatal("goal result/model converter", len(result.Goals), result.ModelConverter)
	}
	var modelRef tactic.ModelRef
	if err = result.ModelConverter.ApplyModel(&modelRef, 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := modelRef.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	core, err := modelRef.Get()
	if err != nil || core == nil {
		t.Fatal("stored SLS model", core, err)
	}
	if count, err := core.NumConstants(); err != nil || count != 1 {
		t.Fatal("stored SLS constant", count, err)
	}
}
