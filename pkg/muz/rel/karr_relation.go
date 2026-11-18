package rel

// Source: src/muz/rel/karr_relation.cpp karr_relation (the matrix
// payload and its operations) and karr_relation_plugin::dualizeI and
// dualizeH; dl_karr_relation_plugin.go holds the plugin's functors.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/hilbert"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var (
	ErrKarrDimension = errors.New("muz/rel: inconsistent Karr relation dimension")
	ErrKarrColumn    = errors.New("muz/rel: Karr relation column out of range")
)

// KarrMatrix is datalog::matrix: rows A*x+b = 0 (eq) or A*x+b >= 0. Its
// rationals belong to the relation's rational manager; reset frees them.
type KarrMatrix struct {
	A  [][]numeral.Rational
	B  []numeral.Rational
	Eq []bool
}

func (m *KarrMatrix) Len() int {
	if m == nil {
		return 0
	}
	return len(m.A)
}

// karrRationals is vector<rational>(n): n zeros.
func karrRationals(n int) []numeral.Rational {
	result := make([]numeral.Rational, n)
	for i := range result {
		result[i] = numeral.NewRational()
	}
	return result
}

// delKarrRationals is ~vector<rational>.
func delKarrRationals(q *numeral.RationalManager, values []numeral.Rational) (result error) {
	for i := range values {
		result = errors.Join(result, q.Del(&values[i]))
	}
	return result
}

// cloneKarrRationals is the vector<rational> copy constructor.
func cloneKarrRationals(q *numeral.RationalManager, values []numeral.Rational) ([]numeral.Rational, error) {
	result := karrRationals(len(values))
	for i := range values {
		if err := q.Copy(&result[i], &values[i]); err != nil {
			return nil, errors.Join(err, delKarrRationals(q, result))
		}
	}
	return result, nil
}

// reset is matrix::reset (and ~matrix): the rows and constants are freed.
func (m *KarrMatrix) reset(q *numeral.RationalManager) (result error) {
	for i := range m.A {
		result = errors.Join(result, delKarrRationals(q, m.A[i]))
	}
	result = errors.Join(result, delKarrRationals(q, m.B))
	*m = KarrMatrix{}
	return result
}

// pushOwned is A.push_back(row); b.push_back(constant); eq.push_back(equal)
// for a row and constant that die right after the push: they are moved in.
func (m *KarrMatrix) pushOwned(row []numeral.Rational, constant numeral.Rational, equal bool) {
	m.A = append(m.A, row)
	m.B = append(m.B, constant)
	m.Eq = append(m.Eq, equal)
}

// push is the same push_back of rows that stay alive: they are copied.
func (m *KarrMatrix) push(q *numeral.RationalManager, row []numeral.Rational, constant *numeral.Rational, equal bool) error {
	copied, err := cloneKarrRationals(q, row)
	if err != nil {
		return err
	}
	b := numeral.NewRational()
	if err = q.Copy(&b, constant); err != nil {
		return errors.Join(err, delKarrRationals(q, copied))
	}
	m.pushOwned(copied, b, equal)
	return nil
}

// assign is matrix's operator=: the old rows are freed and other's copied.
func (m *KarrMatrix) assign(q *numeral.RationalManager, other *KarrMatrix) error {
	if err := m.reset(q); err != nil {
		return err
	}
	return m.appendMatrix(q, other)
}

// appendMatrix is matrix::append(other).
func (m *KarrMatrix) appendMatrix(q *numeral.RationalManager, other *KarrMatrix) error {
	for i := range other.A {
		if err := m.push(q, other.A[i], &other.B[i], other.Eq[i]); err != nil {
			return err
		}
	}
	return nil
}

// KarrRelation is karr_relation's state: m_empty, m_ineqs (valid when
// m_ineqs_valid) and m_basis (valid when m_basis_valid). The relation's
// signature size is dimension; checkpoint is the plugin's hilbert_basis
// m_limit.inc().
type KarrRelation struct {
	dimension         uint32
	empty             bool
	inequalities      KarrMatrix
	inequalitiesValid bool
	basis             KarrMatrix
	basisValid        bool
	checkpoint        func() bool
	// q is the rational manager of the matrices and of the hilbert_basis
	// rational_vectors.
	q *numeral.RationalManager
}

// NewKarrEmpty is karr_relation_plugin::mk_empty: m_empty, and neither
// matrix valid.
func NewKarrEmpty(q *numeral.RationalManager, dimension uint32, checkpoint func() bool) *KarrRelation {
	return &KarrRelation{dimension: dimension, empty: true, checkpoint: checkpoint, q: q}
}

// NewKarrFull is mk_full: no rows, the inequalities valid.
func NewKarrFull(q *numeral.RationalManager, dimension uint32, checkpoint func() bool) *KarrRelation {
	return &KarrRelation{dimension: dimension, inequalitiesValid: true, checkpoint: checkpoint, q: q}
}

func (r *KarrRelation) Dimension() uint32 { return r.dimension }
func (r *KarrRelation) Empty() bool       { return r == nil || r.empty }

// Close is ~karr_relation: m_ineqs and m_basis are freed.
func (r *KarrRelation) Close() error {
	if r == nil {
		return nil
	}
	return errors.Join(r.inequalities.reset(r.q), r.basis.reset(r.q))
}

// Clone is karr_relation::clone: a relation with r's emptiness, then
// copy(*this).
func (r *KarrRelation) Clone() (*KarrRelation, error) {
	if r == nil {
		return nil, nil
	}
	result := &KarrRelation{dimension: r.dimension, checkpoint: r.checkpoint, q: r.q}
	if err := result.copyFrom(r); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

// copyFrom is karr_relation::copy: both matrices, their validity and
// m_empty.
func (r *KarrRelation) copyFrom(other *KarrRelation) error {
	if err := r.inequalities.assign(r.q, &other.inequalities); err != nil {
		return err
	}
	if err := r.basis.assign(r.q, &other.basis); err != nil {
		return err
	}
	r.basisValid = other.basisValid
	r.inequalitiesValid = other.inequalitiesValid
	r.empty = other.empty
	return nil
}

// AddFact is karr_relation::add_fact: x_i - n = 0 for every integer
// numeral n among the values (nil stands for any other value). The source
// asserts an empty relation only in debug builds; release builds append the
// rows to the current inequalities.
func (r *KarrRelation) AddFact(values []*numeral.Rational) error {
	if r == nil {
		return ErrKarrDimension
	}
	r.empty = false
	r.inequalitiesValid = true
	for i, value := range values {
		if value == nil {
			continue
		}
		// row[i] = rational(1); b.push_back(-n)
		row := karrRationals(len(values))
		row[i] = numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}
		b := numeral.NewRational()
		err := r.q.Copy(&b, value)
		if err == nil {
			err = r.q.Neg(&b)
		}
		if err != nil {
			return errors.Join(err, delKarrRationals(r.q, row), r.q.Del(&b))
		}
		r.inequalities.pushOwned(row, b, true)
	}
	return nil
}

// getIneqs is get_ineqs (init_ineqs): dualizeH of the basis when the
// inequalities are not valid.
func (r *KarrRelation) getIneqs() error {
	if r.inequalitiesValid {
		return nil
	}
	if err := dualizeH(r.q, &r.inequalities, &r.basis, r.checkpoint); err != nil {
		return err
	}
	r.inequalitiesValid = true
	return nil
}

// getBasis is get_basis (init_basis): dualizeI of the inequalities when
// the basis is not valid; an infeasible system makes the relation empty.
func (r *KarrRelation) getBasis() error {
	if r.basisValid {
		return nil
	}
	feasible, err := dualizeI(r.q, &r.basis, &r.inequalities, r.checkpoint)
	if err != nil {
		return err
	}
	if feasible {
		r.basisValid = true
	} else {
		r.empty = true
	}
	return nil
}

// saturate is m_hb.saturate() inside dualizeI's and dualizeH's try: any
// exception (checked_int64 overflow) is l_undef.
func saturateKarr(basis *hilbert.Basis) lbool.LBool {
	result, err := basis.Saturate()
	if err != nil {
		return lbool.Undef
	}
	return result
}

// dualizeI is karr_relation_plugin::dualizeI: the generators of
// {x | A x + b (=, >=) 0}. It reports false when the system has no
// solution. A coefficient that is not an int64 makes add_eq/add_ge throw
// (outside the try): the error is returned.
func dualizeI(q *numeral.RationalManager, dst *KarrMatrix, src *KarrMatrix, checkpoint func() bool) (bool, error) {
	if err := dst.reset(q); err != nil {
		return true, err
	}
	hb := hilbert.New(q, checkpoint)
	for i := range src.A {
		// -src.b[i] is a temporary.
		bound := numeral.NewRational()
		err := q.Copy(&bound, &src.B[i])
		if err == nil {
			err = q.Neg(&bound)
		}
		if err == nil {
			if src.Eq[i] {
				err = hb.AddEQ(src.A[i], &bound)
			} else {
				err = hb.AddGE(src.A[i], &bound)
			}
		}
		if err = errors.Join(err, q.Del(&bound)); err != nil {
			return true, err
		}
	}
	for i := 0; len(src.A) != 0 && i < len(src.A[0]); i++ {
		hb.SetIsInt(uint32(i))
	}
	switch saturateKarr(hb) {
	case lbool.False:
		return false, nil
	case lbool.Undef:
		return true, nil
	}
	firstInitial := true
	for i := uint32(0); i < hb.BasisSize(); i++ {
		solution, initial, err := hb.BasisSolution(i)
		if err != nil {
			return true, errors.Join(err, delKarrRationals(q, solution))
		}
		if initial && firstInitial {
			dst.pushOwned(solution, numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}, true)
			firstInitial = false
		} else if !initial {
			dst.pushOwned(solution, numeral.NewRational(), true)
		} else if err = delKarrRationals(q, solution); err != nil {
			return true, err
		}
	}
	return true, nil
}

// dualizeH is karr_relation_plugin::dualizeH: the inequalities whose
// solutions are generated by the basis rows (a homogeneous system over the
// rows extended by their constants).
func dualizeH(q *numeral.RationalManager, dst *KarrMatrix, src *KarrMatrix, checkpoint func() bool) error {
	if err := dst.reset(q); err != nil {
		return err
	}
	if src.Len() == 0 {
		return nil
	}
	hb := hilbert.New(q, checkpoint)
	for i := range src.A {
		// vector<rational> v(src.A[i]); v.push_back(src.b[i]);
		v, err := cloneKarrRationals(q, src.A[i])
		if err != nil {
			return err
		}
		last := numeral.NewRational()
		v = append(v, last)
		if err = q.Copy(&v[len(v)-1], &src.B[i]); err == nil {
			if src.Eq[i] {
				err = hb.AddEQ(v, nil)
			} else {
				err = hb.AddGE(v, nil)
			}
		}
		if err = errors.Join(err, delKarrRationals(q, v)); err != nil {
			return err
		}
	}
	for i := 0; i < 1+len(src.A[0]); i++ {
		hb.SetIsInt(uint32(i))
	}
	if saturateKarr(hb) != lbool.True {
		return nil
	}
	for i := uint32(0); i < hb.BasisSize(); i++ {
		solution, initial, err := hb.BasisSolution(i)
		if err != nil {
			return errors.Join(err, delKarrRationals(q, solution))
		}
		if !initial {
			// dst.b.push_back(soln.back()); soln.pop_back(); dst.A.push_back(soln)
			last := len(solution) - 1
			dst.pushOwned(solution[:last:last], solution[last], true)
		} else if err = delKarrRationals(q, solution); err != nil {
			return err
		}
	}
	return nil
}

// KarrJoin is join_fn: mk_full(nullptr, sig) then mk_join(r1, r2, ...):
// both inequality systems side by side plus x_c1 - y_c2 = 0 per joined
// column pair.
func KarrJoin(left, right *KarrRelation, leftColumns, rightColumns []uint32) (_ *KarrRelation, resultErr error) {
	if left == nil || right == nil || len(leftColumns) != len(rightColumns) {
		return nil, ErrKarrDimension
	}
	result := NewKarrFull(left.q, left.dimension+right.dimension, left.checkpoint)
	if left.empty || right.empty {
		result.empty = true
		return result, nil
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err := left.getIneqs(); err != nil {
		return nil, err
	}
	if err := right.getIneqs(); err != nil {
		return nil, err
	}
	q := result.q
	m1, m2 := &left.inequalities, &right.inequalities
	for i := range m1.A {
		row := karrRationals(int(result.dimension))
		b := numeral.NewRational()
		var err error
		for j := range m1.A[i] {
			if err = q.Copy(&row[j], &m1.A[i][j]); err != nil {
				break
			}
		}
		if err == nil {
			err = q.Copy(&b, &m1.B[i])
		}
		if err != nil {
			return nil, errors.Join(err, delKarrRationals(q, row), q.Del(&b))
		}
		result.inequalities.pushOwned(row, b, m1.Eq[i])
	}
	for i := range m2.A {
		row := karrRationals(int(result.dimension))
		b := numeral.NewRational()
		var err error
		for j := range m2.A[i] {
			if err = q.Copy(&row[left.dimension+uint32(j)], &m2.A[i][j]); err != nil {
				break
			}
		}
		if err == nil {
			err = q.Copy(&b, &m2.B[i])
		}
		if err != nil {
			return nil, errors.Join(err, delKarrRationals(q, row), q.Del(&b))
		}
		result.inequalities.pushOwned(row, b, m2.Eq[i])
	}
	for i := range leftColumns {
		if leftColumns[i] >= left.dimension || rightColumns[i] >= right.dimension {
			return nil, ErrKarrColumn
		}
		row := karrRationals(int(result.dimension))
		row[leftColumns[i]] = numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}
		row[left.dimension+rightColumns[i]] = numeral.Rational{Num: numeral.NewInt(-1), Den: numeral.NewInt(1)}
		result.inequalities.pushOwned(row, numeral.NewRational(), true)
	}
	result.inequalitiesValid = true
	result.basisValid = false
	result.empty = false
	return result, nil
}

// Project is project_fn: mk_full then mk_project(r, cnt, cols): the basis
// rows without the removed columns (cols ascending, as the compiler's
// removed columns are).
func (r *KarrRelation) Project(removed []uint32) (_ *KarrRelation, resultErr error) {
	if r == nil || uint32(len(removed)) > r.dimension {
		return nil, ErrKarrDimension
	}
	result := NewKarrFull(r.q, r.dimension-uint32(len(removed)), r.checkpoint)
	if r.empty {
		result.empty = true
		return result, nil
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err := r.getBasis(); err != nil {
		return nil, err
	}
	q := r.q
	m := &r.basis
	if err := result.basis.reset(q); err != nil {
		return nil, err
	}
	for i := range m.A {
		row := make([]numeral.Rational, 0, len(m.A[i]))
		k := 0
		for j := range m.A[i] {
			if k < len(removed) && uint32(j) == removed[k] {
				k++
			} else {
				row = append(row, numeral.NewRational())
				if err := q.Copy(&row[len(row)-1], &m.A[i][j]); err != nil {
					return nil, errors.Join(err, delKarrRationals(q, row))
				}
			}
		}
		b := numeral.NewRational()
		if err := q.Copy(&b, &m.B[i]); err != nil {
			return nil, errors.Join(err, delKarrRationals(q, row))
		}
		result.basis.pushOwned(row, b, true)
	}
	result.basisValid = true
	result.inequalitiesValid = false
	result.empty = false
	return result, nil
}

// renameKarrMatrix is karr_relation::mk_rename(matrix&, col_cnt, cols):
// rational tmp = row[cols[0]] and the assignments are copies.
func renameKarrMatrix(q *numeral.RationalManager, m *KarrMatrix, cycle []uint32) error {
	for j := range m.A {
		row := m.A[j]
		temporary := numeral.NewRational()
		err := q.Copy(&temporary, &row[cycle[0]])
		for i := 0; err == nil && i+1 < len(cycle); i++ {
			err = q.Copy(&row[cycle[i]], &row[cycle[i+1]])
		}
		if err == nil {
			err = q.Copy(&row[cycle[len(cycle)-1]], &temporary)
		}
		if err = errors.Join(err, q.Del(&temporary)); err != nil {
			return err
		}
	}
	return nil
}

// Rename is rename_fn: mk_full then mk_rename(r, col_cnt, cols).
func (r *KarrRelation) Rename(cycle []uint32) (_ *KarrRelation, resultErr error) {
	if r == nil || len(cycle) == 0 {
		return nil, ErrKarrDimension
	}
	for _, column := range cycle {
		if column >= r.dimension {
			return nil, ErrKarrColumn
		}
	}
	result := NewKarrFull(r.q, r.dimension, r.checkpoint)
	if r.empty {
		result.empty = true
		return result, nil
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	q := r.q
	if err := errors.Join(result.inequalities.reset(q), result.basis.reset(q)); err != nil {
		return nil, err
	}
	result.inequalitiesValid = r.inequalitiesValid
	result.basisValid = r.basisValid
	if result.inequalitiesValid {
		if err := result.inequalities.appendMatrix(q, &r.inequalities); err != nil {
			return nil, err
		}
		if err := renameKarrMatrix(q, &result.inequalities, cycle); err != nil {
			return nil, err
		}
	}
	if result.basisValid {
		if err := result.basis.appendMatrix(q, &r.basis); err != nil {
			return nil, err
		}
		if err := renameKarrMatrix(q, &result.basis, cycle); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// sameKarrRow is karr_relation::same_row.
func sameKarrRow(q *numeral.RationalManager, left, right []numeral.Rational) (bool, error) {
	for i := range left {
		if i >= len(right) {
			return false, nil
		}
		equal, err := q.Equal(&left[i], &right[i])
		if err != nil || !equal {
			return false, err
		}
	}
	return true, nil
}

// Union is union_fn's mk_union(src, delta): the source's basis rows that
// r lacks are appended to r's basis, and r's inequalities are invalidated.
// delta (nil when absent) becomes a copy of r when r grows, or empty when
// src is. It reports whether r grew.
func (r *KarrRelation) Union(source, delta *KarrRelation) (bool, error) {
	if r == nil || source == nil {
		return false, ErrKarrDimension
	}
	if source.empty {
		if delta != nil {
			delta.empty = true
		}
		return false, nil
	}
	if err := source.getBasis(); err != nil {
		return false, err
	}
	q := r.q
	m := &source.basis
	if r.empty {
		// m_basis = M
		if err := r.basis.assign(q, m); err != nil {
			return false, err
		}
		r.basisValid = true
		r.empty = false
		r.inequalitiesValid = false
		if delta != nil {
			if err := delta.copyFrom(r); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	if err := r.getBasis(); err != nil {
		return false, err
	}
	n := &r.basis
	size := n.Len()
	for i := range m.A {
		found := false
		for j := 0; !found && j < size; j++ {
			same, err := sameKarrRow(q, m.A[i], n.A[j])
			if err == nil && same {
				same, err = q.Equal(&m.B[i], &n.B[j])
			}
			if err != nil {
				return false, err
			}
			found = same && m.Eq[i] == n.Eq[j]
		}
		if !found {
			if err := n.push(q, m.A[i], &m.B[i], m.Eq[i]); err != nil {
				return false, err
			}
		}
	}
	r.inequalitiesValid = false
	if size != n.Len() {
		if delta != nil {
			if err := delta.copyFrom(r); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}

// FilterIdentical is filter_identical_fn: get_ineqs, then
// x_c0 - x_ci = 0 for each further column (each invalidating the basis).
func (r *KarrRelation) FilterIdentical(columns []uint32) error {
	if r == nil || len(columns) == 0 {
		return ErrKarrDimension
	}
	for _, column := range columns {
		if column >= r.dimension {
			return ErrKarrColumn
		}
	}
	if err := r.getIneqs(); err != nil {
		return err
	}
	for i := 1; i < len(columns); i++ {
		row := karrRationals(int(r.dimension))
		row[columns[0]] = numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}
		row[columns[i]] = numeral.Rational{Num: numeral.NewInt(-1), Den: numeral.NewInt(1)}
		r.inequalities.pushOwned(row, numeral.NewRational(), true)
		r.basisValid = false
	}
	return nil
}

// FilterEqual is filter_equal_fn: for an integer numeral, get_ineqs and
// the row x_col - 1 = 0 (the source pushes -1, not the value).
func (r *KarrRelation) FilterEqual(column uint32, integerValue bool) error {
	if r == nil || column >= r.dimension {
		return ErrKarrColumn
	}
	if !integerValue {
		return nil
	}
	if err := r.getIneqs(); err != nil {
		return err
	}
	row := karrRationals(int(r.dimension))
	row[column] = numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}
	r.inequalities.pushOwned(row, numeral.Rational{Num: numeral.NewInt(-1), Den: numeral.NewInt(1)}, true)
	r.basisValid = false
	return nil
}
