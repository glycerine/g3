package smtcore

// Source: src/smt/asserted_formulas.cpp refine_inj_axiom_fn::simplify and
// simplify_fmls::operator().

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (k *SMTKernel) refinePendingInjectivityAxioms() (result error) {
	defer k.restoreAssertedConflictOnError(k.assertedInconsistent, &result)
	if !k.preprocess || !k.preprocessState.RefineInjectivityAxiom || !k.assertedHasQuantifiers {
		return nil
	}
	batch, err := macros.NewFormulaBatch(k.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, batch.Close()) }()
	out, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&out)) }()
	// refine_inj_axiom_fn::simplify leaves p null; simplify_fmls builds the
	// rewrite and modus ponens proofs.
	proof, err := k.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, k.manager.DelRef(&proof)) }()
	for _, assertion := range k.assertions[k.assertedHead:] {
		row := macros.Formula{Expression: assertion.Formula, Proof: assertion.Proof}
		kind, err := k.manager.GetKind(assertion.Formula)
		if err != nil {
			return err
		}
		if kind == ast.ASTQuantifier {
			matched, err := rewriter.SimplifyInjectivityAxiom(k.manager, assertion.Formula, out)
			if err != nil {
				return err
			}
			if matched {
				row.Expression, err = k.manager.RefGet(out)
				if err != nil {
					return err
				}
				if err = k.traceAssertedBody("inj_axiom", "simplify", 617, func(traceOut io.Writer) error {
					if _, traceErr := io.WriteString(traceOut, "simplifying...\n"); traceErr != nil {
						return traceErr
					}
					printer, traceErr := ast.NewSimpleASTPrinterContext(k.manager, traceOut, traceOut)
					if traceErr != nil {
						return traceErr
					}
					if traceErr = printer.DisplayExpr(traceOut, assertion.Formula, 0); traceErr != nil {
						return traceErr
					}
					if _, traceErr = io.WriteString(traceOut, "\n"); traceErr != nil {
						return traceErr
					}
					if traceErr = printer.DisplayExpr(traceOut, row.Expression, 0); traceErr != nil {
						return traceErr
					}
					_, traceErr = io.WriteString(traceOut, "\n")
					return traceErr
				}); err != nil {
					return err
				}
			}
		}
		if err := k.manager.SetRef(out, row.Expression); err != nil {
			return err
		}
		if err := k.appendSimplifiedAssertion(batch, assertion, out, proof); err != nil {
			return err
		}
		if canceled, err := k.manager.Canceled(); canceled || err != nil {
			return err
		}
	}
	return k.finishAssertedPass(batch, false)
}
