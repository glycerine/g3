package rewriter

// Source: src/ast/rewriter/seq_rewriter.cpp seq_rewriter::mk_str_in_regexp,
// add_next and the two is_sequence overloads; src/util/hashtable.h
// core_hashtable for the u_map<expr*> frontiers, whose iteration order
// decides the order in which the rewrite builds its conditions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

const seqUMapInitialCapacity = 8 // DEFAULT_HASHTABLE_INITIAL_CAPACITY

type seqUMapEntryState uint8

const (
	seqUMapFree seqUMapEntryState = iota
	seqUMapDeleted
	seqUMapUsed
)

type seqUMapEntry struct {
	state seqUMapEntryState
	key   uint32
	value ast.ASTHandle
}

// seqUMap is u_map<expr*>: open addressing on u_hash(k) == k. Its values
// are raw pointers; the rewrite's trail owns them.
type seqUMap struct {
	table      []seqUMapEntry
	size       int
	numDeleted int
}

func newSeqUMap() seqUMap {
	return seqUMap{table: make([]seqUMapEntry, seqUMapInitialCapacity)}
}

func (u *seqUMap) moveTable(target []seqUMapEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range u.table {
		if entry.state != seqUMapUsed {
			continue
		}
		begin := int(entry.key & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if target[i].state == seqUMapFree {
				target[i], placed = entry, true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if target[i].state == seqUMapFree {
				target[i], placed = entry, true
			}
		}
	}
}

// insert is core_hashtable::insert: an existing key takes the new value.
func (u *seqUMap) insert(key uint32, value ast.ASTHandle) {
	if (u.size+u.numDeleted)<<2 > len(u.table)*3 {
		target := make([]seqUMapEntry, len(u.table)<<1)
		u.moveTable(target)
		u.table, u.numDeleted = target, 0
	}
	mask := uint32(len(u.table) - 1)
	begin := int(key & mask)
	deleted := -1
	visit := func(i int) bool {
		curr := &u.table[i]
		switch curr.state {
		case seqUMapUsed:
			if curr.key == key {
				curr.value = value
				return true
			}
		case seqUMapFree:
			n := i
			if deleted >= 0 {
				n = deleted
				u.numDeleted--
			}
			u.table[n] = seqUMapEntry{state: seqUMapUsed, key: key, value: value}
			u.size++
			return true
		default:
			deleted = i
		}
		return false
	}
	for i := begin; i < len(u.table); i++ {
		if visit(i) {
			return
		}
	}
	for i := 0; i < begin; i++ {
		if visit(i) {
			return
		}
	}
}

func (u *seqUMap) find(key uint32) (ast.ASTHandle, bool) {
	mask := uint32(len(u.table) - 1)
	begin := int(key & mask)
	for k := 0; k < len(u.table); k++ {
		curr := &u.table[(begin+k)%len(u.table)]
		switch curr.state {
		case seqUMapUsed:
			if curr.key == key {
				return curr.value, true
			}
		case seqUMapFree:
			return ast.ASTHandle{}, false
		}
	}
	return ast.ASTHandle{}, false
}

// reset is core_hashtable::reset, which halves a mostly free large table.
func (u *seqUMap) reset() {
	if u.size == 0 && u.numDeleted == 0 {
		return
	}
	overhead := 0
	for i := range u.table {
		if u.table[i].state != seqUMapFree {
			u.table[i] = seqUMapEntry{}
		} else {
			overhead++
		}
	}
	if len(u.table) > 16 && overhead<<2 > len(u.table)*3 {
		u.table = make([]seqUMapEntry, len(u.table)>>1)
	}
	u.size, u.numDeleted = 0, 0
}

// entries lists the used entries in iteration order.
func (u *seqUMap) entries() []seqUMapEntry {
	result := make([]seqUMapEntry, 0, u.size)
	for _, entry := range u.table {
		if entry.state == seqUMapUsed {
			result = append(result, entry)
		}
	}
	return result
}

// seqRefVector is expr_ref_vector: reset and destruction release forward.
type seqRefVector struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (v *seqRefVector) push(h ast.ASTHandle) error {
	ref, err := v.manager.NewRef(h)
	if err != nil {
		return err
	}
	v.refs = append(v.refs, ref)
	return nil
}

func (v *seqRefVector) get(i int) (ast.ASTHandle, error) { return v.manager.RefGet(v.refs[i]) }

func (v *seqRefVector) handles() ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(v.refs))
	for i := range v.refs {
		h, err := v.get(i)
		if err != nil {
			return nil, err
		}
		result[i] = h
	}
	return result, nil
}

func (v *seqRefVector) reset() (err error) {
	for i := range v.refs {
		err = errors.Join(err, v.manager.DelRef(&v.refs[i]))
	}
	v.refs = v.refs[:0]
	return err
}

// isSequenceAutomaton is seq_rewriter::is_sequence(eautomaton&, seq).
func (r *SequenceRewriter) isSequenceAutomaton(aut *StringRegexAutomaton, seq *seqRefVector) (bool, error) {
	if err := seq.reset(); err != nil {
		return false, err
	}
	g, err := aut.Graph()
	if err != nil {
		return false, err
	}
	state := g.Initial()
	visited := make(map[uint32]bool)
	hasFinal := func(state uint32) bool {
		for _, s := range g.EpsilonClosure(state) {
			if g.IsFinal(s) {
				return true
			}
		}
		return false
	}
	final := hasFinal(state)
	mvs := g.ExpandedMovesFrom(state, true)
	for !final {
		if len(mvs) != 1 || visited[state] || g.IsFinal(mvs[0].Source) {
			return false, nil
		}
		visited[state] = true
		if mvs[0].Epsilon {
			return false, nil
		}
		condition, err := aut.Condition(mvs[0].Condition)
		if err != nil {
			return false, err
		}
		if condition.Kind != StringRegexCharacter {
			return false, nil
		}
		unit, err := r.manager.MkSequenceUnit(condition.Character)
		if err != nil {
			return false, err
		}
		if err = seq.push(unit); err != nil {
			return false, err
		}
		state = mvs[0].Destination
		mvs = g.ExpandedMovesFrom(state, true)
		final = hasFinal(state)
	}
	return len(mvs) == 0, nil
}

// isSequenceExpr is seq_rewriter::is_sequence(expr*, seq): a stack walk
// that visits the right operand first and a string's last character first,
// and then reverses the result.
func (r *SequenceRewriter) isSequenceExpr(e ast.ASTHandle, seq *seqRefVector) (bool, error) {
	m := r.manager
	if err := seq.reset(); err != nil {
		return false, err
	}
	todo := []ast.ASTHandle{e}
	for len(todo) > 0 {
		e = todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if text, isString, err := m.SequenceStringValue(e); err != nil {
			return false, err
		} else if isString {
			for i := len(text); i > 0; {
				i--
				ch, err := m.BitVectorNumeralUnsigned(uint32(text[i]), 8)
				if err != nil {
					return false, err
				}
				if err = seq.push(ch); err != nil {
					return false, err
				}
			}
			continue
		}
		if empty, err := m.IsSequenceEmpty(e); err != nil {
			return false, err
		} else if empty {
			continue
		}
		if element, unit, err := m.SequenceUnitArgument(e); err != nil {
			return false, err
		} else if unit {
			if err = seq.push(element); err != nil {
				return false, err
			}
			continue
		}
		if left, right, concat, err := m.SequenceConcatParts(e); err != nil {
			return false, err
		} else if concat {
			todo = append(todo, left, right)
			continue
		}
		return false, nil
	}
	for i, j := 0, len(seq.refs)-1; i < j; i, j = i+1, j-1 {
		seq.refs[i], seq.refs[j] = seq.refs[j], seq.refs[i]
	}
	return true, nil
}

// mkSeqUtilConcat is seq_util::str::mk_concat(n, es): one element is
// returned as is; more are nested to the right as mk_app does for a
// right-associative declaration.
func (r *SequenceRewriter) mkSeqUtilConcat(es []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(es) == 1 {
		return es[0], nil
	}
	j := len(es) - 1
	result, err := r.manager.MkSequenceConcat(es[j-1], es[j])
	for j--; err == nil && j > 0; {
		j--
		result, err = r.manager.MkSequenceConcat(es[j], result)
	}
	return result, err
}

// rewriteStringInRegex is seq_rewriter::mk_str_in_regexp.
func (r *SequenceRewriter) rewriteStringInRegex(a, b ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	m := r.manager
	if empty, err := r.sequenceOperation(b, 22); err != nil {
		return RewriteFailed, err
	} else if empty {
		return r.assignBoolean(false, out)
	}
	if full, err := r.sequenceOperation(b, 23); err != nil {
		return RewriteFailed, err
	} else if full {
		return r.assignBoolean(true, out)
	}
	// scoped_ptr<eautomaton> aut; expr_ref_vector seq. Locals are destroyed
	// in reverse order, aut last.
	aut, err := NewSeqAutomaton(m, b, r.solver)
	if err != nil || aut == nil {
		return RewriteFailed, err
	}
	seq := &seqRefVector{manager: m}
	defer func() { err = errors.Join(err, seq.reset(), aut.Close()) }()
	isSeq, err := r.isSequenceAutomaton(aut, seq)
	if err != nil {
		return RewriteFailed, err
	}
	if isSeq {
		var rhs ast.ASTHandle
		if len(seq.refs) == 0 {
			sort, err := m.GetSort(a)
			if err != nil {
				return RewriteFailed, err
			}
			if rhs, err = m.MkSequenceEmpty(sort); err != nil {
				return RewriteFailed, err
			}
		} else {
			es, err := seq.handles()
			if err != nil {
				return RewriteFailed, err
			}
			if rhs, err = r.mkSeqUtilConcat(es); err != nil {
				return RewriteFailed, err
			}
		}
		result, err := m.MkEq(a, rhs)
		if err != nil {
			return RewriteFailed, err
		}
		if err = m.SetRef(out, result); err != nil {
			return RewriteFailed, err
		}
		return RewriteFull, nil
	}
	if isSeq, err = r.isSequenceExpr(a, seq); err != nil || !isSeq {
		return RewriteFailed, err
	}
	g, err := aut.Graph()
	if err != nil {
		return RewriteFailed, err
	}
	trail := &seqRefVector{manager: m}
	maps := [2]seqUMap{newSeqUMap(), newSeqUMap()}
	selectMap := 0
	ch, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return RewriteFailed, err
	}
	cond, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return RewriteFailed, errors.Join(err, m.DelRef(&ch))
	}
	ors := &seqRefVector{manager: m}
	defer func() {
		err = errors.Join(err, ors.reset(), m.DelRef(&cond), m.DelRef(&ch), trail.reset())
	}()
	truth, err := m.MkTrue()
	if err != nil {
		return RewriteFailed, err
	}
	maps[0].insert(g.Initial(), truth)
	// add_next(next, trail, idx, cond)
	addNext := func(next *seqUMap, idx uint32, c ast.ASTHandle) error {
		if acc, found := next.find(idx); !m.IsTrue(c) && found {
			var err error
			if c, err = m.MkOrArray([]ast.ASTHandle{c, acc}); err != nil {
				return err
			}
		}
		if err := trail.push(c); err != nil {
			return err
		}
		next.insert(idx, c)
		return nil
	}
	for i := range seq.refs {
		frontier := &maps[selectMap]
		next := &maps[1-selectMap]
		selectMap = 1 - selectMap
		character, err := seq.get(i)
		if err != nil {
			return RewriteFailed, err
		}
		if err = m.SetRef(ch, character); err != nil {
			return RewriteFailed, err
		}
		next.reset()
		for _, entry := range frontier.entries() {
			acc := entry.value
			for _, mv := range g.ExpandedMovesFrom(entry.key, false) {
				condition, err := aut.Condition(mv.Condition)
				if err != nil {
					return RewriteFailed, err
				}
				concrete := false
				if condition.Kind == StringRegexCharacter {
					labelValue, err := m.IsValue(condition.Character)
					if err != nil {
						return RewriteFailed, err
					}
					characterValue, err := m.IsValue(character)
					if err != nil {
						return RewriteFailed, err
					}
					concrete = labelValue && characterValue
				}
				if concrete {
					if condition.Character == character {
						if err = addNext(next, mv.Destination, acc); err != nil {
							return RewriteFailed, err
						}
					}
					continue
				}
				accepted, err := aut.Accept(mv.Condition, character)
				if err != nil {
					return RewriteFailed, err
				}
				if err = m.SetRef(cond, accepted); err != nil {
					return RewriteFailed, err
				}
				if m.IsFalse(accepted) {
					continue
				}
				if m.IsTrue(accepted) {
					if err = addNext(next, mv.Destination, acc); err != nil {
						return RewriteFailed, err
					}
					continue
				}
				both, err := m.MkAndArray([]ast.ASTHandle{accepted, acc})
				if err != nil {
					return RewriteFailed, err
				}
				if err = m.SetRef(cond, both); err != nil {
					return RewriteFailed, err
				}
				if err = addNext(next, mv.Destination, both); err != nil {
					return RewriteFailed, err
				}
			}
		}
	}
	for _, entry := range maps[selectMap].entries() {
		final := false
		for _, state := range g.EpsilonClosure(entry.key) {
			if final = g.IsFinal(state); final {
				break
			}
		}
		if final {
			if err = ors.push(entry.value); err != nil {
				return RewriteFailed, err
			}
		}
	}
	disjuncts, err := ors.handles()
	if err != nil {
		return RewriteFailed, err
	}
	result, err := m.MkOrArray(disjuncts)
	if err != nil {
		return RewriteFailed, err
	}
	if err = m.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return RewriteFull, nil
}
