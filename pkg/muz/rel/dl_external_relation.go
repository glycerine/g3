package rel

// Source: src/muz/rel/dl_external_relation.cpp/.h external_relation_plugin
// as a relation plugin of the relation manager (relation_plugin's
// mk_empty, mk_full and mk_*_fn) and external_relation as a relation_base.
// fixedpoint_context::set_state (Z3_fixedpoint_init) registers the plugin
// with rel_context's relation manager; the compiled instructions dispatch
// its relations through relation_manager::mk_*_fn like the other relation
// plugins, also as an inner relation of a product_relation.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

var _ RelationPlugin = (*ExternalRelationPlugin)(nil)
var _ RelationBase = (*ExternalRelation)(nil)

// Name is external_relation_plugin::get_name().
func (p *ExternalRelationPlugin) Name() string { return "external_relation" }

func (p *ExternalRelationPlugin) Manager() *RelationManager { return p.manager }

// CanHandleSignature: external_relation_plugin handles every signature.
func (p *ExternalRelationPlugin) CanHandleSignature([]ast.ASTHandle) (bool, error) {
	return true, nil
}

// MkEmpty is external_relation_plugin::mk_empty: a fresh constant "T" of the
// relation sort, assigned by OP_RA_EMPTY.
func (p *ExternalRelationPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	r, err := p.NewEmpty(sorts)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// MkFull is relation_plugin::mk_full: mk_empty, complement, deallocate.
func (p *ExternalRelationPlugin) MkFull(_ ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	aux, err := p.NewEmpty(sorts)
	if err != nil {
		return nil, err
	}
	res, err := aux.Complement()
	err = errors.Join(err, aux.Close())
	if err != nil {
		if res != nil {
			err = errors.Join(err, res.Close())
		}
		return nil, err
	}
	return res, nil
}

// get is external_relation_plugin::get (dynamic_cast); ok is check_kind.
func (p *ExternalRelationPlugin) get(r RelationBase) (*ExternalRelation, bool) {
	x, ok := r.(*ExternalRelation)
	return x, ok && x.plugin == p
}

// relationSort is get_relation_sort(sig).
func (p *ExternalRelationPlugin) relationSort(sorts []ast.ASTHandle) (ast.ASTHandle, error) {
	return p.ast.MkDatalogRelationSort(sorts)
}

// mkFn is m.mk_func_decl(fid, k, params, n, domain) of the datalog relation
// family, held for the functor's lifetime.
func (p *ExternalRelationPlugin) mkFn(kind int32, ints []int, astParam ast.ASTHandle, domain []ast.ASTHandle) (_ ast.ASTHandle, resultErr error) {
	m := p.ast
	fid, err := m.RegisterDatalogPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	params := make([]ast.ASTParameter, 0, len(ints)+1)
	defer func() {
		for i := range params {
			resultErr = errors.Join(resultErr, m.DelParameter(&params[i]))
		}
	}()
	if astParam != (ast.ASTHandle{}) {
		param, err := m.NewParameterAST(astParam)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		params = append(params, param)
	}
	for _, v := range ints {
		if v < 0 || v > math.MaxInt32 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		param, err := m.NewParameterInt(int32(v))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		params = append(params, param)
	}
	fn, err := m.MkFamilyFuncDecl(fid, kind, params, domain, ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if fn == (ast.ASTHandle{}) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	ref, err := m.NewRef(fn)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.manager.fnRoots = append(p.manager.fnRoots, ref)
	return fn, nil
}

// wrapResult is alloc(external_relation, plugin, sig, res).
func (p *ExternalRelationPlugin) wrapResult(sorts []ast.ASTHandle, value ast.ASTHandle) (*ExternalRelation, error) {
	ref, err := p.ast.NewRef(value)
	if err != nil {
		return nil, err
	}
	p.users++
	return &ExternalRelation{plugin: p, manager: p.manager, ast: p.ast, sorts: append([]ast.ASTHandle(nil), sorts...), relation: ref}, nil
}

func (p *ExternalRelationPlugin) relationOf(r RelationBase) (ast.ASTHandle, error) {
	x, ok := p.get(r)
	if !ok {
		return ast.ASTHandle{}, ErrInstruction
	}
	return x.Relation()
}

// MkJoinFn is external_relation_plugin::mk_join_fn (join_fn).
func (p *ExternalRelationPlugin) MkJoinFn(r1, r2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	if _, ok := p.get(r1); !ok {
		return nil, nil
	}
	if _, ok := p.get(r2); !ok {
		return nil, nil
	}
	params := make([]int, 0, 2*len(cols1))
	for i := range cols1 {
		params = append(params, cols1[i], cols2[i])
	}
	s1, err := p.relationSort(r1.RelationSignature())
	if err != nil {
		return nil, err
	}
	s2, err := p.relationSort(r2.RelationSignature())
	if err != nil {
		return nil, err
	}
	fn, err := p.mkFn(ast.DatalogOpJoin, params, ast.ASTHandle{}, []ast.ASTHandle{s1, s2})
	if err != nil {
		return nil, err
	}
	sorts := joinSorts(r1.RelationSignature(), r2.RelationSignature())
	return func(a, b RelationBase) (RelationBase, error) {
		x, err := p.relationOf(a)
		if err != nil {
			return nil, err
		}
		y, err := p.relationOf(b)
		if err != nil {
			return nil, err
		}
		res, err := p.reduce(fn, []ast.ASTHandle{x, y})
		if err != nil {
			return nil, err
		}
		return p.wrapResult(sorts, res)
	}, nil
}

// MkProjectFn is external_relation_plugin::mk_project_fn (project_fn; the
// C++ makes it without check_kind).
func (p *ExternalRelationPlugin) MkProjectFn(r RelationBase, removed []int) (RelationTransformerFn, error) {
	x, ok := p.get(r)
	if !ok {
		return nil, nil
	}
	s, err := p.relationSort(x.sorts)
	if err != nil {
		return nil, err
	}
	fn, err := p.mkFn(ast.DatalogOpProject, removed, ast.ASTHandle{}, []ast.ASTHandle{s})
	if err != nil {
		return nil, err
	}
	sorts := projectSorts(r.RelationSignature(), removed)
	return func(a RelationBase) (RelationBase, error) {
		rel, err := p.relationOf(a)
		if err != nil {
			return nil, err
		}
		res, err := p.reduce(fn, []ast.ASTHandle{rel})
		if err != nil {
			return nil, err
		}
		return p.wrapResult(sorts, res)
	}, nil
}

// MkRenameFn is external_relation_plugin::mk_rename_fn (rename_fn).
func (p *ExternalRelationPlugin) MkRenameFn(r RelationBase, cycle []int) (RelationTransformerFn, error) {
	x, ok := p.get(r)
	if !ok {
		return nil, nil
	}
	s, err := p.relationSort(x.sorts)
	if err != nil {
		return nil, err
	}
	fn, err := p.mkFn(ast.DatalogOpRename, cycle, ast.ASTHandle{}, []ast.ASTHandle{s})
	if err != nil {
		return nil, err
	}
	sorts := renameSorts(r.RelationSignature(), cycle)
	return func(a RelationBase) (RelationBase, error) {
		rel, err := p.relationOf(a)
		if err != nil {
			return nil, err
		}
		res, err := p.reduce(fn, []ast.ASTHandle{rel})
		if err != nil {
			return nil, err
		}
		return p.wrapResult(sorts, res)
	}, nil
}

// unionFn is union_fn for OP_RA_UNION or OP_RA_WIDEN.
func (p *ExternalRelationPlugin) unionFn(target, source, delta RelationBase, kind int32) (RelationUnionFn, error) {
	if _, ok := p.get(target); !ok {
		return nil, nil
	}
	src, ok := p.get(source)
	if !ok {
		return nil, nil
	}
	if delta != nil {
		if _, ok := p.get(delta); !ok {
			return nil, nil
		}
	}
	s, err := p.relationSort(src.sorts)
	if err != nil {
		return nil, err
	}
	fn, err := p.mkFn(kind, nil, ast.ASTHandle{}, []ast.ASTHandle{s, s})
	if err != nil {
		return nil, err
	}
	return func(t, s, d RelationBase) error {
		x, err := p.relationOf(t)
		if err != nil {
			return err
		}
		y, err := p.relationOf(s)
		if err != nil {
			return err
		}
		outs := []ast.ASTHandle{x}
		if d != nil {
			z, err := p.relationOf(d)
			if err != nil {
				return err
			}
			outs = append(outs, z)
		}
		return p.reduceAssign(fn, []ast.ASTHandle{x, y}, outs)
	}, nil
}

func (p *ExternalRelationPlugin) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.unionFn(target, source, delta, ast.DatalogOpUnion)
}

func (p *ExternalRelationPlugin) MkWidenFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.unionFn(target, source, delta, ast.DatalogOpWiden)
}

// mkFilterFn is external_relation_plugin::mk_filter_fn(s, condition, f).
func (p *ExternalRelationPlugin) mkFilterFn(relationSort, condition ast.ASTHandle) (ast.ASTHandle, error) {
	return p.mkFn(ast.DatalogOpFilter, nil, condition, []ast.ASTHandle{relationSort})
}

// filterAssign applies filter declarations in turn: reduce_assign(fn, 1,
// &r, 1, &r).
func (p *ExternalRelationPlugin) filterAssign(fns []ast.ASTHandle) RelationMutatorFn {
	return func(a RelationBase) error {
		rel, err := p.relationOf(a)
		if err != nil {
			return err
		}
		for _, fn := range fns {
			args := []ast.ASTHandle{rel}
			if err := p.reduceAssign(fn, args, args); err != nil {
				return err
			}
		}
		return nil
	}
}

// MkFilterInterpretedFn is external_relation_plugin::mk_filter_interpreted_fn.
func (p *ExternalRelationPlugin) MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error) {
	x, ok := p.get(t)
	if !ok {
		return nil, nil
	}
	s, err := p.relationSort(x.sorts)
	if err != nil {
		return nil, err
	}
	fn, err := p.mkFilterFn(s, condition)
	if err != nil {
		return nil, err
	}
	return p.filterAssign([]ast.ASTHandle{fn}), nil
}

// MkFilterEqualFn is external_relation_plugin::mk_filter_equal_fn: the
// interpreted filter of (= (:var col) value).
func (p *ExternalRelationPlugin) MkFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (_ RelationMutatorFn, resultErr error) {
	x, ok := p.get(t)
	if !ok {
		return nil, nil
	}
	m := p.ast
	v, err := m.MkVar(uint32(col), x.sorts[col])
	if err != nil {
		return nil, err
	}
	vref, err := m.NewRef(v)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&vref)) }()
	condition, err := m.MkEq(v, value)
	if err != nil {
		return nil, err
	}
	cref, err := m.NewRef(condition)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&cref)) }()
	return p.MkFilterInterpretedFn(t, condition)
}

// MkFilterIdenticalFn is external_relation_plugin::mk_filter_identical_fn:
// one filter of (= v0 (:var col_i)) per further column.
func (p *ExternalRelationPlugin) MkFilterIdenticalFn(t RelationBase, cols []int) (_ RelationMutatorFn, resultErr error) {
	x, ok := p.get(t)
	if !ok {
		return nil, nil
	}
	if len(cols) <= 1 {
		return p.filterAssign(nil), nil
	}
	m := p.ast
	s, err := p.relationSort(x.sorts)
	if err != nil {
		return nil, err
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	hold := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return h, err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return h, err
		}
		refs = append(refs, ref)
		return h, nil
	}
	v0, err := hold(m.MkVar(uint32(cols[0]), x.sorts[cols[0]]))
	if err != nil {
		return nil, err
	}
	fns := make([]ast.ASTHandle, 0, len(cols)-1)
	for _, col := range cols[1:] {
		v, err := hold(m.MkVar(uint32(col), x.sorts[col]))
		if err != nil {
			return nil, err
		}
		eq, err := hold(m.MkEq(v0, v))
		if err != nil {
			return nil, err
		}
		fn, err := p.mkFilterFn(s, eq)
		if err != nil {
			return nil, err
		}
		fns = append(fns, fn)
	}
	return p.filterAssign(fns), nil
}

// MkFilterByNegationFn is external_relation_plugin::mk_filter_by_negation_fn
// (negation_filter_fn).
func (p *ExternalRelationPlugin) MkFilterByNegationFn(t, neg RelationBase, tCols, negCols []int) (RelationIntersectionFilterFn, error) {
	x, ok := p.get(t)
	if !ok {
		return nil, nil
	}
	y, ok := p.get(neg)
	if !ok {
		return nil, nil
	}
	params := make([]int, 0, 2*len(tCols))
	for i := range tCols {
		params = append(params, tCols[i], negCols[i])
	}
	s1, err := p.relationSort(x.sorts)
	if err != nil {
		return nil, err
	}
	s2, err := p.relationSort(y.sorts)
	if err != nil {
		return nil, err
	}
	fn, err := p.mkFn(ast.DatalogOpNegationFilter, params, ast.ASTHandle{}, []ast.ASTHandle{s1, s2})
	if err != nil {
		return nil, err
	}
	return func(a, b RelationBase) error {
		r1, err := p.relationOf(a)
		if err != nil {
			return err
		}
		r2, err := p.relationOf(b)
		if err != nil {
			return err
		}
		args := []ast.ASTHandle{r1, r2}
		return p.reduceAssign(fn, args, args[:1])
	}, nil
}

// The external relation plugin implements no fused operations: the
// relation manager's defaults (join then project, filter then project,
// filter_equal then project) apply.
func (p *ExternalRelationPlugin) MkJoinProjectFn(RelationBase, RelationBase, []int, []int, []int) (RelationJoinFn, error) {
	return nil, nil
}
func (p *ExternalRelationPlugin) MkFilterInterpretedAndProjectFn(RelationBase, ast.ASTHandle, []int) (RelationTransformerFn, error) {
	return nil, nil
}
func (p *ExternalRelationPlugin) MkSelectEqualAndProjectFn(RelationBase, ast.ASTHandle, int) (RelationTransformerFn, error) {
	return nil, nil
}
func (p *ExternalRelationPlugin) MkFilterByIntersectionFn(RelationBase, RelationBase, []int, []int) (RelationIntersectionFilterFn, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// external_relation as relation_base

func (r *ExternalRelation) RelationSignature() []ast.ASTHandle { return r.sorts }
func (r *ExternalRelation) RelationPlugin() RelationPlugin     { return r.plugin }

// FastEmpty is relation_base::fast_empty(): empty().
func (r *ExternalRelation) FastEmpty() bool {
	empty, err := r.Empty()
	return err == nil && empty
}

func (r *ExternalRelation) IsPrecise() bool { return true }

// SizeEstimateRows and SizeEstimateBytes are relation_base's UINT_MAX.
func (r *ExternalRelation) SizeEstimateRows() (uint32, error)  { return math.MaxUint32, nil }
func (r *ExternalRelation) SizeEstimateBytes() (uint32, error) { return math.MaxUint32, nil }

func (r *ExternalRelation) Signature() (Signature, error) {
	m := r.manager
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

func (r *ExternalRelation) Facts() ([]Fact, error) { return nil, ErrRelationNotTable }

// Len is the register-emptiness estimate: 0 for an empty relation,
// otherwise get_size_estimate_rows().
func (r *ExternalRelation) Len() (int, error) {
	empty, err := r.Empty()
	if err != nil || empty {
		return 0, err
	}
	return math.MaxUint32, nil
}

// Reset is relation_base::reset: mk_filter_interpreted_fn(*this, false).
func (r *ExternalRelation) Reset() error {
	f, err := r.ast.MkFalse()
	if err != nil {
		return err
	}
	fn, err := r.manager.MkRelationFilterInterpretedFn(r, f)
	if err != nil {
		return err
	}
	if fn == nil {
		return ErrInstruction
	}
	return fn(r)
}

func (r *ExternalRelation) CloneRelation() (RelationBase, error) {
	c, err := r.Clone()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *ExternalRelation) CloneFinite() (FiniteTable, error) { return r.CloneRelation() }

// AddFactAST is external_relation::add_fact.
func (r *ExternalRelation) AddFactAST(values []ast.ASTHandle) error { return r.AddFact(values) }

func (r *ExternalRelation) decode(fact Fact) (_ []ast.ASTHandle, refs []ast.ASTRef, err error) {
	values := make([]ast.ASTHandle, len(fact))
	for i, element := range fact {
		ref, x := r.manager.DecodeRelationElement(r.sorts[i], element)
		if x != nil {
			return nil, refs, x
		}
		refs = append(refs, ref)
		if values[i], x = r.ast.RefGet(ref); x != nil {
			return nil, refs, x
		}
	}
	return values, refs, nil
}

// Add is add_fact of relation_manager::table_fact_to_relation(fact).
func (r *ExternalRelation) Add(fact Fact) (bool, error) {
	values, refs, err := r.decode(fact)
	if err == nil {
		err = r.AddFact(values)
	}
	return true, errors.Join(err, releaseRefs(r.ast, refs))
}

// Contains is contains_fact.
func (r *ExternalRelation) Contains(fact Fact) (bool, error) {
	values, refs, err := r.decode(fact)
	var found bool
	if err == nil {
		found, err = r.ContainsFact(values)
	}
	return found, errors.Join(err, releaseRefs(r.ast, refs))
}

// NewForeign is NewEmpty for a relation the API user owns (g3's
// NewFixedpointExternalRelation): the fixedpoint context does not close
// while one is live.
func (p *ExternalRelationPlugin) NewForeign(sorts []ast.ASTHandle) (*ExternalRelation, error) {
	r, err := p.NewEmpty(sorts)
	if err != nil {
		return nil, err
	}
	r.foreign = true
	p.foreign++
	return r, nil
}

// ForeignUsers counts the live NewForeign relations.
func (p *ExternalRelationPlugin) ForeignUsers() int {
	if p == nil {
		return 0
	}
	return p.foreign
}
