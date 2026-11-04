package qe

// Source: src/qe/qe.cpp, extract_vars.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type exprQuantifierExtract struct {
	manager   *ast.ASTManager
	body      ast.ASTRef
	variables []ast.ASTRef
}

func extractExprQuantifier(m *ast.ASTManager, quantifier ast.ASTHandle) (result *exprQuantifierExtract, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	result = &exprQuantifierExtract{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	body, err := m.GetQuantifierBody(quantifier)
	if err != nil {
		return result, err
	}
	result.body, err = m.NewRef(body)
	if err != nil {
		return result, err
	}
	n, err := m.GetNumDeclarations(quantifier)
	if err != nil {
		return result, err
	}
	args := make([]ast.ASTHandle, 0, n)
	for i := uint32(0); i < n; i++ {
		sort, e := m.GetDeclarationSort(quantifier, i)
		if e != nil {
			return result, e
		}
		// Own the reference slot before creating the fresh declaration and
		// constant: allocating the slot afterwards can fail and strand both.
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return result, e
		}
		result.variables = append(result.variables, ref)
		fresh, e := m.MkFreshConst("x", sort)
		if e = errors.Join(e, m.SetRef(ref, fresh)); e != nil {
			return result, e
		}
		args = append(args, fresh)
	}
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	sub, err := rewriter.NewVarSubstitution(m, true)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return result, err
	}
	defer func() { err = errors.Join(err, sub.Close()) }()
	if err = sub.Substitute(body, args, tmp); err != nil {
		return result, err
	}
	shift, err := rewriter.NewInverseVarShifter(m)
	if err != nil {
		if shift != nil {
			err = errors.Join(err, shift.Close())
		}
		return result, err
	}
	defer func() { err = errors.Join(err, shift.Close()) }()
	h, err := m.RefGet(tmp)
	if err != nil {
		return result, err
	}
	if err = shift.InverseShift(h, n, result.body); err != nil {
		return result, err
	}
	return result, nil
}

func (s *exprQuantifierExtract) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	if s.body != (ast.ASTRef{}) {
		if err := s.manager.DelRef(&s.body); err != nil {
			return err
		}
	}
	for len(s.variables) > 0 {
		i := len(s.variables) - 1
		if err := s.manager.DelRef(&s.variables[i]); err != nil {
			return err
		}
		s.variables = s.variables[:i]
	}
	s.manager = nil
	return nil
}
