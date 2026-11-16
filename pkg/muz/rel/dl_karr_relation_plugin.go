package rel

// Source: src/muz/rel/karr_relation.cpp karr_relation_plugin and
// karr_relation (filter_interpreted, is_linear, is_eq, to_formula); the
// matrix payload, dualization (hilbert basis), join, project, rename and
// union are KarrRelation's.

import (
	"errors"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// KarrRelationPlugin is karr_relation_plugin, registered by rel_context
// when context::karr() (fixedpoint.xform.karr) holds.
type KarrRelationPlugin struct {
	RelationPluginDefaults
	manager *RelationManager
	cancel  func() bool
}

func NewKarrRelationPlugin(m *RelationManager, cancel func() bool) *KarrRelationPlugin {
	return &KarrRelationPlugin{manager: m, cancel: cancel}
}

func (p *KarrRelationPlugin) Name() string              { return "karr_relation" }
func (p *KarrRelationPlugin) Manager() *RelationManager { return p.manager }

// CanHandleSignature: every column is Int.
func (p *KarrRelationPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	m := p.manager.manager
	arith, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return false, err
	}
	for _, sort := range sorts {
		kind, err := m.GetKind(sort)
		if err != nil || kind != ast.ASTSort {
			return false, err
		}
		family, err := m.GetFamilyID(sort)
		if err != nil {
			return false, err
		}
		decl, err := m.GetDeclKind(sort)
		if err != nil {
			return false, err
		}
		if family != arith || decl != 1 {
			return false, nil
		}
	}
	return true, nil
}

func (p *KarrRelationPlugin) wrap(sorts []ast.ASTHandle, value *KarrRelation, fn ast.ASTHandle) *karrRelationValue {
	return &karrRelationValue{vectorRelationBase: vectorRelationBase{sorts: append([]ast.ASTHandle(nil), sorts...)}, plugin: p, value: value, fn: fn}
}

func (p *KarrRelationPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	return p.wrap(sorts, NewKarrEmpty(uint32(len(sorts)), p.cancel), ast.ASTHandle{}), nil
}

func (p *KarrRelationPlugin) MkFull(predicate ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	return p.wrap(sorts, NewKarrFull(uint32(len(sorts)), p.cancel), predicate), nil
}

func (p *KarrRelationPlugin) get(r RelationBase) (*karrRelationValue, bool) {
	v, ok := r.(*karrRelationValue)
	return v, ok && v.plugin == p
}

func (p *KarrRelationPlugin) MkJoinFn(r1, r2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
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
		joined, err := KarrJoin(x.value, y.value, c1, c2)
		if err != nil {
			return nil, err
		}
		// mk_join: m_fn is r2's when set, else r1's.
		fn := x.fn
		if !y.fn.IsNull() {
			fn = y.fn
		}
		return p.wrap(sorts, joined, fn), nil
	}, nil
}

// MkProjectFn: the C++ returns its project_fn without check_kind.
func (p *KarrRelationPlugin) MkProjectFn(r RelationBase, removed []int) (RelationTransformerFn, error) {
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
		return p.wrap(sorts, projected, x.fn), nil
	}, nil
}

func (p *KarrRelationPlugin) MkRenameFn(r RelationBase, cycle []int) (RelationTransformerFn, error) {
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
		return p.wrap(sorts, renamed, x.fn), nil
	}, nil
}

// MkUnionFn is union_fn (karr_relation has no widening; mk_widen_fn falls
// back to the union).
func (p *KarrRelationPlugin) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	if _, ok := p.get(target); !ok {
		return nil, nil
	}
	if _, ok := p.get(source); !ok {
		return nil, nil
	}
	if delta != nil {
		if _, ok := p.get(delta); !ok {
			return nil, nil
		}
	}
	return func(t, s, d RelationBase) error {
		x, _ := p.get(t)
		y, _ := p.get(s)
		if x == nil || y == nil {
			return ErrInstruction
		}
		changed, err := x.value.Union(y.value)
		if err != nil || d == nil {
			return err
		}
		z, _ := p.get(d)
		if z == nil {
			return ErrInstruction
		}
		if y.value.Empty() {
			// delta->m_empty = true
			z.value.empty = true
			return nil
		}
		if changed {
			// delta->copy(*this)
			z.value = x.value.Clone()
		}
		return nil
	}, nil
}

func (p *KarrRelationPlugin) MkFilterIdenticalFn(t RelationBase, cols []int) (RelationMutatorFn, error) {
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

// MkFilterEqualFn is filter_equal_fn: an integer numeral adds the row
// x_col - 1 = 0 (the C++ ignores the value itself).
func (p *KarrRelationPlugin) MkFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error) {
	if _, ok := p.get(t); !ok {
		return nil, nil
	}
	n, ok, err := relationNumeral(p.manager.manager, value)
	if err != nil {
		return nil, err
	}
	valid := ok && n.IsInt()
	return func(a RelationBase) error {
		x, ok := p.get(a)
		if !ok {
			return ErrInstruction
		}
		return x.value.FilterEqual(uint32(col), valid)
	}, nil
}

func (p *KarrRelationPlugin) MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error) {
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
		return x.filterInterpreted(condition)
	}, nil
}

type karrRelationValue struct {
	vectorRelationBase
	plugin *KarrRelationPlugin
	value  *KarrRelation
	fn     ast.ASTHandle
}

func (r *karrRelationValue) RelationPlugin() RelationPlugin { return r.plugin }
func (r *karrRelationValue) Empty() (bool, error)           { return r.value.Empty(), nil }
func (r *karrRelationValue) FastEmpty() bool                { return r.value.Empty() }
func (r *karrRelationValue) IsPrecise() bool                { return false }
func (r *karrRelationValue) Len() (int, error)              { return r.sizeEstimate(r.value.Empty()), nil }
func (r *karrRelationValue) SizeEstimateRows() (uint32, error) {
	return ^uint32(0), nil
}
func (r *karrRelationValue) SizeEstimateBytes() (uint32, error) {
	return ^uint32(0), nil
}

// Display is karr_relation::display.
func (r *karrRelationValue) Display(out io.Writer) error {
	m := r.plugin.manager.manager
	if !r.fn.IsNull() {
		name, err := m.GetASTName(r.fn)
		if err != nil {
			return err
		}
		text, err := m.Symbols.Display(name)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(out, text+"\n"); err != nil {
			return err
		}
	}
	if r.value.Empty() {
		_, err := io.WriteString(out, "empty\n")
		return err
	}
	return r.value.DisplayMatrices(out)
}

func (r *karrRelationValue) ToFormula() (ast.ASTRef, error) {
	return r.value.ToFormula(r.plugin.manager.manager, r.sorts)
}

// Reset is relation_base::reset: filter_interpreted(false) adds no row
// (false is not linear), so the relation is unchanged.
func (r *karrRelationValue) Reset() error { return nil }

func (r *karrRelationValue) CloneRelation() (RelationBase, error) {
	return r.plugin.wrap(r.sorts, r.value.Clone(), r.fn), nil
}

func (r *karrRelationValue) CloneFinite() (FiniteTable, error) { return r.CloneRelation() }

// AddFactAST is karr_relation::add_fact.
func (r *karrRelationValue) AddFactAST(values []ast.ASTHandle) error {
	numbers := make([]*big.Rat, len(values))
	for i, v := range values {
		n, ok, err := relationNumeral(r.plugin.manager.manager, v)
		if err != nil {
			return err
		}
		if ok && n.IsInt() {
			numbers[i] = n
		}
	}
	return r.value.AddFact(numbers)
}

func (r *karrRelationValue) Add(fact Fact) (bool, error) {
	m := r.plugin.manager
	values := make([]ast.ASTHandle, len(fact))
	refs := make([]ast.ASTRef, 0, len(fact))
	var err error
	for i, e := range fact {
		var ref ast.ASTRef
		if ref, err = m.DecodeRelationElement(r.sorts[i], e); err != nil {
			break
		}
		refs = append(refs, ref)
		if values[i], err = m.manager.RefGet(ref); err != nil {
			break
		}
	}
	if err == nil {
		err = r.AddFactAST(values)
	}
	return true, errors.Join(err, releaseRefs(m.manager, refs))
}

// Contains: karr_relation::contains_fact is UNREACHABLE.
func (r *karrRelationValue) Contains(Fact) (bool, error) { return false, ErrInstruction }

// filterInterpreted is karr_relation::filter_interpreted.
func (r *karrRelationValue) filterInterpreted(cond ast.ASTHandle) (resultErr error) {
	m := r.plugin.manager.manager
	root, err := m.NewRef(cond)
	if err != nil {
		return err
	}
	// flatten_and(cond, conjs)
	owned, err := m.FlattenAndOwned([]ast.ASTRef{root})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseRefs(m, owned)) }()
	conjs := make([]ast.ASTHandle, len(owned))
	for i := range owned {
		if conjs[i], err = m.RefGet(owned[i]); err != nil {
			return err
		}
	}
	n := len(r.sorts)
	one, mone := big.NewRat(1, 1), big.NewRat(-1, 1)
	arith, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return err
	}
	for _, e := range conjs {
		row := make([]big.Rat, n)
		b := new(big.Rat)
		reset := func() {
			for i := range row {
				row[i].SetInt64(0)
			}
			b.SetInt64(0)
		}
		linear2 := func(e1, e2 ast.ASTHandle, m1, m2 *big.Rat) (bool, error) {
			reset()
			ok, err := r.isLinear(e1, row, b, m1, arith)
			if err != nil || !ok {
				return false, err
			}
			return r.isLinear(e2, row, b, m2, arith)
		}
		add := func(constant *big.Rat, eq bool) error {
			return KarrSourceFilterConstraint(r.value, row, constant, eq)
		}
		if l, rr, ok, err := isEqParts(m, e); err != nil {
			return err
		} else if ok {
			if lin, err := linear2(l, rr, one, mone); err != nil {
				return err
			} else if lin {
				if err = add(b, true); err != nil {
					return err
				}
				continue
			}
		}
		fid, op, a1, a2, app, err := intervalParts(m, e)
		if err != nil {
			return err
		}
		if app && fid == arith && op >= 2 && op <= 5 {
			// le: e1 <= e2 (ge: e2 >= e1 swaps), lt/gt likewise.
			e1, e2 := a1, a2
			if op == 3 || op == 5 {
				e1, e2 = a2, a1
			}
			if lin, err := linear2(e1, e2, mone, one); err != nil {
				return err
			} else if lin {
				c := new(big.Rat).Set(b)
				if op == 4 || op == 5 {
					c.Sub(c, one)
				}
				if err = add(c, false); err != nil {
					return err
				}
				continue
			}
		}
		if not, en, err := m.IsNot(e); err != nil {
			return err
		} else if not {
			fid, op, a1, a2, app, err := intervalParts(m, en)
			if err != nil {
				return err
			}
			if app && fid == arith && op >= 2 && op <= 5 {
				// not (e2 < e1) / not (e1 > e2): e1 - e2 >= 0 with b.
				// not (e2 <= e1) / not (e1 >= e2): b - 1.
				var e1, e2 ast.ASTHandle
				switch op {
				case 4, 2: // lt(e2, e1), le(e2, e1)
					e2, e1 = a1, a2
				default: // gt(e1, e2), ge(e1, e2)
					e1, e2 = a1, a2
				}
				if lin, err := linear2(e1, e2, mone, one); err != nil {
					return err
				} else if lin {
					c := new(big.Rat).Set(b)
					if op == 2 || op == 3 {
						c.Sub(c, one)
					}
					if err = add(c, false); err != nil {
						return err
					}
					continue
				}
			}
		}
		if disj, l, rr, err := isBinaryOr(m, e); err != nil {
			return err
		} else if disj {
			v, n1, ok1, err := r.isEqVar(l)
			if err != nil {
				return err
			}
			w, n2, ok2, err := r.isEqVar(rr)
			if err != nil {
				return err
			}
			if ok1 && ok2 && v == w {
				if n1.Cmp(n2) > 0 {
					n1, n2 = n2, n1
				}
				reset()
				row[v].SetInt64(1)
				if err = add(new(big.Rat).Neg(n1), false); err != nil {
					return err
				}
				row[v].SetInt64(-1)
				if err = add(n2, false); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// isLinear is karr_relation::is_linear.
func (r *karrRelationValue) isLinear(e ast.ASTHandle, row []big.Rat, b, mul *big.Rat, arith ast.FamilyID) (bool, error) {
	m := r.plugin.manager.manager
	sort, err := m.GetSort(e)
	if err != nil {
		return false, err
	}
	if fid, err := m.GetFamilyID(sort); err != nil || fid != arith {
		return false, err
	}
	if dk, err := m.GetDeclKind(sort); err != nil || dk != 1 {
		return false, err
	}
	kind, err := m.GetKind(e)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		idx, err := m.GetVarIndex(e)
		if err != nil {
			return false, err
		}
		if int(idx) >= len(row) {
			return false, ast.ErrASTDomain
		}
		row[idx].Add(&row[idx], mul)
		return true, nil
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	if n, ok, err := relationNumeral(m, e); err != nil {
		return false, err
	} else if ok {
		b.Add(b, new(big.Rat).Mul(mul, n))
		return true, nil
	}
	fid, err := m.GetFamilyID(e)
	if err != nil || fid != arith {
		return false, err
	}
	op, err := m.GetDeclKind(e)
	if err != nil {
		return false, err
	}
	nargs, err := m.GetNumArgs(e)
	if err != nil {
		return false, err
	}
	arg := func(i uint32) ast.ASTHandle {
		h, _ := m.GetArg(e, i)
		return h
	}
	switch op {
	case 6: // add
		for i := uint32(0); i < nargs; i++ {
			if ok, err := r.isLinear(arg(i), row, b, mul, arith); err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	case 7: // sub (binary)
		if nargs != 2 {
			return false, nil
		}
		if ok, err := r.isLinear(arg(0), row, b, mul, arith); err != nil || !ok {
			return false, err
		}
		return r.isLinear(arg(1), row, b, new(big.Rat).Neg(mul), arith)
	case 9: // mul (binary) with a numeral factor
		if nargs != 2 {
			return false, nil
		}
		if n, ok, err := relationNumeral(m, arg(0)); err != nil {
			return false, err
		} else if ok {
			return r.isLinear(arg(1), row, b, new(big.Rat).Mul(mul, n), arith)
		}
		if n, ok, err := relationNumeral(m, arg(1)); err != nil {
			return false, err
		} else if ok {
			return r.isLinear(arg(0), row, b, new(big.Rat).Mul(mul, n), arith)
		}
		return false, nil
	case 8: // uminus
		if nargs != 1 {
			return false, nil
		}
		return r.isLinear(arg(0), row, b, new(big.Rat).Neg(mul), arith)
	}
	return false, nil
}

// isEqVar is karr_relation::is_eq: (= v n) or (= n v).
func (r *karrRelationValue) isEqVar(e ast.ASTHandle) (int, *big.Rat, bool, error) {
	m := r.plugin.manager.manager
	e1, e2, ok, err := isEqParts(m, e)
	if err != nil || !ok {
		return 0, nil, false, err
	}
	if k, err := m.GetKind(e1); err != nil {
		return 0, nil, false, err
	} else if k != ast.ASTVar {
		e1, e2 = e2, e1
	}
	if k, err := m.GetKind(e1); err != nil || k != ast.ASTVar {
		return 0, nil, false, err
	}
	idx, err := m.GetVarIndex(e1)
	if err != nil {
		return 0, nil, false, err
	}
	n, ok, err := relationNumeral(m, e2)
	if err != nil || !ok || int(idx) >= len(r.sorts) {
		return 0, nil, false, err
	}
	return int(idx), n, true, nil
}

// isEqParts is m.is_eq(e, e1, e2).
func isEqParts(m *ast.ASTManager, e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, err := m.GetKind(e)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	fid, err := m.GetFamilyID(e)
	if err != nil || fid != m.BasicFamilyID() {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	op, err := m.GetDeclKind(e)
	if err != nil || op != int32(ast.ASTOpEq) {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	a, err := m.GetArg(e, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	b, err := m.GetArg(e, 1)
	return a, b, err == nil, err
}

// isBinaryOr is m.is_or(e, e1, e2).
func isBinaryOr(m *ast.ASTManager, e ast.ASTHandle) (bool, ast.ASTHandle, ast.ASTHandle, error) {
	kind, err := m.GetKind(e)
	if err != nil || kind != ast.ASTApp {
		return false, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	fid, err := m.GetFamilyID(e)
	if err != nil || fid != m.BasicFamilyID() {
		return false, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	op, err := m.GetDeclKind(e)
	if err != nil || op != int32(ast.ASTOpOr) {
		return false, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	n, err := m.GetNumArgs(e)
	if err != nil || n != 2 {
		return false, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	a, err := m.GetArg(e, 0)
	if err != nil {
		return false, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	b, err := m.GetArg(e, 1)
	return err == nil, a, b, err
}

var _ RelationBase = (*karrRelationValue)(nil)
var _ RelationPlugin = (*KarrRelationPlugin)(nil)

// DisplayMatrices is karr_relation::display's non-empty part: the valid
// inequalities, then the valid basis (matrix::display).
func (r *KarrRelation) DisplayMatrices(out io.Writer) error {
	display := func(title string, m KarrMatrix) error {
		if _, err := io.WriteString(out, title); err != nil {
			return err
		}
		for i := range m.A {
			line := ""
			for j := range m.A[i] {
				line += m.A[i][j].RatString() + " "
			}
			op := " >= "
			if m.Eq[i] {
				op = " = "
			}
			line += op + new(big.Rat).Neg(&m.B[i]).RatString() + "\n"
			if _, err := io.WriteString(out, line); err != nil {
				return err
			}
		}
		return nil
	}
	if r.inequalitiesValid {
		if err := display("ineqs:\n", r.inequalities); err != nil {
			return err
		}
	}
	if r.basisValid {
		return display("basis:\n", r.basis)
	}
	return nil
}
