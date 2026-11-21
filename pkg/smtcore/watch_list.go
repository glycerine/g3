package smtcore

// Source: src/smt/watch_list.h and src/smt/watch_list.cpp.

import "github.com/glycerine/g3/pkg/ast"

// SMTWatchList stores the two source watch-list regions as typed Go slices.
// Clause pointers are in insertion order. Literals keep the source layout:
// they grow downward from the end of their buffer (insert_literal writes in
// front of the others, and expand keeps them at the end), so literals[begin:]
// is in iterator order, which is reverse insertion order, and inserting or
// removing the newest literal moves nothing.
type SMTWatchList struct {
	clauses  []*SMTClause
	literals []SMTLiteral // literals[begin:] are live, newest first
	begin    int
}

func (w *SMTWatchList) Size() int {
	if w == nil {
		return 0
	}
	return len(w.clauses) + len(w.literals) - w.begin
}

func (w *SMTWatchList) Reset() {
	if w == nil {
		return
	}
	w.clauses = w.clauses[:0]
	w.begin = len(w.literals)
}

// recycle releases retired clause pointers and keeps the buffers for the
// next variable in this slot. Native resets the list when publishing the ID.
func (w *SMTWatchList) recycle() {
	// A single watched clause needs just one pointer store.
	if len(w.clauses) == 1 {
		w.clauses[0] = nil
	} else {
		clear(w.clauses)
	}
	w.clauses = w.clauses[:0]
	w.begin = len(w.literals)
}

func (w *SMTWatchList) ResetAndReleaseMemory() {
	if w == nil {
		return
	}
	w.clauses = nil
	w.literals = nil
	w.begin = 0
}

// Clauses and Literals return the live mutable views used by propagation.
func (w *SMTWatchList) Clauses() []*SMTClause {
	if w == nil {
		return nil
	}
	return w.clauses
}

func (w *SMTWatchList) Literals() []SMTLiteral {
	if w == nil {
		return nil
	}
	return w.literals[w.begin:]
}

func (w *SMTWatchList) FindClause(clause *SMTClause) int {
	if w == nil {
		return -1
	}
	for i, candidate := range w.clauses {
		if candidate == clause {
			return i
		}
	}
	return -1
}

func (w *SMTWatchList) FindLiteral(literal SMTLiteral) int {
	if w == nil {
		return -1
	}
	for i, candidate := range w.literals[w.begin:] {
		if candidate == literal {
			return i
		}
	}
	return -1
}

func (w *SMTWatchList) InsertClause(clause *SMTClause) error {
	if w == nil {
		return ast.ErrASTHandle
	}
	w.clauses = append(w.clauses, clause)
	return nil
}

func (w *SMTWatchList) InsertLiteral(literal SMTLiteral) error {
	if w == nil {
		return ast.ErrASTHandle
	}
	if w.begin == 0 {
		// expand: grow by half and keep the literals at the end.
		live := len(w.literals)
		capacity := live + live/2 + 4
		grown := make([]SMTLiteral, capacity)
		copy(grown[capacity-live:], w.literals)
		w.literals, w.begin = grown, capacity-live
	}
	w.begin--
	w.literals[w.begin] = literal
	return nil
}

func (w *SMTWatchList) RemoveClause(clause *SMTClause) bool {
	i := w.FindClause(clause)
	if i < 0 {
		return false
	}
	copy(w.clauses[i:], w.clauses[i+1:])
	w.clauses[len(w.clauses)-1] = nil
	w.clauses = w.clauses[:len(w.clauses)-1]
	return true
}

func (w *SMTWatchList) RemoveLiteral(literal SMTLiteral) bool {
	i := w.FindLiteral(literal)
	if i < 0 {
		return false
	}
	// remove_literal shifts only the literals in front of the removed one.
	live := w.literals[w.begin:]
	copy(live[1:i+1], live[:i])
	w.begin++
	return true
}

// SetClauseCount is the source set_end_clause operation used after a caller
// compacts the mutable Clauses view in place.
func (w *SMTWatchList) SetClauseCount(count int) error {
	if w == nil {
		return ast.ErrASTHandle
	}
	if count < 0 || count > len(w.clauses) {
		return ast.ErrASTDomain
	}
	for i := count; i < len(w.clauses); i++ {
		w.clauses[i] = nil
	}
	w.clauses = w.clauses[:count]
	return nil
}
