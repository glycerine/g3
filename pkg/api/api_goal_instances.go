package api

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

// A ground instance is a consequence of a positive universal premise.
// The binding range follows declaration order, which is the reverse of
// de Bruijn index order. All handles in a plan borrow the input assertions.
type goalInstance struct {
	body ast.ASTHandle
	end  int
}

type goalInstancePlan struct {
	goal     ast.ASTHandle
	ground   []ast.ASTHandle
	bindings []ast.ASTHandle
	rules    []goalInstance
}

type groundGoalTerms struct {
	constants    map[ast.ASTHandle]ast.ASTHandle
	maxArity     uint32
	ambiguous    bool
	relationSort ast.ASTHandle
}

// When several independent domains each have a single goal constant,
// extend a failed one-tuple proof with observed applications and witnesses.
func (g *groundGoalTerms) expandedPlan(assertions []ast.ASTHandle) *witnessInstancePlan {
	if g.ambiguous || len(g.constants) < 4 {
		return nil
	}
	p := &witnessInstancePlan{assertions: assertions, width: 3, rounds: 1}
	p.sorts = make([]ast.ASTHandle, 0, len(g.constants))
	for st := range g.constants {
		p.sorts = append(p.sorts, st)
	}
	sort.Slice(p.sorts, func(i, j int) bool { return p.sorts[i].Slot() < p.sorts[j].Slot() })
	if g.maxArity >= 3 {
		p.width, p.rounds = 10, 2
	}
	return p
}

// Find the unambiguous ground constants of uninterpreted sorts in the final
// assertion. Each such sort contributes one term to a substitution.
func goalInstanceWitnesses(m *ast.ASTManager, goal ast.ASTHandle) (*groundGoalTerms, error) {
	ground, err := m.IsGround(goal)
	if err != nil || !ground {
		return nil, err
	}
	relationGoal, err := violatedConjunction(m, goal)
	if err != nil {
		return nil, err
	}
	var relations []ast.ASTHandle
	var threeConstants map[ast.ASTHandle]bool
	seen := make(map[ast.ASTHandle]bool)
	witnesses := make(map[ast.ASTHandle]ast.ASTHandle)
	terms := groundGoalTerms{constants: witnesses}
	functions := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{goal}
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		header, err := m.NodeHeader(h)
		if err != nil {
			return nil, err
		}
		if header.Kind != ast.ASTApp || len(seen) > 256 || header.NumArgs > 256 {
			return nil, nil
		}
		if len(seen) > 160 || header.NumArgs > 32 {
			relationGoal = false
		}
		if relationGoal && header.NumArgs == 2 && header.Family == ast.NullFamilyID {
			boolean, err := m.HasBoolSort(h)
			if err != nil {
				return nil, err
			}
			if boolean {
				relations = append(relations, header.Decl)
			}
		}
		if header.NumArgs > 0 && header.Family == ast.NullFamilyID {
			terms.maxArity = max(terms.maxArity, header.NumArgs)
			if len(functions) < 3 {
				functions[header.Decl] = true
			}
		}
		if header.NumArgs == 0 && header.Family == ast.NullFamilyID {
			sort, err := m.GetSort(h)
			if err != nil {
				return nil, err
			}
			uninterpreted, err := m.IsUninterpretedSort(sort)
			if err != nil {
				return nil, err
			}
			if uninterpreted {
				prior, found := witnesses[sort]
				if !found {
					witnesses[sort] = h
				} else if relationGoal && prior.IsNull() {
					// The walk visits each constant once. A null entry already saw two.
					if threeConstants == nil {
						threeConstants = make(map[ast.ASTHandle]bool)
					}
					threeConstants[sort] = true
				} else if prior != h {
					witnesses[sort] = ast.ASTHandle{}
				}
			}
		}
		for i := header.NumArgs; i > 0; i-- {
			child, err := m.GetArg(h, i-1)
			if err != nil {
				return nil, err
			}
			stack = append(stack, child)
		}
	}
	for sort, h := range witnesses {
		if h.IsNull() {
			terms.ambiguous = true
			delete(witnesses, sort)
		}
	}
	// Goals involving just one or two functions are usually cheaper to
	// check in the existing incremental solver, even with many premises.
	if len(witnesses) < 2 || len(functions) < 3 {
		terms.constants = nil
	}
	if relationGoal && len(threeConstants) != 0 {
		for _, decl := range relations {
			first, err := m.GetDomain(decl, 0)
			if err != nil {
				return nil, err
			}
			if !threeConstants[first] || terms.relationSort == first {
				continue
			}
			second, err := m.GetDomain(decl, 1)
			if err != nil {
				return nil, err
			}
			if first != second {
				continue
			}
			if !terms.relationSort.IsNull() {
				terms.relationSort = ast.ASTHandle{}
				break
			}
			terms.relationSort = first
		}
	}
	if terms.constants == nil && terms.relationSort.IsNull() {
		return nil, nil
	}
	return &terms, nil
}

func (p *goalInstancePlan) addUniversal(m *ast.ASTManager, q ast.ASTHandle, witnesses map[ast.ASTHandle]ast.ASTHandle) error {
	forall, err := m.IsForall(q)
	if err != nil || !forall {
		return err
	}
	body, err := m.GetQuantifierBody(q)
	if err != nil {
		return err
	}
	nested, err := m.HasQuantifiers(body)
	if err != nil || nested {
		return err
	}
	n, err := m.GetNumDeclarations(q)
	if err != nil || n == 0 || n > 12 {
		return err
	}
	start := len(p.bindings)
	for j := uint32(0); j < n; j++ {
		sort, err := m.GetDeclarationSort(q, j)
		if err != nil {
			return err
		}
		h, ok := witnesses[sort]
		if !ok {
			p.bindings = p.bindings[:start]
			return nil
		}
		p.bindings = append(p.bindings, h)
	}
	p.rules = append(p.rules, goalInstance{body: body, end: len(p.bindings)})
	return nil
}

func planGroundGoals(m *ast.ASTManager, assertions []ast.ASTHandle) (*goalInstancePlan, *witnessInstancePlan, error) {
	if len(assertions) < 2 || len(assertions) > 8192 {
		return nil, nil, nil
	}
	terms, err := goalInstanceWitnesses(m, assertions[len(assertions)-1])
	if err != nil || terms == nil {
		return nil, nil, err
	}
	if terms.constants != nil {
		plan, err := planGoalInstances(m, assertions, terms.constants)
		witness := terms.expandedPlan(assertions)
		if plan != nil || err != nil || witness != nil {
			return plan, witness, err
		}
	}
	if terms.relationSort.IsNull() {
		return nil, nil, nil
	}
	// A relation over a single domain can link several named objects in the
	// violated postcondition. Check its connected premises at three terms.
	return nil, &witnessInstancePlan{assertions: assertions, sorts: []ast.ASTHandle{terms.relationSort}, width: 3, rounds: 1, selectedOnly: true}, nil
}

func planGoalInstances(m *ast.ASTManager, assertions []ast.ASTHandle, witnesses map[ast.ASTHandle]ast.ASTHandle) (*goalInstancePlan, error) {
	p := &goalInstancePlan{goal: assertions[len(assertions)-1]}
	stack := make([]ast.ASTHandle, 0, len(assertions))
	for i := len(assertions) - 2; i >= 0; i-- {
		stack = append(stack, assertions[i])
	}
	for visits := 0; len(stack) > 0; visits++ {
		if visits >= 8192 || visits%128 == 0 && m.ResourceLimit().CancelFlag() {
			return nil, nil
		}
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		header, err := m.NodeHeader(h)
		if err != nil {
			return nil, err
		}
		switch {
		case header.Kind == ast.ASTApp && header.Family == m.BasicFamilyID() && header.DeclKind == ast.ASTOpAnd:
			if uint64(header.NumArgs)+uint64(len(stack)) > 8192 {
				return nil, nil
			}
			for i := header.NumArgs; i > 0; i-- {
				child, err := m.GetArg(h, i-1)
				if err != nil {
					return nil, err
				}
				stack = append(stack, child)
			}
		case header.Kind == ast.ASTQuantifier:
			if err = p.addUniversal(m, h, witnesses); err != nil {
				return nil, err
			}
		default:
			ground, err := m.IsGround(h)
			if err != nil {
				return nil, err
			}
			if ground {
				p.ground = append(p.ground, h)
			}
		}
	}
	// Creating a second solver pays for itself on problems dominated by
	// quantified premises that can all be instantiated at the goal terms.
	if len(p.rules) < 96 || len(p.rules) < 3*len(p.ground) {
		return nil, nil
	}
	return p, nil
}

// violatedConjunction recognizes a negated implication with a conjunctive
// consequent. Singleton conjunctions and nested implication guards are common
// in verification conditions. This filter only chooses when to attempt search.
func violatedConjunction(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	negative, implication := false, false
	for depth := 0; depth < 32; depth++ {
		hd, err := m.NodeHeader(h)
		if err != nil {
			return false, err
		}
		if hd.Kind != ast.ASTApp || hd.Family != m.BasicFamilyID() {
			return false, nil
		}
		var child uint32
		switch hd.DeclKind {
		case ast.ASTOpNot:
			if hd.NumArgs != 1 {
				return false, nil
			}
			negative = !negative
		case ast.ASTOpAnd:
			if hd.NumArgs != 1 {
				return implication && negative && hd.NumArgs >= 2, nil
			}
		case ast.ASTOpImplies:
			if !negative || hd.NumArgs != 2 {
				return false, nil
			}
			implication = true
			child = 1
		default:
			return false, nil
		}
		h, err = m.GetArg(h, child)
		if err != nil {
			return false, err
		}
	}
	return false, nil
}
