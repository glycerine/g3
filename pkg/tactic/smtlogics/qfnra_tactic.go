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
	bounded4, bounded6 := copyParams(), copyParams()
	b.uint(bounded4, "nla2bv_max_bv_size", 4)
	b.uint(bounded6, "nla2bv_max_bv_size", 6)
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
		return b.then(
			b.using(b.keep(arith.NewNLA2BVTactic(m, c, g, params, config.ModelEvaluator)), params),
			b.keep(smtcore.NewSMTTactic(m, nil, config)),
			b.keep(tactic.NewFailIfUndecidedTactic(), nil),
		)
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
