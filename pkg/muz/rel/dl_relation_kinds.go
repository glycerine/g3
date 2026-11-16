package rel

// Source: src/muz/rel/dl_relation_manager.cpp get_next_relation_fid,
// get_relation_plugin(family_id), set_predicate_kind,
// get_requested_predicate_kind, mk_empty_relation(s, kind),
// mk_full_relation(s, p, kind), try_mk_default_filter_by_intersection_fn and
// default_relation_intersection_filter_fn; src/muz/rel/dl_table_relation.cpp
// table_relation_plugin (the relation view of a table that relation
// plugins such as product_relation hold as an inner relation).
//
// Relation kinds are the C++ family ids: rel_context registers the table
// plugins sparse, hashtable, bitvector and lazy sparse first, so their
// table_relation plugins hold kinds 0..3, and the relation plugins follow
// in registration order.

import (
	"errors"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

const nullRelationKind = -1

// tableRelationPluginNames are rel_context's table plugins in registration
// order (sparse, hashtable, bitvector, lazy_table_plugin::mk_sparse).
var tableRelationPluginNames = []string{"sparse", "hashtable", "bitvector", "lazy_sparse"}

// kindedRelationPlugin is implemented by plugins whose relations carry a
// kind other than the plugin's own (product_relation): the C++
// can_handle_signature(s, kind), mk_empty(s, kind), mk_full(p, s, kind).
type kindedRelationPlugin interface {
	CanHandleSignatureKind(sorts []ast.ASTHandle, kind int) (bool, error)
	MkEmptyKind(sorts []ast.ASTHandle, kind int) (RelationBase, error)
	MkFullKind(predicate ast.ASTHandle, sorts []ast.ASTHandle, kind int) (RelationBase, error)
}

// kindedRelation is relation_base::get_kind() for relations whose kind is
// not their plugin's.
type kindedRelation interface {
	RelationKind() int
}

// relationPluginInitializer is relation_plugin::initialize(fid).
type relationPluginInitializer interface {
	initializeKind(fid int)
}

func (m *RelationManager) initRelationKinds() {
	if m.kindPlugins != nil {
		return
	}
	m.kindPlugins = make(map[int]RelationPlugin)
	m.pluginKinds = make(map[RelationPlugin]int)
	for _, name := range tableRelationPluginNames {
		p := &TableRelationPlugin{manager: m, table: name}
		m.tableRelationPlugins = append(m.tableRelationPlugins, p)
		m.claimRelationKind(p)
	}
}

// claimRelationKind is get_next_relation_fid(claimer).
func (m *RelationManager) claimRelationKind(p RelationPlugin) int {
	fid := m.nextRelationKind
	m.nextRelationKind++
	m.kindPlugins[fid] = p
	if _, found := m.pluginKinds[p]; !found {
		m.pluginKinds[p] = fid
	}
	return fid
}

// PluginKind is relation_plugin::get_kind().
func (m *RelationManager) PluginKind(p RelationPlugin) int {
	m.initRelationKinds()
	if kind, found := m.pluginKinds[p]; found {
		return kind
	}
	return nullRelationKind
}

// RelationKindOf is relation_base::get_kind() of a register value: a
// table's table_relation plugin kind, or the relation's kind.
func (m *RelationManager) RelationKindOf(t FiniteTable) int {
	m.initRelationKinds()
	if k, ok := t.(kindedRelation); ok {
		return k.RelationKind()
	}
	if r, ok := AsRelation(t); ok {
		return m.PluginKind(r.RelationPlugin())
	}
	name := finiteTablePlugin(t)
	for i, p := range m.tableRelationPlugins {
		if p.table == name {
			return i
		}
	}
	return nullRelationKind
}

// relationPluginByName is get_relation_plugin(symbol) over the
// table_relation plugins and the relation plugins.
func (m *RelationManager) relationPluginByName(name string) RelationPlugin {
	m.initRelationKinds()
	for _, p := range m.tableRelationPlugins {
		if p.Name() == name {
			return p
		}
	}
	return m.GetRelationPlugin(name)
}

// RelationPluginKind returns the kind of the plugin named name
// (get_ordinary_relation_plugin(name).get_kind()); ok is false when no
// such plugin exists.
func (m *RelationManager) RelationPluginKind(name string) (int, bool) {
	p := m.relationPluginByName(name)
	if p == nil {
		return nullRelationKind, false
	}
	return m.PluginKind(p), true
}

// SetRequestedKind is relation_manager::set_predicate_kind.
func (m *RelationManager) SetRequestedKind(predicate ast.ASTHandle, kind int) error {
	if err := m.open(); err != nil {
		return err
	}
	if m.requestedKinds == nil {
		m.requestedKinds = make(map[ast.ASTHandle]int)
	}
	m.requestedKinds[predicate] = kind
	return nil
}

// requestedKind is get_requested_predicate_kind.
func (m *RelationManager) requestedKind(predicate ast.ASTHandle) int {
	if predicate.IsNull() {
		return nullRelationKind
	}
	if kind, found := m.requestedKinds[predicate]; found {
		return kind
	}
	if p := m.requestedRelation[predicate]; p != nil {
		return m.PluginKind(p)
	}
	return nullRelationKind
}

func canHandleKind(p RelationPlugin, sorts []ast.ASTHandle, kind int) (bool, error) {
	if k, ok := p.(kindedRelationPlugin); ok {
		return k.CanHandleSignatureKind(sorts, kind)
	}
	return p.CanHandleSignature(sorts)
}

// mkEmptyRelationKind is relation_manager::mk_empty_relation(s, kind) as a
// relation_base: tables are their table_relation view.
func (m *RelationManager) mkEmptyRelationKind(sorts []ast.ASTHandle, kind int) (RelationBase, error) {
	m.initRelationKinds()
	if kind != nullRelationKind {
		p := m.kindPlugins[kind]
		ok, err := canHandleKind(p, sorts, kind)
		if err != nil {
			return nil, err
		}
		if ok {
			if k, isKinded := p.(kindedRelationPlugin); isKinded {
				return k.MkEmptyKind(sorts, kind)
			}
			return p.MkEmpty(sorts)
		}
	}
	if p := m.favouriteRelation; p != nil {
		ok, err := p.CanHandleSignature(sorts)
		if err != nil {
			return nil, err
		}
		if ok {
			return p.MkEmpty(sorts)
		}
	}
	if signature, ok := m.signatureIsTable(sorts); ok {
		table, err := m.NewDefaultFiniteTable(signature)
		if err != nil {
			return nil, err
		}
		return m.tableView(table, sorts, false), nil
	}
	p, err := m.firstRelationPlugin(sorts)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p.MkEmpty(sorts)
	}
	return m.productPlugin().MkEmpty(sorts)
}

// mkFullRelationKind is relation_manager::mk_full_relation(s, p, kind).
func (m *RelationManager) mkFullRelationKind(sorts []ast.ASTHandle, predicate ast.ASTHandle, kind int) (RelationBase, error) {
	m.initRelationKinds()
	if kind != nullRelationKind {
		p := m.kindPlugins[kind]
		ok, err := canHandleKind(p, sorts, kind)
		if err != nil {
			return nil, err
		}
		if ok {
			if k, isKinded := p.(kindedRelationPlugin); isKinded {
				return k.MkFullKind(predicate, sorts, kind)
			}
			return p.MkFull(predicate, sorts)
		}
	}
	p, err := m.appropriatePlugin(sorts)
	if err != nil {
		return nil, err
	}
	return p.MkFull(predicate, sorts)
}

// appropriatePlugin is get_appropriate_plugin(relation_signature): the
// favourite, then m_relation_plugins in order (table_relation plugins
// first).
func (m *RelationManager) appropriatePlugin(sorts []ast.ASTHandle) (RelationPlugin, error) {
	m.initRelationKinds()
	if p := m.favouriteRelation; p != nil {
		ok, err := p.CanHandleSignature(sorts)
		if err != nil || ok {
			return p, err
		}
	}
	if signature, ok := m.signatureIsTable(sorts); ok {
		// The favourite table's relation plugin (default_table) handles a
		// table signature first.
		if name := m.defaultFinite; name != "" {
			if p := m.tableRelationPluginNamed(name); p != nil && p.canHandle(signature) {
				return p, nil
			}
		}
		for _, p := range m.tableRelationPlugins {
			if p.canHandle(signature) {
				return p, nil
			}
		}
	}
	p, err := m.firstRelationPlugin(sorts)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, z3exception.NewDefaultException("no suitable plugin found for given relation signature")
	}
	return p, nil
}

func (m *RelationManager) tableRelationPluginNamed(table string) *TableRelationPlugin {
	m.initRelationKinds()
	for _, p := range m.tableRelationPlugins {
		if p.table == table {
			return p
		}
	}
	return nil
}

// requestedKindRelation is mk_empty_relation / mk_full_relation's first
// branch for a requested kind whose plugin is not a plain relation plugin
// (a product kind or a table_relation kind): ok is false when the
// remaining selection applies.
func (m *RelationManager) requestedKindRelation(predicate ast.ASTHandle, sorts []ast.ASTHandle, full bool) (FiniteTable, bool, error) {
	kind, found := m.requestedKinds[predicate]
	if predicate.IsNull() || !found || kind == nullRelationKind {
		return nil, false, nil
	}
	m.initRelationKinds()
	p := m.kindPlugins[kind]
	if _, isTable := p.(*TableRelationPlugin); isTable {
		// A table_relation kind: the requested table plugin (handled by
		// the table selection through requestedFinite) or the fallbacks.
		return nil, false, nil
	}
	ok, err := canHandleKind(p, sorts, kind)
	if err != nil || !ok {
		return nil, false, err
	}
	var r RelationBase
	if full {
		r, err = m.mkFullRelationKind(sorts, predicate, kind)
	} else {
		r, err = m.mkEmptyRelationKind(sorts, kind)
	}
	if err != nil {
		return nil, false, err
	}
	return unwrapTableView(r), true, nil
}

// ---------------------------------------------------------------------------
// table_relation_plugin

// TableRelationPlugin is table_relation_plugin for one table plugin. Its
// relations are tableRelationView values around a FiniteTable; registers
// hold the table itself.
type TableRelationPlugin struct {
	RelationPluginDefaults
	manager *RelationManager
	table   string
}

func (p *TableRelationPlugin) Name() string              { return "tr_" + p.table }
func (p *TableRelationPlugin) Manager() *RelationManager { return p.manager }

func (p *TableRelationPlugin) canHandle(signature Signature) bool {
	switch p.table {
	case "sparse":
		return signature.Len() != 0
	case "bitvector":
		return BitvectorCanHandleSignature(signature, 0)
	case "lazy_sparse":
		return p.manager.lazyFinite != nil && p.manager.lazyFinite.CanHandleSignature(signature)
	}
	return true
}

func (p *TableRelationPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	signature, ok := p.manager.signatureIsTable(sorts)
	return ok && p.canHandle(signature), nil
}

func (p *TableRelationPlugin) newTable(signature Signature) (FiniteTable, error) {
	switch p.table {
	case "sparse":
		return NewSparseTable(signature, 0)
	case "bitvector":
		return NewBitvectorTable(signature)
	case "lazy_sparse":
		return p.manager.lazyFinite.New(signature)
	}
	return NewHashTable(signature), nil
}

func (p *TableRelationPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	signature, ok := p.manager.signatureIsTable(sorts)
	if !ok {
		return nil, ErrTable
	}
	table, err := p.newTable(signature)
	if err != nil {
		return nil, err
	}
	return p.manager.tableView(table, sorts, false), nil
}

func (p *TableRelationPlugin) MkFull(_ ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	r, err := p.MkEmpty(sorts)
	if err != nil {
		return nil, err
	}
	v := r.(*tableRelationView)
	signature, _ := v.table.Signature()
	if err = fillTotalTable(v.table, signature); err != nil {
		return nil, errors.Join(err, v.Close())
	}
	return v, nil
}

func (p *TableRelationPlugin) view(r RelationBase) (*tableRelationView, bool) {
	v, ok := r.(*tableRelationView)
	return v, ok
}

func (p *TableRelationPlugin) MkJoinFn(r1, r2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	if _, ok := p.view(r1); !ok {
		return nil, nil
	}
	if _, ok := p.view(r2); !ok {
		return nil, nil
	}
	sorts := joinSorts(r1.RelationSignature(), r2.RelationSignature())
	return func(a, b RelationBase) (RelationBase, error) {
		x, _ := p.view(a)
		y, _ := p.view(b)
		if x == nil || y == nil {
			return nil, ErrInstruction
		}
		t, err := finiteTableJoin(x.table, y.table, cols1, cols2)
		if err != nil {
			return nil, err
		}
		return p.manager.tableView(t, sorts, false), nil
	}, nil
}

func (p *TableRelationPlugin) MkJoinProjectFn(r1, r2 RelationBase, cols1, cols2, removed []int) (RelationJoinFn, error) {
	if _, ok := p.view(r1); !ok {
		return nil, nil
	}
	if _, ok := p.view(r2); !ok {
		return nil, nil
	}
	sorts := projectSorts(joinSorts(r1.RelationSignature(), r2.RelationSignature()), removed)
	return func(a, b RelationBase) (RelationBase, error) {
		x, _ := p.view(a)
		y, _ := p.view(b)
		if x == nil || y == nil {
			return nil, ErrInstruction
		}
		t, err := finiteTableJoinProject(x.table, y.table, cols1, cols2, removed)
		if err != nil {
			return nil, err
		}
		return p.manager.tableView(t, sorts, false), nil
	}, nil
}

func (p *TableRelationPlugin) MkProjectFn(r RelationBase, removed []int) (RelationTransformerFn, error) {
	if _, ok := p.view(r); !ok {
		return nil, nil
	}
	sorts := projectSorts(r.RelationSignature(), removed)
	return func(a RelationBase) (RelationBase, error) {
		x, ok := p.view(a)
		if !ok {
			return nil, ErrInstruction
		}
		t, err := finiteTableProject(x.table, removed)
		if err != nil {
			return nil, err
		}
		return p.manager.tableView(t, sorts, false), nil
	}, nil
}

func (p *TableRelationPlugin) MkRenameFn(r RelationBase, cycle []int) (RelationTransformerFn, error) {
	if _, ok := p.view(r); !ok {
		return nil, nil
	}
	sorts := renameSorts(r.RelationSignature(), cycle)
	return func(a RelationBase) (RelationBase, error) {
		x, ok := p.view(a)
		if !ok {
			return nil, ErrInstruction
		}
		t, err := finiteTableRename(x.table, cycle)
		if err != nil {
			return nil, err
		}
		return p.manager.tableView(t, sorts, false), nil
	}, nil
}

// MkUnionFn: tr_union_fn for tables, universal_target_union_fn for a table
// source and a non-table target or delta.
func (p *TableRelationPlugin) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	if _, ok := p.view(source); !ok {
		return nil, nil
	}
	_, tgtTable := p.view(target)
	deltaTable := true
	if delta != nil {
		_, deltaTable = p.view(delta)
	}
	if !tgtTable || !deltaTable {
		return p.universalTargetUnion, nil
	}
	return func(t, s, d RelationBase) error {
		x, _ := p.view(t)
		y, _ := p.view(s)
		var z FiniteTable
		if d != nil {
			dv, _ := p.view(d)
			z = dv.table
		}
		return finiteTableUnion(x.table, y.table, z)
	}, nil
}

func (p *TableRelationPlugin) MkWidenFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return nil, nil
}

// universalTargetUnion is universal_target_union_fn.
func (p *TableRelationPlugin) universalTargetUnion(tgt, src, delta RelationBase) error {
	v, ok := p.view(src)
	if !ok {
		return ErrInstruction
	}
	facts, err := v.table.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if delta != nil {
			found, err := tgt.Contains(fact)
			if err != nil {
				return err
			}
			if !found {
				if _, err = tgt.Add(fact); err != nil {
					return err
				}
				if _, err = delta.Add(fact); err != nil {
					return err
				}
			}
			continue
		}
		if _, err = tgt.Add(fact); err != nil {
			return err
		}
	}
	return nil
}

func (p *TableRelationPlugin) MkFilterIdenticalFn(t RelationBase, cols []int) (RelationMutatorFn, error) {
	if _, ok := p.view(t); !ok {
		return nil, nil
	}
	return func(a RelationBase) error {
		x, ok := p.view(a)
		if !ok {
			return ErrInstruction
		}
		return finiteTableFilterIdentical(x.table, cols)
	}, nil
}

func (p *TableRelationPlugin) MkFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error) {
	if _, ok := p.view(t); !ok {
		return nil, nil
	}
	element, err := p.manager.EncodeValue(t.RelationSignature()[col], value)
	if err != nil {
		return nil, err
	}
	return func(a RelationBase) error {
		x, ok := p.view(a)
		if !ok {
			return ErrInstruction
		}
		return finiteTableFilterEqual(x.table, element, col)
	}, nil
}

func (p *TableRelationPlugin) MkFilterByNegationFn(t, neg RelationBase, tCols, negCols []int) (RelationIntersectionFilterFn, error) {
	if _, ok := p.view(t); !ok {
		return nil, nil
	}
	if _, ok := p.view(neg); !ok {
		return nil, nil
	}
	return func(a, b RelationBase) error {
		x, _ := p.view(a)
		y, _ := p.view(b)
		if x == nil || y == nil {
			return ErrInstruction
		}
		return finiteTableFilterNegation(x.table, y.table, tCols, negCols)
	}, nil
}

// tableRelationView is a table_relation: a table seen as a relation_base.
// A borrowed view does not own its table.
type tableRelationView struct {
	plugin   *TableRelationPlugin
	table    FiniteTable
	sorts    []ast.ASTHandle
	borrowed bool
}

// tableView wraps table as its table plugin's table_relation.
func (m *RelationManager) tableView(table FiniteTable, sorts []ast.ASTHandle, borrowed bool) *tableRelationView {
	m.initRelationKinds()
	p := m.tableRelationPluginNamed(finiteTablePlugin(table))
	if p == nil {
		p = m.tableRelationPluginNamed("hashtable")
	}
	return &tableRelationView{plugin: p, table: table, sorts: append([]ast.ASTHandle(nil), sorts...), borrowed: borrowed}
}

// unwrapTableView returns the table of a table_relation, which registers
// and the relation map hold directly.
func unwrapTableView(r RelationBase) FiniteTable {
	if v, ok := r.(*tableRelationView); ok {
		return v.table
	}
	return r
}

func (v *tableRelationView) RelationSignature() []ast.ASTHandle { return v.sorts }
func (v *tableRelationView) RelationPlugin() RelationPlugin     { return v.plugin }
func (v *tableRelationView) Signature() (Signature, error)      { return v.table.Signature() }
func (v *tableRelationView) Facts() ([]Fact, error)             { return v.table.Facts() }
func (v *tableRelationView) Add(f Fact) (bool, error)           { return v.table.Add(f) }
func (v *tableRelationView) Contains(f Fact) (bool, error)      { return v.table.Contains(f) }
func (v *tableRelationView) Len() (int, error)                  { return v.table.Len() }
func (v *tableRelationView) Reset() error                       { return v.table.Reset() }
func (v *tableRelationView) IsPrecise() bool                    { return true }
func (v *tableRelationView) Empty() (bool, error) {
	n, err := v.table.Len()
	return n == 0, err
}
func (v *tableRelationView) FastEmpty() bool {
	empty, _ := v.Empty()
	return empty
}
func (v *tableRelationView) Close() error {
	if v.borrowed {
		return nil
	}
	return v.table.Close()
}
func (v *tableRelationView) CloneFinite() (FiniteTable, error) { return v.CloneRelation() }
func (v *tableRelationView) CloneRelation() (RelationBase, error) {
	clone, err := v.table.CloneFinite()
	if err != nil {
		return nil, err
	}
	return v.plugin.manager.tableView(clone, v.sorts, false), nil
}
func (v *tableRelationView) SizeEstimateRows() (uint32, error) {
	rows, _, err := finiteTableSizeEstimates(v.table)
	return rows, err
}
func (v *tableRelationView) SizeEstimateBytes() (uint32, error) {
	_, bytes, err := finiteTableSizeEstimates(v.table)
	return bytes, err
}
func (v *tableRelationView) Display(out io.Writer) error {
	facts, err := v.table.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		for i, e := range fact {
			if i > 0 {
				if _, err = io.WriteString(out, " "); err != nil {
					return err
				}
			}
			if _, err = io.WriteString(out, uitoa(uint64(e))); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func uitoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

// ToFormula is table_relation::to_formula: the disjunction of the rows'
// conjunctions of column equalities.
func (v *tableRelationView) ToFormula() (result ast.ASTRef, resultErr error) {
	m := v.plugin.manager
	facts, err := v.table.Facts()
	if err != nil {
		return ast.ASTRef{}, err
	}
	rows := make([]ast.ASTRef, 0, len(facts))
	defer func() { resultErr = errors.Join(resultErr, releaseTableFormulaRefs(m.manager, rows)) }()
	handles := make([]ast.ASTHandle, 0, len(facts))
	for _, fact := range facts {
		row, err := tableRelationRowFormula(m, v.sorts, fact)
		if err != nil {
			return ast.ASTRef{}, err
		}
		rows = append(rows, row)
		h, err := m.manager.RefGet(row)
		if err != nil {
			return ast.ASTRef{}, err
		}
		handles = append(handles, h)
	}
	result, err = m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = rewriter.SimplifyBooleanGateDefault(m.manager, ast.ASTOpOr, handles, result); err != nil {
		return result, errors.Join(err, m.manager.DelRef(&result))
	}
	return result, nil
}

var _ RelationBase = (*tableRelationView)(nil)
var _ RelationPlugin = (*TableRelationPlugin)(nil)

// ---------------------------------------------------------------------------
// default filter_by_intersection

// MkRelationFilterByIntersectionFn is relation_manager::
// mk_filter_by_intersection_fn(t, src, cols...): the plugins', then
// try_mk_default_filter_by_intersection_fn.
func (m *RelationManager) MkRelationFilterByIntersectionFn(t, src RelationBase, tCols, srcCols []int) (RelationIntersectionFilterFn, error) {
	res, err := t.RelationPlugin().MkFilterByIntersectionFn(t, src, tCols, srcCols)
	if err != nil || res != nil {
		return res, err
	}
	if t.RelationPlugin() != src.RelationPlugin() {
		if res, err = src.RelationPlugin().MkFilterByIntersectionFn(t, src, tCols, srcCols); err != nil || res != nil {
			return res, err
		}
	}
	return m.tryMkDefaultFilterByIntersectionFn(t, src, tCols, srcCols)
}

// MkRelationFilterByIntersectionAllFn is mk_filter_by_intersection_fn(tgt,
// src) over all columns.
func (m *RelationManager) MkRelationFilterByIntersectionAllFn(t, src RelationBase) (RelationIntersectionFilterFn, error) {
	cols := make([]int, len(t.RelationSignature()))
	for i := range cols {
		cols[i] = i
	}
	return m.MkRelationFilterByIntersectionFn(t, src, cols, cols)
}

// relationSwapper is relation_base::can_swap / swap.
type relationSwapper interface {
	canSwap(other RelationBase) bool
	swap(other RelationBase)
}

// tryMkDefaultFilterByIntersectionFn is
// try_mk_default_filter_by_intersection_fn and
// default_relation_intersection_filter_fn.
func (m *RelationManager) tryMkDefaultFilterByIntersectionFn(t, src RelationBase, tCols, srcCols []int) (_ RelationIntersectionFilterFn, resultErr error) {
	tsz, ssz := len(t.RelationSignature()), len(src.RelationSignature())
	removed := make([]int, 0, ssz)
	for i := tsz; i < tsz+ssz; i++ {
		removed = append(removed, i)
	}
	join, err := m.MkRelationJoinProjectFn(t, src, tCols, srcCols, removed, false)
	if err != nil || join == nil {
		return nil, err
	}
	// we perform the join operation here to see what the result is
	joined, err := join(t, src)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, joined.Close()) }()
	var union RelationUnionFn
	if s, ok := t.(relationSwapper); !ok || !s.canSwap(joined) {
		if _, isProduct := joined.RelationPlugin().(*ProductRelationPlugin); isProduct {
			return nil, nil
		}
		if union, err = m.MkRelationUnionFn(t, joined, nil); err != nil || union == nil {
			return nil, err
		}
	}
	return func(tgt, other RelationBase) (result error) {
		filtered, err := join(tgt, other)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, filtered.Close()) }()
		if union == nil {
			tgt.(relationSwapper).swap(filtered)
		}
		if err = tgt.Reset(); err != nil {
			return err
		}
		if union == nil {
			// (*m_union_fun)(tgt, *filtered_rel) with a null functor.
			return z3exception.NewDefaultException("intersection union does not exist")
		}
		return union(tgt, filtered, nil)
	}, nil
}

var _ = math.MaxUint32
