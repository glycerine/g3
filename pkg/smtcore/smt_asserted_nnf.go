package smtcore

// Source: src/smt/asserted_formulas.cpp asserted_formulas::nnf_cnf.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/normal_forms"
)

func (k *SMTKernel) normalizePendingAssertions() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess {
		return nil
	}
	apply := k.preprocessState.NNFCNF || (k.assertedHasQuantifiers &&
		k.internalizer.quantifierManager != nil && k.internalizer.quantifierManager.mbqi)
	if !apply {
		return nil
	}
	// The C++ caller supplies an empty local parameter set, while the NNF
	// constructor still reads the global nnf module's defaults/overrides.
	empty := k.paramContext.EmptyValue()
	proc, err := normal_forms.NewNNFFromParams(k.manager, k.definedNames, k.paramContext, k.paramGlobals, &empty)
	if proc != nil {
		defer func() { result = errors.Join(result, proc.Close()) }()
	}
	if err != nil {
		return err
	}
	batch, err := macros.NewFormulaBatch(k.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
	if err = k.traceAssertedBody("nnf_bug", "nnf_cnf", 359, func(out io.Writer) error {
		_, traceErr := fmt.Fprintf(out, "i: %d sz: %d\n", k.assertedHead, len(k.assertions))
		return traceErr
	}); err != nil {
		return err
	}
	// The function locals push_todo and push_todo_prs are reset at the start
	// of each row and die after swap_asserted_formulas, push_todo_prs first.
	var todo nnfPushTodo
	defer func() {
		result = errors.Join(result, k.releaseNNFPushTodo(&todo.proofs), k.releaseNNFPushTodo(&todo.rows))
	}()
	for _, assertion := range k.assertions[k.assertedHead:] {
		if err = k.traceAssertedExpression("nnf_bug", "nnf_cnf", 362, "processing:\n", assertion.Formula); err != nil {
			return err
		}
		if canceled, err := k.appendNormalizedAssertion(proc, batch, assertion, &todo); canceled || err != nil {
			return err
		}
	}
	return k.swapPendingFormulaBatch(batch)
}

// nnfPushTodo is nnf_cnf's push_todo and push_todo_prs.
type nnfPushTodo struct {
	rows, proofs []ast.ASTRef
}

// releaseNNFPushTodo is ref_vector::reset: the elements are released front
// to back.
func (k *SMTKernel) releaseNNFPushTodo(refs *[]ast.ASTRef) (result error) {
	for i := range *refs {
		result = errors.Join(result, k.manager.DelRef(&(*refs)[i]))
	}
	*refs = nil
	return result
}

func (k *SMTKernel) appendNormalizedAssertion(proc *normal_forms.NNF, batch *macros.FormulaBatch, assertion SMTKernelAssertion, todo *nnfPushTodo) (canceled bool, result error) {
	// The loop body's r1 and pr1 die at the end of the row, pr1 first.
	out, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&out)) }()
	proof, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&proof)) }()
	// push_todo.reset(); push_todo_prs.reset();
	if err = errors.Join(k.releaseNNFPushTodo(&todo.rows), k.releaseNNFPushTodo(&todo.proofs)); err != nil {
		return false, err
	}
	if err = proc.Transform(assertion.Formula, &todo.rows, &todo.proofs, out, proof); err != nil {
		return false, err
	}
	definitions, definitionProofs := todo.rows, todo.proofs
	proofs, err := k.manager.ProofsEnabled()
	if err != nil {
		return false, err
	}
	// pr = m.mk_modus_ponens(pr, pr1); push_todo.push_back(r1);
	// push_todo_prs.push_back(pr): the main row is held before the rows are
	// rewritten, since the rewrites reuse r1 and pr1.
	value, err := k.manager.RefGet(out)
	if err != nil {
		return false, err
	}
	main, err := k.manager.NewRef(value)
	if err != nil {
		return false, err
	}
	todo.rows = append(todo.rows, main)
	var p ast.ASTHandle
	if proofs {
		rewrite, err := k.manager.RefGet(proof)
		if err != nil {
			return false, err
		}
		p, err = k.manager.MkProofModusPonens(assertion.Proof, rewrite)
		if err != nil {
			return false, err
		}
		held, err := k.manager.NewRef(p)
		if err != nil {
			return false, err
		}
		todo.proofs = append(todo.proofs, held)
	}
	if canceled, err := k.manager.Canceled(); canceled || err != nil {
		return canceled, err
	}
	for i, definition := range definitions {
		value, err := k.manager.RefGet(definition)
		if err != nil {
			return false, err
		}
		var dp ast.ASTHandle
		if proofs {
			dp, err = k.manager.RefGet(definitionProofs[i])
			if err != nil {
				return false, err
			}
		}
		if canceled, err := k.appendNNFRewritten(batch, value, dp, out, proof); canceled || err != nil {
			return canceled, err
		}
	}
	return k.appendNNFRewritten(batch, value, p, out, proof)
}

// NNF checks cancellation both after normalization and after rewriting each
// introduced definition/main row, before pushing it into the local vector.
func (k *SMTKernel) appendNNFRewritten(batch *macros.FormulaBatch, formula, proof ast.ASTHandle, r1, pr1 ast.ASTRef) (canceled bool, result error) {
	value, p, err := k.preprocessAssertionWith(formula, proof, r1, pr1)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, k.manager.DecRef(value), k.manager.DecRef(p)) }()
	if canceled, err := k.manager.Canceled(); canceled || err != nil {
		return canceled, err
	}
	return false, k.pushAssertionInto(value, p, batch)
}
