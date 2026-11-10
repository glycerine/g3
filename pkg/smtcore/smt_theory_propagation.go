package smtcore

// Source: src/smt/smt_context.cpp theory equality and disequality queues.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) enqueueTheoryEquality(theory SMTTheoryID, left, right SMTTheoryVar) error {
	if theory == SMTNullTheoryID || left == SMTNullTheoryVar || right == SMTNullTheoryVar || left == right || s.theories[theory] == nil {
		return ast.ErrASTDomain
	}
	s.theoryEqualities = append(s.theoryEqualities, SMTTheoryPropagation{Theory: theory, Left: left, Right: right})
	return nil
}

func (s *SMTInternalizerState) enqueueTheoryDisequality(theory SMTTheoryID, left, right SMTTheoryVar) error {
	plugin := s.theories[theory]
	if plugin == nil || theory == SMTNullTheoryID || left == SMTNullTheoryVar || right == SMTNullTheoryVar || left == right {
		return ast.ErrASTDomain
	}
	s.theoryDisequalities = append(s.theoryDisequalities, SMTTheoryPropagation{Theory: theory, Left: left, Right: right})
	return nil
}

func (s *SMTInternalizerState) enqueueExistingDisequalities(root *SMTEnode, variable SMTTheoryVar, plugin SMTTheoryInternalizer) error {
	if !plugin.UseDisequalities() {
		return nil
	}
	if smtTraceEnabled("push_new_th_diseqs") {
		if id, err := root.OwnerID(); err == nil {
			emitSMTTrace("push_new_th_diseqs", "push_new_th_diseqs", "src/smt/smt_context.cpp", 1496,
				fmt.Sprintf("#%d v%d\n", id, variable))
		}
	}
	// r->get_parents() is the source parent list (sourceParents).
	for _, parent := range root.sourceParents {
		if parent == nil || !parent.IsEquality() || len(parent.arguments) != 2 {
			continue
		}
		boolVariable, found := s.boolVar.lookup(parent.owner)
		if !found {
			continue
		}
		value, err := s.Assignment(NewSMTLiteral(boolVariable, false))
		if err != nil {
			return err
		}
		if value != lbool.False {
			continue
		}
		left, right := parent.arguments[0], parent.arguments[1]
		if smtTraceEnabled("push_new_th_diseqs") {
			pid, _ := parent.OwnerID()
			lid, _ := left.OwnerID()
			rid, _ := right.OwnerID()
			lrid, _ := left.root.OwnerID()
			rrid, _ := right.root.OwnerID()
			oid, _ := root.OwnerID()
			orid, _ := root.root.OwnerID()
			emitSMTTrace("push_new_th_diseqs", "push_new_th_diseqs", "src/smt/smt_context.cpp", 1505,
				fmt.Sprintf("#%d lhs: #%d, rhs: #%d, lhs->root: #%d, rhs->root: #%d, r: #%d, r->root: #%d\n", pid, lid, rid, lrid, rrid, oid, orid))
		}
		if right.root == root {
			left, right = right, left
		}
		if left.root != root || right.root == root {
			continue
		}
		other := s.theoryVariableNear(right, plugin.FamilyID())
		if nearby := s.theoryVariableNear(left, plugin.FamilyID()); nearby != SMTNullTheoryVar {
			variable = nearby
		}
		if other != SMTNullTheoryVar && other != variable {
			if err = s.enqueueTheoryDisequality(plugin.FamilyID(), variable, other); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SMTInternalizerState) PendingTheoryEqualities() []SMTTheoryPropagation {
	if s == nil {
		return nil
	}
	return append([]SMTTheoryPropagation(nil), s.theoryEqualities[s.theoryEqualityHead:]...)
}

func (s *SMTInternalizerState) PendingTheoryDisequalities() []SMTTheoryPropagation {
	if s == nil {
		return nil
	}
	return append([]SMTTheoryPropagation(nil), s.theoryDisequalities[s.theoryDisequalityHead:]...)
}

func (s *SMTInternalizerState) PropagateTheoryEvents() error {
	if err := s.open(); err != nil {
		return err
	}
	// propagate_th_eqs/propagate_th_diseqs stop at an inconsistency and
	// then reset their queues, dropping the events not yet delivered.
	for s.theoryEqualityHead < len(s.theoryEqualities) && !s.inconsistent {
		event := s.theoryEqualities[s.theoryEqualityHead]
		plugin := s.theories[event.Theory]
		if plugin == nil {
			return ast.ErrASTDomain
		}
		if err := plugin.NewEquality(event.Left, event.Right); err != nil {
			return err
		}
		s.theoryEqualityHead++
	}
	s.theoryEqualityHead = len(s.theoryEqualities)
	for s.theoryDisequalityHead < len(s.theoryDisequalities) && !s.inconsistent {
		event := s.theoryDisequalities[s.theoryDisequalityHead]
		plugin := s.theories[event.Theory]
		if plugin == nil {
			return ast.ErrASTDomain
		}
		left, err := plugin.Enode(event.Left)
		if err != nil {
			return err
		}
		right, err := plugin.Enode(event.Right)
		if err != nil {
			return err
		}
		if left.IsInterpreted() && right.IsInterpreted() {
			s.theoryDisequalityHead++
			continue
		}
		if err := plugin.NewDisequality(event.Left, event.Right); err != nil {
			return err
		}
		s.theoryDisequalityHead++
	}
	s.theoryDisequalityHead = len(s.theoryDisequalities)
	return nil
}

func (s *SMTInternalizerState) propagateAssignedAtom(literal SMTLiteral) error {
	variable := literal.Var()
	data, err := s.BooleanData(variable)
	if err != nil {
		return fmt.Errorf("assigned literal %d Boolean metadata: %w", variable, err)
	}
	if !data.IsAtom() || s.atomNotified[variable] {
		return nil
	}
	s.tracePropagateAtom(literal, data)
	if node := s.context.enodes.get(s.boolExpr[variable]); node != nil {
		if err = s.propagateBooleanEnode(variable, node); err != nil {
			return err
		}
		if s.inconsistent {
			return nil
		}
	}
	if data.IsEquality() {
		expression := s.boolExpr[variable]
		leftExpression, leftErr := s.manager.GetArg(expression, 0)
		rightExpression, rightErr := s.manager.GetArg(expression, 1)
		if leftErr != nil || rightErr != nil {
			return fmt.Errorf("assigned equality %d arguments: %w", variable, ast.ErrASTDomain)
		}
		left, right := s.context.enodes.get(leftExpression), s.context.enodes.get(rightExpression)
		if left == nil || right == nil {
			return fmt.Errorf("assigned equality %d enodes: %w", variable, ast.ErrASTDomain)
		}
		if literal.Sign() {
			if err = s.addDisequality(left, right, literal); err != nil {
				return fmt.Errorf("propagate equality disequality: %w", err)
			}
		} else if err = s.addEquality(left, right, NewSMTEqualityEquation(literal)); err != nil {
			return fmt.Errorf("propagate equality merge: %w", err)
		}
		if s.inconsistent {
			return nil
		}
	}
	if data.IsTheoryAtom() {
		if s.relevancyLevel >= 2 && s.relevancy != nil && !s.relevancy.IsRelevant(s.boolExpr[variable]) {
			return nil
		}
		plugin := s.theories[data.Theory()]
		if plugin == nil {
			return fmt.Errorf("assigned theory atom %d missing theory %d: %w", variable, data.Theory(), ast.ErrASTDomain)
		}
		if err = plugin.Assign(variable, !literal.Sign()); err != nil {
			return fmt.Errorf("assigned theory atom %d theory %d: %w", variable, data.Theory(), err)
		}
	} else if data.IsQuantifier() {
		// All universal quantifiers have positive polarity in the input
		// formula, so quantifiers assigned to false are ignored.
		if q := s.quantifierManager; q != nil && s.assignment[NewSMTLiteral(variable, false).Index()] == lbool.True {
			if err = q.AssignQuantifier(s.boolExpr[variable]); err != nil {
				return fmt.Errorf("assign quantifier %d: %w", variable, err)
			}
		}
	}
	if s.atomNotified == nil {
		s.atomNotified = make(map[int32]bool)
	}
	s.atomNotified[variable] = true
	s.atomNotifiedTrail = append(s.atomNotifiedTrail, variable)
	return nil
}

// appendAssignedLiteral records an assignment and assign_core's queue test:
// an atom is queued if m_relevancy_lvl == 0, or m_relevancy_lvl == 1 and it
// is not a quantifier, or it is already relevant. The atom flag is read now,
// so an equality assigned while internalize_eq is still creating it (before
// set_eq_flag) is not queued, as in the source.
func (s *SMTInternalizerState) appendAssignedLiteral(literal SMTLiteral) error {
	queued, err := s.atomQueuedOnAssignment(literal.Var())
	if err != nil {
		return err
	}
	s.assignedLiterals = append(s.assignedLiterals, literal)
	if queued {
		s.atomQueue = append(s.atomQueue, literal)
	}
	return nil
}

func (s *SMTInternalizerState) atomQueuedOnAssignment(variable int32) (bool, error) {
	data, err := s.BooleanData(variable)
	if err != nil {
		return false, err
	}
	if !data.IsAtom() {
		return false, nil
	}
	if s.relevancyLevel == 0 || s.relevancy == nil || !s.relevancy.enabled {
		return true, nil
	}
	if s.relevancyLevel == 1 && !data.IsQuantifier() {
		return true, nil
	}
	return s.relevancy.IsRelevant(s.boolExpr[variable]), nil
}

func (s *SMTInternalizerState) PropagateTheoryAssignments() error {
	if err := s.open(); err != nil {
		return err
	}
	// propagate_atoms: the queue may grow while it is processed; it is reset
	// only when every entry has been processed without a conflict.
	for i := 0; i < len(s.atomQueue); i++ {
		if err := s.propagateAssignedAtom(s.atomQueue[i]); err != nil {
			return err
		}
		if s.inconsistent {
			return nil
		}
	}
	s.atomQueue = s.atomQueue[:0]
	if err := s.propagateCoreEqualities(); err != nil {
		return fmt.Errorf("propagate core equalities: %w", err)
	}
	return nil
}

// Source context::relevant_eh is called by set_relevant immediately,
// before assignment propagation and at the scope where the mark was made.
func (s *SMTInternalizerState) notifyRelevant(expression ast.ASTHandle) error {
	// Quantifiers are only asserted when marked as relevant. Other atoms are
	// only asserted when marked as relevant if m_relevancy_lvl >= 2.
	if variable, found := s.BoolVar(expression); found {
		if d := &s.booleanData[variable]; d.IsAtom() && (d.IsQuantifier() || s.relevancyLevel >= 2) {
			if value := s.assignment[NewSMTLiteral(variable, false).Index()]; value != lbool.Undef {
				s.atomQueue = append(s.atomQueue, NewSMTLiteral(variable, value == lbool.False))
			}
		}
	}
	s.traceRelevantEh(expression)
	if err := s.admitRelevanceCaseSplit(expression); err != nil {
		return err
	}
	kind, err := s.manager.GetKind(expression)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return nil
	}
	if s.quantifierManager != nil {
		s.quantifierManager.NotifyRelevant(s.context.enodes.get(expression))
	}
	notified := make(map[SMTTheoryID]struct{})
	family, err := s.manager.GetFamilyID(expression)
	if err != nil {
		return err
	}
	if family != s.manager.BasicFamilyID() {
		if plugin := s.theories[family]; plugin != nil {
			if err = plugin.Relevant(expression); err != nil {
				return fmt.Errorf("theory %d relevant expression %d: %w", family, expression.Slot(), err)
			}
			notified[family] = struct{}{}
		}
	}
	if node := s.context.enodes.get(expression); node != nil {
		for item := &node.theoryVariables; item != nil && item.theory != SMTNullTheoryID; item = item.next {
			if _, found := notified[item.theory]; found {
				continue
			}
			plugin := s.theories[item.theory]
			if plugin == nil {
				return ast.ErrASTDomain
			}
			if err = plugin.Relevant(expression); err != nil {
				return fmt.Errorf("attached theory %d relevant expression %d: %w", item.theory, expression.Slot(), err)
			}
			notified[item.theory] = struct{}{}
		}
	}
	return nil
}

func (s *SMTInternalizerState) propagateTheoryRelevancy() error {
	if s.relevancy == nil {
		return nil
	}
	if err := s.relevancy.Propagate(); err != nil {
		return err
	}
	// relevant_eh already queued assigned atoms (notifyRelevant).
	s.relevancy.DrainNewlyRelevant()
	return nil
}

// C++ context::propagate saves m_qhead before BCP and delivers assignment
// relevance only after BCP and theory case splits succeed. The end is captured
// once: assignments made by callbacks belong to a subsequent Boolean batch.
// Go also keeps the cursor so a refused callback can be retried without
// replaying completed callbacks or losing the already-consumed BCP prefix.
type smtRelevancyAssignmentBatch struct {
	head, end      int
	active         bool
	caseSplitsDone bool
}

func (s *SMTInternalizerState) propagateRelevancyAssignments() error {
	batch := &s.relevancyBatch
	if s.relevancy == nil || !s.relevancy.enabled {
		batch.head = batch.end
		return nil
	}
	for batch.head < batch.end {
		literal := s.assignedLiterals[batch.head]
		if err := s.relevancy.assignmentVariableEvent(literal.Var(), !literal.Sign()); err != nil {
			return err
		}
		batch.head++
	}
	return nil
}

// PropagateCore runs the retained logical-context queues to a fixed point.
func (s *SMTInternalizerState) PropagateCore() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	for {
		var err error
		if !s.relevancyBatch.active {
			caseSplitHead := s.propagationHead
			consistent, err := s.Propagate()
			if err != nil || !consistent {
				if err != nil {
					return consistent, fmt.Errorf("Boolean propagation: %w", err)
				}
				return consistent, nil
			}
			s.relevancyBatch = smtRelevancyAssignmentBatch{head: caseSplitHead, active: true}
		}
		if !s.relevancyBatch.caseSplitsDone {
			consistent, err := s.propagateTheoryCaseSplits(s.relevancyBatch.head)
			if err != nil || !consistent {
				if err == nil {
					s.relevancyBatch = smtRelevancyAssignmentBatch{}
				}
				return consistent, err
			}
			s.relevancyBatch.end = len(s.assignedLiterals)
			s.relevancyBatch.caseSplitsDone = true
		}
		if err = s.propagateRelevancyAssignments(); err != nil {
			return false, fmt.Errorf("assignment relevancy: %w", err)
		}
		if err = s.propagateTheoryRelevancy(); err != nil {
			return false, fmt.Errorf("theory relevancy: %w", err)
		}
		s.relevancyBatch = smtRelevancyAssignmentBatch{}
		if s.inconsistent {
			return false, nil
		}
		if err = s.PropagateTheoryAssignments(); err != nil {
			return false, fmt.Errorf("theory assignments: %w", err)
		}
		if s.inconsistent { // if (!propagate_atoms()) return false;
			return false, nil
		}
		if err = s.PropagateTheoryEvents(); err != nil {
			return false, fmt.Errorf("theory events: %w", err)
		}
		// propagate_th_eqs(); propagate_th_diseqs(); if (inconsistent()) return false;
		if s.inconsistent {
			return false, nil
		}
		dynamicAckProgress, err := s.dynamicAck.propagate()
		if err != nil {
			return false, fmt.Errorf("dynamic Ackermann propagation: %w", err)
		}
		// propagate_theories: every theory's propagate() runs each round (it
		// returns early when it has nothing to do); can_theories_propagate()
		// then decides whether another round is needed.
		for _, plugin := range s.theoryOrder {
			if err = plugin.Propagate(); err != nil {
				return false, fmt.Errorf("theory %d propagation: %w", plugin.FamilyID(), err)
			}
			if s.inconsistent {
				return false, nil
			}
		}
		theoryProgress := false
		for _, plugin := range s.theoryOrder {
			if plugin.CanPropagate() {
				theoryProgress = true
				break
			}
		}
		quantifierProgress := false
		if s.quantifierManager != nil && s.quantifierManager.CanPropagate() {
			quantifierProgress = true
			if err = s.quantifierManager.Propagate(); err != nil {
				return false, fmt.Errorf("quantifier propagation: %w", err)
			}
		}
		if s.inconsistent {
			return false, nil
		}
		relevancyDone := s.relevancy == nil || !s.relevancy.CanPropagate()
		if s.propagationHead == len(s.assignedLiterals) && len(s.atomQueue) == 0 && s.pendingHead == len(s.pending) && s.theoryEqualityHead == len(s.theoryEqualities) && s.theoryDisequalityHead == len(s.theoryDisequalities) && relevancyDone && !theoryProgress && !quantifierProgress && !dynamicAckProgress {
			return true, nil
		}
	}
}
