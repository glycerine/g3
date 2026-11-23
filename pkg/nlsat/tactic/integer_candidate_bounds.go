package tactic

import (
	"math"
	"math/bits"

	"github.com/glycerine/g3/pkg/ast"
)

// Bounds suggest a concrete integer for each free arithmetic constant.
// A rejected bound or assignment never proves the goal unsatisfiable.
type candidateVariable struct {
	declaration    ast.ASTHandle
	numeral        candidateNumeral
	low, high      int64
	minAbs, maxAbs int64
	successors     []candidateSuccessor
	predecessors   int
}

type candidateSuccessor struct {
	variable int
	gap      int64
}

type candidateTermKind uint8

const (
	candidateOther candidateTermKind = iota
	candidateInteger
	candidateConstant
	candidateSquare
)

type candidateTerm struct {
	index   int32  // small numeral value or variable index
	bits    uint16 // bounded by maxCandidateIntegerBits before publication
	kind    candidateTermKind
	depends bool
}

type integerCandidate struct {
	comparisons                             []candidateComparison
	manager                                 *ast.ASTManager
	arithmetic                              ast.FamilyID
	terms                                   map[ast.ASTHandle]candidateTerm
	variables                               []candidateVariable
	work                                    int
	largest                                 int64
	nonlinear, mixedProduct, symbolicSquare bool
	ordered, assigned                       bool
}

// Arithmetic declaration kinds from src/ast/arith_decl_plugin.h.
const candidateNumeralOp int32 = 0

// Equality belongs to the basic family; use a distinct operation in a comparison.
const candidateEqual int32 = -1
const (
	candidateLE int32 = 2 + iota
	candidateGE
	candidateLT
	candidateGT
	candidateAdd
	candidateSub
	candidateNeg
	candidateMul
)
const candidateToReal int32 = 14
const candidatePower int32 = 18

func (p *integerCandidate) scan(h ast.ASTHandle, depth int) (candidateTerm, bool, error) {
	if term, ok := p.terms[h]; ok {
		return term, true, nil
	}
	var term candidateTerm
	m := p.manager
	if depth > maxCandidateDepth || p.work >= maxCandidateEdges || m.Limit.CancelFlag() {
		return term, false, nil
	}
	hd, err := m.NodeHeader(h)
	if err != nil || hd.Kind != ast.ASTApp {
		return term, false, err
	}
	switch hd.Family {
	case ast.NullFamilyID:
		if hd.NumArgs != 0 {
			return term, false, nil
		}
		sort, err := m.GetSort(h)
		if err != nil {
			return term, false, err
		}
		f, err := m.GetFamilyID(sort)
		if err != nil || f != p.arithmetic {
			return term, false, err
		}
		kind, err := m.GetDeclKind(sort)
		if err != nil || kind < 0 || kind > 1 {
			return term, false, err
		}
		// Input bounds fit in int32 and any order chain has at most maxCandidateEdges unit steps.
		term = candidateTerm{kind: candidateConstant, index: int32(len(p.variables)), depends: true, bits: 33}
		p.variables = append(p.variables, candidateVariable{declaration: hd.Decl, numeral: candidateNumeral{integer: kind == 1}, low: math.MinInt64, high: math.MaxInt64, maxAbs: math.MaxInt64})
	case p.arithmetic:
		switch hd.DeclKind {
		case candidateNumeralOp:
			x, ok, err := m.ArithmeticIntegralInt32(h)
			if err != nil {
				return term, false, err
			}
			if !ok {
				return term, false, nil
			}
			term = candidateTerm{kind: candidateInteger, index: x, bits: uint16(bits.Len64(uint64(max(int64(x), -int64(x)))) + 1)}
			p.largest = max(p.largest, int64(x), -int64(x))
		case candidatePower:
			if hd.NumArgs != 2 {
				return term, false, nil
			}
		case candidateLE, candidateGE, candidateLT, candidateGT, candidateAdd, candidateSub, candidateNeg, candidateMul, candidateToReal:
		default:
			return term, false, nil
		}
	case m.BasicFamilyID():
		if hd.DeclKind < ast.ASTOpTrue || hd.DeclKind > ast.ASTOpImplies {
			return term, false, nil
		}
	default:
		return term, false, nil
	}
	if uint64(hd.NumArgs)+1 > uint64(maxCandidateEdges-p.work) {
		return term, false, nil
	}
	p.work += 1 + int(hd.NumArgs)
	var first, last candidateTerm
	largestBits, totalBits := 0, 0
	for i := uint32(0); i < hd.NumArgs; i++ {
		a, err := m.GetArg(h, i)
		if err != nil {
			return term, false, err
		}
		child, ok, err := p.scan(a, depth+1)
		if err != nil || !ok {
			return term, false, err
		}
		totalBits += int(child.bits)
		largestBits = max(largestBits, int(child.bits))
		if i == 0 {
			first = child
		}
		if hd.Family == p.arithmetic && hd.DeclKind == candidateMul && term.depends && child.depends {
			p.nonlinear = true
			if hd.NumArgs == 2 && first.kind == candidateConstant && child.kind == candidateConstant {
				if first == child {
					term.kind, term.index = candidateSquare, first.index
				} else {
					p.mixedProduct = true
				}
			}
		}
		term.depends = term.depends || child.depends
		last = child
	}
	if hd.NumArgs > 0 {
		width := 1
		switch {
		case hd.Family == p.arithmetic && hd.DeclKind == candidateMul:
			width = totalBits
		case hd.Family == p.arithmetic && hd.DeclKind == candidatePower:
			// Positive integral powers have an exact value on every candidate.
			// Bound the exponent before multiplying widths, including on 32-bit hosts.
			if last.kind != candidateInteger || last.index < 1 || last.index > maxCandidateIntegerBits {
				return term, false, nil
			}
			width = int(first.bits) * int(last.index)
			if last.index == 1 {
				term = first
			} else if first.depends {
				p.nonlinear = true
				if last.index == 2 && first.kind == candidateConstant {
					term.kind, term.index = candidateSquare, first.index
				}
			}
		case hd.Family == p.arithmetic && (hd.DeclKind == candidateAdd || hd.DeclKind == candidateSub):
			width = largestBits + bits.Len32(hd.NumArgs)
		case hd.Family == p.arithmetic && hd.DeclKind == candidateToReal && hd.NumArgs == 1:
			// Coercing an integer leaves its value unchanged in a comparison.
			term = first
			width = largestBits
		case hd.Family == p.arithmetic && hd.DeclKind == candidateNeg:
			// SMT-LIB spells negative numerals as unary minus applications.
			if hd.NumArgs == 1 && first.kind == candidateInteger && first.index != math.MinInt32 {
				term.kind, term.index = candidateInteger, -first.index
			}
			width = largestBits
		case hd.Family == m.BasicFamilyID() && hd.DeclKind == ast.ASTOpIte:
			width = largestBits
		}
		// Even a small shared DAG can square a value exponentially many times.
		// Bound the exact evaluator's intermediate numeral sizes before trying it.
		if width > maxCandidateIntegerBits {
			return term, false, nil
		}
		term.bits = uint16(width)
	}
	p.terms[h] = term
	return term, true, nil
}

// Read only unconditional top-level facts as bounds. All other operations
// contribute solely to validation of the eventual model.
func (p *integerCandidate) constrain(form ast.ASTHandle, comparison *candidateComparison) error {
	m := p.manager
	hd, err := m.NodeHeader(form)
	if err != nil || hd.NumArgs != 2 {
		return err
	}
	equality := hd.Family == m.BasicFamilyID() && hd.DeclKind == ast.ASTOpEq
	if !equality && (hd.Family != p.arithmetic || hd.DeclKind < candidateLE || hd.DeclKind > candidateGT) {
		return nil
	}
	l, err := m.GetArg(form, 0)
	if err != nil {
		return err
	}
	r, err := m.GetArg(form, 1)
	if err != nil {
		return err
	}
	x, y := p.terms[l], p.terms[r]
	if x.kind != candidateOther && y.kind != candidateOther {
		op := hd.DeclKind
		if equality {
			op = candidateEqual
			if x.kind == candidateSquare && y.kind == candidateConstant || y.kind == candidateSquare && x.kind == candidateConstant {
				p.symbolicSquare = true
			}
		}
		*comparison = candidateComparison{left: x, right: y, operation: op}
	}
	greater := hd.DeclKind == candidateGE || hd.DeclKind == candidateGT
	var gap int64
	if !equality && (hd.DeclKind == candidateLT || hd.DeclKind == candidateGT) {
		gap = 1
	}
	if x.kind == candidateConstant && y.kind == candidateConstant && !equality {
		if greater {
			x, y = y, x
		}
		left := &p.variables[x.index]
		left.successors = append(left.successors, candidateSuccessor{int(y.index), gap})
		p.variables[y.index].predecessors++
		p.ordered = true
		return nil
	}
	if x.kind == candidateInteger {
		x, y = y, x
		greater = !greater
	}
	if y.kind != candidateInteger || (x.kind != candidateConstant && x.kind != candidateSquare) {
		return nil
	}
	v := &p.variables[x.index]
	lo, hi := int64(y.index)+gap, int64(y.index)-gap
	lower, upper := &v.low, &v.high
	if x.kind == candidateSquare {
		lower, upper = &v.minAbs, &v.maxAbs
		if lo <= 0 {
			lo = 0
		} else {
			f := int64(math.Sqrt(float64(lo)))
			if f*f < lo {
				f++
			}
			lo = f
		}
		if hi >= 0 {
			hi = int64(math.Sqrt(float64(hi)))
		}
	}
	if equality || greater {
		*lower = max(*lower, lo)
	}
	if equality || !greater {
		*upper = min(*upper, hi)
	}
	return nil
}

func (p *integerCandidate) assign(seed int64) (ok, changed bool) {
	changed = !p.assigned
	for i := range p.variables {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return false, false
		}
		v := &p.variables[i]
		lo, hi := max(v.low, v.minAbs), min(v.high, v.maxAbs)
		preferred := seed
		if lo > hi {
			lo, hi = max(v.low, -v.maxAbs), min(v.high, -v.minAbs)
			preferred = -seed
		}
		if lo > hi {
			return false, false
		}
		value := min(hi, max(lo, preferred))
		changed = changed || v.numeral.value != value
		v.numeral.value = value
	}
	p.assigned = true
	if !p.ordered {
		return true, changed
	}
	// Propagate lower bounds through the order constraints in topological order.
	// A cycle leaves the choice of a model to the ordinary solver.
	degree := make([]int, len(p.variables))
	queue := make([]int, 0, len(degree))
	for i, v := range p.variables {
		degree[i] = v.predecessors
		if degree[i] == 0 {
			queue = append(queue, i)
		}
	}
	for i := 0; i < len(queue); i++ {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return false, false
		}
		v := &p.variables[queue[i]]
		for _, e := range v.successors {
			w := &p.variables[e.variable]
			w.numeral.value = max(w.numeral.value, v.numeral.value+e.gap)
			if w.numeral.value > w.high {
				return false, false
			}
			degree[e.variable]--
			if degree[e.variable] == 0 {
				queue = append(queue, e.variable)
			}
		}
	}
	return len(queue) == len(p.variables), changed
}

type candidateComparison struct {
	left, right candidateTerm
	operation   int32
}

const maxInt64SquareRoot = 3037000499

func (t candidateTerm) value(vars []candidateVariable) (int64, bool) {
	switch t.kind {
	case candidateInteger:
		return int64(t.index), true
	case candidateConstant:
		return vars[t.index].numeral.value, true
	case candidateSquare:
		x := vars[t.index].numeral.value
		if x < -maxInt64SquareRoot || x > maxInt64SquareRoot {
			return 0, false
		}
		return x * x, true
	default:
		return 0, false
	}
}

func (c candidateComparison) satisfied(vars []candidateVariable) (bool, bool) {
	if c.operation == 0 {
		return false, false
	}
	l, ok := c.left.value(vars)
	if !ok {
		return false, false
	}
	r, ok := c.right.value(vars)
	if !ok {
		return false, false
	}
	switch c.operation {
	case candidateEqual:
		return l == r, true
	case candidateLE:
		return l <= r, true
	case candidateGE:
		return l >= r, true
	case candidateLT:
		return l < r, true
	case candidateGT:
		return l > r, true
	}
	return false, false
}
