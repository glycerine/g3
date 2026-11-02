package duality_test

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	graph "github.com/glycerine/g3/pkg/duality"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPreparedQueryBuildGraphNativeEngineAndIndependentLifetime(t *testing.T) {
	for _, reachable := range []bool{false, true} {
		name := "safe"
		if reachable {
			name = "reachable"
		}
		t.Run(name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			owner, err := cmd_context.NewStandaloneContext()
			must(err)
			defer owner.Close()
			m, err := owner.Manager()
			must(err)
			must(m.ToggleProofMode(ast.ASTProofEnabled))
			services, err := owner.ModelEvaluatorConfig()
			must(err)
			solverConfig, err := owner.SolverTranslationConfig()
			must(err)
			config, err := graph.NewConfig(solverConfig.Params, nil)
			must(err)
			defer config.Close()
			context, err := graph.NewContext(m, config)
			must(err)
			defer context.Close()
			simplifier, err := interp.NewManagerWithParams(m, solverConfig.Params, solverConfig.Globals)
			must(err)
			defer simplifier.Close()
			quantifiers, err := qe.NewLite(m, nil, true, qe.Config{Context: solverConfig.Params, Globals: solverConfig.Globals, Cooperation: services.Cooperation})
			must(err)
			defer quantifiers.Close()
			factory, err := smtcore.NewSMTSolverFactory(solverConfig)
			must(err)
			defer factory.Close()
			native, err := factory.New(m, nil, true, true, true, "")
			must(err)
			defer native.Close()
			aux, err := factory.New(m, nil, false, true, false, "")
			must(err)
			defer aux.Close()
			atom, err := context.BoolConst("fixedpoint-query")
			must(err)
			defer atom.Close()
			query, err := atom.Raw()
			must(err)
			decl, err := m.GetDecl(query)
			must(err)
			rule := query
			var ruleRef ast.ASTRef
			if !reachable {
				no, err := m.MkFalse()
				must(err)
				rule, err = m.MkImplies(no, query)
				must(err)
				ruleRef, err = m.NewRef(rule)
				must(err)
				defer m.DelRef(&ruleRef)
			}
			prepared, err := duality.PrepareQuery(m, []ast.ASTHandle{rule}, []uint32{math.MaxUint32}, query, []ast.ASTHandle{decl}, math.MaxUint32)
			must(err)
			defer prepared.Close()
			failed, err := prepared.BuildGraph(context, nil, quantifiers)
			if failed != nil || !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("construction failure published a partial graph", failed, err)
			}
			rpfp, err := prepared.BuildGraph(context, simplifier, quantifiers)
			must(err)
			defer rpfp.Close()
			must(prepared.Close())
			if _, err := prepared.BuildGraph(context, simplifier, quantifiers); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("closed query accepted", err)
			}
			engine, err := graph.NewEngine(rpfp, graph.EngineConfig{Native: native, Auxiliary: aux, Factory: factory, Evaluator: services, Simplifier: simplifier, Quantifiers: quantifiers})
			must(err)
			defer engine.Close()
			solved, err := engine.Solve()
			must(err)
			if solved == reachable {
				t.Fatalf("safety=%v reachable=%v", solved, reachable)
			}
			if reachable && engine.GetCounterexample().Root() == nil {
				t.Fatal("missing fixedpoint counterexample")
			}
			must(engine.Close())
			session, err := duality.NewQueryEngine(context, graph.EngineConfig{Native: native, Auxiliary: aux, Factory: factory, Evaluator: services, Simplifier: simplifier, Quantifiers: quantifiers})
			must(err)
			defer session.Close()
			copied := *session
			if err := copied.Close(); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("copied owning query engine accepted", err)
			}
			var previous *graph.Counterexample
			for round := 0; round < 3; round++ {
				next, err := duality.PrepareQuery(m, []ast.ASTHandle{rule}, []uint32{math.MaxUint32}, query, []ast.ASTHandle{decl}, math.MaxUint32)
				must(err)
				status, err := session.Query(next)
				must(err)
				if previous != nil && previous.Root() != nil {
					t.Fatal("obsolete counterexample owner still holds its graph")
				}
				previous = session.Counterexample()
				must(next.Close())
				want := lbool.False
				if reachable {
					want = lbool.True
				}
				if status != want {
					t.Fatalf("query %d returned %v want %v", round, status, want)
				}
				if reachable && (session.Counterexample() == nil || session.Counterexample().Root() == nil) {
					t.Fatal("query lifecycle lost counterexample")
				}
				if !reachable && session.Counterexample() != nil {
					t.Fatal("safe query published counterexample")
				}
				if session.IsResultRecursionBounded() {
					t.Fatal("unbounded query reported bounded")
				}
			}
			if status, err := session.Query(nil); err == nil || status != lbool.Undef {
				t.Fatal("invalid query did not return unknown/error", status, err)
			}
			must(session.Close())
			must(session.Close())
			if previous != nil && previous.Root() != nil {
				t.Fatal("closed session kept a counterexample graph")
			}
			if session.Counterexample() != nil {
				t.Fatal("closed query engine exposed counterexample")
			}
			if _, err := session.Query(prepared); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("closed engine accepted query", err)
			}

		})
	}
}
