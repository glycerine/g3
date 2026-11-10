package opt

// Source: src/opt/maxsmt.cpp/maxres.cpp filter_model_converter publication.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func (s *ArithmeticSolver) filterResultModel(result *ArithmeticResult) (resultErr error) {
	if result == nil || result.Model == nil || result.ModelFiltered {
		return nil
	}
	if s.filter == nil && s.preprocessor == nil {
		return nil
	}
	var ref tactic.ModelRef
	if err := ref.Set(result.Model); err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, ref.Close()) }()
	if s.preprocessor != nil {
		if err := s.preprocessor.ApplyModel(&ref, 0); err != nil {
			return err
		}
	}
	if s.filter != nil {
		if err := s.filter.ApplyUnindexed(&ref); err != nil {
			return err
		}
	}
	filtered, err := ref.Get()
	if err != nil || filtered == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTHandle
	}
	if err = filtered.IncRef(); err != nil {
		return err
	}
	if err = result.Model.DecRef(); err != nil {
		return errors.Join(err, filtered.DecRef())
	}
	result.Model = filtered
	result.ModelFiltered = true
	return nil
}

// FixModel is opt::context::fix_model applied to mdl = md: the model
// converter, then m_fm. md stays owned by the caller; the result is a new
// reference (md itself when no converter replaces it). Converters that update
// a model in place update md, as the source's model_ref aliasing does.
func (s *ArithmeticSolver) FixModel(md *model.ModelCore) (result *model.ModelCore, resultErr error) {
	if s == nil || md == nil {
		return nil, ast.ErrASTHandle
	}
	var ref tactic.ModelRef
	if err := ref.Set(md); err != nil {
		return nil, errors.Join(err, ref.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, ref.Close()) }()
	if s.preprocessor != nil {
		if err := s.preprocessor.ApplyModel(&ref, 0); err != nil {
			return nil, err
		}
	}
	if s.filter != nil {
		if err := s.filter.ApplyUnindexed(&ref); err != nil {
			return nil, err
		}
	}
	fixed, err := ref.Get()
	if err != nil || fixed == nil {
		return nil, errors.Join(err, ast.ErrASTHandle)
	}
	if err = fixed.IncRef(); err != nil {
		return nil, err
	}
	return fixed, nil
}
