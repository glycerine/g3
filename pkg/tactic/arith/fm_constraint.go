package arith

// Source: src/tactic/arith/fm_tactic.cpp:285-372,496-543.
import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

type FMConstraint struct {
	ID                   uint32
	Literals             []int32
	Variables            []uint32
	Coefficients         []unumeral.Rational // m_as: rationals of the manager's NumeralService().Manager
	Constant             unumeral.Rational   // m_c
	Dependency           ast.ASTDependency
	Strict, Dead, Marked bool
	bytes                uint64
	numerals             *unumeral.RationalManager // the store's NumeralService().Manager
}
type FMConstraintStore struct {
	index       *FMConstraintIndex
	diagnostics *fmConstraintDiagnostics
	self        *FMConstraintStore
	manager     *ast.ASTManager
	budget      *arena.Budget
	constraints []*FMConstraint
	freeIDs     []uint32
	nextID      uint32
	closed      bool
}

func NewFMConstraintStore(m *ast.ASTManager) (*FMConstraintStore, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	s := &FMConstraintStore{manager: m, budget: m.Budget()}
	s.self = s
	return s, nil
}
func (s *FMConstraintStore) open() error {
	if s == nil || s.self != s || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (s *FMConstraintStore) next() uint32 {
	n := len(s.freeIDs)
	if n > 0 {
		id := s.freeIDs[n-1]
		s.freeIDs = s.freeIDs[:n-1]
		return id
	}
	id := s.nextID
	s.nextID++
	return id
}

// fmCoefficientBytes is the per-coefficient charge of a constraint (the
// figure of the earlier Go rational layout, kept so budget accounting is
// unchanged).
const fmCoefficientBytes = 64

// fmRelease destroys a coefficient vector and constant built by a caller.
func fmRelease(q *unumeral.RationalManager, coefficients []unumeral.Rational, constant *unumeral.Rational) (err error) {
	for i := range coefficients {
		err = errors.Join(err, q.Del(&coefficients[i]))
	}
	if constant != nil {
		err = errors.Join(err, q.Del(constant))
	}
	return err
}

func (s *FMConstraintStore) q() *unumeral.RationalManager { return s.manager.NumeralService().Manager }

// New is mk_constraint: the constraint copies coeffs and constant.
func (s *FMConstraintStore) New(lits []int32, vars []uint32, coeffs []unumeral.Rational, constant *unumeral.Rational, strict bool, dep ast.ASTDependency) (*FMConstraint, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	if len(vars) != len(coeffs) || constant == nil {
		return nil, ast.ErrASTDomain
	}
	bytes := uint64(reflect.TypeOf(FMConstraint{}).Size()) + uint64(len(lits))*4 + uint64(len(vars))*(4+fmCoefficientBytes)
	if e := s.budget.Reserve(bytes); e != nil {
		return nil, e
	}
	// Acquire the dependency before consuming an id.  This keeps both the id
	// free-list and the allocation budget unchanged when acquisition fails.
	if e := s.manager.DependencyInc(dep); e != nil {
		s.budget.Release(bytes)
		return nil, e
	}
	q := s.q()
	c := &FMConstraint{ID: s.next(), Literals: append([]int32(nil), lits...), Variables: append([]uint32(nil), vars...), Constant: unumeral.NewRational(), Strict: strict, Dependency: dep, bytes: bytes, numerals: q}
	c.Coefficients = make([]unumeral.Rational, len(coeffs))
	for i := range c.Coefficients {
		c.Coefficients[i] = unumeral.NewRational()
	}
	for i := range coeffs {
		if err := s.diagnostics.allocatedVariable(i, vars[i]); err != nil {
			return nil, errors.Join(err, s.Delete(c))
		}
		if err := q.Copy(&c.Coefficients[i], &coeffs[i]); err != nil {
			return nil, errors.Join(err, s.Delete(c))
		}
	}
	if err := q.Copy(&c.Constant, constant); err != nil {
		return nil, errors.Join(err, s.Delete(c))
	}
	s.constraints = append(s.constraints, c)
	return c, nil
}
func (s *FMConstraintStore) Delete(c *FMConstraint) error {
	if e := s.open(); e != nil {
		return e
	}
	if c == nil || c.bytes == 0 {
		return ast.ErrASTHandle
	}
	consumed, e := s.manager.DependencyDecConsumed(c.Dependency)
	if consumed {
		c.Dependency = ast.ASTDependency{}
	}
	if e != nil {
		return e
	}
	if s.index != nil {
		s.index.Todo.Erase(c)
	}
	if e = fmRelease(s.q(), c.Coefficients, &c.Constant); e != nil {
		return e
	}
	s.freeIDs = append(s.freeIDs, c.ID)
	s.budget.Release(c.bytes)
	c.bytes = 0
	c.Dead = true
	c.Literals = nil
	c.Variables = nil
	c.Coefficients = nil
	return nil
}
func (s *FMConstraintStore) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	for _, c := range s.constraints {
		if c.bytes != 0 {
			if e := s.Delete(c); e != nil {
				return e
			}
		}
	}
	s.constraints = nil
	s.freeIDs = nil
	s.closed = true
	return nil
}
