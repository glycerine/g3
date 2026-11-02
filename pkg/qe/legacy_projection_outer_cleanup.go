package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyOuterArrayCleanupFailure struct {
	child          *legacyArrayProjectionResult
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyOuterArrayCleanupFailure)(nil)

func (f *legacyOuterArrayCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyOuterArrayCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyOuterArrayCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyOuterArrayCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyOuterArrayCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyOuterArrayCleanupFailure) Cleanup() error {
	if f.child == nil {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.child.Close()
	if f.cleanup == nil {
		f.child = nil
	}
	return f.cleanup
}

func closeTransferredLegacyOuterArray(child *legacyArrayProjectionResult, cause error) error {
	if cleanup := child.Close(); cleanup != nil {
		reported := cleanup
		if cause == nil {
			cause = cleanup
		} else {
			reported = errors.Join(cause, cleanup)
		}
		return &legacyOuterArrayCleanupFailure{
			child: child, runtime: child.manager.NumeralService().RuntimeOwner(), cause: cause, cleanup: cleanup,
			reported: reported,
		}
	}
	return cause
}

type legacyOuterArithmeticCleanupFailure struct {
	child          *legacyFormulaProjection
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyOuterArithmeticCleanupFailure)(nil)

func (f *legacyOuterArithmeticCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyOuterArithmeticCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyOuterArithmeticCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyOuterArithmeticCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyOuterArithmeticCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyOuterArithmeticCleanupFailure) Cleanup() error {
	if f.child == nil {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.child.Close()
	if f.cleanup == nil {
		f.child = nil
	}
	return f.cleanup
}

func closeTransferredLegacyOuterArithmetic(child *legacyFormulaProjection, cause error) error {
	if cleanup := child.Close(); cleanup != nil {
		reported := cleanup
		if cause == nil {
			cause = cleanup
		} else {
			reported = errors.Join(cause, cleanup)
		}
		return &legacyOuterArithmeticCleanupFailure{
			child: child, runtime: child.manager.NumeralService().RuntimeOwner(), cause: cause, cleanup: cleanup,
			reported: reported,
		}
	}
	return cause
}
