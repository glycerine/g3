package smtcore

// Source: src/smt/smt_context.cpp reinit_clauses. Clause objects and their
// justifications survive search backtracking. Only retired bindings and their
// two watches are repaired; relevance belongs to its original scope.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) keepClauseAcrossPop(record *SMTInternalizedClause) bool {
	if record.Binary && record.Lemma {
		return !s.closing
	}
	return !s.closing && s.pendingPop != nil && s.pendingPop.preserve && record.clause != nil && record.clause.IsLemma()
}

func (s *SMTInternalizerState) reinitializeLearnedClause(saved *smtPreservedLearnedClause) error {
	if !saved.installed {
		atoms := saved.clause.atoms
		if len(atoms) != 0 {
			if len(atoms) != len(saved.clause.literals) {
				return ast.ErrASTDomain
			}
			// A clause whose watched variables survived the pop is still
			// watched: reinit_clauses removes those watches first, since the
			// clause may get new watch literals.
			if record := &s.lemmas[saved.recordIndex]; saved.bindHead == 0 && record.clause == saved.clause {
				for record.closeWatches < 2 {
					literal := saved.clause.literals[record.closeWatches]
					if !s.watches[literal.Negated().Index()].RemoveClause(saved.clause) {
						return ast.ErrASTDomain
					}
					record.closeWatches++
				}
			}
			for saved.bindHead < len(atoms) {
				atom := atoms[saved.bindHead]
				variable, err := s.restoreLearnedAtom(atom.expression)
				if err != nil {
					return err
				}
				saved.clause.literals[saved.bindHead] = NewSMTLiteral(variable, atom.sign)
				saved.bindHead++
			}
			literals := saved.clause.literals
			first, err := s.selectWatchLiteral(literals, 0)
			if err != nil {
				return err
			}
			literals[0], literals[first] = literals[first], literals[0]
			second, err := s.selectWatchLiteral(literals, 1)
			if err != nil {
				return err
			}
			literals[1], literals[second] = literals[second], literals[1]
			record := &s.lemmas[saved.recordIndex]
			if record.clause != saved.clause || record.closeWatches != 2 {
				return ast.ErrASTDomain
			}
			record.Literals = saved.clause.literals
			for _, literal := range saved.clause.literals[:2] {
				if err = s.watches[literal.Negated().Index()].InsertClause(saved.clause); err != nil {
					return err
				}
			}
			record.closeWatches = 0
			saved.keepReinit = true
		}
		literals := saved.clause.literals
		saved.installed = true
		firstValue, err := s.literalAssignment(literals[0])
		if err != nil {
			return err
		}
		secondValue, err := s.literalAssignment(literals[1])
		if err != nil {
			return err
		}
		allOtherFalse := secondValue == lbool.False
		// After reselecting both watches, a false second watch means every
		// remaining literal is false. Stable watches need the full check.
		if allOtherFalse && len(atoms) == 0 {
			for _, literal := range literals[2:] {
				value, e := s.literalAssignment(literal)
				if e != nil {
					return e
				}
				if value != lbool.False {
					allOtherFalse = false
					break
				}
			}
		}
		justification := NewSMTBooleanClause(saved.clause)
		if firstValue == lbool.False && allOtherFalse {
			s.setBooleanConflict(justification, SMTNullLiteral)
			saved.keepReinit = true
		} else if allOtherFalse && firstValue == lbool.Undef {
			saved.keepReinit = true
			assigned := len(s.assignedLiterals)
			if err = s.AssignLiteral(literals[0], justification); err != nil {
				saved.pending = s.learnedAssignmentRemainder(literals[0], firstValue, assigned, justification)
				return err
			}
		}
	}
	if err := s.finishLearnedAssignment(&saved.pending); err != nil {
		return err
	}
	// reinit_clauses keeps a clause on the reinit stack only while
	// m_scope_lvl > m_base_lvl, pushing it onto m_clauses_to_reinit[m_scope_lvl].
	if s.scopeLevel > s.baseLevel && saved.keepReinit {
		s.markForReinit(saved.clause, s.scopeLevel)
	}
	if s.scopeLevel <= s.baseLevel || !saved.keepReinit {
		if err := saved.clause.ReleaseAtoms(); err != nil {
			return err
		}
		saved.clause.atoms = nil
		saved.clause.reinit, saved.clause.reinternalizeAtoms = false, false
	}
	return nil
}

// A clause holds a reference to each atom and captured it from a Boolean
// variable. Reuse a surviving binding unless NOT needs its nongate enode.
func (s *SMTInternalizerState) restoreLearnedAtom(expression ast.ASTHandle) (int32, error) {
	table := s.context.exprs
	record := table.read(expression)
	found := record != nil && record.flags&smtExprHasBoolVar != 0
	if found && s.booleanData[record.boolVar].basicOp != ast.ASTOpNot {
		return record.boolVar, nil
	}
	if !found && s.manager.TryIncRefHeldUninterpretedConstant(expression) {
		variable := int32(len(s.boolExpr))
		if record == nil {
			s.boolVar.set(expression, variable)
		} else {
			// The fast retain cannot run callbacks or grow the table.
			record.boolVar = variable
			record.flags |= smtExprHasBoolVar
			table.boolVarCount++
		}
		return s.publishBoolVar(expression, SMTBoolVarData{internalLevel: s.scopeLevel, basicOp: -1})
	}
	header, err := s.manager.NodeHeader(expression)
	if err != nil {
		return SMTNullBoolVar, err
	}
	// A nullary uninterpreted Boolean in a gate context has no theory,
	// children, gate constraints or enode to internalize. This is the
	// mk_bool_var path through the source's internalize_formula_core.
	if !found && header.Kind == ast.ASTApp && header.NumArgs == 0 && header.Family == ast.NullFamilyID {
		return s.mkBoolVar(expression, &header, true)
	}
	op, basic := s.smtHeaderBasicOperation(&header)
	if err = s.internalizeFormula(expression, !basic || op != ast.ASTOpNot, &header); err != nil {
		return SMTNullBoolVar, err
	}
	variable, found := s.boolVar.lookup(expression)
	if !found {
		return SMTNullBoolVar, ast.ErrASTDomain
	}
	return variable, nil
}
