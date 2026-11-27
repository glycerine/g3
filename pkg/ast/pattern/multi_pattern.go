package pattern

// Source: src/ast/pattern/pattern_inference.cpp:435-482.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

const maxPatternSplits = 32

type prePattern struct {
	exprs    []ast.ASTHandle
	freeVars patternVarSet
	index    uint32
}

// selectMulti follows candidates2multi_patterns' growing pre-pattern queue.
// Candidate handles are borrowed; each appended ASTRef independently owns the
// resulting pattern. The result slice may already contain unary patterns and
// its completed prefix remains owned by the caller if a later step stops.
func (c *patternCandidates) selectMulti(candidates []ast.ASTHandle, maxPatterns uint32, result []ast.ASTRef) ([]ast.ASTRef, error) {
	if c == nil || c.closed {
		return result, ast.ErrASTHandle
	}
	if len(candidates) == 0 {
		return result, ast.ErrASTDomain
	}
	states := []*prePattern{{}}
	c.tracePrePatternAllocated(states[0], 439)
	defer func() {
		for _, state := range states {
			if state != nil {
				c.tracePrePatternDeallocated(state)
			}
		}
	}()
	var splits uint32
	trace := c.traceTags
	if trace == nil {
		trace = diagnostic.BorrowProcessTraceTags()
	}
	for j := 0; j < len(states); j++ {
		current := states[j]
		if uint32(current.freeVars.count()) == c.numBindings {
			var err error
			if c.preferConnected && len(result) == 0 && maxPatterns == 1 {
				term, e := c.connectingTerm(current.exprs, candidates)
				if e != nil {
					return result, e
				}
				if !term.IsNull() {
					joined := make([]ast.ASTHandle, len(current.exprs)+1)
					copy(joined, current.exprs)
					joined[len(current.exprs)] = term
					result, err = c.appendMultiPattern(result, joined)
					if err != nil {
						return result, err
					}
				}
			}
			result, err = c.appendMultiPattern(result, current.exprs)
			if err != nil {
				return result, err
			}
			if uint64(len(result)) >= uint64(maxPatterns) {
				return result, nil
			}
		} else if uint64(current.index) < uint64(len(candidates)) {
			n := candidates[current.index]
			info, found := c.info[n]
			if !found {
				return result, ast.ErrASTDomain
			}
			if !info.freeVars.subset(current.freeVars) {
				next := &prePattern{
					exprs: append(append([]ast.ASTHandle(nil), current.exprs...), n),
					index: current.index + 1,
				}
				c.tracePrePatternAllocated(next, 455)
				next.freeVars.include(current.freeVars)
				next.freeVars.include(info.freeVars)
				states = append(states, next)
				if splits < maxPatternSplits {
					states[j] = nil
					current.index++
					states = append(states, current)
					splits++
				}
			} else {
				states[j] = nil
				current.index++
				states = append(states, current)
			}
		}
		if trace.ShouldTrace("pattern_inference") {
			if err := trace.Emit("pattern_inference", "candidates2multi_patterns", "src/ast/pattern/pattern_inference.cpp", 474,
				fmt.Sprintf("m_pre_patterns.size(): %d\nnum_splits: %d\n", len(states), splits)); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func (c *patternCandidates) appendMultiPattern(patterns []ast.ASTRef, terms []ast.ASTHandle) ([]ast.ASTRef, error) {
	pattern, err := c.manager.MkFamilyApp(2, 0, nil, terms)
	if err != nil {
		return patterns, err
	}
	ref, err := c.manager.NewRef(pattern)
	if err != nil {
		return patterns, err
	}
	return append(patterns, ref), nil
}
