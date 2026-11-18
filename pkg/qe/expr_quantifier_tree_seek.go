package qe

// Source: src/qe/qe.cpp, update_current with apply=false.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type exprQETreeStatus uint8

const (
	exprQEChooseVariable exprQETreeStatus = iota
	exprQENeedPropagation
)

// seekExprQETree writes the branch number into branch (a rational of the
// manager's NumeralService().Manager owned by the caller).
func seekExprQETree(root *exprQETree, variables *exprQEBranchVariables, evaluator *model.ModelEvaluator, branch *unumeral.Rational, config Config) (current *exprQETree, status exprQETreeStatus, err error) {
	if root == nil || root.manager == nil || root.parent != nil || variables == nil || variables.manager != root.manager || evaluator == nil {
		return nil, status, ast.ErrASTHandle
	}
	m := root.manager
	q := m.NumeralService().Manager
	current = root
	for {
		if current.isUnit() {
			current = current.children[0]
			continue
		}
		if current.selected == (ast.ASTRef{}) {
			return current, exprQEChooseVariable, nil
		}
		variable, e := m.RefGet(current.selected)
		if e != nil {
			return nil, status, e
		}
		proxy, e := variables.branch(variable)
		if e != nil {
			return nil, status, e
		}
		found, e := exprQEEvalBranch(m, evaluator, proxy, branch)
		if e != nil {
			return nil, status, e
		}
		outside := !found
		if found {
			c, e := q.Compare(branch, &current.branches)
			if e != nil {
				return nil, status, e
			}
			outside = c >= 0
		}
		if outside {
			if e = qeTrace(m, config, "update_current", "src/qe/qe.cpp", 1694, "evaluation failed: setting branch to 0\n"); e != nil {
				return nil, status, e
			}
			if e = q.SetInt32(branch, 0); e != nil {
				return nil, status, e
			}
		}
		key, e := q.String(branch)
		if e != nil {
			return nil, status, e
		}
		index, ok := current.branchIndex[key]
		if !ok {
			return current, exprQENeedPropagation, nil
		}
		current = current.children[index]
		if current.formula == (ast.ASTRef{}) {
			return current, exprQEChooseVariable, nil
		}
	}
}
