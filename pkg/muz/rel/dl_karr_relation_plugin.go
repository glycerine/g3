package rel

// Source: src/muz/rel/karr_relation.cpp karr_relation_plugin and
// karr_relation (filter_interpreted, is_linear, is_eq, to_formula); the
// matrix payload, dualization (hilbert basis), join, project, rename and
// union are KarrRelation's.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
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

// CanHandleSignature is karr_relation_plugin::can_handle_signature:
// context::karr(), any signature (the plugin is registered only when
// fixedpoint.xform.karr is set).
func (p *KarrRelationPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	return true, nil
}

func (p *KarrRelationPlugin) wrap(sorts []ast.ASTHandle, value *KarrRelation, fn ast.ASTHandle) *karrRelationValue {
	return &karrRelationValue{vectorRelationBase: vectorRelationBase{sorts: append([]ast.ASTHandle(nil), sorts...)}, plugin: p, value: value, fn: fn}
}

func (p *KarrRelationPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	return p.wrap(sorts, NewKarrEmpty(p.manager.manager.NumeralService().Manager, uint32(len(sorts)), p.cancel), ast.ASTHandle{}), nil
}

func (p *KarrRelationPlugin) MkFull(predicate ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	return p.wrap(sorts, NewKarrFull(p.manager.manager.NumeralService().Manager, uint32(len(sorts)), p.cancel), predicate), nil
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
		var delta *KarrRelation
		if d != nil {
			z, _ := p.get(d)
			if z == nil {
				return ErrInstruction
			}
			delta = z.value
		}
		_, err := x.value.Union(y.value, delta)
		return err
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
	// filter_equal_fn's m_value lives as long as the functor.
	q := p.manager.manager.NumeralService().Manager
	number := numeral.NewRational()
	ok, err := relationRational(p.manager.manager, value, &number)
	valid := false
	if err == nil && ok {
		valid, err = q.IsInt(&number)
	}
	if err != nil {
		return nil, errors.Join(err, q.Del(&number))
	}
	p.manager.fnClosers = append(p.manager.fnClosers, func() error { return q.Del(&number) })
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

// Reset is relation_base::reset: filter_interpreted(false), whose
// get_ineqs() runs and which adds no row (false is not linear).
func (r *karrRelationValue) Reset() error { return r.value.getIneqs() }

func (r *karrRelationValue) CloneRelation() (RelationBase, error) {
	clone, err := r.value.Clone()
	if err != nil {
		return nil, err
	}
	return r.plugin.wrap(r.sorts, clone, r.fn), nil
}

// Close is ~karr_relation: the matrices' rationals are freed.
func (r *karrRelationValue) Close() error {
	if err := r.vectorRelationBase.Close(); err != nil {
		return err
	}
	return r.value.Close()
}

func (r *karrRelationValue) CloneFinite() (FiniteTable, error) { return r.CloneRelation() }

// AddFactAST is karr_relation::add_fact: rational n per value, freed at
// the end of its iteration (here: after the rows are built).
func (r *karrRelationValue) AddFactAST(values []ast.ASTHandle) (resultErr error) {
	q := r.value.q
	numbers := make([]*numeral.Rational, len(values))
	storage := karrRationals(len(values))
	defer func() { resultErr = errors.Join(resultErr, delKarrRationals(q, storage)) }()
	for i, v := range values {
		ok, err := relationRational(r.plugin.manager.manager, v, &storage[i])
		if err != nil {
			return err
		}
		if ok {
			integer, err := q.IsInt(&storage[i])
			if err != nil {
				return err
			}
			if integer {
				numbers[i] = &storage[i]
			}
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
	q := r.value.q
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
	// matrix& M = get_ineqs(): computed once, before the conjuncts; the
	// rows go onto m_ineqs and m_basis_valid is left as it was.
	if err := r.value.getIneqs(); err != nil {
		return err
	}
	n := len(r.sorts)
	// rational one(1), mone(-1); rational n1, n2 (small, never freed).
	one := numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}
	mone := numeral.Rational{Num: numeral.NewInt(-1), Den: numeral.NewInt(1)}
	n1, n2 := numeral.NewRational(), numeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&n2), q.Del(&n1)) }()
	arith, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return err
	}
	for _, e := range conjs {
		if err := r.filterConjunct(e, n, &one, &mone, &n1, &n2, arith); err != nil {
			return err
		}
	}
	return nil
}

// filterConjunct is one iteration of filter_interpreted's loop: its
// rational b(0) and vector<rational> row(num_columns, 0) die at the end of
// the iteration; M.A.push_back(row) and M.b.push_back(...) copy.
func (r *karrRelationValue) filterConjunct(e ast.ASTHandle, n int, one, mone, n1, n2 *numeral.Rational, arith ast.FamilyID) (resultErr error) {
	m := r.plugin.manager.manager
	q := r.value.q
	row := karrRationals(n)
	b := numeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&b), delKarrRationals(q, row)) }()
	reset := func() error {
		for i := range row {
			if err := q.Reset(&row[i]); err != nil {
				return err
			}
		}
		return q.Reset(&b)
	}
	linear2 := func(e1, e2 ast.ASTHandle, m1, m2 *numeral.Rational) (bool, error) {
		if err := reset(); err != nil {
			return false, err
		}
		ok, err := r.isLinear(e1, row, &b, m1, arith)
		if err != nil || !ok {
			return false, err
		}
		return r.isLinear(e2, row, &b, m2, arith)
	}
	add := func(constant *numeral.Rational, eq bool) error {
		return r.value.inequalities.push(q, row, constant, eq)
	}
	// b - rational(1) is a temporary.
	addMinusOne := func() error {
		c := numeral.NewRational()
		err := q.Sub(&b, one, &c)
		if err == nil {
			err = add(&c, false)
		}
		return errors.Join(err, q.Del(&c))
	}
	if l, rr, ok, err := isEqParts(m, e); err != nil {
		return err
	} else if ok {
		if lin, err := linear2(l, rr, one, mone); err != nil {
			return err
		} else if lin {
			return add(&b, true)
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
			if op == 4 || op == 5 {
				return addMinusOne()
			}
			return add(&b, false)
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
				if op == 2 || op == 3 {
					return addMinusOne()
				}
				return add(&b, false)
			}
		}
	}
	if disj, l, rr, err := isBinaryOr(m, e); err != nil {
		return err
	} else if disj {
		v, ok1, err := r.isEqVar(l, n1)
		if err != nil {
			return err
		}
		w, ok2 := 0, false
		if ok1 {
			if w, ok2, err = r.isEqVar(rr, n2); err != nil {
				return err
			}
		}
		if ok1 && ok2 && v == w {
			c, err := q.Compare(n1, n2)
			if err != nil {
				return err
			}
			if c > 0 {
				if err = q.Swap(n1, n2); err != nil {
					return err
				}
			}
			if err = reset(); err != nil {
				return err
			}
			// row[v] = 1; v - n1 >= 0
			if err = q.Copy(&row[v], one); err != nil {
				return err
			}
			negated := numeral.NewRational()
			err = q.Copy(&negated, n1)
			if err == nil {
				err = q.Neg(&negated)
			}
			if err == nil {
				err = add(&negated, false)
			}
			if err = errors.Join(err, q.Del(&negated)); err != nil {
				return err
			}
			// row[v] = -1; -v + n2 >= 0
			if err = q.Copy(&row[v], mone); err != nil {
				return err
			}
			return add(n2, false)
		}
	}
	return nil
}

// isLinear is karr_relation::is_linear: row[i] += mul, b += mul*n, and
// the -mul and mul*n arguments are temporaries.
func (r *karrRelationValue) isLinear(e ast.ASTHandle, row []numeral.Rational, b, mul *numeral.Rational, arith ast.FamilyID) (_ bool, resultErr error) {
	m := r.plugin.manager.manager
	q := r.value.q
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
		return true, q.Add(&row[idx], mul, &row[idx])
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	// rational n
	n := numeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&n)) }()
	// temporary computes a temporary rational and recurses with it.
	temporary := func(f func(out *numeral.Rational) error, h ast.ASTHandle) (bool, error) {
		t := numeral.NewRational()
		err := f(&t)
		ok := false
		if err == nil {
			ok, err = r.isLinear(h, row, b, &t, arith)
		}
		return ok, errors.Join(err, q.Del(&t))
	}
	negated := func(out *numeral.Rational) error {
		if err := q.Copy(out, mul); err != nil {
			return err
		}
		return q.Neg(out)
	}
	product := func(out *numeral.Rational) error { return q.Mul(mul, &n, out) }
	if ok, err := relationRational(m, e, &n); err != nil {
		return false, err
	} else if ok {
		t := numeral.NewRational()
		err = q.Mul(mul, &n, &t)
		if err == nil {
			err = q.Add(b, &t, b)
		}
		return true, errors.Join(err, q.Del(&t))
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
		return temporary(negated, arg(1))
	case 9: // mul (binary) with a numeral factor
		if nargs != 2 {
			return false, nil
		}
		if ok, err := relationRational(m, arg(0), &n); err != nil {
			return false, err
		} else if ok {
			return temporary(product, arg(1))
		}
		if ok, err := relationRational(m, arg(1), &n); err != nil {
			return false, err
		} else if ok {
			return temporary(product, arg(0))
		}
		return false, nil
	case 8: // uminus
		if nargs != 1 {
			return false, nil
		}
		return temporary(negated, arg(0))
	}
	return false, nil
}

// isEqVar is karr_relation::is_eq: (= v n) or (= n v); n is the caller's
// rational.
func (r *karrRelationValue) isEqVar(e ast.ASTHandle, n *numeral.Rational) (int, bool, error) {
	m := r.plugin.manager.manager
	e1, e2, ok, err := isEqParts(m, e)
	if err != nil || !ok {
		return 0, false, err
	}
	if k, err := m.GetKind(e1); err != nil {
		return 0, false, err
	} else if k != ast.ASTVar {
		e1, e2 = e2, e1
	}
	if k, err := m.GetKind(e1); err != nil || k != ast.ASTVar {
		return 0, false, err
	}
	idx, err := m.GetVarIndex(e1)
	if err != nil {
		return 0, false, err
	}
	ok, err = relationRational(m, e2, n)
	if err != nil || !ok || int(idx) >= len(r.sorts) {
		return 0, false, err
	}
	return int(idx), true, nil
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
	display := func(title string, m *KarrMatrix) error {
		if _, err := io.WriteString(out, title); err != nil {
			return err
		}
		q := r.q
		for i := range m.A {
			line := ""
			for j := range m.A[i] {
				text, err := q.String(&m.A[i][j])
				if err != nil {
					return err
				}
				line += text + " "
			}
			op := " >= "
			if m.Eq[i] {
				op = " = "
			}
			// -b[i] is a temporary.
			negated := numeral.NewRational()
			err := q.Copy(&negated, &m.B[i])
			if err == nil {
				err = q.Neg(&negated)
			}
			text := ""
			if err == nil {
				text, err = q.String(&negated)
			}
			if err = errors.Join(err, q.Del(&negated)); err != nil {
				return err
			}
			line += op + text + "\n"
			if _, err := io.WriteString(out, line); err != nil {
				return err
			}
		}
		return nil
	}
	if r.inequalitiesValid {
		if err := display("ineqs:\n", &r.inequalities); err != nil {
			return err
		}
	}
	if r.basisValid {
		return display("basis:\n", &r.basis)
	}
	return nil
}
