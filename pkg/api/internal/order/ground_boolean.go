package order

import "github.com/glycerine/g3/pkg/ast"

// ProjectGroundBooleans keeps the signed goal and the ground conjuncts of
// positive premises. Every source model assigns a consistent Boolean value to
// each opaque ground atom. UNSAT in this relaxation therefore refutes the source.
func ProjectGroundBooleans(m *ast.ASTManager, assertions []ast.ASTHandle, steps uint32) (*BooleanProblem, error) {
	if len(assertions) < 2 || len(assertions) > 8192 {
		return nil, nil
	}
	goal := assertions[len(assertions)-1]
	ground, err := m.IsGround(goal)
	if err != nil || !ground {
		return nil, err
	}
	g := groundBooleans{groundProjection: groundProjection{
		manager: m, compiler: bitCompiler{m: m, ceiling: int(min(steps, 240_000))},
		terms: []groundApplication{{}}, sourceIDs: make(map[ast.ASTHandle]uint32),
	}}
	problem, err := g.project(assertions[:len(assertions)-1], goal)
	if err == errBitLimit || err == inapplicable {
		return nil, nil
	}
	return problem, err
}

type groundBooleans struct {
	groundProjection
	goalConstants int
}

func (g *groundBooleans) project(base []ast.ASTHandle, goal ast.ASTHandle) (*BooleanProblem, error) {
	q, err := g.readBoolean(goal, 0, true)
	if err != nil || g.goalConstants < 2 {
		return nil, err
	}
	seen := make(map[ast.ASTHandle]bool)
	stack := make([]ast.ASTHandle, 0, len(base))
	for i := len(base) - 1; i >= 0; i-- {
		stack = append(stack, base[i])
	}
	for len(stack) != 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		if err := g.compiler.charge(1); err != nil {
			return nil, err
		}
		hd, err := g.manager.NodeHeader(h)
		if err != nil {
			return nil, err
		}
		if hd.Kind == ast.ASTApp && hd.Family == g.manager.BasicFamilyID() && hd.DeclKind == ast.ASTOpAnd {
			if len(seen) > 16_000 || hd.NumArgs > 8192 || len(stack)+int(hd.NumArgs) > 16_384 {
				return nil, errBitLimit
			}
			if err := g.compiler.charge(int(hd.NumArgs)); err != nil {
				return nil, err
			}
			for i := hd.NumArgs; i > 0; i-- {
				a, err := g.manager.GetArg(h, i-1)
				if err != nil {
					return nil, err
				}
				stack = append(stack, a)
			}
			continue
		}
		ground, err := g.manager.IsGround(h)
		if err != nil {
			return nil, err
		}
		if ground {
			id, err := g.readBoolean(h, 0, false)
			if err != nil {
				return nil, err
			}
			g.roots = append(g.roots, id)
		}
	}
	g.roots = append(g.roots, q)
	return g.booleanProblem()
}

func (g *groundBooleans) readBoolean(h ast.ASTHandle, depth int, goal bool) (uint32, error) {
	if id := g.sourceIDs[h]; id != 0 {
		return id, nil
	}
	if depth > 160 || len(g.terms) > 24_000 || goal && len(g.terms) > 128 {
		return 0, errBitLimit
	}
	m := g.manager
	boolean, err := m.HasBoolSort(h)
	if err != nil {
		return 0, err
	}
	if !boolean {
		return 0, inapplicable
	}
	hd, err := m.NodeHeader(h)
	if err != nil {
		return 0, err
	}
	if hd.Kind != ast.ASTApp || hd.NumArgs > 8192 {
		return 0, inapplicable
	}
	op := opaqueOperation
	arity := uint32(0)
	if hd.Family == m.BasicFamilyID() {
		op, err = booleanGateShape(hd.DeclKind, hd.NumArgs)
		if err != nil {
			return 0, err
		}
		if op == ast.ASTOpEq {
			a, err := m.GetArg(h, 0)
			if err != nil {
				return 0, err
			}
			b, err := m.GetArg(h, 1)
			if err != nil {
				return 0, err
			}
			if a == b {
				op = ast.ASTOpTrue
			} else if ok, err := m.HasBoolSort(a); err != nil {
				return 0, err
			} else if !ok {
				op = opaqueOperation
			}
		}
		if op != opaqueOperation && op != ast.ASTOpTrue {
			arity = hd.NumArgs
		}
	}
	if goal && op == opaqueOperation {
		if hd.Family != ast.NullFamilyID || hd.NumArgs != 0 {
			return 0, inapplicable
		}
		g.goalConstants++
	}
	if err := g.compiler.charge(1 + int(arity)); err != nil {
		return 0, err
	}
	start := len(g.arguments)
	g.arguments = append(g.arguments, make([]uint32, arity)...)
	for i := uint32(0); i < arity; i++ {
		a, err := m.GetArg(h, i)
		if err != nil {
			return 0, err
		}
		id, err := g.readBoolean(a, depth+1, goal)
		if err != nil {
			return 0, err
		}
		g.arguments[start+int(i)] = id
	}
	id := uint32(len(g.terms))
	g.sourceIDs[h] = id
	g.terms = append(g.terms, groundApplication{start: uint32(start), count: arity, operation: op, boolean: true})
	return id, nil
}
