package nlsat

// Source: src/nlsat/nlsat_solver.cpp var_info_collector, reorder_lt,
// heuristic_reorder, and can_reorder.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

type variableOrderScore struct {
	degree      uint32
	occurrences uint32
}

// HeuristicVariablePermutation returns the source old-variable-to-new-position
// permutation. Root atoms disable reordering because their indexed-root
// invariants depend on the existing variable order.
func (s *Solver) HeuristicVariablePermutation() (_ []uint32, supported bool, resultErr error) {
	if err := s.open(); err != nil {
		return nil, false, err
	}
	count := len(s.clauses.vars.isInt)
	for _, atom := range s.atoms.bools.atoms {
		if root, ok := atom.(*RootAtom); ok && root != nil {
			return nil, false, nil
		}
	}
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return nil, false, err
	}
	degrees, err := polynomial.NewPolynomialMaxDegrees(core)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, degrees.CloseChecked()) }()
	scores := make([]variableOrderScore, count)
	collect := func(value polynomial.Polynomial) error {
		if err := degrees.Reset(); err != nil {
			return err
		}
		if err := degrees.Init(value); err != nil {
			return err
		}
		n, err := degrees.NumVariables()
		if err != nil {
			return err
		}
		variables, err := degrees.Vars()
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			x, xErr := variables.At(i)
			if xErr != nil {
				return xErr
			}
			if uint64(x) >= uint64(len(scores)) {
				return ErrSearchOwner
			}
			degree, xErr := degrees.Degree(x)
			if xErr != nil {
				return xErr
			}
			scores[x].occurrences++
			if degree > scores[x].degree {
				scores[x].degree = degree
			}
		}
		return nil
	}
	for _, clauses := range [][]*Clause{s.clauses.originals, s.clauses.learned} {
		for _, clause := range clauses {
			for _, literal := range clause.Literals() {
				atom, atomErr := s.clauses.literalAtom(literal)
				if atomErr != nil {
					return nil, false, atomErr
				}
				switch atom := atom.(type) {
				case *InequalityAtom:
					for _, factor := range atom.factors {
						if err := collect(factor.Polynomial); err != nil {
							return nil, false, err
						}
					}
				case *RootAtom:
					return nil, false, nil
				}
			}
		}
	}
	order := make([]uint32, count)
	for i := range order {
		order[i] = uint32(i)
	}
	sort.Slice(order, func(i, j int) bool {
		left, right := scores[order[i]], scores[order[j]]
		if left.degree != right.degree {
			return left.degree > right.degree
		}
		if left.occurrences != right.occurrences {
			return left.occurrences > right.occurrences
		}
		return order[i] < order[j]
	})
	permutation := make([]uint32, count)
	for position, variable := range order {
		permutation[variable] = uint32(position)
	}
	return permutation, true, nil
}
