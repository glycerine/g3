package rel

// Source: src/muz/rel/dl_mk_explanations.cpp mk_explanations: the relation
// manager side of the transformer (the constructor's plugin registration,
// assign_rel_level_kind, translate_rel_level_relation and the per-predicate
// relation transfer of transform_facts). The rule side is in muz/fp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// RegisterExplanationPlugins is the mk_explanations constructor's plugin
// registration: the explanation plugin of the requested level when the
// relation manager has none, and for fact-level explanations its
// finite_product_relation plugin.
func (m *RelationManager) RegisterExplanationPlugins(relationLevel bool) (*ExplanationRelationPlugin, error) {
	if p, ok := m.GetRelationPlugin(ExplanationRelationPluginName(relationLevel)).(*ExplanationRelationPlugin); ok {
		return p, nil
	}
	p, err := NewExplanationRelationPlugin(m, relationLevel)
	if err != nil {
		return nil, err
	}
	if err = m.RegisterRelationPlugin(p); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	if !relationLevel {
		if err = m.RegisterRelationPlugin(NewFiniteProductRelationPlugin(p, m)); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// AssignRelLevelKind is mk_explanations::assign_rel_level_kind: the
// explained predicate's kind is the product of a sieve over the original
// columns (of the original's requested kind) and a sieve over the
// explanation column (of the explanation plugin's kind).
func (m *RelationManager) AssignRelLevelKind(er *ExplanationRelationPlugin, eDecl, orig ast.ASTHandle) error {
	sorts, err := m.predicateSorts(eDecl)
	if err != nil {
		return err
	}
	sz := len(sorts)
	innerSieve := make([]bool, sz)
	explSieve := make([]bool, sz)
	for i := 0; i+1 < sz; i++ {
		innerSieve[i] = true
	}
	explSieve[sz-1] = true
	sieve := m.sievePlugin()
	innerKind := m.requestedKind(orig)
	innerSieveKind := sieve.store.getRelationKind(sorts, sieveSpec{innerCols: innerSieve, innerKind: innerKind})
	explKind := m.PluginKind(er)
	explSieveKind := sieve.store.getRelationKind(sorts, sieveSpec{innerCols: explSieve, innerKind: explKind})
	predKind := m.productPlugin().store.getRelationKind(sorts, []int{innerSieveKind, explSieveKind})
	return m.SetRequestedKind(eDecl, predKind)
}

// RelationView is the relation_base of a predicate's relation: the relation
// itself, or the (borrowed) table_relation of a table.
func (m *RelationManager) RelationView(table FiniteTable, sorts []ast.ASTHandle) RelationBase {
	if r, ok := AsRelation(table); ok {
		return r
	}
	return m.tableView(table, sorts, true)
}

// MkExplanationFactRelation is transform_facts' m_e_fact_relation:
// mk_empty_relation((rule sort), explanation kind) holding the fact
// (fact).
func (m *RelationManager) MkExplanationFactRelation(er *ExplanationRelationPlugin, eSort ast.ASTHandle) (_ RelationBase, resultErr error) {
	single, err := m.mkEmptyRelationKind([]ast.ASTHandle{eSort}, m.PluginKind(er))
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, single.Close())
		}
	}()
	name, err := m.manager.Symbols.Intern("fact")
	if err != nil {
		return nil, err
	}
	fact, err := m.manager.MkDatalogRule(name, nil)
	if err != nil {
		return nil, err
	}
	adder, ok := single.(RelationAddFactAST)
	if !ok {
		return nil, ErrRelationNotTable
	}
	if err = adder.AddFactAST([]ast.ASTHandle{fact}); err != nil {
		return nil, err
	}
	return single, nil
}

// ExplainRelation is transform_facts' transfer of a predicate's relation
// into its explained predicate's relation: translate_rel_level_relation
// for relation-level explanations, otherwise the product with
// m_e_fact_relation united into the explained relation.
func (m *RelationManager) ExplainRelation(er *ExplanationRelationPlugin, origRel, eRel, eFact RelationBase) (resultErr error) {
	if er.relationLevel {
		return m.translateRelLevelRelation(er, origRel, eRel, eFact)
	}
	product, err := m.MkRelationJoinFn(origRel, eFact, nil, nil, true)
	if err != nil {
		return err
	}
	if product == nil {
		// The fixed fork's throw (FORK FIX, see FORK_BUGFIXES.md); 4.7.1
		// called the null functor.
		return unsupportedRelationOperation("join", origRel.RelationPlugin().Name(), eFact.RelationPlugin().Name())
	}
	aux, err := product(origRel, eFact)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, aux.Close()) }()
	union, err := m.MkRelationUnionFn(eRel, aux, nil)
	if err != nil {
		return err
	}
	if union == nil {
		// The fixed fork's throw (FORK FIX, see FORK_BUGFIXES.md).
		return unsupportedRelationOperation("union", eRel.RelationPlugin().Name(), aux.RelationPlugin().Name())
	}
	return union(eRel, aux, nil)
}

// translateRelLevelRelation is translate_rel_level_relation.
func (m *RelationManager) translateRelLevelRelation(er *ExplanationRelationPlugin, orig, eRel, eFact RelationBase) error {
	prod, ok := eRel.(*productRelation)
	if !ok || len(prod.relations) != 2 {
		return ErrInstruction
	}
	s0, ok0 := prod.relations[0].(*sieveRelation)
	s1, ok1 := prod.relations[1].(*sieveRelation)
	if !ok0 || !ok1 {
		return ErrInstruction
	}
	if s0.inner.RelationPlugin() == RelationPlugin(er) {
		s0, s1 = s1, s0
	}
	newOrig := s0.inner
	explRel := s1.inner
	origUnion, err := m.MkRelationUnionFn(newOrig, orig, nil)
	if err != nil {
		return err
	}
	if origUnion == nil {
		// The context's relation, of the context's plugin instance (see
		// udocRelationForPlugin).
		copied, err := relationForPluginInstance(newOrig, orig)
		if err != nil {
			return err
		}
		if copied != nil {
			defer copied.Close()
			orig = copied
			if origUnion, err = m.MkRelationUnionFn(newOrig, orig, nil); err != nil {
				return err
			}
		}
	}
	if origUnion == nil {
		return unsupportedRelationOperation("union", newOrig.RelationPlugin().Name(), orig.RelationPlugin().Name())
	}
	if err = origUnion(newOrig, orig, nil); err != nil {
		return err
	}
	explUnion, err := m.MkRelationUnionFn(explRel, eFact, nil)
	if err != nil {
		return err
	}
	if explUnion == nil {
		return unsupportedRelationOperation("union", explRel.RelationPlugin().Name(), eFact.RelationPlugin().Name())
	}
	return explUnion(explRel, eFact, nil)
}

// RecreatePlaceholderRelation is get_relation for a predicate whose
// placeholder was made before its requested kind was set
// (set_predicate_kind precedes the source's relation creation): the
// placeholder is replaced by mk_empty_relation(sig, pred).
func (m *RelationManager) RecreatePlaceholderRelation(predicate ast.ASTHandle) (FiniteTable, error) {
	table, found, err := m.TryGetFiniteRelation(predicate)
	if err != nil || !found || !m.placeholders[predicate] {
		return table, err
	}
	// Still a placeholder: a check table's mk_empty line comes when
	// get_relation creates the relation (MaterializeFiniteRelation).
	m.reservingPlaceholder = true
	relation, ok, err := m.relationForPredicate(predicate)
	m.reservingPlaceholder = false
	if err != nil || !ok {
		return table, err
	}
	if err = m.ReplaceFiniteRelation(predicate, relation); err != nil {
		return table, errors.Join(err, relation.Close())
	}
	return relation, nil
}

// copyTBVAcross copies a ternary bit vector of another manager with the
// same number of bits into dst.
func copyTBVAcross(dst *TBVManager, src *TBVManager, v *TBV) (*TBV, error) {
	if dst.NumTBits() != src.NumTBits() {
		return nil, ErrTBV
	}
	res, err := dst.Allocate()
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < src.NumTBits(); i++ {
		b, err := src.Get(v, i)
		if err == nil {
			err = dst.Set(res, i, b)
		}
		if err != nil {
			return nil, errors.Join(err, dst.Deallocate(res))
		}
	}
	return res, nil
}

// relationForPluginInstance copies src, a relation of another instance of
// target's plugin, into a relation of target's plugin instance: a doc
// relation (udocRelationForPlugin) or a check_relation over one (its
// m_relation copied, m_fml its formula). nil when src is not such a
// relation.
func relationForPluginInstance(target, src RelationBase) (RelationBase, error) {
	switch t := target.(type) {
	case *UDocPluginRelation:
		s, ok := src.(*UDocPluginRelation)
		if !ok || s.plugin == t.plugin {
			return nil, nil
		}
		copied, err := udocRelationForPlugin(t.plugin, s)
		if err != nil {
			return nil, err
		}
		return copied, nil
	case *CheckRelation:
		s, ok := src.(*CheckRelation)
		if !ok || s.plugin == t.plugin {
			return nil, nil
		}
		inner, err := relationForPluginInstance(t.relation, s.relation)
		if err != nil || inner == nil {
			return nil, err
		}
		copied, err := t.plugin.newCheckRelation(s.sorts, inner)
		if err != nil {
			return nil, err
		}
		return copied, nil
	}
	return nil, nil
}

// udocRelationForPlugin copies a doc relation of another udoc plugin
// instance into p. g3 runs an expression query on a clone of the context's
// relation manager (z3 has the one datalog::context): the clone's relations
// of the context's facts belong to the context's plugin instance, the
// relations mk_explanations creates to the clone's.
func udocRelationForPlugin(p *UDocPlugin, src *UDocPluginRelation) (_ *UDocPluginRelation, resultErr error) {
	res, err := p.newRelation(src.sorts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, res.Close())
		}
	}()
	for _, d := range src.elems.values {
		pos, err := copyTBVAcross(res.dm.vectors, src.dm.vectors, d.positive)
		if err != nil {
			return nil, err
		}
		doc, err := res.dm.adopt(pos)
		if err != nil {
			return nil, err
		}
		for _, n := range d.negative.values {
			c, err := copyTBVAcross(res.dm.vectors, src.dm.vectors, n)
			if err != nil {
				return nil, errors.Join(err, res.dm.Deallocate(doc))
			}
			doc.negative.values = append(doc.negative.values, c)
		}
		res.elems.values = append(res.elems.values, doc)
	}
	return res, nil
}
