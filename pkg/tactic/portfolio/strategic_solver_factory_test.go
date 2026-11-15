package portfolio_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestStrategicSolverFactoryPinsLogicAndSessionOutput(t *testing.T) {
	f := enumSolverFixture(t)
	enumMust(t, f.services.Globals.SourceSetName("sat.dimacs.display", "true"))
	t.Cleanup(func() { enumMust(t, f.services.Globals.SourceSetName("sat.dimacs.display", "false")) })
	var output strings.Builder
	factory, err := portfolio.NewSMTStrategicSolverFactoryWithOutput("QF_BV", smtcore.SMTSolverConfig{
		Params:         f.services.Context,
		Globals:        f.services.Globals,
		ModelEvaluator: f.services,
	}, &output)
	enumMust(t, err)
	t.Cleanup(func() { enumMust(t, factory.Close()) })

	s, err := factory.New(f.manager, nil, false, false, false, "QF_UF")
	enumMust(t, err)
	t.Cleanup(func() { enumMust(t, s.Close()) })
	sort, err := f.manager.MkBoolSort()
	enumMust(t, err)
	pName, err := f.manager.Symbols.Intern("strategic-factory-p")
	enumMust(t, err)
	qName, err := f.manager.Symbols.Intern("strategic-factory-q")
	enumMust(t, err)
	p, err := f.manager.MkConst(pName, sort)
	enumMust(t, err)
	q, err := f.manager.MkConst(qName, sort)
	enumMust(t, err)
	formula, err := f.manager.MkOr(p, q)
	enumMust(t, err)
	enumPin(t, f.manager, formula)
	enumMust(t, s.Push())
	enumMust(t, s.Assert(formula))
	status, err := s.CheckSat(nil)
	enumMust(t, err)
	if status != lbool.True || !strings.Contains(output.String(), "p cnf") {
		t.Fatal("stored QF_BV logic did not select incremental SAT", status, output.String())
	}

	enumMust(t, factory.Close())
	if _, err = factory.New(f.manager, nil, false, false, false, "QF_BV"); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed strategic factory accepted construction", err)
	}
}

// AUDIT.md tactic_small_02-F1: mk_solver_for_logic picks inc_sat for QF_BV
// only when the global rewriter.hi_div0 holds; otherwise the incremental
// solver is the SMT solver.
func TestQFBVStrategicSolverHiDiv0FalseUsesSMTBackend(t *testing.T) {
	f := enumSolverFixture(t)
	enumMust(t, f.services.Globals.SourceSetName("sat.dimacs.display", "true"))
	t.Cleanup(func() { enumMust(t, f.services.Globals.SourceSetName("sat.dimacs.display", "false")) })
	enumMust(t, f.services.Globals.SourceSetName("rewriter.hi_div0", "false"))
	t.Cleanup(func() { enumMust(t, f.services.Globals.SourceSetName("rewriter.hi_div0", "true")) })
	var output strings.Builder
	factory, err := portfolio.NewSMTStrategicSolverFactoryWithOutput("QF_BV", smtcore.SMTSolverConfig{
		Params:         f.services.Context,
		Globals:        f.services.Globals,
		ModelEvaluator: f.services,
	}, &output)
	enumMust(t, err)
	t.Cleanup(func() { enumMust(t, factory.Close()) })
	s, err := factory.New(f.manager, nil, false, false, false, "QF_BV")
	enumMust(t, err)
	t.Cleanup(func() { enumMust(t, s.Close()) })
	sort, err := f.manager.MkBoolSort()
	enumMust(t, err)
	pName, err := f.manager.Symbols.Intern("hi-div0-p")
	enumMust(t, err)
	qName, err := f.manager.Symbols.Intern("hi-div0-q")
	enumMust(t, err)
	p, err := f.manager.MkConst(pName, sort)
	enumMust(t, err)
	q, err := f.manager.MkConst(qName, sort)
	enumMust(t, err)
	formula, err := f.manager.MkOr(p, q)
	enumMust(t, err)
	enumPin(t, f.manager, formula)
	enumMust(t, s.Push())
	enumMust(t, s.Assert(formula))
	status, err := s.CheckSat(nil)
	enumMust(t, err)
	if status != lbool.True || strings.Contains(output.String(), "p cnf") {
		t.Fatal("hi_div0=false still selected incremental SAT", status, output.String())
	}
}
