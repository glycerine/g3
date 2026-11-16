package rel

// Source: src/muz/rel/dl_product_relation.cpp product_relation_plugin and
// product_relation, and dl_relation_manager.h rel_spec_store.

import (
	"errors"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// relSpecStore is rel_spec_store<rel_spec>: per signature, the kinds
// allocated for each specification.
type relSpecStore struct {
	parent    *ProductRelationPlugin
	allocated []int
	// assignment[sigKey] maps a spec key to its index; specs[sigKey] maps a
	// kind to its spec.
	assignment map[string]map[string]int
	specs      map[string]map[int][]int
}

func sortsKey(sorts []ast.ASTHandle) string {
	b := make([]byte, 0, len(sorts)*24)
	for _, s := range sorts {
		b = append(b, uitoa(s.Origin())...)
		b = append(b, ':')
		b = append(b, uitoa(uint64(s.Slot()))...)
		b = append(b, ':')
		b = append(b, uitoa(uint64(s.Generation()))...)
		b = append(b, ';')
	}
	return string(b)
}

func specKey(spec []int) string {
	b := make([]byte, 0, len(spec)*4)
	for _, k := range spec {
		b = append(b, uitoa(uint64(k))...)
		b = append(b, ',')
	}
	return string(b)
}

func (s *relSpecStore) containsSignature(sorts []ast.ASTHandle) bool {
	_, found := s.assignment[sortsKey(sorts)]
	return found
}

// getRelationKind is rel_spec_store::get_relation_kind.
func (s *relSpecStore) getRelationKind(sorts []ast.ASTHandle, spec []int) int {
	key := sortsKey(sorts)
	ids, found := s.assignment[key]
	if !found {
		ids = make(map[string]int)
		s.assignment[key] = ids
		s.specs[key] = make(map[int][]int)
	}
	sk := specKey(spec)
	idx, found := ids[sk]
	if !found {
		idx = len(ids)
		if idx == len(s.allocated) {
			// add_new_kind
			s.allocated = append(s.allocated, s.parent.manager.claimRelationKind(s.parent))
		}
		ids[sk] = idx
		s.specs[key][s.allocated[idx]] = append([]int(nil), spec...)
	}
	return s.allocated[idx]
}

// getRelationSpec is rel_spec_store::get_relation_spec.
func (s *relSpecStore) getRelationSpec(sorts []ast.ASTHandle, kind int) []int {
	return append([]int(nil), s.specs[sortsKey(sorts)][kind]...)
}

// ProductRelationPlugin is product_relation_plugin.
type ProductRelationPlugin struct {
	RelationPluginDefaults
	manager *RelationManager
	kind    int
	store   relSpecStore
}

// productPlugin is product_relation_plugin::get_plugin(rmgr): registered
// on first use.
func (m *RelationManager) productPlugin() *ProductRelationPlugin {
	if p, ok := m.GetRelationPlugin("product_relation").(*ProductRelationPlugin); ok {
		return p
	}
	p := &ProductRelationPlugin{manager: m}
	p.store = relSpecStore{parent: p, assignment: make(map[string]map[string]int), specs: make(map[string]map[int][]int)}
	_ = m.RegisterRelationPlugin(p)
	return p
}

// ProductRelationKind is rel_context::set_predicate_representation's
// product_relation_plugin::get_plugin(rmgr).get_relation_kind(sig, kinds).
func (m *RelationManager) ProductRelationKind(sorts []ast.ASTHandle, kinds []int) int {
	return m.productPlugin().store.getRelationKind(sorts, kinds)
}

func (p *ProductRelationPlugin) initializeKind(fid int) {
	p.kind = fid
	p.store.allocated = append(p.store.allocated, fid)
}

func (p *ProductRelationPlugin) Name() string              { return "product_relation" }
func (p *ProductRelationPlugin) Manager() *RelationManager { return p.manager }

func (p *ProductRelationPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	return p.store.containsSignature(sorts), nil
}

// CanHandleSignatureKind is can_handle_signature(s, kind). z3 4.7.1
// returns true for every signature, and mk_empty(s, kind) then
// dereferences the missing spec of a signature the kind was not made for
// (a predicate's kind reaching a register of another signature, e.g.
// mk_unary_singleton), which crashes. g3 handles only the signatures the
// kind's spec exists for; the others take the remaining
// mk_empty_relation selection (z3 bug, fork fix pending).
func (p *ProductRelationPlugin) CanHandleSignatureKind(sorts []ast.ASTHandle, kind int) (bool, error) {
	_, found := p.store.specs[sortsKey(sorts)][kind]
	return found, nil
}

func (p *ProductRelationPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	return p.newProduct(sorts, nil), nil
}

func (p *ProductRelationPlugin) MkEmptyKind(sorts []ast.ASTHandle, kind int) (RelationBase, error) {
	spec := p.store.getRelationSpec(sorts, kind)
	inner := make([]RelationBase, 0, len(spec))
	for _, k := range spec {
		r, err := p.manager.mkEmptyRelationKind(sorts, k)
		if err != nil {
			return nil, errors.Join(err, closeRelationBases(inner))
		}
		inner = append(inner, r)
	}
	return p.newProduct(sorts, inner), nil
}

func (p *ProductRelationPlugin) MkFull(_ ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	return p.newProduct(sorts, nil), nil
}

func (p *ProductRelationPlugin) MkFullKind(predicate ast.ASTHandle, sorts []ast.ASTHandle, kind int) (RelationBase, error) {
	if kind == nullRelationKind || !p.store.containsSignature(sorts) {
		r := p.newProduct(sorts, nil)
		r.defaultEmpty = false
		return r, nil
	}
	spec := p.store.getRelationSpec(sorts, kind)
	inner := make([]RelationBase, 0, len(spec))
	for _, k := range spec {
		r, err := p.manager.mkFullRelationKind(sorts, predicate, k)
		if err != nil {
			return nil, errors.Join(err, closeRelationBases(inner))
		}
		inner = append(inner, r)
	}
	return p.newProduct(sorts, inner), nil
}

func closeRelationBases(rels []RelationBase) (result error) {
	for _, r := range rels {
		if r != nil {
			result = errors.Join(result, r.Close())
		}
	}
	return result
}

func isProductRelation(r RelationBase) (*productRelation, bool) {
	pr, ok := r.(*productRelation)
	return pr, ok
}

// kindOf is relation_base::get_kind().
func (p *ProductRelationPlugin) kindOf(r RelationBase) int {
	return p.manager.RelationKindOf(r)
}

func (p *ProductRelationPlugin) areAligned(r1, r2 *productRelation) bool {
	if len(r1.relations) != len(r2.relations) {
		return false
	}
	for i := range r1.relations {
		if p.kindOf(r1.relations[i]) != p.kindOf(r2.relations[i]) {
			return false
		}
	}
	return true
}

// getCommonSpec is product_relation_plugin::get_common_spec.
func getCommonSpec(rels []*productRelation) []int {
	specs := make([][]int, 0, len(rels))
	for _, r := range rels {
		s := append([]int(nil), r.spec...)
		sort.Ints(s)
		specs = append(specs, s)
	}
	var res []int
	for {
		next := -1
		for _, s := range specs {
			if len(s) != 0 && s[len(s)-1] > next {
				next = s[len(s)-1]
			}
		}
		if next == -1 {
			break
		}
		res = append(res, next)
		for i := range specs {
			for len(specs[i]) != 0 && specs[i][len(specs[i])-1] == next {
				specs[i] = specs[i][:len(specs[i])-1]
			}
		}
	}
	return res
}

// ---------------------------------------------------------------------------
// join_fn

type productJoinKind uint8

const (
	productInput productJoinKind = iota
	productFull
)

// nonsieveRelation is join_fn::get_nonsieve_relation.
func nonsieveRelation(r RelationBase) RelationBase {
	return sieveInner(r)
}

// nonsievePlugin is join_fn::get_nonsieve_plugin.
func nonsievePlugin(r RelationBase) RelationPlugin {
	return nonsieveRelation(r).RelationPlugin()
}

// nonsieveKind is join_fn::get_nonsieve_kind.
func (p *ProductRelationPlugin) nonsieveKind(r RelationBase) int {
	return p.kindOf(nonsieveRelation(r))
}

// isTableish is join_fn::is_tableish_relation: a table_relation or a
// sieve_relation with a table_relation inside.
func isTableish(r RelationBase) bool {
	_, ok := nonsievePlugin(r).(*TableRelationPlugin)
	return ok
}

func (p *ProductRelationPlugin) MkJoinFn(r1, r2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	_, p1 := isProductRelation(r1)
	_, p2 := isProductRelation(r2)
	if !p1 && !p2 && p.kindOf(r1) == p.kindOf(r2) {
		return nil, nil
	}
	rels := func(r RelationBase) []RelationBase {
		if pr, ok := isProductRelation(r); ok {
			return pr.relations
		}
		return []RelationBase{r}
	}
	return p.newJoinFn(r1.RelationSignature(), rels(r1), r2.RelationSignature(), rels(r2), cols1, cols2)
}

type productJoin struct {
	joins            []RelationJoinFn
	full             []RelationBase
	offset1, offset2 []int
	kind1, kind2     []productJoinKind
}

// newJoinFn is join_fn's constructor and init.
func (p *ProductRelationPlugin) newJoinFn(sig1 []ast.ASTHandle, r1 []RelationBase, sig2 []ast.ASTHandle, r2 []RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	m := p.manager
	j := &productJoin{}
	bv := make([]bool, len(r2))
	var r1Tables, r2Tables []int
	push := func(o1 int, k1 productJoinKind, o2 int, k2 productJoinKind, a, b RelationBase) error {
		fn, err := m.MkRelationJoinFn(a, b, cols1, cols2, false)
		if err != nil {
			return err
		}
		if fn == nil {
			return unsupportedRelationOperation("join", a.RelationPlugin().Name(), b.RelationPlugin().Name())
		}
		j.offset1, j.kind1, j.offset2, j.kind2 = append(j.offset1, o1), append(j.kind1, k1), append(j.offset2, o2), append(j.kind2, k2)
		j.joins = append(j.joins, fn)
		return nil
	}
	fullOf := func(plugin RelationPlugin, kind int, sig []ast.ASTHandle) (RelationBase, error) {
		ok, err := plugin.CanHandleSignature(sig)
		if err != nil {
			return nil, err
		}
		if !ok {
			return p.manager.sievePlugin().full(ast.ASTHandle{}, sig, plugin)
		}
		if k, isKinded := plugin.(kindedRelationPlugin); isKinded {
			return k.MkFullKind(ast.ASTHandle{}, sig, kind)
		}
		return plugin.MkFull(ast.ASTHandle{}, sig)
	}
	for i, a := range r1 {
		if isTableish(a) {
			r1Tables = append(r1Tables, i)
			continue
		}
		kind := p.nonsieveKind(a)
		found := false
		for k := 0; !found && k < len(r2); k++ {
			if kind == p.nonsieveKind(r2[k]) {
				bv[k] = true
				if err := push(i, productInput, k, productInput, a, r2[k]); err != nil {
					return nil, j.close(err)
				}
				found = true
			}
		}
		if !found {
			rel2, err := fullOf(nonsievePlugin(a), kind, sig2)
			if err != nil {
				return nil, j.close(err)
			}
			idx := len(j.full)
			j.full = append(j.full, rel2)
			if err = push(i, productInput, idx, productFull, a, rel2); err != nil {
				return nil, j.close(err)
			}
		}
	}
	for i, b := range r2 {
		if isTableish(b) {
			r2Tables = append(r2Tables, i)
			continue
		}
		if !bv[i] {
			rel1, err := fullOf(nonsievePlugin(b), p.nonsieveKind(b), sig1)
			if err != nil {
				return nil, j.close(err)
			}
			idx := len(j.full)
			j.full = append(j.full, rel1)
			if err = push(idx, productFull, i, productInput, rel1, b); err != nil {
				return nil, j.close(err)
			}
		}
	}
	pop := func(s *[]int) int {
		v := (*s)[len(*s)-1]
		*s = (*s)[:len(*s)-1]
		return v
	}
	switch {
	case len(r1Tables) != 0 && len(r2Tables) != 0:
		i1, i2 := pop(&r1Tables), pop(&r2Tables)
		for {
			if err := push(i1, productInput, i2, productInput, r1[i1], r2[i2]); err != nil {
				return nil, j.close(err)
			}
			if len(r1Tables) == 0 && len(r2Tables) == 0 {
				break
			}
			if len(r2Tables) != 0 {
				i2 = pop(&r2Tables)
			}
			if len(r1Tables) != 0 {
				i1 = pop(&r1Tables)
			}
		}
	case len(r2Tables) != 0:
		for len(r2Tables) != 0 {
			i2 := pop(&r2Tables)
			full, err := p.fullTableishRelation(sig1, p.nonsieveKind(r2[i2]))
			if err != nil {
				return nil, j.close(err)
			}
			idx := len(j.full)
			j.full = append(j.full, full)
			if err = push(idx, productFull, i2, productInput, full, r2[i2]); err != nil {
				return nil, j.close(err)
			}
		}
	case len(r1Tables) != 0:
		for len(r1Tables) != 0 {
			i1 := pop(&r1Tables)
			full, err := p.fullTableishRelation(sig2, p.nonsieveKind(r1[i1]))
			if err != nil {
				return nil, j.close(err)
			}
			idx := len(j.full)
			j.full = append(j.full, full)
			if err = push(i1, productInput, idx, productFull, r1[i1], full); err != nil {
				return nil, j.close(err)
			}
		}
	}
	sorts := joinSorts(sig1, sig2)
	p.manager.fnOwners = append(p.manager.fnOwners, j.full...)
	return func(a, b RelationBase) (RelationBase, error) {
		access := func(idx int, r RelationBase) RelationBase {
			if pr, ok := isProductRelation(r); ok {
				return pr.relations[idx]
			}
			return r
		}
		relations := make([]RelationBase, 0, len(j.joins))
		for i, fn := range j.joins {
			var x, y RelationBase
			if j.kind1[i] == productFull {
				x = j.full[j.offset1[i]]
			} else {
				x = access(j.offset1[i], a)
			}
			if j.kind2[i] == productFull {
				y = j.full[j.offset2[i]]
			} else {
				y = access(j.offset2[i], b)
			}
			r, err := fn(x, y)
			if err != nil {
				return nil, errors.Join(err, closeRelationBases(relations))
			}
			relations = append(relations, r)
		}
		return p.newProduct(sorts, relations), nil
	}, nil
}

func (j *productJoin) close(cause error) error {
	return errors.Join(cause, closeRelationBases(j.full))
}

// fullTableishRelation is join_fn::get_full_tableish_relation.
func (p *ProductRelationPlugin) fullTableishRelation(sig []ast.ASTHandle, kind int) (RelationBase, error) {
	m := p.manager
	if signature, ok := m.signatureIsTable(sig); ok {
		// rmgr.get_appropriate_plugin(tsig).mk_full(p, tsig, kind)
		plugin := m.appropriateTableRelationPlugin(signature)
		if plugin == nil {
			return nil, ErrRelationNoPlugin
		}
		return plugin.MkFull(ast.ASTHandle{}, sig)
	}
	var domains []uint64
	for _, sort := range sig {
		if domain, err := m.sortDomain(sort); err == nil {
			domains = append(domains, domain)
		}
	}
	// table_plugin & tplugin = rmgr.get_appropriate_plugin(tsig);
	// relation_plugin & inner_plugin = rmgr.get_table_relation_plugin(tplugin);
	innerPlugin := m.appropriateTableRelationPlugin(Signature{domains: domains})
	if innerPlugin == nil {
		return nil, ErrRelationNoPlugin
	}
	return m.sievePlugin().full(ast.ASTHandle{}, sig, innerPlugin)
}

// appropriateTableRelationPlugin is get_table_relation_plugin of
// get_appropriate_plugin(table_signature): the favourite table plugin
// (default_table), then the table plugins in registration order.
func (m *RelationManager) appropriateTableRelationPlugin(signature Signature) *TableRelationPlugin {
	m.initRelationKinds()
	if name := m.defaultFinite; name != "" {
		if c := m.tableRelationPluginNamed(name); c != nil && c.canHandle(signature) {
			return c
		}
	}
	for _, c := range m.tableRelationPlugins {
		if c.canHandle(signature) {
			return c
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// transform_fn, project, rename

func (p *ProductRelationPlugin) transformFn(sorts []ast.ASTHandle, transforms []RelationTransformerFn) RelationTransformerFn {
	return func(a RelationBase) (RelationBase, error) {
		r, ok := isProductRelation(a)
		if !ok {
			return nil, ErrInstruction
		}
		relations := make([]RelationBase, 0, len(r.relations))
		for i, inner := range r.relations {
			out, err := transforms[i](inner)
			if err != nil {
				return nil, errors.Join(err, closeRelationBases(relations))
			}
			relations = append(relations, out)
		}
		return p.newProduct(sorts, relations), nil
	}
}

func (p *ProductRelationPlugin) MkProjectFn(t RelationBase, removed []int) (RelationTransformerFn, error) {
	r, ok := isProductRelation(t)
	if !ok {
		return nil, nil
	}
	projs := make([]RelationTransformerFn, len(r.relations))
	for i, inner := range r.relations {
		fn, err := p.manager.MkRelationProjectFn(inner, removed)
		if err != nil {
			return nil, err
		}
		if fn == nil {
			return nil, unsupportedRelationOperationOne("project", inner.RelationPlugin().Name())
		}
		projs[i] = fn
	}
	return p.transformFn(projectSorts(t.RelationSignature(), removed), projs), nil
}

func (p *ProductRelationPlugin) MkRenameFn(t RelationBase, cycle []int) (RelationTransformerFn, error) {
	r, ok := isProductRelation(t)
	if !ok {
		return nil, nil
	}
	trans := make([]RelationTransformerFn, len(r.relations))
	for i, inner := range r.relations {
		fn, err := p.manager.MkRelationRenameFn(inner, cycle)
		if err != nil {
			return nil, err
		}
		if fn == nil {
			return nil, unsupportedRelationOperationOne("rename", inner.RelationPlugin().Name())
		}
		trans[i] = fn
	}
	return p.transformFn(renameSorts(t.RelationSignature(), cycle), trans), nil
}

// ---------------------------------------------------------------------------
// unions

type alignedUnion struct {
	p      *ProductRelationPlugin
	widen  bool
	unions [][]RelationUnionFn
}

func (p *ProductRelationPlugin) newAlignedUnion(tgt, src, delta *productRelation, widen bool) (*alignedUnion, error) {
	u := &alignedUnion{p: p, widen: widen}
	m := p.manager
	for i := range tgt.relations {
		var d RelationBase
		if delta != nil {
			d = delta.relations[i]
		}
		row := make([]RelationUnionFn, len(src.relations))
		for j := range src.relations {
			var fn RelationUnionFn
			var err error
			if widen {
				fn, err = m.MkRelationWidenFn(tgt.relations[i], src.relations[j], d)
			} else {
				fn, err = m.MkRelationUnionFn(tgt.relations[i], src.relations[j], d)
			}
			if err != nil {
				return nil, err
			}
			row[j] = fn
		}
		u.unions = append(u.unions, row)
	}
	return u, nil
}

func (u *alignedUnion) can(i, j int) bool { return u.unions[i][j] != nil }

// doDestructiveIntersection: a nil tgt stands for the full relation.
func (u *alignedUnion) doDestructiveIntersection(tgt *RelationBase, src *RelationBase) error {
	if *src == nil {
		return nil
	}
	if *tgt == nil {
		*tgt, *src = *src, nil
		return nil
	}
	err := u.doIntersection(*tgt, *src)
	err = errors.Join(err, (*src).Close())
	*src = nil
	return err
}

func (u *alignedUnion) doIntersection(tgt, src RelationBase) error {
	fn, err := u.p.manager.MkRelationFilterByIntersectionAllFn(tgt, src)
	if err != nil {
		return err
	}
	if fn == nil {
		return diagnostic.BorrowProcessWarningService().Warningf("%s", "intersection does not exist")
	}
	return fn(tgt, src)
}

func (u *alignedUnion) apply(tgtR, srcR, deltaR RelationBase) (resultErr error) {
	tgt, _ := isProductRelation(tgtR)
	src, _ := isProductRelation(srcR)
	var delta *productRelation
	if deltaR != nil {
		delta, _ = isProductRelation(deltaR)
	}
	if tgt == nil || src == nil {
		return ErrInstruction
	}
	num := len(tgt.relations)
	// The fork reports a missing inner union i,i before any work (FORK
	// FIX, see FORK_BUGFIXES.md): z3 4.7.1's VERIFY(can_do_inner_union(i,i))
	// only printed "Failed to verify" and called the null union.
	for i := 0; i < num; i++ {
		if !u.can(i, i) {
			return unsupportedRelationOperation("union", tgt.relations[i].RelationPlugin().Name(), src.relations[i].RelationPlugin().Name())
		}
	}
	sideResults := make([]RelationBase, num)
	sideDeltas := make([]RelationBase, num)
	defer func() {
		resultErr = errors.Join(resultErr, closeRelationBases(sideResults), closeRelationBases(sideDeltas))
	}()
	freshDelta := func(i int) (RelationBase, error) {
		if delta == nil {
			return nil, nil
		}
		return mkEmptyLike(delta.relations[i])
	}
	clone := func(r RelationBase) (RelationBase, error) {
		if r == nil {
			return nil, nil
		}
		return r.CloneRelation()
	}
	for i := 0; i < num; i++ {
		itgt := tgt.relations[i]
		fd, err := freshDelta(i)
		if err != nil {
			return err
		}
		var sideResult, sideDelta RelationBase
		for j := 0; j < num; j++ {
			if i == j || !(u.can(i, j) && u.can(j, i)) {
				continue
			}
			pairs := [2][2]RelationBase{{itgt, src.relations[j]}, {src.relations[i], tgt.relations[j]}}
			for _, pair := range pairs {
				oneSide, err := pair[0].CloneRelation()
				if err != nil {
					return errors.Join(err, closeRelationBases([]RelationBase{fd, sideResult, sideDelta}))
				}
				oneDelta, err := clone(fd)
				if err != nil {
					return errors.Join(err, oneSide.Close())
				}
				if err = u.unions[i][j](oneSide, pair[1], oneDelta); err != nil {
					return errors.Join(err, closeRelationBases([]RelationBase{oneSide, oneDelta}))
				}
				if err = u.doDestructiveIntersection(&sideResult, &oneSide); err != nil {
					return err
				}
				if oneDelta != nil {
					if err = u.doDestructiveIntersection(&sideDelta, &oneDelta); err != nil {
						return err
					}
				}
			}
		}
		if fd != nil {
			if err = fd.Close(); err != nil {
				return err
			}
		}
		sideResults[i], sideDeltas[i] = sideResult, sideDelta
	}
	for i := 0; i < num; i++ {
		itgt := tgt.relations[i]
		var idelta RelationBase
		if delta != nil {
			idelta = delta.relations[i]
		}
		fd, err := freshDelta(i)
		if err != nil {
			return err
		}
		if err = u.unions[i][i](itgt, src.relations[i], fd); err != nil {
			return errors.Join(err, closeRelationBases([]RelationBase{fd}))
		}
		if sideResults[i] != nil {
			if err = u.doIntersection(itgt, sideResults[i]); err != nil {
				return errors.Join(err, closeRelationBases([]RelationBase{fd}))
			}
		}
		if fd != nil {
			if err = u.doDestructiveIntersection(&fd, &sideDeltas[i]); err != nil {
				return err
			}
			// do_delta_union
			fn, err := u.p.manager.MkRelationUnionFn(idelta, fd, nil)
			if err == nil && fn != nil {
				err = fn(idelta, fd, nil)
			}
			if err = errors.Join(err, fd.Close()); err != nil {
				return err
			}
		}
	}
	if num == 0 {
		if !src.defaultEmpty && tgt.defaultEmpty {
			tgt.defaultEmpty = false
			if delta != nil {
				delta.defaultEmpty = false
			}
		}
	}
	return nil
}

func (p *ProductRelationPlugin) mkUnionWFn(target, source, delta RelationBase, widen bool) (RelationUnionFn, error) {
	tgt, tok := isProductRelation(target)
	src, sok := isProductRelation(source)
	var dp *productRelation
	dok := true
	if delta != nil {
		dp, dok = isProductRelation(delta)
	}
	if tok && sok && dok {
		if p.areAligned(tgt, src) && (dp == nil || p.areAligned(tgt, dp)) {
			u, err := p.newAlignedUnion(tgt, src, dp, widen)
			if err != nil {
				return nil, err
			}
			return u.apply, nil
		}
		// unaligned_union_fn
		rels := []*productRelation{tgt, src}
		if dp != nil {
			rels = append(rels, dp)
		}
		common := getCommonSpec(rels)
		var aligned *alignedUnion
		return func(t, s, d RelationBase) (resultErr error) {
			tp, _ := isProductRelation(t)
			sp0, _ := isProductRelation(s)
			var dpp *productRelation
			if d != nil {
				dpp, _ = isProductRelation(d)
			}
			if err := tp.convertSpec(common); err != nil {
				return err
			}
			if dpp != nil {
				if err := dpp.convertSpec(common); err != nil {
					return err
				}
			}
			sp := sp0
			if p.kindOf(sp0) != p.kindOf(tp) {
				c, err := sp0.CloneRelation()
				if err != nil {
					return err
				}
				sp = c.(*productRelation)
				defer func() { resultErr = errors.Join(resultErr, sp.Close()) }()
				if err = sp.convertSpec(common); err != nil {
					return err
				}
			}
			if aligned == nil {
				var err error
				if aligned, err = p.newAlignedUnion(tp, sp, dpp, widen); err != nil {
					return err
				}
			}
			var dr RelationBase
			if dpp != nil {
				dr = dpp
			}
			return aligned.apply(tp, sp, dr)
		}, nil
	}
	if sok {
		if idx, found := src.singleNonTransparent(); found {
			var inner RelationUnionFn
			var err error
			if widen {
				inner, err = p.manager.MkRelationWidenFn(target, src.relations[idx], delta)
			} else {
				inner, err = p.manager.MkRelationUnionFn(target, src.relations[idx], delta)
			}
			if err != nil {
				return nil, err
			}
			if inner != nil {
				return func(t, s, d RelationBase) error {
					sp, _ := isProductRelation(s)
					return inner(t, sp.relations[idx], d)
				}, nil
			}
		}
	}
	return nil, nil
}

func (p *ProductRelationPlugin) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.mkUnionWFn(target, source, delta, false)
}

func (p *ProductRelationPlugin) MkWidenFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.mkUnionWFn(target, source, delta, true)
}

// ---------------------------------------------------------------------------
// mutators

func (p *ProductRelationPlugin) mutatorFn(mutators []RelationMutatorFn) RelationMutatorFn {
	return func(a RelationBase) error {
		r, ok := isProductRelation(a)
		if !ok {
			return ErrInstruction
		}
		for i, m := range mutators {
			if m != nil {
				if err := m(r.relations[i]); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func (p *ProductRelationPlugin) MkFilterIdenticalFn(t RelationBase, cols []int) (RelationMutatorFn, error) {
	r, ok := isProductRelation(t)
	if !ok {
		return nil, nil
	}
	found := false
	mutators := make([]RelationMutatorFn, len(r.relations))
	for i, inner := range r.relations {
		m, err := p.manager.MkRelationFilterIdenticalFn(inner, cols)
		if err != nil {
			return nil, err
		}
		mutators[i] = m
		found = found || m != nil
	}
	if !found {
		return nil, nil
	}
	return p.mutatorFn(mutators), nil
}

func (p *ProductRelationPlugin) MkFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error) {
	r, ok := isProductRelation(t)
	if !ok {
		return nil, nil
	}
	found := false
	mutators := make([]RelationMutatorFn, len(r.relations))
	for i, inner := range r.relations {
		m, err := p.manager.MkRelationFilterEqualFn(inner, value, col)
		if err != nil {
			return nil, err
		}
		mutators[i] = m
		found = found || m != nil
	}
	if !found {
		return nil, nil
	}
	return p.mutatorFn(mutators), nil
}

// MkFilterInterpretedFn is filter_interpreted_fn; it dereferences every
// inner mutator, so a missing one is the unsupported operation. The bound
// relation's attachment of an interval relation has no effect on its
// filter.
func (p *ProductRelationPlugin) MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error) {
	r, ok := isProductRelation(t)
	if !ok {
		return nil, ErrInstruction
	}
	mutators := make([]RelationMutatorFn, len(r.relations))
	for i, inner := range r.relations {
		m, err := p.manager.MkRelationFilterInterpretedFn(inner, condition)
		if err != nil {
			return nil, err
		}
		if m == nil {
			return nil, unsupportedRelationOperationOne("filter_interpreted", inner.RelationPlugin().Name())
		}
		mutators[i] = m
	}
	return func(a RelationBase) error {
		r, ok := isProductRelation(a)
		if !ok {
			return ErrInstruction
		}
		for i, m := range mutators {
			if err := m(r.relations[i]); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

// ---------------------------------------------------------------------------
// product_relation

type productRelation struct {
	plugin       *ProductRelationPlugin
	sorts        []ast.ASTHandle
	defaultEmpty bool
	relations    []RelationBase
	spec         []int
	kind         int
	closed       bool
}

func (p *ProductRelationPlugin) newProduct(sorts []ast.ASTHandle, relations []RelationBase) *productRelation {
	r := &productRelation{plugin: p, sorts: append([]ast.ASTHandle(nil), sorts...), defaultEmpty: true, relations: relations, kind: p.kind}
	r.ensureCorrectKind()
	return r
}

// ensureCorrectKind is product_relation::ensure_correct_kind.
func (r *productRelation) ensureCorrectKind() {
	n := len(r.relations)
	changed := n != len(r.spec) || n == 0
	if changed {
		spec := make([]int, n)
		copy(spec, r.spec)
		r.spec = spec
	}
	for i, inner := range r.relations {
		k := r.plugin.kindOf(inner)
		changed = changed || r.spec[i] != k
		r.spec[i] = k
	}
	if changed {
		r.kind = r.plugin.store.getRelationKind(r.sorts, r.spec)
	}
}

// convertSpec is product_relation::convert_spec.
func (r *productRelation) convertSpec(spec []int) error {
	m := r.plugin.manager
	newKind := r.plugin.store.getRelationKind(r.sorts, spec)
	if newKind == r.kind {
		return nil
	}
	old := r.relations
	oldSize := len(old)
	newRels := make([]RelationBase, 0, len(spec))
	for _, ikind := range spec {
		var irel RelationBase
		for j := range old {
			if old[j] != nil && r.plugin.kindOf(old[j]) == ikind {
				irel, old[j] = old[j], nil
				break
			}
		}
		if irel == nil {
			var err error
			if oldSize == 0 && r.defaultEmpty {
				irel, err = m.mkEmptyRelationKind(r.sorts, ikind)
			} else {
				irel, err = m.mkFullRelationKind(r.sorts, ast.ASTHandle{}, ikind)
			}
			if err != nil {
				return errors.Join(err, closeRelationBases(newRels))
			}
		}
		newRels = append(newRels, irel)
	}
	r.relations = newRels
	r.kind = newKind
	r.spec = append([]int(nil), spec...)
	return nil
}

// singleNonTransparent is try_get_single_non_transparent: a sieve with no
// inner columns is transparent.
func (r *productRelation) singleNonTransparent() (int, bool) {
	found := false
	candidate := 0
	for i, inner := range r.relations {
		if sr, ok := isSieveRelation(inner); ok && sr.noInnerColumns() {
			continue
		}
		if found {
			return 0, false // we have more than one non-transparent
		}
		found = true
		candidate = i
	}
	return candidate, found
}

func (r *productRelation) RelationKind() int                  { return r.kind }
func (r *productRelation) RelationSignature() []ast.ASTHandle { return r.sorts }
func (r *productRelation) RelationPlugin() RelationPlugin     { return r.plugin }

func (r *productRelation) Signature() (Signature, error) {
	if s, ok := r.plugin.manager.signatureIsTable(r.sorts); ok {
		return s, nil
	}
	domains := make([]uint64, len(r.sorts))
	for i, sort := range r.sorts {
		d, err := r.plugin.manager.relationSortDomain(sort)
		if err != nil {
			return Signature{}, err
		}
		domains[i] = d
	}
	return Signature{domains: domains}, nil
}

func (r *productRelation) Facts() ([]Fact, error) { return nil, ErrRelationNotTable }

func (r *productRelation) Empty() (bool, error) {
	if len(r.relations) == 0 {
		return r.defaultEmpty, nil
	}
	for _, inner := range r.relations {
		empty, err := inner.Empty()
		if err != nil || empty {
			return empty, err
		}
	}
	return false, nil
}

func (r *productRelation) FastEmpty() bool {
	empty, _ := r.Empty()
	return empty
}

func (r *productRelation) IsPrecise() bool {
	for _, inner := range r.relations {
		if !inner.IsPrecise() {
			return false
		}
	}
	return true
}

func (r *productRelation) Len() (int, error) {
	empty, err := r.Empty()
	if empty {
		return 0, err
	}
	rows, err := r.SizeEstimateRows()
	return int(rows), err
}

func (r *productRelation) SizeEstimateRows() (uint32, error)  { return ^uint32(0), nil }
func (r *productRelation) SizeEstimateBytes() (uint32, error) { return ^uint32(0), nil }

// Add is product_relation::add_fact.
func (r *productRelation) Add(fact Fact) (bool, error) {
	for _, inner := range r.relations {
		if _, err := inner.Add(fact); err != nil {
			return false, err
		}
	}
	return true, nil
}

// AddFactAST is add_fact of an expression fact.
func (r *productRelation) AddFactAST(values []ast.ASTHandle) error {
	m := r.plugin.manager
	for _, inner := range r.relations {
		if adder, ok := inner.(RelationAddFactAST); ok {
			if err := adder.AddFactAST(values); err != nil {
				return err
			}
			continue
		}
		fact, err := m.EncodeFact(inner.RelationSignature(), values)
		if err != nil {
			return err
		}
		if _, err = inner.Add(fact); err != nil {
			return err
		}
	}
	return nil
}

func (r *productRelation) Contains(fact Fact) (bool, error) {
	for _, inner := range r.relations {
		ok, err := inner.Contains(fact)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// Reset is relation_base::reset: filter_interpreted(false).
func (r *productRelation) Reset() error {
	m := r.plugin.manager.manager
	f, err := m.MkFalse()
	if err != nil {
		return err
	}
	fn, err := r.plugin.MkFilterInterpretedFn(r, f)
	if err != nil {
		return err
	}
	return fn(r)
}

func (r *productRelation) CloneRelation() (RelationBase, error) {
	relations := make([]RelationBase, 0, len(r.relations))
	for _, inner := range r.relations {
		c, err := inner.CloneRelation()
		if err != nil {
			return nil, errors.Join(err, closeRelationBases(relations))
		}
		relations = append(relations, c)
	}
	return r.plugin.newProduct(r.sorts, relations), nil
}

func (r *productRelation) CloneFinite() (FiniteTable, error) { return r.CloneRelation() }

func (r *productRelation) Close() error {
	if r.closed {
		return ErrTable
	}
	r.closed = true
	return closeRelationBases(r.relations)
}

func (r *productRelation) ToFormula() (result ast.ASTRef, resultErr error) {
	m := r.plugin.manager.manager
	parts := make([]ast.ASTRef, 0, len(r.relations))
	defer func() { resultErr = errors.Join(resultErr, releaseTableFormulaRefs(m, parts)) }()
	handles := make([]ast.ASTHandle, 0, len(r.relations))
	for _, inner := range r.relations {
		f, err := inner.ToFormula()
		if err != nil {
			return ast.ASTRef{}, err
		}
		parts = append(parts, f)
		h, err := m.RefGet(f)
		if err != nil {
			return ast.ASTRef{}, err
		}
		handles = append(handles, h)
	}
	result, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, handles, result); err != nil {
		return result, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}

func (r *productRelation) Display(out io.Writer) error {
	if len(r.relations) == 0 {
		_, err := io.WriteString(out, "{}\n")
		return err
	}
	if _, err := io.WriteString(out, "Product of the following relations:\n"); err != nil {
		return err
	}
	for _, inner := range r.relations {
		if err := inner.Display(out); err != nil {
			return err
		}
	}
	return nil
}

var _ RelationBase = (*productRelation)(nil)
var _ RelationPlugin = (*ProductRelationPlugin)(nil)
var _ kindedRelationPlugin = (*ProductRelationPlugin)(nil)
