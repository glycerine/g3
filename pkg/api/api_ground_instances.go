package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// An enabled batch allocates the encoding and SAT runtime on its first
// eligible goal. Each successful refutation still validates its source input.
type groundInstanceSolver struct {
	pool             *order.GroundInstancePool
	sat              orderBooleanSolver
	variableIDs      []uint32 // zero or one plus the private SAT variable
	installed, units int
	validated        bool
}

// Release a partial installation while keeping the active resource scope
// alive. Close releases that scope's owner after the query has returned.
func (s *groundInstanceSolver) resetSAT() error {
	if err := s.sat.reset(); err != nil {
		return err
	}
	s.variableIDs = nil
	s.installed, s.units = 0, 0
	return nil
}
func (s *groundInstanceSolver) close() error {
	if err := s.sat.close(); err != nil {
		return err
	}
	*s = groundInstanceSolver{}
	return nil
}

func (s *groundInstanceSolver) prove(src *APISimpleSolver, m *ast.ASTManager, assertions []ast.ASTHandle, steps uint32, minRules int) (proved bool, err error) {
	if len(assertions) < 2 || len(assertions) > 8192 || steps == 0 {
		return false, nil
	}
	base, goal := assertions[:len(assertions)-1], assertions[len(assertions)-1]
	if !s.pool.Matches(m, base) {
		if err := s.close(); err != nil {
			return false, err
		}
		s.pool = order.NewGroundInstancePool(m, base)
	}
	if s.pool.Disabled() {
		return false, nil
	}
	if s.sat.limit == nil {
		s.sat.limit, err = control.NewReslimit(m.Budget())
		if err != nil {
			return false, err
		}
	}
	parent, limit := m.ResourceLimit(), s.sat.limit
	limit.RunSignals = parent.RunSignals
	defer func() { limit.RunSignals = nil }()
	if err := limit.Push(steps); err != nil {
		return false, err
	}
	defer limit.Pop()
	if err := parent.PushChild(limit); err != nil {
		return false, err
	}
	defer parent.PopChild()
	defer func() {
		if proved && err == nil && !s.validated {
			s.validated, err = validProofInput(m, base, limit)
			proved = s.validated
		}
		if proved && err == nil {
			proved, err = validProofInput(m, assertions[len(base):], limit)
		}
		if parent.CancelFlag() || !limit.IncOffset(0) {
			proved = false
			if errors.Is(err, sat.ErrSATSolverCanceled) {
				err = nil
			}
		}
	}()
	if parent.CancelFlag() {
		return false, nil
	}
	q, err := s.pool.Project(goal, steps, minRules, limit)
	if q == nil || err != nil {
		if s.pool.Disabled() {
			err = errors.Join(err, s.resetSAT())
			if parent.CancelFlag() || !limit.IncOffset(0) {
				// A later call can retry setup under a new resource scope.
				s.pool = nil
				s.validated = false
			}
		}
		return false, err
	}
	if !limit.IncOffset(q.Work) {
		return false, nil
	}
	conflict, complete := q.Reduce(limit)
	if !complete {
		return false, s.resetSAT()
	}
	if conflict {
		return true, nil
	}
	return s.search(src, m, q)
}

func (s *groundInstanceSolver) literal(x int32) (sat.Literal, error) {
	n := x
	if x < 0 {
		n = -x
	}
	id := s.variableIDs[n]
	if id == 0 {
		v, err := s.sat.solver.MkVar(true, true)
		if err != nil {
			return sat.DefaultLiteral(), err
		}
		id = uint32(v) + 1
		s.variableIDs[n] = id
	}
	return sat.MakeLiteral(sat.BoolVar(id-1), x < 0), nil
}

func (s *groundInstanceSolver) install(q *order.GroundInstanceQuery) (bool, error) {
	limit, solver := s.sat.limit, s.sat.solver
	if n := q.Variables + 1 - len(s.variableIDs); n > 0 {
		s.variableIDs = append(s.variableIDs, make([]uint32, n)...)
	}
	units := q.PermanentUnits()
	for i, x := range units[s.units:] {
		if i%256 == 0 && !limit.IncOffset(uint32(min(256, len(units)-s.units-i))) {
			return false, nil
		}
		n := x
		if n < 0 {
			n = -n
		}
		// An unmapped variable has no unsatisfied clause in the old SAT
		// database. Its value is substituted directly in all future rows.
		if id := s.variableIDs[n]; id != 0 {
			if err := solver.MkClause([]sat.Literal{sat.MakeLiteral(sat.BoolVar(id-1), x < 0)}); err != nil {
				return false, err
			}
		}
	}
	s.units = len(units)
	for i, row := range q.Clauses[s.installed:] {
		if i%256 == 0 && !limit.IncOffset(uint32(min(256, len(q.Clauses)-s.installed-i))) {
			return false, nil
		}
		var residual [3]int32
		used := 0
		satisfied := false
		for _, x := range row {
			if x == 0 {
				break
			}
			value := q.Value(x)
			if value > 0 {
				satisfied = true
				break
			}
			if value == 0 {
				residual[used] = x
				used++
			}
		}
		if satisfied {
			continue
		}
		var lits [3]sat.Literal
		for j, x := range residual[:used] {
			lit, err := s.literal(x)
			if err != nil {
				return false, err
			}
			lits[j] = lit
		}
		if err := solver.MkClause(lits[:used]); err != nil {
			return false, err
		}
	}
	s.installed = len(q.Clauses)
	return true, nil
}

func (s *groundInstanceSolver) search(src *APISimpleSolver, m *ast.ASTManager, q *order.GroundInstanceQuery) (proved bool, err error) {
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, s.resetSAT())
		}
	}()
	if s.sat.solver == nil {
		if err := s.sat.start(src, m); err != nil {
			return false, err
		}
	}
	if ok, err := s.install(q); err != nil || !ok {
		return false, err
	}
	var assumptions []sat.Literal
	if q.Value(q.Goal) == 0 {
		lit, err := s.literal(q.Goal)
		if err != nil {
			return false, err
		}
		assumptions = []sat.Literal{lit}
	}
	// Most refutations of the shared ground instances are shallow. Deeper
	// searches spend their time completing models of this relaxation, which
	// cannot establish satisfiability of the original quantified input.
	const maxDecisionLevel = 256
	result, err := s.sat.solver.CheckWithDecisionDepth(assumptions, maxDecisionLevel)
	if err != nil {
		return false, err
	}
	if err := s.sat.solver.PopToBaseLevel(); err != nil {
		return false, err
	}
	keep = s.sat.limit.IncOffset(0)
	return keep && result == lbool.False, nil
}
