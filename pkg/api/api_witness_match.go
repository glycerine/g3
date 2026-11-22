package api

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

// A flat application supplies an entire tuple when every binder occurs as a
// direct argument. Nested quantifiers contribute patterns after substitution.
func (s *witnessClauseSet) preparePatterns(q *witnessAtom) {
	todo := []ast.ASTHandle{q.key.body}
	seen := make(map[ast.ASTHandle]bool)
	var only ast.ASTHandle
	many := false
	for len(todo) > 0 && s.charge(1) {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		hd, e := s.m.NodeHeader(h)
		if e != nil {
			s.err = e
			return
		}
		if hd.Kind != ast.ASTApp {
			continue
		}
		if hd.Family == ast.NullFamilyID && hd.NumArgs != 0 {
			if only.IsNull() {
				only = hd.Decl
			} else if only != hd.Decl {
				many = true
			}
		}
		has, e := s.m.HasQuantifiers(h)
		if e != nil {
			s.err = e
			return
		}
		g, e := s.m.IsGround(h)
		if e != nil {
			s.err = e
			return
		}
		if g && !has {
			s.observe(h, q.goal)
			continue
		}
		pat := witnessPattern{decl: hd.Decl}
		seenVars := 0
		for i := uint32(0); i < hd.NumArgs; i++ {
			a, e := s.m.GetArg(h, i)
			if e != nil {
				s.err = e
				return
			}
			todo = append(todo, a)
			if hd.Family != ast.NullFamilyID || hd.NumArgs > 16 {
				continue
			}
			k, e := s.m.GetKind(a)
			if e != nil {
				s.err = e
				return
			}
			if k != ast.ASTVar {
				continue
			}
			j, e := s.m.GetVarIndex(a)
			if e != nil {
				s.err = e
				return
			}
			if int(j) >= q.key.n {
				continue
			}
			p := q.key.n - 1 - int(j)
			if pat.positions[p] == 0 {
				pat.positions[p] = uint8(i) + 1
				seenVars++
			}
		}
		if seenVars == q.key.n {
			found := false
			for _, k := range q.patterns {
				if k == pat {
					found = true
					break
				}
			}
			if !found {
				q.patterns = append(q.patterns, pat)
			}
		}
	}
	if many || only.IsNull() || q.key.n < 2 || q.key.n > 3 {
		return
	}
	for _, st := range q.key.sorts[1:q.key.n] {
		if st != q.key.sorts[0] {
			return
		}
	}
	ar, e := s.m.GetArity(only)
	if e != nil {
		s.err = e
		return
	}
	if ar == 2 {
		d0, e := s.m.GetDomain(only, 0)
		if e != nil {
			s.err = e
			return
		}
		d1, e := s.m.GetDomain(only, 1)
		if e != nil {
			s.err = e
			return
		}
		res, e := s.m.GetRange(only)
		if e != nil {
			s.err = e
			return
		}
		bs, e := s.m.MkBoolSort()
		if e != nil {
			s.err = e
			return
		}
		q.singleBinaryRelation = d0 == q.key.sorts[0] && d1 == d0 && res == bs
	}
}
func (s *witnessClauseSet) generate(width, rounds int) {
	prepared := 0
	s.generating = true
	for round := 0; round < rounds && s.err == nil; round++ {
		for ; prepared < len(s.atoms); prepared++ {
			s.preparePatterns(s.atoms[prepared])
			if s.err != nil {
				return
			}
		}
		pool := s.termPool(width)
		// Publish each round's terms together so rule order cannot starve later rules.
		var batch []witnessMatch
		c := witnessRanking{positions: make(map[witnessTuple]int)}
		for _, q := range s.atoms {
			if !s.shouldInstantiate(q) {
				continue
			}
			tuples := s.candidates(q, pool, width, &c)
			if s.err != nil {
				return
			}
			for _, t := range tuples[:min(len(tuples), maxWitnessInstances-s.instances-len(batch))] {
				batch = append(batch, witnessMatch{q, t.tuple})
			}
			if len(batch)+s.instances >= maxWitnessInstances {
				break
			}
		}
		if len(batch) == 0 {
			break
		}
		for _, p := range batch {
			s.instantiate(p.q, p.v)
			if s.err != nil {
				return
			}
		}
	}
}

func (s *witnessClauseSet) termPool(width int) map[ast.ASTHandle][]ast.ASTHandle {
	pool := make(map[ast.ASTHandle][]ast.ASTHandle)
	for st, vs := range s.terms {
		list := append([]ast.ASTHandle(nil), vs...)
		sort.SliceStable(list, func(i, j int) bool {
			a, b := s.value(list[i].Slot()), s.value(list[j].Slot())
			if a.flags&witnessGoalTerm != b.flags&witnessGoalTerm {
				return a.flags&witnessGoalTerm != 0
			}
			if a.flags&witnessFresh != b.flags&witnessFresh {
				return a.flags&witnessFresh != 0
			}
			return a.occurrences > b.occurrences
		})
		pool[st] = list[:min(width, len(list))]
	}
	return pool
}

func (s *witnessClauseSet) shouldInstantiate(q *witnessAtom) bool {
	pol := uint8(witnessPositive)
	if !q.key.universal {
		pol = witnessNegative
	}
	if q.used&pol == 0 {
		return false
	}
	for _, st := range q.key.sorts[:q.key.n] {
		if s.goalSorts[st] {
			return true
		}
	}
	return false
}

const maxWitnessInstances = 12_000

type witnessMatch struct {
	q *witnessAtom
	v witnessTuple
}

// Stable ranking preserves discovery order when candidates receive equal scores.
type witnessRanking struct {
	list      []witnessCandidate
	positions map[witnessTuple]int
}

func (s *witnessClauseSet) candidates(q *witnessAtom, pool map[ast.ASTHandle][]ast.ASTHandle, width int, c *witnessRanking) []witnessCandidate {
	list := c.list[:0]
	positions := c.positions
	clear(positions)
	maxrows := width
	add := func(t witnessTuple, priority uint32) {
		if q.tuples[t] {
			return
		}
		if ix := positions[t]; ix != 0 {
			if score := priority << 24; score > list[ix-1].score {
				list[ix-1].score = list[ix-1].score&0xffffff | score
			}
			return
		}
		// The high byte orders candidate sources. With at most eight binders the
		// goal, freshness and frequency contributions fit below it.
		var score uint32 = priority << 24
		for _, part := range t.args[:q.key.n] {
			v := s.value(part)
			if v.flags&witnessGoalTerm != 0 {
				score += 1 << 20
			}
			if v.flags&witnessDerived != 0 {
				score += 1 << 16
			} else if v.flags&witnessFresh != 0 {
				score += 1 << 12
			}
			score += uint32(min(100, v.occurrences))
		}
		list = append(list, witnessCandidate{t, score})
		positions[t] = len(list)
	}
	if q.key.n == 1 {
		for _, h := range pool[q.key.sorts[0]] {
			var t witnessTuple
			t.args[0] = h.Slot()
			add(t, 2)
		}
	}
	for _, p := range q.patterns {
		for _, as := range s.apps[p.decl] {
			if !s.charge(1) {
				return nil
			}
			var t witnessTuple
			for j, pos := range p.positions[:q.key.n] {
				t.args[j] = as[int(pos)-1].Slot()
			}
			add(t, 0)
			if len(list) > 500 {
				break
			}
		}
	}
	// No single binary application binds all three variables of a relation
	// axiom such as transitivity. Instantiate the original body at ordered
	// triples. Its Boolean guard and the relation's meaning stay in the formula.
	if q.singleBinaryRelation && q.key.n == 3 {
		maxrows = 300
		terms := pool[q.key.sorts[0]]
		for _, a := range terms {
			if !s.charge(1) {
				return nil
			}
			for _, b := range terms {
				if a == b {
					continue
				}
				for _, c := range terms {
					if c == a || c == b {
						continue
					}
					var t witnessTuple
					t.args[0], t.args[1], t.args[2] = a.Slot(), b.Slot(), c.Slot()
					add(t, 2)
				}
			}
		}
	}
	if q.singleBinaryRelation && q.key.n == 2 {
		maxrows = 180
		for _, a := range pool[q.key.sorts[0]] {
			for _, b := range pool[q.key.sorts[0]] {
				if a == b || s.value(b.Slot()).flags&witnessGoalTerm == 0 {
					continue
				}
				var t witnessTuple
				t.args[0], t.args[1] = a.Slot(), b.Slot()
				add(t, 3)
				t.args[0], t.args[1] = t.args[1], t.args[0]
				add(t, 3)
			}
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].score > list[j].score })
	c.list = list
	return list[:min(maxrows, len(list))]
}
