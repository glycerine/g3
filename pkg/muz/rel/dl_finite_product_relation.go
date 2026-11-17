package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp/.h
// finite_product_relation_plugin and finite_product_relation. A relation
// is a table over the columns that have table sorts plus a functional
// column holding an index into a vector of inner relations (of the inner
// plugin) over the remaining columns. In z3 4.7.1 the plugin is registered
// only by mk_explanations, over the fact_explanation plugin.
//
// The table is the sparse table plugin's (get_appropriate_plugin of the
// table signature with a functional INT_MAX column: the hashtable and
// bitvector plugins cannot handle it). The table operations are the
// relation manager's (finiteTable*), with default_table_map_fn and
// default_table_project_with_reduce_fn as relation_manager::mk_map_fn and
// mk_project_with_reduce_fn (the sparse plugin implements neither).

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var _ RelationPlugin = (*FiniteProductRelationPlugin)(nil)
var _ RelationBase = (*finiteProductRelation)(nil)

// fprRelIdxSort is finite_product_relation::s_rel_idx_sort (INT_MAX).
const fprRelIdxSort = uint64(math.MaxInt32)

// fprNoIndex is UINT_MAX in m_sig2table / m_sig2other / m_full_rel_idx.
const fprNoIndex = -1

// fprSpec is finite_product_relation_plugin::rel_spec.
type fprSpec struct {
	innerKind int
	tableCols []bool
}

func (s fprSpec) key() string {
	b := make([]byte, 0, len(s.tableCols)+8)
	for _, c := range s.tableCols {
		if c {
			b = append(b, '1')
		} else {
			b = append(b, '0')
		}
	}
	b = append(b, ':')
	b = append(b, uitoa(uint64(s.innerKind+1))...)
	return string(b)
}

// fprSpecStore is rel_spec_store<rel_spec> of the plugin.
type fprSpecStore struct {
	parent     *FiniteProductRelationPlugin
	allocated  []int
	assignment map[string]map[string]int
	specs      map[string]map[int]fprSpec
}

func (s *fprSpecStore) getRelationKind(sorts []ast.ASTHandle, spec fprSpec) int {
	key := sortsKey(sorts)
	ids, found := s.assignment[key]
	if !found {
		ids = make(map[string]int)
		s.assignment[key] = ids
		s.specs[key] = make(map[int]fprSpec)
	}
	sk := spec.key()
	idx, found := ids[sk]
	if !found {
		idx = len(ids)
		if idx == len(s.allocated) {
			s.allocated = append(s.allocated, s.parent.manager.claimRelationKind(s.parent))
		}
		ids[sk] = idx
		s.specs[key][s.allocated[idx]] = fprSpec{innerKind: spec.innerKind, tableCols: append([]bool(nil), spec.tableCols...)}
	}
	return s.allocated[idx]
}

func (s *fprSpecStore) getRelationSpec(sorts []ast.ASTHandle, kind int) (fprSpec, bool) {
	spec, found := s.specs[sortsKey(sorts)][kind]
	return spec, found
}

// FiniteProductRelationPlugin is finite_product_relation_plugin.
type FiniteProductRelationPlugin struct {
	RelationPluginDefaults
	manager *RelationManager
	inner   RelationPlugin
	kind    int
	store   fprSpecStore
}

// NewFiniteProductRelationPlugin is the plugin constructor (registered by
// the caller with register_plugin).
func NewFiniteProductRelationPlugin(inner RelationPlugin, manager *RelationManager) *FiniteProductRelationPlugin {
	p := &FiniteProductRelationPlugin{manager: manager, inner: inner, kind: nullRelationKind}
	p.store = fprSpecStore{parent: p, assignment: make(map[string]map[string]int), specs: make(map[string]map[int]fprSpec)}
	return p
}

// initializeKind is initialize(fid): m_spec_store.add_available_kind.
func (p *FiniteProductRelationPlugin) initializeKind(fid int) {
	p.kind = fid
	p.store.allocated = append(p.store.allocated, fid)
}

// Name is get_name(inner): "fpr_" + the inner plugin's name.
func (p *FiniteProductRelationPlugin) Name() string              { return "fpr_" + p.inner.Name() }
func (p *FiniteProductRelationPlugin) Manager() *RelationManager { return p.manager }

// InnerPlugin is get_inner_plugin().
func (p *FiniteProductRelationPlugin) InnerPlugin() RelationPlugin { return p.inner }

// get is the static_cast of a relation of this plugin (check_kind).
func (p *FiniteProductRelationPlugin) get(r RelationBase) (*finiteProductRelation, bool) {
	x, ok := r.(*finiteProductRelation)
	return x, ok && x.plugin == p
}

// relationSortToTable is relation_manager::relation_sort_to_table.
func (p *FiniteProductRelationPlugin) relationSortToTable(sort ast.ASTHandle) (uint64, bool) {
	return p.manager.SortDomain(sort)
}

// getAllPossibleTableColumns is get_all_possible_table_columns.
func (p *FiniteProductRelationPlugin) getAllPossibleTableColumns(sorts []ast.ASTHandle) []bool {
	cols := make([]bool, len(sorts))
	for i, s := range sorts {
		_, cols[i] = p.relationSortToTable(s)
	}
	return cols
}

// splitSignatures is split_signatures(s, table_sig, remaining_sig).
func (p *FiniteProductRelationPlugin) splitSignatures(sorts []ast.ASTHandle) ([]uint64, []ast.ASTHandle) {
	var table []uint64
	var remaining []ast.ASTHandle
	for _, s := range sorts {
		if d, ok := p.relationSortToTable(s); ok {
			table = append(table, d)
		} else {
			remaining = append(remaining, s)
		}
	}
	return table, remaining
}

// CanHandleSignature: the inner plugin handles the non-table columns and a
// table plugin the table columns (the hashtable plugin handles every
// signature without functional columns).
func (p *FiniteProductRelationPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	_, remaining := p.splitSignatures(sorts)
	return p.inner.CanHandleSignature(remaining)
}

// MkEmpty is mk_empty(s): every column that can be a table column is one
// (the _EXTERNAL_RELEASE build omits dbg_fpr_nonempty_relation_signature).
func (p *FiniteProductRelationPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	return p.mkEmptyCols(sorts, p.getAllPossibleTableColumns(sorts), nullRelationKind)
}

// mkEmptyCols is mk_empty(s, table_columns, inner_kind).
func (p *FiniteProductRelationPlugin) mkEmptyCols(sorts []ast.ASTHandle, tableCols []bool, innerKind int) (*finiteProductRelation, error) {
	return newFiniteProductRelation(p, sorts, tableCols, p.inner, innerKind)
}

// CanHandleSignatureKind is relation_plugin::can_handle_signature(s, kind).
func (p *FiniteProductRelationPlugin) CanHandleSignatureKind(sorts []ast.ASTHandle, kind int) (bool, error) {
	return p.CanHandleSignature(sorts)
}

// MkEmptyKind is mk_empty(s, kind): the kind's rel_spec.
func (p *FiniteProductRelationPlugin) MkEmptyKind(sorts []ast.ASTHandle, kind int) (RelationBase, error) {
	spec, found := p.store.getRelationSpec(sorts, kind)
	if !found {
		return p.MkEmpty(sorts)
	}
	return p.mkEmptyCols(sorts, spec.tableCols, spec.innerKind)
}

// MkFullKind is relation_plugin::mk_full(p, s, kind).
func (p *FiniteProductRelationPlugin) MkFullKind(predicate ast.ASTHandle, sorts []ast.ASTHandle, kind int) (RelationBase, error) {
	if kind == p.kind || kind == nullRelationKind {
		return p.MkFull(predicate, sorts)
	}
	aux, err := p.MkEmptyKind(sorts, kind)
	if err != nil {
		return nil, err
	}
	res, err := aux.(*finiteProductRelation).complement(predicate)
	return res, errors.Join(err, aux.Close())
}

// mkEmptyLike is mk_empty(original): mk_empty(sig, original.get_kind()).
func (p *FiniteProductRelationPlugin) mkEmptyLike(original *finiteProductRelation) (*finiteProductRelation, error) {
	r, err := p.MkEmptyKind(original.sorts, original.kind)
	if err != nil {
		return nil, err
	}
	return r.(*finiteProductRelation), nil
}

// MkFull is mk_full(p, s): mk_empty(s), complement_self(p).
func (p *FiniteProductRelationPlugin) MkFull(predicate ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	r, err := p.mkEmptyCols(sorts, p.getAllPossibleTableColumns(sorts), nullRelationKind)
	if err != nil {
		return nil, err
	}
	if err = r.complementSelf(predicate); err != nil {
		return nil, errors.Join(err, r.Close())
	}
	return r, nil
}

// getRelationKind is get_relation_kind(r, table_columns).
func (p *FiniteProductRelationPlugin) getRelationKind(sorts []ast.ASTHandle, tableCols []bool) int {
	return p.store.getRelationKind(sorts, fprSpec{innerKind: nullRelationKind, tableCols: tableCols})
}

// canConvertToTableRelation is can_convert_to_table_relation.
func (p *FiniteProductRelationPlugin) canConvertToTableRelation(r *finiteProductRelation) bool {
	return len(r.otherSig) == 0
}

// toTableRelation is to_table_relation: garbage_collect(true), project
// away the index column, mk_table_relation.
func (p *FiniteProductRelationPlugin) toTableRelation(r *finiteProductRelation) (RelationBase, error) {
	if err := r.garbageCollect(true); err != nil {
		return nil, err
	}
	removed := len(r.tableSig) - 1
	res, err := finiteTableProject(r.table, []int{removed})
	if err != nil {
		return nil, err
	}
	return p.manager.tableView(res, r.sorts, false), nil
}

// isTableRelation is relation_base::from_table().
func isTableRelation(r RelationBase) (*tableRelationView, bool) {
	v, ok := r.(*tableRelationView)
	return v, ok
}

// canBeConverted is can_be_converted.
func (p *FiniteProductRelationPlugin) canBeConverted(r RelationBase) (bool, error) {
	if r.RelationPlugin() == p.inner {
		return true, nil
	}
	if _, ok := isTableRelation(r); ok {
		return p.inner.CanHandleSignature(nil)
	}
	return false, nil
}

// newIdxSingleton is the idx_singleton table: signature (s_rel_idx_sort)
// with one functional column, holding the fact (0).
func newIdxSingleton() (*SparseTable, error) {
	t, err := NewSparseTable(Signature{domains: []uint64{fprRelIdxSort}}, 1)
	if err != nil {
		return nil, err
	}
	if _, err = t.Add(Fact{0}); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

// mkFromTableRelation is mk_from_table_relation (nil when the inner plugin
// cannot handle the empty signature).
func (p *FiniteProductRelationPlugin) mkFromTableRelation(r *tableRelationView) (_ *finiteProductRelation, resultErr error) {
	ok, err := p.inner.CanHandleSignature(nil)
	if err != nil || !ok {
		return nil, err
	}
	idx, err := newIdxSingleton()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, idx.Close()) }()
	resTable, err := finiteTableJoin(r.table, idx, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, resTable.Close()) }()
	tableCols := make([]bool, len(r.sorts))
	for i := range tableCols {
		tableCols[i] = true
	}
	res, err := p.mkEmptyCols(r.sorts, tableCols, nullRelationKind)
	if err != nil {
		return nil, err
	}
	inner, err := mkFullNullKind(p.inner, ast.ASTHandle{}, nil)
	if err != nil {
		return nil, errors.Join(err, res.Close())
	}
	if err = res.init(resTable, []RelationBase{inner}, true); err != nil {
		return nil, errors.Join(err, res.Close())
	}
	return res, nil
}

// mkFromInnerRelation is mk_from_inner_relation.
func (p *FiniteProductRelationPlugin) mkFromInnerRelation(r RelationBase) (_ *finiteProductRelation, resultErr error) {
	sorts := r.RelationSignature()
	idx, err := newIdxSingleton()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, idx.Close()) }()
	res, err := p.mkEmptyCols(sorts, make([]bool, len(sorts)), nullRelationKind)
	if err != nil {
		return nil, err
	}
	clone, err := r.CloneRelation()
	if err != nil {
		return nil, errors.Join(err, res.Close())
	}
	if err = res.init(idx, []RelationBase{clone}, true); err != nil {
		return nil, errors.Join(err, res.Close())
	}
	return res, nil
}

// convert is converting_join_fn::convert.
func (p *FiniteProductRelationPlugin) convert(r RelationBase) (*finiteProductRelation, error) {
	if r.RelationPlugin() == p.inner {
		return p.mkFromInnerRelation(r)
	}
	tr, ok := isTableRelation(r)
	if !ok {
		return nil, ErrInstruction
	}
	res, err := p.mkFromTableRelation(tr)
	if err == nil && res == nil {
		err = ErrInstruction
	}
	return res, err
}

// MkJoinFn is mk_join_fn: join_fn of two finite product relations, else
// converting_join_fn when every foreign operand can be converted.
func (p *FiniteProductRelationPlugin) MkJoinFn(rb1, rb2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	r1, ok1 := p.get(rb1)
	r2, ok2 := p.get(rb2)
	if !ok1 || !ok2 {
		if !ok1 {
			if c, err := p.canBeConverted(rb1); err != nil || !c {
				return nil, err
			}
		}
		if !ok2 {
			if c, err := p.canBeConverted(rb2); err != nil || !c {
				return nil, err
			}
		}
		return p.convertingJoin(cols1, cols2), nil
	}
	return newFprJoinFn(r1, r2, cols1, cols2).apply, nil
}

// convertingJoin is converting_join_fn.
func (p *FiniteProductRelationPlugin) convertingJoin(cols1, cols2 []int) RelationJoinFn {
	c1, c2 := append([]int(nil), cols1...), append([]int(nil), cols2...)
	var native RelationJoinFn
	return func(r1, r2 RelationBase) (_ RelationBase, resultErr error) {
		fpr1, ok := p.get(r1)
		if !ok {
			conv, err := p.convert(r1)
			if err != nil {
				return nil, err
			}
			defer func() { resultErr = errors.Join(resultErr, conv.Close()) }()
			fpr1 = conv
		}
		fpr2, ok := p.get(r2)
		if !ok {
			conv, err := p.convert(r2)
			if err != nil {
				return nil, err
			}
			defer func() { resultErr = errors.Join(resultErr, conv.Close()) }()
			fpr2 = conv
		}
		if native == nil {
			fn, err := p.manager.MkRelationJoinFn(fpr1, fpr2, c1, c2, false)
			if err != nil {
				return nil, err
			}
			if fn == nil {
				return nil, ErrInstruction
			}
			native = fn
		}
		return native(fpr1, fpr2)
	}
}

// fprJoinFn is finite_product_relation_plugin::join_fn.
type fprJoinFn struct {
	sorts          []ast.ASTHandle
	tCols1, tCols2 []int
	rCols1, rCols2 []int
	// trTableCols / trRelCols are m_tr_table_joined_cols /
	// m_tr_rel_joined_cols (columns of the result's table / inner relation).
	trTableCols, trRelCols []int
	resTableCols           []bool
	rjoin                  RelationJoinFn
	filterTR               RelationMutatorFn
}

func newFprJoinFn(r1, r2 *finiteProductRelation, cols1, cols2 []int) *fprJoinFn {
	j := &fprJoinFn{sorts: joinSorts(r1.sorts, r2.sorts)}
	secondTableOfs := len(r1.table2sig)
	secondInnerOfs := len(r1.other2sig)
	for i := range cols1 {
		t1, t2 := r1.isTableColumn(cols1[i]), r2.isTableColumn(cols2[i])
		switch {
		case !t1 && !t2:
			j.rCols1 = append(j.rCols1, r1.sig2other[cols1[i]])
			j.rCols2 = append(j.rCols2, r2.sig2other[cols2[i]])
		case t1 && t2:
			j.tCols1 = append(j.tCols1, r1.sig2table[cols1[i]])
			j.tCols2 = append(j.tCols2, r2.sig2table[cols2[i]])
		case !t1 && t2:
			j.trRelCols = append(j.trRelCols, r1.sig2other[cols1[i]])
			j.trTableCols = append(j.trTableCols, secondTableOfs+r2.sig2table[cols2[i]])
		default:
			j.trTableCols = append(j.trTableCols, r1.sig2table[cols1[i]])
			j.trRelCols = append(j.trRelCols, secondInnerOfs+r2.sig2other[cols2[i]])
		}
	}
	for i := range r1.sorts {
		j.resTableCols = append(j.resTableCols, r1.isTableColumn(i))
	}
	for i := range r2.sorts {
		j.resTableCols = append(j.resTableCols, r2.isTableColumn(i))
	}
	return j
}

// doRjoin is do_rjoin.
func (j *fprJoinFn) doRjoin(r1, r2 RelationBase) (RelationBase, error) {
	if j.rjoin == nil {
		fn, err := r1.RelationPlugin().Manager().MkRelationJoinFn(r1, r2, j.rCols1, j.rCols2, false)
		if err != nil {
			return nil, err
		}
		if fn == nil {
			return nil, ErrInstruction
		}
		j.rjoin = fn
	}
	return j.rjoin(r1, r2)
}

func (j *fprJoinFn) apply(rb1, rb2 RelationBase) (_ RelationBase, resultErr error) {
	r1 := rb1.(*finiteProductRelation)
	r2 := rb2.(*finiteProductRelation)
	plugin := r1.plugin
	tjoined, err := finiteTableJoin(r1.table, r2.table, j.tCols1, j.tCols2)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, tjoined.Close()) }()
	var joined []RelationBase
	keepJoined := false
	defer func() {
		if !keepJoined {
			resultErr = errors.Join(resultErr, closeRelationBases(joined))
		}
	}()
	// join_maker over rmgr.mk_map_fn(*tjoined).
	if err = fprMap(tjoined, func(fn []Element) (bool, error) {
		or1, err := r1.getInnerRel(int(fn[0]))
		if err != nil {
			return false, err
		}
		or2, err := r2.getInnerRel(int(fn[1]))
		if err != nil {
			return false, err
		}
		n := len(joined)
		res, err := j.doRjoin(or1, or2)
		if err != nil {
			return false, err
		}
		joined = append(joined, res)
		fn[0] = Element(n)
		return true, nil
	}); err != nil {
		return nil, err
	}
	sig, err := tjoined.Signature()
	if err != nil {
		return nil, err
	}
	// remove the second functional column from tjoined
	resTable, err := finiteTableProject(tjoined, []int{sig.Len() - 1})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, resTable.Close()) }()
	oplugin := r1.otherPlugin
	if len(joined) != 0 {
		oplugin = joined[len(joined)-1].RelationPlugin()
	}
	res, err := newFiniteProductRelation(plugin, j.sorts, j.resTableCols, oplugin, nullRelationKind)
	if err != nil {
		return nil, err
	}
	keepJoined = true
	if err = res.init(resTable, joined, true); err != nil {
		return nil, errors.Join(err, res.Close())
	}
	if len(j.trTableCols) != 0 {
		// There were some shared variables between the table and the
		// relation part. We enforce those equalities here.
		if j.filterTR == nil {
			j.filterTR = plugin.mkFilterIdenticalPairs(res, j.trTableCols, j.trRelCols)
		}
		if err = j.filterTR(res); err != nil {
			return nil, errors.Join(err, res.Close())
		}
	}
	return res, nil
}

// fprMap is relation_manager::mk_map_fn: default_table_map_fn. The mapper
// gets the row's functional columns; a false result drops the row.
func fprMap(t FiniteTable, mapper func(fn []Element) (bool, error)) (resultErr error) {
	sparse, ok := t.(*SparseTable)
	if !ok || sparse.functional == 0 {
		return ErrTable
	}
	first := sparse.signature.Len() - sparse.functional
	aux, err := NewSparseTable(sparse.signature, sparse.functional)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, aux.Close()) }()
	facts, err := sparse.Facts()
	if err != nil {
		return err
	}
	for _, f := range facts {
		row := append(Fact(nil), f...)
		keep, err := mapper(row[first:])
		if err != nil {
			return err
		}
		if keep {
			if _, err = aux.Add(row); err != nil {
				return err
			}
		}
	}
	if err = sparse.Reset(); err != nil {
		return err
	}
	return sparse.Union(aux, nil)
}

// fprTableUnion is relation_manager::mk_union_fn(tgt, src, delta) on
// tables: the sparse plugin's union_fn when the signatures (functional
// marks included) agree, else default_table_union_fn.
func fprTableUnion(tgt, src, delta FiniteTable) error {
	a, ok1 := tgt.(*SparseTable)
	b, ok2 := src.(*SparseTable)
	if ok1 && ok2 && sparseSameFunctionalSignature(a, b) {
		if delta == nil {
			return a.Union(b, nil)
		}
		if d, ok := delta.(*SparseTable); ok && sparseSameFunctionalSignature(a, d) {
			return a.Union(b, d)
		}
	}
	facts, err := src.Facts()
	if err != nil {
		return err
	}
	for _, f := range facts {
		row := append(Fact(nil), f...)
		if delta == nil {
			if _, err = tgt.Add(row); err != nil {
				return err
			}
			continue
		}
		found, err := tgt.Contains(row)
		if err != nil {
			return err
		}
		if found {
			continue
		}
		if _, err = tgt.Add(row); err != nil {
			return err
		}
		if _, err = delta.Add(row); err != nil {
			return err
		}
	}
	return nil
}

// fprProjectWithReduce is relation_manager::mk_project_with_reduce_fn
// (default_table_project_with_reduce_fn).
func fprProjectWithReduce(t FiniteTable, removed []int, reducer SparseFunctionalRowReducer) (*SparseTable, error) {
	sparse, ok := t.(*SparseTable)
	if !ok {
		return nil, ErrTable
	}
	return sparse.ProjectWithReduce(removed, reducer)
}

// fprPermutationRename is relation_manager::mk_permutation_rename_fn on a
// table: default_table_permutation_rename_fn (a rename per cycle; the
// identity is a clone).
func fprPermutationRename(t FiniteTable, permutation []int) (FiniteTable, error) {
	perm := make([]uint32, len(permutation))
	for i, v := range permutation {
		perm[i] = uint32(v)
	}
	var res FiniteTable = t
	owned := false
	for {
		cycle32, found := removePermutationCycle(perm)
		if !found {
			break
		}
		cycle := make([]int, len(cycle32))
		for i, c := range cycle32 {
			cycle[i] = int(c)
		}
		next, err := finiteTableRename(res, cycle)
		if owned {
			err = errors.Join(err, res.Close())
		}
		if err != nil {
			if next != nil {
				err = errors.Join(err, next.Close())
			}
			return nil, err
		}
		res, owned = next, true
	}
	if !owned {
		return t.CloneFinite()
	}
	return res, nil
}

// MkProjectFn is mk_project_fn (project_fn).
func (p *FiniteProductRelationPlugin) MkProjectFn(rb RelationBase, removed []int) (RelationTransformerFn, error) {
	r, ok := p.get(rb)
	if !ok {
		return nil, nil
	}
	pr := &fprProjectFn{sorts: projectSorts(r.sorts, removed)}
	for _, col := range removed {
		if r.isTableColumn(col) {
			pr.removedTableCols = append(pr.removedTableCols, r.sig2table[col])
		} else {
			pr.removedRelCols = append(pr.removedRelCols, r.sig2other[col])
		}
	}
	next := 0
	for i := range r.sorts {
		if next < len(removed) && removed[next] == i {
			next++
			continue
		}
		pr.resTableCols = append(pr.resTableCols, r.isTableColumn(i))
	}
	return pr.apply, nil
}

// fprProjectFn is finite_product_relation_plugin::project_fn.
type fprProjectFn struct {
	sorts            []ast.ASTHandle
	removedTableCols []int
	removedRelCols   []int
	relProjector     RelationTransformerFn
	innerRelUnion    RelationUnionFn
	resTableCols     []bool
}

func (pr *fprProjectFn) apply(rb RelationBase) (_ RelationBase, resultErr error) {
	r := rb.(*finiteProductRelation)
	rmgr := r.plugin.manager
	if err := r.garbageCollect(false); err != nil {
		return nil, err
	}
	resRelations := make([]RelationBase, len(r.others))
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, closeRelationBases(resRelations))
		}
	}()
	for i, orig := range r.others {
		if orig == nil {
			continue
		}
		c, err := orig.CloneRelation()
		if err != nil {
			return nil, err
		}
		resRelations[i] = c
	}
	var resTable FiniteTable = r.table
	shared := true
	if len(pr.removedTableCols) != 0 {
		shared = false
		var reduceErr error
		projected, err := fprProjectWithReduce(r.table, pr.removedTableCols, func(funcColumns, merged []Element) {
			// project_reducer::operator()
			if reduceErr != nil {
				return
			}
			tgt, err := resRelations[int(funcColumns[0])].CloneRelation()
			if err != nil {
				reduceErr = err
				return
			}
			src := resRelations[int(merged[0])]
			if pr.innerRelUnion == nil {
				fn, err := tgt.RelationPlugin().Manager().MkRelationUnionFn(tgt, src, nil)
				if err == nil && fn == nil {
					err = ErrInstruction
				}
				if err != nil {
					reduceErr = errors.Join(err, tgt.Close())
					return
				}
				pr.innerRelUnion = fn
			}
			if err = pr.innerRelUnion(tgt, src, nil); err != nil {
				reduceErr = errors.Join(err, tgt.Close())
				return
			}
			newIdx := len(resRelations)
			resRelations = append(resRelations, tgt)
			funcColumns[0] = Element(newIdx)
		})
		if err == nil {
			err = reduceErr
		}
		if err != nil {
			if projected != nil {
				err = errors.Join(err, projected.Close())
			}
			return nil, err
		}
		resTable = projected
		defer func() { resultErr = errors.Join(resultErr, projected.Close()) }()
	}
	var resOPlugin RelationPlugin
	if len(pr.removedRelCols) != 0 {
		for i := range resRelations {
			inner := resRelations[i]
			if inner == nil {
				continue
			}
			if pr.relProjector == nil {
				fn, err := rmgr.MkRelationProjectFn(inner, pr.removedRelCols)
				if err == nil && fn == nil {
					err = ErrInstruction
				}
				if err != nil {
					return nil, err
				}
				pr.relProjector = fn
			}
			projected, err := pr.relProjector(inner)
			if err != nil {
				return nil, err
			}
			resRelations[i] = projected
			if err = inner.Close(); err != nil {
				return nil, err
			}
			if resOPlugin == nil {
				resOPlugin = projected.RelationPlugin()
			}
		}
	}
	if resOPlugin == nil {
		resOPlugin = r.otherPlugin
	}
	res, err := newFiniteProductRelation(r.plugin, pr.sorts, pr.resTableCols, resOPlugin, nullRelationKind)
	if err != nil {
		return nil, err
	}
	keep = true
	if err = res.init(resTable, resRelations, false); err != nil {
		return nil, errors.Join(err, res.Close())
	}
	_ = shared
	return res, nil
}

// MkRenameFn is mk_rename_fn (rename_fn).
func (p *FiniteProductRelationPlugin) MkRenameFn(rb RelationBase, cycle []int) (RelationTransformerFn, error) {
	r, ok := p.get(rb)
	if !ok {
		return nil, nil
	}
	rn := &fprRenameFn{sorts: renameSorts(r.sorts, cycle)}
	n := len(r.sorts)
	permutation := make([]int, n)
	for i := range permutation {
		permutation[i] = i
	}
	if len(cycle) >= 2 {
		first := permutation[cycle[0]]
		for i := 1; i < len(cycle); i++ {
			permutation[cycle[i-1]] = permutation[cycle[i]]
		}
		permutation[cycle[len(cycle)-1]] = first
	}
	for newI := 0; newI < n; newI++ {
		rn.resTableCols = append(rn.resTableCols, r.isTableColumn(permutation[newI]))
	}
	tablePermutation, tableIdentity := collectSubPermutation(permutation, r.sig2table)
	tablePermutation = append(tablePermutation, len(tablePermutation))
	rn.relPermutation, rn.relIdentity = collectSubPermutation(permutation, r.sig2other)
	if !tableIdentity {
		rn.tablePermutation = tablePermutation
	}
	return rn.apply, nil
}

// collectSubPermutation is collect_sub_permutation.
func collectSubPermutation(permutation, translation []int) ([]int, bool) {
	var res []int
	identity := true
	for _, idx := range permutation {
		if sel := translation[idx]; sel != fprNoIndex {
			if len(res) != 0 && sel != res[len(res)-1]+1 {
				identity = false
			}
			res = append(res, sel)
		}
	}
	return res, identity
}

// fprRenameFn is finite_product_relation_plugin::rename_fn.
type fprRenameFn struct {
	sorts            []ast.ASTHandle
	tablePermutation []int // m_table_renamer when not nil
	relRenamer       RelationTransformerFn
	relIdentity      bool
	relPermutation   []int
	resTableCols     []bool
}

func (rn *fprRenameFn) apply(rb RelationBase) (_ RelationBase, resultErr error) {
	r := rb.(*finiteProductRelation)
	if err := r.garbageCollect(false); err != nil {
		return nil, err
	}
	resRelations := make([]RelationBase, len(r.others))
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, closeRelationBases(resRelations))
		}
	}()
	for i, orig := range r.others {
		if orig == nil {
			continue
		}
		c, err := orig.CloneRelation()
		if err != nil {
			return nil, err
		}
		resRelations[i] = c
	}
	if !rn.relIdentity {
		for i := range resRelations {
			inner := resRelations[i]
			if inner == nil {
				continue
			}
			if rn.relRenamer == nil {
				perm := make([]uint32, len(rn.relPermutation))
				for k, v := range rn.relPermutation {
					perm[k] = uint32(v)
				}
				fn, err := r.plugin.manager.MkRelationPermutationRenameFn(inner, perm)
				if err == nil && fn == nil {
					err = ErrInstruction
				}
				if err != nil {
					return nil, err
				}
				rn.relRenamer = fn
			}
			renamed, err := rn.relRenamer(inner)
			if err != nil {
				return nil, err
			}
			resRelations[i] = renamed
			if renamed != inner {
				if err = inner.Close(); err != nil {
					return nil, err
				}
			}
		}
	}
	var resTable FiniteTable = r.table
	if rn.tablePermutation != nil {
		t, err := fprPermutationRename(r.table, rn.tablePermutation)
		if err != nil {
			return nil, err
		}
		resTable = t
		defer func() { resultErr = errors.Join(resultErr, t.Close()) }()
	}
	res, err := newFiniteProductRelation(r.plugin, rn.sorts, rn.resTableCols, r.otherPlugin, nullRelationKind)
	if err != nil {
		return nil, err
	}
	keep = true
	if err = res.init(resTable, resRelations, false); err != nil {
		return nil, errors.Join(err, res.Close())
	}
	return res, nil
}

// MkUnionFn is mk_union_fn: union_fn, or converting_union_fn into a
// relation of another plugin when the source can be a table_relation.
func (p *FiniteProductRelationPlugin) MkUnionFn(tgtb, srcb, deltab RelationBase) (RelationUnionFn, error) {
	src, ok := p.get(srcb)
	if !ok {
		return nil, nil
	}
	_, tgtOK := p.get(tgtb)
	deltaOK := true
	if deltab != nil {
		_, deltaOK = p.get(deltab)
	}
	if !tgtOK || !deltaOK {
		if p.canConvertToTableRelation(src) {
			return p.convertingUnion(), nil
		}
		return nil, nil
	}
	u := &fprUnionFn{useDelta: deltab != nil}
	return u.apply, nil
}

// convertingUnion is converting_union_fn.
func (p *FiniteProductRelationPlugin) convertingUnion() RelationUnionFn {
	var trUnion RelationUnionFn
	return func(tgtb, srcb, deltab RelationBase) (resultErr error) {
		src, ok := p.get(srcb)
		if !ok {
			return ErrInstruction
		}
		trSrc, err := p.toTableRelation(src)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, trSrc.Close()) }()
		if trUnion == nil {
			fn, err := p.manager.MkRelationUnionFn(tgtb, trSrc, deltab)
			if err == nil && fn == nil {
				err = ErrInstruction
			}
			if err != nil {
				return err
			}
			trUnion = fn
		}
		return trUnion(tgtb, trSrc, deltab)
	}
}

// fprUnionFn is finite_product_relation_plugin::union_fn.
type fprUnionFn struct {
	useDelta  bool
	dataCols  []int
	commonSet bool
	relUnion  RelationUnionFn
	// deltaMergingUnion is m_delta_merging_union.
	deltaMergingUnion RelationUnionFn
}

// innerRelUnionOp is get_inner_rel_union_op(r).
func (u *fprUnionFn) innerRelUnionOp(r RelationBase) (RelationUnionFn, error) {
	if u.relUnion == nil {
		var delta RelationBase
		if u.useDelta {
			delta = r
		}
		fn, err := r.RelationPlugin().Manager().MkRelationUnionFn(r, r, delta)
		if err == nil && fn == nil {
			err = ErrInstruction
		}
		if err != nil {
			return nil, err
		}
		u.relUnion = fn
	}
	return u.relUnion, nil
}

func (u *fprUnionFn) apply(tgtb, srcb, deltab RelationBase) (resultErr error) {
	tgt := tgtb.(*finiteProductRelation)
	src0 := srcb.(*finiteProductRelation)
	var delta *finiteProductRelation
	if deltab != nil {
		delta = deltab.(*finiteProductRelation)
	}
	src := src0
	if !intsEqual(tgt.table2sig, src0.table2sig) || (delta != nil && !intsEqual(tgt.table2sig, delta.table2sig)) {
		c, err := src0.clone()
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, c.Close()) }()
		rels := []*finiteProductRelation{c, tgt}
		if delta != nil {
			rels = append(rels, delta)
		}
		ok, err := tryUnifySpecifications(rels)
		if err != nil {
			return err
		}
		if !ok {
			return z3exception.NewDefaultException("finite_product_relation union: cannot convert relations to common specification")
		}
		src = c
	}
	if !u.commonSet {
		u.commonSet = true
		dataColsCnt := len(tgt.tableSig) - 1
		for i := 0; i < dataColsCnt; i++ {
			u.dataCols = append(u.dataCols, i)
		}
	}
	// m_common_join: result contains (data columns), tgt_rel_idx, src_rel_idx
	tableOverlap, err := finiteTableJoinProject(tgt.table, src.table, u.dataCols, u.dataCols, u.dataCols)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, tableOverlap.Close()) }()
	var deltaIndexes *SparseTable
	var deltaRels []RelationBase
	keepDeltaRels := false
	defer func() {
		if !keepDeltaRels {
			resultErr = errors.Join(resultErr, closeRelationBases(deltaRels))
		}
	}()
	if u.useDelta {
		deltaIndexes, err = NewSparseTable(Signature{domains: []uint64{fprRelIdxSort, fprRelIdxSort}}, 1)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, deltaIndexes.Close()) }()
	}
	// union_mapper
	if err = fprMap(tableOverlap, func(fn []Element) (bool, error) {
		otgtOrig, err := tgt.getInnerRel(int(fn[0]))
		if err != nil {
			return false, err
		}
		osrc, err := src.getInnerRel(int(fn[1]))
		if err != nil {
			return false, err
		}
		otgt, err := otgtOrig.CloneRelation()
		if err != nil {
			return false, err
		}
		newTgtIdx := tgt.getNextRelIdx()
		tgt.setInnerRel(newTgtIdx, otgt)
		op, err := u.innerRelUnionOp(otgt)
		if err != nil {
			return false, err
		}
		if deltaIndexes != nil {
			odelta, err := mkEmptyLike(otgt)
			if err != nil {
				return false, err
			}
			if err = op(otgt, osrc, odelta); err != nil {
				return false, errors.Join(err, odelta.Close())
			}
			deltaIdx := len(deltaRels)
			deltaRels = append(deltaRels, odelta)
			if _, err = deltaIndexes.Add(Fact{Element(newTgtIdx), Element(deltaIdx)}); err != nil {
				return false, err
			}
		} else if err = op(otgt, osrc, nil); err != nil {
			return false, err
		}
		fn[0] = Element(newTgtIdx)
		return true, nil
	}); err != nil {
		return err
	}
	overlapSig, err := tableOverlap.Signature()
	if err != nil {
		return err
	}
	regularOverlap, err := finiteTableProject(tableOverlap, []int{overlapSig.Len() - 1})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, regularOverlap.Close()) }()
	// in tgt keep only the rows that are in tgt only
	if err = finiteTableFilterNegation(tgt.table, regularOverlap, u.dataCols, u.dataCols); err != nil {
		return err
	}
	// add rows in which tgt and src overlapped
	if err = fprTableUnion(tgt.table, regularOverlap, nil); err != nil {
		return err
	}
	srcOnly, err := src.table.CloneFinite()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, srcOnly.Close()) }()
	if err = finiteTableFilterNegation(srcOnly, regularOverlap, u.dataCols, u.dataCols); err != nil {
		return err
	}
	var srcOnly2 FiniteTable
	if u.useDelta {
		if srcOnly2, err = srcOnly.CloneFinite(); err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, srcOnly2.Close()) }()
	}
	// src_copying_mapper
	if err = fprMap(srcOnly, srcCopyingMapper(tgt, src)); err != nil {
		return err
	}
	// add rows that were only in src
	if err = fprTableUnion(tgt.table, srcOnly, nil); err != nil {
		return err
	}
	if u.useDelta {
		extending, err := delta.isNonEmpty()
		if err != nil {
			return err
		}
		cdelta := delta
		if extending {
			if cdelta, err = delta.plugin.mkEmptyLike(delta); err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, cdelta.Close()) }()
		}
		tableFnCol := overlapSig.Len() - 2
		overlapDeltaTable, err := finiteTableJoinProject(regularOverlap, deltaIndexes, []int{tableFnCol}, []int{0}, []int{tableFnCol, tableFnCol + 1})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, overlapDeltaTable.Close()) }()
		keepDeltaRels = true
		if err = cdelta.init(overlapDeltaTable, deltaRels, true); err != nil {
			return err
		}
		if err = fprMap(srcOnly2, srcCopyingMapper(cdelta, src)); err != nil {
			return err
		}
		if err = fprTableUnion(cdelta.table, srcOnly2, nil); err != nil {
			return err
		}
		if extending {
			if u.deltaMergingUnion == nil {
				fn, err := delta.plugin.manager.MkRelationUnionFn(delta, cdelta, nil)
				if err == nil && fn == nil {
					err = ErrInstruction
				}
				if err != nil {
					return err
				}
				u.deltaMergingUnion = fn
			}
			if err = u.deltaMergingUnion(delta, cdelta, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// srcCopyingMapper is union_fn::src_copying_mapper.
func srcCopyingMapper(tgt, src *finiteProductRelation) func([]Element) (bool, error) {
	return func(fn []Element) (bool, error) {
		osrc, err := src.getInnerRel(int(fn[0]))
		if err != nil {
			return false, err
		}
		newTgtIdx := tgt.getNextRelIdx()
		c, err := osrc.CloneRelation()
		if err != nil {
			return false, err
		}
		tgt.setInnerRel(newTgtIdx, c)
		fn[0] = Element(newTgtIdx)
		return true, nil
	}
}

// MkFilterIdenticalFn is mk_filter_identical_fn (filter_identical_fn).
func (p *FiniteProductRelationPlugin) MkFilterIdenticalFn(rb RelationBase, cols []int) (RelationMutatorFn, error) {
	r, ok := p.get(rb)
	if !ok {
		return nil, nil
	}
	var tableCols, relCols []int
	for _, col := range cols {
		if r.isTableColumn(col) {
			tableCols = append(tableCols, r.sig2table[col])
		} else {
			relCols = append(relCols, r.sig2other[col])
		}
	}
	var trFilter RelationMutatorFn
	if len(tableCols) != 0 && len(relCols) != 0 {
		trFilter = p.mkFilterIdenticalPairs(r, []int{tableCols[0]}, []int{relCols[0]})
	}
	var relFilter RelationMutatorFn
	return func(rb RelationBase) error {
		r := rb.(*finiteProductRelation)
		if len(tableCols) > 1 {
			if err := finiteTableFilterIdentical(r.table, tableCols); err != nil {
				return err
			}
		}
		if len(relCols) > 1 {
			if err := r.garbageCollect(true); err != nil {
				return err
			}
			for _, inner := range r.others {
				if inner == nil {
					continue
				}
				if relFilter == nil {
					fn, err := inner.RelationPlugin().Manager().MkRelationFilterIdenticalFn(inner, relCols)
					if err == nil && fn == nil {
						err = ErrInstruction
					}
					if err != nil {
						return err
					}
					relFilter = fn
				}
				if err := relFilter(inner); err != nil {
					return err
				}
			}
		}
		if len(tableCols) != 0 && len(relCols) != 0 {
			return trFilter(r)
		}
		return nil
	}, nil
}

// MkFilterEqualFn is mk_filter_equal_fn (filter_equal_fn).
func (p *FiniteProductRelationPlugin) MkFilterEqualFn(rb RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error) {
	r, ok := p.get(rb)
	if !ok {
		return nil, nil
	}
	ref, err := p.manager.manager.NewRef(value)
	if err != nil {
		return nil, err
	}
	p.manager.fnRoots = append(p.manager.fnRoots, ref)
	if r.isTableColumn(col) {
		tval, err := p.manager.EncodeValue(r.sorts[col], value)
		if err != nil {
			return nil, err
		}
		tcol := r.sig2table[col]
		return func(rb RelationBase) error {
			return finiteTableFilterEqual(rb.(*finiteProductRelation).table, tval, tcol)
		}, nil
	}
	var relFilter RelationMutatorFn
	return func(rb RelationBase) error {
		r := rb.(*finiteProductRelation)
		if err := r.garbageCollect(false); err != nil {
			return err
		}
		for _, inner := range r.others {
			if inner == nil {
				continue
			}
			if relFilter == nil {
				fn, err := r.plugin.manager.MkRelationFilterEqualFn(inner, value, r.sig2other[col])
				if err == nil && fn == nil {
					err = ErrInstruction
				}
				if err != nil {
					return err
				}
				relFilter = fn
			}
			if err := relFilter(inner); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

// renamingArgs is get_renaming_args(map, orig_sig, renaming_arg): the
// variables (map[i], orig_sig[i]) at position sz-1-i, null elsewhere.
func (p *FiniteProductRelationPlugin) renamingArgs(mapping []int, sorts []ast.ASTHandle) ([]ast.ASTRef, error) {
	m := p.manager.manager
	sz := len(mapping)
	res := make([]ast.ASTRef, sz)
	for i := 0; i < sz; i++ {
		if mapping[i] == fprNoIndex {
			continue
		}
		v, err := m.MkVar(uint32(mapping[i]), sorts[i])
		if err != nil {
			return res, err
		}
		if res[sz-1-i], err = m.NewRef(v); err != nil {
			return res, err
		}
	}
	return res, nil
}

func refHandles(m *ast.ASTManager, refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	hs := make([]ast.ASTHandle, len(refs))
	for i, r := range refs {
		if r == (ast.ASTRef{}) {
			continue
		}
		h, err := m.RefGet(r)
		if err != nil {
			return nil, err
		}
		hs[i] = h
	}
	return hs, nil
}

// substitute is m_subst(cond, args) with the context var_subst; the caller
// owns the result.
func (p *FiniteProductRelationPlugin) substitute(cond ast.ASTHandle, args []ast.ASTRef) (ast.ASTRef, error) {
	m := p.manager.manager
	hs, err := refHandles(m, args)
	if err != nil {
		return ast.ASTRef{}, err
	}
	res, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = p.manager.varSubst.Substitute(cond, hs, res); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&res))
	}
	return res, nil
}

// MkFilterInterpretedFn is mk_filter_interpreted_fn (filter_interpreted_fn).
func (p *FiniteProductRelationPlugin) MkFilterInterpretedFn(rb RelationBase, condition ast.ASTHandle) (_ RelationMutatorFn, resultErr error) {
	r, ok := p.get(rb)
	if !ok {
		return nil, nil
	}
	m := p.manager.manager
	cond, err := m.NewRef(condition)
	if err != nil {
		return nil, err
	}
	f := &fprFilterInterpretedFn{plugin: p, cond: condition, condRef: cond}
	// The functor owns its members; it is destroyed with its owner (the
	// instruction that cached it, else the relation manager).
	p.manager.fnClosers = append(p.manager.fnClosers, f.close)
	condColumns, err := collectVarIndexes(m, condition)
	if err != nil {
		return nil, err
	}
	f.tableLocalCondColumns = make(map[int]bool)
	for i := range r.sorts {
		if !condColumns[i] {
			continue
		}
		if r.isTableColumn(i) {
			f.tableCondColumns = append(f.tableCondColumns, i)
			f.tableLocalCondColumns[r.sig2table[i]] = true
		} else {
			f.relCondColumns = append(f.relCondColumns, i)
		}
	}
	if len(f.relCondColumns) == 0 {
		renaming, err := p.renamingArgs(r.sig2table, r.sorts)
		defer func() { resultErr = errors.Join(resultErr, releaseNullableRefs(m, renaming)) }()
		if err != nil {
			return nil, err
		}
		tableCond, err := p.substitute(condition, renaming)
		if err != nil {
			return nil, err
		}
		// m_table_filter keeps the renamed condition.
		f.tableCondRef = tableCond
		if f.tableCond, err = m.RefGet(tableCond); err != nil {
			return nil, err
		}
		f.tableSorts = r.tableSorts()
	} else {
		// m_renaming_for_inner_rel is the functor's; operator() replaces
		// its table entries.
		if f.renamingForInner, err = p.renamingArgs(r.sig2other, r.sorts); err != nil {
			return nil, err
		}
		if len(f.tableCondColumns) != 0 {
			tableDataColCnt := len(r.tableSig) - 1
			var removed []int
			for i := 0; i < tableDataColCnt; i++ {
				if f.tableLocalCondColumns[i] {
					f.globalOrigins = append(f.globalOrigins, r.table2sig[i])
				} else {
					removed = append(removed, i)
				}
			}
			f.projectRemoved = removed
		}
	}
	return f.apply, nil
}

// collectVarIndexes is rule_manager::collect_vars(cond): the free
// variable indexes of the condition.
func collectVarIndexes(m *ast.ASTManager, cond ast.ASTHandle) (map[int]bool, error) {
	used, err := ast.NewASTUsedVars(m)
	if err != nil {
		return nil, err
	}
	if err = used.Process(cond); err != nil {
		return nil, err
	}
	res := make(map[int]bool)
	n := used.MaxFoundVarIndexPlusOne()
	for i := uint32(0); i < n; i++ {
		s, err := used.Get(i)
		if err != nil {
			return nil, err
		}
		if !s.IsNull() {
			res[int(i)] = true
		}
	}
	return res, nil
}

// fprFilterInterpretedFn is finite_product_relation_plugin::filter_interpreted_fn.
type fprFilterInterpretedFn struct {
	plugin                *FiniteProductRelationPlugin
	cond                  ast.ASTHandle
	tableCond             ast.ASTHandle
	tableSorts            []ast.ASTHandle
	relFilter             RelationMutatorFn
	tableCondColumns      []int
	relCondColumns        []int
	tableLocalCondColumns map[int]bool
	projectRemoved        []int
	globalOrigins         []int
	renamingForInner      []ast.ASTRef
	assembling            bool
	condRef               ast.ASTRef
	tableCondRef          ast.ASTRef
	// relFilterResources are m_rel_filter's state, made at the first
	// operator() call.
	relFilterResources *fnResources
	closed             bool
}

// close is ~filter_interpreted_fn: the members in reverse declaration
// order: m_renaming_for_inner_rel (released front to back), m_cond,
// m_rel_filter, then m_table_filter (which keeps the table condition).
func (f *fprFilterInterpretedFn) close() (result error) {
	if f.closed {
		return nil
	}
	f.closed = true
	m := f.plugin.manager.manager
	for i := range f.renamingForInner {
		if f.renamingForInner[i] != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&f.renamingForInner[i]))
		}
	}
	f.renamingForInner = nil
	if f.condRef != (ast.ASTRef{}) {
		result = errors.Join(result, m.DelRef(&f.condRef))
	}
	if f.relFilterResources != nil {
		result = errors.Join(result, f.relFilterResources.release())
		f.relFilterResources = nil
	}
	if f.tableCondRef != (ast.ASTRef{}) {
		result = errors.Join(result, m.DelRef(&f.tableCondRef))
	}
	return result
}

func (f *fprFilterInterpretedFn) apply(rb RelationBase) (resultErr error) {
	r := rb.(*finiteProductRelation)
	p := f.plugin
	rmgr := p.manager
	m := rmgr.manager
	if len(f.relCondColumns) == 0 {
		return rmgr.tableFilterInterpreted(r.table, f.tableSorts, f.tableCond)
	}
	if len(f.tableCondColumns) == 0 {
		if err := r.garbageCollect(false); err != nil {
			return err
		}
		for _, inner := range r.others {
			if inner == nil {
				continue
			}
			if f.relFilter == nil {
				innerCond, err := p.substitute(f.cond, f.renamingForInner)
				if err != nil {
					return err
				}
				h, err := m.RefGet(innerCond)
				if err == nil {
					var fn RelationMutatorFn
					mark := rmgr.fnMark()
					fn, err = rmgr.MkRelationFilterInterpretedFn(inner, h)
					f.relFilterResources = rmgr.takeFnResources(mark)
					if err == nil && fn == nil {
						// The fixed fork's throw (FORK FIX, see
						// FORK_BUGFIXES.md); 4.7.1 called the null functor.
						err = unsupportedRelationOperationOne("filter_interpreted", inner.RelationPlugin().Name())
					}
					f.relFilter = fn
				}
				err = errors.Join(err, m.DelRef(&innerCond))
				if err != nil {
					return err
				}
			}
			if err := f.relFilter(inner); err != nil {
				return err
			}
		}
		return nil
	}
	// get table without the data columns on which we don't enforce
	// identities
	var tproj FiniteTable = r.table
	if len(f.projectRemoved) != 0 {
		t, err := finiteTableProject(r.table, f.projectRemoved)
		if err != nil {
			return err
		}
		tproj = t
		defer func() { resultErr = errors.Join(resultErr, t.Close()) }()
	}
	tprojSig, err := tproj.Signature()
	if err != nil {
		return err
	}
	projectedDataCols := tprojSig.Len() - 1
	filteredDomains := append(append([]uint64(nil), tprojSig.domains...), fprRelIdxSort)
	filtered, err := NewSparseTable(Signature{domains: filteredDomains}, 1)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, filtered.Close()) }()
	var newRels []RelationBase
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, closeRelationBases(newRels))
		}
	}()
	renamingOfs := len(f.renamingForInner) - 1
	facts, err := tproj.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		row := append(Fact(nil), fact...)
		oldRel, err := r.getInnerRel(int(row[len(row)-1]))
		if err != nil {
			return err
		}
		// put the table values into the substitution
		for i := 0; i < projectedDataCols; i++ {
			orig := f.globalOrigins[i]
			el, err := rmgr.DecodeValue(r.sorts[orig], row[i])
			if err != nil {
				return err
			}
			old := f.renamingForInner[renamingOfs-orig]
			f.renamingForInner[renamingOfs-orig] = el
			if old != (ast.ASTRef{}) {
				if err = m.DelRef(&old); err != nil {
					return err
				}
			}
		}
		innerCond, err := p.substitute(f.cond, f.renamingForInner)
		if err != nil {
			return err
		}
		newRel, err := oldRel.CloneRelation()
		if err != nil {
			return errors.Join(err, m.DelRef(&innerCond))
		}
		h, err := m.RefGet(innerCond)
		if err == nil {
			var filter RelationMutatorFn
			filter, err = rmgr.MkRelationFilterInterpretedFn(newRel, h)
			if err == nil && filter == nil {
				// The condition with the table values substituted in may
				// have no filter in the inner plugin (explanation_relation
				// filters (= var app) only): the fixed fork's throw (FORK
				// FIX, see FORK_BUGFIXES.md); 4.7.1 called the null functor.
				err = unsupportedRelationOperationOne("filter_interpreted", newRel.RelationPlugin().Name())
			}
			if err == nil {
				err = filter(newRel)
			}
		}
		err = errors.Join(err, m.DelRef(&innerCond))
		if err != nil {
			return errors.Join(err, newRel.Close())
		}
		empty, err := newRel.Empty()
		if err != nil {
			return errors.Join(err, newRel.Close())
		}
		if empty {
			if err = newRel.Close(); err != nil {
				return err
			}
			continue
		}
		newIdx := len(newRels)
		newRels = append(newRels, newRel)
		if _, err = filtered.Add(append(row, Element(newIdx))); err != nil {
			return err
		}
	}
	var selected []int
	for i := range r.sorts {
		if f.tableLocalCondColumns[i] {
			selected = append(selected, i)
		}
	}
	newTable, err := mkAssemblerOfFilterResult(r.table, filtered, selected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, newTable.Close()) }()
	if err = r.Reset(); err != nil {
		return err
	}
	keep = true
	return r.init(newTable, newRels, true)
}

// mkAssemblerOfFilterResult is mk_assembler_of_filter_result applied to
// (relation_table, filtered_table).
func mkAssemblerOfFilterResult(relationTable, filteredTable FiniteTable, selected []int) (FiniteTable, error) {
	rsig, err := relationTable.Signature()
	if err != nil {
		return nil, err
	}
	rsz := rsig.Len()
	n := len(selected)
	rCols := append(append([]int(nil), selected...), rsz-1)
	fCols := make([]int, 0, n+1)
	for i := 0; i < n; i++ {
		fCols = append(fCols, i)
	}
	fCols = append(fCols, n)
	// (all relation table columns)(all filtered relation table columns)
	// (unfiltered rel idx non-func from 'filtered')(unfiltered rel idx func
	// from 'rtable')(filtered rel idx)
	var removed []int
	ofs := rsz - 1
	for i := 0; i < n; i++ {
		removed = append(removed, ofs+i)
	}
	idxOfs := ofs + n
	removed = append(removed, idxOfs, idxOfs+1)
	return finiteTableJoinProject(relationTable, filteredTable, rCols, fCols, removed)
}

// MkFilterByNegationFn is mk_filter_by_negation_fn (negation_filter_fn).
func (p *FiniteProductRelationPlugin) MkFilterByNegationFn(rb, negb RelationBase, rCols, negCols []int) (RelationIntersectionFilterFn, error) {
	r, ok1 := p.get(rb)
	neg, ok2 := p.get(negb)
	if !ok1 || !ok2 {
		return nil, nil
	}
	nf := &fprNegationFilterFn{rCols: append([]int(nil), rCols...), negCols: append([]int(nil), negCols...), tableOverlapsOnly: true}
	for i := range rCols {
		if r.isTableColumn(rCols[i]) && neg.isTableColumn(negCols[i]) {
			nf.rSharedTableCols = append(nf.rSharedTableCols, r.sig2table[rCols[i]])
			nf.negSharedTableCols = append(nf.negSharedTableCols, neg.sig2table[negCols[i]])
		} else {
			nf.tableOverlapsOnly = false
		}
	}
	if !nf.tableOverlapsOnly {
		var removed []int
		for i := 0; i < len(neg.sorts); i++ {
			removed = append(removed, len(r.sorts)+i)
		}
		fn, err := p.manager.MkRelationJoinProjectFn(r, neg, nf.rCols, nf.negCols, removed, false)
		if err == nil && fn == nil {
			err = ErrInstruction
		}
		if err != nil {
			return nil, err
		}
		nf.negIntersectionJoin = fn
		tsz := len(r.tableSig)
		for i := 0; i < tsz-1; i++ {
			nf.dataCols = append(nf.dataCols, i)
		}
		for i := 0; i < tsz-1; i++ {
			nf.removedSecondData = append(nf.removedSecondData, tsz-1+i)
		}
	}
	return nf.apply, nil
}

// fprNegationFilterFn is finite_product_relation_plugin::negation_filter_fn.
type fprNegationFilterFn struct {
	rCols, negCols                       []int
	tableOverlapsOnly                    bool
	rSharedTableCols, negSharedTableCols []int
	negIntersectionJoin                  RelationJoinFn
	dataCols                             []int
	removedSecondData                    []int
	innerSubtract                        RelationIntersectionFilterFn
}

func (nf *fprNegationFilterFn) apply(rb, negb RelationBase) (resultErr error) {
	r := rb.(*finiteProductRelation)
	neg := negb.(*finiteProductRelation)
	if nf.tableOverlapsOnly {
		return finiteTableFilterNegation(r.table, neg.table, nf.rSharedTableCols, nf.negSharedTableCols)
	}
	interRel, err := nf.negIntersectionJoin(r, neg)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, interRel.Close()) }()
	intersection, ok := interRel.(*finiteProductRelation)
	if !ok {
		return ErrInstruction
	}
	// (data_columns)(original rel idx)(subtracted rel idx)
	tableOverlap0, err := finiteTableJoinProject(r.table, intersection.table, nf.dataCols, nf.dataCols, nf.removedSecondData)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, tableOverlap0.Close()) }()
	// the rows that don't appear in the table of the intersection are safe
	// to stay
	if err = finiteTableFilterNegation(r.table, intersection.table, nf.dataCols, nf.dataCols); err != nil {
		return err
	}
	overlapSig, err := tableOverlap0.Signature()
	if err != nil {
		return err
	}
	tableOverlap, err := NewSparseTable(overlapSig, 2)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, tableOverlap.Close()) }()
	if err = fprTableUnion(tableOverlap, tableOverlap0, nil); err != nil {
		return err
	}
	// rel_subtractor
	if err = fprMap(tableOverlap, func(fn []Element) (bool, error) {
		orig, err := r.getInnerRel(int(fn[0]))
		if err != nil {
			return false, err
		}
		rInner, err := orig.CloneRelation()
		if err != nil {
			return false, err
		}
		intersInner, err := intersection.getInnerRel(int(fn[1]))
		if err != nil {
			return false, errors.Join(err, rInner.Close())
		}
		if nf.innerSubtract == nil {
			all := make([]int, len(rInner.RelationSignature()))
			for i := range all {
				all[i] = i
			}
			sub, err := r.plugin.manager.MkRelationFilterByNegationFn(rInner, intersInner, all, all)
			if err == nil && sub == nil {
				err = ErrInstruction
			}
			if err != nil {
				return false, errors.Join(err, rInner.Close())
			}
			nf.innerSubtract = sub
		}
		if err = nf.innerSubtract(rInner, intersInner); err != nil {
			return false, errors.Join(err, rInner.Close())
		}
		newNum := r.getNextRelIdx()
		r.setInnerRel(newNum, rInner)
		fn[0] = Element(newNum)
		return true, nil
	}); err != nil {
		return err
	}
	final, err := finiteTableProject(tableOverlap, []int{overlapSig.Len() - 1})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, final.Close()) }()
	return fprTableUnion(r.table, final, nil)
}

// mkFilterIdenticalPairs is mk_filter_identical_pairs (filter_identical_pairs_fn).
func (p *FiniteProductRelationPlugin) mkFilterIdenticalPairs(r *finiteProductRelation, tableCols, relCols []int) RelationMutatorFn {
	pairs := make([][2]int, len(tableCols))
	for i := range tableCols {
		pairs[i] = [2]int{tableCols[i], relCols[i]}
	}
	sortPairedColumnIndices(pairs)
	tcols := make([]int, len(pairs))
	rcols := make([]int, len(pairs))
	for i, pr := range pairs {
		tcols[i], rcols[i] = pr[0], pr[1]
	}
	tsz := len(r.tableSig)
	inTable := make(map[int]bool, len(tcols))
	for _, c := range tcols {
		inTable[c] = true
	}
	var removed []int
	for i := 0; i < tsz-1; i++ {
		if !inTable[i] {
			removed = append(removed, i)
		}
	}
	return func(rb RelationBase) (resultErr error) {
		r := rb.(*finiteProductRelation)
		rmgr := r.plugin.manager
		m := rmgr.manager
		var tproj FiniteTable
		var err error
		if len(removed) != 0 {
			tproj, err = finiteTableProject(r.table, removed)
		} else {
			tproj, err = r.table.CloneFinite()
		}
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, tproj.Close()) }()
		tprojSig, err := tproj.Signature()
		if err != nil {
			return err
		}
		filtered, err := NewSparseTable(Signature{domains: append(append([]uint64(nil), tprojSig.domains...), fprRelIdxSort)}, 1)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, filtered.Close()) }()
		var newRels []RelationBase
		keep := false
		defer func() {
			if !keep {
				resultErr = errors.Join(resultErr, closeRelationBases(newRels))
			}
		}()
		facts, err := tproj.Facts()
		if err != nil {
			return err
		}
		for _, fact := range facts {
			row := append(Fact(nil), fact...)
			oldRel, err := r.getInnerRel(int(row[len(row)-1]))
			if err != nil {
				return err
			}
			newRel, err := oldRel.CloneRelation()
			if err != nil {
				return err
			}
			for i := range tcols {
				el, err := rmgr.DecodeValue(r.otherSig[rcols[i]], row[i])
				if err != nil {
					return errors.Join(err, newRel.Close())
				}
				h, err := m.RefGet(el)
				if err == nil {
					var filter RelationMutatorFn
					filter, err = rmgr.MkRelationFilterEqualFn(newRel, h, rcols[i])
					if err == nil && filter == nil {
						err = ErrInstruction
					}
					if err == nil {
						err = filter(newRel)
					}
				}
				err = errors.Join(err, m.DelRef(&el))
				if err != nil {
					return errors.Join(err, newRel.Close())
				}
			}
			empty, err := newRel.Empty()
			if err != nil {
				return errors.Join(err, newRel.Close())
			}
			if empty {
				if err = newRel.Close(); err != nil {
					return err
				}
				continue
			}
			newIdx := len(newRels)
			newRels = append(newRels, newRel)
			if _, err = filtered.Add(append(row, Element(newIdx))); err != nil {
				return err
			}
		}
		newTable, err := mkAssemblerOfFilterResult(r.table, filtered, tcols)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, newTable.Close()) }()
		if err = r.Reset(); err != nil {
			return err
		}
		keep = true
		return r.init(newTable, newRels, true)
	}
}

// ---------------------------------------------------------------------------
// finite_product_relation

// finiteProductRelation is finite_product_relation.
type finiteProductRelation struct {
	plugin *FiniteProductRelationPlugin
	sorts  []ast.ASTHandle
	kind   int
	// tableSig is m_table_sig (the last column is the functional index
	// column).
	tableSig  []uint64
	table2sig []int
	sig2table []int
	otherSig  []ast.ASTHandle
	other2sig []int
	sig2other []int
	// otherPlugin and otherKind are m_other_plugin and m_other_kind.
	otherPlugin RelationPlugin
	otherKind   int
	table       *SparseTable
	others      []RelationBase
	available   []int
	fullRelIdx  int
	closed      bool
}

// newFiniteProductRelation is the finite_product_relation constructor.
func newFiniteProductRelation(p *FiniteProductRelationPlugin, sorts []ast.ASTHandle, tableColumns []bool, oplugin RelationPlugin, otherKind int) (*finiteProductRelation, error) {
	r := &finiteProductRelation{plugin: p, sorts: append([]ast.ASTHandle(nil), sorts...), otherPlugin: oplugin, otherKind: otherKind, fullRelIdx: fprNoIndex}
	n := len(sorts)
	r.sig2table = make([]int, n)
	r.sig2other = make([]int, n)
	for i := 0; i < n; i++ {
		r.sig2table[i], r.sig2other[i] = fprNoIndex, fprNoIndex
		if tableColumns[i] {
			r.sig2table[i] = len(r.tableSig)
			d, ok := p.relationSortToTable(sorts[i])
			if !ok {
				return nil, fmt.Errorf("%w: finite product table column without a table sort", ErrTable)
			}
			r.tableSig = append(r.tableSig, d)
			r.table2sig = append(r.table2sig, i)
		} else {
			r.sig2other[i] = len(r.otherSig)
			r.otherSig = append(r.otherSig, sorts[i])
			r.other2sig = append(r.other2sig, i)
		}
	}
	r.tableSig = append(r.tableSig, fprRelIdxSort)
	t, err := NewSparseTable(Signature{domains: append([]uint64(nil), r.tableSig...)}, 1)
	if err != nil {
		return nil, err
	}
	r.table = t
	r.kind = p.getRelationKind(r.sorts, tableColumns)
	return r, nil
}

func (r *finiteProductRelation) isTableColumn(col int) bool { return r.sig2table[col] != fprNoIndex }

// tableSorts is the relation sorts of the table's data columns.
func (r *finiteProductRelation) tableSorts() []ast.ASTHandle {
	res := make([]ast.ASTHandle, len(r.table2sig))
	for i, s := range r.table2sig {
		res[i] = r.sorts[s]
	}
	return res
}

// RelationKind is relation_base::get_kind().
func (r *finiteProductRelation) RelationKind() int { return r.kind }

func (r *finiteProductRelation) getInnerRel(idx int) (RelationBase, error) {
	if idx < 0 || idx >= len(r.others) || r.others[idx] == nil {
		return nil, fmt.Errorf("%w: finite product inner relation %d", ErrInstruction, idx)
	}
	return r.others[idx], nil
}

// getNextRelIdx is get_next_rel_idx.
func (r *finiteProductRelation) getNextRelIdx() int {
	if n := len(r.available); n != 0 {
		res := r.available[n-1]
		r.available = r.available[:n-1]
		return res
	}
	res := len(r.others)
	r.others = append(r.others, nil)
	return res
}

func (r *finiteProductRelation) setInnerRel(idx int, inner RelationBase) {
	r.others[idx] = inner
}

// recycleRelIdx is recycle_rel_idx.
func (r *finiteProductRelation) recycleRelIdx(idx int) {
	r.available = append(r.available, idx)
}

// getFullRelIdx is get_full_rel_idx.
func (r *finiteProductRelation) getFullRelIdx() (int, error) {
	if r.fullRelIdx == fprNoIndex {
		r.fullRelIdx = r.getNextRelIdx()
		full, err := r.mkFullInner(ast.ASTHandle{})
		if err != nil {
			return 0, err
		}
		r.others[r.fullRelIdx] = full
	}
	return r.fullRelIdx, nil
}

// init is init(table_vals, others, contiguous): the relation takes the
// inner relations.
func (r *finiteProductRelation) init(tableVals FiniteTable, others []RelationBase, contiguous bool) error {
	if len(r.others) != 0 {
		if err := r.garbageCollect(false); err != nil {
			return err
		}
	}
	r.others = append([]RelationBase(nil), others...)
	r.available = nil
	if err := fprTableUnion(r.table, tableVals, nil); err != nil {
		return err
	}
	if !contiguous {
		for i, o := range r.others {
			if o == nil {
				r.available = append(r.available, i)
			}
		}
	}
	return nil
}

// extractTableFact is extract_table_fact.
func (r *finiteProductRelation) extractTableFact(values []ast.ASTHandle) (Fact, error) {
	tf := make(Fact, 0, len(r.tableSig))
	for _, sigIdx := range r.table2sig {
		el, err := r.plugin.manager.EncodeValue(r.sorts[sigIdx], values[sigIdx])
		if err != nil {
			return nil, err
		}
		tf = append(tf, el)
	}
	return append(tf, 0), nil
}

// extractOtherFact is extract_other_fact.
func (r *finiteProductRelation) extractOtherFact(values []ast.ASTHandle) []ast.ASTHandle {
	of := make([]ast.ASTHandle, len(r.other2sig))
	for i, sigIdx := range r.other2sig {
		of[i] = values[sigIdx]
	}
	return of
}

// mkEmptyInner is mk_empty_inner.
func (r *finiteProductRelation) mkEmptyInner() (RelationBase, error) {
	if r.otherKind == nullRelationKind {
		return r.otherPlugin.MkEmpty(r.otherSig)
	}
	if k, ok := r.otherPlugin.(kindedRelationPlugin); ok {
		return k.MkEmptyKind(r.otherSig, r.otherKind)
	}
	return r.otherPlugin.MkEmpty(r.otherSig)
}

// mkFullInner is mk_full_inner(pred): m_other_plugin.mk_full(pred,
// m_other_sig, m_other_kind).
func (r *finiteProductRelation) mkFullInner(pred ast.ASTHandle) (RelationBase, error) {
	if k, ok := r.otherPlugin.(kindedRelationPlugin); ok {
		return k.MkFullKind(pred, r.otherSig, r.otherKind)
	}
	if r.otherKind == nullRelationKind || r.otherKind == r.plugin.manager.PluginKind(r.otherPlugin) {
		return r.otherPlugin.MkFull(pred, r.otherSig)
	}
	return nil, ErrInstruction
}

// AddFactAST is add_fact.
func (r *finiteProductRelation) AddFactAST(values []ast.ASTHandle) error {
	tf, err := r.extractTableFact(values)
	if err != nil {
		return err
	}
	of := r.extractOtherFact(values)
	newIdx := r.getNextRelIdx()
	tf[len(tf)-1] = Element(newIdx)
	added, err := r.table.SuggestFact(tf)
	if err != nil {
		return err
	}
	var newRel RelationBase
	if added {
		if newRel, err = r.mkEmptyInner(); err != nil {
			return err
		}
	} else {
		old, err := r.getInnerRel(int(tf[len(tf)-1]))
		if err != nil {
			return err
		}
		if newRel, err = old.CloneRelation(); err != nil {
			return err
		}
		tf[len(tf)-1] = Element(newIdx)
		if err = r.table.EnsureFact(tf); err != nil {
			return errors.Join(err, newRel.Close())
		}
	}
	adder, ok := newRel.(RelationAddFactAST)
	if !ok {
		return errors.Join(ErrRelationNotTable, newRel.Close())
	}
	if err = adder.AddFactAST(of); err != nil {
		return errors.Join(err, newRel.Close())
	}
	r.setInnerRel(newIdx, newRel)
	return nil
}

// ContainsFactAST is contains_fact.
func (r *finiteProductRelation) ContainsFactAST(values []ast.ASTHandle) (bool, error) {
	tf, err := r.extractTableFact(values)
	if err != nil {
		return false, err
	}
	found, err := r.table.FetchFact(tf)
	if err != nil || !found {
		return false, err
	}
	other, err := r.getInnerRel(int(tf[len(tf)-1]))
	if err != nil {
		return false, err
	}
	c, ok := other.(interface {
		ContainsFactAST([]ast.ASTHandle) (bool, error)
	})
	if !ok {
		return false, ErrRelationNotTable
	}
	return c.ContainsFactAST(r.extractOtherFact(values))
}

// Empty is empty(): garbage_collect(true), m_table->empty().
func (r *finiteProductRelation) Empty() (bool, error) {
	if err := r.garbageCollect(true); err != nil {
		return false, err
	}
	n, err := r.table.Len()
	return n == 0, err
}

func (r *finiteProductRelation) isNonEmpty() (bool, error) {
	empty, err := r.Empty()
	return !empty, err
}

// FastEmpty is relation_base::fast_empty(): empty().
func (r *finiteProductRelation) FastEmpty() bool {
	empty, err := r.Empty()
	return err == nil && empty
}

func (r *finiteProductRelation) IsPrecise() bool { return true }

// Reset is reset(): m_table->reset(), garbage_collect(false).
func (r *finiteProductRelation) Reset() error {
	if err := r.table.Reset(); err != nil {
		return err
	}
	return r.garbageCollect(false)
}

// clone is the copy constructor.
func (r *finiteProductRelation) clone() (*finiteProductRelation, error) {
	t, err := r.table.Clone()
	if err != nil {
		return nil, err
	}
	c := &finiteProductRelation{plugin: r.plugin, sorts: r.sorts, kind: r.kind,
		tableSig: r.tableSig, table2sig: r.table2sig, sig2table: r.sig2table,
		otherSig: r.otherSig, other2sig: r.other2sig, sig2other: r.sig2other,
		otherPlugin: r.otherPlugin, otherKind: r.otherKind, table: t,
		others: make([]RelationBase, len(r.others)), available: append([]int(nil), r.available...), fullRelIdx: r.fullRelIdx}
	for i, o := range r.others {
		if o == nil {
			continue
		}
		if c.others[i], err = o.CloneRelation(); err != nil {
			return nil, errors.Join(err, c.Close())
		}
	}
	return c, nil
}

func (r *finiteProductRelation) CloneRelation() (RelationBase, error) {
	c, err := r.clone()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *finiteProductRelation) CloneFinite() (FiniteTable, error) { return r.CloneRelation() }

// complementSelf is complement_self(p).
func (r *finiteProductRelation) complementSelf(p ast.ASTHandle) (resultErr error) {
	for i, o := range r.others {
		if o == nil {
			continue
		}
		c, ok := o.(interface {
			Complement(ast.ASTHandle) (RelationBase, error)
		})
		if !ok {
			return ErrInstruction
		}
		res, err := c.Complement(p)
		if err != nil {
			return err
		}
		r.others[i] = res
		if err = o.Close(); err != nil {
			return err
		}
	}
	full, err := r.getFullRelIdx()
	if err != nil {
		return err
	}
	complement, err := r.table.ComplementFunctional(Fact{Element(full)})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, complement.Close()) }()
	return fprTableUnion(r.table, complement, nil)
}

// complement is complement(p): clone, complement_self.
func (r *finiteProductRelation) complement(p ast.ASTHandle) (RelationBase, error) {
	res, err := r.clone()
	if err != nil {
		return nil, err
	}
	if err = res.complementSelf(p); err != nil {
		return nil, errors.Join(err, res.Close())
	}
	return res, nil
}

// Complement is relation_base::complement(p).
func (r *finiteProductRelation) Complement(p ast.ASTHandle) (RelationBase, error) {
	return r.complement(p)
}

// collectLiveRelationIndexes is collect_live_relation_indexes.
func (r *finiteProductRelation) collectLiveRelationIndexes() (map[int]bool, error) {
	res := make(map[int]bool)
	dataCols := len(r.tableSig) - 1
	if dataCols == 0 {
		facts, err := r.table.Facts()
		if err != nil {
			return nil, err
		}
		if len(facts) != 0 {
			res[int(facts[0][0])] = true
		}
		return res, nil
	}
	removed := make([]int, dataCols)
	for i := range removed {
		removed[i] = i
	}
	live, err := r.table.ProjectWithReduce(removed, func(_, merged []Element) {
		res[int(merged[0])] = true
	})
	if err != nil {
		return nil, err
	}
	facts, err := live.Facts()
	err = errors.Join(err, live.Close())
	if err != nil {
		return nil, err
	}
	if len(facts) != 0 {
		res[int(facts[0][0])] = true
	}
	return res, nil
}

// garbageCollect is garbage_collect(remove_empty).
func (r *finiteProductRelation) garbageCollect(removeEmpty bool) (resultErr error) {
	live, err := r.collectLiveRelationIndexes()
	if err != nil {
		return err
	}
	var emptyRelIndexes *SparseTable
	defer func() {
		if emptyRelIndexes != nil {
			resultErr = errors.Join(resultErr, emptyRelIndexes.Close())
		}
	}()
	for relIdx := range r.others {
		o := r.others[relIdx]
		if o == nil {
			continue
		}
		if live[relIdx] {
			if !removeEmpty {
				continue
			}
			empty, err := o.Empty()
			if err != nil {
				return err
			}
			if !empty {
				continue
			}
			if emptyRelIndexes == nil {
				if emptyRelIndexes, err = NewSparseTable(Signature{domains: []uint64{fprRelIdxSort}}, 0); err != nil {
					return err
				}
			}
			if _, err = emptyRelIndexes.Add(Fact{Element(relIdx)}); err != nil {
				return err
			}
		}
		r.others[relIdx] = nil
		if err := o.Close(); err != nil {
			return err
		}
		if relIdx == r.fullRelIdx {
			r.fullRelIdx = fprNoIndex
		}
		r.recycleRelIdx(relIdx)
	}
	if len(r.available) == len(r.others) {
		r.available = nil
		r.others = nil
	}
	if emptyRelIndexes != nil {
		return finiteTableFilterNegation(r.table, emptyRelIndexes, []int{len(r.tableSig) - 1}, []int{0})
	}
	return nil
}

// tryUnifySpecifications is try_unify_specifications.
func tryUnifySpecifications(rels []*finiteProductRelation) (bool, error) {
	if len(rels) == 0 {
		return true, nil
	}
	sz := len(rels[len(rels)-1].sorts)
	tableCols := make([]bool, sz)
	for i := range tableCols {
		tableCols[i] = true
	}
	for _, rel := range rels {
		for i := 0; i < sz; i++ {
			tableCols[i] = tableCols[i] && rel.isTableColumn(i)
		}
	}
	for _, rel := range rels {
		ok, err := rel.tryModifySpecification(tableCols)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// tryModifySpecification is try_modify_specification(table_cols).
func (r *finiteProductRelation) tryModifySpecification(tableCols []bool) (_ bool, resultErr error) {
	rmgr := r.plugin.manager
	var newRelColumns, toProjectAway []int
	var movedColsSig []ast.ASTHandle
	for i := range r.sorts {
		if tableCols[i] && !r.isTableColumn(i) {
			// we cannot move columns from relation into the table, only the
			// other way round
			return false, nil
		}
		if r.isTableColumn(i) {
			if !tableCols[i] {
				newRelColumns = append(newRelColumns, i)
				movedColsSig = append(movedColsSig, r.sorts[i])
			} else {
				toProjectAway = append(toProjectAway, r.sig2table[i])
			}
		}
	}
	toProjectAway = append(toProjectAway, len(r.tableSig)-1)
	if len(newRelColumns) == 0 {
		return true, nil
	}
	if ok, err := r.otherPlugin.CanHandleSignature(movedColsSig); err != nil || !ok {
		return false, err
	}
	movedColsTable, err := finiteTableProject(r.table, toProjectAway)
	if err != nil {
		return false, err
	}
	movedColsTrel := rmgr.tableView(movedColsTable, movedColsSig, false)
	defer func() { resultErr = errors.Join(resultErr, movedColsTrel.Close()) }()
	movedColsRel, err := r.plugin.mkEmptyCols(movedColsSig, make([]bool, len(movedColsSig)), nullRelationKind)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, movedColsRel.Close()) }()
	unionFn, err := rmgr.MkRelationUnionFn(movedColsRel, movedColsTrel, nil)
	if err == nil && unionFn == nil {
		err = ErrInstruction
	}
	if err != nil {
		return false, err
	}
	if err = unionFn(movedColsRel, movedColsTrel, nil); err != nil {
		return false, err
	}
	allMoved := make([]int, len(movedColsSig))
	for i := range allMoved {
		allMoved[i] = i
	}
	joinFn, err := rmgr.MkRelationJoinProjectFn(r, movedColsRel, newRelColumns, allMoved, newRelColumns, false)
	if err == nil && joinFn == nil {
		err = ErrInstruction
	}
	if err != nil {
		return false, err
	}
	unordered, err := joinFn(r, movedColsRel)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, unordered.Close()) }()
	sz := len(r.sorts)
	permutation := make([]uint32, 0, sz)
	movedCnt := len(newRelColumns)
	nextReplaced, nextOrig := 0, 0
	for i := 0; i < sz; i++ {
		if nextReplaced < movedCnt && newRelColumns[nextReplaced] == i {
			permutation = append(permutation, uint32(sz-movedCnt+nextReplaced))
			nextReplaced++
		} else {
			permutation = append(permutation, uint32(nextOrig))
			nextOrig++
		}
	}
	for {
		cycle32, found := removePermutationCycle(permutation)
		if !found {
			break
		}
		cycle := make([]int, len(cycle32))
		for i, c := range cycle32 {
			cycle[i] = int(c)
		}
		permFn, err := rmgr.MkRelationRenameFn(unordered, cycle)
		if err == nil && permFn == nil {
			err = ErrInstruction
		}
		if err != nil {
			return false, err
		}
		next, err := permFn(unordered)
		if err != nil {
			return false, err
		}
		if err = unordered.Close(); err != nil {
			return false, errors.Join(err, next.Close())
		}
		unordered = next
	}
	newRel, ok := unordered.(*finiteProductRelation)
	if !ok {
		return false, ErrInstruction
	}
	r.swapContent(newRel)
	return true, nil
}

// swapContent is swap(r).
func (r *finiteProductRelation) swapContent(o *finiteProductRelation) {
	r.tableSig, o.tableSig = o.tableSig, r.tableSig
	r.table2sig, o.table2sig = o.table2sig, r.table2sig
	r.sig2table, o.sig2table = o.sig2table, r.sig2table
	r.otherSig, o.otherSig = o.otherSig, r.otherSig
	r.other2sig, o.other2sig = o.other2sig, r.other2sig
	r.sig2other, o.sig2other = o.sig2other, r.sig2other
	r.table, o.table = o.table, r.table
	r.others, o.others = o.others, r.others
	r.available, o.available = o.available, r.available
	r.fullRelIdx, o.fullRelIdx = o.fullRelIdx, r.fullRelIdx
	// relation_base::swap: the kinds.
	r.kind, o.kind = o.kind, r.kind
}

// canSwap / swap are relation_base::can_swap / swap.
func (r *finiteProductRelation) canSwap(other RelationBase) bool {
	return other.RelationPlugin() == r.plugin
}

func (r *finiteProductRelation) swap(other RelationBase) {
	r.swapContent(other.(*finiteProductRelation))
}

// Close is ~finite_product_relation.
func (r *finiteProductRelation) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	err := r.table.Close()
	for _, o := range r.others {
		if o != nil {
			err = errors.Join(err, o.Close())
		}
	}
	r.others = nil
	return err
}

func (r *finiteProductRelation) RelationSignature() []ast.ASTHandle { return r.sorts }
func (r *finiteProductRelation) RelationPlugin() RelationPlugin     { return r.plugin }

func (r *finiteProductRelation) SizeEstimateRows() (uint32, error) {
	return r.table.SizeEstimateRows()
}
func (r *finiteProductRelation) SizeEstimateBytes() (uint32, error) {
	return r.table.SizeEstimateBytes()
}

// Display is display(out).
func (r *finiteProductRelation) Display(out io.Writer) error {
	if err := r.garbageCollect(true); err != nil {
		return err
	}
	if _, err := io.WriteString(out, "finite_product_relation:\n table:\n"); err != nil {
		return err
	}
	if err := r.table.Display(out); err != nil {
		return err
	}
	for i, o := range r.others {
		if o == nil {
			continue
		}
		if _, err := fmt.Fprintf(out, " inner relation %d:\n", i); err != nil {
			return err
		}
		if err := o.Display(out); err != nil {
			return err
		}
	}
	return nil
}

// ToFormula is to_formula.
func (r *finiteProductRelation) ToFormula() (result ast.ASTRef, resultErr error) {
	m := r.plugin.manager.manager
	facts, err := r.table.Facts()
	if err != nil {
		return ast.ASTRef{}, err
	}
	// The source's locals: expr_ref_vector disjs, conjs; expr_ref tmp;
	// shift_vars sh. They are destroyed in reverse declaration order (sh,
	// tmp, conjs, disjs), each vector first to last.
	var disjs, conjs []ast.ASTRef
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	shifter, err := rewriter.NewVarShifter(m)
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&tmp))
	}
	release := func(refs []ast.ASTRef) (out error) {
		for i := range refs {
			out = errors.Join(out, m.DelRef(&refs[i]))
		}
		return out
	}
	defer func() {
		resultErr = errors.Join(resultErr, shifter.Close(), m.DelRef(&tmp), release(conjs), release(disjs))
	}()
	factSz := len(r.tableSig)
	for _, fact := range facts {
		// conjs.reset()
		if err = release(conjs); err != nil {
			return ast.ASTRef{}, err
		}
		conjs = conjs[:0]
		inner, err := r.getInnerRel(int(fact[factSz-1]))
		if err != nil {
			return ast.ASTRef{}, err
		}
		// m_others[rel_idx]->to_formula(tmp)
		f, err := inner.ToFormula()
		if err != nil {
			return ast.ASTRef{}, err
		}
		fh, err := m.RefGet(f)
		if err == nil {
			err = m.SetRef(tmp, fh)
		}
		if err = errors.Join(err, m.DelRef(&f)); err != nil {
			return ast.ASTRef{}, err
		}
		for i := 0; i+1 < factSz; i++ {
			// conjs.push_back(m.mk_eq(m.mk_var(i, sig[i]),
			// util.mk_numeral(fact[i], sig[i]))): the variable and the
			// numeral are held by the equality only.
			v, err := m.MkVar(uint32(i), r.sorts[i])
			if err != nil {
				return ast.ASTRef{}, err
			}
			vRef, err := m.NewRef(v)
			if err != nil {
				return ast.ASTRef{}, err
			}
			numRef, err := r.plugin.manager.DecodeValue(r.sorts[i], fact[i])
			if err != nil {
				return ast.ASTRef{}, errors.Join(err, m.DelRef(&vRef))
			}
			num, err := m.RefGet(numRef)
			var eq ast.ASTHandle
			if err == nil {
				eq, err = m.MkEq(v, num)
			}
			var eqRef ast.ASTRef
			if err == nil {
				eqRef, err = m.NewRef(eq)
			}
			err = errors.Join(err, m.DelRef(&numRef), m.DelRef(&vRef))
			if err != nil {
				return ast.ASTRef{}, err
			}
			conjs = append(conjs, eqRef)
		}
		// sh(tmp, fact_sz-1, tmp)
		th, err := m.RefGet(tmp)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err = shifter.ShiftBy(th, uint32(factSz-1), tmp); err != nil {
			return ast.ASTRef{}, err
		}
		// conjs.push_back(tmp)
		th, err = m.RefGet(tmp)
		if err != nil {
			return ast.ASTRef{}, err
		}
		tRef, err := m.NewRef(th)
		if err != nil {
			return ast.ASTRef{}, err
		}
		conjs = append(conjs, tRef)
		args := make([]ast.ASTHandle, len(conjs))
		for i := range conjs {
			if args[i], err = m.RefGet(conjs[i]); err != nil {
				return ast.ASTRef{}, err
			}
		}
		and, err := m.MkAndArgs(args)
		if err != nil {
			return ast.ASTRef{}, err
		}
		andRef, err := m.NewRef(and)
		if err != nil {
			return ast.ASTRef{}, err
		}
		disjs = append(disjs, andRef)
	}
	disjH := make([]ast.ASTHandle, len(disjs))
	for i := range disjs {
		if disjH[i], err = m.RefGet(disjs[i]); err != nil {
			return ast.ASTRef{}, err
		}
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpOr, disjH, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}

// Signature is the register signature: table domains, relationOnlyDomain
// for the other columns.
func (r *finiteProductRelation) Signature() (Signature, error) {
	domains := make([]uint64, len(r.sorts))
	for i, s := range r.sorts {
		d, err := r.plugin.manager.relationSortDomain(s)
		if err != nil {
			return Signature{}, err
		}
		domains[i] = d
	}
	return Signature{domains: domains}, nil
}

func (r *finiteProductRelation) Facts() ([]Fact, error) { return nil, ErrRelationNotTable }

// Len is the register-emptiness estimate.
func (r *finiteProductRelation) Len() (int, error) {
	empty, err := r.Empty()
	if err != nil || empty {
		return 0, err
	}
	return r.table.Len()
}

func (r *finiteProductRelation) decodeFact(fact Fact) ([]ast.ASTHandle, []ast.ASTRef, error) {
	values := make([]ast.ASTHandle, len(fact))
	refs := make([]ast.ASTRef, 0, len(fact))
	for i, e := range fact {
		ref, err := r.plugin.manager.DecodeRelationElement(r.sorts[i], e)
		if err != nil {
			return nil, refs, err
		}
		refs = append(refs, ref)
		if values[i], err = r.plugin.manager.manager.RefGet(ref); err != nil {
			return nil, refs, err
		}
	}
	return values, refs, nil
}

// Add is add_fact of relation_manager::table_fact_to_relation(fact).
func (r *finiteProductRelation) Add(fact Fact) (bool, error) {
	values, refs, err := r.decodeFact(fact)
	if err == nil {
		err = r.AddFactAST(values)
	}
	return true, errors.Join(err, releaseRefs(r.plugin.manager.manager, refs))
}

// Contains is contains_fact.
func (r *finiteProductRelation) Contains(fact Fact) (bool, error) {
	values, refs, err := r.decodeFact(fact)
	var found bool
	if err == nil {
		found, err = r.ContainsFactAST(values)
	}
	return found, errors.Join(err, releaseRefs(r.plugin.manager.manager, refs))
}

// releaseNullableRefs releases the non-null references of an
// expr_ref_vector with null entries.
func releaseNullableRefs(m *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for i := len(refs) - 1; i >= 0; i-- {
		if refs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}
	return result
}
