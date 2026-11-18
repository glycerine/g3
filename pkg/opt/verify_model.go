package opt

// Source: src/opt/opt_context.cpp verify_model and fix_model;
// src/opt/opt_solver.h adjust_value.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// objectiveVerification is opt::context::objective's m_term and
// m_adjust_value for a MaxSMT objective converted from maximize/minimize.
type objectiveVerification struct {
	term   ast.ASTRef
	offset numeral.Rational // owned
	negate bool
}

// SetObjectiveVerification records m_term and m_adjust_value for objective
// index (from_fmls: obj.m_term = orig_term, set_offset, set_negate). Without
// it verify_model accepts every model, as for a null m_term.
// offset is borrowed; the verification keeps a copy.
func (s *ArithmeticSolver) SetObjectiveVerification(index int, term ast.ASTHandle, offset *numeral.Rational, negate bool) error {
	if s == nil || index < 0 || index >= len(s.objectives) || term.IsNull() || offset == nil {
		return ast.ErrASTHandle
	}
	ref, err := s.manager.NewRef(term)
	if err != nil {
		return err
	}
	for len(s.verifications) <= index {
		s.verifications = append(s.verifications, nil)
	}
	o := newRatOps(s.manager)
	if old := s.verifications[index]; old != nil {
		o.del(&old.offset)
		if err = o.joined(s.manager.DelRef(&old.term)); err != nil {
			return errors.Join(err, s.manager.DelRef(&ref))
		}
	}
	v := &objectiveVerification{term: ref, negate: negate, offset: o.clone(offset)}
	s.verifications[index] = v
	return o.err
}

func (s *ArithmeticSolver) closeVerifications() error {
	var err error
	for _, v := range s.verifications {
		if v != nil {
			o := newRatOps(s.manager)
			o.del(&v.offset)
			err = errors.Join(err, s.manager.DelRef(&v.term), o.err)
		}
	}
	s.verifications = nil
	return err
}

// verifyModel is context::verify_model(index, md, v): a copy of md is fixed
// with the model converter and the filter, then m_term must evaluate (without
// model completion) to the numeral m_adjust_value(v).
func (s *ArithmeticSolver) verifyModel(index int, md *model.ModelCore, v *numeral.Rational) (ok bool, resultErr error) {
	if index < 0 || index >= len(s.verifications) || s.verifications[index] == nil {
		return true, nil
	}
	verification := s.verifications[index]
	m := s.manager
	term, err := m.RefGet(verification.term)
	if err != nil {
		return false, err
	}
	o := newRatOps(m)
	adjusted := o.zero()
	defer func() {
		o.del(&adjusted)
		resultErr = o.joined(resultErr)
	}()
	s.adjustSoftBound(&o, index, v, &adjusted)
	if !o.ok() {
		return false, nil
	}
	// model_ref mdl = md->copy();
	copied, err := copyModelCore(m, md)
	if err != nil {
		if copied != nil {
			err = errors.Join(err, copied.Close())
		}
		return false, err
	}
	// mdl holds the copy's only reference; releasing it destroys the copy.
	var ref tactic.ModelRef
	if err = ref.Set(copied.Core()); err != nil {
		return false, errors.Join(err, copied.Close(), ref.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, ref.Close()) }()
	// fix_model(mdl)
	if s.preprocessor != nil {
		if err = s.preprocessor.ApplyModel(&ref, 0); err != nil {
			return false, err
		}
	}
	if s.filter != nil {
		if err = s.filter.ApplyUnindexed(&ref); err != nil {
			return false, err
		}
	}
	fixed, err := ref.Get()
	if err != nil || fixed == nil {
		return false, errors.Join(err, ast.ErrASTHandle)
	}
	evaluator, err := model.NewModelEvaluator(fixed, s.config.ModelEvaluator)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	valRef, err := evaluator.EvaluateValue(term)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&valRef)) }()
	val, err := m.RefGet(valRef)
	if err != nil {
		return false, err
	}
	r := o.zero()
	defer o.del(&r)
	matched, err := numeralRational(m, val, &r)
	if err != nil || !matched {
		return false, err
	}
	return o.cmp(&r, &adjusted) == 0, nil
}

// numeralRational is is_numeral for arithmetic or bit-vector values,
// copying the value into out.
func numeralRational(m *ast.ASTManager, h ast.ASTHandle, out *numeral.Rational) (bool, error) {
	_, matched, err := m.ArithmeticNumeralRational(h, out)
	if err != nil || matched {
		return matched, err
	}
	_, matched, err = m.BitVectorNumeralRational(h, out)
	return matched, err
}

// copyModelCore is model::copy for a model core: constants, functions and,
// for a full model, universes.
func copyModelCore(m *ast.ASTManager, md *model.ModelCore) (*model.Model, error) {
	if derived := md.DerivedModel(); derived != nil {
		return derived.Copy()
	}
	copied, err := model.NewModel(m)
	if err != nil {
		return copied, err
	}
	if err = copied.Core().CopyConstantInterpretations(md); err == nil {
		err = copied.Core().CopyFunctionInterpretations(md)
	}
	if err != nil {
		return nil, errors.Join(err, copied.Close())
	}
	return copied, nil
}
