package fp

// Source: src/muz/transforms/dl_transforms.cpp and the fixedpoint backend
// query setup that applies the default Horn-rule transformation pipeline.

import (
	"errors"
	"slices"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
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
	modelConversions       []hornModelConversion
	sliceProof             *transforms.SliceProofConverter

	closed  bool
	closing bool
}

func (r *hornPreprocessResult) Close() error {
	if r == nil {
		return base.ErrRuleSetOwner
	}
	if r.closed {
		return nil
	}
	r.closing = true
	if r.rules != nil {
		if err := r.rules.Close(); err != nil {
			return err
		}
		r.rules = nil
	}
	if r.sliceProof != nil {
		if err := r.sliceProof.Close(); err != nil {
			return err
		}
		r.sliceProof = nil
	}
	for len(r.modelConversions) > 0 {
		last := len(r.modelConversions) - 1
		if err := closeHornModelConversions(r.modelConversions[last:]); err != nil {
			return err
		}
		r.modelConversions[last] = hornModelConversion{}
		r.modelConversions = r.modelConversions[:last]
	}
	r.slicedPredicates = nil
	r.slicedColumns = nil
	r.quantifiedPredicates = nil
	r.instantiatedPredicates = nil
	r.bitBlastedPredicates = nil
	r.scaledPredicates = nil
	r.magicPredicates = nil
	r.modelConversions = nil
	r.sliceProof = nil
	r.closed = true
	return nil
}

func (r *hornPreprocessResult) takeModelConversions() []hornModelConversion {
	if r == nil || r.closed || r.closing {
		return nil
	}
	conversions := r.modelConversions
	r.modelConversions = nil
	return conversions
}

// hornInlineConverter is the rule inliner's horn_subsume_model_converter:
// the inlined clauses wait in m_delay_head/m_delay_body until the first
// conversion turns them (mk_horn) into m_funcs/m_bodies, which every later
// conversion evaluates again.
type hornInlineConverter struct {
	manager *ast.ASTManager
	delayed []transforms.InlineModelRule
	// normalized counts the delayed clauses already in m_funcs/m_bodies.
	normalized int
	funcs      []ast.ASTRef
	bodies     []ast.ASTRef
}

func newHornInlineConversion(manager *ast.ASTManager, rules []transforms.InlineModelRule) hornModelConversion {
	return hornModelConversion{kind: hornConvertInline, inline: &hornInlineConverter{manager: manager, delayed: rules}}
}

func (c *hornInlineConverter) empty() bool {
	return c == nil || len(c.delayed) == 0 && len(c.funcs) == 0
}

// close is ~horn_subsume_model_converter: m_delay_body, m_delay_head, then
// m_bodies and m_funcs (reverse declaration order), each front to back.
func (c *hornInlineConverter) close() error {
	result := transforms.CloseInlineModelRules(c.delayed)
	if result == nil {
		c.delayed = nil
	}
	for _, refs := range []*[]ast.ASTRef{&c.bodies, &c.funcs} {
		for i := range *refs {
			if (*refs)[i] != (ast.ASTRef{}) {
				result = errors.Join(result, c.manager.DelRef(&(*refs)[i]))
			}
		}
		if result == nil {
			*refs = nil
		}
	}
	return result
}

type hornModelConversionKind uint8

const (
	hornConvertSlice hornModelConversionKind = iota + 1
	hornConvertQuantifier
	hornConvertBitBlast
	hornConvertScale
	hornConvertCone
	hornConvertInline
)

// hornConversionKeys lists keys in order first, then any remaining keys of
// set in AST id order of their values (Go map order is random).
func hornConversionKeys(m *ast.ASTManager, order []ast.ASTHandle, keys []ast.ASTHandle, value func(ast.ASTHandle) ast.ASTHandle) []ast.ASTHandle {
	present := make(map[ast.ASTHandle]bool, len(keys))
	for _, key := range keys {
		present[key] = true
	}
	result := make([]ast.ASTHandle, 0, len(keys))
	for _, key := range order {
		if present[key] {
			result = append(result, key)
			delete(present, key)
		}
	}
	rest := make([]ast.ASTHandle, 0, len(present))
	for _, key := range keys {
		if present[key] {
			rest = append(rest, key)
		}
	}
	ids := make(map[ast.ASTHandle]uint32, len(rest))
	for _, key := range rest {
		ids[key], _ = m.GetASTID(value(key))
	}
	slices.SortFunc(rest, func(a, b ast.ASTHandle) int { return int(ids[a]) - int(ids[b]) })
	return append(result, rest...)
}

// freshPredicateKeys orders an old -> fresh map by order, then by the fresh
// declarations' AST ids (creation order).
func freshPredicateKeys(m *ast.ASTManager, order []ast.ASTHandle, fresh map[ast.ASTHandle]ast.ASTHandle) []ast.ASTHandle {
	keys := make([]ast.ASTHandle, 0, len(fresh))
	for key := range fresh {
		keys = append(keys, key)
	}
	return hornConversionKeys(m, order, keys, func(key ast.ASTHandle) ast.ASTHandle { return fresh[key] })
}

type hornModelConversion struct {
	kind       hornModelConversionKind
	predicates map[ast.ASTHandle]ast.ASTHandle
	columns    map[ast.ASTHandle][]bool
	// order lists the predicates keys in the source model
	// converter's iteration order; nil means AST id order.
	order  []ast.ASTHandle
	cone   transforms.ConeModelStep
	inline *hornInlineConverter
}

func closeHornModelConversions(conversions []hornModelConversion) (result error) {
	for i := len(conversions) - 1; i >= 0; i-- {
		if conversions[i].kind == hornConvertCone {
			result = errors.Join(result, transforms.CloseConeModelSteps([]transforms.ConeModelStep{conversions[i].cone}))
		}
		if conversions[i].kind == hornConvertInline && conversions[i].inline != nil {
			result = errors.Join(result, conversions[i].inline.close())
		}
	}
	return result
}
