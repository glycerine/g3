package qe

// Source: src/qe/nlqsat.cpp:742-778 (nlqsat::mk_model).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// nlqIsUninterpretedConstant is is_uninterp_const.
func nlqIsUninterpretedConstant(m *ast.ASTManager, e ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(e)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	count, err := m.GetNumArgs(e)
	if err != nil || count != 0 {
		return false, err
	}
	declaration, err := m.GetDecl(e)
	if err != nil {
		return false, err
	}
	family, err := m.GetFamilyID(declaration)
	return family == ast.NullFamilyID, err
}

// mkModel walks m_t2x and then m_a2b in their obj_map order and registers the
// free uninterpreted constants (m_aux_vars is never populated): reals take
// m_rmodel0's value, Booleans m_bmodel0's unless it is undefined, and the
// is_true variable is skipped. ok is false when an irrational value was
// wrapped in to_int.
func (s *nlqSearch) mkModel(m *ast.ASTManager, compiler *nlsattactic.GoalCompiler, free map[ast.ASTHandle]bool, isTrue sat.Literal) (result *model.Model, ok bool, err error) {
	if err = s.open(); err != nil {
		return nil, false, err
	}
	result, err = model.NewModel(m)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	ok = true
	terms, err := compiler.TermMapping().Entries()
	if err != nil {
		return nil, false, err
	}
	for _, entry := range terms {
		constant, cause := nlqIsUninterpretedConstant(m, entry.Expr)
		if cause != nil {
			return nil, false, cause
		}
		if !constant || !free[entry.Expr] {
			continue
		}
		value, cause := s.state.answerReal.Value(entry.Variable)
		if cause != nil {
			return nil, false, cause
		}
		sort, cause := m.GetSort(entry.Expr)
		if cause != nil {
			return nil, false, cause
		}
		kind, cause := m.GetDeclKind(sort)
		if cause != nil {
			return nil, false, cause
		}
		numeral, integral, cause := compiler.AlgebraicNumeral(value, kind == 1)
		if cause != nil {
			return nil, false, cause
		}
		if !integral {
			ok = false
		}
		declaration, cause := m.GetDecl(entry.Expr)
		if cause != nil {
			return nil, false, cause
		}
		if cause = result.Core().RegisterConstant(declaration, numeral); cause != nil {
			return nil, false, cause
		}
	}
	atoms, err := compiler.AtomMapping().Entries()
	if err != nil {
		return nil, false, err
	}
	for _, entry := range atoms {
		constant, cause := nlqIsUninterpretedConstant(m, entry.Expr)
		if cause != nil {
			return nil, false, cause
		}
		if !constant || sat.BoolVar(entry.Variable) == isTrue.Variable() || !free[entry.Expr] {
			continue
		}
		// m_bmodel0.get(b, l_undef)
		value := lbool.Undef
		if uint64(entry.Variable) < uint64(len(s.state.answerBoolean)) {
			value = s.state.answerBoolean[entry.Variable]
		}
		if value == lbool.Undef {
			continue
		}
		var interpretation ast.ASTHandle
		if value == lbool.True {
			interpretation, cause = m.MkTrue()
		} else {
			interpretation, cause = m.MkFalse()
		}
		if cause != nil {
			return nil, false, cause
		}
		declaration, cause := m.GetDecl(entry.Expr)
		if cause != nil {
			return nil, false, cause
		}
		if cause = result.Core().RegisterConstant(declaration, interpretation); cause != nil {
			return nil, false, cause
		}
	}
	return result, ok, nil
}
