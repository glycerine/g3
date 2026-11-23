package tactic

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	basetactic "github.com/glycerine/g3/pkg/tactic"
)

// Two independent constants subject to the same set of unary formulas may
// take the same value. Keep the first constant's original assertions, then
// extend a satisfying model by assigning its value to each omitted constant.
// An UNSAT result uses a subset of the original assertions and dependencies.
func (t *NLSATTactic) reduceIndependentPolynomials(goal *basetactic.Goal, result *basetactic.TacticResult, forms []ast.ASTHandle, arithmetic ast.FamilyID) (bool, error) {
	p := polynomialComponents{
		manager: t.manager, arithmetic: arithmetic,
		terms: make(map[ast.ASTHandle]componentTerm), shapes: make(map[componentShape]uint32),
		sequences: make(map[componentLink]uint32),
	}
	for i, form := range forms {
		if i%64 == 0 && t.manager.Limit.CancelFlag() {
			return false, nil
		}
		term, ok, err := p.scan(form, 0)
		if err != nil || !ok {
			return false, err
		}
		if term.variable != 0 && term.variable != mixedComponent {
			v := &p.variables[term.variable-1]
			v.constraints = append(v.constraints, int(term.shape))
		}
	}

	// Small reductions rarely repay the construction of a new solver goal.
	if p.group() < 4 || t.manager.Limit.CancelFlag() {
		return false, nil
	}
	return p.publish(t, goal, result, forms)
}

const mixedComponent = ^uint32(0)

type componentTerm struct {
	shape, variable uint32 // zero variable for a closed interpreted term
}

type componentShape struct {
	head      ast.ASTHandle // exact declaration, or the sort of the variable hole
	arguments uint32        // interned sequence of ordered child shapes
}

type componentLink struct {
	prefix, element uint32
}

type componentVariable struct {
	expression     ast.ASTHandle
	constraints    []int
	representative uint32
	mixed          bool
}

type polynomialComponents struct {
	manager    *ast.ASTManager
	arithmetic ast.FamilyID
	terms      map[ast.ASTHandle]componentTerm
	shapes     map[componentShape]uint32
	sequences  map[componentLink]uint32
	variables  []componentVariable
	work       int
}

// A sequence ID denotes an exact list. Go map equality checks both fields.
func componentAppend(table map[componentLink]uint32, prefix, element uint32) uint32 {
	link := componentLink{prefix, element}
	if id := table[link]; id != 0 {
		return id
	}
	id := uint32(len(table) + 1)
	table[link] = id
	return id
}

func (p *polynomialComponents) scan(h ast.ASTHandle, depth int) (term componentTerm, ok bool, err error) {
	if term, found := p.terms[h]; found {
		return term, true, nil
	}
	m := p.manager
	if depth > maxCandidateDepth || p.work >= maxCandidateEdges || m.Limit.CancelFlag() {
		return term, false, nil
	}
	hd, err := m.NodeHeader(h)
	if err != nil || hd.Kind != ast.ASTApp || uint64(hd.NumArgs)+1 > uint64(maxCandidateEdges-p.work) {
		return term, false, err
	}
	p.work += int(hd.NumArgs) + 1
	key := componentShape{head: hd.Decl}
	switch hd.Family {
	case ast.NullFamilyID:
		if hd.NumArgs != 0 {
			return term, false, nil
		}
		s, err := m.GetSort(h)
		if err != nil {
			return term, false, err
		}
		family, err := m.GetFamilyID(s)
		if err != nil || family != p.arithmetic {
			return term, false, err
		}
		kind, err := m.GetDeclKind(s)
		if err != nil || kind < 0 || kind > 1 {
			return term, false, err
		}
		key.head = s
		term.variable = uint32(len(p.variables) + 1)
		p.variables = append(p.variables, componentVariable{expression: h})
	case m.BasicFamilyID():
		if hd.DeclKind < ast.ASTOpTrue || hd.DeclKind > ast.ASTOpImplies {
			return term, false, nil
		}
	case p.arithmetic:
		// Numerals, arithmetic operations, comparisons and Int/Real coercions.
		if hd.DeclKind < candidateNumeralOp || hd.DeclKind > candidatePower {
			return term, false, nil
		}
	default:
		return term, false, nil
	}
	for i := uint32(0); i < hd.NumArgs; i++ {
		a, err := m.GetArg(h, i)
		if err != nil {
			return term, false, err
		}
		child, ok, err := p.scan(a, depth+1)
		if err != nil || !ok {
			return term, false, err
		}
		key.arguments = componentAppend(p.sequences, key.arguments, child.shape)
		term.variable = p.combineVariables(term.variable, child.variable)
	}
	term.shape = p.shapes[key]
	if term.shape == 0 {
		term.shape = uint32(len(p.shapes) + 1)
		p.shapes[key] = term.shape
	}
	p.terms[h] = term
	return term, true, nil
}

func (p *polynomialComponents) combineVariables(a, b uint32) uint32 {
	switch {
	case a == 0:
		return b
	case b == 0 || a == b:
		return a
	}
	// Variables in a multivariate child were marked when that child was built.
	if a != mixedComponent {
		p.variables[a-1].mixed = true
	}
	if b != mixedComponent {
		p.variables[b-1].mixed = true
	}
	return mixedComponent
}

func (p *polynomialComponents) group() int {
	sets := make(map[componentLink]uint32)
	owners := make(map[uint32]uint32)
	aliases := 0
	for i := range p.variables {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return 0
		}
		v := &p.variables[i]
		if v.mixed || len(v.constraints) == 0 {
			continue
		}
		sort.Ints(v.constraints)
		var key, previous uint32
		for _, k := range v.constraints {
			shape := uint32(k)
			if shape != previous {
				key = componentAppend(sets, key, shape)
				previous = shape
			}
		}
		if owner := owners[key]; owner != 0 {
			v.representative = owner
			aliases++
		} else {
			owners[key] = uint32(i + 1)
		}
	}
	return aliases
}

func (p *polynomialComponents) publish(t *NLSATTactic, goal *basetactic.Goal, result *basetactic.TacticResult, forms []ast.ASTHandle) (_ bool, err error) {
	next, err := basetactic.NewGoalConfigCopy(goal)
	if err != nil {
		return false, err
	}
	defer func() {
		if next != nil {
			err = errors.Join(err, next.Close())
		}
	}()
	if err = next.IncDepth(); err != nil {
		return false, err
	}
	cores, err := goal.UnsatCoresEnabled()
	if err != nil {
		return false, err
	}
	for i, form := range forms {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return false, nil
		}
		v := p.terms[form].variable
		if v != 0 && v != mixedComponent && p.variables[v-1].representative != 0 {
			continue
		}
		var dep ast.ASTDependency
		if cores {
			dep, err = goal.Dependency(uint32(i))
			if err != nil {
				return false, err
			}
		}
		if err = next.AssertWithProof(form, ast.ASTHandle{}, dep); err != nil {
			return false, err
		}
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return false, err
	}
	if models {
		var converter *basetactic.ExtensionModelConverter
		converter, err = basetactic.NewExtensionModel(p.manager, t.services)
		if err != nil {
			return false, err
		}
		defer func() {
			if converter != nil {
				err = errors.Join(err, converter.Close())
			}
		}()
		for i, v := range p.variables {
			if i%64 == 0 && p.manager.Limit.CancelFlag() {
				return false, nil
			}
			if v.representative == 0 {
				continue
			}
			decl, err := p.manager.GetDecl(v.expression)
			if err != nil {
				return false, err
			}
			if err = converter.Insert(decl, p.variables[v.representative-1].expression); err != nil {
				return false, err
			}
		}
		if err = result.SetModelConverter(converter); err != nil {
			return false, err
		}
		converter = nil
	}
	if err = result.TakeGoal(next); err != nil {
		return false, err
	}
	next = nil
	return true, nil
}
