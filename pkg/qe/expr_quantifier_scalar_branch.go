package qe

// Source: src/qe/qe.cpp update_current, Boolean and bit-vector branch paths.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// stepExprQEScalarBranch first records an assignment, then substitutes its
// concrete Boolean or bit-vector value on the next visit.
func stepExprQEScalarBranch(parent *exprQETree, variables *exprQEBranchVariables, plugin SolverPlugin, branch *big.Rat, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer, withDefinitions bool) (child *exprQETree, status exprQETreeStatus, err error) {
	if parent == nil || parent.manager == nil || variables == nil || variables.manager != parent.manager || plugin == nil || branch == nil || rw == nil || nnf == nil || parent.selected == (ast.ASTRef{}) {
		return nil, status, ast.ErrASTHandle
	}
	switch plugin.(type) {
	case *BoolPlugin, *BitVectorPlugin:
	default:
		return nil, status, ast.ErrASTDomain
	}
	m := parent.manager
	variable, err := m.RefGet(parent.selected)
	if err != nil {
		return nil, status, err
	}
	entry, ok := variables.index[variable]
	if !ok {
		return nil, status, ast.ErrASTDomain
	}
	formula, err := m.RefGet(parent.formula)
	if err != nil {
		return nil, status, err
	}
	n, err := m.NumeralService().NewValue()
	if err != nil {
		return nil, status, err
	}
	defer func() { err = errors.Join(err, m.NumeralService().Del(&n)) }()
	if err = m.NumeralService().SetString(&n, branch.RatString()); err != nil {
		return nil, status, err
	}
	index, exists := parent.branchIndex[branch.RatString()]
	if !exists {
		assignment, e := exprQEBranchEquality(m, variable, branch)
		if e != nil {
			return nil, status, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&assignment)) }()
		h, e := m.RefGet(assignment)
		if e != nil {
			return nil, status, e
		}
		child, e = parent.addBranch(config, branch, h)
		if e != nil {
			return nil, status, e
		}
		switch p := plugin.(type) {
		case *BoolPlugin:
			e = p.Assign(variables.entries[entry].contains, formula, &n)
		case *BitVectorPlugin:
			e = p.Assign(variables.entries[entry].contains, formula, &n)
		}
		if e != nil {
			return nil, status, e
		}
		if e = variables.consume(child); e != nil {
			return nil, status, e
		}
		return child, exprQENeedPropagation, nil
	}
	child = parent.children[index]
	if child.formula != (ast.ASTRef{}) {
		return child, exprQEChooseVariable, nil
	}
	tmp, err := m.NewRef(formula)
	if err != nil {
		return nil, status, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	var definition ast.ASTRef
	var definitionArg *ast.ASTRef
	if withDefinitions {
		definition, err = m.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, status, err
		}
		defer func() { err = errors.Join(err, m.DelRef(&definition)) }()
		definitionArg = &definition
	}
	switch p := plugin.(type) {
	case *BoolPlugin:
		err = p.Substitute(variables.entries[entry].contains, &n, tmp, definitionArg)
	case *BitVectorPlugin:
		err = p.Substitute(variables.entries[entry].contains, &n, tmp, definitionArg)
	}
	if err != nil {
		return nil, status, err
	}
	if err = variables.consume(child); err != nil {
		return nil, status, err
	}
	h, err := m.RefGet(tmp)
	if err != nil {
		return nil, status, err
	}
	if err = child.initialize(h); err != nil {
		return nil, status, err
	}
	if withDefinitions {
		d, e := m.RefGet(definition)
		if e != nil {
			return nil, status, e
		}
		if e = child.addDefinition(variable, d); e != nil {
			return nil, status, e
		}
	}
	if err = normalizeExprQETree(child, rw, nnf); err != nil {
		return nil, status, err
	}
	return child, exprQEChooseVariable, nil
}
