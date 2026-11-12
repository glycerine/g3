package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::add_constraint.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// exprQELiterals is quant_elim_plugin::m_literals: the literals of the last
// add_constraint stay referenced until the next one (or the plugin's end).
type exprQELiterals struct {
	refs []ast.ASTRef
}

// reset is expr_ref_vector::reset, releasing front to back.
func (l *exprQELiterals) reset(m *ast.ASTManager) (err error) {
	for i := range l.refs {
		err = errors.Join(err, m.DelRef(&l.refs[i]))
	}
	l.refs = l.refs[:0]
	return err
}

func (l *exprQELiterals) push(m *ast.ASTManager, h ast.ASTHandle) error {
	ref, err := m.NewRef(h)
	if err == nil {
		l.refs = append(l.refs, ref)
	}
	return err
}

// assertExprQETreeConstraint is quant_elim_plugin::add_constraint. A tree
// built outside a quant_elim_plugin keeps its literals only for the call.
func assertExprQETreeConstraint(current *exprQETree, solver *smtcore.SMTSolver, useCurrent bool, literals ...ast.ASTHandle) (err error) {
	if current == nil || current.manager == nil || solver == nil || len(literals) > 3 {
		return ast.ErrASTDomain
	}
	m := current.manager
	held := current.literals
	if held == nil {
		held = &exprQELiterals{}
		defer func() { err = errors.Join(err, held.reset(m)) }()
	}
	if err = held.reset(m); err != nil {
		return err
	}
	if !useCurrent {
		current = current.parent
	}
	for node := current; node != nil; node = node.parent {
		assignment, e := m.RefGet(node.assignment)
		if e != nil {
			return e
		}
		negated, e := m.MkNotSimplified(assignment)
		if e != nil {
			return e
		}
		if e = held.push(m, negated); e != nil {
			return e
		}
	}
	for _, lit := range literals {
		if !lit.IsNull() {
			if err = held.push(m, lit); err != nil {
				return err
			}
		}
	}
	args := make([]ast.ASTHandle, len(held.refs))
	for i, ref := range held.refs {
		if args[i], err = m.RefGet(ref); err != nil {
			return err
		}
	}
	clause, err := m.MkOrArgs(args)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(clause)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&ref)) }()
	return solver.AssertFormula(clause)
}
