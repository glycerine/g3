package smtlogics

// Source: src/tactic/smtlogics/nra_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewNRATactic is mk_nra_tactic: a quantifier-free arm of three NLSAT
// attempts and the NLQSAT-first quantified fallback.
func NewNRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
	c, g := config.Params, config.Globals
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	b := qfliaConstruction{c: c, m: m}
	defer func() {
		if cleanup := b.close(); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
		if err != nil && result != nil {
			if acquire := result.IncRef(); acquire != nil {
				err = errors.Join(err, acquire)
			} else {
				err = errors.Join(err, result.DecRef())
			}
			result = nil
		}
	}()
	// cond(mk_is_qfnra_probe(), or_else(try_for(mk_qfnra_nlsat_tactic(m, p),
	// 5000), try_for(mk_qfnra_nlsat_tactic(m, p1), 10000),
	// mk_qfnra_nlsat_tactic(m, p2)), ...)
	copyParams := func() *uparams.ParamsRef {
		value := b.params()
		if b.err == nil && p != nil {
			b.err = c.Assign(value, p)
		}
		return value
	}
	p1, p2 := copyParams(), copyParams()
	b.uint(p1, "seed", 11)
	b.boolean(p1, "factor", false)
	b.uint(p2, "seed", 13)
	b.boolean(p2, "factor", false)
	if b.err != nil {
		return nil, b.err
	}
	qf := b.otherwise(
		b.timed(b.keep(NewQFNRANLSATTactic(m, p, config)), 5000),
		b.timed(b.keep(NewQFNRANLSATTactic(m, p1, config)), 10000),
		b.keep(NewQFNRANLSATTactic(m, p2, config)),
	)
	nlq := b.keep(qe.NewNLQSATTacticWithParams(m, p, qe.Config{
		Context: c, Globals: g,
		Cooperation: config.ModelEvaluator.Cooperation,
		Trace:       config.ModelEvaluator.Trace,
	}))
	fallback := b.otherwise(nlq, b.keep(smtcore.NewSMTTactic(m, nil, config)))
	qfnra := b.probe(arith.NewArithmeticFragmentProbe(m.Budget(), c, g, arith.FragmentQFNRA))
	backend := b.keep(tactic.NewCondTactic(qfnra, qf, fallback))
	if b.err != nil {
		return nil, b.err
	}
	result, err = tactic.NewAndThenTactic(
		b.keep(core.MakeSimplifyTactic(m, c, g, p)),
		b.keep(core.NewPropagateValuesTactic(m, c, g, p)),
		b.keep(qe.NewLiteTactic(m, p, qe.Config{
			Context: c, Globals: g,
			Cooperation: config.ModelEvaluator.Cooperation,
			Trace:       config.ModelEvaluator.Trace,
		})),
		backend,
	)
	if b.err != nil {
		return nil, b.err
	}
	return result, err
}
