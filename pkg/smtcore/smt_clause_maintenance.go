package smtcore

// Source: src/smt/smt_context.cpp learned-clause activity and inactive-lemma
// deletion. The retained search has no restart loop, so fixed/geometric
// conflict-triggered collection is the live path.

import (
	"sort"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type SMTClauseMaintenanceConfig struct {
	Strategy                   uint32
	Half                       bool
	Recent, Initial            uint32
	Factor, InverseDecay       float64
	NewOldRatio                uint32
	NewActivity, OldActivity   uint32
	NewRelevancy, OldRelevancy uint32
}

func defaultSMTClauseMaintenanceConfig() SMTClauseMaintenanceConfig {
	return SMTClauseMaintenanceConfig{
		Strategy: 0, Recent: 100, Initial: 5000, Factor: 1.1, InverseDecay: 1,
		NewOldRatio: 16, NewActivity: 10, OldActivity: 500, NewRelevancy: 45, OldRelevancy: 6,
	}
}

// smtClauseMaintenanceConfigFor selects the m_fparams fields that lemma
// collection reads.
func smtClauseMaintenanceConfigFor(state SMTParamsState) SMTClauseMaintenanceConfig {
	return SMTClauseMaintenanceConfig{
		Strategy: state.LemmaGcStrategy, Half: state.LemmaGcHalf,
		Recent: state.RecentLemmasSize, Initial: state.LemmaGcInitial,
		Factor: state.LemmaGcFactor, InverseDecay: state.InvClauseDecay,
		NewOldRatio: state.NewOldRatio, NewActivity: state.NewClauseActivity,
		OldActivity: state.OldClauseActivity, NewRelevancy: state.NewClauseRelevancy,
		OldRelevancy: state.OldClauseRelevancy,
	}
}

func (s *SMTInternalizerState) ConfigureClauseMaintenance(config SMTClauseMaintenanceConfig) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.numClauseRecords() != 0 || len(s.scopes) != 0 || config.Strategy > 3 || config.NewOldRatio == 0 || config.Factor < 1 || config.InverseDecay <= 0 {
		return ast.ErrASTDomain
	}
	s.clauseMaintenance = config
	s.lemmaGCThreshold = config.Initial
	s.conflictsSinceLemmaGC = 0
	return nil
}

// clauseIsReason is context::is_justifying: the clause is the justification
// of the variable of one of its two watch literals.
func (s *SMTInternalizerState) clauseIsReason(clause *SMTClause) bool {
	for i := 0; i < 2 && i < len(clause.literals); i++ {
		value, ok := s.booleanData[clause.literals[i].Var()].Justification().Clause()
		if ok && value == clause {
			return true
		}
	}
	return false
}

// clauseCanDelete is context::can_delete.
func (s *SMTInternalizerState) clauseCanDelete(clause *SMTClause) bool {
	return !clause.InReinitStack() && !s.clauseIsReason(clause)
}

func (s *SMTInternalizerState) clauseHasUnknowns(clause *SMTClause, threshold uint32) (bool, error) {
	if threshold == 0 {
		return true, nil
	}
	for _, literal := range clause.literals {
		value, err := s.Assignment(literal)
		if err != nil {
			return false, err
		}
		if value == lbool.Undef {
			threshold--
			if threshold == 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

// delClause is context::del_clause for a record of m_aux_clauses or
// m_lemmas.
func (s *SMTInternalizerState) delClause(record *SMTInternalizedClause) error {
	if err := s.closeClause(record); err != nil {
		return err
	}
	s.statistics.NumDelClause++
	return nil
}

// compactClauseRecords finishes an in-place filter of records that read
// up to i and kept j records: on error the unread tail moves down behind
// the kept prefix, so the list holds every live record exactly once.
func compactClauseRecords(records []SMTInternalizedClause, i, j int) []SMTInternalizedClause {
	if i != j {
		j += copy(records[j:], records[i:])
		clear(records[j:])
	}
	return records[:j]
}

// reindexLemmas repairs indices after sorting or compacting live records.
func (s *SMTInternalizerState) reindexLemmas(start, end int) {
	for i := start; i < end; i++ {
		s.lemmas[i].clause.lemmaIndex = i
	}
}

// lemmasStart is m_base_lvl == 0 ? 0 : m_base_scopes[m_base_lvl - 1].m_lemmas_lim.
func (s *SMTInternalizerState) lemmasStart() (int, error) {
	if s.baseLevel == 0 {
		return 0, nil
	}
	if int(s.baseLevel) > len(s.scopes) {
		return 0, ast.ErrASTDomain
	}
	start := s.scopes[s.baseLevel-1].lemmaLimit
	if start < 0 || start > len(s.lemmas) {
		return 0, ast.ErrASTDomain
	}
	return start, nil
}

// garbageCollectLearnedClauses is context::del_inactive_lemmas without the
// threshold update, which delInactiveLemmasIfDue does.
func (s *SMTInternalizerState) garbageCollectLearnedClauses() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.clauseMaintenance.Strategy == 3 { // LGC_NONE
		return nil
	}
	if s.clauseMaintenance.Half {
		return s.delInactiveLemmas1()
	}
	return s.delInactiveLemmas2()
}

// clauseActivity is clause::get_activity of a lemma.
func clauseActivity(record *SMTInternalizedClause) uint32 {
	activity, _ := record.clause.Activity()
	return activity
}

// delInactiveLemmas1 is context::del_inactive_lemmas1: it sorts the old
// lemmas by activity in place, so the next collection starts from that
// order, and deletes (approximately) the less active half.
func (s *SMTInternalizerState) delInactiveLemmas1() (result error) {
	config := s.clauseMaintenance
	sz := len(s.lemmas)
	startAt, err := s.lemmasStart()
	if err != nil {
		return err
	}
	if uint64(startAt)+uint64(config.Recent) >= uint64(sz) {
		return nil
	}
	// IF_VERBOSE(2, "(smt.delete-inactive-lemmas") (AUDIT.md FUZZ-F5;
	// g3trace builds only).
	if diagnostic.VerboseProgress {
		if err := progressVerbose(2, "(smt.delete-inactive-lemmas"); err != nil {
			return err
		}
	}
	numDelCls := 0
	endAt := sz - int(config.Recent)
	// std::stable_sort(m_lemmas.begin() + start_at, m_lemmas.begin() + end_at, clause_lt()).
	old := s.lemmas[startAt:endAt]
	sort.SliceStable(old, func(i, j int) bool { return clauseActivity(&old[i]) > clauseActivity(&old[j]) })
	startDelAt := (startAt + endAt) / 2
	s.reindexLemmas(startAt, startDelAt)
	i, j := startDelAt, startDelAt
	if smtTraceEnabled("del_inactive_lemmas") {
		emitSMTTrace("del_inactive_lemmas", "del_inactive_lemmas1", "src/smt/smt_context.cpp", 2698,
			"sz: "+strconv.Itoa(sz)+", start_at: "+strconv.Itoa(startAt)+", end_at: "+strconv.Itoa(endAt)+
				", start_del_at: "+strconv.Itoa(startDelAt)+"\n")
	}
	defer func() {
		if result != nil {
			s.lemmas = compactClauseRecords(s.lemmas, i, j)
			s.reindexLemmas(j, len(s.lemmas))
		}
	}()
	for ; i < endAt; i++ {
		record := s.lemmas[i]
		if s.clauseCanDelete(record.clause) {
			s.traceDelInactiveLemma(record.clause)
			if err := s.delClause(&s.lemmas[i]); err != nil {
				return err
			}
			numDelCls++
			continue
		}
		s.lemmas[j] = record
		record.clause.lemmaIndex = j
		j++
	}
	// keep recent clauses
	for ; i < sz; i++ {
		record := s.lemmas[i]
		if record.clause.Deleted() && s.clauseCanDelete(record.clause) {
			if err := s.delClause(&s.lemmas[i]); err != nil {
				return err
			}
			numDelCls++
			continue
		}
		s.lemmas[j] = record
		record.clause.lemmaIndex = j
		j++
	}
	clear(s.lemmas[j:])
	s.lemmas = s.lemmas[:j]
	// The source then rescales activities when m_clause_decay > 1. No
	// parameter sets m_clause_decay, which stays 1, so that loop is not
	// reached.
	return deleteInactiveVerboseEnd(numDelCls)
}

// deleteInactiveVerboseEnd is del_inactive_lemmas1/2's closing
// IF_VERBOSE(2, " :num-deleted-clauses " << num_del_cls << ")" << std::endl).
func deleteInactiveVerboseEnd(numDelCls int) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	return progressVerbose(2, " :num-deleted-clauses "+strconv.Itoa(numDelCls)+")\n")
}

// traceDelInactiveLemma is del_inactive_lemmas1's TRACE of a deleted lemma:
// display_clause and its activity.
func (s *SMTInternalizerState) traceDelInactiveLemma(clause *SMTClause) {
	if !smtTraceEnabled("del_inactive_lemmas") {
		return
	}
	var out strings.Builder
	out.WriteString("deleting: ")
	if err := clause.DisplayCompact(&out, s.boolExpr); err != nil {
		out.WriteString("<" + err.Error() + ">")
	}
	activity, _ := clause.Activity()
	out.WriteString(", activity: " + strconv.FormatUint(uint64(activity), 10) + "\n")
	emitSMTTrace("del_inactive_lemmas", "del_inactive_lemmas1", "src/smt/smt_context.cpp", 2703, out.String())
}

// delInactiveLemmas2 is context::del_inactive_lemmas2.
func (s *SMTInternalizerState) delInactiveLemmas2() (result error) {
	// IF_VERBOSE(2, "(smt.delete-inactive-clauses ") (AUDIT.md FUZZ-F5;
	// g3trace builds only).
	if diagnostic.VerboseProgress {
		if err := progressVerbose(2, "(smt.delete-inactive-clauses "); err != nil {
			return err
		}
	}
	numDelCls := 0
	config := s.clauseMaintenance
	sz := len(s.lemmas)
	startAt, err := s.lemmasStart()
	if err != nil {
		return err
	}
	realSz := uint32(sz - startAt)
	// idx of the first learned clause considered "new"
	newFirstIdx := uint32(startAt) + (realSz/config.NewOldRatio)*(config.NewOldRatio-1)
	i, j := startAt, startAt
	defer func() {
		if result != nil {
			s.lemmas = compactClauseRecords(s.lemmas, i, j)
			s.reindexLemmas(j, len(s.lemmas))
		}
	}()
	for ; i < sz; i++ {
		record := s.lemmas[i]
		clause := record.clause
		activity, err := clause.Activity()
		if err != nil {
			return err
		}
		if s.clauseCanDelete(clause) {
			if clause.Deleted() {
				// clause is already marked for deletion
				if err := s.delClause(&s.lemmas[i]); err != nil {
					return err
				}
				numDelCls++
				continue
			}
			// act_threshold is old - (old - new) * ((i - start_at) / real_sz)
			// in unsigned arithmetic; (i - start_at) / real_sz is 0 for every
			// lemma.
			actThreshold := config.OldActivity - (config.OldActivity-config.NewActivity)*(uint32(i-startAt)/realSz)
			if activity < actThreshold {
				relThreshold := config.OldRelevancy
				if uint32(i) >= newFirstIdx {
					relThreshold = config.NewRelevancy
				}
				unknowns, err := s.clauseHasUnknowns(clause, relThreshold)
				if err != nil {
					return err
				}
				if unknowns {
					if err := s.delClause(&s.lemmas[i]); err != nil {
						return err
					}
					numDelCls++
					continue
				}
			}
		}
		s.lemmas[j] = record
		record.clause.lemmaIndex = j
		j++
		if err = clause.SetActivity(uint32(float64(activity) / config.InverseDecay)); err != nil {
			i++
			return err
		}
	}
	clear(s.lemmas[j:])
	s.lemmas = s.lemmas[:j]
	return deleteInactiveVerboseEnd(numDelCls)
}

// delInactiveLemmasIfDue is bounded_search's
// if (m_num_conflicts_since_lemma_gc > m_lemma_gc_threshold &&
// (LGC_FIXED || LGC_GEOMETRIC)) del_inactive_lemmas(); resolve_conflict
// counts the conflicts.
func (s *SMTInternalizerState) delInactiveLemmasIfDue() error {
	if s.clauseMaintenance.Strategy > 1 || s.conflictsSinceLemmaGC <= s.lemmaGCThreshold {
		return nil
	}
	if err := s.garbageCollectLearnedClauses(); err != nil {
		return err
	}
	s.conflictsSinceLemmaGC = 0
	if s.clauseMaintenance.Strategy == 1 {
		s.lemmaGCThreshold = uint32(float64(s.lemmaGCThreshold) * s.clauseMaintenance.Factor)
	}
	return nil
}
