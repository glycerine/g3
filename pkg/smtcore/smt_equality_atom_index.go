package smtcore

// Source: context::is_diseq in src/smt/smt_context.cpp. The source finds the
// equality atom by a congruence-table probe. Go equality atoms need not own
// enodes, so atoms are indexed by their argument terms.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) equalityAtomTerms(expression ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	op, basic, err := s.basicOperation(expression)
	if err != nil || !basic || op != ast.ASTOpEq && op != ast.ASTOpIff {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	left, err := s.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err := s.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return left, right, true, nil
}

// indexEqualityAtoms catches up with variables created since the last query.
func (s *SMTInternalizerState) indexEqualityAtoms() error {
	for s.equalityAtomCursor < len(s.boolExpr) {
		variable := int32(s.equalityAtomCursor)
		if variable >= SMTFirstBoolVar {
			left, right, isEquality, err := s.equalityAtomTerms(s.boolExpr[variable])
			if err != nil {
				return err
			}
			if isEquality {
				s.equalityAtomsByTerm[left] = append(s.equalityAtomsByTerm[left], variable)
				if right != left {
					s.equalityAtomsByTerm[right] = append(s.equalityAtomsByTerm[right], variable)
				}
			}
		}
		s.equalityAtomCursor++
	}
	return nil
}

// unindexEqualityAtom retires a variable before its expression is released.
// Variables retire in reverse creation order, so its entries are last.
func (s *SMTInternalizerState) unindexEqualityAtom(variable int32, expression ast.ASTHandle) {
	if int(variable) >= s.equalityAtomCursor {
		return
	}
	s.equalityAtomCursor = int(variable)
	left, right, isEquality, err := s.equalityAtomTerms(expression)
	if err != nil || !isEquality {
		return
	}
	drop := func(term ast.ASTHandle) {
		atoms := s.equalityAtomsByTerm[term]
		if n := len(atoms); n != 0 && atoms[n-1] == variable {
			if n == 1 {
				delete(s.equalityAtomsByTerm, term)
			} else {
				s.equalityAtomsByTerm[term] = atoms[:n-1]
			}
		}
	}
	drop(left)
	if right != left {
		drop(right)
	}
}

// hasRelevantFalseEquality walks the smaller class, examining only equality
// atoms over its terms, rather than every Boolean variable.
func (s *SMTInternalizerState) hasRelevantFalseEquality(first, second *SMTEnode) (bool, error) {
	if err := s.indexEqualityAtoms(); err != nil {
		return false, err
	}
	near, far := first.root, second.root
	if near == far {
		return false, nil
	}
	if near.classSize > far.classSize {
		near, far = far, near
	}
	for member := near; ; member = member.next {
		for _, variable := range s.equalityAtomsByTerm[member.owner] {
			expression := s.boolExpr[variable]
			if s.relevancy != nil && !s.relevancy.IsRelevant(expression) {
				continue
			}
			if s.assignment[NewSMTLiteral(variable, false).Index()] != lbool.False {
				continue
			}
			left, right, isEquality, err := s.equalityAtomTerms(expression)
			if err != nil {
				return false, err
			}
			if !isEquality {
				continue
			}
			a, b := s.context.enodes.get(left), s.context.enodes.get(right)
			if a == nil || b == nil {
				continue
			}
			if a.root == near && b.root == far || a.root == far && b.root == near {
				return true, nil
			}
		}
		if member.next == near {
			return false, nil
		}
	}
}
