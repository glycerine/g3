package realclosure

// Source: src/math/realclosure/realclosure.cpp:254-281, 1743-1775,
// 1967-1978, and 2018-2090.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// signCondition is one conjunct in the persistent sign assignment chain.
// Several surviving columns may share an antecedent.
type signCondition struct {
	polynomial int
	sign       int
	previous   *signCondition
	deleted    bool
}

// signDetermination holds the concrete matrix, Tarski results, polynomial
// rows, and sign-condition columns for root discrimination.
type signDetermination struct {
	matrix      signMatrix
	tarski      []int
	conditions  []*signCondition
	polynomials int
	numRoots    int
	store       *realValueStore
	prs, qs     []realPolynomial
}

// newSignDetermination starts with the 1x1 matrix [1] over the integer
// manager z; closeMatrix releases the matrix unless finish moved it.
func newSignDetermination(z *numeral.IntegerManager, numRoots int) (*signDetermination, error) {
	if numRoots < 2 {
		panic("realclosure: sign determination requires multiple roots")
	}
	s := &signDetermination{tarski: []int{numRoots}, conditions: []*signCondition{nil}, numRoots: numRoots}
	if err := s.matrix.make(z, 1, 1); err != nil {
		return nil, err
	}
	if err := z.SetInt64(s.matrix.at(0, 0), 1); err != nil {
		return nil, errors.Join(err, s.matrix.del())
	}
	return s, nil
}

// closeMatrix releases the working matrix (scoped_mpz_matrix M).
func (s *signDetermination) closeMatrix() error {
	if s == nil {
		return nil
	}
	return s.matrix.del()
}

// keepSignAssignment follows imp::keep_new_sc_assignment. At least two
// realizable children of the same old condition must survive.
func keepSignAssignment(cardinalities []int, useSquare bool) bool {
	step := 2
	if useSquare {
		step = 3
	}
	if len(cardinalities)%step != 0 {
		panic("realclosure: malformed sign cardinalities")
	}
	for i := 0; i < len(cardinalities); i += step {
		positive := 0
		for j := 0; j < step; j++ {
			if cardinalities[i+j] > 0 {
				positive++
			}
		}
		if positive >= 2 {
			return true
		}
	}
	return false
}

// advance consumes the Tarski-query results for one derivative q. The
// caller computes those results from the polynomial sequence; this method
// performs the source matrix/cardinality/column transition. A rejected q
// leaves the state intact. The returned flags are kept and all-one.
func (s *signDetermination) advance(equal, positive, negative int, expandedTarski []int) (bool, bool, error) {
	kept, allOne, _, err := s.advanceCore(equal, positive, negative, expandedTarski)
	return kept, allOne, err
}

func (s *signDetermination) advanceCore(equal, positive, negative int, expandedTarski []int) (kept, allOne bool, rows []int, err error) {
	z := s.matrix.z
	qMatrix, usefulSigns, err := signDetMatrix(z, equal, positive, negative)
	if err != nil || !usefulSigns {
		return false, false, nil, err
	}
	expanded := signMatrix{z: z}
	selected := signMatrix{z: z}
	defer func() { err = errors.Join(err, selected.del(), expanded.del(), qMatrix.del()) }()
	if err = expanded.tensorProduct(&s.matrix, &qMatrix); err != nil {
		return false, false, nil, err
	}
	if len(expandedTarski) != expanded.rows {
		panic("realclosure: Tarski result count differs from sign matrix")
	}
	cardinalities, solvable, err := expanded.solveInt(expandedTarski)
	if err != nil {
		return false, false, nil, err
	}
	if !solvable {
		panic("realclosure: sign matrix has no integer cardinalities")
	}
	for _, count := range cardinalities {
		if count < 0 || count > s.numRoots {
			panic("realclosure: invalid sign cardinality")
		}
	}
	useSquare := qMatrix.cols == 3
	if !keepSignAssignment(cardinalities, useSquare) {
		return false, false, nil, nil
	}
	columns := make([]int, 0, expanded.cols)
	conditions := make([]*signCondition, 0, expanded.cols)
	allOne = true
	for col, count := range cardinalities {
		if count == 0 {
			continue
		}
		old := col / qMatrix.cols
		sign64, e := qMatrix.int64At(1, col%qMatrix.cols)
		if e != nil {
			return false, false, nil, e
		}
		sign := int(sign64)
		conditions = append(conditions, &signCondition{polynomial: s.polynomials, sign: sign, previous: s.conditions[old]})
		columns = append(columns, col)
		if count > 1 {
			allOne = false
		}
	}
	if err = selected.filterColumns(&expanded, columns); err != nil {
		return false, false, nil, err
	}
	if rows, err = selected.independentRows(&selected); err != nil {
		return false, false, nil, err
	}
	if len(rows) != len(columns) {
		panic("realclosure: surviving sign columns are dependent")
	}
	tarski := make([]int, len(rows))
	for i, row := range rows {
		tarski[i] = expandedTarski[row]
	}
	if err = s.matrix.move(&selected); err != nil {
		return false, false, nil, err
	}
	s.tarski = tarski
	s.conditions = conditions
	s.polynomials++
	return true, allOne, rows, nil
}
