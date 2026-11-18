package qe

// Source: src/qe/qe.cpp, quant_elim_plugin's i_solver_context view of the
// search tree (get_num_vars, get_var, contains, elim_var, add_var) used by
// solve_vars.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// exprQETreeContext is the part of quant_elim_plugin a solver plugin reaches
// through i_solver_context: m_current, m_var2contains/m_new_vars, the
// rewriter and the NNF normalizer.
type exprQETreeContext struct {
	current   *exprQETree
	variables *exprQEBranchVariables
	config    Config
	rw        *rewriter.TheoryRewriter
	nnf       *QENNFNormalizer
}

func (t *exprQETreeContext) variable(index uint32) (ast.ASTHandle, error) {
	if index >= uint32(len(t.current.variables)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return t.current.manager.RefGet(t.current.variables[index])
}

func (t *exprQETreeContext) contains(index uint32) (*ast.ContainsApp, error) {
	v, err := t.variable(index)
	if err != nil {
		return nil, err
	}
	entry, ok := t.variables.index[v]
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return t.variables.entries[entry].contains, nil
}

// eliminate is quant_elim_plugin::elim_var: m_current->set_var(x, 1);
// m_current = m_current->add_child(fml); add_def; consume_vars(m_new_vars);
// normalize(*m_current).
func (t *exprQETreeContext) eliminate(index uint32, formula, definition ast.ASTHandle) error {
	if t == nil || t.current == nil || t.current.manager == nil || t.variables == nil || t.variables.manager != t.current.manager || t.rw == nil || t.nnf == nil {
		return ast.ErrASTHandle
	}
	node := t.current
	variable, err := t.variable(index)
	if err != nil {
		return err
	}
	// m_current->set_var(x, rational(1))
	q := node.manager.NumeralService().Manager
	one := unumeral.NewRational()
	err = q.SetInt32(&one, 1)
	if err == nil {
		err = node.selectVariable(variable, &one)
	}
	if err = errors.Join(err, q.Del(&one)); err != nil {
		return err
	}
	child, err := node.addUnit(t.config, formula)
	if err != nil {
		return err
	}
	t.current = child
	if err = child.addDefinition(variable, definition); err != nil {
		return err
	}
	if err = t.variables.consume(child); err != nil {
		return err
	}
	return normalizeExprQETree(child, t.rw, t.nnf)
}
