package rel

import "github.com/glycerine/g3/pkg/ast"

// SetCheckedFiniteFactory borrows a factory for compiler temporaries created
// during one checked query scope. Nil restores ordinary favourite selection.
func (m *RelationManager) SetCheckedFiniteFactory(factory *FiniteCheckedFactory) error {
	if err := m.open(); err != nil {
		return err
	}
	m.checkedFinite = factory
	return nil
}

// checkedFactory is the check_table plugin of table creation: the checked
// scope's, else the default check_table favourite.
func (m *RelationManager) checkedFactory() *FiniteCheckedFactory {
	if m.checkedFinite != nil {
		return m.checkedFinite
	}
	return m.defaultChecked
}

// SetDefaultCheckedFactory makes factory (nil for none) the favourite
// check_table plugin of datalog.default_table_checked. register_plugin
// makes a second check_table_plugin of the same tables for the favourite
// table_relation plugin, whose table_relation plugin becomes
// m_favourite_relation_plugin; relationFavourite is false when
// rel_context::updt_params makes check_relation the favourite instead.
func (m *RelationManager) SetDefaultCheckedFactory(factory *FiniteCheckedFactory, relationFavourite bool) error {
	if err := m.open(); err != nil {
		return err
	}
	if old := m.defaultChecked; factory != nil && old != nil && old.primaryKind == factory.primaryKind && old.checkerKind == factory.checkerKind && old.lazy == factory.lazy {
		// The plugins rel_context registered stay: the tables of earlier
		// queries are theirs.
		m.checkedRelationFavourite = relationFavourite
		return nil
	}
	m.defaultChecked, m.defaultCheckedRelation, m.checkedRelationFavourite = nil, nil, false
	if factory == nil {
		return nil
	}
	factory.manager = m
	second := *factory
	second.relationFavourite = true
	m.defaultChecked = factory
	m.defaultCheckedRelation = &second
	m.checkedRelationFavourite = relationFavourite
	if m.kindPlugins != nil {
		m.registerCheckTableRelationPlugins()
	}
	return nil
}

// predicateCheckedFactory is the check_table plugin of a predicate's
// relation (mk_empty_relation): the favourite table_relation plugin's
// when it is the favourite relation plugin, else mk_empty_table's.
func (m *RelationManager) predicateCheckedFactory() *FiniteCheckedFactory {
	if m.checkedRelationFavourite {
		return m.defaultCheckedRelation
	}
	return m.defaultChecked
}

// SetLazySparseFiniteFactory borrows the context-owned factory used when
// lazy_sparse is the favourite table. The manager closes its table owners
// first; the context closes the factory afterwards.
func (m *RelationManager) SetLazySparseFiniteFactory(factory *LazySparseFiniteFactory) error {
	if err := m.open(); err != nil {
		return err
	}
	if factory != nil {
		if err := factory.open(); err != nil {
			return err
		}
	}
	if factory != nil && factory.engine.sparse == nil {
		factory.engine.sparse = m.sparse
	}
	m.lazyFinite = factory
	return nil
}

// SetDefaultFiniteTable selects the source favourite table plugin used for
// future finite relation and compiler-temporary construction. Existing
// predicate owners are not migrated; explicit predicate selections therefore
// keep precedence over this default.
func (m *RelationManager) SetDefaultFiniteTable(name string) error {
	if err := m.open(); err != nil {
		return err
	}
	switch name {
	case "hashtable", "sparse", "bitvector":
		m.defaultFinite = name
		return nil
	case "lazy_sparse":
		if m.lazyFinite == nil || m.lazyFinite.open() != nil {
			return ErrTable
		}
		m.defaultFinite = name
		return nil
	default:
		return ErrTable
	}
}

// SetFavouriteTableRelation records a table_relation favourite relation
// plugin (relation_manager::register_plugin(table_plugin*)): the plugin
// named tr_<table> by datalog.default_relation becomes
// m_favourite_relation_plugin unless the default table's plugin, registered
// after it, takes its place. table is "" when the favourite is not a
// table_relation plugin or is the default table's.
func (m *RelationManager) SetFavouriteTableRelation(table string) error {
	if err := m.open(); err != nil {
		return err
	}
	m.favouriteTable = table
	return nil
}

// newFavouriteFiniteTable is mk_empty_relation(s, null_family_id) for a
// table signature when no relation plugin handles it: the table_relation
// favourite's table when it handles the signature (table_relation_plugin::
// can_handle_signature), otherwise mk_empty_table_relation's table of the
// appropriate table plugin.
func (m *RelationManager) newFavouriteFiniteTable(signature Signature) (FiniteTable, error) {
	if m.checkedFinite == nil && m.checkedRelationFavourite && m.defaultCheckedRelation.CanHandleSignature(signature) {
		return m.defaultCheckedRelation.New(signature)
	}
	if m.checkedFactory() == nil && m.favouriteTable != "" {
		if p := m.tableRelationPluginNamed(m.favouriteTable); p != nil && p.canHandle(signature) {
			return p.newTable(signature)
		}
	}
	return m.NewDefaultFiniteTable(signature)
}

// SetRequestedFiniteKind records rel_context::set_predicate_representation's
// requested table plugin for predicate (kind is tr_sparse, tr_hashtable,
// tr_bitvector or tr_lazy_sparse); "" removes the request.
func (m *RelationManager) SetRequestedFiniteKind(predicate ast.ASTHandle, kind string) error {
	if err := m.open(); err != nil {
		return err
	}
	if kind == "" {
		delete(m.requestedFinite, predicate)
		return nil
	}
	if m.requestedFinite == nil {
		m.requestedFinite = make(map[ast.ASTHandle]string)
	}
	m.requestedFinite[predicate] = kind
	return nil
}

// NewFiniteTableForPredicate is relation_manager::mk_empty_relation(s, pred):
// the predicate's requested plugin when it can handle the signature,
// otherwise the appropriate default plugin.
func (m *RelationManager) NewFiniteTableForPredicate(predicate ast.ASTHandle, signature Signature) (FiniteTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if relation, ok, err := m.relationForPredicate(predicate); err != nil || ok {
		return relation, err
	}
	return m.newRequestedFiniteTable(predicate, signature)
}

// newRequestedFiniteTable is mk_empty_table_relation for a signature that
// is not necessarily the predicate's (a register of the predicate's rule):
// the predicate's requested table plugin, else the appropriate default.
func (m *RelationManager) newRequestedFiniteTable(predicate ast.ASTHandle, signature Signature) (FiniteTable, error) {
	if m.checkedFactory() == nil {
		switch m.requestedFinite[predicate] {
		case "tr_hashtable":
			return NewHashTable(signature), nil
		case "tr_sparse":
			if signature.Len() != 0 {
				return m.sparse.MkEmpty(signature, 0)
			}
		case "tr_bitvector":
			if BitvectorCanHandleSignature(signature, 0) {
				return newBitvectorTableIn(m.sparse, signature)
			}
		case "tr_lazy_sparse":
			if m.lazyFinite != nil && m.lazyFinite.CanHandleSignature(signature) {
				return m.lazyFinite.New(signature)
			}
		}
	}
	return m.newFavouriteFiniteTable(signature)
}

// NewDefaultFiniteTable mirrors get_appropriate_plugin: the configured sparse
// plugin is preferred when it handles the signature, while the empty
// signature falls through to the always-applicable Hash implementation.
func (m *RelationManager) NewDefaultFiniteTable(signature Signature) (FiniteTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if m.checkedFactory() != nil {
		if m.checkedFactory().CanHandleSignature(signature) {
			return m.checkedFactory().New(signature)
		}
		// The source checked favourite rejects a signature if either wrapped
		// plugin rejects it, then registered-plugin selection falls through to
		// the first capable implementation. Hash is that fallback for nullary.
		return NewHashTable(signature), nil
	}
	switch m.defaultFinite {
	case "sparse":
		if signature.Len() != 0 {
			return m.sparse.MkEmpty(signature, 0)
		}
	case "bitvector":
		if BitvectorCanHandleSignature(signature, 0) {
			return newBitvectorTableIn(m.sparse, signature)
		}
		// try_get_appropriate_plugin: the first registered plugin that
		// handles the signature, sparse for every nonnullary signature (the
		// bitvector plugin handles the nullary one).
		if signature.Len() != 0 {
			return m.sparse.MkEmpty(signature, 0)
		}
	case "lazy_sparse":
		if m.lazyFinite == nil {
			return nil, ErrTable
		}
		if m.lazyFinite.CanHandleSignature(signature) {
			return m.lazyFinite.New(signature)
		}
		// Nullary: z3 4.7.1 segfaults creating a 0-column sparse table via
		// lazy_table_plugin::mk_empty (AUDIT.md muz_rel_04-F2); the fixed
		// fork falls back to the manager's plugin choice, Hash, as here.
	}
	return NewHashTable(signature), nil
}

// relationForPredicate is mk_empty_relation(sig, pred)'s relation-plugin
// branch: the requested or favourite relation plugin when it handles the
// predicate's signature. ok is false when the table_relation path applies.
func (m *RelationManager) relationForPredicate(predicate ast.ASTHandle) (FiniteTable, bool, error) {
	if predicate.IsNull() || len(m.relationPlugins) == 0 {
		return nil, false, nil
	}
	if !m.deferRelationCreation {
		m.deferRelationCreation = true
		defer func() { m.deferRelationCreation = false }()
	}
	sorts, err := m.predicateSorts(predicate)
	if err != nil {
		return nil, false, err
	}
	if r, ok, err := m.requestedKindRelation(predicate, sorts, false); err != nil || ok {
		return r, ok, err
	}
	plugin, err := m.relationPluginFor(predicate, sorts)
	if err != nil {
		return nil, false, err
	}
	if plugin == nil {
		// mk_empty_table_relation, then the relation plugin list.
		if _, ok := m.signatureIsTable(sorts); ok {
			return nil, false, nil
		}
		if plugin, err = m.firstRelationPlugin(sorts); err != nil {
			return nil, false, err
		}
		if plugin == nil {
			// If there is no plugin to handle the signature, we just
			// create an empty product relation and stuff will be added to
			// it by later operations.
			plugin = m.productPlugin()
		}
	}
	if check, ok := plugin.(*CheckRelationPlugin); ok {
		// A placeholder until get_relation creates it.
		relation, err := check.mkEmptyPlaceholder(sorts)
		if err != nil {
			return nil, false, err
		}
		return relation, true, nil
	}
	relation, err := plugin.MkEmpty(sorts)
	if err != nil {
		return nil, false, err
	}
	return relation, true, nil
}

// predicateSorts is relation_manager::from_predicate(pred, sig).
func (m *RelationManager) predicateSorts(declaration ast.ASTHandle) ([]ast.ASTHandle, error) {
	arity, err := m.manager.GetArity(declaration)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		if sorts[i], err = m.manager.GetDomain(declaration, i); err != nil {
			return nil, err
		}
	}
	return sorts, nil
}

// DefaultCheckedHandles reports whether the default check_table favourite
// handles the signature of sorts.
func (m *RelationManager) DefaultCheckedHandles(sorts []ast.ASTHandle) bool {
	if m.defaultChecked == nil {
		return false
	}
	signature, ok := m.signatureIsTable(sorts)
	return ok && m.predicateCheckedFactory().CanHandleSignature(signature)
}

// NewDefaultCheckedPredicateTable is mk_empty_relation's check table of a
// predicate (predicateCheckedFactory's mk_empty).
func (m *RelationManager) NewDefaultCheckedPredicateTable(signature Signature) (FiniteTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if m.defaultChecked == nil {
		return nil, ErrCheckTable
	}
	return m.predicateCheckedFactory().New(signature)
}

// NewDefaultCheckedPlaceholder is the default check_table favourite's table
// for a predicate whose relation the source creates later: its mk_empty
// line is printed at that creation (MaterializeFiniteRelation).
func (m *RelationManager) NewDefaultCheckedPlaceholder(signature Signature) (FiniteTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if m.defaultChecked == nil {
		return nil, ErrCheckTable
	}
	pair, err := m.predicateCheckedFactory().newPair(signature, 0)
	if err != nil {
		return nil, err
	}
	pair.deferredMkEmpty = true
	return pair, nil
}
