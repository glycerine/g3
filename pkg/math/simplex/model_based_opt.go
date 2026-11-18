package simplex

// Source: src/math/simplex/model_based_opt.h/.cpp. The source's numerals are
// rationals; here they are rationals of the RationalManager the caller hands
// to NewModelBasedOpt (qe passes the AST manager's NumeralService().Manager,
// z3's g_mpq_manager). Every rational a ModelBasedOpt holds is freed when it
// is overwritten or by Close.

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/util/numeral"
)

type Relation uint8

const (
	Equal Relation = iota
	Less
	LessEqual
	Divides
)

func (r Relation) String() string {
	switch r {
	case Equal:
		return "="
	case Less:
		return "<"
	case LessEqual:
		return "<="
	case Divides:
		return "mod"
	default:
		return "?"
	}
}

// LinearTerm is model_based_opt::var: a variable and its rational
// coefficient. The coefficient is owned by the term (Del frees it).
type LinearTerm struct {
	Variable    uint32
	Coefficient numeral.Rational
}

// NewLinearTerm returns var(variable, coefficient) with its own copy of the
// coefficient (nil means 0).
func NewLinearTerm(q *numeral.RationalManager, variable uint32, coefficient *numeral.Rational) (LinearTerm, error) {
	t := LinearTerm{Variable: variable, Coefficient: numeral.NewRational()}
	if coefficient != nil {
		if err := q.Copy(&t.Coefficient, coefficient); err != nil {
			return t, errors.Join(err, q.Del(&t.Coefficient))
		}
	}
	return t, nil
}

// Del frees the term's coefficient.
func (t *LinearTerm) Del(q *numeral.RationalManager) error {
	err := q.Del(&t.Coefficient)
	t.Coefficient = numeral.NewRational()
	return err
}

// DelTerms frees every term of v, last to first.
func DelTerms(q *numeral.RationalManager, v []LinearTerm) error {
	var err error
	for i := len(v) - 1; i >= 0; i-- {
		err = errors.Join(err, v[i].Del(q))
	}
	return err
}

// OptRow is model_based_opt::row. Its rationals are owned by the row.
type OptRow struct {
	Terms                    []LinearTerm
	Constant, Modulus, Value numeral.Rational
	Relation                 Relation
	Alive                    bool
}

func newOptRow() OptRow {
	// row(): m_type(t_le), m_value(0), m_alive(false) (AUDIT.md math_misc_02-F4).
	return OptRow{Constant: numeral.NewRational(), Modulus: numeral.NewRational(), Value: numeral.NewRational(), Relation: LessEqual}
}

// Del frees the row's rationals (~row).
func (r *OptRow) Del(q *numeral.RationalManager) error {
	err := DelTerms(q, r.Terms)
	err = errors.Join(err, q.Del(&r.Value), q.Del(&r.Modulus), q.Del(&r.Constant))
	*r = newOptRow()
	return err
}

// DelRows frees every row of v (the vector<row> destructor).
func DelRows(q *numeral.RationalManager, v []OptRow) error {
	var err error
	for i := range v {
		err = errors.Join(err, v[i].Del(q))
	}
	return err
}

// Optimum is maximize's inf_eps result: oo when Infinite, otherwise
// Value + Epsilon*epsilon. Its rationals are owned by the holder.
type Optimum struct {
	Infinite       bool
	Value, Epsilon numeral.Rational
}

func NewOptimum() Optimum {
	return Optimum{Value: numeral.NewRational(), Epsilon: numeral.NewRational()}
}

// Set copies src into o (the inf_eps assignment).
func (o *Optimum) Set(q *numeral.RationalManager, src *Optimum) error {
	o.Infinite = src.Infinite
	if err := q.Copy(&o.Value, &src.Value); err != nil {
		return err
	}
	return q.Copy(&o.Epsilon, &src.Epsilon)
}

// Del frees o's rationals and leaves it a valid 0.
func (o *Optimum) Del(q *numeral.RationalManager) error {
	err := errors.Join(q.Del(&o.Epsilon), q.Del(&o.Value))
	*o = NewOptimum()
	return err
}

type ModelBasedOpt struct {
	c         ratCalc
	rows      []OptRow
	values    []numeral.Rational
	integers  []bool
	incidence [][]uint32
	retired   []uint32
}

// NewModelBasedOpt returns an optimizer whose rationals live on q.
func NewModelBasedOpt(q *numeral.RationalManager) *ModelBasedOpt {
	return &ModelBasedOpt{c: ratCalc{q: q}, rows: []OptRow{newOptRow()}}
}

// Close is ~model_based_opt: it frees the rows and the variable values.
func (m *ModelBasedOpt) Close() error {
	if m == nil {
		return nil
	}
	err := m.c.take()
	for i := range m.rows {
		err = errors.Join(err, m.rows[i].Del(m.c.q))
	}
	m.rows = nil
	for i := range m.values {
		err = errors.Join(err, m.c.q.Del(&m.values[i]))
	}
	m.values = nil
	return err
}

// result returns err joined with the first failed rational operation.
func (m *ModelBasedOpt) result(err error) error { return m.c.joined(err) }

// AddVariable is add_var(value, is_int).
func (m *ModelBasedOpt) AddVariable(value *numeral.Rational, integer bool) (uint32, error) {
	id := m.addVariable(value, integer)
	return id, m.result(nil)
}
func (m *ModelBasedOpt) addVariable(value *numeral.Rational, integer bool) uint32 {
	q := numeral.NewRational()
	if value != nil {
		m.c.set(&q, value)
	}
	id := uint32(len(m.values))
	m.values = append(m.values, q)
	m.integers = append(m.integers, integer)
	m.incidence = append(m.incidence, nil)
	return id
}

// Value is get_value(v): it sets the caller-owned out to v's value.
func (m *ModelBasedOpt) Value(v uint32, out *numeral.Rational) error {
	if int(v) >= len(m.values) {
		return fmt.Errorf("simplex: invalid model variable")
	}
	m.c.set(out, &m.values[v])
	return m.result(nil)
}

// rowValue sets out to r's value under the current model.
func (m *ModelBasedOpt) rowValue(r *OptRow, out *numeral.Rational) {
	m.c.set(out, &r.Constant)
	for i := range r.Terms {
		m.c.addMul(out, out, &r.Terms[i].Coefficient, &m.values[r.Terms[i].Variable])
	}
}

// canonical sums the coefficients of first and of second scaled by scale
// (nil: unscaled) per variable, in increasing variable order, dropping zero
// sums. The result is fresh; the inputs are not modified.
func (m *ModelBasedOpt) canonical(first, second []LinearTerm, scale *numeral.Rational) ([]LinearTerm, error) {
	index := make(map[uint32]int)
	var order []uint32
	var sums []numeral.Rational
	defer func() { m.c.delAll(sums) }()
	product := numeral.NewRational()
	defer m.c.del(&product)
	for pass, terms := range [][]LinearTerm{first, second} {
		for i := range terms {
			t := &terms[i]
			if int(t.Variable) >= len(m.values) {
				return nil, fmt.Errorf("simplex: invalid model variable")
			}
			k, ok := index[t.Variable]
			if !ok {
				k = len(sums)
				index[t.Variable] = k
				sums = append(sums, numeral.NewRational())
				order = append(order, t.Variable)
			}
			if pass == 1 && scale != nil {
				m.c.mul(&product, &t.Coefficient, scale)
				m.c.add(&sums[k], &sums[k], &product)
			} else {
				m.c.add(&sums[k], &sums[k], &t.Coefficient)
			}
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	out := make([]LinearTerm, 0, len(order))
	for _, v := range order {
		s := &sums[index[v]]
		if m.c.sign(s) != 0 {
			out = append(out, LinearTerm{Variable: v, Coefficient: m.c.clone(s)})
		}
	}
	return out, nil
}

// setTerms replaces r's terms with terms, freeing the old ones.
func (m *ModelBasedOpt) setTerms(r *OptRow, terms []LinearTerm) {
	m.c.fail(DelTerms(m.c.q, r.Terms))
	r.Terms = terms
}

func (m *ModelBasedOpt) newRow() uint32 {
	if len(m.retired) > 0 {
		id := m.retired[len(m.retired)-1]
		m.retired = m.retired[:len(m.retired)-1]
		m.c.fail(m.rows[id].Del(m.c.q))
		return id
	}
	id := uint32(len(m.rows))
	m.rows = append(m.rows, newOptRow())
	return id
}
func (m *ModelBasedOpt) setRow(id uint32, terms []LinearTerm, constant, modulus *numeral.Rational, relation Relation) error {
	canonical, err := m.canonical(terms, nil, nil)
	if err != nil {
		return err
	}
	// The new constant and modulus are copied before the old row dies: the
	// caller may pass another row's rationals.
	c, mod := numeral.NewRational(), numeral.NewRational()
	if constant != nil {
		m.c.set(&c, constant)
	}
	if modulus != nil {
		m.c.set(&mod, modulus)
	}
	r := &m.rows[id]
	m.c.fail(r.Del(m.c.q))
	r.Terms = canonical
	r.Relation = relation
	r.Alive = true
	r.Constant, r.Modulus = c, mod
	integerRow := true
	for i := range canonical {
		t := &canonical[i]
		integerRow = integerRow && m.integers[t.Variable]
		if m.integers[t.Variable] && !m.c.isInt(&t.Coefficient) {
			return fmt.Errorf("simplex: nonintegral integer coefficient")
		}
	}
	m.rowValue(r, &r.Value)
	if integerRow && relation == Less {
		one := m.c.int64(1)
		r.Relation = LessEqual
		m.c.add(&r.Constant, &r.Constant, &one)
		m.c.add(&r.Value, &r.Value, &one)
		m.c.del(&one)
	}
	return nil
}
func (m *ModelBasedOpt) AddConstraint(terms []LinearTerm, constant *numeral.Rational, relation Relation) error {
	return m.result(m.addConstraint(terms, constant, nil, relation))
}
func (m *ModelBasedOpt) AddDivides(terms []LinearTerm, constant, modulus *numeral.Rational) error {
	return m.result(m.addConstraint(terms, constant, modulus, Divides))
}
func (m *ModelBasedOpt) addConstraint(terms []LinearTerm, constant, modulus *numeral.Rational, relation Relation) error {
	id := m.newRow()
	if err := m.setRow(id, terms, constant, modulus, relation); err != nil {
		m.retire(id)
		return err
	}
	for _, t := range m.rows[id].Terms {
		m.incidence[t.Variable] = append(m.incidence[t.Variable], id)
	}
	if err := m.checkRow(id); err != nil {
		m.retire(id)
		return err
	}
	return nil
}

// addDividesOwned is addConstraint(terms, constant, modulus, Divides) for
// fresh terms and constant, which it frees.
func (m *ModelBasedOpt) addDividesOwned(terms []LinearTerm, constant *numeral.Rational, modulus *numeral.Rational) error {
	err := m.addConstraint(terms, constant, modulus, Divides)
	m.c.fail(DelTerms(m.c.q, terms))
	m.c.del(constant)
	return err
}
func (m *ModelBasedOpt) SetObjective(terms []LinearTerm, constant *numeral.Rational) error {
	return m.result(m.setRow(0, terms, constant, nil, LessEqual))
}

// coefficient returns a fresh copy of x's coefficient in row (0 if absent).
func (m *ModelBasedOpt) coefficient(row uint32, v uint32) numeral.Rational {
	r := &m.rows[row]
	i := sort.Search(len(r.Terms), func(i int) bool { return r.Terms[i].Variable >= v })
	if i < len(r.Terms) && r.Terms[i].Variable == v {
		return m.c.clone(&r.Terms[i].Coefficient)
	}
	return numeral.NewRational()
}
func (m *ModelBasedOpt) checkRow(id uint32) error {
	r := &m.rows[id]
	if id == 0 {
		return nil
	}
	switch r.Relation {
	case Equal:
		if m.c.sign(&r.Value) != 0 {
			return fmt.Errorf("simplex: equality not satisfied by model")
		}
	case Less:
		if m.c.sign(&r.Value) >= 0 {
			return fmt.Errorf("simplex: strict inequality not satisfied by model")
		}
	case LessEqual:
		if m.c.sign(&r.Value) > 0 {
			return fmt.Errorf("simplex: inequality not satisfied by model")
		}
	case Divides:
		if m.c.sign(&r.Modulus) == 0 {
			return fmt.Errorf("simplex: divisibility not satisfied by model")
		}
		residue := m.ratMod(&r.Value, &r.Modulus)
		zero := m.c.sign(&residue) == 0
		m.c.del(&residue)
		if !zero {
			return fmt.Errorf("simplex: divisibility not satisfied by model")
		}
	}
	return nil
}

// ratMod returns a fresh a mod |b| for integral a and nonzero integral b,
// and a copy of a otherwise.
func (m *ModelBasedOpt) ratMod(a, b *numeral.Rational) numeral.Rational {
	out := m.c.clone(a)
	if !m.c.isInt(a) || !m.c.isInt(b) || m.c.sign(b) == 0 {
		return out
	}
	modulus := m.c.clone(b)
	m.c.abs(&modulus)
	m.c.mod(&out, a, &modulus)
	m.c.del(&modulus)
	return out
}
func (m *ModelBasedOpt) retire(id uint32) {
	if id == 0 || !m.rows[id].Alive {
		return
	}
	m.rows[id].Alive = false
	m.retired = append(m.retired, id)
}

func (m *ModelBasedOpt) combine(dst uint32, c *numeral.Rational, src uint32, sameSign bool) error {
	// mul_add: if (c.is_zero()) return;
	if m.c.sign(c) == 0 {
		return nil
	}
	a := &m.rows[dst]
	b := &m.rows[src]
	// mul_add records row_id1 in m_var2row_ids only for the variables of
	// row2 that row1 did not have (AUDIT.md math_misc_02-F4).
	present := make(map[uint32]bool, len(a.Terms))
	for _, t := range a.Terms {
		present[t.Variable] = true
	}
	canonical, err := m.canonical(a.Terms, b.Terms, c)
	if err != nil {
		return err
	}
	m.setTerms(a, canonical)
	m.c.addMul(&a.Constant, &a.Constant, c, &b.Constant)
	m.c.addMul(&a.Value, &a.Value, c, &b.Value)
	if !sameSign && b.Relation == Less {
		a.Relation = Less
	} else if sameSign && a.Relation == Less && b.Relation == Less {
		a.Relation = LessEqual
	}
	if dst != 0 {
		for _, t := range b.Terms {
			if !present[t.Variable] {
				m.incidence[t.Variable] = append(m.incidence[t.Variable], dst)
			}
		}
	}
	return m.checkRow(dst)
}
func (m *ModelBasedOpt) resolve(source uint32, a1 *numeral.Rational, destination, x uint32) error {
	if !m.rows[destination].Alive {
		return nil
	}
	a2 := m.coefficient(destination, x)
	defer m.c.del(&a2)
	if m.c.sign(&a2) == 0 {
		return nil
	}
	if m.integers[x] && m.rows[source].Relation == Equal {
		// Equality substitution must retain integral rows. Dividing by a1
		// produced fractional coefficients that cannot be reconstructed as
		// integer ASTs. Scale the destination and eliminate exactly instead;
		// inequality-only slack cuts do not apply to an equality source.
		factor := m.c.clone(a1)
		m.c.abs(&factor)
		defer m.c.del(&factor)
		m.scale(destination, &factor)
		if m.rows[destination].Relation == Divides {
			modulus := &m.rows[destination].Modulus
			m.c.mul(modulus, modulus, &factor)
		}
		multiple := m.c.clone(&a2)
		defer m.c.del(&multiple)
		if m.c.sign(a1) > 0 {
			m.c.neg(&multiple)
		}
		if err := m.combine(destination, &multiple, source, false); err != nil {
			return err
		}
		m.normalize(destination)
		return nil
	}
	if m.integers[x] && m.rows[source].Relation != Equal {
		return m.resolveInteger(source, a1, destination, &a2, x)
	}
	same := destination != 0 && m.c.sign(a1) == m.c.sign(&a2)
	factor := numeral.NewRational()
	defer m.c.del(&factor)
	m.c.div(&factor, &a2, a1)
	m.c.neg(&factor)
	return m.combine(destination, &factor, source, same)
}

func (m *ModelBasedOpt) scale(row uint32, c *numeral.Rational) {
	r := &m.rows[row]
	for i := range r.Terms {
		m.c.mul(&r.Terms[i].Coefficient, &r.Terms[i].Coefficient, c)
	}
	m.c.mul(&r.Constant, &r.Constant, c)
	m.c.mul(&r.Value, &r.Value, c)
}

func (m *ModelBasedOpt) addConstant(row uint32, c *numeral.Rational) {
	r := &m.rows[row]
	m.c.add(&r.Constant, &r.Constant, c)
	m.c.add(&r.Value, &r.Value, c)
}

func (m *ModelBasedOpt) normalize(row uint32) {
	r := &m.rows[row]
	if len(r.Terms) == 0 || r.Relation == Divides {
		return
	}
	if !m.c.isInt(&r.Terms[0].Coefficient) {
		return
	}
	g := m.c.clone(&r.Terms[0].Coefficient)
	defer m.c.del(&g)
	m.c.abs(&g)
	for i := 1; i < len(r.Terms) && !m.c.isOne(&g); i++ {
		if !m.c.isInt(&r.Terms[i].Coefficient) {
			return
		}
		m.c.gcd(&g, &g, &r.Terms[i].Coefficient)
	}
	if m.c.sign(&r.Constant) != 0 {
		if !m.c.isInt(&r.Constant) {
			return
		}
		m.c.gcd(&g, &g, &r.Constant)
	}
	if m.c.sign(&g) != 0 && !m.c.isOne(&g) {
		m.c.inv(&g)
		m.scale(row, &g)
	}
}

// termsWithout returns fresh copies of terms other than x's.
func (m *ModelBasedOpt) termsWithout(terms []LinearTerm, x uint32) []LinearTerm {
	out := make([]LinearTerm, 0, len(terms))
	for i := range terms {
		if terms[i].Variable != x {
			out = append(out, LinearTerm{Variable: terms[i].Variable, Coefficient: m.c.clone(&terms[i].Coefficient)})
		}
	}
	return out
}

// normalSign returns a fresh -1 for a positive q and 1 otherwise.
func (m *ModelBasedOpt) normalSign(q *numeral.Rational) numeral.Rational {
	if m.c.sign(q) > 0 {
		return m.c.int64(-1)
	}
	return m.c.int64(1)
}

func (m *ModelBasedOpt) resolveInteger(source uint32, sourceCoeff *numeral.Rational, destination uint32, destinationCoeff *numeral.Rational, x uint32) error {
	if !m.c.isInt(sourceCoeff) || !m.c.isInt(destinationCoeff) {
		return fmt.Errorf("simplex: integer resolution requires integral coefficients")
	}
	if m.c.sign(sourceCoeff) != m.c.sign(destinationCoeff) {
		if err := m.mulAddInteger(x, sourceCoeff, source, destinationCoeff, destination); err != nil {
			return err
		}
	} else {
		absSource := m.c.clone(sourceCoeff)
		m.c.abs(&absSource)
		m.scale(destination, &absSource)
		m.c.del(&absSource)
		factor := m.c.clone(destinationCoeff)
		m.c.abs(&factor)
		m.c.neg(&factor)
		err := m.combine(destination, &factor, source, false)
		m.c.del(&factor)
		if err != nil {
			return err
		}
	}
	m.normalize(destination)
	return nil
}

// mulAddInteger is model_based_opt::mul_add, the resolution for integer rows
// (src/math/simplex/model_based_opt.cpp:383-454). The caller normalizes.
func (m *ModelBasedOpt) mulAddInteger(x uint32, sourceCoeff *numeral.Rational, source uint32, destinationCoeff *numeral.Rational, destination uint32) error {
	var temps [9]numeral.Rational
	for i := range temps {
		temps[i] = numeral.NewRational()
	}
	defer m.c.delAll(temps[:])
	absSource, absDestination, slack, a, b := &temps[0], &temps[1], &temps[2], &temps[3], &temps[4]
	destinationValue, sourceValue, caseValue, product := &temps[5], &temps[6], &temps[7], &temps[8]
	m.c.set(absSource, sourceCoeff)
	m.c.abs(absSource)
	m.c.set(absDestination, destinationCoeff)
	m.c.abs(absDestination)
	one := m.c.int64(1)
	defer m.c.del(&one)
	m.c.sub(a, absSource, &one)
	m.c.sub(b, absDestination, &one)
	m.c.mul(slack, a, b)
	xValue := &m.values[x]
	m.c.mul(product, xValue, destinationCoeff)
	m.c.sub(destinationValue, &m.rows[destination].Value, product)
	m.c.mul(product, xValue, sourceCoeff)
	m.c.sub(sourceValue, &m.rows[source].Value, product)
	m.c.mul(caseValue, sourceCoeff, destinationValue)
	m.c.addMul(caseValue, caseValue, destinationCoeff, sourceValue)
	m.c.add(caseValue, caseValue, slack)
	if m.c.sign(caseValue) <= 0 || m.c.isOne(absSource) || m.c.isOne(absDestination) {
		m.scale(destination, absSource)
		m.c.neg(slack)
		m.addConstant(destination, slack)
		return m.combine(destination, absDestination, source, false)
	}

	if m.c.cmp(absDestination, absSource) <= 0 {
		z := m.ratMod(destinationValue, absDestination)
		defer m.c.del(&z)
		if m.c.sign(&z) != 0 {
			m.c.sub(&z, absDestination, &z)
		}
		constant := numeral.NewRational()
		m.c.add(&constant, &m.rows[destination].Constant, &z)
		if err := m.addDividesOwned(m.termsWithout(m.rows[destination].Terms, x), &constant, absDestination); err != nil {
			return err
		}
		m.addConstant(destination, &z)
		sign := m.normalSign(destinationCoeff)
		m.c.mul(product, sourceCoeff, &sign)
		m.c.del(&sign)
		m.scale(destination, product)
		return m.combine(destination, absDestination, source, false)
	}

	z := m.ratMod(sourceValue, absSource)
	defer m.c.del(&z)
	if m.c.sign(&z) != 0 {
		m.c.sub(&z, absSource, &z)
	}
	constant := numeral.NewRational()
	m.c.add(&constant, &m.rows[source].Constant, &z)
	if err := m.addDividesOwned(m.termsWithout(m.rows[source].Terms, x), &constant, absSource); err != nil {
		return err
	}
	m.scale(destination, absSource)
	sign := m.normalSign(sourceCoeff)
	m.c.mul(product, destinationCoeff, &sign)
	m.c.del(&sign)
	m.c.mul(a, &z, product)
	m.addConstant(destination, a)
	return m.combine(destination, product, source, false)
}

// findBound returns the selected row and a fresh copy of its x coefficient.
func (m *ModelBasedOpt) findBound(x uint32, positive bool) (uint32, numeral.Rational, []uint32, []uint32, bool) {
	selected := NullVariable
	selectedValue := numeral.NewRational()
	defer m.c.del(&selectedValue)
	selectedCoeff := numeral.NewRational()
	value := numeral.NewRational()
	defer m.c.del(&value)
	var above, below []uint32
	seen := make(map[uint32]bool)
	for _, id := range m.incidence[x] {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		r := &m.rows[id]
		if !r.Alive {
			continue
		}
		a := m.coefficient(id, x)
		if m.c.sign(&a) == 0 {
			m.c.del(&a)
			continue
		}
		if (m.c.sign(&a) > 0) == positive || r.Relation == Equal {
			m.c.div(&value, &r.Value, &a)
			m.c.sub(&value, &m.values[x], &value)
			better := selected == NullVariable || (m.c.cmp(&value, &selectedValue) == 0 && r.Relation == Less) || (positive && m.c.cmp(&value, &selectedValue) < 0) || (!positive && m.c.cmp(&value, &selectedValue) > 0)
			if better {
				if selected != NullVariable {
					above = append(above, selected)
				}
				selected = id
				m.c.set(&selectedValue, &value)
				m.c.del(&selectedCoeff)
				selectedCoeff = a
				continue
			}
			above = append(above, id)
		} else {
			below = append(below, id)
		}
		m.c.del(&a)
	}
	return selected, selectedCoeff, above, below, selected != NullVariable
}

// Maximize is model_based_opt::maximize. The caller owns the result.
func (m *ModelBasedOpt) Maximize() (Optimum, error) {
	result := NewOptimum()
	err := m.maximize(&result)
	return result, m.result(err)
}
func (m *ModelBasedOpt) maximize(result *Optimum) error {
	var trail, variables []uint32
	for len(m.rows[0].Terms) > 0 && m.c.ok() {
		last := &m.rows[0].Terms[len(m.rows[0].Terms)-1]
		variable := last.Variable
		coefficient := m.c.clone(&last.Coefficient)
		bound, a, above, below, ok := m.findBound(variable, m.c.sign(&coefficient) > 0)
		err := m.maximizeStep(variable, &coefficient, bound, &a, above, below, ok)
		m.c.del(&a)
		m.c.del(&coefficient)
		if err != nil {
			return err
		}
		if !ok {
			m.restoreBounds(variables, trail)
			result.Infinite = true
			return nil
		}
		trail = append(trail, bound)
		variables = append(variables, variable)
	}
	m.restoreBounds(variables, trail)
	m.c.set(&result.Value, &m.rows[0].Value)
	if m.rows[0].Relation == Less {
		m.c.setInt64(&result.Epsilon, -1)
	}
	return nil
}
func (m *ModelBasedOpt) maximizeStep(variable uint32, coefficient *numeral.Rational, bound uint32, a *numeral.Rational, above, below []uint32, ok bool) error {
	if !ok {
		return nil
	}
	for _, id := range above {
		if err := m.resolve(bound, a, id, variable); err != nil {
			return err
		}
	}
	for _, id := range below {
		if err := m.resolve(bound, a, id, variable); err != nil {
			return err
		}
	}
	factor := numeral.NewRational()
	defer m.c.del(&factor)
	m.c.div(&factor, coefficient, a)
	m.c.neg(&factor)
	if err := m.combine(0, &factor, bound, false); err != nil {
		return err
	}
	m.retire(bound)
	return nil
}
func (m *ModelBasedOpt) restoreBounds(vars, trail []uint32) {
	var temps [4]numeral.Rational
	for i := range temps {
		temps[i] = numeral.NewRational()
	}
	defer m.c.delAll(temps[:])
	sum, a, next, distance := &temps[0], &temps[1], &temps[2], &temps[3]
	one := m.c.int64(1)
	defer m.c.del(&one)
	two := m.c.int64(2)
	defer m.c.del(&two)
	for i := len(trail) - 1; i >= 0; i-- {
		x, rid := vars[i], trail[i]
		r := &m.rows[rid]
		m.c.set(sum, &r.Constant)
		m.c.setInt64(a, 0)
		for j := range r.Terms {
			t := &r.Terms[j]
			if t.Variable == x {
				m.c.set(a, &t.Coefficient)
			} else {
				m.c.addMul(sum, sum, &t.Coefficient, &m.values[t.Variable])
			}
		}
		m.c.div(next, sum, a)
		m.c.neg(next)
		if r.Relation == Less {
			m.c.sub(distance, &m.values[x], next)
			m.c.abs(distance)
			m.c.div(distance, distance, &two)
			if m.c.sign(distance) == 0 || m.c.cmp(distance, &one) > 0 {
				m.c.setInt64(distance, 1)
			}
			if m.c.sign(a) > 0 {
				m.c.sub(next, next, distance)
			} else {
				m.c.add(next, next, distance)
			}
		}
		m.c.set(&m.values[x], next)
	}
	for i := range m.rows {
		m.rowValue(&m.rows[i], &m.rows[i].Value)
	}
}

func (m *ModelBasedOpt) copyRow(source uint32) (uint32, error) {
	id := m.newRow()
	r := &m.rows[source]
	if err := m.setRow(id, r.Terms, &r.Constant, &r.Modulus, r.Relation); err != nil {
		m.retire(id)
		return 0, err
	}
	for _, term := range m.rows[id].Terms {
		m.incidence[term.Variable] = append(m.incidence[term.Variable], id)
	}
	return id, nil
}

func (m *ModelBasedOpt) replaceWithConstant(row, x uint32, constant *numeral.Rational) {
	r := &m.rows[row]
	coefficient := m.coefficient(row, x)
	defer m.c.del(&coefficient)
	if m.c.sign(&coefficient) == 0 {
		return
	}
	next := r.Terms[:0]
	for i := range r.Terms {
		if r.Terms[i].Variable != x {
			next = append(next, r.Terms[i])
		} else {
			m.c.fail(r.Terms[i].Del(m.c.q))
		}
	}
	for i := len(next); i < len(r.Terms); i++ {
		r.Terms[i] = LinearTerm{}
	}
	r.Terms = next
	m.c.addMul(&r.Constant, &r.Constant, &coefficient, constant)
	delta := numeral.NewRational()
	m.c.sub(&delta, constant, &m.values[x])
	m.c.addMul(&r.Value, &r.Value, &coefficient, &delta)
	m.c.del(&delta)
}

func (m *ModelBasedOpt) replaceWithAffine(row, x uint32, scale *numeral.Rational, y uint32, constant *numeral.Rational) error {
	if !m.rows[row].Alive {
		return nil
	}
	coefficient := m.coefficient(row, x)
	defer m.c.del(&coefficient)
	if m.c.sign(&coefficient) == 0 {
		return nil
	}
	m.replaceWithConstant(row, x, constant)
	yTerm := []LinearTerm{{Variable: y, Coefficient: numeral.NewRational()}}
	defer func() { m.c.fail(DelTerms(m.c.q, yTerm)) }()
	m.c.mul(&yTerm[0].Coefficient, &coefficient, scale)
	canonical, err := m.canonical(m.rows[row].Terms, yTerm, nil)
	if err != nil {
		return err
	}
	m.setTerms(&m.rows[row], canonical)
	product := numeral.NewRational()
	m.c.mul(&product, scale, &m.values[y])
	m.c.addMul(&m.rows[row].Value, &m.rows[row].Value, &coefficient, &product)
	m.c.del(&product)
	m.incidence[y] = append(m.incidence[y], row)
	return m.checkRow(row)
}

func (m *ModelBasedOpt) solveFor(row, x uint32) error {
	coefficient := m.coefficient(row, x)
	defer m.c.del(&coefficient)
	if m.c.sign(&coefficient) == 0 || m.rows[row].Relation != Equal {
		return fmt.Errorf("simplex: invalid equality projection")
	}
	absolute := m.c.clone(&coefficient)
	m.c.abs(&absolute)
	defer m.c.del(&absolute)
	if m.integers[x] && !m.c.isOne(&absolute) {
		constant := m.c.clone(&m.rows[row].Constant)
		if err := m.addDividesOwned(m.termsWithout(m.rows[row].Terms, x), &constant, &absolute); err != nil {
			return err
		}
	}
	seen := map[uint32]bool{row: true}
	for _, other := range m.incidence[x] {
		if seen[other] {
			continue
		}
		seen[other] = true
		if !m.rows[other].Alive {
			continue
		}
		c := m.coefficient(other, x)
		nonzero := m.c.sign(&c) != 0
		m.c.del(&c)
		if nonzero {
			if err := m.resolve(row, &coefficient, other, x); err != nil {
				return err
			}
		}
	}
	m.retire(row)
	return nil
}

// lcm sets divisor to lcm(divisor, b) for integral b (0 when either is 0).
func (m *ModelBasedOpt) lcm(divisor, b *numeral.Rational) error {
	if !m.c.isInt(divisor) || !m.c.isInt(b) {
		return fmt.Errorf("simplex: divisibility modulus must be integral")
	}
	if m.c.sign(divisor) == 0 || m.c.sign(b) == 0 {
		m.c.setInt64(divisor, 0)
		return nil
	}
	bb := m.c.clone(b)
	m.c.abs(&bb)
	m.c.abs(divisor)
	m.c.lcm(divisor, divisor, &bb)
	m.c.del(&bb)
	return nil
}

func (m *ModelBasedOpt) solveMod(x uint32, rows []uint32) error {
	divisor := m.c.int64(1)
	defer m.c.del(&divisor)
	for _, row := range rows {
		if err := m.lcm(&divisor, &m.rows[row].Modulus); err != nil {
			return err
		}
	}
	if m.c.sign(&divisor) == 0 {
		return fmt.Errorf("simplex: modulo zero is not defined")
	}
	residue := m.ratMod(&m.values[x], &divisor)
	defer m.c.del(&residue)
	for _, row := range rows {
		m.replaceWithConstant(row, x, &residue)
		if err := m.checkRow(row); err != nil {
			return err
		}
	}
	newValue := numeral.NewRational()
	m.c.sub(&newValue, &m.values[x], &residue)
	m.c.div(&newValue, &newValue, &divisor)
	integral := m.c.isInt(&newValue)
	if !integral {
		m.c.del(&newValue)
		return fmt.Errorf("simplex: modular projection produced a nonintegral quotient")
	}
	y := m.addVariable(&newValue, true)
	m.c.del(&newValue)
	seen := make(map[uint32]bool)
	for _, row := range m.incidence[x] {
		if seen[row] {
			continue
		}
		seen[row] = true
		if err := m.replaceWithAffine(row, x, &divisor, y, &residue); err != nil {
			return err
		}
	}
	return m.projectOne(y)
}

// Project eliminates the variables in order while preserving the rows selected
// by the current model, matching the source model-based projection algorithm.
func (m *ModelBasedOpt) Project(variables ...uint32) error {
	for _, variable := range variables {
		if int(variable) >= len(m.values) {
			return m.result(fmt.Errorf("simplex: invalid model variable"))
		}
		if err := m.projectOne(variable); err != nil {
			return m.result(err)
		}
	}
	return m.result(nil)
}

func (m *ModelBasedOpt) projectOne(x uint32) error {
	if !m.c.ok() {
		return nil
	}
	var upper, lower, modular []uint32
	upperIndex, lowerIndex := NullVariable, NullVariable
	upperValue, lowerValue, bound := numeral.NewRational(), numeral.NewRational(), numeral.NewRational()
	defer m.c.del(&bound)
	defer m.c.del(&lowerValue)
	defer m.c.del(&upperValue)
	upperStrict, lowerStrict := false, false
	// The retained source initializes both flags false and updates them with
	// &=, leaving the small cross-product shortcut disabled for integers.
	upperUnit, lowerUnit := false, false
	seen := make(map[uint32]bool)
	for _, row := range m.incidence[x] {
		if seen[row] {
			continue
		}
		seen[row] = true
		r := &m.rows[row]
		if !r.Alive {
			continue
		}
		coefficient := m.coefficient(row, x)
		sign := m.c.sign(&coefficient)
		if sign == 0 {
			m.c.del(&coefficient)
			continue
		}
		if r.Relation == Equal {
			m.c.del(&coefficient)
			return m.solveFor(row, x)
		}
		if r.Relation == Divides {
			m.c.del(&coefficient)
			modular = append(modular, row)
			continue
		}
		m.c.div(&bound, &r.Value, &coefficient)
		m.c.sub(&bound, &m.values[x], &bound)
		if sign > 0 {
			if upperIndex == NullVariable || m.c.cmp(&bound, &upperValue) < 0 || (m.c.cmp(&bound, &upperValue) == 0 && r.Relation == Less && !upperStrict) {
				upperIndex, upperStrict = row, r.Relation == Less
				m.c.set(&upperValue, &bound)
			}
			upper = append(upper, row)
			upperUnit = upperUnit && m.c.isOne(&coefficient)
		} else {
			if lowerIndex == NullVariable || m.c.cmp(&bound, &lowerValue) > 0 || (m.c.cmp(&bound, &lowerValue) == 0 && r.Relation == Less && !lowerStrict) {
				lowerIndex, lowerStrict = row, r.Relation == Less
				m.c.set(&lowerValue, &bound)
			}
			lower = append(lower, row)
			lowerUnit = lowerUnit && m.c.isMinusOne(&coefficient)
		}
		m.c.del(&coefficient)
	}
	if len(modular) != 0 {
		return m.solveMod(x, modular)
	}

	representative := lowerIndex
	if len(upper) <= len(lower) {
		representative = upperIndex
	}
	all := append(append([]uint32{}, lower...), upper...)
	if representative == NullVariable {
		for _, row := range all {
			m.retire(row)
		}
		return nil
	}
	if (len(upper) <= 2 || len(lower) <= 2) && len(upper) <= 3 && len(lower) <= 3 && (!m.integers[x] || upperUnit || lowerUnit) {
		for i, upperRow := range upper {
			coefficient := m.coefficient(upperRow, x)
			err := m.resolveAll(upperRow, &coefficient, lower, x, i+1 == len(upper))
			m.c.del(&coefficient)
			if err != nil {
				return err
			}
		}
		for _, row := range upper {
			m.retire(row)
		}
		return nil
	}
	coefficient := m.coefficient(representative, x)
	defer m.c.del(&coefficient)
	for _, row := range all {
		if row != representative {
			if err := m.resolve(representative, &coefficient, row, x); err != nil {
				return err
			}
		}
	}
	m.retire(representative)
	return nil
}

// resolveAll resolves the upper row against each lower row, into a copy of
// the lower row unless this is the last upper row.
func (m *ModelBasedOpt) resolveAll(upperRow uint32, coefficient *numeral.Rational, lower []uint32, x uint32, last bool) error {
	for _, lowerRow := range lower {
		destination := lowerRow
		if !last {
			var err error
			destination, err = m.copyRow(lowerRow)
			if err != nil {
				return err
			}
		}
		if err := m.resolve(upperRow, coefficient, destination, x); err != nil {
			return err
		}
	}
	return nil
}

// LiveRows is get_live_rows: copies of the live rows, which the caller frees
// (DelRows).
func (m *ModelBasedOpt) LiveRows() ([]OptRow, error) {
	var out []OptRow
	for i := range m.rows {
		if m.rows[i].Alive {
			r := &m.rows[i]
			x := newOptRow()
			x.Terms = m.termsWithout(r.Terms, NullVariable)
			m.c.set(&x.Constant, &r.Constant)
			m.c.set(&x.Modulus, &r.Modulus)
			m.c.set(&x.Value, &r.Value)
			x.Relation = r.Relation
			x.Alive = r.Alive
			out = append(out, x)
		}
	}
	return out, m.result(nil)
}

func (m *ModelBasedOpt) DisplayRow(w io.Writer, row *OptRow) error {
	if row == nil {
		return fmt.Errorf("simplex: nil optimization row")
	}
	if err := m.displayRow(w, row); err != nil {
		return err
	}
	return m.result(nil)
}
func (m *ModelBasedOpt) displayRow(w io.Writer, row *OptRow) error {
	if row.Alive {
		if _, err := io.WriteString(w, "+ "); err != nil {
			return err
		}
	} else if _, err := io.WriteString(w, "- "); err != nil {
		return err
	}
	for i := range row.Terms {
		if i > 0 && m.c.sign(&row.Terms[i].Coefficient) > 0 {
			if _, err := io.WriteString(w, "+ "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%s* v%d ", m.c.text(&row.Terms[i].Coefficient), row.Terms[i].Variable); err != nil {
			return err
		}
	}
	if s := m.c.sign(&row.Constant); s > 0 {
		if _, err := fmt.Fprintf(w, " + %s ", m.c.text(&row.Constant)); err != nil {
			return err
		}
	} else if s < 0 {
		if _, err := fmt.Fprintf(w, "%s ", m.c.text(&row.Constant)); err != nil {
			return err
		}
	}
	// operator<<(ineq_type) pads the relation with spaces (AUDIT.md
	// math_misc_02-F4).
	relation := map[Relation]string{Equal: " = ", Less: " < ", LessEqual: " <= ", Divides: " mod "}[row.Relation]
	if row.Relation == Divides {
		_, err := fmt.Fprintf(w, "%s %s = 0; value: %s\n", relation, m.c.text(&row.Modulus), m.c.text(&row.Value))
		return err
	}
	_, err := fmt.Fprintf(w, "%s 0; value: %s\n", relation, m.c.text(&row.Value))
	return err
}

func (m *ModelBasedOpt) Display(w io.Writer) error {
	for i := range m.rows {
		if err := m.displayRow(w, &m.rows[i]); err != nil {
			return err
		}
	}
	for variable, rows := range m.incidence {
		if _, err := fmt.Fprintf(w, "%d: ", variable); err != nil {
			return err
		}
		for _, row := range rows {
			if _, err := fmt.Fprintf(w, "%d ", row); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return m.result(nil)
}
