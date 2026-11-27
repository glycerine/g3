package portfolio

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/bv"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// The default strategy constructs two QSAT kernels and a BV1 blaster before
// its incremental SMT backend. The constructors intern the bit-zero, QI cost
// and bit-one terms in that order. One owner of each keeps their IDs stable
// in this arena. Quantifier matching and search tie breaks observe these IDs.
// The remaining tactic objects can wait until the first tactic operation.
type deferredDefaultTactic struct {
	config smtcore.SMTSolverConfig
	params uparams.ParamsRef
	bit0   *rewriter.Bit2Int
	cost   *smtcore.SMTQICostModel
	bv1    *bv.BV1BlasterTactic
}

func newDeferredDefaultTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (_ *deferredDefaultTactic, err error) {
	f := &deferredDefaultTactic{config: config, params: config.Params.EmptyValue()}
	defer func() {
		if err != nil {
			err = errors.Join(err, f.Close())
		}
	}()
	if p != nil {
		if err = config.Params.Copy(&f.params, p); err != nil {
			return nil, err
		}
	}
	f.bit0, err = rewriter.NewBit2Int(m)
	if err != nil {
		return nil, err
	}
	f.cost, err = newDefaultQICost(m, config)
	if err != nil {
		return nil, err
	}
	f.bv1, err = bv.NewBV1BlasterTactic(m, config.Params, config.Globals, p, config.ModelEvaluator.Cooperation)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func newDefaultQICost(m *ast.ASTManager, config smtcore.SMTSolverConfig) (_ *smtcore.SMTQICostModel, err error) {
	// These nested QSAT kernels use empty local params. Honor the SMT
	// module's configured qi.cost and qi.new_gen when interning their terms.
	empty := config.Params.EmptyValue()
	qi, err := smtcore.NewSMTQIParams(config.Params, &empty, config.Globals, m.Budget())
	if qi != nil {
		defer func() { err = errors.Join(err, qi.Close()) }()
	}
	if err != nil {
		return nil, err
	}
	cost, err := qi.CostText()
	if err != nil {
		return nil, err
	}
	generation, err := qi.NewGenerationText()
	if err != nil {
		return nil, err
	}
	return smtcore.NewSMTQICostModel(m, cost, generation, nil)
}

func (f *deferredDefaultTactic) New(m *ast.ASTManager, _ *uparams.ParamsRef) (tactic.Tactic, error) {
	// CheckSatCore installs the current params after construction, as it
	// does on the eager solver. Constructors see the original local params.
	return newDefaultStrategicTactic(m, &f.params, f.config)
}

func (f *deferredDefaultTactic) Close() error {
	if f.bv1 != nil {
		if err := f.bv1.Close(); err != nil {
			return err
		}
		f.bv1 = nil
	}
	if f.cost != nil {
		if err := f.cost.Close(); err != nil {
			return err
		}
		f.cost = nil
	}
	if f.bit0 != nil {
		if err := f.bit0.Close(); err != nil {
			return err
		}
		f.bit0 = nil
	}
	if f.params != (uparams.ParamsRef{}) {
		return f.config.Params.Del(&f.params)
	}
	return nil
}

func newIncrementalDefaultSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	factory, err := newDeferredDefaultTactic(m, p, config)
	if err != nil {
		return nil, err
	}
	config.ReduceScopedClauses = !proofs && !cores
	second, err := smtcore.NewSMTSolver(m, p, "ALL", config)
	if err != nil {
		return nil, errors.Join(err, factory.Close())
	}
	first, err := solver.NewDeferredTacticSolver(m, factory, p, proofs, models, cores, "ALL", strategicTacticSolverConfig(config))
	if err != nil {
		return nil, errors.Join(err, second.Close(), factory.Close())
	}
	return combineStrategicSolvers(first, second, p, config)
}
