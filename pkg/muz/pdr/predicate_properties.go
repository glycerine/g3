package pdr

// Source: pred_transformer::inherit_properties in src/muz/pdr/pdr_context.cpp.

import "github.com/glycerine/g3/pkg/ast"

// FrameProperty is one borrowed retained lemma and its strongest assigned
// frame. InfinityLevel identifies the invariant frame.
type FrameProperty struct {
	Formula ast.ASTHandle
	Level   uint32
}

// Properties snapshots the property-to-level map m_prop2level in its
// obj_map iteration order, each lemma at its strongest frame. Stale entries
// in weaker frame vectors are omitted.
func (p *PredicateTransformer) Properties() ([]FrameProperty, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	entries := p.propertyOrder.Entries()
	result := make([]FrameProperty, 0, len(entries))
	for _, entry := range entries {
		result = append(result, FrameProperty{Formula: entry.Key, Level: p.propertyLevel[entry.Key]})
	}
	return result, nil
}

// InheritProperties installs the current effective frame lemmas from another
// transformer for the same predicate.
func (p *PredicateTransformer) InheritProperties(other *PredicateTransformer) error {
	if err := p.open(); err != nil {
		return err
	}
	if other == nil {
		return ErrPredicateTransformerOwner
	}
	if err := other.open(); err != nil {
		return err
	}
	if p.head != other.head || p.manager != other.manager || p.owner != other.owner {
		return ast.ErrASTDomain
	}
	properties, err := other.Properties()
	if err != nil {
		return err
	}
	for _, property := range properties {
		if err = verbosePP(p.manager, 2, "(pdr-inherit: ", property.Formula, ")\n"); err != nil {
			return err
		}
		if _, err = p.AddProperty(property.Formula, property.Level); err != nil {
			return err
		}
	}
	return nil
}
