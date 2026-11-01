package spacer

import (
	"errors"

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
