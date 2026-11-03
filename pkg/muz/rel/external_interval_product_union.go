package rel

// Source: src/muz/rel/dl_product_relation.cpp aligned_union_fn and the
// external/interval concrete union and widening operations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalUnionOwner struct {
	runtime *memory.Runtime
	nested  memory.ConstructionFailure
	fresh   *ExternalRelation
}

func (o *externalIntervalUnionOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.fresh != nil {
		if err := o.fresh.Close(); err != nil {
			return err
		}
		o.fresh = nil
	}
	return nil
}

type externalIntervalUnionCleanupFailure struct {
	owner             *externalIntervalUnionOwner
	original, cleanup error
	reported          error
	text              string
}

var _ memory.ConstructionFailure = (*externalIntervalUnionCleanupFailure)(nil)

func (f *externalIntervalUnionCleanupFailure) Error() string        { return f.text }
func (f *externalIntervalUnionCleanupFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalUnionCleanupFailure) OriginalCause() error { return f.original }
func (f *externalIntervalUnionCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalUnionCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalUnionCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalIntervalUnion(owner *externalIntervalUnionOwner, cause error) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalUnionCleanupFailure{
		owner: owner, original: original, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}

func (p *ExternalIntervalProduct) unionCompatible(other *ExternalIntervalProduct) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if p.plugin != other.plugin || len(p.sorts) != len(other.sorts) || len(p.kinds) != len(other.kinds) {
		return ErrTableRelation
	}
	for i := range p.sorts {
		if p.sorts[i] != other.sorts[i] {
			return ast.ErrASTDomain
		}
	}
	for i := range p.kinds {
		if p.kinds[i] != other.kinds[i] {
			return ErrTableRelation
		}
	}
	return nil
}

// Union performs the source aligned product update. External and interval
// plugins do not support cross-kind unions in both directions, so only the two
// corresponding component unions are applied. Local interval candidates are
// published after the external callback reports success.
func (p *ExternalIntervalProduct) Union(source, delta *ExternalIntervalProduct, widen bool) (resultErr error) {
	if err := p.unionCompatible(source); err != nil {
		return err
	}
	if delta != nil {
		if delta == p || delta == source {
			return ast.ErrASTDomain
		}
		if err := p.unionCompatible(delta); err != nil {
			return err
		}
	}
	targetCandidate, err := p.interval.Clone()
	if err != nil {
		return err
	}
	if delta == nil {
		if _, err = targetCandidate.Union(source.interval, widen); err != nil {
			return err
		}
		if err = p.external.Union(source.external, nil, widen); err != nil {
			return err
		}
		p.interval = targetCandidate
		return nil
	}

	// aligned_union_fn supplies a fresh empty component delta to the basic
	// update, then accumulates that fresh result into the caller's delta using
	// an ordinary union. It never passes the caller delta directly.
	freshInterval := NewIntervalEmpty(uint32(len(p.sorts)))
	if _, err = targetCandidate.UnionWithDelta(source.interval, freshInterval, widen); err != nil {
		return err
	}
	deltaCandidate, err := delta.interval.Clone()
	if err != nil {
		return err
	}
	if _, err = deltaCandidate.Union(freshInterval, false); err != nil {
		return err
	}
	owner := &externalIntervalUnionOwner{runtime: p.plugin.ast.NumeralService().RuntimeOwner()}
	defer func() { resultErr = finishExternalIntervalUnion(owner, resultErr) }()
	owner.fresh, err = p.plugin.NewEmpty(p.sorts)
	if err != nil {
		return err
	}
	if err = p.external.Union(source.external, owner.fresh, widen); err != nil {
		return err
	}
	// The target component update is complete even when later delta
	// accumulation fails, so publish its local counterpart at the same point.
	p.interval = targetCandidate
	if err = delta.external.Union(owner.fresh, nil, false); err != nil {
		return err
	}
	delta.interval = deltaCandidate
	return nil
}
