package rel

// Source: src/muz/rel/dl_interval_relation.cpp interval_relation_plugin and
// src/muz/rel/dl_bound_relation.cpp bound_relation_plugin: the relation
// plugins rel_context registers for Int/Real signatures, dispatched through
// relation_manager::mk_*_fn by the compiled instructions. The vector_relation
// payloads are IntervalRelation and BoundRelation.

import (
	"errors"
	"io"
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// relationOnlyDomain is the register-signature domain the compiler records
// for an Int/Real column: no table plugin can handle it
// (relation_manager::relation_sort_to_table fails), so only relation plugins
// create such registers.
const relationOnlyDomain = ^uint64(0)

// arithRelationSort is arith_util::is_int(s) || is_real(s).
func arithRelationSort(m *ast.ASTManager, sort ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(sort)
	if err != nil || kind != ast.ASTSort {
		return false, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	arith, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return false, err
	}
	if family != arith {
		return false, nil
	}
	declaration, err := m.GetDeclKind(sort)
	if err != nil {
		return false, err
	}
	// arith_decl_plugin.h: REAL_SORT, INT_SORT.
	return declaration == 0 || declaration == 1, nil
}

func arithRelationSignature(m *ast.ASTManager, sorts []ast.ASTHandle) (bool, error) {
	for _, sort := range sorts {
		ok, err := arithRelationSort(m, sort)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// relationOnlySort is a sort relation_sort_to_table rejects whose columns
// only relation plugins hold: Int/Real, or the datalog rule sort of the
// explanation relations.
func relationOnlySort(m *ast.ASTManager, sort ast.ASTHandle) (bool, error) {
	if ok, err := arithRelationSort(m, sort); err != nil || ok {
		return ok, err
	}
	kind, err := m.GetKind(sort)
	if err != nil || kind != ast.ASTSort {
		return false, err
	}
	return m.IsDatalogRuleSort(sort)
}

// IsRuleSort is dl_decl_util::is_rule_sort.
func IsRuleSort(m *ast.ASTManager, sort ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(sort)
	if err != nil || kind != ast.ASTSort {
		return false, err
	}
	return m.IsDatalogRuleSort(sort)
}

// relationSortDomain is the compiler's register-signature domain of a
// relation sort: the table domain, or relationOnlyDomain for Int/Real and
// the rule sort.
func (m *RelationManager) relationSortDomain(sort ast.ASTHandle) (uint64, error) {
	if ok, err := relationOnlySort(m.manager, sort); err != nil {
		return 0, err
	} else if ok {
		return relationOnlyDomain, nil
	}
	return m.sortDomain(sort)
}

// RelationPredicateSignature is relation_manager::from_predicate with the
// compiler's register signature (relationSortDomain per column).
func (m *RelationManager) RelationPredicateSignature(declaration ast.ASTHandle) ([]ast.ASTHandle, Signature, error) {
	if err := m.open(); err != nil {
		return nil, Signature{}, err
	}
	sorts, err := m.predicateSorts(declaration)
	if err != nil {
		return nil, Signature{}, err
	}
	domains := make([]uint64, len(sorts))
	for i, sort := range sorts {
		if domains[i], err = m.relationSortDomain(sort); err != nil {
			return nil, Signature{}, err
		}
	}
	signature, err := NewSignature(domains)
	return sorts, signature, err
}

// EncodeRelationElement is the compiler's relation_element of a constant:
// a table element, or for an Int/Real column an index into the manager's
// interned numerals (the C++ keeps the expr* itself).
func (m *RelationManager) EncodeRelationElement(sort, value ast.ASTHandle) (Element, error) {
	if ok, err := relationOnlySort(m.manager, sort); err != nil {
		return 0, err
	} else if !ok {
		return m.EncodeValue(sort, value)
	}
	if index, found := m.arithIndex[value]; found {
		return index, nil
	}
	ref, err := m.manager.NewRef(value)
	if err != nil {
		return 0, err
	}
	if m.arithIndex == nil {
		m.arithIndex = make(map[ast.ASTHandle]Element)
	}
	index := Element(len(m.arithElements))
	m.arithElements = append(m.arithElements, ref)
	m.arithIndex[value] = index
	return index, nil
}

// DecodeRelationElement inverts EncodeRelationElement; the caller owns the
// returned reference.
func (m *RelationManager) DecodeRelationElement(sort ast.ASTHandle, value Element) (ast.ASTRef, error) {
	if ok, err := relationOnlySort(m.manager, sort); err != nil {
		return ast.ASTRef{}, err
	} else if !ok {
		return m.DecodeValue(sort, value)
	}
	if uint64(value) >= uint64(len(m.arithElements)) {
		return ast.ASTRef{}, ErrTable
	}
	handle, err := m.manager.RefGet(m.arithElements[value])
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.manager.NewRef(handle)
}

func (m *RelationManager) closeRelationElements() (result error) {
	for i := len(m.arithElements) - 1; i >= 0; i-- {
		result = errors.Join(result, m.manager.DelRef(&m.arithElements[i]))
	}
	m.arithElements, m.arithIndex = nil, nil
	return result
}

// relationNumeral is arith_util::is_numeral(e, r).
func relationNumeral(m *ast.ASTManager, value ast.ASTHandle) (*big.Rat, bool, error) {
	text, _, matched, err := m.ArithmeticNumeralText(value)
	if err != nil || !matched {
		return nil, false, err
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return number, true, nil
}

func uint32Columns(cols []int) []uint32 {
	result := make([]uint32, len(cols))
	for i, c := range cols {
		result[i] = uint32(c)
	}
	return result
}

// vectorRelationBase is the relation_base part shared by interval_relation
// and bound_relation (vector_relation<T>).
type vectorRelationBase struct {
	sorts  []ast.ASTHandle
	closed bool
}

func (r *vectorRelationBase) RelationSignature() []ast.ASTHandle { return r.sorts }

func (r *vectorRelationBase) Signature() (Signature, error) {
	domains := make([]uint64, len(r.sorts))
	for i := range domains {
		domains[i] = relationOnlyDomain
	}
	return Signature{domains: domains}, nil
}

func (r *vectorRelationBase) Facts() ([]Fact, error) { return nil, ErrRelationNotTable }

// Len is get_size_estimate_rows(): UINT_MAX, or 0 for an empty relation so
// that register emptiness follows fast_empty.
func (r *vectorRelationBase) sizeEstimate(empty bool) int {
	if empty {
		return 0
	}
	return math.MaxUint32
}

func (r *vectorRelationBase) Close() error {
	if r.closed {
		return ErrTable
	}
	r.closed = true
	return nil
}

// ---------------------------------------------------------------------------
// interval_relation_plugin

type IntervalRelationPlugin struct {
	RelationPluginDefaults
	manager *RelationManager
}

func NewIntervalRelationPlugin(m *RelationManager) *IntervalRelationPlugin {
	return &IntervalRelationPlugin{manager: m}
}

func (p *IntervalRelationPlugin) Name() string              { return "interval_relation" }
func (p *IntervalRelationPlugin) Manager() *RelationManager { return p.manager }

func (p *IntervalRelationPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	return arithRelationSignature(p.manager.manager, sorts)
}

func (p *IntervalRelationPlugin) wrap(sorts []ast.ASTHandle, value *IntervalRelation) *intervalRelationValue {
	return &intervalRelationValue{vectorRelationBase: vectorRelationBase{sorts: append([]ast.ASTHandle(nil), sorts...)}, plugin: p, value: value}
}

func (p *IntervalRelationPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	return p.wrap(sorts, NewIntervalEmpty(uint32(len(sorts)))), nil
}

func (p *IntervalRelationPlugin) MkFull(_ ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	return p.wrap(sorts, NewIntervalFull(uint32(len(sorts)))), nil
}

func (p *IntervalRelationPlugin) get(r RelationBase) (*intervalRelationValue, bool) {
	v, ok := r.(*intervalRelationValue)
	return v, ok && v.plugin == p
}

func (p *IntervalRelationPlugin) MkJoinFn(r1, r2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	if _, ok := p.get(r1); !ok {
		return nil, nil
	}
	if _, ok := p.get(r2); !ok {
		return nil, nil
	}
	c1, c2 := uint32Columns(cols1), uint32Columns(cols2)
	sorts := joinSorts(r1.RelationSignature(), r2.RelationSignature())
	return func(a, b RelationBase) (RelationBase, error) {
		x, _ := p.get(a)
		y, _ := p.get(b)
		if x == nil || y == nil {
			return nil, ErrInstruction
		}
		joined, err := JoinIntervalRelations(x.value, y.value, c1, c2)
		if err != nil {
			return nil, err
		}
		return p.wrap(sorts, joined), nil
	}, nil
}

// MkProjectFn: the C++ returns its project_fn without check_kind.
func (p *IntervalRelationPlugin) MkProjectFn(r RelationBase, removed []int) (RelationTransformerFn, error) {
	cols := uint32Columns(removed)
	sorts := projectSorts(r.RelationSignature(), removed)
	return func(a RelationBase) (RelationBase, error) {
		x, ok := p.get(a)
		if !ok {
			return nil, ErrInstruction
		}
		projected, err := x.value.Project(cols)
		if err != nil {
			return nil, err
		}
		return p.wrap(sorts, projected), nil
	}, nil
}

func (p *IntervalRelationPlugin) MkRenameFn(r RelationBase, cycle []int) (RelationTransformerFn, error) {
	if _, ok := p.get(r); !ok {
		return nil, nil
	}
	cols := uint32Columns(cycle)
	sorts := renameSorts(r.RelationSignature(), cycle)
	return func(a RelationBase) (RelationBase, error) {
		x, ok := p.get(a)
		if !ok {
			return nil, ErrInstruction
		}
		renamed, err := x.value.Rename(cols)
		if err != nil {
			return nil, err
		}
		return p.wrap(sorts, renamed), nil
	}, nil
}

func (p *IntervalRelationPlugin) unionFn(target, source, delta RelationBase, widen bool) RelationUnionFn {
	if _, ok := p.get(target); !ok {
		return nil
	}
	if _, ok := p.get(source); !ok {
		return nil
	}
	if delta != nil {
		if _, ok := p.get(delta); !ok {
			return nil
		}
	}
	return func(t, s, d RelationBase) error {
		x, _ := p.get(t)
		y, _ := p.get(s)
		if x == nil || y == nil {
			return ErrInstruction
		}
		if d != nil {
			z, _ := p.get(d)
			if z == nil {
				return ErrInstruction
			}
			_, err := x.value.UnionWithDelta(y.value, z.value, widen)
			return err
		}
		_, err := x.value.Union(y.value, widen)
		return err
	}
}

func (p *IntervalRelationPlugin) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.unionFn(target, source, delta, false), nil
}

func (p *IntervalRelationPlugin) MkWidenFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.unionFn(target, source, delta, true), nil
}

func (p *IntervalRelationPlugin) MkFilterIdenticalFn(t RelationBase, cols []int) (RelationMutatorFn, error) {
	if _, ok := p.get(t); !ok {
		return nil, nil
	}
	columns := uint32Columns(cols)
	return func(a RelationBase) error {
		x, ok := p.get(a)
		if !ok {
			return ErrInstruction
		}
		return x.value.FilterIdenticalColumns(columns)
	}, nil
}

func (p *IntervalRelationPlugin) MkFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error) {
	if _, ok := p.get(t); !ok {
		return nil, nil
	}
	number, ok, err := relationNumeral(p.manager.manager, value)
	if err != nil {
		return nil, err
	}
	if !ok {
		// VERIFY(arith.is_numeral(value, m_value))
		return nil, ast.ErrASTDomain
	}
	return func(a RelationBase) error {
		x, ok := p.get(a)
		if !ok {
			return ErrInstruction
		}
		return x.value.FilterEqual(uint32(col), number)
	}, nil
}

func (p *IntervalRelationPlugin) MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error) {
	if _, ok := p.get(t); !ok {
		return nil, nil
	}
	m := p.manager.manager
	ref, err := m.NewRef(condition)
	if err != nil {
		return nil, err
	}
	p.manager.fnRoots = append(p.manager.fnRoots, ref)
	return func(a RelationBase) error {
		x, ok := p.get(a)
		if !ok {
			return ErrInstruction
		}
		return x.value.FilterInterpreted(m, condition)
	}, nil
}

type intervalRelationValue struct {
	vectorRelationBase
	plugin *IntervalRelationPlugin
	value  *IntervalRelation
}

func (r *intervalRelationValue) RelationPlugin() RelationPlugin { return r.plugin }
func (r *intervalRelationValue) Empty() (bool, error)           { return r.value.Empty(), nil }
func (r *intervalRelationValue) FastEmpty() bool                { return r.value.Empty() }
func (r *intervalRelationValue) IsPrecise() bool                { return false }
func (r *intervalRelationValue) Len() (int, error)              { return r.sizeEstimate(r.value.Empty()), nil }
func (r *intervalRelationValue) SizeEstimateRows() (uint32, error) {
	return math.MaxUint32, nil
}
func (r *intervalRelationValue) SizeEstimateBytes() (uint32, error) {
	return math.MaxUint32, nil
}
func (r *intervalRelationValue) Display(out io.Writer) error { return r.value.Display(out) }
func (r *intervalRelationValue) ToFormula() (ast.ASTRef, error) {
	return r.value.ToFormula(r.plugin.manager.manager, r.sorts)
}

// Reset is vector_relation's reset: set_empty.
func (r *intervalRelationValue) Reset() error {
	r.value.values.setEmpty()
	return nil
}

func (r *intervalRelationValue) CloneRelation() (RelationBase, error) {
	clone, err := r.value.Clone()
	if err != nil {
		return nil, err
	}
	return r.plugin.wrap(r.sorts, clone), nil
}

func (r *intervalRelationValue) CloneFinite() (FiniteTable, error) { return r.CloneRelation() }

func (r *intervalRelationValue) decode(fact Fact) (_ []ast.ASTHandle, refs []ast.ASTRef, err error) {
	m := r.plugin.manager
	values := make([]ast.ASTHandle, len(fact))
	for i, element := range fact {
		ref, x := m.DecodeRelationElement(r.sorts[i], element)
		if x != nil {
			return nil, refs, x
		}
		refs = append(refs, ref)
		if values[i], x = m.manager.RefGet(ref); x != nil {
			return nil, refs, x
		}
	}
	return values, refs, nil
}

func releaseRefs(m *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for i := len(refs) - 1; i >= 0; i-- {
		result = errors.Join(result, m.DelRef(&refs[i]))
	}
	return result
}

// AddFactAST is interval_relation::add_fact(relation_fact).
func (r *intervalRelationValue) AddFactAST(values []ast.ASTHandle) error {
	return r.value.AddFactExpressions(r.plugin.manager.manager, r.sorts, values)
}

func (r *intervalRelationValue) Add(fact Fact) (bool, error) {
	values, refs, err := r.decode(fact)
	if err == nil {
		err = r.AddFactAST(values)
	}
	return true, errors.Join(err, releaseRefs(r.plugin.manager.manager, refs))
}

func (r *intervalRelationValue) Contains(fact Fact) (bool, error) {
	values, refs, err := r.decode(fact)
	var found bool
	if err == nil {
		found, err = r.value.ContainsFactExpressions(r.plugin.manager.manager, values)
	}
	return found, errors.Join(err, releaseRefs(r.plugin.manager.manager, refs))
}

// ---------------------------------------------------------------------------
// bound_relation_plugin

type BoundRelationPluginRel struct {
	RelationPluginDefaults
	manager *RelationManager
}

func NewBoundRelationPluginRel(m *RelationManager) *BoundRelationPluginRel {
	return &BoundRelationPluginRel{manager: m}
}

func (p *BoundRelationPluginRel) Name() string              { return "bound_relation" }
func (p *BoundRelationPluginRel) Manager() *RelationManager { return p.manager }

func (p *BoundRelationPluginRel) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	return arithRelationSignature(p.manager.manager, sorts)
}

func (p *BoundRelationPluginRel) wrap(sorts []ast.ASTHandle, value *BoundRelation) *boundRelationValue {
	return &boundRelationValue{vectorRelationBase: vectorRelationBase{sorts: append([]ast.ASTHandle(nil), sorts...)}, plugin: p, value: value}
}

func (p *BoundRelationPluginRel) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	return p.wrap(sorts, NewBoundRelation(uint32(len(sorts)), true)), nil
}

func (p *BoundRelationPluginRel) MkFull(_ ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	return p.wrap(sorts, NewBoundRelation(uint32(len(sorts)), false)), nil
}

func (p *BoundRelationPluginRel) get(r RelationBase) (*boundRelationValue, bool) {
	v, ok := r.(*boundRelationValue)
	return v, ok && v.plugin == p
}

func (p *BoundRelationPluginRel) MkJoinFn(r1, r2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	if _, ok := p.get(r1); !ok {
		return nil, nil
	}
	if _, ok := p.get(r2); !ok {
		return nil, nil
	}
	c1, c2 := uint32Columns(cols1), uint32Columns(cols2)
	sorts := joinSorts(r1.RelationSignature(), r2.RelationSignature())
	return func(a, b RelationBase) (RelationBase, error) {
		x, _ := p.get(a)
		y, _ := p.get(b)
		if x == nil || y == nil {
			return nil, ErrInstruction
		}
		joined, err := JoinBoundRelations(x.value, y.value, c1, c2)
		if err != nil {
			return nil, err
		}
		return p.wrap(sorts, joined), nil
	}, nil
}

// MkProjectFn: the C++ returns its project_fn without check_kind.
func (p *BoundRelationPluginRel) MkProjectFn(r RelationBase, removed []int) (RelationTransformerFn, error) {
	cols := uint32Columns(removed)
	sorts := projectSorts(r.RelationSignature(), removed)
	return func(a RelationBase) (RelationBase, error) {
		x, ok := p.get(a)
		if !ok {
			return nil, ErrInstruction
		}
		projected, err := x.value.Project(cols)
		if err != nil {
			return nil, err
		}
		return p.wrap(sorts, projected), nil
	}, nil
}

func (p *BoundRelationPluginRel) MkRenameFn(r RelationBase, cycle []int) (RelationTransformerFn, error) {
	if _, ok := p.get(r); !ok {
		return nil, nil
	}
	cols := uint32Columns(cycle)
	sorts := renameSorts(r.RelationSignature(), cycle)
	return func(a RelationBase) (RelationBase, error) {
		x, ok := p.get(a)
		if !ok {
			return nil, ErrInstruction
		}
		renamed, err := x.value.Rename(cols)
		if err != nil {
			return nil, err
		}
		return p.wrap(sorts, renamed), nil
	}, nil
}

func (p *BoundRelationPluginRel) unionFn(target, source, delta RelationBase, widen bool) RelationUnionFn {
	if _, ok := p.get(target); !ok {
		return nil
	}
	if delta != nil {
		if _, ok := p.get(delta); !ok {
			return nil
		}
	}
	if interval, ok := source.(*intervalRelationValue); ok {
		// union_fn_i: mk_union_i ignores delta and widening.
		_ = interval
		return func(t, s, _ RelationBase) error {
			x, _ := p.get(t)
			y, _ := s.(*intervalRelationValue)
			if x == nil || y == nil {
				return ErrInstruction
			}
			return x.value.UnionInterval(y.value)
		}
	}
	if _, ok := p.get(source); !ok {
		return nil
	}
	return func(t, s, d RelationBase) error {
		x, _ := p.get(t)
		y, _ := p.get(s)
		if x == nil || y == nil {
			return ErrInstruction
		}
		if d != nil {
			z, _ := p.get(d)
			if z == nil {
				return ErrInstruction
			}
			_, err := x.value.UnionWithDelta(y.value, z.value, widen)
			return err
		}
		var err error
		if widen {
			_, err = x.value.Widen(y.value)
		} else {
			_, err = x.value.Union(y.value)
		}
		return err
	}
}

func (p *BoundRelationPluginRel) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.unionFn(target, source, delta, false), nil
}

func (p *BoundRelationPluginRel) MkWidenFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.unionFn(target, source, delta, true), nil
}

func (p *BoundRelationPluginRel) MkFilterIdenticalFn(t RelationBase, cols []int) (RelationMutatorFn, error) {
	if _, ok := p.get(t); !ok {
		return nil, nil
	}
	columns := uint32Columns(cols)
	return func(a RelationBase) error {
		x, ok := p.get(a)
		if !ok {
			return ErrInstruction
		}
		return x.value.FilterIdentical(columns)
	}, nil
}

// MkFilterEqualFn: bound_relation's filter_equal_fn does nothing.
func (p *BoundRelationPluginRel) MkFilterEqualFn(t RelationBase, _ ast.ASTHandle, _ int) (RelationMutatorFn, error) {
	if _, ok := p.get(t); !ok {
		return nil, nil
	}
	return func(RelationBase) error { return nil }, nil
}

// MkFilterInterpretedFn: the C++ builds its filter_interpreted_fn without
// check_kind.
func (p *BoundRelationPluginRel) MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error) {
	m := p.manager.manager
	ref, err := m.NewRef(condition)
	if err != nil {
		return nil, err
	}
	p.manager.fnRoots = append(p.manager.fnRoots, ref)
	return func(a RelationBase) error {
		x, ok := p.get(a)
		if !ok {
			return ErrInstruction
		}
		_, err := x.value.FilterInterpreted(m, condition)
		return err
	}, nil
}

type boundRelationValue struct {
	vectorRelationBase
	plugin *BoundRelationPluginRel
	value  *BoundRelation
}

func (r *boundRelationValue) RelationPlugin() RelationPlugin { return r.plugin }
func (r *boundRelationValue) Empty() (bool, error)           { return r.value.Empty(), nil }
func (r *boundRelationValue) FastEmpty() bool                { return r.value.Empty() }
func (r *boundRelationValue) IsPrecise() bool                { return false }
func (r *boundRelationValue) Len() (int, error)              { return r.sizeEstimate(r.value.Empty()), nil }
func (r *boundRelationValue) SizeEstimateRows() (uint32, error) {
	return math.MaxUint32, nil
}
func (r *boundRelationValue) SizeEstimateBytes() (uint32, error) {
	return math.MaxUint32, nil
}
func (r *boundRelationValue) Display(out io.Writer) error { return r.value.Display(out) }
func (r *boundRelationValue) ToFormula() (ast.ASTRef, error) {
	return r.value.ToFormula(r.plugin.manager.manager, r.sorts)
}

func (r *boundRelationValue) Reset() error {
	r.value.reset()
	return nil
}

func (r *boundRelationValue) CloneRelation() (RelationBase, error) {
	return r.plugin.wrap(r.sorts, r.value.Clone()), nil
}

func (r *boundRelationValue) CloneFinite() (FiniteTable, error) { return r.CloneRelation() }

// AddFactAST is bound_relation::add_fact.
func (r *boundRelationValue) AddFactAST(values []ast.ASTHandle) error {
	return r.value.AddFact(values)
}

func (r *boundRelationValue) Add(fact Fact) (bool, error) {
	return true, r.value.AddFact(make([]ast.ASTHandle, len(fact)))
}

func (r *boundRelationValue) Contains(fact Fact) (bool, error) {
	return !r.value.Empty(), nil
}

// RelationAddFactAST is relation_base::add_fact(relation_fact) for the
// relation plugins that take expression facts.
type RelationAddFactAST interface {
	AddFactAST(values []ast.ASTHandle) error
}

var _ RelationBase = (*intervalRelationValue)(nil)
var _ RelationBase = (*boundRelationValue)(nil)
var _ RelationPlugin = (*IntervalRelationPlugin)(nil)
var _ RelationPlugin = (*BoundRelationPluginRel)(nil)

// ReplacePlaceholderWithRelation is get_relation for a predicate whose
// table is still a placeholder created before the relation plugins were
// registered: when mk_empty_relation now selects a relation plugin, the
// placeholder becomes that plugin's empty relation. It returns the
// predicate's current relation.
func (m *RelationManager) ReplacePlaceholderWithRelation(predicate ast.ASTHandle) (FiniteTable, error) {
	table, found, err := m.TryGetFiniteRelation(predicate)
	if err != nil || !found {
		return table, err
	}
	if !m.placeholders[predicate] {
		return table, nil
	}
	if _, ok := AsRelation(table); ok {
		// A relation placeholder is replaced only when it is the empty
		// product relation mk_empty_relation made without a kind: the
		// inherited kind's relation takes its place.
		if pr, isProduct := table.(*productRelation); !isProduct || len(pr.relations) != 0 {
			return table, nil
		}
	}
	// Still a placeholder: a check table's mk_empty line comes when
	// get_relation creates the relation (MaterializeFiniteRelation).
	m.reservingPlaceholder = true
	m.beginPlaceholder()
	relation, ok, err := m.relationForPredicate(predicate)
	m.endPlaceholder()
	m.reservingPlaceholder = false
	if err != nil || !ok {
		return table, err
	}
	if err = m.ReplaceFiniteRelation(predicate, relation); err != nil {
		return table, errors.Join(err, relation.Close())
	}
	return relation, nil
}
