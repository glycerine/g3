package rel

// Source: src/muz/rel/dl_relation_manager.cpp relation-level plugin
// registry and operations: register_relation_plugin_impl,
// get_relation_plugin, try_get_appropriate_plugin, mk_empty_relation,
// mk_full_relation, and the relation_manager::mk_*_fn dispatch with its
// default functors (empty_signature_relation_join_fn,
// default_relation_join_project_fn,
// default_relation_filter_interpreted_and_project_fn,
// default_relation_select_equal_and_project_fn).
//
// The table_relation plugins (one per table plugin, registered first) are
// the existing finite-table selection (NewFiniteTableForPredicate /
// NewDefaultFiniteTable); relation plugins registered here follow them in
// m_relation_plugins.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// relationPluginCloser is implemented by plugins that own resources
// (relation_plugin destructors run when the manager is destroyed).
type relationPluginCloser interface {
	Close() error
}

// RegisterRelationPlugin is register_relation_plugin_impl. The plugin whose
// name is the configured default relation becomes the favourite.
func (m *RelationManager) RegisterRelationPlugin(plugin RelationPlugin) error {
	if err := m.open(); err != nil {
		return err
	}
	if plugin == nil || plugin.Manager() != m {
		return ErrTable
	}
	m.relationPlugins = append(m.relationPlugins, plugin)
	m.initRelationKinds()
	fid := m.claimRelationKind(plugin)
	if init, ok := plugin.(relationPluginInitializer); ok {
		init.initializeKind(fid)
	}
	if plugin.Name() == m.defaultRelation {
		m.favouriteRelation = plugin
	}
	return nil
}

// SetDefaultRelation records context::default_relation() for
// register_relation_plugin_impl's favourite selection. An already
// registered plugin of that name becomes the favourite.
func (m *RelationManager) SetDefaultRelation(name string) error {
	if err := m.open(); err != nil {
		return err
	}
	m.defaultRelation = name
	m.favouriteRelation = nil
	for _, plugin := range m.relationPlugins {
		if plugin.Name() == name {
			m.favouriteRelation = plugin
		}
	}
	return nil
}

// GetRelationPlugin is get_relation_plugin(symbol): nil when absent.
func (m *RelationManager) GetRelationPlugin(name string) RelationPlugin {
	if m == nil {
		return nil
	}
	for _, plugin := range m.relationPlugins {
		if plugin.Name() == name {
			return plugin
		}
	}
	return nil
}

// FavouriteRelationPlugin is m_favourite_relation_plugin when it is a
// relation (not table_relation) plugin.
func (m *RelationManager) FavouriteRelationPlugin() RelationPlugin {
	if m == nil {
		return nil
	}
	return m.favouriteRelation
}

// SetRequestedRelationKind records get_requested_predicate_kind for a
// relation plugin (rel_context::set_predicate_representation).
func (m *RelationManager) SetRequestedRelationKind(predicate ast.ASTHandle, plugin RelationPlugin) error {
	if err := m.open(); err != nil {
		return err
	}
	if m.requestedRelation == nil {
		m.requestedRelation = make(map[ast.ASTHandle]RelationPlugin)
	}
	m.requestedRelation[predicate] = plugin
	return nil
}

func (m *RelationManager) closeRelationPlugins() (result error) {
	for i := len(m.relationPlugins) - 1; i >= 0; i-- {
		if closer, ok := m.relationPlugins[i].(relationPluginCloser); ok {
			result = errors.Join(result, closer.Close())
		}
	}
	m.relationPlugins = nil
	m.favouriteRelation = nil
	m.requestedRelation = nil
	return result
}

// relationPluginFor is the relation-plugin part of
// mk_empty_relation(s, kind): the requested kind's plugin when it handles
// the signature, otherwise the favourite relation plugin when it does.
// It returns nil when the table_relation path (mk_empty_table_relation)
// is next.
func (m *RelationManager) relationPluginFor(predicate ast.ASTHandle, sorts []ast.ASTHandle) (RelationPlugin, error) {
	if !predicate.IsNull() {
		if plugin := m.requestedRelation[predicate]; plugin != nil {
			ok, err := plugin.CanHandleSignature(sorts)
			if err != nil || ok {
				return plugin, err
			}
		}
		// A requested table_relation plugin that handles the signature.
		if kind := m.requestedTable[predicate]; kind != "" && m.RequestedFiniteKindHandles(sorts, kind) {
			return nil, nil
		}
	}
	if p := m.favouriteRelation; p != nil {
		ok, err := p.CanHandleSignature(sorts)
		if err != nil || ok {
			return p, err
		}
	}
	return nil, nil
}

// signatureIsTable is relation_signature_to_table's success test: every
// sort is a bounded finite domain.
func (m *RelationManager) signatureIsTable(sorts []ast.ASTHandle) (Signature, bool) {
	domains := make([]uint64, len(sorts))
	for i, sort := range sorts {
		domain, err := m.sortDomain(sort)
		if err != nil {
			return Signature{}, false
		}
		domains[i] = domain
	}
	return Signature{domains: domains}, true
}

// MkEmptyRelation is relation_manager::mk_empty_relation(s, pred): the
// requested plugin, the favourite relation plugin, a table_relation of the
// appropriate table plugin, then the first registered relation plugin
// that handles the signature.
func (m *RelationManager) MkEmptyRelation(sorts []ast.ASTHandle, predicate ast.ASTHandle) (FiniteTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if r, ok, err := m.requestedKindRelation(predicate, sorts, false); err != nil || ok {
		return r, err
	}
	plugin, err := m.relationPluginFor(predicate, sorts)
	if err != nil {
		return nil, err
	}
	if plugin != nil {
		return plugin.MkEmpty(sorts)
	}
	if signature, ok := m.signatureIsTable(sorts); ok {
		if !predicate.IsNull() {
			return m.newRequestedFiniteTable(predicate, signature)
		}
		return m.NewDefaultFiniteTable(signature)
	}
	p, err := m.firstRelationPlugin(sorts)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p.MkEmpty(sorts)
	}
	// product_relation_plugin::mk_empty(s): the empty product relation;
	// later operations add inner relations to it.
	return m.productPlugin().MkEmpty(sorts)
}

// firstRelationPlugin is the m_relation_plugins loop past the
// table_relation plugins: the first relation plugin handling sorts.
func (m *RelationManager) firstRelationPlugin(sorts []ast.ASTHandle) (RelationPlugin, error) {
	for _, p := range m.relationPlugins {
		ok, err := p.CanHandleSignature(sorts)
		if err != nil {
			return nil, err
		}
		if ok {
			return p, nil
		}
	}
	return nil, nil
}

// MkFullRelation is relation_manager::mk_full_relation(s, pred): the
// requested plugin, else get_appropriate_plugin(s).mk_full (favourite
// relation plugin first). ok is false when the table_relation path (the
// existing total-table construction) applies.
func (m *RelationManager) MkFullRelation(sorts []ast.ASTHandle, predicate ast.ASTHandle) (RelationBase, bool, error) {
	if err := m.open(); err != nil {
		return nil, false, err
	}
	if r, ok, err := m.requestedKindRelation(predicate, sorts, true); err != nil || ok {
		if rb, isRelation := AsRelation(r); isRelation || err != nil {
			return rb, ok && err == nil, err
		}
		// A table_relation full relation: the table construction applies.
		return nil, false, r.Close()
	}
	plugin, err := m.relationPluginFor(predicate, sorts)
	if err != nil {
		return nil, false, err
	}
	if plugin == nil {
		// get_appropriate_plugin(s): the table_relation plugins come first
		// in m_relation_plugins, then the relation plugins.
		if _, ok := m.signatureIsTable(sorts); ok {
			return nil, false, nil
		}
		if plugin, err = m.firstRelationPlugin(sorts); err != nil {
			return nil, false, err
		}
		if plugin == nil {
			return nil, false, z3exception.NewDefaultException("no suitable plugin found for given relation signature")
		}
	}
	r, err := plugin.MkFull(predicate, sorts)
	return r, err == nil, err
}

// ErrRelationNoPlugin stands for the product_relation fallbacks of
// mk_empty_relation / mk_join_fn, which belong to the product relation
// plugin port.
var ErrRelationNoPlugin = errors.New("muz/rel: no relation plugin handles the signature")

// relationKindsText is "trying to perform unsupported <op> operation on
// relations of kinds <k1> and <k2>" from dl_instruction.cpp.
func unsupportedRelationOperation(op string, kinds ...string) error {
	text := "trying to perform unsupported " + op + " operation on relations of kinds "
	for i, kind := range kinds {
		if i > 0 {
			text += " and "
		}
		text += kind
	}
	return z3exception.NewDefaultException(text)
}

func relationPluginName(r FiniteTable) string {
	if rel, ok := AsRelation(r); ok {
		return rel.RelationPlugin().Name()
	}
	if name := finiteTablePlugin(r); name != "" {
		return "tr_" + name
	}
	return fmt.Sprintf("%T", r)
}

// mkEmptyLike is relation_plugin::mk_empty(const relation_base& original).
// Plugins with several kinds (product_relation, sieve_relation) create the
// relation of original's kind: mk_empty(original.get_signature(),
// original.get_kind()).
func mkEmptyLike(r RelationBase) (RelationBase, error) {
	if k, ok := r.RelationPlugin().(kindedRelationPlugin); ok {
		if m := r.RelationPlugin().Manager(); m != nil {
			return k.MkEmptyKind(r.RelationSignature(), m.RelationKindOf(r))
		}
	}
	return r.RelationPlugin().MkEmpty(r.RelationSignature())
}

// MkRelationJoinFn is relation_manager::mk_join_fn(t1, t2, ...,
// allow_product_relation).
func (m *RelationManager) MkRelationJoinFn(t1, t2 RelationBase, cols1, cols2 []int, allowProduct bool) (RelationJoinFn, error) {
	p1, p2 := t1.RelationPlugin(), t2.RelationPlugin()
	res, err := p1.MkJoinFn(t1, t2, cols1, cols2)
	if err != nil {
		return nil, err
	}
	if res == nil && p1 != p2 {
		if res, err = p2.MkJoinFn(t1, t2, cols1, cols2); err != nil {
			return nil, err
		}
	}
	if res == nil && (len(t1.RelationSignature()) == 0 || len(t2.RelationSignature()) == 0) {
		res = m.emptySignatureRelationJoinFn()
	}
	// finite_product_relation fallback: not ported yet.
	if res == nil && allowProduct {
		return m.productPlugin().MkJoinFn(t1, t2, cols1, cols2)
	}
	return res, nil
}

// emptySignatureRelationJoinFn is empty_signature_relation_join_fn.
func (m *RelationManager) emptySignatureRelationJoinFn() RelationJoinFn {
	return func(r1, r2 RelationBase) (RelationBase, error) {
		if len(r1.RelationSignature()) == 0 {
			empty, err := r1.Empty()
			if err != nil {
				return nil, err
			}
			if empty {
				return r2.RelationPlugin().MkEmpty(r2.RelationSignature())
			}
			return r2.CloneRelation()
		}
		empty, err := r2.Empty()
		if err != nil {
			return nil, err
		}
		if empty {
			return r1.RelationPlugin().MkEmpty(r1.RelationSignature())
		}
		return r1.CloneRelation()
	}
}

// MkRelationProjectFn is relation_manager::mk_project_fn.
func (m *RelationManager) MkRelationProjectFn(t RelationBase, removed []int) (RelationTransformerFn, error) {
	return t.RelationPlugin().MkProjectFn(t, removed)
}

// MkRelationFilterInterpretedAndProjectFn is
// relation_manager::mk_filter_interpreted_and_project_fn.
func (m *RelationManager) MkRelationFilterInterpretedAndProjectFn(t RelationBase, condition ast.ASTHandle, removed []int) (RelationTransformerFn, error) {
	res, err := t.RelationPlugin().MkFilterInterpretedAndProjectFn(t, condition, removed)
	if err != nil || res != nil {
		return res, err
	}
	filter, err := m.MkRelationFilterInterpretedFn(t, condition)
	if err != nil || filter == nil {
		return nil, err
	}
	removedCols := append([]int(nil), removed...)
	var project RelationTransformerFn
	// default_relation_filter_interpreted_and_project_fn
	return func(r RelationBase) (RelationBase, error) {
		t1, err := r.CloneRelation()
		if err != nil {
			return nil, err
		}
		defer t1.Close()
		if err = filter(t1); err != nil {
			return nil, err
		}
		if project == nil {
			project, err = m.MkRelationProjectFn(t1, removedCols)
			if err != nil {
				return nil, err
			}
			if project == nil {
				return nil, z3exception.NewDefaultException("projection does not exist")
			}
		}
		return project(t1)
	}, nil
}

// MkRelationJoinProjectFn is relation_manager::mk_join_project_fn.
func (m *RelationManager) MkRelationJoinProjectFn(t1, t2 RelationBase, cols1, cols2, removed []int, allowProduct bool) (RelationJoinFn, error) {
	res, err := t1.RelationPlugin().MkJoinProjectFn(t1, t2, cols1, cols2, removed)
	if err != nil {
		return nil, err
	}
	if res == nil && t1.RelationPlugin() != t2.RelationPlugin() {
		if res, err = t2.RelationPlugin().MkJoinProjectFn(t1, t2, cols1, cols2, removed); err != nil {
			return nil, err
		}
	}
	if res != nil {
		return res, nil
	}
	join, err := m.MkRelationJoinFn(t1, t2, cols1, cols2, allowProduct)
	if err != nil || join == nil {
		return nil, err
	}
	removedCols := append([]int(nil), removed...)
	var project RelationTransformerFn
	// default_relation_join_project_fn
	return func(r1, r2 RelationBase) (RelationBase, error) {
		aux, err := join(r1, r2)
		if err != nil {
			return nil, err
		}
		defer aux.Close()
		if project == nil {
			project, err = m.MkRelationProjectFn(aux, removedCols)
			if err != nil {
				return nil, err
			}
			if project == nil {
				return nil, z3exception.NewDefaultException("projection does not exist")
			}
		}
		return project(aux)
	}, nil
}

// MkRelationRenameFn is relation_manager::mk_rename_fn.
func (m *RelationManager) MkRelationRenameFn(t RelationBase, cycle []int) (RelationTransformerFn, error) {
	return t.RelationPlugin().MkRenameFn(t, cycle)
}

// MkRelationUnionFn is relation_manager::mk_union_fn.
func (m *RelationManager) MkRelationUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	res, err := target.RelationPlugin().MkUnionFn(target, source, delta)
	if err != nil {
		return nil, err
	}
	if res == nil && target.RelationPlugin() != source.RelationPlugin() {
		if res, err = source.RelationPlugin().MkUnionFn(target, source, delta); err != nil {
			return nil, err
		}
	}
	if res == nil && delta != nil && target.RelationPlugin() != delta.RelationPlugin() && source.RelationPlugin() != delta.RelationPlugin() {
		res, err = delta.RelationPlugin().MkUnionFn(target, source, delta)
	}
	return res, err
}

// MkRelationWidenFn is relation_manager::mk_widen_fn.
func (m *RelationManager) MkRelationWidenFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	res, err := target.RelationPlugin().MkWidenFn(target, source, delta)
	if err != nil {
		return nil, err
	}
	if res == nil && target.RelationPlugin() != source.RelationPlugin() {
		if res, err = source.RelationPlugin().MkWidenFn(target, source, delta); err != nil {
			return nil, err
		}
	}
	if res == nil && delta != nil && target.RelationPlugin() != delta.RelationPlugin() && source.RelationPlugin() != delta.RelationPlugin() {
		if res, err = delta.RelationPlugin().MkWidenFn(target, source, delta); err != nil {
			return nil, err
		}
	}
	if res == nil {
		return m.MkRelationUnionFn(target, source, delta)
	}
	return res, nil
}

// MkRelationFilterIdenticalFn is relation_manager::mk_filter_identical_fn.
func (m *RelationManager) MkRelationFilterIdenticalFn(t RelationBase, cols []int) (RelationMutatorFn, error) {
	return t.RelationPlugin().MkFilterIdenticalFn(t, cols)
}

// MkRelationFilterEqualFn is relation_manager::mk_filter_equal_fn.
func (m *RelationManager) MkRelationFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error) {
	return t.RelationPlugin().MkFilterEqualFn(t, value, col)
}

// MkRelationFilterInterpretedFn is relation_manager::mk_filter_interpreted_fn.
func (m *RelationManager) MkRelationFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error) {
	return t.RelationPlugin().MkFilterInterpretedFn(t, condition)
}

// MkRelationSelectEqualAndProjectFn is
// relation_manager::mk_select_equal_and_project_fn.
func (m *RelationManager) MkRelationSelectEqualAndProjectFn(t RelationBase, value ast.ASTHandle, col int) (RelationTransformerFn, error) {
	res, err := t.RelationPlugin().MkSelectEqualAndProjectFn(t, value, col)
	if err != nil || res != nil {
		return res, err
	}
	selector, err := m.MkRelationFilterEqualFn(t, value, col)
	if err != nil || selector == nil {
		return nil, err
	}
	projector, err := m.MkRelationProjectFn(t, []int{col})
	if err != nil || projector == nil {
		return nil, err
	}
	// default_relation_select_equal_and_project_fn
	return func(r RelationBase) (RelationBase, error) {
		aux, err := r.CloneRelation()
		if err != nil {
			return nil, err
		}
		defer aux.Close()
		if err = selector(aux); err != nil {
			return nil, err
		}
		return projector(aux)
	}, nil
}

// MkRelationFilterByNegationFn is relation_manager::mk_filter_by_negation_fn.
func (m *RelationManager) MkRelationFilterByNegationFn(t, neg RelationBase, tCols, negCols []int) (RelationIntersectionFilterFn, error) {
	res, err := t.RelationPlugin().MkFilterByNegationFn(t, neg, tCols, negCols)
	if err != nil {
		return nil, err
	}
	if res == nil && t.RelationPlugin() != neg.RelationPlugin() {
		res, err = neg.RelationPlugin().MkFilterByNegationFn(t, neg, tCols, negCols)
	}
	return res, err
}

// SetRequestedTableKind records rel_context::set_predicate_representation's
// requested table_relation plugin (tr_*) for mk_empty_relation's choice
// among relation plugins.
func (m *RelationManager) SetRequestedTableKind(predicate ast.ASTHandle, kind string) error {
	if err := m.open(); err != nil {
		return err
	}
	if m.requestedTable == nil {
		m.requestedTable = make(map[ast.ASTHandle]string)
	}
	m.requestedTable[predicate] = kind
	return nil
}

// RequestedFiniteKindHandles is can_handle_signature(s, kind) of the
// table_relation plugin of the requested table plugin kind (tr_sparse,
// tr_hashtable, tr_bitvector, tr_lazy_sparse).
func (m *RelationManager) RequestedFiniteKindHandles(sorts []ast.ASTHandle, kind string) bool {
	signature, ok := m.signatureIsTable(sorts)
	if !ok {
		return false
	}
	switch kind {
	case "tr_hashtable":
		return true
	case "tr_sparse":
		return signature.Len() != 0
	case "tr_bitvector":
		return BitvectorCanHandleSignature(signature, 0)
	case "tr_lazy_sparse":
		// lazy_table_plugin::can_handle_signature is its sparse plugin's.
		return signature.Len() != 0
	}
	return false
}

// SortDomain is the table domain of a relation sort, ok false when the
// sort is not a bounded finite domain.
func (m *RelationManager) SortDomain(sort ast.ASTHandle) (uint64, bool) {
	domain, err := m.sortDomain(sort)
	return domain, err == nil
}
