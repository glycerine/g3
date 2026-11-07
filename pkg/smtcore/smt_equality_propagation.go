package smtcore

// Source: src/smt/smt_context.cpp add_eq, add_diseq, propagate_eqs, and
// congruence-table reinsertion. This is the retained single-goroutine core.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func cloneSMTTheoryVariables(source SMTTheoryVarList) SMTTheoryVarList {
	result := SMTTheoryVarList{theory: source.theory, variable: source.variable}
	to := &result
	for from := source.next; from != nil; from = from.next {
		to.next = &SMTTheoryVarList{theory: from.theory, variable: from.variable}
		to = to.next
	}
	return result
}

// recordEqualityUndo follows source add_eq_trail. The class links, roots, and
// proof tree are derived again on undo, so one merge costs one fixed trail entry.
func (s *SMTInternalizerState) recordEqualityUndo(r1, left, r2 *SMTEnode) {
	s.equalityTrail = append(s.equalityTrail, smtEqualityUndo{
		r1: r1, left: left, r2: r2, r2NumParents: len(r2.parents), r2NumDisequalities: len(r2.disequalities), theoryLimit: len(s.theoryTrail),
		r2TheoryVariables: cloneSMTTheoryVariables(r2.theoryVariables),
	})
}

// undoEquality follows source context::undo_add_eq. Its stage keeps a retried
// pop from repeating the class unmerge after a later owner refuses.
func (s *SMTInternalizerState) undoEquality(entry *smtEqualityUndo) error {
	r1, r2 := entry.r1, entry.r2
	if entry.stage == 0 {
		// Parents added since this merge are retiring nodes or r1's parents;
		// r1's are reinserted below. Retired ones just leave the table.
		if entry.merged {
			for _, parent := range r2.parents[entry.r2NumParents:] {
				if err := s.eraseMergedCongruenceRoot(parent); err != nil {
					return err
				}
				if parent.congruence == parent {
					parent.congruence = nil
				}
			}
			for _, parent := range r1.parents {
				if err := s.eraseMergedCongruenceRoot(parent); err != nil {
					return err
				}
				if parent.congruence == parent {
					parent.congruence = nil
				}
			}
		}
		entry.stage = 1
	}
	if entry.stage == 1 {
		if entry.merged {
			r2.classSize -= r1.classSize
			r1.next, r2.next = r2.next, r1.next
			for current := r1; ; current = current.next {
				current.root = r1
				if current.next == r1 {
					break
				}
			}
			clear(r2.parents[entry.r2NumParents:])
			r2.parents = r2.parents[:entry.r2NumParents]
			r2.truncateDisequalities(entry.r2NumDisequalities)
			entry.left.trans = SMTTransJustification{Justification: NullSMTEqualityJustification()}
			smtInvertTrans(r1)
		}
		r2.theoryVariables = entry.r2TheoryVariables
		entry.stage = 2
	}
	if !entry.merged {
		return nil
	}
	for _, parent := range r1.parents {
		if !parent.congruenceEnabled {
			continue
		}
		if parent.IsTrueEquality() {
			parent.congruence = parent
			continue
		}
		if keep := parent.congruence; keep != nil && keep != parent {
			congruent, _, err := SMTEnodesCongruent(parent, keep)
			if err != nil {
				return err
			}
			if congruent {
				continue
			}
		}
		inserted, err := s.congruence.Insert(parent)
		if err != nil {
			return err
		}
		parent.congruence = inserted.Node
	}
	return nil
}

func (s *SMTInternalizerState) eraseMergedCongruenceRoot(parent *SMTEnode) error {
	if parent.congruence != parent || !parent.congruenceEnabled || parent.IsTrueEquality() {
		return nil
	}
	_, err := s.congruence.Erase(parent)
	return err
}

func smtInvertTrans(node *SMTEnode) {
	current := node.trans.Target
	previous := node
	justification := node.trans.Justification
	previous.trans = SMTTransJustification{Justification: NullSMTEqualityJustification()}
	for current != nil {
		next := current.trans.Target
		nextJustification := current.trans.Justification
		current.trans = SMTTransJustification{Target: previous, Justification: justification}
		previous, justification, current = current, nextJustification, next
	}
}

func (s *SMTInternalizerState) setEqualityConflict(justification SMTJustification, literal SMTLiteral) error {
	wrapper, err := NewSMTBooleanJustification(justification)
	if err != nil {
		return err
	}
	s.setBooleanConflict(wrapper, literal)
	return nil
}

func (s *SMTInternalizerState) addDisequality(left, right *SMTEnode, literal SMTLiteral) error {
	if left == nil || right == nil || literal == SMTNullLiteral {
		return ast.ErrASTDomain
	}
	if left.root == right.root {
		justification, err := NewSMTEqualityPropagationJustification(left, right)
		if err != nil {
			return err
		}
		return s.setEqualityConflict(justification, literal)
	}
	s.context.recordDisequality(smtDisequalityEntry{left: left, right: right, reason: literal})
	var err error
	for item := &left.root.theoryVariables; item != nil && item.theory != SMTNullTheoryID; item = item.next {
		variable := s.theoryVariableNear(left, item.theory)
		other := s.theoryVariableNear(right, item.theory)
		if other != SMTNullTheoryVar && other != variable && s.theories[item.theory] != nil && s.theories[item.theory].UseDisequalities() {
			if err = s.enqueueTheoryDisequality(item.theory, variable, other); err != nil {
				return err
			}
		}
	}
	return nil
}

// Source get_closest_var walks the proof branch, before it is inverted by
// add_eq, so a theory receives the actual nearby terms rather than root IDs.
func (s *SMTInternalizerState) theoryVariableNear(node *SMTEnode, theory SMTTheoryID) SMTTheoryVar {
	if theory == SMTNullTheoryID {
		return SMTNullTheoryVar
	}
	if !s.newCore2thEq {
		return node.root.TheoryVar(theory)
	}
	for node != nil {
		if variable := node.TheoryVar(theory); variable != SMTNullTheoryVar {
			return variable
		}
		node = node.trans.Target
	}
	return SMTNullTheoryVar
}

func (s *SMTInternalizerState) mergeTheoryVariables(from, to *SMTEnode, justification SMTEqualityJustification) error {
	r1, r2 := from.root, to.root
	fromTheory := SMTNullTheoryID
	if owner, object := justification.Justification(); object && owner != nil {
		fromTheory = owner.FromTheory()
	}
	pushEquality := func(theory SMTTheoryID, first, second SMTTheoryVar) error {
		if theory == fromTheory {
			return nil
		}
		return s.enqueueTheoryEquality(theory, first, second)
	}
	pushDisequalities := func(root *SMTEnode, theory SMTTheoryID, variable SMTTheoryVar) error {
		plugin := s.theories[theory]
		if plugin == nil {
			return ast.ErrASTDomain
		}
		return s.enqueueExistingDisequalities(root, variable, plugin)
	}
	if r1.theoryVariables.next == nil && r2.theoryVariables.next == nil {
		t1, t2 := r1.theoryVariables.theory, r2.theoryVariables.theory
		v1, v2 := s.theoryVariableNear(from, t1), s.theoryVariableNear(to, t2)
		if v1 != SMTNullTheoryVar && v2 != SMTNullTheoryVar {
			if t1 == t2 {
				return pushEquality(t1, v2, v1)
			}
			if err := r2.AddTheoryVar(t1, v1); err != nil {
				return err
			}
			if err := pushDisequalities(r2, t1, v1); err != nil {
				return err
			}
			return pushDisequalities(r1, t2, v2)
		}
		if v1 == SMTNullTheoryVar && v2 != SMTNullTheoryVar {
			return pushDisequalities(r1, t2, v2)
		}
		if v1 != SMTNullTheoryVar && v2 == SMTNullTheoryVar {
			if err := r2.AddTheoryVar(t1, v1); err != nil {
				return err
			}
			return pushDisequalities(r2, t1, v1)
		}
		return nil
	}
	for item := &r2.theoryVariables; item != nil && item.theory != SMTNullTheoryID; item = item.next {
		v2, v1 := s.theoryVariableNear(to, item.theory), s.theoryVariableNear(from, item.theory)
		if v1 != SMTNullTheoryVar {
			if err := pushEquality(item.theory, v2, v1); err != nil {
				return err
			}
		} else if err := pushDisequalities(r1, item.theory, v2); err != nil {
			return err
		}
	}
	for item := &r1.theoryVariables; item != nil && item.theory != SMTNullTheoryID; item = item.next {
		if r2.TheoryVar(item.theory) == SMTNullTheoryVar {
			variable := s.theoryVariableNear(from, item.theory)
			if err := r2.AddTheoryVar(item.theory, variable); err != nil {
				return err
			}
			if err := pushDisequalities(r2, item.theory, variable); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SMTInternalizerState) propagateTrueEquality(node *SMTEnode) error {
	if !node.IsTrueEquality() {
		return nil
	}
	variable, found := s.boolVar.lookup(node.owner)
	if !found {
		return nil
	}
	assigned, err := s.Assignment(NewSMTLiteral(variable, false))
	if err != nil {
		return err
	}
	if assigned == lbool.True {
		return nil
	}
	justification, err := NewSMTEqualityPropagationJustification(node.arguments[0], node.arguments[1])
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(justification)
	if err != nil {
		return err
	}
	if err = s.AssignLiteral(NewSMTLiteral(variable, false), wrapped); err != nil {
		return err
	}
	return nil
}

// Source context::remove_parents_from_cg_table / reinsert_parents_into_cg_table.
// Only parents of the class whose roots change need a new signature. The Go
// parent vectors also retain nonrepresentatives for concrete enode deletion.
func (s *SMTInternalizerState) detachCongruenceParents(from, to *SMTEnode) error {
	for _, parent := range from.parents {
		parent.parentOfMergeTarget = false
		for _, arg := range parent.arguments {
			if arg.root == to {
				parent.parentOfMergeTarget = true
				break
			}
		}
		if parent.congruenceEnabled && parent.congruence == parent && !parent.IsTrueEquality() {
			if _, err := s.congruence.Erase(parent); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *SMTInternalizerState) reinsertCongruenceParents(from, to *SMTEnode) error {
	for _, parent := range from.parents {
		if !parent.parentOfMergeTarget {
			to.parents = append(to.parents, parent)
		}
		parent.parentOfMergeTarget = false
		// A nonrepresentative stays attached to its representative, whose
		// arguments have the same roots, as in the source.
		if parent.congruence != parent {
			continue
		}
		if !parent.congruenceEnabled || parent.IsTrueEquality() {
			continue
		}
		inserted, err := s.congruence.Insert(parent)
		if err != nil {
			return err
		}
		parent.congruence = inserted.Node
		if inserted.Node != parent && inserted.Node.root != parent.root {
			s.pending = append(s.pending, smtPendingEquality{parent, inserted.Node, NewSMTEqualityCongruence(inserted.Commuted)})
		}
	}
	for _, parent := range from.parents {
		if err := s.propagateTrueEquality(parent); err != nil {
			return err
		}
		if s.inconsistent {
			return nil
		}
	}
	return nil
}

func (s *SMTInternalizerState) propagateBooleanClass(root *SMTEnode) error {
	if root == nil || !root.boolean {
		return nil
	}
	var source *SMTEnode
	value := lbool.Undef
	for node := root; ; node = node.next {
		if s.manager.IsTrue(node.owner) || s.manager.IsFalse(node.owner) {
			source, value = node, lbool.False
			if s.manager.IsTrue(node.owner) {
				value = lbool.True
			}
			break
		}
		if variable, found := s.boolVar.lookup(node.owner); found {
			assigned, err := s.Assignment(NewSMTLiteral(variable, false))
			if err != nil {
				return err
			}
			if assigned != lbool.Undef {
				source, value = node, assigned
				break
			}
		}
		if node.next == root {
			break
		}
	}
	if source == nil {
		return nil
	}
	for node := root; ; node = node.next {
		if variable, found := s.boolVar.lookup(node.owner); found && node != source {
			literal := NewSMTLiteral(variable, value == lbool.False)
			assigned, e := s.Assignment(literal)
			if e != nil {
				return e
			}
			if assigned == lbool.True {
				if node.next == root {
					break
				}
				continue
			}
			var justification SMTJustification
			var err error
			if s.manager.IsTrue(source.owner) || s.manager.IsFalse(source.owner) {
				justification, err = NewSMTEqualityRootJustification(node)
			} else {
				justification, err = NewSMTModusPonensIffJustification(source, node)
			}
			if err != nil {
				return err
			}
			wrapped, err := NewSMTBooleanJustification(justification)
			if err != nil {
				return err
			}
			if err = s.AssignLiteral(literal, wrapped); err != nil {
				return err
			}
			if s.inconsistent {
				return nil
			}
		}
		if node.next == root {
			break
		}
	}
	return nil
}

// The source context creates true/false enodes at initialization. The concrete
// Go context creates them when a Boolean value first participates as a term.
func (s *SMTInternalizerState) booleanValueEnode(value bool) (*SMTEnode, error) {
	var expression ast.ASTHandle
	var err error
	if value {
		expression, err = s.manager.MkTrue()
	} else {
		expression, err = s.manager.MkFalse()
	}
	if err != nil {
		return nil, err
	}
	if node := s.context.enodes.get(expression); node != nil {
		return node, nil
	}
	return s.MkEnode(expression, true, true, false)
}

// Source: context::propagate_bool_var_enode. This is also needed when a term
// enode is attached after its Boolean variable has already been assigned.
func (s *SMTInternalizerState) propagateBooleanEnode(variable int32, node *SMTEnode) error {
	literal := NewSMTLiteral(variable, false)
	value, err := s.Assignment(literal)
	if err != nil || value == lbool.Undef {
		return err
	}
	if node.mergeTruthValue {
		constant, err := s.booleanValueEnode(value == lbool.True)
		if err != nil {
			return err
		}
		if value == lbool.False {
			literal = literal.Negated()
		}
		if err = s.addEquality(node, constant, NewSMTEqualityEquation(literal)); err != nil || s.inconsistent {
			return err
		}
	}
	// Source propagate_bool_var_enode also transfers the assignment to the
	// other members of the class when merge_tf is disabled. Congruent Boolean
	// terms can have merged before any member received a truth value.
	return s.propagateBooleanClass(node.root)
}

func (s *SMTInternalizerState) propagateEqualityClassRelevancy(first, second *SMTEnode) error {
	relevant := false
	for _, root := range []*SMTEnode{first, second} {
		for node := root; ; node = node.next {
			if s.context.relevant.get(node.owner) {
				relevant = true
				break
			}
			if node.next == root {
				break
			}
		}
		if relevant {
			break
		}
	}
	if !relevant {
		return nil
	}
	for _, root := range []*SMTEnode{first, second} {
		for node := root; ; node = node.next {
			if !s.context.relevant.get(node.owner) {
				if err := s.markRelevant(node.owner); err != nil {
					return err
				}
			}
			if node.next == root {
				break
			}
		}
	}
	return nil
}

func (s *SMTInternalizerState) addEquality(left, right *SMTEnode, justification SMTEqualityJustification) error {
	if left == nil || right == nil || left.manager != s.manager || right.manager != s.manager {
		return ast.ErrASTDomain
	}
	r1, r2 := left.root, right.root
	s.statistics.NumAddEq++
	if r1 == r2 {
		return nil
	}
	if entry, found := smtFindDisequality(r1, r2); found {
		proof, err := NewSMTDisequalityConflictJustification(left, right, justification)
		if err != nil {
			return err
		}
		proof.DiseqLeft, proof.DiseqRight, proof.Reason = entry.left, entry.right, entry.reason
		return s.setEqualityConflict(proof, entry.reason)
	}
	if r1.interpreted && r2.interpreted {
		proof, err := NewSMTEqualityConflictJustification(left, right, justification)
		if err != nil {
			return err
		}
		return s.setEqualityConflict(proof, SMTNullLiteral)
	}
	if (r1.classSize > r2.classSize && !r2.interpreted) || r1.interpreted {
		left, right, r1, r2 = right, left, r2, r1
	}
	trailIndex := len(s.equalityTrail)
	s.recordEqualityUndo(r1, left, r2)
	if err := s.propagateEqualityClassRelevancy(r1, r2); err != nil {
		return fmt.Errorf("propagate equality-class relevancy: %w", err)
	}
	if err := s.mergeTheoryVariables(left, right, justification); err != nil {
		return fmt.Errorf("merge theory variables: %w", err)
	}
	smtInvertTrans(left)
	left.trans = SMTTransJustification{Target: right, Justification: justification}
	s.equalityTrail[trailIndex].merged = true
	if err := s.detachCongruenceParents(r1, r2); err != nil {
		return fmt.Errorf("detach congruence parents: %w", err)
	}
	for current := r1; ; current = current.next {
		current.root = r2
		if current.next == r1 {
			break
		}
	}
	r1.next, r2.next = r2.next, r1.next
	r2.classSize += r1.classSize
	r2.disequalities = append(r2.disequalities, r1.disequalities...)
	if err := s.reinsertCongruenceParents(r1, r2); err != nil {
		return fmt.Errorf("reinsert congruence parents: %w", err)
	}
	if s.quantifierManager != nil && s.quantifierManager.ematching && !s.quantifierManager.registry.Empty() {
		s.quantifierManager.mam.AddEquality(r1, r2)
	}
	if err := s.propagateBooleanClass(r2); err != nil {
		return fmt.Errorf("propagate Boolean equality class: %w", err)
	}
	if s.inconsistent {
		return nil
	}
	return nil
}

// Source context::assign_eq/push_eq only enqueue; add_eq runs later during
// propagate_eqs, after Boolean atom callbacks and before theory equality events.
func (s *SMTInternalizerState) assignEquality(left, right *SMTEnode, justification SMTEqualityJustification) error {
	if err := s.open(); err != nil {
		return err
	}
	if left == nil || right == nil || left.manager != s.manager || right.manager != s.manager || smtcoreSourceDebug && left == right {
		return ast.ErrASTDomain
	}
	s.pending = append(s.pending, smtPendingEquality{left, right, justification})
	return nil
}

func (s *SMTInternalizerState) propagateCoreEqualities() error {
	for s.pendingHead < len(s.pending) && !s.inconsistent {
		entry := s.pending[s.pendingHead]
		s.pendingHead++
		if err := s.addEquality(entry.left, entry.right, entry.justification); err != nil {
			return fmt.Errorf("merge pending nodes: %w", err)
		}
	}
	return nil
}
