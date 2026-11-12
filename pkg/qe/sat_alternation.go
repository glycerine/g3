package qe

// Source: src/qe/qe_sat_tactic.cpp, skolemize_existential_prefix and
// the quantifier-extraction portion of extract_alt_form.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type satAlternation struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables [][]ast.ASTRef
	// hoisted is extract_alt_form's local result: the last hoist output,
	// alive until extract_alt_form returns after init_Ms.
	hoisted ast.ASTRef
	closed  bool
}

func extractSATAlternation(m *ast.ASTManager, config Config, formula ast.ASTHandle) (_ *satAlternation, resultErr error) {
	if m == nil || config.Context == nil || config.Globals == nil {
		return nil, ast.ErrASTHandle
	}
	hoist, err := rewriter.NewQuantifierHoister(m, config.Context, config.Globals, nil)
	if err != nil {
		if hoist != nil {
			err = errors.Join(err, hoist.Close())
		}
		return nil, err
	}
	result := &satAlternation{manager: m}
	defer func() {
		resultErr = errors.Join(resultErr, hoist.Close())
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	prefix, err := hoist.PullExists(formula, true, true)
	if err != nil {
		return nil, err
	}
	result.formula, err = m.CopyRef(prefix.Formula)
	err = errors.Join(err, prefix.Close())
	if err != nil {
		return nil, err
	}
	for {
		current, e := m.RefGet(result.formula)
		if e != nil {
			return nil, e
		}
		block, e := hoist.Hoist(current, true, true)
		if e != nil {
			return nil, e
		}
		if len(block.Variables) == 0 {
			if result.hoisted, e = m.CopyRef(block.Formula); e != nil {
				return nil, errors.Join(e, block.Close())
			}
			if e = block.Close(); e != nil {
				return nil, e
			}
			break
		}
		if block.IsForall != (len(result.variables)%2 == 0) {
			return nil, errors.Join(ast.ErrASTDomain, block.Close())
		}
		var refs []ast.ASTRef
		for _, v := range block.Variables {
			ref, cause := m.CopyRef(v)
			if cause != nil {
				for i := range refs {
					cause = errors.Join(cause, m.DelRef(&refs[i]))
				}
				return nil, errors.Join(cause, block.Close())
			}
			refs = append(refs, ref)
		}
		result.variables = append(result.variables, refs)
		e = m.AssignRef(result.formula, block.Formula)
		e = errors.Join(e, block.Close())
		if e != nil {
			return nil, e
		}
	}
	if len(result.variables)%2 == 1 {
		current, e := m.RefGet(result.formula)
		if e != nil {
			return nil, e
		}
		negated, e := m.MkNot(current)
		if e != nil {
			return nil, e
		}
		if e = m.SetRef(result.formula, negated); e != nil {
			return nil, e
		}
	}
	return result, nil
}

// releaseHoisted ends extract_alt_form's scope.
func (s *satAlternation) releaseHoisted() error {
	if s == nil || s.hoisted == (ast.ASTRef{}) {
		return nil
	}
	return s.manager.DelRef(&s.hoisted)
}

// releaseFormula is m_fml = nullptr.
func (s *satAlternation) releaseFormula() error {
	if s == nil || s.formula == (ast.ASTRef{}) {
		return nil
	}
	return s.manager.DelRef(&s.formula)
}

// Close releases m_fml, then m_vars (vector::reset destroys each
// app_ref_vector in order, each dropping its references forward).
func (s *satAlternation) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if err := s.releaseHoisted(); err != nil {
		return err
	}
	if err := s.releaseFormula(); err != nil {
		return err
	}
	for i := range s.variables {
		for j := range s.variables[i] {
			if s.variables[i][j] != (ast.ASTRef{}) {
				if err := s.manager.DelRef(&s.variables[i][j]); err != nil {
					return err
				}
			}
		}
	}
	s.variables = nil
	s.closed = true
	return nil
}
