package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Each proof batch owns its Boolean solver. Its permanent rows are an exact
// prefix of the current query in the query's own numeric variable space.
// A fresh selector guards each suffix. After a refutation, permanently
// satisfying that guard makes every clause in the old suffix redundant.
const (
	booleanProofTail   = 64 // Leave room for nearby goals that change the last clauses.
	booleanProofChecks = 32 // Bound the number of spent selectors.
)

type orderBooleanSolver struct {
	limit   *control.Reslimit
	runtime *sat.Runtime
	solver  *sat.Solver
	vars    []sat.BoolVar
	prefix  []order.BooleanClause
	checks  int
}

func (b *orderBooleanSolver) reset() error {
	if b.solver != nil {
		if err := b.solver.Close(); err != nil {
			return err
		}
		b.solver = nil
	}
	if b.runtime != nil {
		if err := b.runtime.Close(); err != nil {
			return err
		}
		b.runtime = nil
	}
	b.vars = nil
	b.prefix = nil
	b.checks = 0
	return nil
}

func (b *orderBooleanSolver) close() error {
	if err := b.reset(); err != nil {
		return err
	}
	if b.limit != nil {
		b.limit.Close()
		b.limit = nil
	}
	return nil
}

func (b *orderBooleanSolver) start(source *APISimpleSolver, m *ast.ASTManager) (err error) {
	services, err := source.owner.command.ModelEvaluatorConfig()
	if err != nil {
		return err
	}
	params, err := services.Context.NewValue()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, services.Context.Del(&params)) }()
	// Each small residual CNF has already gone through unit propagation.
	for _, key := range []string{"asymm_branch", "probing", "scc", "resolution", "subsumption", "elim_vars"} {
		if err := services.Context.SetBoolName(&params, &key, false); err != nil {
			return err
		}
	}
	b.runtime, err = sat.NewRuntime(sat.RuntimeConfig{
		Budget: m.Budget(), ParamsContext: services.Context, Globals: services.Globals,
		Process: m.NumeralService().RuntimeOwner().ProcessOwner(),
	})
	if err != nil {
		return err
	}
	b.solver, err = sat.NewSATSolver(sat.SolverConfig{
		ParamsContext: services.Context, Params: params, RLimit: b.limit,
		Budget: m.Budget(), Runtime: b.runtime, TraceTags: services.Trace,
	})
	if err != nil {
		return err
	}
	b.vars = []sat.BoolVar{0} // Input variable IDs start at one.
	return nil
}

func (b *orderBooleanSolver) add(rows []order.BooleanClause, selector sat.Literal) (bool, error) {
	var clause [4]sat.Literal
	for i, row := range rows {
		if i%256 == 0 && !b.limit.IncOffset(uint32(min(256, len(rows)-i))) {
			return false, nil
		}
		n := 0
		for _, x := range row {
			if x == 0 {
				break
			}
			v := x
			if v < 0 {
				v = -v
			}
			clause[n] = sat.MakeLiteral(b.vars[v], x < 0)
			n++
		}
		if selector != sat.DefaultLiteral() {
			clause[n] = selector
			n++
		}
		if err := b.solver.MkClause(clause[:n]); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (p *orderProofState) checkBoolean(m *ast.ASTManager, k *order.BooleanProblem, steps uint32) (bool, error) {
	return p.checkBooleanInput(m, k, steps, nil)
}

// A projection that omitted input rows can validate their binding scopes
// under the same private resource limit before reporting a refutation.
func (p *orderProofState) checkBooleanInput(m *ast.ASTManager, k *order.BooleanProblem, steps uint32, source []ast.ASTHandle) (proved bool, err error) {
	if p.bits == nil {
		p.bits = &orderBooleanSolver{}
	}
	b := p.bits
	if b.limit == nil {
		b.limit, err = control.NewReslimit(m.Budget())
		if err != nil {
			return false, err
		}
	}
	parent, limit := m.ResourceLimit(), b.limit
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
	if parent.CancelFlag() || !limit.IncOffset(k.Work) {
		return false, nil
	}
	defer func() {
		if proved && err == nil && len(source) != 0 {
			proved, err = validProofInput(m, source, limit)
		}
		if !limit.IncOffset(0) || parent.CancelFlag() {
			proved, err = false, nil
		}
	}()
	conflict, complete := k.Reduce(16, limit)
	if !complete || conflict {
		return complete && conflict, nil
	}
	if len(k.Clauses) == 0 {
		return false, nil
	}
	return b.search(p.source, m, k)
}

// Any inconclusive result makes prove release this state, including a
// partially installed prefix or an interrupted search under assumptions.
func (b *orderBooleanSolver) search(source *APISimpleSolver, m *ast.ASTManager, k *order.BooleanProblem) (bool, error) {
	limit := b.limit
	reuse := b.solver != nil && b.checks < booleanProofChecks && len(b.prefix) <= len(k.Clauses)
	if reuse {
		for i, row := range b.prefix {
			if i%256 == 0 && !limit.IncOffset(uint32(min(256, len(b.prefix)-i))) {
				return false, nil
			}
			if row != k.Clauses[i] {
				reuse = false
				break
			}
		}
	}
	if !reuse {
		if err := b.reset(); err != nil {
			return false, err
		}
		if err := b.start(source, m); err != nil {
			return false, err
		}
		cut := max(0, len(k.Clauses)-booleanProofTail)
		b.prefix = append([]order.BooleanClause(nil), k.Clauses[:cut]...)
	}
	firstNew := len(b.vars)
	for len(b.vars) <= k.Vars {
		if (len(b.vars)-firstNew)%256 == 0 && !limit.IncOffset(uint32(min(256, k.Vars+1-len(b.vars)))) {
			return false, nil
		}
		v, err := b.solver.MkVar(true, true)
		if err != nil {
			return false, err
		}
		b.vars = append(b.vars, v)
	}
	if !reuse {
		if ok, err := b.add(b.prefix, sat.DefaultLiteral()); err != nil || !ok {
			return false, err
		}
	}
	v, err := b.solver.MkVar(true, false)
	if err != nil {
		return false, err
	}
	disabled := sat.MakeLiteral(v, true)
	if ok, err := b.add(k.Clauses[len(b.prefix):], disabled); err != nil || !ok {
		return false, err
	}
	b.checks++
	answer, err := b.solver.Check([]sat.Literal{disabled.Negated()})
	if err != nil || answer != lbool.False {
		return false, err
	}
	if err := b.solver.PopToBaseLevel(); err != nil {
		return false, err
	}
	if err := b.solver.MkClause([]sat.Literal{disabled}); err != nil {
		return false, err
	}
	return true, nil
}
