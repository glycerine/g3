package smtcore

// Arithmetic helpers shared by the arithmetic, LRA, difference-logic,
// UTVPI, sequence and string theories. They were defined next to the
// replay arithmetic theory, which SMTTheoryArith replaced.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

// SMTArithmeticObjectiveValue is the concrete result of the arithmetic
// maximizer: an unbounded sentinel, or an exact tableau point. For inf_ext,
// Finite also retains the point's infinity coefficient; it must not be reduced
// to Unbounded. All big.Rat components are owned and cloned before mutation.
type SMTArithmeticObjectiveValue struct {
	Unbounded bool
	Finite    simplex.Infinitesimal
}

const (
	smtArithOpIDiv int32 = 11
	smtArithOpRem  int32 = 12
	smtArithOpMod  int32 = 13
)

// parameters returns owning copies of the source rule symbol followed by
// literal coefficients and then equality coefficients. Callers release them
// after their justification has taken its own copies.
func (a *smtArithmeticAntecedents) parameters(m *ast.ASTManager, rule string) (result []ast.ASTParameter, err error) {
	if a.numParameters() == 0 {
		return nil, nil
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, smtArithmeticReleaseParameters(m, result))
			result = nil
		}
	}()
	symbol, err := m.Symbols.Intern(rule)
	if err != nil {
		return nil, err
	}
	p, err := m.NewParameterSymbol(symbol)
	if err != nil {
		return nil, err
	}
	result = append(result, p)
	for _, coefficients := range [][]*big.Rat{a.literalCoefficients, a.equalityCoefficients} {
		for _, coefficient := range coefficients {
			p, err = m.NewParameterRat(coefficient)
			if err != nil {
				return result, err
			}
			result = append(result, p)
		}
	}
	return result, nil
}

func smtArithmeticReleaseParameters(m *ast.ASTManager, parameters []ast.ASTParameter) (err error) {
	for i := range parameters {
		err = errors.Join(err, m.DelParameter(&parameters[i]))
	}
	return
}

// Source context::is_shared common cases. consultTheory is true only for a
// single-theory class whose owner-specific sharing rule remains to be checked.
func (s *SMTInternalizerState) sharedTheoryNode(node *SMTEnode) (shared, consultTheory bool, result error) {
	if err := s.open(); err != nil {
		return false, false, err
	}
	if node == nil || node.manager != s.manager || node.root == nil {
		return false, false, ast.ErrASTDomain
	}
	root := node.root
	family := root.family
	if family == s.manager.BasicFamilyID() {
		if root.declKind == ast.ASTOpIte {
			return true, false, nil
		}
	}
	count := root.NumTheoryVars()
	if count == 0 {
		return false, false, nil
	}
	if count > 1 {
		return true, false, nil
	}
	if q := s.quantifierManager; q != nil && q.IsShared(root) {
		return true, false, nil
	}
	theory := root.theoryVariables.theory
	for _, parent := range root.sourceParents {
		family := parent.family
		if family != theory && family != s.manager.BasicFamilyID() {
			return true, false, nil
		}
	}
	return false, true, nil
}

func smtSharedArithmeticNode(owner *SMTTheoryBase, node *SMTEnode, division bool) (bool, error) {
	s, err := owner.State()
	if err != nil {
		return false, err
	}
	shared, consult, err := s.sharedTheoryNode(node)
	if err != nil || !consult || !division {
		return shared, err
	}
	for _, parent := range node.root.sourceParents {
		if parent.family == owner.FamilyID() {
			if kind := parent.declKind; kind == smtArithOpDiv || kind == smtArithOpIDiv || kind == smtArithOpRem || kind == smtArithOpMod {
				return true, nil
			}
		}
	}
	return false, nil
}

type smtArithmeticEqualityCandidate struct {
	left, right SMTTheoryVar
}

type smtArithmeticSharedValueKey struct {
	sort                    ast.ASTHandle
	infinity, main, epsilon string
}

func floorSMTRational(value *big.Rat) *big.Int {
	floor := new(big.Int).Quo(value.Num(), value.Denom())
	if value.Sign() < 0 && !value.IsInt() {
		floor.Sub(floor, big.NewInt(1))
	}
	return floor
}

// Source inf_eps_rational<inf_rational>::to_string and inf_rational::to_string.
func smtArithmeticBoundName(value simplex.Infinitesimal) string {
	finite := value.Main.RatString()
	if value.Epsilon.Sign() != 0 {
		sign := " +e*"
		if value.Epsilon.Sign() < 0 {
			sign = " -e*"
		}
		finite = "(" + finite + sign + new(big.Rat).Abs(&value.Epsilon).RatString() + ")"
	}
	if value.Infinity.Sign() == 0 {
		return finite
	}
	infinity := value.Infinity.RatString() + "*oo"
	if value.Infinity.Cmp(big.NewRat(1, 1)) == 0 {
		infinity = "oo"
	} else if value.Infinity.Cmp(big.NewRat(-1, 1)) == 0 {
		infinity = "-oo"
	}
	if value.Main.Sign() == 0 && value.Epsilon.Sign() == 0 {
		return infinity
	}
	return "(" + infinity + " + " + finite + ")"
}
