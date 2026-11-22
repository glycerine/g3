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
}

// lookup prepares a query for observation if it misses. Called with at least
// one assertion and at most maxRecentPremises preceding the final assertion.
func (r *recentRefutations) lookup(assertions []ast.ASTHandle) bool {
	base := assertions[:len(assertions)-1]
	r.candidate = assertions[len(base)]
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
		r.next = 0
		return false
	}
	for _, h := range r.goals {
		if r.candidate == h && !h.IsNull() {
			return true
		}
	}
	return false
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
}
