package sat

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Cursor borrows follow the captured native backing, with explicit lifetime and
// current logical bounds checks before each propagation read/write.
func satPropagationCursorRecord(i *satSolverWatchIterator, n uint32) (*satWatchBacking, error) {
	if i == nil || i.store == nil {
		return nil, ErrSATOwner
	}
	if e := i.store.open(); e != nil {
		return nil, e
	}
	if i.backing == nil || !i.backing.alive {
		return nil, arena.ErrInvalidSlot
	}
	if uint64(n) >= uint64(len(i.backing.data)) {
		return nil, arena.ErrBounds
	}
	return i.backing, nil
}
func satPropagationCursorRead(i *satSolverWatchIterator, n uint32) (satWatched, error) {
	x, e := satPropagationCursorRecord(i, n)
	if e != nil {
		return satWatched{}, e
	}
	return x.data[n], nil
}
func satPropagationCursorWrite(i *satSolverWatchIterator, n uint32, w satWatched) error {
	x, e := satPropagationCursorRecord(i, n)
	if e != nil {
		return e
	}
	x.data[n] = w
	return nil
}
func (s *satSolverPropagationState) propagationConflictCleanup(i *satSolverWatchIterator, from uint32, to *uint32, w satSolverWatchListView) error {
	// A conflict preserves the unread suffix in its original order. No
	// source callback runs while this iterator owns the backing.
	if from < i.end {
		backing, err := satPropagationCursorRecord(i, from)
		if err != nil {
			return err
		}
		if *to <= from && uint64(i.end) <= uint64(len(backing.data)) {
			count := i.end - from
			if *to != from {
				copy(backing.data[*to:*to+count], backing.data[from:i.end])
			}
			*to += count
			return s.finishPropagationList(i, w, *to)
		}
	}
	// Preserve the cursor and the first error for malformed ranges.
	for ; from < i.end; from++ {
		x, e := satPropagationCursorRead(i, from)
		if e != nil {
			return e
		}
		if e = satPropagationCursorWrite(i, *to, x); e != nil {
			return e
		}
		*to++
	}
	return s.finishPropagationList(i, w, *to)
}
func (s *satSolverPropagationState) propagateCore(update bool) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if s.inconsistent {
		return false, nil
	}
	v := s.variables
	tracing := s.tracingPropagation()
	for {
		size, e := v.trail.Size(v.trailRef)
		if e != nil {
			return false, e
		}
		if v.qhead >= size {
			break
		}
		if e = s.checkpoint(); e != nil {
			return false, e
		}
		if s.cleanerCounter == -1<<31 {
			return false, ErrSATSignedCounterDomain
		}
		s.cleanerCounter--
		if satStorageSourceDebug && s.inconsistent {
			return false, ErrSATAssertion
		}
		l, e := v.trailRead(v.qhead)
		if e != nil {
			return false, e
		}
		if tracing {
			s.tracePropagating(l)
		}
		v.qhead++
		not := l.Negated()
		if satStorageSourceDebug {
			a, e := v.valueLiteral(l)
			if e != nil {
				return false, e
			}
			if a != lbool.True {
				return false, ErrSATAssertion
			}
			a, e = v.valueLiteral(not)
			if e != nil {
				return false, e
			}
			if a != lbool.False {
				return false, ErrSATAssertion
			}
		}
		list, e := v.watches.Get(l.index())
		if e != nil {
			return false, e
		}
		count, e := list.Size()
		if e != nil {
			return false, e
		}
		s.asymmCounter = int32(uint32(s.asymmCounter) - count)
		s.probingCounter = int32(uint32(s.probingCounter) - count)
		ref, e := list.ref()
		if e != nil {
			return false, e
		}
		iter, e := v.watches.store.Iter(ref)
		if e != nil {
			return false, e
		}
		out := uint32(0)
		// The list keeps its backing through this loop, since a clause's new
		// watch is never this literal's. Only an extension can reallocate it.
		var data []satWatched
		if iter.backing != nil {
			data = iter.backing.data
		}
		if uint64(iter.end) > uint64(len(data)) {
			return false, arena.ErrBounds
		}
		assignment := v.assignment
		for pos := uint32(0); pos < iter.end; pos++ {
			w := data[pos]
			keep := true
			switch w.Kind() {
			case 0:
				l1 := Literal(w.val1)
				switch assignment[l1.index()] {
				case lbool.False:
					if e = s.propagationConflictCleanup(&iter, pos, &out, list); e != nil {
						return false, e
					}
					s.setConflict(newSATBinaryJustification(not), l1.Negated())
					return false, nil
				case lbool.Undef:
					s.binaryPropagations++
					if e = s.assignCore(l1, newSATBinaryJustification(not)); e != nil {
						return false, e
					}
				}
			case 1:
				l1, e := w.Literal1()
				if e != nil {
					return false, e
				}
				l2, e := w.Literal2()
				if e != nil {
					return false, e
				}
				a, b := assignment[l1.index()], assignment[l2.index()]
				if a == lbool.False && b == lbool.Undef {
					s.ternaryPropagations++
					if e = s.assignCore(l2, newSATTernaryJustification(l1, not)); e != nil {
						return false, e
					}
				} else if a == lbool.Undef && b == lbool.False {
					s.ternaryPropagations++
					if e = s.assignCore(l1, newSATTernaryJustification(l2, not)); e != nil {
						return false, e
					}
				} else if a == lbool.False && b == lbool.False {
					if e = s.propagationConflictCleanup(&iter, pos, &out, list); e != nil {
						return false, e
					}
					s.setConflict(newSATTernaryJustification(l1, not), l2.Negated())
					return false, nil
				}
			case 2:
				blocked, e := w.BlockedLiteral()
				if e != nil {
					return false, e
				}
				if assignment[blocked.index()] == lbool.True {
					if tracing {
						s.traceBlockedClause(blocked, w)
					}
					break
				}
				off, e := w.ClauseOffset()
				if e != nil {
					return false, e
				}
				c := StoredClauseRef(off)
				clause, e := s.clauses.view(c)
				if e != nil {
					return false, e
				}
				if tracing {
					s.traceProcessingClause(c)
				}
				c0, c1 := clause.watchedLiterals(not)
				if tracing {
					s.traceClauseMismatch(l, c, c1 != not)
				}
				if clause.flag(satClauseFlagRemoved) || c1 != not {
					break
				}
				if assignment[c0.index()] == lbool.True {
					w.SetClause(c0, off)
					break
				}
				replacement, moved := clause.moveWatch(not, assignment)
				if moved {
					if e = v.watches.pushClause(replacement.Negated().index(), c0, off); e != nil {
						return false, e
					}
					keep = false
					break
				}
				value := assignment[c0.index()]
				if satStorageSourceDebug && value != lbool.False && value != lbool.Undef {
					return false, ErrSATAssertion
				}
				clause.setFlag(satClauseFlagUsed)
				if value == lbool.False {
					if e = s.propagationConflictCleanup(&iter, pos, &out, list); e != nil {
						return false, e
					}
					s.setConflict(newSATClauseJustification(off), DefaultLiteral())
					return false, nil
				}
				// Source retains this watch BEFORE increment/mark/assignment/glue actions.
				data[out] = w
				out++
				keep = false
				s.clausePropagations++
				if e = s.assignCore(c0, newSATClauseJustification(off)); e != nil {
					return false, e
				}
				if update {
					learned, e := s.clauses.Learned(c)
					if e != nil {
						return false, e
					}
					if learned {
						glue, e := s.clauses.Glue(c)
						if e != nil {
							return false, e
						}
						if glue > 2 {
							g, e := s.numDiffLevels(c)
							if e != nil {
								return false, e
							}
							if g+1 < glue {
								if e = s.clauses.SetGlue(c, g); e != nil {
									return false, e
								}
							}
						}
					}
				}
			case 3:
				if satStorageSourceDebug && !s.extensionPresent {
					return false, ErrSATAssertion
				}
				if keep, e = s.propagateExtension(l, w, keep); e != nil {
					return false, e
				}
				if iter.backing == nil || !iter.backing.alive {
					return false, arena.ErrInvalidSlot
				}
				data = iter.backing.data
				if uint64(iter.end) > uint64(len(data)) {
					return false, arena.ErrBounds
				}
				assignment = v.assignment
				if s.inconsistent {
					if e = s.propagationConflictCleanup(&iter, pos, &out, list); e != nil {
						return false, e
					}
					return false, nil
				}
			default:
				return false, ErrSATAssertion
			}
			if keep {
				data[out] = w
				out++
			}
		}
		if e = s.finishPropagationList(&iter, list, out); e != nil {
			return false, e
		}
	}
	if satStorageSourceDebug {
		n, e := v.trail.Size(v.trailRef)
		if e != nil {
			return false, e
		}
		if v.qhead != n || s.inconsistent {
			return false, ErrSATAssertion
		}
	}
	return true, nil
}

// Source set_end subtracts the captured pointer from the current list backing.
// Validate that relation at this action, including the no-more-reads path.
func (s *satSolverPropagationState) finishPropagationList(i *satSolverWatchIterator, w satSolverWatchListView, n uint32) error {
	r, e := w.ref()
	if e != nil {
		return e
	}
	x, e := i.store.record(r)
	if e != nil {
		return e
	}
	if x.backing != i.backing {
		return arena.ErrInvalidSlot
	}
	return x.setEnd(n)
}
