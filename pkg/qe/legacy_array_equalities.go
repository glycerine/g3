package qe

// Source: src/muz/spacer/spacer_qe_project.cpp array_project utilities.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// legacyArrayProjection holds no th_rewriter: each source util builds its own
// (array_project_eqs_util::mk_result a local one, array_select_reducer its
// m_rw member) inside qe_project's scoped_no_proof, so its proof mode and
// cache lifetime are the util's.
type legacyArrayProjection struct {
	syntax *ArrayProjection
}

// legacyArrayProjectionConstructionFailure keeps the actual partially built
// legacy service reachable when constructor rollback is refused.
type legacyArrayProjectionConstructionFailure struct {
	owner          *legacyArrayProjection
	runtime        *memory.Runtime
	cause, cleanup error
}

var _ memory.ConstructionFailure = (*legacyArrayProjectionConstructionFailure)(nil)

func (f *legacyArrayProjectionConstructionFailure) Error() string { return f.Unwrap().Error() }
func (f *legacyArrayProjectionConstructionFailure) Unwrap() error {
	return errors.Join(f.cause, f.cleanup)
}
func (f *legacyArrayProjectionConstructionFailure) OriginalCause() error          { return f.cause }
func (f *legacyArrayProjectionConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArrayProjectionConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArrayProjectionConstructionFailure) Cleanup() error {
	if f.owner == nil {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.owner.Close()
	if f.cleanup == nil {
		f.owner = nil
	}
	return f.cleanup
}

func closeFailedLegacyArrayProjection(owner *legacyArrayProjection, cause error) error {
	if cleanup := owner.Close(); cleanup != nil {
		return &legacyArrayProjectionConstructionFailure{owner: owner, runtime: owner.syntax.manager.NumeralService().RuntimeOwner(), cause: cause, cleanup: cleanup}
	}
	return cause
}

func newLegacyArrayProjection(manager *ast.ASTManager, config Config) (*legacyArrayProjection, error) {
	if _, err := manager.RegisterArrayPlugin(); err != nil {
		return nil, err
	}
	// The source ast_manager always carries every declaration plugin; the
	// utils' th_rewriters and model evaluators are built later, on use.
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	syntax, err := NewArrayProjection(manager, config)
	if err != nil {
		return nil, err
	}
	return &legacyArrayProjection{syntax: syntax}, nil
}

func (p *legacyArrayProjection) Close() error {
	if p == nil || p.syntax == nil {
		return ast.ErrASTHandle
	}
	if err := p.syntax.Close(); err != nil {
		return err
	}
	p.syntax = nil
	return nil
}

func (p *legacyArrayProjection) arrayExpression(expression ast.ASTHandle) (bool, error) {
	sort, err := p.syntax.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	family, err := p.syntax.manager.GetFamilyID(sort)
	return family == p.syntax.family, err
}
