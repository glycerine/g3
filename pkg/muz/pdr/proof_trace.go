package pdr

// Source: model_search::get_proof_trace, context::get_proof, and
// context::mk_sat_answer in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *Context) completeTraceAnswer(root *ModelNode) (result ast.ASTRef, resultErr error) {
	constraints, err := c.unifiedTraceConstraints(root, true)
	if err != nil {
		return result, fmt.Errorf("pdr: unify complete proof trace: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, closeASTRefs(c.manager, constraints))
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	handles := make([]ast.ASTHandle, len(constraints))
	for index := range constraints {
		handles[index], err = c.manager.RefGet(constraints[index])
		if err != nil {
			return result, err
		}
	}
	formula, err := c.manager.MkAndArray(handles)
	if err != nil {
		return result, err
	}
	return c.manager.NewRef(formula)
}

// Proof reconstructs the latest reachable Horn derivation as a hyper-
// resolution proof. The asserted premises are the selected source rules; the
// conclusion is the complete bank-unified trace formula. The caller releases
// the returned reference through ASTManager.DelRef.
func (c *Context) Proof(queryDeclaration ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	if !c.generateProofTrace || c.lastResult != lbool.True || c.queryDecl != queryDeclaration || c.query == nil {
		return result, ast.ErrASTDomain
	}
	root, found, err := c.search.Root()
	if err != nil || !found {
		if err != nil {
			return result, err
		}
		return result, ast.ErrASTDomain
	}
	if err = updateTraceModels(root); err != nil {
		return result, err
	}
	restoreProofMode, err := c.enterPDRProofMode()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, restoreProofMode()) }()

	references := make([]ast.ASTRef, 0)
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(c.manager, references)) }()
	premises := make([]ast.ASTHandle, 0)
	seen := make(map[ast.ASTHandle]bool)
	work := []*ModelNode{root}
	for len(work) != 0 {
		last := len(work) - 1
		node := work[last]
		work = work[:last]
		if seen[node.state] {
			continue
		}
		seen[node.state] = true
		rule, x := c.selectedTraceRule(node)
		if x != nil {
			return result, fmt.Errorf("pdr: select proof-trace rule: %w", x)
		}
		formula, x := rule.rule.ToFormula()
		if x != nil {
			return result, x
		}
		references = append(references, formula)
		formulaHandle, x := c.manager.RefGet(formula)
		if x != nil {
			return result, x
		}
		proof, x := c.manager.MkProofAsserted(formulaHandle)
		if x != nil {
			return result, x
		}
		proofReference, x := c.manager.NewRef(proof)
		if x != nil {
			return result, x
		}
		references = append(references, proofReference)
		premises = append(premises, proof)
		children, x := node.Children()
		if x != nil {
			return result, x
		}
		work = append(work, children...)
	}
	if len(premises) == 0 {
		return result, ast.ErrASTDomain
	}
	conclusion, err := c.completeTraceAnswer(root)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&conclusion)) }()
	conclusionHandle, err := c.manager.RefGet(conclusion)
	if err != nil {
		return result, err
	}
	positions := make([]ast.ASTProofPosition, len(premises)-1)
	for index := range positions {
		positions[index] = ast.ASTProofPosition{First: uint32(index + 1), Second: 0}
	}
	substitutions := make([][]ast.ASTHandle, len(premises))
	proof, err := c.manager.MkProofHyperResolve(premises, conclusionHandle, positions, substitutions)
	if err != nil {
		return result, err
	}
	return c.manager.NewRef(proof)
}
