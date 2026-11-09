package smtcore

// Source: src/smt/smt_model_finder.cpp:2762-2775,2841-2843,2917-2929.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *hintSearchState) traceTags() *diagnostic.TraceTags {
	if s.trace != nil {
		return s.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (s *hintSearchState) writeSearchState(w io.Writer, m *ast.ASTManager) error {
	if _, err := io.WriteString(w, "fs:\n"); err != nil {
		return err
	}
	for _, entry := range s.definitions.Entries() {
		f := entry.Key
		name, err := m.GetASTName(f)
		if err != nil {
			return err
		}
		if err := m.Symbols.WriteDisplay(w, name); err != nil {
			return err
		}
		if _, err := io.WriteString(w, " "); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "\nsatisfied:\n"); err != nil {
		return err
	}
	for _, q := range s.satisfied.snapshot() {
		qid, _, err := m.GetQuantifierIDs(q)
		if err != nil {
			return err
		}
		if err := m.Symbols.WriteDisplay(w, qid); err != nil {
			return err
		}
		if _, err := io.WriteString(w, " "); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "\nresidue:\n"); err != nil {
		return err
	}
	for _, q := range s.residue.snapshot() {
		qid, _, err := m.GetQuantifierIDs(q)
		if err != nil {
			return err
		}
		if err := m.Symbols.WriteDisplay(w, qid); err != nil {
			return err
		}
		if _, err := io.WriteString(w, " "); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func (s *hintSearchState) traceGreedyFunction(m *ast.ASTManager, f ast.ASTHandle, depth uint32) {
	_ = s.traceTags().EmitConditional("model_finder_hint", "greedy", "src/smt/smt_model_finder.cpp", 2841,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "greedy depth: "); err != nil {
				return err
			}
			if _, err := io.WriteString(w, strconv.FormatUint(uint64(depth), 10)); err != nil {
				return err
			}
			if _, err := io.WriteString(w, ", f: "); err != nil {
				return err
			}
			name, err := m.GetASTName(f)
			if err != nil {
				return err
			}
			if err := m.Symbols.WriteDisplay(w, name); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
			return s.writeSearchState(w, m)
		})
}

func (s *hintSearchState) traceFoundSubset(m *ast.ASTManager) {
	_ = s.traceTags().EmitConditional("model_finder_hint", "greedy", "src/smt/smt_model_finder.cpp", 2917,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "found subset that is satisfied by macros\n"); err != nil {
				return err
			}
			return s.writeSearchState(w, m)
		})
}

func (s *hintSearchState) traceResidueCandidates(m *ast.ASTManager, candidates *hintSet) {
	tags := s.traceTags()
	_ = tags.EmitConditional("model_finder_hint", "greedy", "src/smt/smt_model_finder.cpp", 2925,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "candidates from residue:\n"); err != nil {
				return err
			}
			for _, f := range candidates.keys() {
				name, err := m.GetASTName(f)
				if err != nil {
					return err
				}
				if err := m.Symbols.WriteDisplay(w, name); err != nil {
					return err
				}
				if _, err := io.WriteString(w, " "); err != nil {
					return err
				}
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
}
