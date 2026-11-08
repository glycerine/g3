package bv

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/statistics"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func bvBoundFixture(t *testing.T) *BVBoundCheckTactic {
	t.Helper()
	base := sharingTacticFixture(t, true)
	m := base.manager
	sharingMust(t, m.RegisterDeclarationPlugins())
	coop, err := cooperate.NewCooperation(m.Budget())
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, coop.Close()) })
	globals := uparams.BorrowProcessGlobalParamsService()
	services := model.ModelEvaluatorConfig{Context: base.context, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: coop}
	r, err := NewBVBoundCheckTactic(m, base.context, globals, nil, services)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, r.Close()) })
	bvBoundPolicy(t, r, "bv_ineq_consistency_test_max", 100)
	return r
}
func bvBoundPolicy(t *testing.T, r *BVBoundCheckTactic, key string, value uint32) {
	t.Helper()
	p, err := r.context.NewValue()
	sharingMust(t, err)
	defer func() { sharingMust(t, r.context.Del(&p)) }()
	sharingMust(t, r.context.Assign(&p, &r.params))
	sharingMust(t, r.context.SetUIntName(&p, &key, value))
	sharingMust(t, r.UpdateParams(&p))
}
func bvBoundGoal(t *testing.T, r *BVBoundCheckTactic, lower, upper uint32) *tactic.Goal {
	t.Helper()
	m := r.manager
	sort, err := m.BitVectorSort(8)
	sharingMust(t, err)
	name, err := m.Symbols.Intern("bv-bound-x")
	sharingMust(t, err)
	x, err := m.MkConst(name, sort)
	sharingMust(t, err)
	lo, err := m.BitVectorNumeralUnsigned(lower, 8)
	sharingMust(t, err)
	hi, err := m.BitVectorNumeralUnsigned(upper, 8)
	sharingMust(t, err)
	family, err := m.RegisterBitVectorPlugin()
	sharingMust(t, err)
	a, err := m.MkFamilyApp(family, 22, nil, []ast.ASTHandle{lo, x})
	sharingMust(t, err)
	b, err := m.MkFamilyApp(family, 22, nil, []ast.ASTHandle{x, hi})
	sharingMust(t, err)
	bounds, err := m.MkAnd(a, b)
	sharingMust(t, err)
	bs, err := m.MkBoolSort()
	sharingMust(t, err)
	name, err = m.Symbols.Intern("bv-bound-p")
	sharingMust(t, err)
	p, err := m.MkConst(name, bs)
	sharingMust(t, err)
	form, err := m.MkOr(p, bounds)
	sharingMust(t, err)
	g, err := tactic.NewGoal(m, false, true, false)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	sharingMust(t, g.Assert(form))
	return g
}

func TestBVBoundCheckStatisticsCleanupAndGuards(t *testing.T) {
	r := bvBoundFixture(t)
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	g := bvBoundGoal(t, r, 3, 2)
	sharingMust(t, r.Apply(g, &result))
	if r.stats.Unsats != 1 || result.Goals[0] != g {
		t.Fatal("missing inconsistent interval statistic", r.stats)
	}
	old := r.current
	r.manager.Budget().FailAfter(0)
	err := r.Cleanup()
	r.manager.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || r.current != old {
		t.Fatal("cleanup published before construction succeeded", err)
	}
	sharingMust(t, r.Cleanup())
	if r.current == old || r.stats.Unsats != 1 {
		t.Fatal("cleanup did not preserve shared statistics")
	}
	singleton := bvBoundGoal(t, r, 2, 2)
	sharingMust(t, r.Apply(singleton, &result))
	if statistics.Sums(r.CollectStatistics)["bv singletons"] != 1 {
		t.Fatal("missing singleton statistic", r.stats)
	}
	for _, proofs := range []bool{true, false} {
		refused, err := tactic.NewGoal(r.manager, proofs, false, !proofs)
		sharingMust(t, err)
		t.Cleanup(func() { sharingMust(t, refused.Close()) })
		err = r.Apply(refused, &result)
		var exception tactic.TacticException
		if !errors.As(err, &exception) || !strings.Contains(err.Error(), "does not support") || len(result.Goals) != 1 || result.Goals[0] != singleton {
			t.Fatal("refusal modified previous result", err)
		}
	}
	sharingMust(t, r.ResetStatistics())
	if r.stats != (BVBoundCheckStats{}) {
		t.Fatal("statistics reset")
	}
	descr, err := uparams.NewParamDescrs(r.manager.Symbols, uparams.ParamDescrsConfig{Budget: r.manager.Budget()})
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, descr.Close()) })
	sharingMust(t, r.CollectParamDescrs(descr))
	n, err := descr.Size()
	sharingMust(t, err)
	if n != 0 {
		t.Fatal("source exposes no tactic parameter descriptors", n)
	}
}

func TestBVBoundCheckLimitsTranslationCancellationAndTrace(t *testing.T) {
	r := bvBoundFixture(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("bv-bound-chk")
	tags.Enable("bv_bound_chk_step")
	sharingMust(t, r.BindTrace(tags))
	g := bvBoundGoal(t, r, 3, 2)
	var result tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, result.Close()) })
	sharingMust(t, r.Apply(g, &result))
	if tags.ShouldTrace("bv-bound-chk") {
		for _, text := range []string{"before:", "after:", "---------->", "bv_bound_chk_tactic.cpp:63"} {
			if !strings.Contains(output.String(), text) {
				t.Fatal("missing trace", text, output.String())
			}
		}
	}
	closed := diagnostic.NewTraceTagsWithWriter(bv1ClosedWriter{})
	closed.Enable("bv-bound-chk")
	closed.Enable("bv_bound_chk_step")
	sharingMust(t, r.BindTrace(closed))
	sharingMust(t, r.Apply(bvBoundGoal(t, r, 2, 2), &result))
	bvBoundPolicy(t, r, "max_steps", 0)
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: r.manager.Symbols, Numerals: r.manager.NumeralService(), Budget: r.manager.Budget()})
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, target.Close()) })
	sharingMust(t, target.RegisterDeclarationPlugins())
	other, err := r.Translate(target)
	sharingMust(t, err)
	translated := other.(*BVBoundCheckTactic)
	t.Cleanup(func() { sharingMust(t, translated.Close()) })
	bvBoundPolicy(t, r, "max_steps", 10000)
	if translated.current.configuration.maxSteps != 0 {
		t.Fatal("translated parameters alias original")
	}
	tr, err := ast.NewASTTranslation(r.manager, target, true)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, tr.Close()) })
	copy, err := g.Translate(tr)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, copy.Close()) })
	var translatedResult tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, translatedResult.Close()) })
	err = translated.Apply(copy, &translatedResult)
	if !rewriter.IsRewriterException(err) || !strings.Contains(err.Error(), "max. steps") {
		t.Fatal("translated step limit", err)
	}
	sharingMust(t, r.Cleanup())
	fresh := bvBoundGoal(t, r, 3, 2)
	r.manager.ResourceLimit().IncCancel()
	err = r.Apply(fresh, &result)
	r.manager.ResourceLimit().DecCancel()
	if !rewriter.IsRewriterException(err) || !strings.Contains(err.Error(), "canceled") {
		t.Fatal("cancellation", err)
	}
	sharingMust(t, r.Cleanup())
	bvBoundPolicy(t, r, "max_memory", 0)
	err = r.Apply(fresh, &result)
	var exception tactic.TacticException
	if !errors.As(err, &exception) || !strings.Contains(err.Error(), "max. memory") {
		t.Fatal("raw byte memory limit", err)
	}
}

func TestBVBoundCheckAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "apply", "cleanup"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 2048 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					r := bvBoundFixture(t)
					var result tactic.TacticResult
					t.Cleanup(func() { sharingMust(t, result.Close()) })
					g := bvBoundGoal(t, r, 2, 2)
					if phase == "cleanup" {
						sharingMust(t, r.Apply(g, &result))
					}
					r.manager.Budget().FailAfter(stop)
					var err error
					switch phase {
					case "constructor":
						var fresh *BVBoundCheckTactic
						fresh, err = NewBVBoundCheckTactic(r.manager, r.context, r.globals, &r.params, r.services)
						r.manager.Budget().FailAfter(-1)
						if fresh != nil {
							sharingMust(t, fresh.Close())
						}
						var failed memory.ConstructionFailure
						if errors.As(err, &failed) {
							sharingMust(t, failed.Cleanup())
						}
					case "apply":
						err = r.Apply(g, &result)
					case "cleanup":
						err = r.Cleanup()
					}
					r.manager.Budget().FailAfter(-1)
					if err == nil {
						success = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal(err)
					}
					sharingMust(t, r.Cleanup())
				})
				if t.Failed() {
					t.Fatal("reservation frontier", phase, stop)
				}
			}
			if !success {
				t.Fatal("no successful reservation frontier", phase)
			}
		})
	}
}
