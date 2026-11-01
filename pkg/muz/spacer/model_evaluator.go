package spacer

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

// newSpacerModelEvaluator consumes partial construction owners before returning
// an error, so callers only own an evaluator after successful construction.
func newSpacerModelEvaluator(md *model.ModelCore, config model.ModelEvaluatorConfig) (*model.ModelEvaluator, error) {
	evaluator, err := model.NewModelEvaluator(md, config)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return nil, err
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
