package smtlogics

// Source: src/tactic/smtlogics/qfnra_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewQFNRATactic retains the source route order. The available simplify and
// propagation prefix handles the polynomial CNF fragment; unsupported
// preprocessing cases fall through to the bounded and SMT routes.
func NewQFNRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
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
	copyParams := func() *uparams.ParamsRef {
		value := b.params()
		if b.err == nil && p != nil {
			b.err = c.Assign(value, p)
		}
		return value
	}
	main := copyParams()
	p0, p1, p2 := copyParams(), copyParams(), copyParams()
	b.boolean(p0, "inline_vars", true)
	b.uint(p1, "seed", 11)
	b.boolean(p1, "factor", false)
	b.uint(p2, "seed", 13)
	b.boolean(p2, "factor", false)
	// mk_qfnra_sat_solver: nra2sat_p = p with nla2bv_max_bv_size set to
	// p.get_uint("nla2bv_max_bv_size", bv_size). These are only constructor
	// params: updt_params later replaces them (nla2bv's m_params = p).
	boundedParams := func(size uint32) *uparams.ParamsRef {
		value := copyParams()
		if b.err != nil {
			return value
		}
		source := p
		if source == nil {
			empty := c.EmptyValue()
			source = &empty
		}
		key := "nla2bv_max_bv_size"
		n, e := c.GetUIntName(source, &key, size)
		if e != nil {
			b.err = e
			return value
		}
		b.uint(value, key, n)
		return value
	}
	bounded4, bounded6 := boundedParams(4), boundedParams(6)
	if b.err != nil {
		return nil, b.err
	}
	simplify := func(params *uparams.ParamsRef) tactic.Tactic {
		return b.using(b.clean(b.keep(core.NewSimplifyTactic(m, c, g, nil))), params)
	}
	// try_for(mk_qfnra_nlsat_tactic(m, pN), ms): the same strategy as the
	// registered qfnra-nlsat tactic (which adds elim_and/blast_distinct and
	// reads factor itself), with no fail_if_undecided after it.
	nlsatRoute := func(params *uparams.ParamsRef, timeout uint32) tactic.Tactic {
		route := b.keep(NewQFNRANLSATTactic(m, params, config))
		if timeout != 0 {
			route = b.timed(route, timeout)
		}
		return route
	}
	bounded := func(params *uparams.ParamsRef) tactic.Tactic {
		return b.keep(qfnraSATSolverRoute(m, params, config))
	}
	timedSMT := b.then(
		b.timed(b.keep(smtcore.NewSMTTactic(m, nil, config)), 5000),
		b.keep(tactic.NewFailIfUndecidedTactic(), nil),
	)
	body := b.then(
		simplify(main),
		b.keep(core.NewPropagateValuesTactic(m, c, g, nil)),
		b.otherwise(
			nlsatRoute(p0, 5000),
			nlsatRoute(p1, 10000),
			bounded(bounded4),
			timedSMT,
			bounded(bounded6),
			nlsatRoute(p2, 0),
		),
	)
	if b.err != nil {
		return nil, b.err
	}
	result, err = tactic.NewUsingParamsTactic(c, body, main)
	if err != nil {
		return nil, err
	}
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	err = result.UpdateParams(p)
	return result, err
}

// qfnraSATSolverRoute is the and_then of mk_qfnra_sat_solver, built from the
// already adjusted nra2sat_p. nla2bv keeps the params only until its next
// updt_params, which replaces them (m_params = p).
func qfnraSATSolverRoute(m *ast.ASTManager, params *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
	b := qfliaConstruction{c: config.Params, m: m}
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
	nla2bv := b.keep(arith.NewNLA2BVTactic(m, config.Params, config.Globals, params, config.ModelEvaluator))
	smt := b.keep(smtcore.NewSMTTactic(m, nil, config))
	undecided := b.keep(tactic.NewFailIfUndecidedTactic(), nil)
	if b.err != nil {
		return nil, b.err
	}
	return tactic.NewAndThenTactic(nla2bv, smt, undecided)
}
