package order

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// BooleanProblem encodes the retained assertions on two-element domains.
// Its clauses use signed literals. Before reduction, literal 1 is forced true.
type BooleanProblem struct {
	Vars    int
	Clauses []BooleanClause
	Work    uint32
}

// The builder and its truth tables die before the SAT solver is opened.
type bitCompiler struct {
	BooleanProblem
	ceiling, nextPoll int
	nodes             []bitGate
	roots             []bitRoot
	firstGate         int32
	work              int
	m                 *ast.ASTManager
	gates             map[uint64]int32
	values            []int32
	firstVariable     []int32
}
type bitRoot struct {
	after int
	lit   int32
}

// An AND uses two signed literals. An ITE always has a nonzero third literal.
type bitGate struct{ a, b, c int32 }

// Truth-table inputs and generated gates each have this many available slots.
const bitCompilerLimit = 100_000

var errBitLimit = errors.New("one-bit expansion limit")

// ProofProblem records either a direct refutation or the input and remaining
// step budget for an independent solver.
type ProofProblem struct {
	Order       Problem
	Boolean     *BooleanProblem
	Conditional *ConditionalGoal
	Refuted     bool
	SearchSteps uint32
}

// ForProof checks universal instances before constructing a Boolean or order
// problem. Only a refutation establishes a result for the original assertions.
func ForProof(m *ast.ASTManager, assertions []ast.ASTHandle, steps uint32) (ProofProblem, bool, error) {
	var p TransitionPremises
	return p.ForProof(m, assertions, steps)
}

// ForProof reuses the admission check across goals with the same premises.
func (p *TransitionPremises) ForProof(m *ast.ASTManager, assertions []ast.ASTHandle, steps uint32) (ProofProblem, bool, error) {
	var t inference
	if ok, err := t.infer(m, assertions, p); err != nil || !ok {
		return ProofProblem{}, false, err
	}
	refuted, spent := t.matchFinalAssertion(steps)
	if m.ResourceLimit().CancelFlag() {
		return ProofProblem{}, false, nil
	}
	if refuted {
		return ProofProblem{Refuted: true}, true, nil
	}
	if spent >= steps {
		return ProofProblem{}, false, nil
	}
	remaining := steps - spent
	bits, err := t.booleanProblem(remaining)
	if err != nil || m.ResourceLimit().CancelFlag() {
		return ProofProblem{}, false, err
	}
	if bits != nil {
		return ProofProblem{Boolean: bits, SearchSteps: remaining}, true, nil
	}
	spent = t.addInductionInstance(remaining)
	if spent >= remaining || m.ResourceLimit().CancelFlag() {
		return ProofProblem{}, false, nil
	}
	if t.unbounded && t.transition == nil {
		return ProofProblem{}, false, nil
	}
	remaining -= spent
	proof := ProofProblem{Order: t.renderProof(&remaining)}
	if c := t.transition; c != nil {
		primary := proof.Order
		if c.primaryPremises == nil {
			primary.Text = primary.Text[:primary.LastAssertionStart] + t.renderAssertion(c.specialized)
		} else {
			t.assertions = append(c.primaryPremises, c.specialized)
			primary = t.renderProof(&remaining)
		}
		t.assertions = append(c.premises, c.negatedFacts)
		proof.Conditional = &ConditionalGoal{Condition: t.renderProof(&remaining), Primary: primary}
	}
	proof.SearchSteps = remaining
	return proof, true, nil
}

func (t *inference) booleanProblem(steps uint32) (*BooleanProblem, error) {
	for _, w := range t.widths {
		if w != 1 {
			return nil, nil
		}
	}
	ceiling := int(min(steps, 500_000))
	k := &bitCompiler{
		BooleanProblem: BooleanProblem{Vars: 1},
		m:              t.m,
		ceiling:        ceiling,
		roots:          make([]bitRoot, 0, min(len(t.assertions), ceiling, 8192)),
		values:         make([]int32, len(t.declarations)),
		firstVariable:  make([]int32, len(t.signatures)),
	}

	for i, s := range t.signatures {
		if len(s.inputs) > 10 {
			return nil, nil
		}
		rows := 1 << uint(len(s.inputs))
		if k.Vars+rows > bitCompilerLimit {
			return nil, nil
		}
		if e := k.charge(rows); e != nil {
			return nil, nil
		}
		k.firstVariable[i] = int32(k.Vars + 1)
		k.Vars += rows
	}
	// Limit the starting capacity on repeated assertions and small budgets.
	// Both tables grow if the expanded circuit needs more gates.
	size := min(8192, len(t.terms), k.ceiling/2, 12*k.Vars)
	k.gates = make(map[uint64]int32, size)
	k.nodes = make([]bitGate, 0, size)
	k.firstGate = int32(k.Vars + 1)
	for _, id := range t.assertions {
		lit, err := k.eval(t, id)
		if err == errBitLimit {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if lit != 1 {
			k.roots = append(k.roots, bitRoot{len(k.nodes), lit})
		}
	}
	if e := k.charge(len(k.roots) + 2*len(k.nodes)); e != nil {
		return nil, nil
	}
	if !k.emitGates() {
		return nil, nil
	}
	return &BooleanProblem{Vars: k.Vars, Clauses: k.Clauses, Work: uint32(k.work)}, nil
}

func (k *bitCompiler) charge(n int) error {
	k.work += n
	if k.work > k.ceiling {
		return errBitLimit
	}
	if k.work >= k.nextPoll {
		if k.m.ResourceLimit().CancelFlag() {
			return errBitLimit
		}
		k.nextPoll = k.work + 256
	}
	return nil
}

// Each signed literal fits in 21 bits: input tables contribute at most
// bitCompilerLimit variables and gate construction adds at most that many more.
// The zero third operand distinguishes a conjunction from a conditional.
func (k *bitCompiler) intern(g bitGate) int32 {
	const offset = 1 << 20
	key := uint64(g.a+offset) | uint64(g.b+offset)<<21 | uint64(g.c+offset)<<42
	if x := k.gates[key]; x != 0 {
		return x
	}
	if len(k.nodes) == bitCompilerLimit {
		// No result can escape an exhausted budget. booleanProblem calls
		// charge once more before emitting any clauses.
		k.work = k.ceiling + 1
		return 1
	}
	k.Vars++
	x := int32(k.Vars)
	k.gates[key] = x
	k.nodes = append(k.nodes, g)
	return x
}

// Interning AND and ITE gates gives every occurrence of the same Boolean
// operation the same SAT variable. Negation is the sign of the literal.
func (k *bitCompiler) and(a, b int32) int32 {
	if a == 1 || a == b {
		return b
	}
	if b == 1 {
		return a
	}
	if a == -1 || b == -1 || a == -b {
		return -1
	}
	if a > b {
		a, b = b, a
	}
	return k.intern(bitGate{a: a, b: b})
}

func (k *bitCompiler) ite(c, a, b int32) int32 {
	if c == 1 || a == b {
		return a
	}
	if c == -1 {
		return b
	}
	if a == 1 {
		return -k.and(-c, -b)
	}
	if a == -1 {
		return k.and(-c, b)
	}
	if b == 1 {
		return -k.and(c, -a)
	}
	if b == -1 {
		return k.and(c, a)
	}
	if c < 0 {
		a, b = b, a
		c = -c
	}
	neg := int32(1)
	if a < 0 {
		a, b = -a, -b
		neg = -1
	}
	return neg * k.intern(bitGate{a: c, b: a, c: b})
}

// Each function's truth table occupies consecutive SAT variables. A known
// argument chooses a half by offset. A symbolic argument needs an ITE gate.
func (k *bitCompiler) lookup(args []int32, first int32) int32 {
	for len(args) > 0 {
		half := int32(1) << uint(len(args)-1)
		selector := args[0]
		args = args[1:]
		switch selector {
		case 1:
			first += half
		case -1:
		default:
			return k.ite(selector, k.lookup(args, first+half), k.lookup(args, first))
		}
	}
	return first
}

func (k *bitCompiler) eval(t *inference, id int32) (int32, error) {
	if e := k.charge(1); e != nil {
		return 0, e
	}
	n := &t.terms[id]
	switch n.op {
	case opTrue:
		return 1, nil
	case opFalse:
		return -1, nil
	case opVariable:
		return k.values[n.value], nil
	case opNumeral:
		if n.value == 1 {
			return 1, nil
		}
		return -1, nil
	case opForall, opExists:
		// Each syntactic binder has its own slot, including shadowed names.
		binders := t.declarations[n.start : n.start+n.count]
		if len(binders) > 12 {
			return 0, errBitLimit
		}
		all := n.op == opForall
		res := int32(1)
		for mask := 0; mask < 1<<uint(len(binders)); mask++ {
			for j, b := range binders {
				v := int32(-1)
				if mask&(1<<uint(j)) != 0 {
					v = 1
				}
				k.values[b.name] = v
			}
			r, e := k.eval(t, n.value)
			if e != nil {
				return 0, e
			}
			if !all {
				r = -r
			}
			res = k.and(res, r)
			if res == -1 {
				break
			}
		}
		if !all {
			res = -res
		}
		return res, nil
	}
	var small [8]int32
	args := small[:0]
	for _, a := range t.arguments(n) {
		v, e := k.eval(t, a)
		if e != nil {
			return 0, e
		}
		args = append(args, v)
	}
	switch n.op {
	case opFunction:
		if e := k.charge(1 << uint(len(args))); e != nil {
			return 0, e
		}
		return k.lookup(args, k.firstVariable[n.value]), nil
	case opNot:
		return -args[0], nil
	case opAnd, opOr:
		or := n.op == opOr
		r := int32(1)
		for _, a := range args {
			if or {
				a = -a
			}
			r = k.and(r, a)
		}
		if or {
			r = -r
		}
		return r, nil
	case opXor:
		return k.ite(args[0], -args[1], args[1]), nil
	case opEqual, opDistinct:
		if n.op == opDistinct && len(args) > 2 {
			return -1, nil
		}
		r := int32(1)
		for _, a := range args[1:] {
			r = k.and(r, k.ite(args[0], a, -a))
		}
		if n.op == opDistinct {
			r = -r
		}
		return r, nil
	case opIte:
		return k.ite(args[0], args[1], args[2]), nil
	case opImplies:
		return -k.and(args[0], -args[1]), nil
	case opOrder:
		a, b := args[0], args[1]
		switch n.value {
		case bvULT:
			return k.and(-a, b), nil
		case bvUGT:
			return k.and(a, -b), nil
		case bvULE:
			return -k.and(a, -b), nil
		case bvUGE:
			return -k.and(-a, b), nil
		}
	}
	return 0, inapplicable
}

// A positive occurrence of a gate needs only the forward implication.
// Mark dependencies in reverse construction order before writing clauses.
func (k *bitCompiler) emitGates() bool {
	// Count the constant, roots and polarity-dependent clauses in this pass.
	rows := 1 + len(k.roots)
	need := make([]uint8, k.Vars+1)
	mark := func(x int32) {
		if x < 0 {
			need[-x] |= 2
		} else {
			need[x] |= 1
		}
	}
	for i, root := range k.roots {
		if i%512 == 0 && k.m.ResourceLimit().CancelFlag() {
			return false
		}
		mark(root.lit)
	}
	for i := len(k.nodes) - 1; i >= 0; i-- {
		if i%512 == 0 && k.m.ResourceLimit().CancelFlag() {
			return false
		}
		g := k.nodes[i]
		x := k.firstGate + int32(i)
		flags := need[x]
		if flags == 0 {
			continue
		}
		a, b := g.a, g.b
		if g.c != 0 {
			mark(a)
			mark(-a)
			a, b = g.b, g.c
		}
		if flags&1 != 0 {
			rows += 2
			mark(a)
			mark(b)
		}
		if flags&2 != 0 {
			rows++
			if g.c != 0 {
				rows++
			}
			mark(-a)
			mark(-b)
		}
	}
	k.Clauses = make([]BooleanClause, 1, rows)
	k.Clauses[0] = BooleanClause{1}
	next := 0
	for i := 0; i <= len(k.nodes); i++ {
		if i%512 == 0 && k.m.ResourceLimit().CancelFlag() {
			return false
		}
		for next < len(k.roots) && k.roots[next].after == i {
			lit := k.roots[next].lit
			k.emit(lit, 0, 0)
			next++
			if next%512 == 0 && k.m.ResourceLimit().CancelFlag() {
				return false
			}
		}
		if i == len(k.nodes) {
			break
		}
		g := k.nodes[i]
		x := k.firstGate + int32(i)
		flags := need[x]
		if g.c == 0 {
			if flags&1 != 0 {
				k.emit(-x, g.a, 0)
				k.emit(-x, g.b, 0)
			}
			if flags&2 != 0 {
				k.emit(x, -g.a, -g.b)
			}
		} else {
			if flags&1 != 0 {
				k.emit(-g.a, -x, g.b)
			}
			if flags&2 != 0 {
				k.emit(-g.a, x, -g.b)
			}
			if flags&1 != 0 {
				k.emit(g.a, -x, g.c)
			}
			if flags&2 != 0 {
				k.emit(g.a, x, -g.c)
			}
		}
	}
	return true
}

// BooleanClause contains one to three signed, one-based variable IDs.
// Zero terminates short clauses. An all-zero row is the empty clause.
type BooleanClause [3]int32

func (c *BooleanClause) literals() []int32 {
	if c[2] != 0 {
		return c[:3]
	}
	if c[1] != 0 {
		return c[:2]
	}
	if c[0] != 0 {
		return c[:1]
	}
	return nil
}

func (k *bitCompiler) emit(a, b, c int32) { k.Clauses = append(k.Clauses, BooleanClause{a, b, c}) }
