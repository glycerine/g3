package duality_test

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var queryScopeRefusal = errors.New("refused query scope boundary")

// The wrapper delegates all logical work to the real native solver. Only the
// outer background scope boundary can refuse before or after publication.
type queryScopeSolver struct {
	solver.Solver
	base      uint32
	failure   string
	pushes    int
	outerPops int
}

func (s *queryScopeSolver) Push() error {
	level, err := s.Solver.ScopeLevel()
	if err != nil {
		return err
	}
	s.pushes++
	if level == s.base && s.failure == "push-before" {
		return queryScopeRefusal
	}
	if err := s.Solver.Push(); err != nil {
		return err
	}
	if level == s.base && s.failure == "push-after" {
		return queryScopeRefusal
	}
	return nil
}

func (s *queryScopeSolver) Pop(count uint32) error {
	level, err := s.Solver.ScopeLevel()
	if err != nil {
		return err
	}
	outer := level == s.base+1 && count == 1
	if outer {
		s.outerPops++
		if s.failure == "pop-before" {
			return queryScopeRefusal
		}
	}
	if err := s.Solver.Pop(count); err != nil {
		return err
	}
	if outer && s.failure == "pop-after" {
		return queryScopeRefusal
	}
	return nil
}

type queryScopeFixture struct {
	manager           *ast.ASTManager
	engine            *duality.QueryEngine
	native, auxiliary *queryScopeSolver
	with, without     *duality.PreparedQuery
}

func newQueryScopeFixture(t *testing.T) *queryScopeFixture {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := cmd_context.NewStandaloneContext()
	must(err)
	t.Cleanup(func() { must(owner.Close()) })
	m, err := owner.Manager()
	must(err)
	config, err := owner.SolverTranslationConfig()
	must(err)
	cfg, err := graph.NewConfig(config.Params, nil)
	must(err)
	t.Cleanup(func() { must(cfg.Close()) })
	context, err := graph.NewContext(m, cfg)
	must(err)
	t.Cleanup(func() { must(context.Close()) })
	factory, err := smtcore.NewSMTSolverFactory(config)
	must(err)
	t.Cleanup(func() { must(factory.Close()) })
	f := &queryScopeFixture{manager: m}
	for _, target := range []**queryScopeSolver{&f.native, &f.auxiliary} {
		native, err := duality.NewNativeSolver(context, config.Params, factory, true)
		must(err)
		t.Cleanup(func() { must(native.Close()) })
		must(native.Push())
		*target = &queryScopeSolver{Solver: native, base: 1}
	}
	simplifier, err := interp.NewManagerWithParams(m, config.Params, config.Globals)
	must(err)
	t.Cleanup(func() { must(simplifier.Close()) })
	quantifiers, err := qe.NewLite(m, nil, true, qe.Config{Context: config.Params, Globals: config.Globals, Cooperation: config.ModelEvaluator.Cooperation})
	must(err)
	t.Cleanup(func() { must(quantifiers.Close()) })
	f.engine, err = duality.NewQueryEngine(context, graph.EngineConfig{Native: f.native, Auxiliary: f.auxiliary, Factory: factory, Simplifier: simplifier, Quantifiers: quantifiers, Evaluator: config.ModelEvaluator, Extensional: true})
	must(err)
	t.Cleanup(func() { f.native.failure = ""; f.auxiliary.failure = ""; must(f.engine.Close()) })
	var refs []ast.ASTRef
	t.Cleanup(func() {
		for i := range refs {
			must(m.DelRef(&refs[i]))
		}
	})
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		ref, err := m.NewRef(h)
		must(err)
		refs = append(refs, ref)
		return h
	}
	boolean := pin(m.MkBoolSort())
	name, err := m.Symbols.Intern("query-scope-background")
	must(err)
	background := pin(m.MkConst(name, boolean))
	name, err = m.Symbols.Intern("query-scope-Goal")
	must(err)
	predicate := pin(m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{}))
	goal := pin(m.MkApp(predicate, nil))
	rule := pin(m.MkImplies(background, goal))
	for _, target := range []**duality.PreparedQuery{&f.with, &f.without} {
		prepared, err := duality.PrepareQuery(m, []ast.ASTHandle{rule}, []uint32{math.MaxUint32}, goal, []ast.ASTHandle{predicate}, math.MaxUint32)
		must(err)
		t.Cleanup(func() { must(prepared.Close()) })
		*target = prepared
	}
	must(f.with.AddBackground(pin(m.MkNot(background))))
	return f
}

func TestDualityBackgroundScopeFailureAndRetry(t *testing.T) {
	for _, side := range []string{"native", "auxiliary"} {
		for _, failure := range []string{"push-before", "push-after", "pop-before", "pop-after"} {
			t.Run(side+"/"+failure, func(t *testing.T) {
				f := newQueryScopeFixture(t)
				if status, err := f.engine.Query(f.with); err != nil || status != lbool.False {
					t.Fatal("baseline query", status, err)
				}
				selected := f.native
				if side == "auxiliary" {
					selected = f.auxiliary
				}
				selected.failure = failure
				if status, err := f.engine.Query(f.with); status != lbool.Undef || !errors.Is(err, queryScopeRefusal) {
					t.Fatal("query scope refusal", status, err)
				}
				if failure == "push-before" || failure == "push-after" {
					for _, native := range []*queryScopeSolver{f.native, f.auxiliary} {
						if level, err := native.ScopeLevel(); err != nil || level != native.base {
							t.Fatal("failed Push changed caller scope", level, native.base, err)
						}
					}
				}
				if md, err := f.engine.GetModel(); md != nil || err == nil {
					t.Fatal("failed scope cleanup published model", md, err)
				}
				if f.engine.Counterexample() != nil || f.engine.IsResultRecursionBounded() {
					t.Fatal("failed scope cleanup published result metadata")
				}
				if failure == "pop-before" {
					pushes := f.native.pushes + f.auxiliary.pushes
					if status, err := f.engine.Query(f.without); status != lbool.Undef || !errors.Is(err, queryScopeRefusal) {
						t.Fatal("pending cleanup admitted query", status, err)
					}
					if f.native.pushes+f.auxiliary.pushes != pushes {
						t.Fatal("pending cleanup entered another solver scope")
					}
				}
				selected.failure = ""
				if status, err := f.engine.Query(f.without); err != nil || status != lbool.True {
					t.Fatal("background-free retry inherited failed axioms", status, err)
				}
				for _, native := range []*queryScopeSolver{f.native, f.auxiliary} {
					if level, err := native.ScopeLevel(); err != nil || level != native.base {
						t.Fatal("query changed caller scope", level, err)
					}
				}
			})
		}
	}
}

func TestDualityBackgroundScopeCloseRetry(t *testing.T) {
	f := newQueryScopeFixture(t)
	f.auxiliary.failure = "pop-before"
	if status, err := f.engine.Query(f.with); status != lbool.Undef || !errors.Is(err, queryScopeRefusal) {
		t.Fatal("query scope refusal", status, err)
	}
	pushes := f.native.pushes + f.auxiliary.pushes
	if err := f.engine.Close(); !errors.Is(err, queryScopeRefusal) {
		t.Fatal("close discarded refused scope", err)
	}
	if status, err := f.engine.Query(f.without); status != lbool.Undef || err == nil {
		t.Fatal("closing query engine admitted query", status, err)
	}
	if f.native.pushes+f.auxiliary.pushes != pushes {
		t.Fatal("close retry entered another scope")
	}
	if md, err := f.engine.GetModel(); md != nil || err == nil {
		t.Fatal("close refusal published model", md, err)
	}
	f.auxiliary.failure = ""
	if err := f.engine.Close(); err != nil {
		t.Fatal("close retry", err)
	}
	pops := f.native.outerPops + f.auxiliary.outerPops
	if err := f.engine.Close(); err != nil || pops != f.native.outerPops+f.auxiliary.outerPops {
		t.Fatal("completed close replayed scope retirement", err)
	}
	for _, native := range []*queryScopeSolver{f.native, f.auxiliary} {
		if level, err := native.ScopeLevel(); err != nil || level != native.base {
			t.Fatal("close changed caller scope", level, native.base, err)
		}
	}
}

func TestDualityBackgroundScopeAllocationFrontiers(t *testing.T) {
	f := newQueryScopeFixture(t)
	if status, err := f.engine.Query(f.with); err != nil || status != lbool.False {
		t.Fatal("baseline query", status, err)
	}
	complete, failures := false, 0
	for stop := int64(0); stop < 4096 && !complete; stop++ {
		f.manager.Budget().FailAfter(stop)
		status, err := f.engine.Query(f.with)
		f.manager.Budget().FailAfter(-1)
		if err != nil {
			failures++
			if status != lbool.Undef || !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("allocation frontier", stop, status, err)
			}
		} else {
			complete = true
			if status != lbool.False {
				t.Fatal("completed background query", status)
			}
		}
		for _, native := range []*queryScopeSolver{f.native, f.auxiliary} {
			if level, err := native.ScopeLevel(); err != nil || level != native.base {
				t.Fatal("allocation changed caller scope", stop, level, err)
			}
		}
	}
	if !complete || failures == 0 {
		t.Fatal("allocation campaign missed failure or completion", complete, failures)
	}
	t.Logf("background scope allocation campaign: %d failures before completion", failures)
	if status, err := f.engine.Query(f.without); err != nil || status != lbool.True {
		t.Fatal("post-frontier query inherited background", status, err)
	}
}
