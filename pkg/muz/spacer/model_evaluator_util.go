package spacer

// Source: src/muz/spacer/spacer_util.cpp model_evaluator_util. set_model
// allocates one model_evaluator whose rewriter cache lives until the util is
// reset or destroyed; context::expand_node declares one for a reachable
// expansion and passes it to mk_reach_fact and create_children.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type modelEvaluatorUtil struct {
	core      *model.ModelCore
	config    model.ModelEvaluatorConfig
	evaluator *model.ModelEvaluator
}

// evaluatedCore is the model the evaluator reads: the derived model's core
// when the core carries one.
func evaluatedCore(md *model.ModelCore) *model.ModelCore {
	if md != nil && md.DerivedModel() != nil {
		return md.DerivedModel().Core()
	}
	return md
}

func newModelEvaluatorUtil(md *model.ModelCore, config model.ModelEvaluatorConfig) (*modelEvaluatorUtil, error) {
	if md == nil {
		return nil, ast.ErrASTHandle
	}
	evaluator, err := newSpacerModelEvaluator(evaluatedCore(md), config)
	if err != nil {
		return nil, err
	}
	return &modelEvaluatorUtil{core: md, config: config, evaluator: evaluator}, nil
}

func (u *modelEvaluatorUtil) Close() error {
	if u == nil || u.evaluator == nil {
		return nil
	}
	err := u.evaluator.Close()
	u.evaluator = nil
	return err
}

var errModelEvaluatorUtil = errors.New("spacer: model evaluator util without a model")
