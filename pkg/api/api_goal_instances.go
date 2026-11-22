package api

import "github.com/glycerine/g3/pkg/ast"

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

// Find the unambiguous ground constants of uninterpreted sorts in the final
// assertion. Each such sort contributes one term to a substitution.
func goalInstanceWitnesses(m *ast.ASTManager, goal ast.ASTHandle) (map[ast.ASTHandle]ast.ASTHandle, error) {
	ground, err := m.IsGround(goal)
	if err != nil || !ground {
		return nil, err
	}
	seen := make(map[ast.ASTHandle]bool)
	witnesses := make(map[ast.ASTHandle]ast.ASTHandle)
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
			delete(witnesses, sort)
		}
	}
	if len(witnesses) < 2 {
		return nil, nil
	}
	return witnesses, nil
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

func planGoalInstances(m *ast.ASTManager, assertions []ast.ASTHandle) (*goalInstancePlan, error) {
	if len(assertions) < 2 || len(assertions) > 8192 {
		return nil, nil
	}
	p := &goalInstancePlan{goal: assertions[len(assertions)-1]}
	witnesses, err := goalInstanceWitnesses(m, p.goal)
	if err != nil || witnesses == nil {
		return nil, err
	}
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
