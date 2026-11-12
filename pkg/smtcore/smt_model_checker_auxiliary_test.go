package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelCheckerAuxiliarySnapshotAndConcretePlugins(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	p, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
	name := "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&p, &name, false))
	for key, value := range map[string]uint32{"relevancy": 2, "case_split": 3, "max_conflicts": 17} {
		key := key
		astTestMust(t, cfg.Context.SetUIntName(&p, &key, value))
	}
	s, err := NewSMTSolver(m, &p, "QF_AUFBVLIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	status, err := s.CheckSatCore(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("source setup", status)
	}
	c := s.kernel.internalizer.quantifierManager.modelChecker
	astTestMust(t, c.initAuxiliaryContext())
	a := c.auxiliaryContext
	if a.logic != (symbol.Symbol{}) || a.kernel.setupLogic != "" || a.kernel.smtState.RelevancyLvl != 0 || a.kernel.relevancyConfigured || a.kernel.contextRelevancyLvl != 2 || a.kernel.smtState.CaseSplitStrategy != 0 || a.kernel.smtState.MaxConflicts != 17 {
		t.Fatal("auxiliary did not use the source snapshot overrides", a.kernel.smtState)
	}
	if !a.kernel.copiedArithmeticPlugin || a.kernel.copiedBVOwner == nil || !a.kernel.copiedArrayPlugin {
		t.Fatal("auxiliary omitted an installed concrete plugin")
	}
	arithmetic, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	if smtArithmeticLifecycleKind(a.kernel.internalizer.theories[arithmetic]) != smtArithmeticLifecycleKind(s.kernel.internalizer.theories[arithmetic]) {
		t.Fatal("empty-logic auxiliary replaced the source arithmetic backend")
	}
	for family, previous := range s.kernel.internalizer.theories {
		next := a.kernel.internalizer.theories[family]
		if next == previous {
			t.Fatal("auxiliary borrowed a theory instance", family)
		}
	}
	for key, value := range map[string]uint32{"relevancy": 1, "case_split": 6, "max_conflicts": 29} {
		key := key
		astTestMust(t, cfg.Context.SetUIntName(&p, &key, value))
	}
	astTestMust(t, s.UpdateParams(&p))
	astTestMust(t, c.InitSearch())
	astTestMust(t, c.initAuxiliaryContext())
	if c.auxiliaryContext != a || a.kernel.smtState.MaxConflicts != 17 || a.kernel.smtState.RelevancyLvl != 0 || a.kernel.smtState.CaseSplitStrategy != 0 {
		t.Fatal("auxiliary snapshot was replaced by live parent settings")
	}
	astTestMust(t, s.Close())
	if !a.closed {
		t.Fatal("checker destruction did not close cached auxiliary")
	}
}

func smtAuxiliaryQueryFixture(t *testing.T) (*ast.ASTManager, *SMTModelChecker, *protomodel.ProtoModel, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integer, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	body, err := m.MkEq(x, x)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body})
	astTestMust(t, err)
	astTestPin(t, m, q)
	astTestPin(t, m, zero)
	status, err := s.CheckSatCore(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("source setup", status)
	}
	proto, err := protomodel.NewProtoModel(m, cfg)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	return m, s.kernel.internalizer.quantifierManager.modelChecker, proto, q, zero
}

func TestSMTModelCheckerAuxiliaryQueriesReuseCleanScopes(t *testing.T) {
	_, c, proto, q, _ := smtAuxiliaryQueryFixture(t)
	for i := 0; i < 3; i++ {
		remaining := uint32(1)
		found, err := c.auxiliaryCounterexamples(proto, q, &remaining)
		astTestMust(t, err)
		if found || c.Exhausted() {
			t.Fatal("valid identity produced a counterexample")
		}
		a := c.auxiliaryContext
		level, err := a.ScopeLevel()
		astTestMust(t, err)
		if level != 0 || len(a.kernel.assertions) != 0 || a.kernel.Statistics().NumChecks != uint32(i+1) || a.kernel.relevancy.enabled || a.kernel.smtState.CaseSplitStrategy != 0 {
			t.Fatal("auxiliary query did not pop or reuse its context", level, a.kernel.Statistics())
		}
	}
}

func TestSMTModelCheckerAuxiliaryAllocationFrontiers(t *testing.T) {
	for _, query := range []bool{false, true} {
		succeeded := false
		for stop := int64(0); stop < 512 && !succeeded; stop++ {
			t.Run(fmt.Sprint(query, "/", stop), func(t *testing.T) {
				m, c, proto, q, _ := smtAuxiliaryQueryFixture(t)
				if query {
					astTestMust(t, c.initAuxiliaryContext())
				}
				m.Budget().FailAfter(stop)
				var err error
				if query {
					remaining := uint32(1)
					_, err = c.auxiliaryCounterexamples(proto, q, &remaining)
				} else {
					err = c.initAuxiliaryContext()
				}
				m.Budget().FailAfter(-1)
				if err == nil {
					succeeded = true
				} else if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal("unexpected auxiliary failure", err)
				}
				if !query && err != nil && c.auxiliaryContext != nil {
					t.Fatal("failed construction published a partial auxiliary")
				}
				if c.auxiliaryContext != nil {
					level, e := c.auxiliaryContext.ScopeLevel()
					astTestMust(t, e)
					if level != 0 {
						t.Fatal("stopped auxiliary query retained a scope", level)
					}
				}
				if err != nil {
					remaining := uint32(1)
					found, retryErr := c.auxiliaryCounterexamples(proto, q, &remaining)
					astTestMust(t, retryErr)
					if found || c.Exhausted() {
						t.Fatal("stopped auxiliary could not retry")
					}
				}
			})
			if t.Failed() {
				t.Fatalf("auxiliary frontier %v/%d", query, stop)
			}
		}
		if !succeeded {
			t.Fatal("no successful auxiliary allocation frontier", query)
		}
	}
}
