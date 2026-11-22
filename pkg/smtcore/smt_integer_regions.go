package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Within a region each integer subexpression equals slope*x + offset and
// each Boolean subexpression is constant. Bounding both coefficients by C
// puts every root of a comparison within [-2*C, 2*C], since a nonzero
// integer slope difference has magnitude at least one. Representatives just
// outside that range therefore cover the two infinite tails. The bound also
// keeps multiplication and evaluation within int64.
const (
	integerCoefficientLimit int64 = 1 << 27
	integerTail                   = 2*integerCoefficientLimit + 1
	integerRegionWork             = 450_000
)

type integerAffine struct{ slope, offset int64 }

func (v integerAffine) fits() bool {
	return -integerCoefficientLimit <= v.slope && v.slope <= integerCoefficientLimit &&
		-integerCoefficientLimit <= v.offset && v.offset <= integerCoefficientLimit
}

type integerRegionOp struct {
	kind       int32
	first, end uint16
	constant   int64
}

const (
	regionVariable int32 = -1 - iota
	regionNumeral
)

// Arithmetic and Boolean declaration kinds use separate numbering in the AST.
const regionArithmetic int32 = 100

type integerRegions struct {
	ops    []integerRegionOp
	args   []uint16
	values []integerAffine
}

type integerRegionCompiler struct {
	m       *ast.ASTManager
	family  ast.FamilyID
	intSort ast.ASTHandle
	nodes   map[ast.ASTHandle]uint16
	program integerRegions
}

func compileIntegerRegions(m *ast.ASTManager, family ast.FamilyID, intSort, body ast.ASTHandle) *integerRegions {
	ok, err := m.HasBoolSort(body)
	if err != nil || !ok {
		return nil
	}
	c := integerRegionCompiler{m: m, family: family, intSort: intSort, nodes: make(map[ast.ASTHandle]uint16)}
	if _, ok := c.compile(body, 0); !ok {
		return nil
	}
	c.program.values = make([]integerAffine, len(c.program.ops))
	return &c.program
}

func (c *integerRegionCompiler) compile(h ast.ASTHandle, depth int) (uint16, bool) {
	if id, ok := c.nodes[h]; ok {
		return id, true
	}
	if depth >= 256 || c.m.ResourceLimit().CancelFlag() {
		return 0, false
	}
	header, err := c.m.NodeHeader(h)
	if err != nil {
		return 0, false
	}
	op := integerRegionOp{kind: header.DeclKind}
	n := int(header.NumArgs)
	switch {
	case header.Kind == ast.ASTVar:
		index, err := c.m.GetVarIndex(h)
		if err != nil || index != 0 {
			return 0, false
		}
		sort, err := c.m.GetSort(h)
		if err != nil || sort != c.intSort {
			return 0, false
		}
		op.kind = regionVariable
	case header.Kind != ast.ASTApp:
		return 0, false
	case header.Family == c.family:
		op.kind += regionArithmetic
		switch header.DeclKind {
		case smtArithOpNum:
			v, ok, err := c.m.ArithmeticSmallInteger(h)
			if err != nil || !ok || int64(v) < -integerCoefficientLimit || int64(v) > integerCoefficientLimit {
				return 0, false
			}
			op.kind, op.constant = regionNumeral, int64(v)
		case smtArithOpLE, smtArithOpGE, smtArithOpLT, smtArithOpGT:
			if n != 2 {
				return 0, false
			}
		case smtArithOpAdd, smtArithOpSub, smtArithOpMul:
			if n == 0 {
				return 0, false
			}
		case smtArithOpUMinus:
			if n != 1 {
				return 0, false
			}
		default:
			return 0, false
		}
	case header.Family == c.m.BasicFamilyID():
		switch op.kind {
		case ast.ASTOpTrue, ast.ASTOpFalse, ast.ASTOpAnd, ast.ASTOpOr:
		case ast.ASTOpNot:
			if n != 1 {
				return 0, false
			}
		case ast.ASTOpEq, ast.ASTOpIff, ast.ASTOpXor, ast.ASTOpImplies:
			if n != 2 {
				return 0, false
			}
		case ast.ASTOpIte:
			if n != 3 {
				return 0, false
			}
		default:
			return 0, false
		}
	default:
		return 0, false
	}
	p := &c.program
	if n > 1024 || len(p.args)+n > 24000 {
		return 0, false
	}
	op.first, op.end = uint16(len(p.args)), uint16(len(p.args)+n)
	p.args = append(p.args, make([]uint16, n)...)
	for i := 0; i < n; i++ {
		arg, err := c.m.GetArg(h, uint32(i))
		if err != nil {
			return 0, false
		}
		id, ok := c.compile(arg, depth+1)
		if !ok {
			return 0, false
		}
		p.args[int(op.first)+i] = id
	}
	if len(p.ops) >= 4096 {
		return 0, false
	}
	id := uint16(len(p.ops))
	p.ops = append(p.ops, op)
	c.nodes[h] = id
	return id, true
}

func regionTruth(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

// An exact root can form a singleton for equality or a strict inequality.
// Visit both adjacent integer boundaries, including for negative quotients.
func (v integerAffine) nextBoundary(x, next int64) int64 {
	if v.slope == 0 {
		return next
	}
	numerator := -v.offset
	floor := numerator / v.slope
	if numerator%v.slope != 0 && (numerator < 0) != (v.slope < 0) {
		floor--
	}
	if floor > x {
		return min(next, floor)
	}
	if floor == x {
		return min(next, floor+1)
	}
	return next
}

// evaluate returns the body's truth at x and the first later integer at
// which any comparison could change. An unsupported operation in either
// arm of an ite declines the evaluation.
func (p *integerRegions) evaluate(x int64) (truth bool, next int64, ok bool) {
	next = integerTail + 1
	for i, op := range p.ops {
		ids := p.args[op.first:op.end]
		var v integerAffine
		switch op.kind {
		case regionVariable:
			v.slope = 1
		case regionNumeral:
			v.offset = op.constant
		case ast.ASTOpTrue:
			v.offset = 1
		case ast.ASTOpFalse:
		case ast.ASTOpNot:
			v.offset = 1 - p.values[ids[0]].offset
		case ast.ASTOpAnd:
			v.offset = 1
			for _, id := range ids {
				v.offset &= p.values[id].offset
			}
		case ast.ASTOpOr:
			for _, id := range ids {
				v.offset |= p.values[id].offset
			}
		case ast.ASTOpIte:
			if p.values[ids[0]].offset != 0 {
				v = p.values[ids[1]]
			} else {
				v = p.values[ids[2]]
			}
		case ast.ASTOpIff:
			v.offset = regionTruth(p.values[ids[0]].offset == p.values[ids[1]].offset)
		case ast.ASTOpXor:
			v.offset = p.values[ids[0]].offset ^ p.values[ids[1]].offset
		case ast.ASTOpImplies:
			v.offset = (1 - p.values[ids[0]].offset) | p.values[ids[1]].offset
		case regionArithmetic + smtArithOpUMinus:
			u := p.values[ids[0]]
			v = integerAffine{-u.slope, -u.offset}
		case regionArithmetic + smtArithOpMul:
			v.offset = 1
			for _, id := range ids {
				u := p.values[id]
				if v.slope != 0 && u.slope != 0 {
					return false, 0, false
				}
				v = integerAffine{v.slope*u.offset + u.slope*v.offset, v.offset * u.offset}
				if !v.fits() {
					return false, 0, false
				}
			}
		case regionArithmetic + smtArithOpAdd, regionArithmetic + smtArithOpSub:
			for j, id := range ids {
				u := p.values[id]
				if op.kind == regionArithmetic+smtArithOpSub && (j != 0 || len(ids) == 1) {
					v.slope -= u.slope
					v.offset -= u.offset
				} else {
					v.slope += u.slope
					v.offset += u.offset
				}
				if !v.fits() {
					return false, 0, false
				}
			}
		case ast.ASTOpEq, regionArithmetic + smtArithOpLE, regionArithmetic + smtArithOpGE, regionArithmetic + smtArithOpLT, regionArithmetic + smtArithOpGT:
			l, r := p.values[ids[0]], p.values[ids[1]]
			diff := integerAffine{l.slope - r.slope, l.offset - r.offset}
			next = diff.nextBoundary(x, next)
			d := diff.slope*x + diff.offset
			switch op.kind {
			case ast.ASTOpEq:
				v.offset = regionTruth(d == 0)
			case regionArithmetic + smtArithOpLE:
				v.offset = regionTruth(d <= 0)
			case regionArithmetic + smtArithOpGE:
				v.offset = regionTruth(d >= 0)
			case regionArithmetic + smtArithOpLT:
				v.offset = regionTruth(d < 0)
			case regionArithmetic + smtArithOpGT:
				v.offset = regionTruth(d > 0)
			}
		default:
			return false, 0, false
		}
		p.values[i] = v
	}
	return p.values[len(p.values)-1].offset != 0, next, true
}

// counterexample decides whether the body is false somewhere on the integer
// line. Undef sends the original obligation to the auxiliary SMT solver.
func (p *integerRegions) counterexample(m *ast.ASTManager) (lbool.LBool, int64) {
	work := integerRegionWork
	for x := -integerTail; x <= integerTail; {
		work -= len(p.ops) + len(p.args)
		if work < 0 || m.ResourceLimit().CancelFlag() {
			return lbool.Undef, 0
		}
		truth, next, ok := p.evaluate(x)
		if !ok {
			return lbool.Undef, 0
		}
		if !truth {
			return lbool.True, max(x, min(int64(0), next-1))
		}
		x = next
	}
	return lbool.False, 0
}
