package spacer

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

// spacerModelEvaluatorConstructionFailure keeps a partially initialized
// evaluator reachable when its cleanup is refused. This is the concrete Go
// owner corresponding to model_evaluator_util's m_mev pointer during reset.
type spacerModelEvaluatorConstructionFailure struct {
	evaluator  *model.ModelEvaluator
	runtime    *memory.Runtime
	cause      error
	cleanupErr error
}

func (f *spacerModelEvaluatorConstructionFailure) Error() string {
	return errors.Join(f.cause, f.cleanupErr).Error()
}
func (f *spacerModelEvaluatorConstructionFailure) Unwrap() error {
	return errors.Join(f.cause, f.cleanupErr)
}
func (f *spacerModelEvaluatorConstructionFailure) Cleanup() error {
	if f.evaluator == nil {
		f.cleanupErr = nil
		return nil
	}
	f.cleanupErr = f.evaluator.Close()
	if f.cleanupErr == nil {
		f.evaluator = nil
	}
	return f.cleanupErr
}
func (f *spacerModelEvaluatorConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *spacerModelEvaluatorConstructionFailure) OriginalCause() error          { return f.cause }
func (f *spacerModelEvaluatorConstructionFailure) CleanupCause() error           { return f.cleanupErr }

func closeFailedSpacerModelEvaluator(evaluator *model.ModelEvaluator, config model.ModelEvaluatorConfig, cause error) (*model.ModelEvaluator, error) {
	if evaluator == nil {
		return nil, cause
	}
	if cleanup := evaluator.Close(); cleanup != nil {
		return nil, &spacerModelEvaluatorConstructionFailure{
			evaluator: evaluator, runtime: config.Memory, cause: cause, cleanupErr: cleanup,
		}
	}
	return nil, cause
}

// newSpacerModelEvaluator consumes partial construction owners before returning
// an error, so callers only own an evaluator after successful construction.
func newSpacerModelEvaluator(md *model.ModelCore, config model.ModelEvaluatorConfig) (*model.ModelEvaluator, error) {
	evaluator, err := model.NewModelEvaluator(md, config)
	if err != nil {
		return closeFailedSpacerModelEvaluator(evaluator, config, err)
	}
	return evaluator, nil
}

// Source: spacer_util.cpp model_evaluator_util::eval's typed exception catch.
func traceSpacerModelEvaluationFailure(manager *ast.ASTManager, config model.ModelEvaluatorConfig, failure error) error {
	tags := config.Trace
	if tags == nil {
		tags = manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("spacer_model_evaluator", "eval", "src/muz/spacer/spacer_util.cpp", 101,
		func() bool { return true }, func(out io.Writer) error { _, err := io.WriteString(out, failure.Error()+"\n"); return err })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// evaluateSpacerModelBoolean implements model_evaluator_util's Boolean probes:
// evaluator rewrite exceptions mean unknown; ownership/parameter errors remain
// errors. An interrupted evaluator still obeys the rewriter resume contract.
func evaluateSpacerModelBoolean(manager *ast.ASTManager, evaluator *model.ModelEvaluator, config model.ModelEvaluatorConfig, expression ast.ASTHandle) (bool, bool, error) {
	if err := evaluator.SetModelCompletion(false); err != nil {
		return false, false, err
	}
	value, err := evaluator.EvaluateValue(expression)
	if err != nil {
		if rewriter.IsRewriterException(err) {
			return false, false, traceSpacerModelEvaluationFailure(manager, config, err)
		}
		return false, false, err
	}
	h, err := manager.RefGet(value)
	if err != nil {
		return false, false, errors.Join(err, manager.DelRef(&value))
	}
	truth, falsehood := manager.IsTrue(h), manager.IsFalse(h)
	return truth, falsehood, manager.DelRef(&value)
}
