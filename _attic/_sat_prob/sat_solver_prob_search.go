package sat

// Source: src/sat/sat_solver.cpp:1237-1240,1415-1455;
// src/sat/sat_prob.cpp:94-129.

import "github.com/glycerine/g3/pkg/util/lbool"

func (s *Solver) doProbSearch(count uint32) (lbool.LBool, error) {
	if s.propagation.extensionPresent || count != 0 {
		return lbool.Undef, nil
	}
	nScopes, err := s.conflict.userScopeLiterals.Size()
	if err != nil || nScopes != 0 {
		return lbool.Undef, err
	}
	// The source prob object owns a default-seeded random generator. Unlike the
	// periodic local-search path, do_prob_search does not reseed it from the
	// solver configuration.
	p := newSATProb(s.Limit, 0)
	numVars, err := s.Variables.numVarsChecked()
	if err != nil {
		return lbool.Undef, err
	}
	p.reserveVariables(numVars)
	n, err := s.sourceInitTrailSize()
	if err != nil {
		return lbool.Undef, err
	}
	for i := uint32(0); i < n; i++ {
		l, e := s.Variables.trail.Read(s.Variables.trailRef, i)
		if e != nil {
			return lbool.Undef, e
		}
		p.addClause([]Literal{Literal(l)})
	}
	pairs, err := s.collectBinClauses(nil, false)
	if err != nil {
		return lbool.Undef, err
	}
	for _, q := range pairs {
		p.addClause([]Literal{q.Literal1(), q.Literal2()})
	}
	it, err := s.beginOriginalClauses()
	if err != nil {
		return lbool.Undef, err
	}
	for {
		r, yes, e := it.Next()
		if e != nil {
			return lbool.Undef, e
		}
		if !yes {
			break
		}
		lits, e := s.Clauses.AppendLiterals(r, nil)
		if e != nil {
			return lbool.Undef, e
		}
		p.addClause(lits)
	}
	result := p.check()
	if result != lbool.True {
		return result, nil
	}
	if err = s.model.Shrink(0); err != nil {
		return lbool.Undef, err
	}
	if err = satSolverResizeModel(&s.model, uint32(len(p.model))); err != nil {
		return lbool.Undef, err
	}
	span := satSolverModelSpan{column: &s.model}
	for i, v := range p.model {
		if err = span.write(BoolVar(i), v); err != nil {
			return lbool.Undef, err
		}
	}
	s.Variables.modelIsCurrent = true
	s.Variables.modelKnown = true
	return lbool.True, nil
}
