package rel

// Source: src/muz/rel/dl_sieve_relation.cpp and dl_sieve_relation.h:
// sieve_relation_plugin and sieve_relation. A sieve relation holds an
// inner relation over a subset of its columns (the inner columns); the
// other (sieved) columns are unconstrained. product_relation's join_fn
// creates sieves when an inner relation's plugin cannot handle the other
// operand's signature (sieve_relation_plugin::full).

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// sieveSpec is sieve_relation_plugin::rel_spec.
type sieveSpec struct {
	innerCols []bool
	innerKind int
}

func (s sieveSpec) key() string {
	b := make([]byte, 0, len(s.innerCols)+8)
	for _, c := range s.innerCols {
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

// sieveSpecStore is rel_spec_store<rel_spec> of the sieve plugin.
type sieveSpecStore struct {
	parent     *SieveRelationPlugin
	allocated  []int
	assignment map[string]map[string]int
	specs      map[string]map[int]sieveSpec
}

// getRelationKind is rel_spec_store::get_relation_kind.
func (s *sieveSpecStore) getRelationKind(sorts []ast.ASTHandle, spec sieveSpec) int {
	key := sortsKey(sorts)
	ids, found := s.assignment[key]
	if !found {
		ids = make(map[string]int)
		s.assignment[key] = ids
		s.specs[key] = make(map[int]sieveSpec)
	}
	sk := spec.key()
	idx, found := ids[sk]
	if !found {
		idx = len(ids)
		if idx == len(s.allocated) {
			// add_new_kind
			s.allocated = append(s.allocated, s.parent.manager.claimRelationKind(s.parent))
		}
		ids[sk] = idx
		s.specs[key][s.allocated[idx]] = sieveSpec{innerCols: append([]bool(nil), spec.innerCols...), innerKind: spec.innerKind}
	}
	return s.allocated[idx]
}

// getRelationSpec is rel_spec_store::get_relation_spec.
func (s *sieveSpecStore) getRelationSpec(sorts []ast.ASTHandle, kind int) (sieveSpec, bool) {
	spec, found := s.specs[sortsKey(sorts)][kind]
	return spec, found
}

// SieveRelationPlugin is sieve_relation_plugin.
type SieveRelationPlugin struct {
	RelationPluginDefaults
	manager *RelationManager
	kind    int
	store   sieveSpecStore
}

// sievePlugin is sieve_relation_plugin::get_plugin(rmgr): registered on
// first use.
func (m *RelationManager) sievePlugin() *SieveRelationPlugin {
	if p, ok := m.GetRelationPlugin("sieve_relation").(*SieveRelationPlugin); ok {
		return p
	}
	p := &SieveRelationPlugin{manager: m}
	p.store = sieveSpecStore{parent: p, assignment: make(map[string]map[string]int), specs: make(map[string]map[int]sieveSpec)}
	_ = m.RegisterRelationPlugin(p)
	return p
}

// initializeKind is initialize(fid): m_spec_store.add_available_kind.
func (p *SieveRelationPlugin) initializeKind(fid int) {
	p.kind = fid
	p.store.allocated = append(p.store.allocated, fid)
}

func (p *SieveRelationPlugin) Name() string              { return "sieve_relation" }
func (p *SieveRelationPlugin) Manager() *RelationManager { return p.manager }

// CanHandleSignature: "we do not want this plugin to handle anything by
// default".
func (p *SieveRelationPlugin) CanHandleSignature([]ast.ASTHandle) (bool, error) {
	return false, nil
}

// MkEmpty(s) is UNREACHABLE() in the C++.
func (p *SieveRelationPlugin) MkEmpty([]ast.ASTHandle) (RelationBase, error) {
	return nil, ErrInstruction
}

// CanHandleSignatureKind is can_handle_signature(s, kind): the fork's
// override (FORK FIX, see FORK_BUGFIXES.md). z3 4.7.1 inherited
// can_handle_signature(s), which is false, so mk_empty_relation(s, k) and
// mk_full_relation(s, p, k) of a product spec's sieve kind made a table or
// a full relation in place of the sieve; the product's aligned union then
// called a missing inner union and crashed.
func (p *SieveRelationPlugin) CanHandleSignatureKind(sorts []ast.ASTHandle, kind int) (bool, error) {
	_, found := p.store.getRelationSpec(sorts, kind)
	return found, nil
}

// MkEmptyKind is mk_empty(s, kind).
func (p *SieveRelationPlugin) MkEmptyKind(sorts []ast.ASTHandle, kind int) (RelationBase, error) {
	spec, found := p.store.getRelationSpec(sorts, kind)
	if !found {
		return nil, ErrInstruction
	}
	inner, err := p.manager.mkEmptyRelationKind(collectInnerSignature(sorts, spec.innerCols), spec.innerKind)
	if err != nil {
		return nil, err
	}
	return p.mkFromInner(sorts, spec.innerCols, inner), nil
}

// MkFullKind is the fork's mk_full(p, s, kind): the sieve of the kind's
// spec over the inner kind's full relation.
func (p *SieveRelationPlugin) MkFullKind(predicate ast.ASTHandle, sorts []ast.ASTHandle, kind int) (RelationBase, error) {
	spec, found := p.store.getRelationSpec(sorts, kind)
	if kind == nullRelationKind || !found {
		return p.MkFull(predicate, sorts)
	}
	inner, err := p.manager.mkFullRelationKind(collectInnerSignature(sorts, spec.innerCols), predicate, spec.innerKind)
	if err != nil {
		return nil, err
	}
	return p.mkFromInner(sorts, spec.innerCols, inner), nil
}

// MkFull is mk_full(p, s): a sieve with no inner columns over the
// appropriate plugin's full relation of the empty signature.
func (p *SieveRelationPlugin) MkFull(predicate ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	plugin, err := p.manager.appropriatePlugin(sorts)
	if err != nil {
		return nil, err
	}
	inner, err := mkFullNullKind(plugin, predicate, nil)
	if err != nil {
		return nil, err
	}
	return p.mkFromInner(sorts, make([]bool, len(sorts)), inner), nil
}

// mkFullNullKind is plugin.mk_full(p, s, null_family_id).
func mkFullNullKind(plugin RelationPlugin, predicate ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	if k, ok := plugin.(kindedRelationPlugin); ok {
		return k.MkFullKind(predicate, sorts, nullRelationKind)
	}
	return plugin.MkFull(predicate, sorts)
}

// extractInnerColumns is extract_inner_columns: the columns whose sort
// the inner plugin handles as a singleton signature.
func extractInnerColumns(sorts []ast.ASTHandle, inner RelationPlugin) ([]bool, error) {
	cols := make([]bool, len(sorts))
	for i, s := range sorts {
		ok, err := inner.CanHandleSignature([]ast.ASTHandle{s})
		if err != nil {
			return nil, err
		}
		cols[i] = ok
	}
	return cols, nil
}

// collectInnerSignature is collect_inner_signature.
func collectInnerSignature(sorts []ast.ASTHandle, innerCols []bool) []ast.ASTHandle {
	inner := make([]ast.ASTHandle, 0, len(sorts))
	for i, s := range sorts {
		if innerCols[i] {
			inner = append(inner, s)
		}
	}
	return inner
}

// mkEmptyWithInner is mk_empty(s, inner_plugin).
func (p *SieveRelationPlugin) mkEmptyWithInner(sorts []ast.ASTHandle, inner RelationPlugin) (*sieveRelation, error) {
	cols, err := extractInnerColumns(sorts, inner)
	if err != nil {
		return nil, err
	}
	rel, err := inner.MkEmpty(collectInnerSignature(sorts, cols))
	if err != nil {
		return nil, err
	}
	return p.mkFromInner(sorts, cols, rel), nil
}

// full is sieve_relation_plugin::full(p, s, inner_plugin).
func (p *SieveRelationPlugin) full(predicate ast.ASTHandle, sorts []ast.ASTHandle, inner RelationPlugin) (*sieveRelation, error) {
	cols, err := extractInnerColumns(sorts, inner)
	if err != nil {
		return nil, err
	}
	rel, err := mkFullNullKind(inner, predicate, collectInnerSignature(sorts, cols))
	if err != nil {
		return nil, err
	}
	return p.mkFromInner(sorts, cols, rel), nil
}

// mkFromInner is mk_from_inner (and the sieve_relation constructor).
func (p *SieveRelationPlugin) mkFromInner(sorts []ast.ASTHandle, innerCols []bool, inner RelationBase) *sieveRelation {
	n := len(sorts)
	r := &sieveRelation{plugin: p, sorts: append([]ast.ASTHandle(nil), sorts...), innerCols: append([]bool(nil), innerCols...), inner: inner}
	r.sig2inner = make([]int, n)
	for i := 0; i < n; i++ {
		if innerCols[i] {
			r.sig2inner[i] = len(r.inner2sig)
			r.inner2sig = append(r.inner2sig, i)
		} else {
			r.sig2inner[i] = -1
			r.ignoredCols = append(r.ignoredCols, i)
		}
	}
	r.kind = p.store.getRelationKind(r.sorts, sieveSpec{innerCols: r.innerCols, innerKind: p.manager.RelationKindOf(inner)})
	return r
}

// isSieveRelation is relation_plugin::is_sieve_relation of r's plugin.
func isSieveRelation(r RelationBase) (*sieveRelation, bool) {
	s, ok := r.(*sieveRelation)
	return s, ok
}

// sieveInner is "r_sieved ? sr->get_inner() : r".
func sieveInner(r RelationBase) RelationBase {
	if s, ok := isSieveRelation(r); ok {
		return s.inner
	}
	return r
}

// ---------------------------------------------------------------------------
// join_fn

func (p *SieveRelationPlugin) MkJoinFn(r1, r2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	sr1, r1Sieved := isSieveRelation(r1)
	sr2, r2Sieved := isSieveRelation(r2)
	if !r1Sieved && !r2Sieved {
		// we create just operations that involve the current plugin
		return nil, nil
	}
	inner1, inner2 := sieveInner(r1), sieveInner(r2)
	var innerCols1, innerCols2 []int
	for i := range cols1 {
		// if at least one end of an equality is not an inner column, we
		// ignore that equality (which introduces imprecision)
		if r1Sieved && !sr1.isInnerCol(cols1[i]) {
			continue
		}
		if r2Sieved && !sr2.isInnerCol(cols2[i]) {
			continue
		}
		c1, c2 := cols1[i], cols2[i]
		if r1Sieved {
			c1 = sr1.sig2inner[c1]
		}
		if r2Sieved {
			c2 = sr2.sig2inner[c2]
		}
		innerCols1 = append(innerCols1, c1)
		innerCols2 = append(innerCols2, c2)
	}
	innerJoin, err := p.manager.MkRelationJoinFn(inner1, inner2, innerCols1, innerCols2, false)
	if err != nil || innerJoin == nil {
		return nil, err
	}
	// join_fn's constructor: the result's inner columns.
	var resultCols []bool
	if r1Sieved {
		resultCols = append(resultCols, sr1.innerCols...)
	} else {
		resultCols = appendTrue(resultCols, len(r1.RelationSignature()))
	}
	if r2Sieved {
		resultCols = append(resultCols, sr2.innerCols...)
	} else {
		resultCols = appendTrue(resultCols, len(r2.RelationSignature()))
	}
	sorts := joinSorts(r1.RelationSignature(), r2.RelationSignature())
	return func(a, b RelationBase) (RelationBase, error) {
		innerRes, err := innerJoin(sieveInner(a), sieveInner(b))
		if err != nil {
			return nil, err
		}
		return p.mkFromInner(sorts, resultCols, innerRes), nil
	}, nil
}

func appendTrue(cols []bool, n int) []bool {
	for i := 0; i < n; i++ {
		cols = append(cols, true)
	}
	return cols
}

// ---------------------------------------------------------------------------
// transformer_fn: project and rename

func (p *SieveRelationPlugin) transformerFn(inner RelationTransformerFn, sorts []ast.ASTHandle, resultCols []bool) RelationTransformerFn {
	return func(a RelationBase) (RelationBase, error) {
		r, ok := isSieveRelation(a)
		if !ok {
			return nil, ErrInstruction
		}
		innerRes, err := inner(r.inner)
		if err != nil {
			return nil, err
		}
		return p.mkFromInner(sorts, resultCols, innerRes), nil
	}
}

// identityRelationTransformer is identity_relation_transformer_fn.
func identityRelationTransformer(r RelationBase) (RelationBase, error) {
	return r.CloneRelation()
}

func (p *SieveRelationPlugin) MkProjectFn(t RelationBase, removed []int) (RelationTransformerFn, error) {
	r, ok := isSieveRelation(t)
	if !ok {
		return nil, nil
	}
	var innerRemoved []int
	for _, col := range removed {
		if r.isInnerCol(col) {
			innerRemoved = append(innerRemoved, r.sig2inner[col])
		}
	}
	// project_out_vector_columns(result_inner_cols, removed)
	resultCols := make([]bool, 0, len(r.innerCols))
	next := 0
	for i, c := range r.innerCols {
		if next < len(removed) && removed[next] == i {
			next++
			continue
		}
		resultCols = append(resultCols, c)
	}
	sorts := projectSorts(r.sorts, removed)
	var innerFn RelationTransformerFn
	if len(innerRemoved) == 0 {
		innerFn = identityRelationTransformer
	} else {
		var err error
		if innerFn, err = p.manager.MkRelationProjectFn(r.inner, innerRemoved); err != nil {
			return nil, err
		}
	}
	if innerFn == nil {
		return nil, nil
	}
	return p.transformerFn(innerFn, sorts, resultCols), nil
}

func (p *SieveRelationPlugin) MkRenameFn(t RelationBase, cycle []int) (RelationTransformerFn, error) {
	r, ok := isSieveRelation(t)
	if !ok {
		return nil, nil
	}
	n := len(r.sorts)
	// add_sequence(0, sig_sz, permutation); permutate_by_cycle
	permutation := make([]int, n)
	for i := range permutation {
		permutation[i] = i
	}
	permutateIntsByCycle(permutation, cycle)
	// collect_sub_permutation(permutation, m_sig2inner, inner_permutation)
	var innerPermutation []uint32
	for _, idx := range permutation {
		if sel := r.sig2inner[idx]; sel >= 0 {
			innerPermutation = append(innerPermutation, uint32(sel))
		}
	}
	resultCols := append([]bool(nil), r.innerCols...)
	if len(cycle) >= 2 {
		first := resultCols[cycle[0]]
		for i := 0; i+1 < len(cycle); i++ {
			resultCols[cycle[i]] = resultCols[cycle[i+1]]
		}
		resultCols[cycle[len(cycle)-1]] = first
	}
	sorts := renameSorts(r.sorts, cycle)
	innerFn, err := p.manager.MkRelationPermutationRenameFn(r.inner, innerPermutation)
	if err != nil || innerFn == nil {
		return nil, err
	}
	return p.transformerFn(innerFn, sorts, resultCols), nil
}

// permutateIntsByCycle is permutate_by_cycle.
func permutateIntsByCycle(v []int, cycle []int) {
	if len(cycle) < 2 {
		return
	}
	first := v[cycle[0]]
	for i := 0; i+1 < len(cycle); i++ {
		v[cycle[i]] = v[cycle[i+1]]
	}
	v[cycle[len(cycle)-1]] = first
}

// MkRelationPermutationRenameFn is relation_manager::
// mk_permutation_rename_fn: no relation plugin implements
// mk_permutation_rename_fn, so default_relation_permutation_rename_fn
// renames by the permutation's cycles, building its renamers on the first
// call.
func (m *RelationManager) MkRelationPermutationRenameFn(t RelationBase, permutation []uint32) (RelationTransformerFn, error) {
	perm := append([]uint32(nil), permutation...)
	initialized := false
	var renamers []RelationTransformerFn
	return func(o RelationBase) (RelationBase, error) {
		res := o
		var owned RelationBase
		step := func(fn RelationTransformerFn) error {
			next, err := fn(res)
			if owned != nil {
				err = errors.Join(err, owned.Close())
			}
			if err != nil {
				if next != nil {
					err = errors.Join(err, next.Close())
				}
				owned = nil
				return err
			}
			res, owned = next, next
			return nil
		}
		if initialized {
			for _, fn := range renamers {
				if err := step(fn); err != nil {
					return nil, err
				}
			}
		} else {
			for {
				cycle32, found := removePermutationCycle(perm)
				if !found {
					break
				}
				cycle := make([]int, len(cycle32))
				for i, c := range cycle32 {
					cycle[i] = int(c)
				}
				renamer, err := m.MkRelationRenameFn(res, cycle)
				if err == nil && renamer == nil {
					err = unsupportedRelationOperationOne("rename", res.RelationPlugin().Name())
				}
				if err != nil {
					if owned != nil {
						err = errors.Join(err, owned.Close())
					}
					return nil, err
				}
				renamers = append(renamers, renamer)
				if err = step(renamer); err != nil {
					return nil, err
				}
			}
			initialized = true
		}
		if owned != nil {
			return owned, nil
		}
		return res.CloneRelation()
	}, nil
}

// ---------------------------------------------------------------------------
// union_fn

func (p *SieveRelationPlugin) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	stgt, tgtSieved := isSieveRelation(target)
	ssrc, srcSieved := isSieveRelation(source)
	var sdelta *sieveRelation
	deltaSieved := false
	if delta != nil {
		sdelta, deltaSieved = isSieveRelation(delta)
	}
	if !tgtSieved && !srcSieved && (delta != nil && !deltaSieved) {
		// we create the operation only if it involves this plugin
		return nil, nil
	}
	// Now we require that the sieved and inner columns must match on all
	// relations.
	if tgtSieved && srcSieved && (delta == nil || deltaSieved) {
		if !boolsEqual(stgt.innerCols, ssrc.innerCols) || (delta != nil && !boolsEqual(stgt.innerCols, sdelta.innerCols)) {
			return nil, nil
		}
	} else {
		if (stgt != nil && !stgt.noSievedColumns()) || (ssrc != nil && !ssrc.noSievedColumns()) || (sdelta != nil && !sdelta.noSievedColumns()) {
			// We have an unsieved relation and then some relation with some
			// sieved columns, which means there is an misalignment.
			return nil, nil
		}
	}
	var idelta RelationBase
	if delta != nil {
		idelta = sieveInner(delta)
	}
	unionFn, err := p.manager.MkRelationUnionFn(sieveInner(target), sieveInner(source), idelta)
	if err != nil || unionFn == nil {
		return nil, err
	}
	return func(t, s, d RelationBase) error {
		var id RelationBase
		if d != nil {
			id = sieveInner(d)
		}
		return unionFn(sieveInner(t), sieveInner(s), id)
	}, nil
}

func boolsEqual(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// filter_fn

func (p *SieveRelationPlugin) filterFn(inner RelationMutatorFn) RelationMutatorFn {
	return func(a RelationBase) error {
		r, ok := isSieveRelation(a)
		if !ok {
			return ErrInstruction
		}
		return inner(r.inner)
	}
}

// identityRelationMutator is identity_relation_mutator_fn.
func identityRelationMutator(RelationBase) error { return nil }

func (p *SieveRelationPlugin) MkFilterIdenticalFn(t RelationBase, cols []int) (RelationMutatorFn, error) {
	r, ok := isSieveRelation(t)
	if !ok {
		return nil, nil
	}
	var innerCols []int
	// we ignore the columns which do not belong to the inner relation
	// (which introduces imprecision)
	for _, col := range cols {
		if r.isInnerCol(col) {
			innerCols = append(innerCols, r.sig2inner[col])
		}
	}
	if len(innerCols) < 2 {
		return identityRelationMutator, nil
	}
	innerFn, err := p.manager.MkRelationFilterIdenticalFn(r.inner, innerCols)
	if err != nil || innerFn == nil {
		return nil, err
	}
	return p.filterFn(innerFn), nil
}

func (p *SieveRelationPlugin) MkFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error) {
	r, ok := isSieveRelation(t)
	if !ok {
		return nil, nil
	}
	if !r.isInnerCol(col) {
		// if the column which do not belong to the inner relation, we do
		// nothing (which introduces imprecision)
		return identityRelationMutator, nil
	}
	innerFn, err := p.manager.MkRelationFilterEqualFn(r.inner, value, r.sig2inner[col])
	if err != nil || innerFn == nil {
		return nil, err
	}
	return p.filterFn(innerFn), nil
}

func (p *SieveRelationPlugin) MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (_ RelationMutatorFn, resultErr error) {
	r, ok := isSieveRelation(t)
	if !ok {
		return nil, nil
	}
	m := p.manager.manager
	sz := len(r.sorts)
	// get_context().get_rule_manager().collect_vars(condition)
	used, err := ast.NewASTUsedVars(m)
	if err != nil {
		return nil, err
	}
	if err = used.Collect(condition); err != nil {
		return nil, err
	}
	subst := make([]ast.ASTHandle, sz)
	var refs []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseTableFormulaRefs(m, refs)) }()
	substOfs := sz - 1
	for i := 0; i < sz; i++ {
		if used.Contains(uint32(i)) == (ast.ASTHandle{}) {
			continue
		}
		if !r.isInnerCol(i) {
			// If the condition involves columns which do not belong to
			// the inner relation, we do nothing (which introduces
			// imprecision).
			return identityRelationMutator, nil
		}
		v, err := m.MkVar(uint32(r.sig2inner[i]), r.sorts[i])
		if err != nil {
			return nil, err
		}
		ref, err := m.NewRef(v)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
		subst[substOfs-i] = v
	}
	innerCond, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	refs = append(refs, innerCond)
	vs := p.manager.varSubst
	if vs == nil {
		return nil, ErrTable
	}
	err = vs.Substitute(condition, subst, innerCond)
	if err = errors.Join(err, vs.Reset()); err != nil {
		return nil, err
	}
	cond, err := m.RefGet(innerCond)
	if err != nil {
		return nil, err
	}
	innerFn, err := p.manager.MkRelationFilterInterpretedFn(r.inner, cond)
	if err != nil || innerFn == nil {
		return nil, err
	}
	return p.filterFn(innerFn), nil
}

// ---------------------------------------------------------------------------
// negation_filter_fn

// identityRelationIntersectionFilter is
// identity_relation_intersection_filter_fn.
func identityRelationIntersectionFilter(RelationBase, RelationBase) error { return nil }

func (p *SieveRelationPlugin) MkFilterByNegationFn(t, neg RelationBase, tCols, negCols []int) (RelationIntersectionFilterFn, error) {
	sr, rSieved := isSieveRelation(t)
	sneg, negSieved := isSieveRelation(neg)
	if !rSieved && !negSieved {
		// we create just operations that involve the current plugin
		return nil, nil
	}
	var irCols, inegCols []int
	for i := range tCols {
		// if at least one end of an equality is not an inner column, we
		// ignore that equality (which introduces imprecision)
		// FORK FIX #54 (sieve negation inner-column test, AUDIT
		// muz_rel_06-F5).
		rColInner := !rSieved || sr.isInnerCol(tCols[i])
		negColInner := !negSieved || sneg.isInnerCol(negCols[i])
		switch {
		case rColInner && negColInner:
			rc, nc := tCols[i], negCols[i]
			if rSieved {
				rc = sr.sig2inner[rc]
			}
			if negSieved {
				nc = sneg.sig2inner[nc]
			}
			irCols = append(irCols, rc)
			inegCols = append(inegCols, nc)
		case !rColInner && negColInner:
			// Sieved (i.e. full) column in r is matched on an inner column
			// in neg. If we assume the column in neg is not full, no rows
			// from the inner relation of r would be removed. So in this
			// case we perform no operation at cost of a little
			// impresicion.
			return identityRelationIntersectionFilter, nil
		default:
			// Inner or sieved column in r must match a sieved column in
			// neg. Since sieved columns are full, this is always true so
			// we can skip the equality.
		}
	}
	innerFn, err := p.manager.MkRelationFilterByNegationFn(sieveInner(t), sieveInner(neg), irCols, inegCols)
	if err != nil || innerFn == nil {
		return nil, err
	}
	return func(a, b RelationBase) error {
		return innerFn(sieveInner(a), sieveInner(b))
	}, nil
}

// ---------------------------------------------------------------------------
// sieve_relation

type sieveRelation struct {
	plugin      *SieveRelationPlugin
	sorts       []ast.ASTHandle
	innerCols   []bool
	sig2inner   []int // -1 for a sieved column (UINT_MAX)
	inner2sig   []int
	ignoredCols []int // ascending
	inner       RelationBase
	kind        int
	closed      bool
}

func (r *sieveRelation) isInnerCol(idx int) bool { return r.sig2inner[idx] >= 0 }
func (r *sieveRelation) noSievedColumns() bool   { return len(r.ignoredCols) == 0 }
func (r *sieveRelation) noInnerColumns() bool    { return len(r.ignoredCols) == len(r.sorts) }

func (r *sieveRelation) RelationKind() int                  { return r.kind }
func (r *sieveRelation) RelationSignature() []ast.ASTHandle { return r.sorts }
func (r *sieveRelation) RelationPlugin() RelationPlugin     { return r.plugin }

func (r *sieveRelation) Signature() (Signature, error) {
	m := r.plugin.manager
	if s, ok := m.signatureIsTable(r.sorts); ok {
		return s, nil
	}
	domains := make([]uint64, len(r.sorts))
	for i, sort := range r.sorts {
		d, err := m.relationSortDomain(sort)
		if err != nil {
			return Signature{}, err
		}
		domains[i] = d
	}
	return Signature{domains: domains}, nil
}

func (r *sieveRelation) Facts() ([]Fact, error) { return nil, ErrRelationNotTable }

// innerFact is project_out_vector_columns(inner_f, m_ignored_cols).
func (r *sieveRelation) innerFact(fact Fact) Fact {
	out := make(Fact, 0, len(r.inner2sig))
	for _, i := range r.inner2sig {
		out = append(out, fact[i])
	}
	return out
}

// Add is add_fact.
func (r *sieveRelation) Add(fact Fact) (bool, error) {
	return r.inner.Add(r.innerFact(fact))
}

// AddFactAST is add_fact of an expression fact.
func (r *sieveRelation) AddFactAST(values []ast.ASTHandle) error {
	inner := make([]ast.ASTHandle, 0, len(r.inner2sig))
	for _, i := range r.inner2sig {
		inner = append(inner, values[i])
	}
	if adder, ok := r.inner.(RelationAddFactAST); ok {
		return adder.AddFactAST(inner)
	}
	fact, err := r.plugin.manager.EncodeFact(r.inner.RelationSignature(), inner)
	if err != nil {
		return err
	}
	_, err = r.inner.Add(fact)
	return err
}

// Contains is contains_fact.
func (r *sieveRelation) Contains(fact Fact) (bool, error) {
	return r.inner.Contains(r.innerFact(fact))
}

func (r *sieveRelation) Empty() (bool, error) { return r.inner.Empty() }
func (r *sieveRelation) FastEmpty() bool      { return r.inner.FastEmpty() }

// IsPrecise is relation_base::is_precise() (sieve_relation does not
// override it).
func (r *sieveRelation) IsPrecise() bool { return true }

func (r *sieveRelation) Len() (int, error) { return r.inner.Len() }
func (r *sieveRelation) Reset() error      { return r.inner.Reset() }

func (r *sieveRelation) SizeEstimateRows() (uint32, error)  { return r.inner.SizeEstimateRows() }
func (r *sieveRelation) SizeEstimateBytes() (uint32, error) { return r.inner.SizeEstimateBytes() }

// CloneRelation is clone().
func (r *sieveRelation) CloneRelation() (RelationBase, error) {
	inner, err := r.inner.CloneRelation()
	if err != nil {
		return nil, err
	}
	return r.plugin.mkFromInner(r.sorts, r.innerCols, inner), nil
}

func (r *sieveRelation) CloneFinite() (FiniteTable, error) { return r.CloneRelation() }

func (r *sieveRelation) Close() error {
	if r.closed {
		return ErrTable
	}
	r.closed = true
	return r.inner.Close()
}

// ToFormula is to_formula: the inner formula with inner variable i
// renamed to the variable of its outer column.
func (r *sieveRelation) ToFormula() (result ast.ASTRef, resultErr error) {
	m := r.plugin.manager.manager
	sig := r.inner.RelationSignature()
	sz := len(sig)
	s := make([]ast.ASTHandle, 0, sz)
	var refs []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseTableFormulaRefs(m, refs)) }()
	for i := sz; i > 0; {
		i--
		v, err := m.MkVar(uint32(r.inner2sig[i]), sig[i])
		if err != nil {
			return ast.ASTRef{}, err
		}
		ref, err := m.NewRef(v)
		if err != nil {
			return ast.ASTRef{}, err
		}
		refs = append(refs, ref)
		s = append(s, v)
	}
	tmp, err := r.inner.ToFormula()
	if err != nil {
		return ast.ASTRef{}, err
	}
	refs = append(refs, tmp)
	h, err := m.RefGet(tmp)
	if err != nil {
		return ast.ASTRef{}, err
	}
	vs := r.plugin.manager.varSubst
	if vs == nil {
		return ast.ASTRef{}, ErrTable
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	err = vs.Substitute(h, s, result)
	if err = errors.Join(err, vs.Reset()); err != nil {
		return result, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}

// Display is display: "Sieve relation " and print_container(m_inner_cols).
func (r *sieveRelation) Display(out io.Writer) error {
	b := []byte("Sieve relation (")
	for i, c := range r.innerCols {
		if i > 0 {
			b = append(b, ',')
		}
		if c {
			b = append(b, '1')
		} else {
			b = append(b, '0')
		}
	}
	b = append(b, ")\n"...)
	if _, err := out.Write(b); err != nil {
		return err
	}
	return r.inner.Display(out)
}

var _ RelationBase = (*sieveRelation)(nil)
var _ RelationPlugin = (*SieveRelationPlugin)(nil)
var _ kindedRelationPlugin = (*SieveRelationPlugin)(nil)
