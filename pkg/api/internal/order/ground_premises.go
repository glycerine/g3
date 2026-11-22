package order

import (
	"slices"

	"github.com/glycerine/g3/pkg/ast"
)

// GroundPremises reuses ground instances and their Boolean consequences across
// sibling goals. A refutation follows from the original premises. A miss leaves
// the caller free to use the ordinary solver. All handles are weak identities.
type GroundPremises struct {
	manager   *ast.ASTManager
	base      []ast.ASTHandle
	attempted uint32
	index     *groundTermIndex
	units     groundUnits
	conflict  bool
}

// The seed goal can supply ground terms for the quantifier substitutions. It
// contributes no asserted literal to the saved state.
func (p *GroundPremises) Refutes(m *ast.ASTManager, assertions []ast.ASTHandle, steps uint32) (bool, error) {
	if len(assertions) < 2 || len(assertions) > 8192 || steps < 150_000 || m.ResourceLimit().CancelFlag() {
		return false, nil
	}
	goal := assertions[len(assertions)-1]
	has, err := m.HasQuantifiers(goal)
	if err != nil || has {
		return false, err
	}
	base := assertions[:len(assertions)-1]
	budget := int(min(steps, 1_000_000))
	if p.manager != m || !slices.Equal(p.base, base) || p.index == nil && steps > p.attempted {
		*p = GroundPremises{manager: m, base: slices.Clone(base), attempted: steps}
		var quantified bool
		for _, premise := range base {
			quantified, err = m.HasQuantifiers(premise)
			if err != nil {
				*p = GroundPremises{}
				return false, err
			}
			if quantified {
				break
			}
		}
		if !quantified {
			return false, nil
		}
		g := newGroundProjection(m, uint32(budget))
		problem, err := g.problem(assertions, true)
		if err != nil {
			if err == errBitLimit || err == inapplicable {
				if m.ResourceLimit().CancelFlag() {
					*p = GroundPremises{}
				}
				return false, nil
			}
			*p = GroundPremises{}
			return false, err
		}
		budget -= int(problem.Work)
		units := groundUnits{clauses: problem.Clauses}
		conflict, ok := units.prepare(m, problem.Vars, &budget)
		if !ok {
			if m.ResourceLimit().CancelFlag() {
				*p = GroundPremises{}
			}
			return false, nil
		}
		p.index = &groundTermIndex{
			global: g.global, sourceIDs: g.sourceIDs, symbols: g.symbols,
			applications: g.applications, terms: g.terms,
			arguments: g.arguments, literals: g.termLiterals,
		}
		p.units = units
		p.conflict = conflict
	}
	if p.index == nil || m.ResourceLimit().CancelFlag() {
		return false, nil
	}
	if p.conflict {
		return true, nil
	}
	start := len(p.units.trail)
	defer p.units.undo(start)
	goalAllowance := min(budget, 2048)
	goalBudget := goalAllowance
	conflict, err := p.assume(m, goal, true, 0, &goalBudget)
	if err != nil {
		return false, err
	}
	budget = min(budget-(goalAllowance-goalBudget), 150_000)
	if !conflict {
		conflict, _ = p.units.propagate(m, start, &budget)
	}
	return conflict && !m.ResourceLimit().CancelFlag(), nil
}

// assume only descends when the asserted polarity fixes the sign of the child.
// Ignoring unsupported facts weakens the assumptions and preserves refutations.
func (p *GroundPremises) assume(m *ast.ASTManager, h ast.ASTHandle, positive bool, depth int, work *int) (bool, error) {
	if *work <= 0 || depth > 64 {
		return false, nil
	}
	*work--
	head, err := m.NodeHeader(h)
	if err != nil || head.Kind != ast.ASTApp || head.NumArgs > 64 {
		return false, err
	}
	if head.Family == m.BasicFamilyID() {
		if head.DeclKind == ast.ASTOpNot && head.NumArgs == 1 {
			child, err := m.GetArg(h, 0)
			if err != nil {
				return false, err
			}
			return p.assume(m, child, !positive, depth+1, work)
		}
		implies := head.DeclKind == ast.ASTOpImplies && head.NumArgs == 2
		if positive && head.DeclKind == ast.ASTOpAnd || !positive && (head.DeclKind == ast.ASTOpOr || implies) {
			for j := uint32(0); j < head.NumArgs && *work > 0; j++ {
				child, err := m.GetArg(h, j)
				if err != nil {
					return false, err
				}
				conflict, err := p.assume(m, child, positive || implies && j == 0, depth+1, work)
				if conflict || err != nil {
					return conflict, err
				}
			}
			return false, nil
		}
	}
	id, err := p.index.lookup(m, h, 0, work)
	if err != nil || id == 0 {
		return false, err
	}
	lit := p.index.literals[id]
	if lit == 0 {
		return false, nil
	}
	if !positive {
		lit = -lit
	}
	return !p.units.assign(lit), nil
}

type groundTermIndex struct {
	global, arguments  []uint32
	sourceIDs, symbols map[ast.ASTHandle]uint32
	applications       map[uint64]uint32
	terms              []groundApplication
	literals           []int32
}

// A sibling may introduce an AST that already occurs as an instantiated term.
// Look up its full declaration and argument vector in the saved term table.
func (t *groundTermIndex) lookup(m *ast.ASTManager, h ast.ASTHandle, depth int, work *int) (uint32, error) {
	if *work <= 0 {
		return 0, nil
	}
	*work--
	if i := t.sourceIDs[h]; i != 0 && t.global[i] != 0 {
		return t.global[i], nil
	}
	if depth > 5 {
		return 0, nil
	}
	head, err := m.NodeHeader(h)
	if err != nil || head.Kind != ast.ASTApp || head.NumArgs > 5 {
		return 0, err
	}
	symbol := t.symbols[head.Decl]
	if symbol == 0 {
		return 0, nil
	}
	var small [5]uint32
	args := small[:head.NumArgs]
	for j := range args {
		a, err := m.GetArg(h, uint32(j))
		if err != nil {
			return 0, err
		}
		id, err := t.lookup(m, a, depth+1, work)
		if err != nil || id == 0 {
			return 0, err
		}
		args[j] = id
	}
	if head.Family == m.BasicFamilyID() && head.DeclKind == ast.ASTOpEq && len(args) == 2 && args[1] < args[0] {
		args[0], args[1] = args[1], args[0]
	}
	for id := t.applications[groundTermHash(symbol, args)]; id != 0 && *work > 0; id = t.terms[id].next {
		*work--
		term := &t.terms[id]
		if term.symbol == symbol && term.count == head.NumArgs && slices.Equal(t.arguments[term.start:term.start+term.count], args) {
			return id, nil
		}
	}
	return 0, nil
}
