package sat

import (
	"io"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type dimacsModelFormula struct {
	variables uint32
	literals  []Literal
	ends      []uint32
}

// readDIMACSModelFormula uses the normal scanner with flat literal storage.
func readDIMACSModelFormula(in io.Reader, size int, limit *control.Reslimit) (dimacsModelFormula, bool) {
	var result dimacsModelFormula
	reader := &io.LimitedReader{R: in, N: int64(size) + 1}
	b := newDIMACSStreamBuffer(reader)
	defer b.syncShared()
	result.literals = make([]Literal, 0, size/4)
	result.ends = make([]uint32, 0, size/20)
	for b.nextClause() {
		if len(result.ends)%128 == 0 && limit.CancelFlag() {
			return dimacsModelFormula{}, false
		}
		start := len(result.literals)
		for {
			lit, err := b.parseInt()
			if err != nil || lit < -dimacsModelMaxVariable || lit > dimacsModelMaxVariable {
				return dimacsModelFormula{}, false
			}
			if lit == 0 {
				break
			}
			sign := uint32(0)
			if lit < 0 {
				lit = -lit
				sign = 1
			}
			result.variables = max(result.variables, uint32(lit))
			result.literals = append(result.literals, Literal(uint32(lit)*2+sign))
		}
		if len(result.literals) == start {
			return dimacsModelFormula{}, false
		}
		result.ends = append(result.ends, uint32(len(result.literals)))
	}
	return result, reader.N == 1 && result.variables != 0
}

type dimacsModelRepair struct {
	cnf           dimacsModelFormula
	value         []uint32 // true literal for each variable
	count         []uint32
	unsatisfied   []uint32
	positions     []uint32 // index of each unsatisfied clause in unsatisfied
	offsets       []uint32 // literal to range in occurrenceIDs
	occurrenceIDs []uint32
	rng           uint32
}

func (r *dimacsModelRepair) random(n int) int {
	r.rng ^= r.rng << 13
	r.rng ^= r.rng >> 17
	r.rng ^= r.rng << 5
	return int(r.rng % uint32(n))
}

func (r *dimacsModelRepair) occurrence(word uint32) []uint32 {
	return r.occurrenceIDs[r.offsets[word]:r.offsets[word+1]]
}

func (r *dimacsModelRepair) bump(clause uint32, delta int32) {
	old := r.count[clause]
	// Each occurrence being removed currently contributes to old.
	now := old + uint32(delta)
	r.count[clause] = now
	if old == 0 {
		at := r.positions[clause]
		last := r.unsatisfied[len(r.unsatisfied)-1]
		r.unsatisfied[at] = last
		r.positions[last] = at
		r.unsatisfied = r.unsatisfied[:len(r.unsatisfied)-1]
	} else if now == 0 {
		r.positions[clause] = uint32(len(r.unsatisfied))
		r.unsatisfied = append(r.unsatisfied, clause)
	}
}

func (r *dimacsModelRepair) clause(id uint32) []Literal {
	start := uint32(0)
	if id > 0 {
		start = r.cnf.ends[id-1]
	}
	return r.cnf.literals[start:r.cnf.ends[id]]
}

func newDIMACSModelRepair(cnf dimacsModelFormula) dimacsModelRepair {
	n := int(cnf.variables) + 1
	r := dimacsModelRepair{
		cnf:       cnf,
		value:     make([]uint32, n),
		count:     make([]uint32, len(cnf.ends)),
		positions: make([]uint32, len(cnf.ends)),
		offsets:   make([]uint32, 2*n+1),
		rng:       0x53721aef,
	}
	for i := range r.value {
		r.value[i] = uint32(i)*2 + 1
	}
	for _, lit := range cnf.literals {
		r.offsets[lit+1]++
	}
	for i := 1; i < len(r.offsets); i++ {
		r.offsets[i] += r.offsets[i-1]
	}
	r.occurrenceIDs = make([]uint32, len(cnf.literals))
	next := append([]uint32(nil), r.offsets...)
	for i := range cnf.ends {
		ci := uint32(i)
		c := r.clause(ci)
		t := uint32(0)
		for _, lit := range c {
			r.occurrenceIDs[next[lit]] = ci
			next[lit]++
			t += uint32(lit & 1)
		}
		r.count[i] = t
		if t == 0 {
			r.positions[i] = uint32(len(r.unsatisfied))
			r.unsatisfied = append(r.unsatisfied, ci)
		}
	}
	return r
}

func (r *dimacsModelRepair) validate() bool {
	for i := range r.cnf.ends {
		satisfied := false
		for _, l := range r.clause(uint32(i)) {
			if r.value[l>>1] == uint32(l) {
				satisfied = true
				break
			}
		}
		if !satisfied {
			return false
		}
	}
	return true
}

// Dense three-literal formulas can sit near a satisfying assignment for
// many flips. Give this small, uniform-width workload a longer trial.
func (cnf dimacsModelFormula) denseTernary() bool {
	n, clauses := int(cnf.variables), len(cnf.ends)
	if n < 128 || n > 4096 || 2*clauses < 7*n || 2*clauses > 9*n || len(cnf.literals) != 3*clauses {
		return false
	}
	for i, end := range cnf.ends {
		if end != 3*uint32(i+1) {
			return false
		}
	}
	return true
}

func repairDIMACSModel(cnf dimacsModelFormula, limitState *control.Reslimit) []uint32 {
	r := newDIMACSModelRepair(cnf)
	// Bound both the number of flips and the number of occurrence entries
	// examined. A high-degree variable cannot exhaust the search budget in
	// the middle of a flip.
	maxFlips := 16 * len(r.value)
	work := min(25*len(cnf.literals), 2_000_000)
	patience := min(2048, max(256, len(r.value)))
	dense := cnf.denseTernary()
	if dense {
		maxFlips = min(170*len(r.value), 120_000)
		work = min(500*len(cnf.literals), 2_000_000)
		patience = min(10_000, 35*len(r.value))
	}
	bestRemaining, lastImprovement := len(r.unsatisfied), 0
	for flips := 0; len(r.unsatisfied) != 0 && flips < maxFlips && work > 0; flips++ {
		if flips%64 == 0 && limitState.CancelFlag() {
			return nil
		}
		if remaining := len(r.unsatisfied); remaining < bestRemaining {
			bestRemaining, lastImprovement = remaining, flips
		} else if flips-lastImprovement >= patience {
			break
		}
		row := r.clause(r.unsatisfied[r.random(len(r.unsatisfied))])
		// Prefer a flip that leaves more currently satisfied clauses intact.
		best, score := uint32(0), int(^uint(0)>>1)
		var weights [3]int
		total := 0
		for i, lit := range row {
			live := r.value[lit>>1]
			breaks := 0
			occurrences := r.occurrence(live)
			cost := len(occurrences) + 1
			if cost > work {
				return nil
			}
			work -= cost
			for _, ci := range occurrences {
				if r.count[ci] == 1 {
					breaks++
				}
			}
			if dense {
				// Keep every flip possible, with a stronger preference for
				// small break counts than the ordinary uniform random step.
				weights[i] = 4096/(breaks*breaks+1) + 1
				total += weights[i]
			}
			if breaks < score {
				best, score = live, breaks
				if breaks == 0 && !dense {
					break
				}
			}
		}
		if dense {
			choice := r.random(total)
			for i, weight := range weights {
				if choice < weight {
					best = r.value[row[i]>>1]
					break
				}
				choice -= weight
			}
		} else if score > 0 && r.random(3) == 0 {
			best = r.value[row[r.random(len(row))]>>1]
		}
		cost := len(r.occurrence(best)) + len(r.occurrence(best^1)) + 1
		if cost > work {
			return nil
		}
		work -= cost
		for _, ci := range r.occurrence(best) {
			r.bump(ci, -1)
		}
		for _, ci := range r.occurrence(best ^ 1) {
			r.bump(ci, 1)
		}
		r.value[best>>1] ^= 1
	}
	// Check the assignment against every original clause independently of
	// the counters used to guide the search.
	if len(r.unsatisfied) == 0 && r.validate() {
		return r.value
	}
	return nil
}

// PrepareDIMACSFile tries a model for a fresh formula of the given byte size.
// If the complete scan succeeds without a model, it loads those same clauses
// for ordinary search. An inconclusive scan leaves ingestion to the caller.
func (s *Solver) PrepareDIMACSFile(in io.Reader, size int) (model Model, loaded bool, err error) {
	if size <= 0 || size > 4<<20 || !s.canProbeDIMACSModel(in) {
		return nil, false, nil
	}
	cnf, ok := readDIMACSModelFormula(in, size, s.Limit)
	if !ok || s.Limit.CancelFlag() {
		return nil, false, nil
	}
	words := repairDIMACSModel(cnf, s.Limit)
	if s.Limit.CancelFlag() {
		return nil, false, nil
	}
	if words == nil {
		return nil, true, cnf.load(s)
	}
	model = make(Model, len(words))
	for i, word := range words {
		model[i] = lbool.FromBool(word&1 == 0)
	}
	return model, false, nil
}

func (cnf dimacsModelFormula) load(s *Solver) error {
	var start, variables uint32
	for _, end := range cnf.ends {
		clause := cnf.literals[start:end]
		for _, lit := range clause {
			for uint32(lit.variable()) >= variables {
				if _, err := s.MkVar(false, true); err != nil {
					return err
				}
				variables++
			}
		}
		if err := s.MkClause(clause); err != nil {
			return err
		}
		start = end
	}
	return nil
}
