package tab

import "github.com/glycerine/g3/pkg/ast"

// Session owns source imp state that intentionally survives between queries:
// predicate-selection cache/decay and monotonically increasing clause IDs.
type Session struct {
	manager      *ast.ASTManager
	selection    *predicateSelection
	nextSequence uint32
	// grounded holds the subsumption index's last setup across queries.
	grounded *groundClause
	// rules and search are imp::m_rules and imp::m_clauses of the last
	// query: m_rules.init resets the former and init_clause the latter.
	rules  *RuleIndex
	search *tabSearch
	closed bool
}

// releaseRules is tb::rules::init's reset of the previous query's rules.
func (s *Session) releaseRules() error {
	if s == nil || s.rules == nil {
		return nil
	}
	if err := s.rules.Close(); err != nil {
		return err
	}
	s.rules = nil
	return nil
}

// releaseSearch is init_clause's m_clauses.reset().
func (s *Session) releaseSearch() error {
	if s == nil || s.search == nil {
		return nil
	}
	if err := s.search.Close(); err != nil {
		return err
	}
	s.search = nil
	return nil
}

// retain keeps a finished query's rules and clauses with the engine.
func (s *Session) retain(index *RuleIndex, search *tabSearch) {
	s.nextSequence = search.nextSequence
	search.session = nil
	s.rules, s.search = index, search
}

func NewSession(manager *ast.ASTManager, strategy string) (*Session, error) {
	selection, err := newPredicateSelection(manager, strategy)
	if err != nil {
		return nil, err
	}
	return &Session{manager: manager, selection: selection}, nil
}

func (s *Session) open(manager *ast.ASTManager) error {
	if s == nil || s.closed || s.manager == nil || s.manager != manager || s.selection == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *Session) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if err := s.releaseSearch(); err != nil {
		return err
	}
	if err := s.releaseRules(); err != nil {
		return err
	}
	if s.grounded != nil {
		if err := s.grounded.Close(); err != nil {
			return err
		}
		s.grounded = nil
	}
	if err := s.selection.Close(); err != nil {
		return err
	}
	s.selection = nil
	s.manager = nil
	s.closed = true
	return nil
}
