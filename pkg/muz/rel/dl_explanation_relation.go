package rel

// Source: src/muz/rel/dl_mk_explanations.cpp explanation_relation_plugin
// and explanation_relation (lines 31-586): the relation plugin that
// mk_explanations registers with the relation manager. A relation holds at
// most one row of rule-sort terms (relation_fact m_data); a null term is
// an undefined explanation.

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

var _ RelationPlugin = (*ExplanationRelationPlugin)(nil)
var _ RelationBase = (*explanationRelation)(nil)

// ExplanationRelationPlugin is explanation_relation_plugin.
type ExplanationRelationPlugin struct {
	RelationPluginDefaults
	manager *RelationManager
	ast     *ast.ASTManager
	// relationLevel is m_relation_level_explanations.
	relationLevel bool
	// unionDecl is m_union_decl (func_decl_ref).
	unionDecl ast.ASTRef
	// pool is m_pool: recycled relations by signature size; they keep
	// their m_data until mk_empty reuses them or the plugin is destroyed.
	pool   [][]*explanationRelation
	closed bool
}

// ExplanationRelationPluginName is explanation_relation_plugin::get_name.
func ExplanationRelationPluginName(relationLevel bool) string {
	if relationLevel {
		return "relation_explanation"
	}
	return "fact_explanation"
}

// NewExplanationRelationPlugin is the explanation_relation_plugin
// constructor: m_union_decl(mk_explanations::get_union_decl(ctx)).
func NewExplanationRelationPlugin(manager *RelationManager, relationLevel bool) (*ExplanationRelationPlugin, error) {
	if manager == nil {
		return nil, ErrTable
	}
	p := &ExplanationRelationPlugin{manager: manager, ast: manager.manager, relationLevel: relationLevel}
	decl, err := ExplanationUnionDecl(p.ast)
	if err != nil {
		return nil, err
	}
	if p.unionDecl, err = p.ast.NewRef(decl); err != nil {
		return nil, err
	}
	return p, nil
}

// ExplanationUnionDecl is mk_explanations::get_union_decl(ctx):
// e_union : rule rule -> rule.
func ExplanationUnionDecl(m *ast.ASTManager) (_ ast.ASTHandle, resultErr error) {
	s, err := m.MkDatalogRuleSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// sort_ref s(...): held while the declaration is made.
	held, err := m.NewRef(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&held)) }()
	name, err := m.Symbols.Intern("e_union")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkFuncDecl(name, []ast.ASTHandle{s, s}, s, ast.ASTFuncDeclInfo{})
}

func (p *ExplanationRelationPlugin) Name() string {
	return ExplanationRelationPluginName(p.relationLevel)
}
func (p *ExplanationRelationPlugin) Manager() *RelationManager { return p.manager }

// RelationLevel is m_relation_level_explanations.
func (p *ExplanationRelationPlugin) RelationLevel() bool { return p.relationLevel }

// Close is ~explanation_relation_plugin: the pooled relations are
// deallocated.
func (p *ExplanationRelationPlugin) Close() (result error) {
	if p.closed {
		return nil
	}
	p.closed = true
	for i := range p.pool {
		for _, r := range p.pool[i] {
			result = errors.Join(result, r.release())
		}
	}
	p.pool = nil
	return errors.Join(result, p.ast.DelRef(&p.unionDecl))
}

// isRuleSort is dl_decl_util::is_rule_sort.
func (p *ExplanationRelationPlugin) isRuleSort(sort ast.ASTHandle) (bool, error) {
	return p.ast.IsDatalogRuleSort(sort)
}

// CanHandleSignature: every column has the rule sort.
func (p *ExplanationRelationPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	for _, s := range sorts {
		ok, err := p.isRuleSort(s)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// MkEmpty is explanation_relation_plugin::mk_empty: a pooled relation of
// the signature size (m_empty = true, m_data.reset()) or a new one.
func (p *ExplanationRelationPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	return p.mkEmpty(sorts)
}

func (p *ExplanationRelationPlugin) mkEmpty(sorts []ast.ASTHandle) (*explanationRelation, error) {
	n := len(sorts)
	if len(p.pool) > n && len(p.pool[n]) != 0 {
		r := p.pool[n][len(p.pool[n])-1]
		p.pool[n] = p.pool[n][:len(p.pool[n])-1]
		r.empty = true
		// The pooled relation keeps the signature it was made with; the
		// C++ reuses it for any signature of this size (all columns are
		// rule sorts).
		r.sorts = append(r.sorts[:0], sorts...)
		r.closed = false
		return r, r.resetData()
	}
	return &explanationRelation{plugin: p, sorts: append([]ast.ASTHandle(nil), sorts...), empty: true}, nil
}

// recycle is explanation_relation_plugin::recycle.
func (p *ExplanationRelationPlugin) recycle(r *explanationRelation) {
	n := len(r.sorts)
	for len(p.pool) <= n {
		p.pool = append(p.pool, nil)
	}
	p.pool[n] = append(p.pool[n], r)
}

// MkFull is relation_plugin::mk_full(p, s): mk_empty, complement.
func (p *ExplanationRelationPlugin) MkFull(predicate ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	aux, err := p.mkEmpty(sorts)
	if err != nil {
		return nil, err
	}
	res, err := aux.Complement(predicate)
	err = errors.Join(err, aux.Close())
	if err != nil {
		if res != nil {
			err = errors.Join(err, res.Close())
		}
		return nil, err
	}
	return res, nil
}

// get is the static_cast with &r.get_plugin()==this (check_kind).
func (p *ExplanationRelationPlugin) get(r RelationBase) (*explanationRelation, bool) {
	x, ok := r.(*explanationRelation)
	return x, ok && x.plugin == p
}

// mkUnion is mk_union(a1, a2): e_union(a1, a2).
func (p *ExplanationRelationPlugin) mkUnion(a1, a2 ast.ASTHandle) (ast.ASTHandle, error) {
	decl, err := p.ast.RefGet(p.unionDecl)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.ast.MkApp(decl, []ast.ASTHandle{a1, a2})
}

// MkJoinFn is mk_join_fn: only a product (no joined columns) of two
// explanation relations.
func (p *ExplanationRelationPlugin) MkJoinFn(r1, r2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	if _, ok := p.get(r1); !ok {
		return nil, nil
	}
	if _, ok := p.get(r2); !ok {
		return nil, nil
	}
	if len(cols1) != 0 {
		return nil, nil
	}
	sorts := joinSorts(r1.RelationSignature(), r2.RelationSignature())
	return func(a, b RelationBase) (RelationBase, error) {
		x, ok1 := p.get(a)
		y, ok2 := p.get(b)
		if !ok1 || !ok2 {
			return nil, ErrInstruction
		}
		res, err := p.mkEmpty(sorts)
		if err != nil {
			return nil, err
		}
		if !x.empty && !y.empty {
			res.empty = false
			if err = res.appendData(x.data); err == nil {
				err = res.appendData(y.data)
			}
			if err != nil {
				return nil, errors.Join(err, res.Close())
			}
		}
		return res, nil
	}, nil
}

// MkProjectFn is mk_project_fn (project_fn: project_out_vector_columns).
func (p *ExplanationRelationPlugin) MkProjectFn(r RelationBase, removed []int) (RelationTransformerFn, error) {
	if _, ok := p.get(r); !ok {
		return nil, nil
	}
	sorts := projectSorts(r.RelationSignature(), removed)
	removedCols := append([]int(nil), removed...)
	return func(a RelationBase) (RelationBase, error) {
		x, ok := p.get(a)
		if !ok {
			return nil, ErrInstruction
		}
		res, err := p.mkEmpty(sorts)
		if err != nil {
			return nil, err
		}
		if !x.empty {
			if err = res.assignData(projectRefs(x.data, removedCols)); err != nil {
				return nil, errors.Join(err, res.Close())
			}
		}
		return res, nil
	}, nil
}

// projectRefs is project_out_vector_columns over the data terms.
func projectRefs(data []ast.ASTRef, removed []int) []ast.ASTRef {
	res := make([]ast.ASTRef, 0, len(data))
	next := 0
	for i, d := range data {
		if next < len(removed) && removed[next] == i {
			next++
			continue
		}
		res = append(res, d)
	}
	return res
}

// MkRenameFn is mk_rename_fn (rename_fn: permutate_by_cycle). The C++
// makes it without check_kind.
func (p *ExplanationRelationPlugin) MkRenameFn(r RelationBase, cycle []int) (RelationTransformerFn, error) {
	sorts := renameSorts(r.RelationSignature(), cycle)
	cyc := append([]int(nil), cycle...)
	return func(a RelationBase) (RelationBase, error) {
		x, ok := p.get(a)
		if !ok {
			return nil, ErrInstruction
		}
		res, err := p.mkEmpty(sorts)
		if err != nil {
			return nil, err
		}
		if !x.empty {
			permuted := append([]ast.ASTRef(nil), x.data...)
			if len(cyc) >= 2 {
				first := permuted[cyc[0]]
				for i := 0; i+1 < len(cyc); i++ {
					permuted[cyc[i]] = permuted[cyc[i+1]]
				}
				permuted[cyc[len(cyc)-1]] = first
			}
			if err = res.assignData(permuted); err != nil {
				return nil, errors.Join(err, res.Close())
			}
		}
		return res, nil
	}, nil
}

// MkUnionFn is mk_union_fn: union_fn for two explanation relations, the
// foreign_union_fn for a source of another plugin ("this is to handle the
// product relation").
func (p *ExplanationRelationPlugin) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	if _, ok := p.get(target); !ok {
		return nil, nil
	}
	if delta != nil {
		if _, ok := p.get(delta); !ok {
			return nil, nil
		}
	}
	if _, ok := p.get(source); !ok {
		return p.foreignUnion, nil
	}
	u := &explanationUnionFn{plugin: p}
	return u.apply, nil
}

// explanationUnionFn is explanation_relation_plugin::union_fn.
type explanationUnionFn struct {
	plugin *ExplanationRelationPlugin
	// deltaUnion is m_delta_union_fun.
	deltaUnion RelationUnionFn
}

func (u *explanationUnionFn) apply(tgt0, src0, delta0 RelationBase) error {
	p := u.plugin
	tgt, ok1 := p.get(tgt0)
	src, ok2 := p.get(src0)
	if !ok1 || !ok2 {
		return ErrInstruction
	}
	var delta *explanationRelation
	if delta0 != nil {
		if delta, ok1 = p.get(delta0); !ok1 {
			return ErrInstruction
		}
	}
	if !src.noUndefined() || !tgt.noUndefined() || (delta != nil && !delta.noUndefined()) {
		// UNREACHABLE()
		return fmt.Errorf("%w: explanation union of undefined explanations", ErrInstruction)
	}
	if src.empty {
		return nil
	}
	if p.relationLevel {
		if err := tgt.uniteWithData(src.data); err != nil {
			return err
		}
		if delta != nil {
			if u.deltaUnion == nil {
				fn, err := p.manager.MkRelationUnionFn(delta, src, nil)
				if err != nil {
					return err
				}
				if fn == nil {
					return ErrInstruction
				}
				u.deltaUnion = fn
			}
			return u.deltaUnion(delta, src, nil)
		}
		return nil
	}
	if tgt.empty {
		if err := tgt.assignData(src.data); err != nil {
			return err
		}
		if delta != nil && delta.empty {
			return delta.assignData(src.data)
		}
	}
	return nil
}

// foreignUnion is foreign_union_fn: a nonempty source of another plugin
// makes the target (and the delta) undefined.
func (p *ExplanationRelationPlugin) foreignUnion(tgt0, src, delta0 RelationBase) error {
	tgt, ok := p.get(tgt0)
	if !ok {
		return ErrInstruction
	}
	empty, err := src.Empty()
	if err != nil || empty {
		return err
	}
	if err = tgt.setUndefined(); err != nil {
		return err
	}
	if delta0 != nil {
		delta, ok := p.get(delta0)
		if !ok {
			return ErrInstruction
		}
		return delta.setUndefined()
	}
	return nil
}

// MkFilterInterpretedFn is mk_filter_interpreted_fn: the
// assignment_filter_fn of an equality between a rule-sort column variable
// and an application.
func (p *ExplanationRelationPlugin) MkFilterInterpretedFn(r RelationBase, cond ast.ASTHandle) (RelationMutatorFn, error) {
	if _, ok := p.get(r); !ok {
		return nil, nil
	}
	m := p.ast
	family, op, arg1, arg2, binary, err := intervalParts(m, cond)
	if err != nil || !binary || family != m.BasicFamilyID() || op != ast.ASTOpEq {
		return nil, err
	}
	if k, err := m.GetKind(arg2); err != nil {
		return nil, err
	} else if k == ast.ASTVar {
		arg1, arg2 = arg2, arg1
	}
	k1, err := m.GetKind(arg1)
	if err != nil {
		return nil, err
	}
	k2, err := m.GetKind(arg2)
	if err != nil {
		return nil, err
	}
	if k1 != ast.ASTVar || k2 != ast.ASTApp {
		return nil, nil
	}
	sort, err := m.GetSort(arg1)
	if err != nil {
		return nil, err
	}
	if ok, err := p.isRuleSort(sort); err != nil || !ok {
		return nil, err
	}
	col, err := m.GetVarIndex(arg1)
	if err != nil {
		return nil, err
	}
	newRule, err := m.NewRef(arg2)
	if err != nil {
		return nil, err
	}
	// The functor owns m_new_rule; the relation manager keeps it for the
	// manager's lifetime like the other functors' declarations.
	p.manager.fnRoots = append(p.manager.fnRoots, newRule)
	return func(r0 RelationBase) error {
		return p.assignmentFilter(r0, int(col), arg2)
	}, nil
}

// assignmentFilter is assignment_filter_fn::operator().
func (p *ExplanationRelationPlugin) assignmentFilter(r0 RelationBase, col int, newRule ast.ASTHandle) (resultErr error) {
	r, ok := p.get(r0)
	if !ok {
		return ErrInstruction
	}
	if col >= len(r.data) || !r.isUndefined(col) {
		// UNREACHABLE()
		return fmt.Errorf("%w: explanation assignment to a defined column", ErrInstruction)
	}
	sz := len(r.sorts)
	subst := make([]ast.ASTHandle, sz)
	ofs := sz - 1
	for i := 0; i < sz; i++ {
		if r.data[i] == (ast.ASTRef{}) {
			continue
		}
		h, err := p.ast.RefGet(r.data[i])
		if err != nil {
			return err
		}
		subst[ofs-i] = h
	}
	res, err := p.ast.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.ast.DelRef(&res)) }()
	if err = p.manager.varSubst.Substitute(newRule, subst, res); err != nil {
		return err
	}
	h, err := p.ast.RefGet(res)
	if err != nil {
		return err
	}
	return r.setData(col, h)
}

// MkFilterByNegationFn is mk_filter_by_negation_fn (negation_filter_fn).
func (p *ExplanationRelationPlugin) MkFilterByNegationFn(r, neg RelationBase, tCols, negCols []int) (RelationIntersectionFilterFn, error) {
	if _, ok := p.get(r); !ok {
		return nil, nil
	}
	if _, ok := p.get(neg); !ok {
		return nil, nil
	}
	return func(r0, neg0 RelationBase) error {
		empty, err := neg0.Empty()
		if err != nil || empty {
			return err
		}
		return r0.Reset()
	}, nil
}

// MkFilterByIntersectionFn is mk_filter_by_intersection_fn
// (intersection_filter_fn) for a one-to-one join on all columns.
func (p *ExplanationRelationPlugin) MkFilterByIntersectionFn(tgt, src RelationBase, tgtCols, srcCols []int) (RelationIntersectionFilterFn, error) {
	if _, ok := p.get(tgt); !ok {
		return nil, nil
	}
	if _, ok := p.get(src); !ok {
		return nil, nil
	}
	ts, ss := tgt.RelationSignature(), src.RelationSignature()
	if !sameSorts(ts, ss) || len(tgtCols) != len(ts) || !intsEqual(tgtCols, srcCols) {
		return nil, nil
	}
	// counter ctr; ctr.count(joined_col_cnt, tgt_cols)
	counts := make(map[int]int)
	maxCount, maxPositive := 0, -1
	for _, c := range tgtCols {
		counts[c]++
		if counts[c] > maxCount {
			maxCount = counts[c]
		}
		if c > maxPositive {
			maxPositive = c
		}
	}
	if maxCount > 1 || (len(tgtCols) != 0 && maxPositive != len(tgtCols)-1) {
		return nil, nil
	}
	return p.intersectionFilter, nil
}

// intersectionFilter is intersection_filter_fn::operator().
func (p *ExplanationRelationPlugin) intersectionFilter(tgt0, src0 RelationBase) error {
	tgt, ok1 := p.get(tgt0)
	src, ok2 := p.get(src0)
	if !ok1 || !ok2 {
		return ErrInstruction
	}
	if src.empty {
		return tgt.Reset()
	}
	if tgt.empty {
		return nil
	}
	unionDecl, err := p.ast.RefGet(p.unionDecl)
	if err != nil {
		return err
	}
	for i := range tgt.sorts {
		if src.isUndefined(i) {
			continue
		}
		currSrc, err := p.ast.RefGet(src.data[i])
		if err != nil {
			return err
		}
		if tgt.isUndefined(i) {
			if err = tgt.setData(i, currSrc); err != nil {
				return err
			}
			continue
		}
		currTgt, err := p.ast.RefGet(tgt.data[i])
		if err != nil {
			return err
		}
		decl, err := p.ast.GetDecl(currTgt)
		if err != nil {
			return err
		}
		if decl == unionDecl {
			a0, err := p.ast.GetArg(currTgt, 0)
			if err != nil {
				return err
			}
			a1, err := p.ast.GetArg(currTgt, 1)
			if err != nil {
				return err
			}
			if a0 == currSrc || a1 == currSrc {
				if err = tgt.setData(i, currSrc); err != nil {
					return err
				}
				continue
			}
		}
		// the intersection is imprecise because we do nothing here, but it
		// is good enough for the purpose of explanations
	}
	return nil
}

func sameSorts(a, b []ast.ASTHandle) bool {
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

func intsEqual(a, b []int) bool {
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
// explanation_relation

// explanationRelation is explanation_relation.
type explanationRelation struct {
	plugin *ExplanationRelationPlugin
	sorts  []ast.ASTHandle
	// empty is m_empty.
	empty bool
	// data is m_data (relation_fact: an app_ref_vector); a zero ASTRef is
	// a null (undefined) entry. Valid only when !empty; no elements means
	// undefined.
	data   []ast.ASTRef
	closed bool
}

func (r *explanationRelation) resetData() (result error) {
	for i := len(r.data) - 1; i >= 0; i-- {
		if r.data[i] != (ast.ASTRef{}) {
			result = errors.Join(result, r.plugin.ast.DelRef(&r.data[i]))
		}
	}
	r.data = r.data[:0]
	return result
}

// appendData is m_data.append(f).
func (r *explanationRelation) appendData(f []ast.ASTRef) error {
	for _, ref := range f {
		if ref == (ast.ASTRef{}) {
			r.data = append(r.data, ast.ASTRef{})
			continue
		}
		c, err := r.plugin.ast.CopyRef(ref)
		if err != nil {
			return err
		}
		r.data = append(r.data, c)
	}
	return nil
}

// assignData is assign_data(f). The new terms are referenced before the
// old ones are released (f may share them).
func (r *explanationRelation) assignData(f []ast.ASTRef) error {
	r.empty = false
	old := r.data
	r.data = nil
	err := r.appendData(f)
	for i := len(old) - 1; i >= 0; i-- {
		if old[i] != (ast.ASTRef{}) {
			err = errors.Join(err, r.plugin.ast.DelRef(&old[i]))
		}
	}
	return err
}

// setUndefined is set_undefined.
func (r *explanationRelation) setUndefined() error {
	r.empty = false
	err := r.resetData()
	r.data = make([]ast.ASTRef, len(r.sorts))
	return err
}

// setData is m_data[i] = value.
func (r *explanationRelation) setData(i int, value ast.ASTHandle) error {
	ref, err := r.plugin.ast.NewRef(value)
	if err != nil {
		return err
	}
	old := r.data[i]
	r.data[i] = ref
	if old != (ast.ASTRef{}) {
		return r.plugin.ast.DelRef(&old)
	}
	return nil
}

// uniteWithData is unite_with_data(f).
func (r *explanationRelation) uniteWithData(f []ast.ASTRef) error {
	if r.empty {
		return r.assignData(f)
	}
	for i := range r.sorts {
		a1, err := r.plugin.ast.RefGet(r.data[i])
		if err != nil {
			return err
		}
		a2, err := r.plugin.ast.RefGet(f[i])
		if err != nil {
			return err
		}
		u, err := r.plugin.mkUnion(a1, a2)
		if err != nil {
			return err
		}
		if err = r.setData(i, u); err != nil {
			return err
		}
	}
	return nil
}

func (r *explanationRelation) isUndefined(col int) bool {
	return r.data[col] == (ast.ASTRef{})
}

func (r *explanationRelation) noUndefined() bool {
	if r.empty {
		return true
	}
	for i := range r.sorts {
		if i >= len(r.data) || r.isUndefined(i) {
			return false
		}
	}
	return true
}

func (r *explanationRelation) RelationSignature() []ast.ASTHandle { return r.sorts }
func (r *explanationRelation) RelationPlugin() RelationPlugin     { return r.plugin }
func (r *explanationRelation) Empty() (bool, error)               { return r.empty, nil }
func (r *explanationRelation) FastEmpty() bool                    { return r.empty }
func (r *explanationRelation) IsPrecise() bool                    { return true }

// Reset is reset(): m_empty = true.
func (r *explanationRelation) Reset() error {
	r.empty = true
	return nil
}

// AddFactAST is add_fact(f) (SASSERT(empty())).
func (r *explanationRelation) AddFactAST(values []ast.ASTHandle) error {
	refs := make([]ast.ASTRef, 0, len(values))
	for _, v := range values {
		ref, err := r.plugin.ast.NewRef(v)
		if err != nil {
			return errors.Join(err, releaseRefs(r.plugin.ast, refs))
		}
		refs = append(refs, ref)
	}
	err := r.assignData(refs)
	return errors.Join(err, releaseRefs(r.plugin.ast, refs))
}

// ContainsFactAST is contains_fact: UNREACHABLE(); throw 0.
func (r *explanationRelation) ContainsFactAST([]ast.ASTHandle) (bool, error) {
	return false, fmt.Errorf("%w: explanation_relation::contains_fact", ErrInstruction)
}

// CloneRelation is clone(): mk_empty, m_empty, m_data.append(m_data).
func (r *explanationRelation) CloneRelation() (RelationBase, error) {
	res, err := r.plugin.mkEmpty(r.sorts)
	if err != nil {
		return nil, err
	}
	res.empty = r.empty
	if err = res.appendData(r.data); err != nil {
		return nil, errors.Join(err, res.Close())
	}
	return res, nil
}

func (r *explanationRelation) CloneFinite() (FiniteTable, error) { return r.CloneRelation() }

// Complement is complement(pred): an empty relation becomes undefined.
func (r *explanationRelation) Complement(ast.ASTHandle) (RelationBase, error) {
	res, err := r.plugin.mkEmpty(r.sorts)
	if err != nil {
		return nil, err
	}
	if r.empty {
		if err = res.setUndefined(); err != nil {
			return nil, errors.Join(err, res.Close())
		}
	}
	return res, nil
}

// ToFormula is to_formula: (= (:var 0) m_data[0]).
func (r *explanationRelation) ToFormula() (ast.ASTRef, error) {
	m := r.plugin.ast
	if len(r.data) == 0 || r.data[0] == (ast.ASTRef{}) {
		return ast.ASTRef{}, fmt.Errorf("%w: explanation_relation::to_formula of an undefined explanation", ErrInstruction)
	}
	value, err := m.RefGet(r.data[0])
	if err != nil {
		return ast.ASTRef{}, err
	}
	sort, err := m.GetSort(value)
	if err != nil {
		return ast.ASTRef{}, err
	}
	variable, err := m.MkVar(0, sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	eq, err := m.MkEq(variable, value)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.NewRef(eq)
}

// Display is display(out) (display_explanation of m_data[0] per column).
func (r *explanationRelation) Display(out io.Writer) error {
	if r.empty {
		_, err := io.WriteString(out, "<empty explanation relation>\n")
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(r.plugin.ast)
	if err != nil {
		return err
	}
	for i := range r.sorts {
		if i != 0 {
			if _, err = io.WriteString(out, ", "); err != nil {
				return err
			}
		}
		if len(r.data) == 0 || r.data[0] == (ast.ASTRef{}) {
			if _, err = io.WriteString(out, "<undefined>"); err != nil {
				return err
			}
			continue
		}
		value, err := r.plugin.ast.RefGet(r.data[0])
		if err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, value, nil); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, "\n")
	return err
}

// SizeEstimateRows is get_size_estimate (the relation_base override
// get_size_estimate_rows is the base's UINT_MAX).
func (r *explanationRelation) SizeEstimateRows() (uint32, error)  { return math.MaxUint32, nil }
func (r *explanationRelation) SizeEstimateBytes() (uint32, error) { return math.MaxUint32, nil }

func (r *explanationRelation) Signature() (Signature, error) {
	domains := make([]uint64, len(r.sorts))
	for i := range r.sorts {
		domains[i] = relationOnlyDomain
	}
	return Signature{domains: domains}, nil
}

func (r *explanationRelation) Facts() ([]Fact, error) { return nil, ErrRelationNotTable }

// Len is the register-emptiness estimate.
func (r *explanationRelation) Len() (int, error) {
	if r.empty {
		return 0, nil
	}
	return 1, nil
}

// Add is add_fact of relation_manager::table_fact_to_relation(fact).
func (r *explanationRelation) Add(fact Fact) (bool, error) {
	values := make([]ast.ASTHandle, len(fact))
	refs := make([]ast.ASTRef, 0, len(fact))
	for i, e := range fact {
		ref, err := r.plugin.manager.DecodeRelationElement(r.sorts[i], e)
		if err != nil {
			return false, errors.Join(err, releaseRefs(r.plugin.ast, refs))
		}
		refs = append(refs, ref)
		if values[i], err = r.plugin.ast.RefGet(ref); err != nil {
			return false, errors.Join(err, releaseRefs(r.plugin.ast, refs))
		}
	}
	err := r.AddFactAST(values)
	return true, errors.Join(err, releaseRefs(r.plugin.ast, refs))
}

func (r *explanationRelation) Contains(Fact) (bool, error) {
	return r.ContainsFactAST(nil)
}

// Close is deallocate(): get_plugin().recycle(this).
func (r *explanationRelation) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.plugin.closed {
		return r.resetData()
	}
	r.plugin.recycle(r)
	return nil
}

// release frees a pooled relation's terms (plugin destruction).
func (r *explanationRelation) release() error {
	return r.resetData()
}
