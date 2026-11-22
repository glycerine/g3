package order

import "github.com/glycerine/g3/pkg/ast"

// Opaque Boolean applications get one propositional variable per distinct
// ground term. Equality on data can identify a term with itself. All other
// relations between data and their theory interpretations are unconstrained.
func (g *groundProjection) booleanProblem() (*BooleanProblem, error) {
	k := &g.compiler
	k.Vars = 1
	values := make([]int32, len(g.terms))
	for i := 1; i < len(g.terms); i++ {
		t := &g.terms[i]
		if !t.boolean {
			continue
		}
		atom := t.operation == opaqueOperation
		if t.operation == ast.ASTOpEq {
			args := g.arguments[t.start : t.start+t.count]
			atom = args[0] != args[1] && !g.terms[args[0]].boolean
		}
		if atom {
			k.Vars++
			values[i] = int32(k.Vars)
		}
	}
	k.firstGate = int32(k.Vars + 1)
	k.gates = make(map[uint64]int32)
	for i := 1; i < len(g.terms); i++ {
		t := &g.terms[i]
		if err := k.charge(1 + int(t.count)); err != nil {
			return nil, err
		}
		if !t.boolean || values[i] != 0 {
			continue
		}
		a := g.arguments[t.start : t.start+t.count]
		switch t.operation {
		case ast.ASTOpTrue:
			values[i] = 1
		case ast.ASTOpFalse:
			values[i] = -1
		case ast.ASTOpNot:
			values[i] = -values[a[0]]
		case ast.ASTOpAnd, ast.ASTOpOr:
			sign := int32(1)
			if t.operation == ast.ASTOpOr {
				sign = -1
			}
			lit := int32(1)
			for _, j := range a {
				lit = k.and(lit, sign*values[j])
			}
			values[i] = sign * lit
		case ast.ASTOpImplies:
			values[i] = -k.and(values[a[0]], -values[a[1]])
		case ast.ASTOpIff, ast.ASTOpEq, ast.ASTOpXor:
			lit := int32(1)
			if a[0] != a[1] {
				lit = k.ite(values[a[0]], values[a[1]], -values[a[1]])
			}
			if t.operation == ast.ASTOpXor {
				lit = -lit
			}
			values[i] = lit
		case ast.ASTOpIte:
			values[i] = k.ite(values[a[0]], values[a[1]], values[a[2]])
		default:
			return nil, inapplicable
		}
	}
	for _, i := range g.roots {
		if lit := values[i]; lit != 1 {
			k.roots = append(k.roots, bitRoot{after: len(k.nodes), lit: lit})
		}
	}
	for _, p := range g.implications {
		lit := -k.and(values[p.from], -values[p.to])
		if lit != 1 {
			k.roots = append(k.roots, bitRoot{after: len(k.nodes), lit: lit})
		}
	}
	if err := k.charge(len(k.roots) + 2*len(k.nodes)); err != nil {
		return nil, err
	}
	if !k.emitGates() {
		return nil, errBitLimit
	}
	g.termLiterals = values
	k.Work = uint32(k.work)
	return &k.BooleanProblem, nil
}
