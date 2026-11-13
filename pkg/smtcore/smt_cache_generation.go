package smtcore

// Source: src/smt/smt_context.cpp cache_generation and
// reset_cache_generation. Enodes deleted by backtracking and recreated by
// reinit_clauses, reassert_units or the conflict lemma keep the generation
// they had, through m_cached_generation (read by mk_enode and the
// quantifier internalizer); m_cache_generation_visited bounds the walk.

import "github.com/glycerine/g3/pkg/ast"

// cacheGenerationLiterals is cache_generation(num_lits, lits, new_scope_lvl):
// each literal whose variable was internalized above newLevel.
func (s *SMTInternalizerState) cacheGenerationLiterals(literals []SMTLiteral, newLevel uint32) error {
	for _, literal := range literals {
		variable := literal.Var()
		if variable < SMTFirstBoolVar || int(variable) >= len(s.booleanData) || int(variable) >= len(s.boolExpr) {
			return ast.ErrASTDomain
		}
		if s.booleanData[variable].InternalLevel() > newLevel {
			if err := s.cacheGenerationExpr(s.boolExpr[variable], newLevel); err != nil {
				return err
			}
		}
	}
	return nil
}

// cacheGenerationExpr is cache_generation(expr * n, new_scope_lvl).
func (s *SMTInternalizerState) cacheGenerationExpr(root ast.ASTHandle, newLevel uint32) error {
	if s.cacheGenerationVisited == nil {
		s.cacheGenerationVisited = make(map[ast.ASTHandle]struct{})
	}
	todo := []ast.ASTHandle{root}
	for len(todo) != 0 {
		n := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if _, seen := s.cacheGenerationVisited[n]; seen {
			continue
		}
		s.cacheGenerationVisited[n] = struct{}{}
		kind, err := s.manager.GetKind(n)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			if node := s.context.enodes.get(n); node != nil {
				if node.InternalScopeLevel() <= newLevel {
					// node and its children will not be recreated
					continue
				}
				s.cachedGeneration[n] = node.Generation()
			}
			count, err := s.manager.GetNumArgs(n)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				arg, err := s.manager.GetArg(n, i)
				if err != nil {
					return err
				}
				argKind, err := s.manager.GetKind(arg)
				if err != nil {
					return err
				}
				if argKind == ast.ASTApp || argKind == ast.ASTQuantifier {
					todo = append(todo, arg)
				}
			}
		case ast.ASTQuantifier:
			if _, internalized := s.BoolVar(n); !internalized || s.quantifiers == nil {
				continue
			}
			generation, err := s.quantifiers.Generation(n)
			if err != nil {
				return err
			}
			s.cachedGeneration[n] = generation
			body, err := s.manager.GetQuantifierBody(n)
			if err != nil {
				return err
			}
			todo = append(todo, body)
		}
	}
	return nil
}

// cacheGenerationForPop is pop_scope_core's cache_generation(new_lvl): the
// literals of the clauses reinit_clauses will rebuild and the units
// reassert_units will reassert.
func (s *SMTInternalizerState) cacheGenerationForPop(learned []smtPreservedLearnedClause, newLevel uint32) error {
	for i := range learned {
		if clause := learned[i].clause; clause != nil {
			if err := s.cacheGenerationLiterals(clause.literals, newLevel); err != nil {
				return err
			}
		}
	}
	for i := range s.learnedUnits {
		unit := &s.learnedUnits[i]
		if unit.discard || unit.ownerLevel > newLevel || unit.installedLevel <= newLevel {
			continue
		}
		if err := s.cacheGenerationExpr(unit.expression, newLevel); err != nil {
			return err
		}
	}
	return nil
}

// resetCacheGeneration is reset_cache_generation.
func (s *SMTInternalizerState) resetCacheGeneration() {
	if len(s.cacheGenerationVisited) != 0 {
		s.cacheGenerationVisited = nil
	}
	if len(s.cachedGeneration) != 0 {
		s.cachedGeneration = make(map[ast.ASTHandle]uint32)
	}
}
