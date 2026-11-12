package smtcore

// Source: src/smt/asserted_formulas.h lift_ite, ng_lift_ite and
// elim_term_ite_fn; src/smt/asserted_formulas.cpp setup and simplify_fmls.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

const (
	smtLiftITENone uint32 = iota
	smtLiftITEConservative
	smtLiftITEFull
)

func (k *SMTKernel) liftPendingITEs(nonground bool) (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess {
		return nil
	}
	kind := k.preprocessState.LiftITE
	if nonground {
		kind = k.preprocessState.NonGroundLiftITE
		// asserted_formulas::setup suppresses redundant nonground passes.
		if k.preprocessState.LiftITE == smtLiftITEFull ||
			(k.preprocessState.LiftITE == smtLiftITEConservative && kind == smtLiftITEConservative) {
			return nil
		}
	}
	if kind == smtLiftITENone {
		return nil
	}
	var proc *rewriter.PushAppITE
	var err error
	if nonground {
		proc, err = rewriter.NewNGPushAppITE(k.manager, kind == smtLiftITEConservative)
	} else {
		proc, err = rewriter.NewPushAppITE(k.manager, kind == smtLiftITEConservative)
	}
	if proc != nil {
		defer func() { result = errors.Join(result, proc.Close()) }()
	}
	if err != nil {
		return err
	}
	out, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&out)) }()
	proof, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&proof)) }()
	batch, err := macros.NewFormulaBatch(k.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
	for _, assertion := range k.assertions[k.assertedHead:] {
		if err := proc.Rewrite(assertion.Formula, out, proof); err != nil {
			return err
		}
		if err := k.appendSimplifiedAssertion(batch, assertion, out, proof); err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	return k.finishAssertedPass(batch, true)
}

func (k *SMTKernel) eliminatePendingTermITEs() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || !k.preprocessState.EliminateTermITE ||
		k.preprocessState.LiftITE == smtLiftITEFull {
		return nil
	}
	proc, err := NewSMTTermITEEliminator(k.manager, k.definedNames)
	if proc != nil {
		defer func() { result = errors.Join(result, proc.Close()) }()
	}
	if err != nil {
		return err
	}
	out, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&out)) }()
	proof, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&proof)) }()
	batch, err := macros.NewFormulaBatch(k.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
	for _, assertion := range k.assertions[k.assertedHead:] {
		if err := proc.Rewrite(assertion.Formula, out, proof); err != nil {
			return err
		}
		if err := k.appendSimplifiedAssertion(batch, assertion, out, proof); err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	// Source post_op appends the generated definitions after all transformed
	// original assertions, then reduces the entire pending suffix.
	definitions, err := proc.NewDefinitions()
	if err != nil {
		return err
	}
	for _, definition := range definitions {
		formula, err := definition.Formula()
		if err != nil {
			return err
		}
		p, err := definition.Proof()
		if err != nil {
			return err
		}
		if err = batch.Insert(macros.Formula{Expression: formula, Proof: p}); err != nil {
			return err
		}
	}
	return k.finishAssertedPass(batch, true)
}

// appendSimplifiedAssertion follows simplify_fmls::operator(): unchanged
// formulas keep their original justification; changed formulas compose the
// simplifier proof, or a rewrite proof when the functor provides none.
func (k *SMTKernel) appendSimplifiedAssertion(batch *macros.FormulaBatch, assertion SMTKernelAssertion, out, proof ast.ASTRef) error {
	value, err := k.manager.RefGet(out)
	if err != nil {
		return err
	}
	row := macros.Formula{Expression: value, Proof: assertion.Proof}
	proofs, err := k.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		// if (!result_pr) result_pr = mk_rewrite(f, result);
		// result_pr = mk_modus_ponens(j.get_proof(), result_pr): built even
		// when the formula is unchanged (j is then kept and result_pr dies).
		rewrite, err := k.manager.RefGet(proof)
		if err != nil {
			return err
		}
		if rewrite.IsNull() {
			rewrite, err = k.manager.MkProofRewrite(assertion.Formula, value)
			if err != nil {
				return err
			}
			if err = k.manager.SetRef(proof, rewrite); err != nil {
				return err
			}
		}
		mp, err := k.manager.MkProofModusPonens(assertion.Proof, rewrite)
		if err != nil {
			return err
		}
		if err = k.manager.SetRef(proof, mp); err != nil {
			return err
		}
		if value != assertion.Formula {
			row.Proof = mp
		}
	} else if value != assertion.Formula {
		// Without proofs, push_assertion(result, result_pr) stores the
		// simplifier's own proof (a reflexivity when it generates proofs).
		if row.Proof, err = k.manager.RefGet(proof); err != nil {
			return err
		}
	}
	if err = k.appendTransformedAssertion(batch, assertion, row.Expression, row.Proof); err != nil {
		return err
	}
	// simplify_fmls::operator() declares result and result_pr per formula:
	// they die at the end of each iteration, result_pr first.
	return errors.Join(k.manager.SetRef(proof, ast.ASTHandle{}), k.manager.SetRef(out, ast.ASTHandle{}))
}
