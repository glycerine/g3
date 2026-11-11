package smtcore

// Source: src/smt/asserted_formulas.cpp push_assertion().

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
)

// DisplayAssertedFormulas renders the asserted-formula manager view, including
// the first formula that has not yet been committed to the internalizer.
func (k *SMTKernel) DisplayAssertedFormulas(out io.Writer) error {
	if err := k.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	printer, err := ast.NewLegacySMTPrinter(k.manager)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "asserted formulas:\n"); err != nil {
		return err
	}
	for i, assertion := range k.assertions {
		if i == k.assertedHead {
			if _, err = io.WriteString(out, "[HEAD] ==>\n"); err != nil {
				return err
			}
		}
		if err = printer.DisplayAST(out, assertion.Formula, 0, nil); err != nil {
			return err
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	inconsistent := 0
	if k.assertedFormulasInconsistent() {
		inconsistent = 1
	}
	_, err = fmt.Fprintf(out, "inconsistent: %d\n", inconsistent)
	return err
}

func (k *SMTKernel) assertedFormulasInconsistent() bool {
	return k != nil && k.assertedInconsistent
}

// AssertionInconsistencyProof returns the borrowed proof attached to the first
// false asserted formula, matching asserted_formulas::get_inconsistency_proof.
func (k *SMTKernel) AssertionInconsistencyProof() (ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if !k.assertedFormulasInconsistent() {
		return ast.ASTHandle{}, nil
	}
	proofs, err := k.manager.ProofsEnabled()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !proofs {
		return ast.ASTHandle{}, nil
	}
	for _, assertion := range k.assertions {
		if k.manager.IsFalse(assertion.Formula) {
			return assertion.Proof, nil
		}
	}
	if k.assertedInconsistent {
		if err := k.assertedSourceAssertion(610, "UNREACHABLE", func() (bool, error) { return false, nil }); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return ast.ASTHandle{}, nil
}

func (k *SMTKernel) appendAssertion(formula, proof ast.ASTHandle) error {
	if err := k.manager.IncRef(formula); err != nil {
		return err
	}
	if !proof.IsNull() {
		if err := k.manager.IncRef(proof); err != nil {
			return errors.Join(err, k.manager.DecRef(formula))
		}
	}
	k.assertions = append(k.assertions, SMTKernelAssertion{Formula: formula, Proof: proof})
	return nil
}

func (k *SMTKernel) pushAssertion(formula, proof ast.ASTHandle) error {
	return k.pushAssertionInto(formula, proof, nil)
}

// A nil batch is the public assertion vector. Simplification uses a separate
// owning vector until the source cancellation check permits its swap.
func (k *SMTKernel) pushAssertionInto(formula, proof ast.ASTHandle, batch *macros.FormulaBatch) error {
	if k.assertedFormulasInconsistent() {
		return nil
	}
	if k.manager.IsTrue(formula) {
		return nil
	}
	op, basic, err := k.internalizer.basicOperation(formula)
	if err != nil {
		return err
	}
	if basic && op == ast.ASTOpAnd {
		count, err := k.manager.GetNumArgs(formula)
		if err != nil {
			return err
		}
		for index := uint32(0); index < count; index++ {
			child, err := k.manager.GetArg(formula, index)
			if err != nil {
				return err
			}
			childProof := ast.ASTHandle{}
			if !proof.IsNull() {
				childProof, err = k.manager.MkProofAndElim(proof, index)
				if err != nil {
					return err
				}
			}
			if err = k.pushAssertionInto(child, childProof, batch); err != nil {
				return fmt.Errorf("push conjunction child %d: %w", index, err)
			}
		}
		return nil
	}
	if basic && op == ast.ASTOpNot {
		child, err := k.manager.GetArg(formula, 0)
		if err != nil {
			return err
		}
		childOp, childBasic, err := k.internalizer.basicOperation(child)
		if err != nil {
			return err
		}
		if childBasic && childOp == ast.ASTOpOr {
			count, err := k.manager.GetNumArgs(child)
			if err != nil {
				return err
			}
			for index := uint32(0); index < count; index++ {
				argument, err := k.manager.GetArg(child, index)
				if err != nil {
					return err
				}
				childProof := ast.ASTHandle{}
				if !proof.IsNull() {
					childProof, err = k.manager.MkProofNotOrElim(proof, index)
					if err != nil {
						return err
					}
				}
				negated, err := k.manager.MkNotSimplified(argument)
				if err != nil {
					return err
				}
				if err = k.pushAssertionInto(negated, childProof, batch); err != nil {
					return fmt.Errorf("push negated disjunction child %d: %w", index, err)
				}
			}
			return nil
		}
	}
	if batch == nil {
		err = k.appendAssertion(formula, proof)
	} else {
		err = batch.Insert(macros.Formula{Expression: formula, Proof: proof})
	}
	if err == nil && k.manager.IsFalse(formula) {
		k.assertedInconsistent = true
	}
	return err
}

func (k *SMTKernel) preprocessAssertion(formula, proof ast.ASTHandle) (result, resultProof ast.ASTHandle, err error) {
	if !k.preprocess || k.preprocessor == nil {
		return formula, proof, nil
	}
	resultRef, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, k.manager.DelRef(&resultRef)) }()
	proofRef, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, k.manager.DelRef(&proofRef)) }()
	return k.preprocessAssertionWithMode(formula, proof, resultRef, proofRef, assertedProofAssertExpr)
}

// assertedProofMode is how a pass combines the input proof with m_rewriter's.
type assertedProofMode uint8

const (
	// assert_expr: pr = (e == r) ? in_pr : mk_modus_ponens(in_pr, pr).
	assertedProofAssertExpr assertedProofMode = iota
	// propagate_values, nnf_cnf: mk_modus_ponens(pr, pr1) always.
	assertedProofAlways
	// simplify_fmls: if (!pr) pr = mk_rewrite(f, r); mk_modus_ponens(pr0, pr).
	assertedProofSimplify
)

// preprocessAssertionMode is preprocessAssertion with a pass's proof rule.
func (k *SMTKernel) preprocessAssertionMode(formula, proof ast.ASTHandle, mode assertedProofMode) (result, resultProof ast.ASTHandle, err error) {
	if !k.preprocess || k.preprocessor == nil {
		return formula, proof, nil
	}
	resultRef, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, k.manager.DelRef(&resultRef)) }()
	proofRef, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, k.manager.DelRef(&proofRef)) }()
	return k.preprocessAssertionWithMode(formula, proof, resultRef, proofRef, mode)
}

// preprocessAssertionWith is m_rewriter(n, r1, pr1) with the caller's r1
// and pr1, which the rewrite overwrites (so their old values may die there).
func (k *SMTKernel) preprocessAssertionWith(formula, proof ast.ASTHandle, resultRef, proofRef ast.ASTRef) (result, resultProof ast.ASTHandle, err error) {
	return k.preprocessAssertionWithMode(formula, proof, resultRef, proofRef, assertedProofAlways)
}

func (k *SMTKernel) preprocessAssertionWithMode(formula, proof ast.ASTHandle, resultRef, proofRef ast.ASTRef, mode assertedProofMode) (result, resultProof ast.ASTHandle, err error) {
	if !k.preprocess || k.preprocessor == nil {
		return formula, proof, nil
	}
	proofs, err := k.manager.ProofsEnabled()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	// m_rewriter(e, r, pr) generates proofs when the rewriter was built
	// with proofs (m_proof_gen); m.proofs_enabled() only decides below
	// whether that proof is combined with the input proof.
	proofGen := k.preprocessor.ProofGen()
	if proofGen {
		err = k.preprocessor.RewriteWithProof(formula, resultRef, proofRef)
	} else {
		err = k.preprocessor.Rewrite(formula, resultRef)
	}
	if err != nil {
		// A failed rewrite can leave resumable frames. Assertion submission
		// does not expose Resume, so discard that traversal before retry or
		// teardown while restoring the borrowed assertion substitution.
		return ast.ASTHandle{}, ast.ASTHandle{}, errors.Join(err, k.flushAssertedCache())
	}
	result, err = k.manager.RefGet(resultRef)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	resultProof = proof
	if !proofs && proofGen {
		// The source's pr then keeps the rewriter's proof.
		if resultProof, err = k.manager.RefGet(proofRef); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
	}
	if proofs && (mode != assertedProofAssertExpr || result != formula) {
		rewriteProof, getErr := k.manager.RefGet(proofRef)
		if getErr != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, getErr
		}
		if mode == assertedProofSimplify && rewriteProof.IsNull() {
			if rewriteProof, err = k.manager.MkProofRewrite(formula, result); err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
		}
		resultProof, err = k.manager.MkProofModusPonens(proof, rewriteProof)
	}
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = k.manager.IncRef(result); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if !resultProof.IsNull() {
		if err = k.manager.IncRef(resultProof); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, errors.Join(err, k.manager.DecRef(result))
		}
	}
	return result, resultProof, nil
}
