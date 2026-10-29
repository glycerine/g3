package fp

// Source: src/muz/transforms/dl_transforms.cpp and the fixedpoint backend
// query setup that applies the default Horn-rule transformation pipeline.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

// hornPreprocessResult owns the transformed rule set and keeps the borrowed
// declaration mappings produced with it together for the consuming backend.
// The mappings remain valid while the engine's rule manager and initialized
// backend context retain the transformed declarations.
type hornPreprocessResult struct {
	rules *base.RuleSet

	slicedPredicates       map[ast.ASTHandle]ast.ASTHandle
	slicedColumns          map[ast.ASTHandle][]bool
	quantifiedPredicates   map[ast.ASTHandle]ast.ASTHandle
	instantiatedPredicates map[ast.ASTHandle]ast.ASTHandle
	bitBlastedPredicates   map[ast.ASTHandle]ast.ASTHandle
	scaledPredicates       map[ast.ASTHandle]ast.ASTHandle
	magicPredicates        map[ast.ASTHandle]ast.ASTHandle
	karrInvariants         map[ast.ASTHandle]rel.KarrMatrix
	coneModelSteps         []transforms.ConeModelStep

	closed bool
}

func (r *hornPreprocessResult) Close() (result error) {
	if r == nil {
		return base.ErrRuleSetOwner
	}
	if r.closed {
		return nil
	}
	if r.rules != nil {
		result = errors.Join(result, r.rules.Close())
	}
	result = errors.Join(result, transforms.CloseConeModelSteps(r.coneModelSteps))
	r.rules = nil
	r.slicedPredicates = nil
	r.slicedColumns = nil
	r.quantifiedPredicates = nil
	r.instantiatedPredicates = nil
	r.bitBlastedPredicates = nil
	r.scaledPredicates = nil
	r.magicPredicates = nil
	r.karrInvariants = nil
	r.coneModelSteps = nil
	r.closed = true
	return result
}

func (r *hornPreprocessResult) takeConeModelSteps() []transforms.ConeModelStep {
	if r == nil || r.closed {
		return nil
	}
	steps := r.coneModelSteps
	r.coneModelSteps = nil
	return steps
}
