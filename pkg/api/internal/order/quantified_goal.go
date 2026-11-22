package order

import "github.com/glycerine/g3/pkg/ast"

// ProjectQuantifiedGoal builds a propositional relaxation of a quantified goal
// and its premises. A universal Q implies each ground instance B of its body.
// For an existential the implication runs from B to Q. Keeping Q as an independent
// Boolean atom lets the source use the quantifier under any connective. Every
// source model satisfies the resulting implications and the projected roots.
func ProjectQuantifiedGoal(m *ast.ASTManager, assertions []ast.ASTHandle, steps uint32) (*BooleanProblem, error) {
	if len(assertions) < 2 || len(assertions) > 8192 {
		return nil, nil
	}
	eligible, err := projectGoalShape(m, assertions[len(assertions)-1])
	if err != nil || !eligible {
		return nil, err
	}
	g := groundProjection{
		manager:      m,
		input:        []projectionNode{{}},
		terms:        []groundApplication{{}},
		sourceIDs:    make(map[ast.ASTHandle]uint32),
		symbols:      make(map[ast.ASTHandle]uint32),
		applications: make(map[uint64]uint32),
		constants:    make(map[ast.ASTHandle]uint32),
		compiler:     bitCompiler{m: m, ceiling: int(min(steps, 500_000))},
	}
	problem, err := g.problem(assertions)
	if err == errBitLimit || err == inapplicable {
		return nil, nil
	}
	return problem, err
}

// Try a single quantified subformula or a nested quantifier. On incremental
// Apple checks, the ordinary solver resolves combinations of flat quantifiers
// for less work than rebuilding the propositional problem.
func projectGoalShape(m *ast.ASTManager, goal ast.ASTHandle) (bool, error) {
	has, err := m.HasQuantifiers(goal)
	if err != nil || !has {
		return false, err
	}
	seen := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{goal}
	count := 0
	for len(todo) > 0 {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		if len(seen) > 512 {
			return false, nil
		}
		head, err := m.NodeHeader(h)
		if err != nil {
			return false, err
		}
		if head.Kind == ast.ASTQuantifier {
			count++
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return false, err
			}
			nested, err := m.HasQuantifiers(body)
			if err != nil || nested {
				return nested, err
			}
			continue
		}
		if head.NumArgs > 256 {
			return false, nil
		}
		for i := uint32(0); i < head.NumArgs; i++ {
			child, err := m.GetArg(h, i)
			if err != nil {
				return false, err
			}
			todo = append(todo, child)
		}
	}
	return count == 1, nil
}

type groundProjection struct {
	manager            *ast.ASTManager
	compiler           bitCompiler
	input              []projectionNode
	children           []uint32
	sourceIDs, symbols map[ast.ASTHandle]uint32
	constants          map[ast.ASTHandle]uint32

	terms                        []groundApplication
	arguments                    []uint32
	applications                 map[uint64]uint32
	global, local, version       []uint32
	epoch                        uint32
	bindings, quantifiers, roots []uint32
	implications                 []groundImplication
}

type groundImplication struct{ from, to uint32 }

type projectionNode struct {
	expr, sort                ast.ASTHandle
	start, count              uint32
	symbol, index             uint32
	operation                 int32
	kind                      ast.ASTKind
	ground, boolean, constant bool
}

const opaqueOperation int32 = -1

func (g *groundProjection) problem(assertions []ast.ASTHandle) (*BooleanProblem, error) {
	inputs := make([]uint32, len(assertions))
	for i, h := range assertions {
		id, err := g.read(h, 0)
		if err != nil {
			return nil, err
		}
		if !g.input[id].boolean {
			return nil, inapplicable
		}
		inputs[i] = id
	}
	if err := g.chooseConstants(inputs); err != nil {
		return nil, err
	}
	g.global = make([]uint32, len(g.input))
	g.local = make([]uint32, len(g.input))
	g.version = make([]uint32, len(g.input))
	for _, id := range inputs {
		term, err := g.ground(id, 0)
		if err != nil {
			return nil, err
		}
		g.roots = append(g.roots, term)
	}
	for _, id := range g.quantifiers {
		if err := g.addInstance(id); err != nil {
			return nil, err
		}
	}
	return g.booleanProblem()
}

// Count occurrences in the unfolded assertions with one reverse pass over
// the shared DAG. The first most frequent constant of each sort wins ties.
func (g *groundProjection) chooseConstants(inputs []uint32) error {
	count := make([]uint64, len(g.input))
	for _, id := range inputs {
		count[id]++
	}
	for i := len(g.input) - 1; i > 0; i-- {
		n := &g.input[i]
		if err := g.compiler.charge(1 + int(n.count)); err != nil {
			return err
		}
		for _, child := range g.children[n.start : n.start+n.count] {
			count[child] = min(uint64(1)<<40, count[i]+count[child])
		}
	}
	for i := 1; i < len(g.input); i++ {
		n := &g.input[i]
		if n.constant && count[g.constants[n.sort]] < count[i] {
			g.constants[n.sort] = uint32(i)
		}
	}
	return nil
}

func (g *groundProjection) read(h ast.ASTHandle, depth int) (uint32, error) {
	if id := g.sourceIDs[h]; id != 0 {
		return id, nil
	}
	if depth > 192 || len(g.input) > 12000 {
		return 0, errBitLimit
	}
	hd, err := g.manager.NodeHeader(h)
	if err != nil {
		return 0, err
	}
	n := projectionNode{expr: h, kind: hd.Kind, operation: opaqueOperation}
	n.boolean, err = g.manager.HasBoolSort(h)
	if err != nil {
		return 0, err
	}
	symbol := hd.Decl
	switch hd.Kind {
	case ast.ASTVar:
		n.index, err = g.manager.GetVarIndex(h)
		if err == nil {
			n.sort, err = g.manager.GetSort(h)
		}
	case ast.ASTQuantifier:
		if !n.boolean {
			return 0, inapplicable
		}
		symbol = h
		var body ast.ASTHandle
		body, err = g.manager.GetQuantifierBody(h)
		if err != nil {
			return 0, err
		}
		child, err := g.read(body, depth+1)
		if err != nil {
			return 0, err
		}
		if !g.input[child].boolean {
			return 0, inapplicable
		}
		n.start, n.count = uint32(len(g.children)), 1
		g.children = append(g.children, child)
	case ast.ASTApp:
		if hd.NumArgs > 8192 {
			return 0, errBitLimit
		}
		if err = g.compiler.charge(int(hd.NumArgs)); err != nil {
			return 0, err
		}
		n.ground, err = g.manager.IsGround(h)
		if err != nil {
			return 0, err
		}
		n.constant = !n.boolean && hd.NumArgs == 0 && hd.Family == ast.NullFamilyID
		if n.constant {
			n.sort, err = g.manager.GetSort(h)
			if err != nil {
				return 0, err
			}
		}
		n.start, n.count = uint32(len(g.children)), hd.NumArgs
		g.children = append(g.children, make([]uint32, hd.NumArgs)...)
		for j := uint32(0); j < hd.NumArgs; j++ {
			ch, err := g.manager.GetArg(h, j)
			if err != nil {
				return 0, err
			}
			id, err := g.read(ch, depth+1)
			if err != nil {
				return 0, err
			}
			g.children[n.start+j] = id
		}
		if n.boolean && hd.Family == g.manager.BasicFamilyID() {
			n.operation, err = g.booleanOperation(hd.DeclKind, g.children[n.start:n.start+n.count])
		}
	default:
		return 0, inapplicable
	}
	if err != nil {
		return 0, err
	}
	if err = g.compiler.charge(1); err != nil {
		return 0, err
	}
	if !symbol.IsNull() {
		n.symbol = g.symbols[symbol]
		if n.symbol == 0 {
			n.symbol = uint32(len(g.symbols) + 1)
			g.symbols[symbol] = n.symbol
		}
	}
	id := uint32(len(g.input))
	g.input = append(g.input, n)
	g.sourceIDs[h] = id
	return id, nil
}

// Validate the shapes that will become Boolean gates. Applications from
// other theories can be represented by their exact ground syntax.
func (g *groundProjection) booleanOperation(op int32, args []uint32) (int32, error) {
	op, err := booleanGateShape(op, uint32(len(args)))
	if err != nil || op == opaqueOperation {
		return op, err
	}
	if op == ast.ASTOpEq {
		a, err := g.manager.GetSort(g.input[args[0]].expr)
		if err != nil {
			return 0, err
		}
		b, err := g.manager.GetSort(g.input[args[1]].expr)
		if err != nil {
			return 0, err
		}
		if a != b {
			return 0, inapplicable
		}
		return op, nil
	}
	for _, a := range args {
		if !g.input[a].boolean {
			return 0, inapplicable
		}
	}
	return op, nil
}

// Other basic-family operations can be left as independent propositions.
// Gate encodings consume the arities checked here.
func booleanGateShape(op int32, count uint32) (int32, error) {
	switch op {
	case ast.ASTOpTrue, ast.ASTOpFalse:
		if count != 0 {
			return 0, inapplicable
		}
	case ast.ASTOpAnd, ast.ASTOpOr:
	case ast.ASTOpNot:
		if count != 1 {
			return 0, inapplicable
		}
	case ast.ASTOpImplies, ast.ASTOpIff, ast.ASTOpXor, ast.ASTOpEq:
		if count != 2 {
			return 0, inapplicable
		}
	case ast.ASTOpIte:
		if count != 3 {
			return 0, inapplicable
		}
	default:
		return opaqueOperation, nil
	}
	return op, nil
}
