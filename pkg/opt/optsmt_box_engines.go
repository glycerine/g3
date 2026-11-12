package opt

// Source: src/opt/optsmt.cpp farkas_opt, update_upper and symba_opt.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// boxInfArith is farkas_opt/symba_opt's typeid(smt::theory_inf_arith)
// test on m_s->get_optimizer(), the theory every objective shares.
func (s *ArithmeticSolver) boxInfArith() (bool, error) {
	if len(s.objectives) == 0 {
		return false, nil
	}
	return s.backend.ObjectiveSupportsConflictRecording(s.objectives[0].backendIndex)
}

// farkasBox is optsmt::farkas_opt: update_upper rounds until one reports
// no progress, then the solution is set tight.
func (s *ArithmeticSolver) farkasBox(result *ArithmeticResult) (_ *ArithmeticResult, resultErr error) {
	supported, err := s.boxInfArith()
	if err != nil {
		return nil, err
	}
	models, release := s.boxModels()
	defer func() { resultErr = errors.Join(resultErr, release()) }()
	if !supported {
		return s.finishBox(result, models, lbool.Undef)
	}
	status := lbool.True
	for status == lbool.True {
		canceled, err := s.manager.Canceled()
		if err != nil {
			return nil, err
		}
		if canceled {
			break
		}
		if status, err = s.farkasUpdateUpper(result, models); err != nil {
			return nil, err
		}
	}
	return s.finishBox(result, models, status)
}

// farkasUpdateUpper is optsmt::update_upper. Every bound expression is
// created before check_sat, at the scoped push's base level.
func (s *ArithmeticSolver) farkasUpdateUpper(result *ArithmeticResult, models *OptsmtModels) (status lbool.LBool, resultErr error) {
	// solver::scoped_push _push(*m_s)
	if err := s.backend.Push(); err != nil {
		return lbool.Undef, err
	}
	// expr_ref_vector bounds(m) and the bound local die before the pop.
	var bounds []ast.ASTRef
	var bound ast.ASTRef
	defer func() {
		// Destruction runs in reverse declaration order: _push, bounds,
		// bound.
		resultErr = errors.Join(resultErr, s.backend.Pop(1))
		for i := range bounds {
			if bounds[i] != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, s.manager.DelRef(&bounds[i]))
			}
		}
		if bound != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, s.manager.DelRef(&bound))
		}
		if resultErr != nil {
			status = lbool.Undef
		}
	}()
	n := len(s.objectives)
	mid := make([]simplex.Infinitesimal, 0, n)
	bounds = make([]ast.ASTRef, 0, n)
	for k := 0; k < n; k++ {
		canceled, err := s.manager.Canceled()
		if err != nil {
			return lbool.Undef, err
		}
		if canceled {
			break
		}
		i := s.boxOrigin(k)
		lower, upper := s.optLowerOf(i), s.optUpperOf(i)
		if lower.Cmp(&upper) < 0 {
			m := upper.Clone()
			m.Add(&lower)
			m.Quo(big.NewRat(2, 1))
			mid = append(mid, m)
			ge, err := s.backend.OptimizationObjectiveGE(s.objectives[k].backendIndex, s.filter, m)
			if err != nil {
				return lbool.Undef, err
			}
			held, err := s.manager.NewRef(ge)
			if err != nil {
				return lbool.Undef, err
			}
			if bound != (ast.ASTRef{}) {
				if err = s.manager.DelRef(&bound); err != nil {
					return lbool.Undef, errors.Join(err, s.manager.DelRef(&held))
				}
			}
			bound = held
			copied, err := s.manager.NewRef(ge)
			if err != nil {
				return lbool.Undef, err
			}
			bounds = append(bounds, copied)
		} else {
			bounds = append(bounds, ast.ASTRef{})
			mid = append(mid, simplex.Infinitesimal{})
		}
	}
	progress := false
	for k := 0; k < n && k < len(mid); k++ {
		canceled, err := s.manager.Canceled()
		if err != nil {
			return lbool.Undef, err
		}
		if canceled {
			break
		}
		i := s.boxOrigin(k)
		lower, upper := s.optLowerOf(i), s.optUpperOf(i)
		if !(lower.Cmp(&mid[k]) <= 0 && mid[k].Cmp(&upper) <= 0 && lower.Cmp(&upper) < 0) {
			continue
		}
		index := s.objectives[k].backendIndex
		h, err := s.manager.RefGet(bounds[k])
		if err != nil {
			return lbool.Undef, err
		}
		if err = s.backend.EnableObjectiveConflictRecording(index, h); err != nil {
			return lbool.Undef, err
		}
		checked, err := s.backend.CheckSat([]ast.ASTHandle{h})
		if err != nil {
			return lbool.Undef, err
		}
		switch checked {
		case lbool.True:
			s.setOptLower(i, mid[k])
			if err = s.backend.EnableObjectiveConflictRecording(index, ast.ASTHandle{}); err != nil {
				return lbool.Undef, err
			}
			// m_s->assert_expr(update_lower())
			if err = s.replaceModel(result); err != nil {
				return lbool.Undef, err
			}
			lowered, err := s.boxUpdateLower(models)
			if err != nil {
				return lbool.Undef, err
			}
			f, err := s.manager.RefGet(lowered)
			if err == nil {
				err = s.backend.Assert(f)
			}
			if err = errors.Join(err, s.manager.DelRef(&lowered)); err != nil {
				return lbool.Undef, err
			}
		case lbool.False:
			minimum, finite, err := s.backend.ObjectiveConflictMinimize(index)
			if err != nil {
				return lbool.Undef, err
			}
			if !finite {
				// bounds is not in the core. The context is unsat.
				s.setOptUpper(i, lower)
				return lbool.False, nil
			}
			if minimum.Cmp(&upper) < 0 {
				s.setOptUpper(i, minimum)
			}
		default:
			if err = s.backend.EnableObjectiveConflictRecording(index, ast.ASTHandle{}); err != nil {
				return lbool.Undef, err
			}
			return lbool.Undef, nil
		}
		if err = s.backend.EnableObjectiveConflictRecording(index, ast.ASTHandle{}); err != nil {
			return lbool.Undef, err
		}
		progress = true
	}
	canceled, err := s.manager.Canceled()
	if err != nil {
		return lbool.Undef, err
	}
	if canceled {
		return lbool.Undef, nil
	}
	if !progress {
		return lbool.False, nil
	}
	return lbool.True, nil
}

// symbaBox is optsmt::symba_opt: guarded rounds ask whether some objective
// reaches its upper end m_upper; afterwards mk_or(m_lower_fmls) is
// asserted and geometric_opt finishes. ors, disj, fml, bound and tmp are
// symba_opt's locals and outlive geometric_opt.
func (s *ArithmeticSolver) symbaBox(result *ArithmeticResult) (_ *ArithmeticResult, resultErr error) {
	m := s.manager
	models, release := s.boxModels()
	defer func() { resultErr = errors.Join(resultErr, release()) }()
	supported, err := s.boxInfArith()
	if err != nil {
		return nil, err
	}
	if !supported {
		return s.finishBox(result, models, lbool.Undef)
	}
	status, err := func() (_ lbool.LBool, resultErr error) {
		// expr_ref_vector ors(m), disj(m);
		// expr_ref fml(m), bound(m.mk_true(), m), tmp(m);
		var ors, disj []ast.ASTRef
		fml, bound, tmp := &maxResLocal{m: m}, &maxResLocal{m: m}, &maxResLocal{m: m}
		releaseVector := func(v []ast.ASTRef) {
			for i := range v {
				if v[i] != (ast.ASTRef{}) {
					resultErr = errors.Join(resultErr, m.DelRef(&v[i]))
				}
			}
		}
		defer func() {
			// Destruction runs in reverse declaration order.
			resultErr = errors.Join(resultErr, tmp.release(), bound.release(), fml.release())
			releaseVector(disj)
			releaseVector(ors)
		}()
		if _, err := bound.set(m.MkTrue()); err != nil {
			return lbool.Undef, err
		}
		status, err := s.symbaRounds(result, models, &ors, &disj, fml, tmp)
		if err != nil || status == lbool.Undef {
			return lbool.Undef, err
		}
		// bound = mk_or(m_lower_fmls); m_s->assert_expr(bound)
		args := make([]ast.ASTHandle, len(s.objectives))
		for k := range args {
			if args[k], err = s.lowerFormula(s.boxOrigin(k)); err != nil {
				return lbool.Undef, err
			}
		}
		or, err := bound.set(m.MkOrArray(args))
		if err != nil {
			return lbool.Undef, err
		}
		if err = s.backend.Assert(or); err != nil {
			return lbool.Undef, err
		}
		canceled, err := m.Canceled()
		if err != nil || canceled {
			return lbool.Undef, err
		}
		return s.geometricOpt(result, models)
	}()
	if err != nil {
		return nil, err
	}
	return s.finishBox(result, models, status)
}

// symbaRounds is symba_opt's scoped block: the scoped push ends with it.
func (s *ArithmeticSolver) symbaRounds(result *ArithmeticResult, models *OptsmtModels, ors, disj *[]ast.ASTRef, fml, tmp *maxResLocal) (_ lbool.LBool, resultErr error) {
	m := s.manager
	releaseVector := func(v []ast.ASTRef) error {
		var result error
		for i := range v {
			if v[i] != (ast.ASTRef{}) {
				result = errors.Join(result, m.DelRef(&v[i]))
			}
		}
		return result
	}
	for k := range s.objectives {
		upper := s.optUpperOf(s.boxOrigin(k))
		ge, err := s.backend.OptimizationObjectiveGE(s.objectives[k].backendIndex, s.filter, upper)
		if err != nil {
			return lbool.Undef, err
		}
		ref, err := m.NewRef(ge)
		if err != nil {
			return lbool.Undef, err
		}
		*ors = append(*ors, ref)
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return lbool.Undef, err
	}
	// fml = mk_or(ors); tmp = fresh b; fml = (tmp => fml); vars[0] = tmp
	guard := func() (ast.ASTHandle, error) {
		args := make([]ast.ASTHandle, len(*ors))
		for i := range *ors {
			var err error
			if args[i], err = m.RefGet((*ors)[i]); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		current, err := fml.set(m.MkOrArray(args))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		fresh, err := tmp.set(m.MkFreshConst("b", boolean))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if _, err = fml.set(m.MkImplies(fresh, current)); err != nil {
			return ast.ASTHandle{}, err
		}
		return fresh, nil
	}
	variable, err := guard()
	if err != nil {
		return lbool.Undef, err
	}
	// solver::scoped_push _push(*m_s)
	if err = s.backend.Push(); err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.backend.Pop(1)) }()
	for {
		canceled, err := m.Canceled()
		if err != nil {
			return lbool.Undef, err
		}
		if canceled {
			return lbool.True, nil
		}
		f, err := m.RefGet(fml.ref)
		if err != nil {
			return lbool.Undef, err
		}
		if err = s.backend.Assert(f); err != nil {
			return lbool.Undef, err
		}
		checked, err := s.backend.CheckSat([]ast.ASTHandle{variable})
		if err != nil {
			return lbool.Undef, err
		}
		if checked == lbool.Undef {
			return lbool.Undef, nil
		}
		if checked != lbool.True {
			return lbool.True, nil
		}
		// disj.reset(); m_s->maximize_objectives(disj)
		if err = releaseVector(*disj); err != nil {
			return lbool.Undef, err
		}
		*disj = (*disj)[:0]
		values, err := s.boxMaximizeObjectives(disj)
		if err != nil {
			return lbool.Undef, err
		}
		// m_s->get_model(m_model)
		if err = s.replaceModel(result); err != nil {
			return lbool.Undef, err
		}
		for k := range *ors {
			// expr_ref tmp(m); m_model->eval(ors[i], tmp) && m.is_true(tmp)
			var val ast.ASTRef
			term, err := m.RefGet((*ors)[k])
			if err == nil {
				err = s.evalInto(result.Model, term, &val)
			}
			var reached bool
			if err == nil {
				var h ast.ASTHandle
				if h, err = m.RefGet(val); err == nil {
					reached = m.IsTrue(h)
				}
			}
			if val != (ast.ASTRef{}) {
				err = errors.Join(err, m.DelRef(&val))
			}
			if err != nil {
				return lbool.Undef, err
			}
			if !reached {
				continue
			}
			i := s.boxOrigin(k)
			s.setOptLower(i, s.optUpperOf(i))
			falsity, err := m.MkFalse()
			if err != nil {
				return lbool.Undef, err
			}
			for _, slot := range []*ast.ASTRef{&(*ors)[k], &(*disj)[k]} {
				held, err := m.NewRef(falsity)
				if err == nil {
					err = m.DelRef(slot)
				}
				*slot = held
				if err != nil {
					return lbool.Undef, err
				}
			}
		}
		if err = s.boxSetMax(models, values, *disj); err != nil {
			return lbool.Undef, err
		}
		if variable, err = guard(); err != nil {
			return lbool.Undef, err
		}
	}
}
