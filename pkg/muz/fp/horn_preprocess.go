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
	modelConversions       []hornModelConversion
	sliceProof             *transforms.SliceProofConverter

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
	if r.sliceProof != nil {
		result = errors.Join(result, r.sliceProof.Close())
	}
	result = errors.Join(result, closeHornModelConversions(r.modelConversions))
	r.rules = nil
	r.slicedPredicates = nil
	r.slicedColumns = nil
	r.quantifiedPredicates = nil
	r.instantiatedPredicates = nil
	r.bitBlastedPredicates = nil
	r.scaledPredicates = nil
	r.magicPredicates = nil
	r.karrInvariants = nil
	r.modelConversions = nil
	r.sliceProof = nil
	r.closed = true
	return result
}

func (r *hornPreprocessResult) takeModelConversions() []hornModelConversion {
	if r == nil || r.closed {
		return nil
	}
	conversions := r.modelConversions
	r.modelConversions = nil
	return conversions
}

type hornModelConversionKind uint8

const (
	hornConvertSlice hornModelConversionKind = iota + 1
	hornConvertQuantifier
	hornConvertBitBlast
	hornConvertScale
	hornConvertKarr
	hornConvertCone
	hornConvertInline
)

type hornModelConversion struct {
	kind       hornModelConversionKind
	predicates map[ast.ASTHandle]ast.ASTHandle
	columns    map[ast.ASTHandle][]bool
	karr       map[ast.ASTHandle]rel.KarrMatrix
	cone       transforms.ConeModelStep
	inline     []transforms.InlineModelRule
}

func closeHornModelConversions(conversions []hornModelConversion) (result error) {
	for i := len(conversions) - 1; i >= 0; i-- {
		if conversions[i].kind == hornConvertCone {
			result = errors.Join(result, transforms.CloseConeModelSteps([]transforms.ConeModelStep{conversions[i].cone}))
		}
		if conversions[i].kind == hornConvertInline {
			result = errors.Join(result, transforms.CloseInlineModelRules(conversions[i].inline))
		}
	}
	return result
}
