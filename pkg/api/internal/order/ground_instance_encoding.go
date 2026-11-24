package order

import (
	"math/bits"

	"github.com/glycerine/g3/pkg/ast"
)

// Give each sort's values integer labels in order of first occurrence in a
// source model. Its nth encountered term can always use a label below n.
// Older terms extend with high zero bits as the pool grows. These codes
// represent the observed terms and place no bound on the source domain.
type groundValueCode struct {
	start int32
	width int
}

func (c groundValueCode) bit(i int) int32 {
	if i >= c.width {
		return -1
	}
	return c.start + int32(i)
}

type groundBoolEncoding struct {
	result    []int32
	codes     []groundValueCode
	labels    map[ast.ASTHandle]int
	sorts     map[uint32]ast.ASTHandle
	dataITE   map[uint32]bool
	nextInput int
	group     map[uint32][]uint32
	equal     map[[2]uint32]int32
	flags     []uint8
}

// Input slots interleave with gate slots. A zero gate denotes an input.
func (p *GroundInstancePool) newVar() int32 {
	k := &p.g.compiler
	if len(k.nodes) >= bitCompilerLimit {
		k.work = k.ceiling + 1
		return 1
	}
	k.nodes = append(k.nodes, bitGate{})
	k.Vars++
	return int32(k.Vars)
}

func (p *GroundInstancePool) eq(a, b uint32) int32 {
	if a == b {
		return 1
	}
	if a > b {
		a, b = b, a
	}
	key := [2]uint32{a, b}
	if v := p.equal[key]; v != 0 {
		return v
	}
	v := int32(1)
	if p.g.terms[a].boolean {
		v = p.g.compiler.ite(p.result[a], p.result[b], -p.result[b])
	} else {
		x, y := p.codes[a], p.codes[b]
		for j := 0; j < max(x.width, y.width); j++ {
			u, w := x.bit(j), y.bit(j)
			v = p.g.compiler.and(v, p.g.compiler.ite(u, w, -w))
		}
	}
	p.equal[key] = v
	return v
}

func (p *GroundInstancePool) assertLiteral(v int32) {
	if v != 1 {
		p.need(v)
		p.g.compiler.Clauses = append(p.g.compiler.Clauses, BooleanClause{v})
	}
}

func (p *GroundInstancePool) compileTerms() error {
	g := &p.g
	for i := uint32(len(p.result)); i < uint32(len(g.terms)); i++ {
		if p.g.compiler.Vars >= bitCompilerLimit || len(p.g.compiler.Clauses) > 120_000 {
			return errBitLimit
		}
		t := &g.terms[i]
		a := g.arguments[t.start : t.start+t.count]
		if e := g.compiler.charge(1 + len(a)); e != nil {
			return e
		}
		vs := p.result
		v := int32(0)
		var code groundValueCode
		if !t.boolean {
			st, ok := p.sorts[t.symbol]
			if !ok {
				return inapplicable
			}
			n := bits.Len(uint(p.labels[st]))
			if len(g.compiler.nodes)+n > bitCompilerLimit {
				return errBitLimit
			}
			p.labels[st]++
			code = groundValueCode{int32(p.g.compiler.Vars) + 1, n}
			for range n {
				p.newVar()
			}
		} else {
			switch t.operation {
			case opaqueOperation:
				v = p.newVar()
			case ast.ASTOpTrue:
				v = 1
			case ast.ASTOpFalse:
				v = -1
			case ast.ASTOpNot:
				v = -vs[a[0]]
			case ast.ASTOpAnd, ast.ASTOpOr:
				sign := int32(1)
				if t.operation == ast.ASTOpOr {
					sign = -1
				}
				v = 1
				for _, j := range a {
					v = p.g.compiler.and(v, sign*vs[j])
				}
				v *= sign
			case ast.ASTOpImplies:
				v = -p.g.compiler.and(vs[a[0]], -vs[a[1]])
			case ast.ASTOpIte:
				v = p.g.compiler.ite(vs[a[0]], vs[a[1]], vs[a[2]])
			case ast.ASTOpIff, ast.ASTOpEq:
				v = p.eq(a[0], a[1])
			case ast.ASTOpXor:
				v = -p.eq(a[0], a[1])
			default:
				return inapplicable
			}
		}
		p.result = append(p.result, v)
		p.codes = append(p.codes, code)
		if p.dataITE[t.symbol] {
			if vs[a[0]] == 0 || g.terms[a[1]].boolean || g.terms[a[2]].boolean {
				return inapplicable
			}
			p.guardEqual(vs[a[0]], i, a[1])
			p.guardEqual(-vs[a[0]], i, a[2])
		}
		if t.count == 0 || t.count > 8 || t.operation != opaqueOperation {
			continue
		}
		list := p.group[t.symbol]
		// Omitting later congruence pairs only weakens this private problem.
		if len(list) >= 16 {
			continue
		}
		for _, o := range list {
			ot := g.terms[o]
			if ot.count != t.count {
				return inapplicable
			}
			other := g.arguments[ot.start : ot.start+ot.count]
			if e := g.compiler.charge(1 + len(a)); e != nil {
				return e
			}
			same := int32(1)
			for j, x := range a {
				same = p.g.compiler.and(same, p.eq(x, other[j]))
			}
			p.guardEqual(same, i, o)
		}
		p.group[t.symbol] = append(list, i)
	}
	return nil
}

// Emit the directions of gate definitions reached from this polarity. Later
// siblings may need the opposite direction of a gate already in the SAT solver.
func (p *GroundInstancePool) need(l int32) {
	v, sign := l, uint8(1)
	if l < 0 {
		v = -l
		sign = 2
	}
	if v < 2 {
		return
	}
	k := &p.g.compiler
	i := v - 2
	gate := k.nodes[i]
	if gate.a == 0 {
		return
	}
	if n := len(k.nodes) - len(p.flags); n > 0 {
		p.flags = append(p.flags, make([]uint8, n)...)
	}
	if p.flags[i]&sign != 0 {
		return
	}
	p.flags[i] |= sign
	a, b, c := gate.a, gate.b, gate.c
	if c == 0 {
		if sign == 1 {
			p.need(a)
			p.need(b)
			p.g.compiler.Clauses = append(p.g.compiler.Clauses, BooleanClause{-v, a}, BooleanClause{-v, b})
		} else {
			p.need(-a)
			p.need(-b)
			p.g.compiler.Clauses = append(p.g.compiler.Clauses, BooleanClause{v, -a, -b})
		}
	} else {
		p.need(a)
		p.need(-a)
		if sign == 1 {
			p.need(b)
			p.need(c)
			p.g.compiler.Clauses = append(p.g.compiler.Clauses, BooleanClause{-a, -v, b}, BooleanClause{a, -v, c})
		} else {
			p.need(-b)
			p.need(-c)
			p.g.compiler.Clauses = append(p.g.compiler.Clauses, BooleanClause{-a, v, -b}, BooleanClause{a, v, -c})
		}
	}
}

func (p *GroundInstancePool) clause(a, b, c int32) {
	var row BooleanClause
	n := 0
	for _, x := range [3]int32{a, b, c} {
		if x == 0 || x == -1 {
			continue
		}
		if x == 1 || n > 0 && row[0] == -x || n > 1 && row[1] == -x {
			return
		}
		if n > 0 && row[0] == x || n > 1 && row[1] == x {
			continue
		}
		row[n] = x
		n++
	}
	for _, x := range row[:n] {
		p.need(x)
	}
	p.g.compiler.Clauses = append(p.g.compiler.Clauses, row)
}

func (p *GroundInstancePool) guardEqual(guard int32, a, b uint32) {
	if a == b || guard == -1 {
		return
	}
	require := func(x, y int32) { p.clause(-guard, -x, y); p.clause(-guard, x, -y) }
	if p.g.terms[a].boolean {
		require(p.result[a], p.result[b])
		return
	}
	x, y := p.codes[a], p.codes[b]
	for j := 0; j < max(x.width, y.width); j++ {
		u, w := x.bit(j), y.bit(j)
		require(u, w)
	}
}
