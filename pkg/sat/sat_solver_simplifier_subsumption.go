package sat

import "github.com/glycerine/g3/pkg/arena"

// Source: src/sat/sat_simplifier.cpp; src/sat/sat_simplifier.h.

func (k *satSimplifierKernel) occurrenceCount(l Literal) (uint32, error) {
	v, e := k.useList.Get(l)
	if e != nil {
		return 0, e
	}
	return v.Size()
}
func (k *satSimplifierKernel) minOccurrenceVar(c StoredClauseRef) (BoolVar, error) {
	if e := k.open(); e != nil {
		return 0, e
	}
	bestLit, e := k.clauseLiteral(c, 0)
	if e != nil {
		return 0, e
	}
	p, e := k.occurrenceCount(bestLit)
	if e != nil {
		return 0, e
	}
	q, e := k.occurrenceCount(bestLit.Negated())
	if e != nil {
		return 0, e
	}
	best := p + q
	n, e := k.clauseSize(c)
	if e != nil {
		return 0, e
	}
	for i := uint32(1); i < n; i++ {
		l, e := k.clauseLiteral(c, i)
		if e != nil {
			return 0, e
		}
		p, e := k.occurrenceCount(l)
		if e != nil {
			return 0, e
		}
		q, e := k.occurrenceCount(l.Negated())
		if e != nil {
			return 0, e
		}
		count := p + q
		if count < best {
			bestLit = l
			best = count
		}
	}
	return bestLit.variable(), nil
}
func (k *satSimplifierKernel) minOccurrenceLiteral(c StoredClauseRef) (Literal, error) {
	if e := k.open(); e != nil {
		return 0, e
	}
	bestLit, e := k.clauseLiteral(c, 0)
	if e != nil {
		return 0, e
	}
	best, e := k.occurrenceCount(bestLit)
	if e != nil {
		return 0, e
	}
	n, e := k.clauseSize(c)
	if e != nil {
		return 0, e
	}
	for i := uint32(1); i < n; i++ {
		l, e := k.clauseLiteral(c, i)
		if e != nil {
			return 0, e
		}
		count, e := k.occurrenceCount(l)
		if e != nil {
			return 0, e
		}
		if count < best {
			bestLit = l
			best = count
		}
	}
	return bestLit, nil
}
func (k *satSimplifierKernel) subsumes1(c1, c2 StoredClauseRef, out *Literal) (bool, error) {
	if e := k.open(); e != nil {
		return false, e
	}
	// A Go nil output is outside the source reference domain, checked before use.
	if out == nil {
		return false, ErrSATOwner
	}
	v2, e := k.clauseView(c2)
	if e != nil {
		return false, e
	}
	return k.subsumes1WithViews(c1, nil, v2, out)
}

func (k *satSimplifierKernel) subsumes1WithViews(c1 StoredClauseRef, v1, v2 satClauseView, out *Literal) (bool, error) {
	n2 := v2.size()
	for i := uint32(0); i < n2; i++ {
		l := v2.literal(i)
		if e := k.markVisited(l); e != nil {
			return false, e
		}
	}
	r := true
	*out = DefaultLiteral()
	if v1 == nil {
		// A direct call resolves the source after publishing the candidate marks.
		var e error
		v1, e = k.clauseView(c1)
		if e != nil {
			return false, e
		}
	}
	n1 := v1.size()
	for i := uint32(0); i < n1; i++ {
		l := v1.literal(i)
		marked, e := k.isMarked(l)
		if e != nil {
			return false, e
		}
		if !marked {
			if *out == DefaultLiteral() {
				opposite, e := k.isMarked(l.Negated())
				if e != nil {
					return false, e
				}
				if opposite {
					*out = l.Negated()
					continue
				}
			}
			*out = DefaultLiteral()
			r = false
			break
		}
	}
	for i := uint32(0); i < n2; i++ {
		l := v2.literal(i)
		if e := k.unmarkVisited(l); e != nil {
			return false, e
		}
	}
	return r, nil
}
func (k *satSimplifierKernel) subsumes0(c1, c2 StoredClauseRef) (bool, error) {
	if e := k.open(); e != nil {
		return false, e
	}
	v2, e := k.clauseView(c2)
	if e != nil {
		return false, e
	}
	n2 := v2.size()
	for i := uint32(0); i < n2; i++ {
		l := v2.literal(i)
		if e = k.markVisited(l); e != nil {
			return false, e
		}
	}
	r := true
	v1, e := k.clauseView(c1)
	if e != nil {
		return false, e
	}
	n1 := v1.size()
	for i := uint32(0); i < n1; i++ {
		l := v1.literal(i)
		yes, e := k.isMarked(l)
		if e != nil {
			return false, e
		}
		if !yes {
			r = false
			break
		}
	}
	for i := uint32(0); i < n2; i++ {
		l := v2.literal(i)
		if e = k.unmarkVisited(l); e != nil {
			return false, e
		}
	}
	return r, nil
}
func (k *satSimplifierKernel) candidate(c1, c2 StoredClauseRef) (bool, error) {
	if c2 == c1 {
		return false, nil
	}
	v1, e := k.clauseView(c1)
	if e != nil {
		return false, e
	}
	n1 := v1.size()
	v2, e := k.clauseView(c2)
	if e != nil {
		return false, e
	}
	n2 := v2.size()
	if n1 > n2 {
		return false, nil
	}
	a1 := satApproxSet(v1.word(3))
	a2 := satApproxSet(v2.word(3))
	if !a1.approxSubset(a2) {
		return false, nil
	}
	// The size operands are unsigned: preserve modulo32 before conversion to int.
	k.subCounter = int32(uint32(k.subCounter) - (n1 + n2))
	return true, nil
}
func (k *satSimplifierKernel) collectSubsumed1Core(c StoredClauseRef, out *satClauseVector, lits *satSolverLiteralStore, list satSolverLiteralListRef, target Literal) (err error) {
	if e := k.open(); e != nil {
		return e
	}
	view, e := k.useList.Get(target)
	if e != nil {
		return e
	}
	record, e := view.record()
	if e != nil {
		return e
	}
	it := satClauseUseListsIterator{view: view, size: uint32(len(record.payload))}
	// Once iteration reaches its first live clause, the source destructor
	// finishes compaction even if the collector returns an error.
	started := false
	defer func() {
		if !started && err != nil {
			return
		}
		if e := it.Close(); err == nil {
			err = e
		}
	}()
	// The scan only changes visitation marks, use-list entries and output
	// slices. No operation can relocate or release the borrowed source.
	var v1 satClauseView
	var n1 uint32
	var a1 satApproxSet
	for !it.AtEnd() {
		c2 := record.payload[it.i]
		v2, e := k.allocator.view(c2)
		if e != nil {
			return e
		}
		if len(v2) < satClauseLiteralsOffset {
			return arena.ErrBounds
		}
		if v2.flag(satClauseFlagRemoved) {
			it.i++
			continue
		}
		if it.j != it.i {
			record.payload[it.j] = c2
		}
		started = true
		if c != c2 {
			if v1 == nil {
				v1, e = k.clauseView(c)
				if e != nil {
					return e
				}
				n1, a1 = v1.size(), satApproxSet(v1.word(3))
			}
			n2 := v2.size()
			if n1 <= n2 && a1.approxSubset(satApproxSet(v2.word(3))) {
				// The size operands are unsigned, including on underflow.
				k.subCounter = int32(uint32(k.subCounter) - (n1 + n2))
				var l Literal
				yes, e := k.subsumes1WithViews(c, v1, v2, &l)
				if e != nil {
					return e
				}
				if yes {
					if e = out.Push(c2); e != nil {
						return e
					}
					if e = lits.Push(list, l); e != nil {
						return e
					}
				}
			}
		}
		it.i++
		it.j++
	}
	return nil
}
func (k *satSimplifierKernel) collectSubsumed1(c StoredClauseRef, out *satClauseVector, lits *satSolverLiteralStore, list satSolverLiteralListRef) error {
	v, e := k.minOccurrenceVar(c)
	if e != nil {
		return e
	}
	if e = k.collectSubsumed1Core(c, out, lits, list, MakeLiteral(v, false)); e != nil {
		return e
	}
	return k.collectSubsumed1Core(c, out, lits, list, MakeLiteral(v, true))
}
func (k *satSimplifierKernel) collectSubsumed0Core(c StoredClauseRef, out *satClauseVector, target Literal) (err error) {
	if e := k.open(); e != nil {
		return e
	}
	view, e := k.useList.Get(target)
	if e != nil {
		return e
	}
	it, e := view.Begin()
	if e != nil {
		return e
	}
	defer func() {
		if e := it.Close(); err == nil {
			err = e
		}
	}()
	for !it.AtEnd() {
		c2, e := it.Current()
		if e != nil {
			return e
		}
		if satStorageSourceDebug {
			dead, e := k.clauseRemoved(c2)
			if e != nil {
				return e
			}
			if dead {
				return ErrSATAssertion
			}
		}
		yes, e := k.candidate(c, c2)
		if e != nil {
			return e
		}
		if yes {
			yes, e = k.subsumes0(c, c2)
			if e != nil {
				return e
			}
			if yes {
				if e = out.Push(c2); e != nil {
					return e
				}
			}
		}
		if e = it.Next(); e != nil {
			return e
		}
	}
	return nil
}
func (k *satSimplifierKernel) collectSubsumed0(c StoredClauseRef, out *satClauseVector) error {
	l, e := k.minOccurrenceLiteral(c)
	if e != nil {
		return e
	}
	return k.collectSubsumed0Core(c, out, l)
}
