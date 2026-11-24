package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

// A batch remembers a short sequence of UNSAT queries with the same
// ordered premises. These handles are identity tokens only. They include the
// arena owner and generation and do not hold any source AST alive.
const maxRecentPremises = 128

type recentRefutations struct {
	premises  []ast.ASTHandle
	goals     [8]ast.ASTHandle
	candidate ast.ASTHandle
	next      int

	units         [8]signedRefutationGoal
	candidateUnit signedRefutationGoal
	nextUnit      int
}

type signedRefutationGoal struct {
	term    ast.ASTHandle
	negated bool
}

// lookup prepares a query for observation if it misses. Called with at least
// one assertion and at most maxRecentPremises preceding the final assertion.
func (r *recentRefutations) lookup(m *ast.ASTManager, assertions []ast.ASTHandle) bool {
	base := assertions[:len(assertions)-1]
	r.candidate = assertions[len(base)]
	r.candidateUnit = signedRefutationGoal{}
	same := len(r.premises) == len(base)
	if same {
		for i, h := range base {
			if h != r.premises[i] {
				same = false
				break
			}
		}
	}
	if !same {
		if cap(r.premises) < len(base) {
			r.premises = make([]ast.ASTHandle, len(base))
		}
		r.premises = r.premises[:len(base)]
		copy(r.premises, base)
		clear(r.goals[:])
		clear(r.units[:])
		r.next, r.nextUnit = 0, 0
	} else {
		for _, h := range r.goals {
			if r.candidate == h && !h.IsNull() {
				return true
			}
		}
	}
	return m.TraceStream == nil && r.refutedConjunct(m)
}

func hasOriginalAssertions(source solver.Solver) bool {
	combined, ok := source.(*solver.CombinedSolver)
	return ok && combined.HasOriginalAssertions()
}

// The caller can change scopes, assertions, or tracked assumptions between
// Prove and Check. Compare the concrete solver's actual input before the check.
func (r *recentRefutations) matchesCurrent(source solver.Solver) bool {
	if !hasOriginalAssertions(source) {
		return false
	}
	n, err := source.NumAssumptions()
	if err != nil || n != 0 {
		return false
	}
	n, err = source.NumAssertions()
	if err != nil || int(n) != len(r.premises)+1 {
		return false
	}
	goal, err := source.Assertion(n - 1)
	if err != nil || goal != r.candidate {
		return false
	}
	for i, h := range r.premises {
		current, err := source.Assertion(uint32(i))
		if err != nil || current != h {
			return false
		}
	}
	return true
}

func (r *recentRefutations) remember() {
	r.goals[r.next] = r.candidate
	r.next = (r.next + 1) % len(r.goals)
	if !r.candidateUnit.term.IsNull() {
		r.units[r.nextUnit] = r.candidateUnit
		r.nextUnit = (r.nextUnit + 1) % len(r.units)
	}
}

// Unwrap Boolean negation and unary conjunction or disjunction. Keep the
// sign separate so that looking up a complement never constructs a source AST.
func (g signedRefutationGoal) unwrap(m *ast.ASTManager, remaining *int) (signedRefutationGoal, ast.ASTNodeHeader, bool) {
	for *remaining > 0 {
		(*remaining)--
		h, err := m.NodeHeader(g.term)
		if err != nil {
			break
		}
		if h.Kind != ast.ASTApp || h.Family != m.BasicFamilyID() || h.NumArgs != 1 {
			return g, h, true
		}
		switch h.DeclKind {
		case ast.ASTOpNot:
			g.negated = !g.negated
		case ast.ASTOpAnd, ast.ASTOpOr:
		default:
			return g, h, true
		}
		g.term, err = m.GetArg(g.term, 0)
		if err != nil {
			break
		}
	}
	return signedRefutationGoal{}, ast.ASTNodeHeader{}, false
}

func (r *recentRefutations) containsUnit(g signedRefutationGoal) bool {
	for _, old := range r.units {
		if old == g {
			return true
		}
	}
	return false
}

// If the same premises refuted one required conjunct, they also refute the
// entire goal. Only a completed proof of a whole goal can populate units.
func (r *recentRefutations) refutedConjunct(m *ast.ASTManager) bool {
	remaining := 32
	root, h, ok := (signedRefutationGoal{term: r.candidate}).unwrap(m, &remaining)
	if !ok {
		return false
	}
	conjunction := !root.negated && h.DeclKind == ast.ASTOpAnd
	negatedOr := root.negated && h.DeclKind == ast.ASTOpOr
	negatedImplication := root.negated && h.DeclKind == ast.ASTOpImplies && h.NumArgs == 2
	if h.Kind != ast.ASTApp || h.Family != m.BasicFamilyID() || !(conjunction || negatedOr || negatedImplication) {
		r.candidateUnit = root
		return r.containsUnit(root)
	}
	if r.units[0].term.IsNull() {
		return false
	}
	for i := uint32(0); i < h.NumArgs && remaining > 0; i++ {
		child, err := m.GetArg(root.term, i)
		if err != nil {
			return false
		}
		// A negated implication requires its antecedent and the negation of
		// its consequent. A negated disjunction requires each negated child.
		negated := root.negated && !(negatedImplication && i == 0)
		g, _, ok := (signedRefutationGoal{term: child, negated: negated}).unwrap(m, &remaining)
		if ok && r.containsUnit(g) {
			return true
		}
	}
	return false
}
