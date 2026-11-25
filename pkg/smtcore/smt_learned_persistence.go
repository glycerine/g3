package smtcore

// Source: src/smt/smt_context.cpp learned-clause reinitialization across
// search backtracking and base-scope lifetime.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtPersistentLearnedUnit struct {
	expression                 ast.ASTHandle
	proof                      ast.ASTHandle
	sign                       bool
	ownerLevel, installedLevel uint32
	discard                    bool
	pending                    smtLearnedAssignment
}

// A live search pop holds the existing clause object. Only a standalone
// snapshot owns an additional proof and atom references.
type smtLearnedClauseSnapshot struct {
	atoms    []smtRetainedLemmaAtom
	proof    ast.ASTHandle
	activity uint32
}

type smtPreservedLearnedClause struct {
	clause      *SMTClause
	recordIndex int // index of an in-place lemma in s.lemmas
	bindHead    int
	snapshot    *smtLearnedClauseSnapshot // nil when restoring the live clause in place
	pending     *smtLearnedAssignment     // set only if a published assignment needs retry
	retired     bool                      // popTo detaches the watches of a clause with a retired variable
	installed   bool
	keepReinit  bool
}

// Assignment publication precedes context/relevance notifications. Preserve
// the exact remainder when clause installation fails after that publication.
type smtLearnedAssignment struct {
	literal       SMTLiteral
	justification SMTBooleanJustification
	stage         uint8
}

const (
	smtLearnedAssignmentDone uint8 = iota
	smtLearnedAssignmentMetadata
	smtLearnedAssignmentGoal
	smtLearnedAssignmentEvent
)

func (s *SMTInternalizerState) finishLearnedAssignment(p *smtLearnedAssignment) error {
	if p == nil || p.stage == smtLearnedAssignmentDone {
		return nil
	}
	literal := p.literal
	if p.stage == smtLearnedAssignmentMetadata {
		data := &s.booleanData[literal.Var()]
		data.SetJustification(p.justification)
		data.SetScopeLevel(s.scopeLevel)
		if s.searching && s.searchRestartAdaptive && data.PhaseAvailable() {
			// float64() rounds the product, blocking FMA contraction (match z3 bit for bit).
			s.searchAgility = float64(s.searchAgility * s.searchAgilityFactor)
			if data.Phase() == literal.Sign() {
				s.searchAgility += 1 - s.searchAgilityFactor
			}
		}
		data.SetPhaseAvailable(true)
		data.SetPhase(!literal.Sign())
		if err := s.appendAssignedLiteral(literal); err != nil {
			return err
		}
		p.stage = smtLearnedAssignmentGoal
	}
	if p.stage == smtLearnedAssignmentGoal {
		if err := s.relevanceGoalAssigned(literal); err != nil {
			return err
		}
		p.stage = smtLearnedAssignmentEvent
	}
	if p.stage == smtLearnedAssignmentEvent {
		// The published literal is already in assignedLiterals. Its relevance
		// callback belongs to the later post-BCP batch, including on retry.
		p.stage = smtLearnedAssignmentDone
	}
	return nil
}

func (s *SMTInternalizerState) learnedAssignmentRemainder(literal SMTLiteral, before lbool.LBool, assigned int, justification SMTBooleanJustification) smtLearnedAssignment {
	if before != lbool.Undef || s.assignment[literal.Index()] != lbool.True {
		return smtLearnedAssignment{}
	}
	stage := smtLearnedAssignmentMetadata
	if len(s.assignedLiterals) > assigned {
		stage = smtLearnedAssignmentGoal
	}
	return smtLearnedAssignment{literal: literal, justification: justification, stage: stage}
}

func (s *SMTInternalizerState) beginSearchPersistence(base uint32, params SMTParamsState) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.searching || base != s.scopeLevel {
		return ast.ErrASTDomain
	}
	s.relevanceFIFO.baseBooleanLimit = int32(len(s.boolExpr))
	if s.relevanceGoal.heap != nil {
		s.generation = 0
	}
	s.searching = true
	s.searchBaseLevel = base
	s.searchLevel = base
	s.searchUserBaseLevel = base
	s.searchAgility = 0
	s.searchAgilityFactor = params.AgilityFactor
	s.searchRestartAdaptive = params.RestartAdaptive
	return nil
}

func (s *SMTInternalizerState) endSearchPersistence() (result error) {
	s.relevanceFIFO.baseBooleanLimit = -1
	s.searching = false
	s.searchBaseLevel = 0
	s.searchUserBaseLevel = 0
	s.searchAgility = 0
	s.searchAgilityFactor = 0
	s.searchRestartAdaptive = false
	for index := len(s.learnedUnits) - 1; index >= 0; index-- {
		unit := &s.learnedUnits[index]
		if unit.discard || unit.installedLevel == unit.ownerLevel {
			if err := s.releaseLearnedUnit(unit); err != nil {
				return err
			}
			s.removeLearnedUnit(index)
		}
	}
	return result
}

func (s *SMTInternalizerState) retainProof(proof ast.ASTHandle) (ast.ASTHandle, error) {
	if proof.IsNull() {
		return proof, nil
	}
	if err := s.manager.IncRef(proof); err != nil {
		return ast.ASTHandle{}, err
	}
	return proof, nil
}

func (s *SMTInternalizerState) releaseLearnedRoot(root *ast.ASTHandle) error {
	if root.IsNull() {
		return nil
	}
	consumed, err := s.manager.DecRefConsumed(*root)
	if consumed {
		*root = ast.ASTHandle{}
	}
	if err != nil {
		return err
	}
	if !consumed {
		return ast.ErrASTPending
	}
	return nil
}

func (s *SMTInternalizerState) learnedClauseProof(clause *SMTClause) (ast.ASTHandle, error) {
	proofs, err := s.manager.ProofsEnabled()
	if err != nil || !proofs {
		return ast.ASTHandle{}, err
	}
	wrapper, ok := clause.Justification().(*SMTJustificationProofWrapper)
	if !ok {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return wrapper.Proof()
}

// A failed acquisition returns its owned prefix to the pending Pop; it must
// release that prefix before attempting another capture. During a live
// search pop it walks m_clauses_to_reinit as reinit_clauses does, and limit
// is unused; otherwise it snapshots m_lemmas[limit..].
func (s *SMTInternalizerState) preserveLearnedClauses(limit, firstScoped int) ([]smtPreservedLearnedClause, error) {
	if limit < 0 || limit > len(s.lemmas) {
		return nil, ast.ErrASTDomain
	}
	var result []smtPreservedLearnedClause
	livePop := s.pendingPop != nil && s.pendingPop.preserve && s.pendingPop.stage == 0
	if livePop {
		// One pop at a time owns the scratch; PopScopes clears it after
		// reinit_clauses.
		result = s.preservedScratch[:0]
	}
	start := limit
	if livePop {
		// reinit_clauses walks m_clauses_to_reinit[m_scope_lvl+1..lim], which
		// also holds older lemmas re-pushed by an earlier reinit. A search
		// pop deletes no lemma, so every index stays valid.
		start = 0
	}
	if livePop {
		// Only the lemmas on m_clauses_to_reinit[level+1..m_scope_lvl] are
		// revisited; every lemma with an atom this pop retires is among them
		// (mk_clause marks a lemma at its atoms' maximum scope, and reinit
		// re-pushes it at the scope that re-internalizes them).
		// reinitCandidates appends them in reinit_clauses order: level by
		// level, each level in push order.
		var err error
		result, err = s.reinitCandidates(s.pendingPop.level, result, firstScoped)
		if err != nil {
			return nil, err
		}
		if smtcoreSourceDebug {
			if err = s.checkReinitCandidates(result, firstScoped); err != nil {
				return nil, err
			}
		}
		start = len(s.lemmas)
	}
	for index := start; index < len(s.lemmas); index++ {
		clause := s.lemmas[index].activeClause()
		if clause == nil || !clause.IsLemma() {
			continue
		}
		result = append(result, smtPreservedLearnedClause{snapshot: new(smtLearnedClauseSnapshot)})
		saved := &result[len(result)-1]
		var err error
		saved.snapshot.atoms, err = s.captureLemmaAtoms(clause.literals, firstScoped)
		if err != nil {
			return result, err
		}
		proof, err := s.learnedClauseProof(clause)
		if err == nil {
			saved.snapshot.proof, err = s.retainProof(proof)
		}
		if err != nil {
			return result, err
		}
		saved.snapshot.activity, err = clause.Activity()
		if err != nil {
			return result, err
		}
	}
	if livePop {
		s.preservedScratch = result[:0]
	}
	return result, nil
}

// smtReinitEntry is one push onto m_clauses_to_reinit.
type smtReinitEntry struct {
	clause *SMTClause
	seq    uint64
}

// markForReinit records the clause's place in m_clauses_to_reinit.
func (s *SMTInternalizerState) markForReinit(clause *SMTClause, level uint32) {
	s.reinitSeq++
	clause.reinitLevel, clause.reinitSeq = level, s.reinitSeq
	for uint32(len(s.reinitBuckets)) <= level {
		s.reinitBuckets = append(s.reinitBuckets, nil)
	}
	s.reinitBuckets[level] = append(s.reinitBuckets[level], smtReinitEntry{clause, s.reinitSeq})
}

// dropReinitBucketsAbove empties m_clauses_to_reinit[level+1..]: a pop to
// level has walked (or deleted) them.
func (s *SMTInternalizerState) dropReinitBucketsAbove(level int) {
	for l := level + 1; l < len(s.reinitBuckets); l++ {
		clear(s.reinitBuckets[l])
		s.reinitBuckets[l] = s.reinitBuckets[l][:0]
	}
}

// reinitCandidates appends to result, as in-place entries, the lemmas on
// m_clauses_to_reinit[level+1..m_scope_lvl], each once, with their index in
// m_lemmas. A clause sits validly in exactly one bucket (its reinitLevel,
// under its unique reinitSeq), so the bucket walk yields reinit_clauses
// order without sorting. The stored lemma index identifies each live record
// even when only a small fraction of the lemma vector needs restoration.
func (s *SMTInternalizerState) reinitCandidates(level int, result []smtPreservedLearnedClause, booleanLimit int) ([]smtPreservedLearnedClause, error) {
	first := len(result)
	for l := level + 1; l <= int(s.scopeLevel) && l < len(s.reinitBuckets); l++ {
		for _, entry := range s.reinitBuckets[l] {
			clause := entry.clause
			if clause.reinitSeq != entry.seq || !clause.InReinitStack() || int(clause.reinitLevel) != l || !clause.IsLemma() {
				continue
			}
			index := clause.lemmaIndex
			if index < 0 || index >= len(s.lemmas) || s.lemmas[index].activeClause() != clause {
				clear(result[first:])
				return result[:first], ast.ErrASTDomain
			}
			retired := false
			for _, literal := range clause.literals {
				retired = retired || int(literal.Var()) >= booleanLimit
			}
			// C++ keeps the object, proof owner, literal storage and watches.
			// There is no snapshot or reconstruction for surviving bindings.
			// Source clause objects already own their atoms and proof. Do not
			// acquire a second snapshot merely to keep the same object alive.
			result = append(result, smtPreservedLearnedClause{clause: clause, recordIndex: index, retired: retired})
		}
	}
	return result, nil
}

func (s *SMTInternalizerState) releasePreservedLearnedClauses(clauses []smtPreservedLearnedClause) (result error) {
	for index := len(clauses) - 1; index >= 0; index-- {
		if clauses[index].snapshot == nil {
			continue
		}
		if err := s.releaseLearnedRoot(&clauses[index].snapshot.proof); err != nil {
			return err
		}
		if err := s.releaseLemmaAtoms(clauses[index].snapshot.atoms); err != nil {
			return err
		}
	}
	return result
}

func (s *SMTInternalizerState) restoreLearnedClauses(clauses []smtPreservedLearnedClause) error {
	for index := range clauses {
		saved := &clauses[index]
		if saved.snapshot == nil {
			if err := s.reinitializeLearnedClause(saved); err != nil {
				return err
			}
			continue
		}
		if !saved.installed {
			literals, err := s.reinternalizeLemmaAtoms(saved.snapshot.atoms)
			if err != nil {
				return err
			}
			lemmas, binaries, assigned := len(s.lemmas), s.binaryClauses.count(), len(s.assignedLiterals)
			first := literals[0]
			assignment := s.assignment[first.Index()]
			err = s.addLearnedClauseWithProof(literals, saved.snapshot.proof, false)
			addedLemma := len(s.lemmas) > lemmas
			if addedLemma || s.binaryClauses.count() > binaries {
				saved.installed = true
				if addedLemma {
					saved.clause = s.lemmas[len(s.lemmas)-1].activeClause()
				} else {
					saved.clause = nil
				}
				if err != nil {
					justification := NewSMTBooleanClause(saved.clause)
					if !addedLemma {
						pair := s.binaryClauses.pair(s.binaryClauses.count() - 1)
						justification = s.binaryReason(pair[1].Negated())
					}
					remaining := s.learnedAssignmentRemainder(first, assignment, assigned, justification)
					saved.pending = &remaining
				}
			}
			if err != nil {
				return err
			}
			saved.installed = true
		}
		if err := s.finishLearnedAssignment(saved.pending); err != nil {
			return err
		}
		if saved.clause != nil && saved.clause.IsLearned() {
			if err := saved.clause.SetActivity(saved.snapshot.activity); err != nil {
				return err
			}
		}
	}
	return s.releasePreservedLearnedClauses(clauses)
}

func (s *SMTInternalizerState) captureLearnedUnit(literal SMTLiteral, proof ast.ASTHandle) error {
	expression, err := s.BoolExpression(literal.Var())
	if err != nil {
		return err
	}
	if err = s.manager.IncRef(expression); err != nil {
		return err
	}
	proof, err = s.retainProof(proof)
	if err != nil {
		unit := smtPersistentLearnedUnit{expression: expression, discard: true}
		cleanup := s.releaseLearnedUnit(&unit)
		if cleanup != nil {
			s.learnedUnits = append(s.learnedUnits, unit)
		}
		return errors.Join(err, cleanup)
	}
	s.learnedUnits = append(s.learnedUnits, smtPersistentLearnedUnit{
		expression: expression, proof: proof, sign: literal.Sign(), ownerLevel: s.baseLevel, installedLevel: s.scopeLevel,
	})
	return nil
}

func (s *SMTInternalizerState) releaseLearnedUnit(unit *smtPersistentLearnedUnit) (result error) {
	if err := s.releaseLearnedRoot(&unit.proof); err != nil {
		return err
	}
	if err := s.releaseLearnedRoot(&unit.expression); err != nil {
		return err
	}
	*unit = smtPersistentLearnedUnit{}
	return result
}

func (s *SMTInternalizerState) restoreLearnedUnits(level uint32) error {
	for index := range s.learnedUnits {
		unit := &s.learnedUnits[index]
		if unit.discard {
			continue
		}
		if err := s.finishLearnedAssignment(&unit.pending); err != nil {
			return err
		}
		if unit.ownerLevel > level || unit.installedLevel <= level {
			continue
		}
		if err := s.Internalize(unit.expression, true); err != nil {
			return err
		}
		literal, err := s.Literal(unit.expression)
		if err != nil {
			return err
		}
		if unit.sign {
			literal = literal.Negated()
		}
		assignment, assigned, owners := s.assignment[literal.Index()], len(s.assignedLiterals), len(s.ownedJustifications)
		if err = s.addLearnedClauseWithProof([]SMTLiteral{literal}, unit.proof, false); err != nil {
			justification := NewSMTBooleanAxiom()
			if len(s.ownedJustifications) > owners {
				var cause error
				justification, cause = NewSMTBooleanJustification(s.ownedJustifications[len(s.ownedJustifications)-1])
				if cause != nil {
					return errors.Join(err, cause)
				}
			}
			unit.pending = s.learnedAssignmentRemainder(literal, assignment, assigned, justification)
			if unit.pending.stage != smtLearnedAssignmentDone {
				unit.installedLevel = level
			}
			return err
		}
		unit.installedLevel = level
	}
	// reassert_units: if (at_base_level()) m_units_to_reassert.reset();
	// PopScopes has already lowered baseLevel when this runs.
	if level == s.baseLevel {
		return s.releaseLearnedUnits()
	}
	return nil
}

func (s *SMTInternalizerState) discardLearnedUnitsAbove(level uint32) (result error) {
	for index := len(s.learnedUnits) - 1; index >= 0; index-- {
		if s.learnedUnits[index].discard || s.learnedUnits[index].ownerLevel > level {
			if err := s.releaseLearnedUnit(&s.learnedUnits[index]); err != nil {
				return err
			}
			s.removeLearnedUnit(index)
		}
	}
	return result
}

func (s *SMTInternalizerState) releaseLearnedUnits() (result error) {
	for index := len(s.learnedUnits) - 1; index >= 0; index-- {
		if err := s.releaseLearnedUnit(&s.learnedUnits[index]); err != nil {
			return err
		}
		s.learnedUnits = s.learnedUnits[:index]
	}
	s.learnedUnits = nil
	return result
}

func (s *SMTInternalizerState) removeLearnedUnit(index int) {
	copy(s.learnedUnits[index:], s.learnedUnits[index+1:])
	s.learnedUnits[len(s.learnedUnits)-1] = smtPersistentLearnedUnit{}
	s.learnedUnits = s.learnedUnits[:len(s.learnedUnits)-1]
}

func (s *SMTInternalizerState) discardLearnedUnitsFrom(limit int) error {
	if limit < 0 || limit > len(s.learnedUnits) {
		return ast.ErrASTDomain
	}
	for index := limit; index < len(s.learnedUnits); index++ {
		s.learnedUnits[index].discard = true
	}
	for index := len(s.learnedUnits) - 1; index >= limit; index-- {
		if err := s.releaseLearnedUnit(&s.learnedUnits[index]); err != nil {
			return err
		}
		s.learnedUnits = s.learnedUnits[:index]
	}
	return nil
}

// checkReinitCandidates (debug builds) checks the claim preserveLearnedClauses
// relies on: no lemma outside m_clauses_to_reinit[level+1..] has an atom the
// pop retires, and every lemma on those buckets was found.
func (s *SMTInternalizerState) checkReinitCandidates(result []smtPreservedLearnedClause, booleanLimit int) error {
	level := uint32(s.pendingPop.level)
	candidates := make(map[*SMTClause]int, len(result))
	for i := range result {
		candidates[result[i].clause] = result[i].recordIndex
	}
	for index := range s.lemmas {
		clause := s.lemmas[index].activeClause()
		if clause == nil || !clause.IsLemma() {
			continue
		}
		retired := false
		for _, literal := range clause.literals {
			retired = retired || int(literal.Var()) >= booleanLimit
		}
		inBucket := clause.InReinitStack() && clause.reinitLevel > level && clause.reinitLevel <= s.scopeLevel
		_, candidate := candidates[clause]
		if (retired || inBucket) != candidate {
			return fmt.Errorf("reinit buckets out of step with m_lemmas: retired %v, on stack %v, candidate %v: %w", retired, inBucket, candidate, ast.ErrASTDomain)
		}
	}
	return nil
}
