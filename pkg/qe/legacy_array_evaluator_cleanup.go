package qe

// Source: src/muz/spacer/spacer_qe_project.cpp local model_evaluator owners.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyArrayEvaluatorCleanupFailure struct {
	evaluator *model.ModelEvaluator
	nested    memory.ConstructionFailure
	runtime   *memory.Runtime
	cause     error
	cleanup   error
}

var _ memory.ConstructionFailure = (*legacyArrayEvaluatorCleanupFailure)(nil)

func (f *legacyArrayEvaluatorCleanupFailure) Error() string {
	return errors.Join(f.cause, f.cleanup).Error()
}
func (f *legacyArrayEvaluatorCleanupFailure) Unwrap() error                 { return errors.Join(f.cause, f.cleanup) }
func (f *legacyArrayEvaluatorCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyArrayEvaluatorCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArrayEvaluatorCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArrayEvaluatorCleanupFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.evaluator != nil {
		f.cleanup = f.evaluator.Close()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.evaluator = nil
	}
	f.cleanup = nil
	return nil
}

func closeFailedLegacyArrayEvaluator(manager *ast.ASTManager, evaluator *model.ModelEvaluator, cause error) error {
	cleanup := evaluator.Close()
	if cleanup == nil {
		return cause
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	var nested memory.ConstructionFailure
	_ = errors.As(cause, &nested)
	return &legacyArrayEvaluatorCleanupFailure{
		evaluator: evaluator, nested: nested, runtime: manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup,
	}
}
