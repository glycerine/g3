package rel

// Source: src/muz/rel/karr_relation.cpp karr_relation (the matrix
// payload and its operations) and karr_relation_plugin::dualizeI and
// dualizeH; dl_karr_relation_plugin.go holds the plugin's functors.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/math/hilbert"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var (
	ErrKarrDimension = errors.New("muz/rel: inconsistent Karr relation dimension")
	ErrKarrColumn    = errors.New("muz/rel: Karr relation column out of range")
)

// KarrMatrix is datalog::matrix: rows A*x+b = 0 (eq) or A*x+b >= 0.
type KarrMatrix struct {
	A  [][]big.Rat
	B  []big.Rat
	Eq []bool
}

func (m *KarrMatrix) Len() int {
	if m == nil {
		return 0
	}
	return len(m.A)
}

func cloneRationals(values []big.Rat) []big.Rat {
	result := make([]big.Rat, len(values))
	for i := range values {
		result[i].Set(&values[i])
	}
	return result
}

func cloneKarrMatrix(source KarrMatrix) KarrMatrix {
	result := KarrMatrix{A: make([][]big.Rat, len(source.A)), B: make([]big.Rat, len(source.B)), Eq: append([]bool(nil), source.Eq...)}
	for i := range source.A {
		result.A[i] = cloneRationals(source.A[i])
	}
	for i := range source.B {
		result.B[i].Set(&source.B[i])
	}
	return result
}

func (m *KarrMatrix) append(row []big.Rat, constant *big.Rat, equal bool) {
	m.A = append(m.A, cloneRationals(row))
	var value big.Rat
	if constant != nil {
		value.Set(constant)
	}
	m.B = append(m.B, value)
	m.Eq = append(m.Eq, equal)
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
	// q is the rational manager of the hilbert_basis rational_vectors.
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

// Clone is karr_relation::clone: a relation with r's emptiness, then
// copy(*this).
func (r *KarrRelation) Clone() *KarrRelation {
	if r == nil {
		return nil
	}
	result := &KarrRelation{dimension: r.dimension, checkpoint: r.checkpoint, q: r.q}
	result.copyFrom(r)
	return result
}

// copyFrom is karr_relation::copy: both matrices, their validity and
// m_empty.
func (r *KarrRelation) copyFrom(other *KarrRelation) {
	r.inequalities = cloneKarrMatrix(other.inequalities)
	r.basis = cloneKarrMatrix(other.basis)
	r.basisValid = other.basisValid
	r.inequalitiesValid = other.inequalitiesValid
	r.empty = other.empty
}

// AddFact is karr_relation::add_fact: x_i - n = 0 for every integer
// numeral n among the values (nil stands for any other value). The source
// asserts an empty relation only in debug builds; release builds append the
// rows to the current inequalities.
func (r *KarrRelation) AddFact(values []*big.Rat) error {
	if r == nil {
		return ErrKarrDimension
	}
	r.empty = false
	r.inequalitiesValid = true
	for i, value := range values {
		if value == nil || !value.IsInt() {
			continue
		}
		row := make([]big.Rat, len(values))
		row[i].SetInt64(1)
		r.inequalities.append(row, new(big.Rat).Neg(value), true)
	}
	return nil
}

// getIneqs is get_ineqs (init_ineqs): dualizeH of the basis when the
// inequalities are not valid.
func (r *KarrRelation) getIneqs() error {
	if r.inequalitiesValid {
		return nil
	}
	if err := dualizeH(r.q, &r.inequalities, r.basis, r.checkpoint); err != nil {
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
	feasible, err := dualizeI(r.q, &r.basis, r.inequalities, r.checkpoint)
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
func dualizeI(q *numeral.RationalManager, dst *KarrMatrix, src KarrMatrix, checkpoint func() bool) (bool, error) {
	*dst = KarrMatrix{}
	hb := hilbert.New(q, checkpoint)
	for i := range src.A {
		bound := new(big.Rat).Neg(&src.B[i])
		var err error
		if src.Eq[i] {
			err = karrHilbertAdd(q, hb.AddEQ, src.A[i], bound)
		} else {
			err = karrHilbertAdd(q, hb.AddGE, src.A[i], bound)
		}
		if err != nil {
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
		solution, initial, err := karrBasisSolution(q, hb, i)
		if err != nil {
			return true, err
		}
		if initial && firstInitial {
			dst.append(solution, big.NewRat(1, 1), true)
			firstInitial = false
		} else if !initial {
			dst.append(solution, nil, true)
		}
	}
	return true, nil
}

// dualizeH is karr_relation_plugin::dualizeH: the inequalities whose
// solutions are generated by the basis rows (a homogeneous system over the
// rows extended by their constants).
func dualizeH(q *numeral.RationalManager, dst *KarrMatrix, src KarrMatrix, checkpoint func() bool) error {
	*dst = KarrMatrix{}
	if src.Len() == 0 {
		return nil
	}
	hb := hilbert.New(q, checkpoint)
	for i := range src.A {
		v := append(cloneRationals(src.A[i]), src.B[i])
		var err error
		if src.Eq[i] {
			err = karrHilbertAdd(q, hb.AddEQ, v, nil)
		} else {
			err = karrHilbertAdd(q, hb.AddGE, v, nil)
		}
		if err != nil {
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
		solution, initial, err := karrBasisSolution(q, hb, i)
		if err != nil {
			return err
		}
		if !initial {
			last := len(solution) - 1
			dst.append(solution[:last], &solution[last], true)
		}
	}
	return nil
}

// Inequalities is get_ineqs: a copy of the (computed) inequalities.
func (r *KarrRelation) Inequalities() (KarrMatrix, error) {
	if err := r.getIneqs(); err != nil {
		return KarrMatrix{}, err
	}
	return cloneKarrMatrix(r.inequalities), nil
}

// Basis is get_basis: a copy of the (computed) basis.
func (r *KarrRelation) Basis() (KarrMatrix, error) {
	if err := r.getBasis(); err != nil {
		return KarrMatrix{}, err
	}
	return cloneKarrMatrix(r.basis), nil
}

// KarrJoin is join_fn: mk_full(nullptr, sig) then mk_join(r1, r2, ...):
// both inequality systems side by side plus x_c1 - y_c2 = 0 per joined
// column pair.
func KarrJoin(left, right *KarrRelation, leftColumns, rightColumns []uint32) (*KarrRelation, error) {
	if left == nil || right == nil || len(leftColumns) != len(rightColumns) {
		return nil, ErrKarrDimension
	}
	result := NewKarrFull(left.q, left.dimension+right.dimension, left.checkpoint)
	if left.empty || right.empty {
		result.empty = true
		return result, nil
	}
	if err := left.getIneqs(); err != nil {
		return nil, err
	}
	if err := right.getIneqs(); err != nil {
		return nil, err
	}
	m1, m2 := left.inequalities, right.inequalities
	for i := range m1.A {
		row := make([]big.Rat, result.dimension)
		copy(row, m1.A[i])
		result.inequalities.append(row, &m1.B[i], m1.Eq[i])
	}
	for i := range m2.A {
		row := make([]big.Rat, result.dimension)
		for j := range m2.A[i] {
			row[left.dimension+uint32(j)].Set(&m2.A[i][j])
		}
		result.inequalities.append(row, &m2.B[i], m2.Eq[i])
	}
	for i := range leftColumns {
		if leftColumns[i] >= left.dimension || rightColumns[i] >= right.dimension {
			return nil, ErrKarrColumn
		}
		row := make([]big.Rat, result.dimension)
		row[leftColumns[i]].SetInt64(1)
		row[left.dimension+rightColumns[i]].SetInt64(-1)
		result.inequalities.append(row, nil, true)
	}
	result.inequalitiesValid = true
	result.basisValid = false
	result.empty = false
	return result, nil
}

// Project is project_fn: mk_full then mk_project(r, cnt, cols): the basis
// rows without the removed columns (cols ascending, as the compiler's
// removed columns are).
func (r *KarrRelation) Project(removed []uint32) (*KarrRelation, error) {
	if r == nil || uint32(len(removed)) > r.dimension {
		return nil, ErrKarrDimension
	}
	result := NewKarrFull(r.q, r.dimension-uint32(len(removed)), r.checkpoint)
	if r.empty {
		result.empty = true
		return result, nil
	}
	if err := r.getBasis(); err != nil {
		return nil, err
	}
	m := r.basis
	result.basis = KarrMatrix{}
	for i := range m.A {
		row := make([]big.Rat, 0, len(m.A[i]))
		k := 0
		for j := range m.A[i] {
			if k < len(removed) && uint32(j) == removed[k] {
				k++
			} else {
				row = append(row, m.A[i][j])
			}
		}
		result.basis.append(row, &m.B[i], true)
	}
	result.basisValid = true
	result.inequalitiesValid = false
	result.empty = false
	return result, nil
}

// renameKarrMatrix is karr_relation::mk_rename(matrix&, col_cnt, cols).
func renameKarrMatrix(m *KarrMatrix, cycle []uint32) {
	for j := range m.A {
		row := m.A[j]
		var temporary big.Rat
		temporary.Set(&row[cycle[0]])
		for i := 0; i+1 < len(cycle); i++ {
			row[cycle[i]].Set(&row[cycle[i+1]])
		}
		row[cycle[len(cycle)-1]].Set(&temporary)
	}
}

// Rename is rename_fn: mk_full then mk_rename(r, col_cnt, cols).
func (r *KarrRelation) Rename(cycle []uint32) (*KarrRelation, error) {
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
	result.inequalities, result.basis = KarrMatrix{}, KarrMatrix{}
	result.inequalitiesValid = r.inequalitiesValid
	result.basisValid = r.basisValid
	if result.inequalitiesValid {
		result.inequalities = cloneKarrMatrix(r.inequalities)
		renameKarrMatrix(&result.inequalities, cycle)
	}
	if result.basisValid {
		result.basis = cloneKarrMatrix(r.basis)
		renameKarrMatrix(&result.basis, cycle)
	}
	return result, nil
}

func sameKarrRow(left, right []big.Rat) bool {
	for i := range left {
		if i >= len(right) || left[i].Cmp(&right[i]) != 0 {
			return false
		}
	}
	return true
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
	m := source.basis
	if r.empty {
		r.basis = cloneKarrMatrix(m)
		r.basisValid = true
		r.empty = false
		r.inequalitiesValid = false
		if delta != nil {
			delta.copyFrom(r)
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
			found = sameKarrRow(m.A[i], n.A[j]) && m.B[i].Cmp(&n.B[j]) == 0 && m.Eq[i] == n.Eq[j]
		}
		if !found {
			n.append(m.A[i], &m.B[i], m.Eq[i])
		}
	}
	r.inequalitiesValid = false
	if size != n.Len() {
		if delta != nil {
			delta.copyFrom(r)
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
		row := make([]big.Rat, r.dimension)
		row[columns[0]].SetInt64(1)
		row[columns[i]].SetInt64(-1)
		r.inequalities.append(row, nil, true)
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
	row := make([]big.Rat, r.dimension)
	row[column].SetInt64(1)
	r.inequalities.append(row, big.NewRat(-1, 1), true)
	r.basisValid = false
	return nil
}

// karrHilbertAdd passes a big.Rat row to hilbert_basis's rational_vector
// API (add_ge/add_eq). TEMPORARY bridge (bigB): KarrMatrix still holds
// big.Rat; it goes away when KarrMatrix holds numeral rationals.
func karrHilbertAdd(q *numeral.RationalManager, add func([]numeral.Rational, *numeral.Rational) error, row []big.Rat, bound *big.Rat) (err error) {
	values := make([]numeral.Rational, len(row))
	for i := range values {
		values[i] = numeral.NewRational()
	}
	b := numeral.NewRational()
	defer func() {
		err = errors.Join(err, q.Del(&b))
		for i := range values {
			err = errors.Join(err, q.Del(&values[i]))
		}
	}()
	for i := range row {
		if err = q.SetBigRat(&values[i], &row[i]); err != nil {
			return err
		}
	}
	if bound == nil {
		return add(values, nil)
	}
	if err = q.SetBigRat(&b, bound); err != nil {
		return err
	}
	return add(values, &b)
}

// karrBasisSolution is get_basis_solution into a big.Rat row (the same
// TEMPORARY bridge as karrHilbertAdd).
func karrBasisSolution(q *numeral.RationalManager, hb *hilbert.Basis, i uint32) (row []big.Rat, initial bool, err error) {
	values, initial, err := hb.BasisSolution(i)
	defer func() {
		for j := range values {
			err = errors.Join(err, q.Del(&values[j]))
		}
	}()
	if err != nil {
		return nil, false, err
	}
	row = make([]big.Rat, len(values))
	for j := range values {
		if err = q.BigRatOf(&values[j], &row[j]); err != nil {
			return nil, false, err
		}
	}
	return row, initial, nil
}
