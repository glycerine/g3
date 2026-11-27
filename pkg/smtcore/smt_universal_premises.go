package smtcore

import (
	"slices"

	"github.com/glycerine/g3/pkg/ast"
)

// UniversalPremises indexes consequences of the original assertions. Each
// entry follows from a single universally quantified premise (or a negated
// existential). The index holds weak arena handles owned by one solver batch.
type UniversalPremises struct {
	manager *ast.ASTManager
	base    []ast.ASTHandle
	rules   map[signedDeclaration][]universalRule
	matched bool
}

type signedDeclaration struct {
	decl     ast.ASTHandle
	positive bool
}

type universalRule struct {
	body, quantifier ast.ASTHandle
	variables        uint32
}

const maxUniversalVariables = 16

func (p *UniversalPremises) spend(work *int, depth int) bool {
	if *work == 0 || depth > 64 {
		return false
	}
	if *work%256 == 0 && p.manager.ResourceLimit().CancelFlag() {
		*work = 0
		return false
	}
	*work--
	return true
}

// Refutes checks whether assuming the last, ground assertion would contradict
// a universal instance. Before the first physical push, a match can avoid
// initializing the incremental solver. Once search has started, let the usual
// path see the first match for this base so later siblings can reuse its work.
func (p *UniversalPremises) Refutes(m *ast.ASTManager, assertions []ast.ASTHandle, steps uint32, beforePush bool) (bool, error) {
	if len(assertions) < 2 || len(assertions) > 8192 || steps < 150_000 || m.ResourceLimit().CancelFlag() {
		return false, nil
	}
	goal := assertions[len(assertions)-1]
	ground, err := m.IsGround(goal)
	if err != nil || !ground {
		return false, err
	}
	base := assertions[:len(assertions)-1]
	if p.manager != m || !slices.Equal(p.base, base) {
		*p = UniversalPremises{manager: m, base: slices.Clone(base)}
		work := int(min(steps/8, 100_000))
		for _, premise := range base {
			if work == 0 {
				break
			}
			if err := p.collect(premise, true, universalRule{}, 0, &work); err != nil || m.ResourceLimit().CancelFlag() {
				*p = UniversalPremises{}
				return false, err
			}
		}
	}
	work := 20_000
	if !p.entails(goal, false, 0, &work) || m.ResourceLimit().CancelFlag() {
		return false, nil
	}
	use := beforePush || p.matched
	p.matched = true
	return use, nil
}

// Visit only premise positions that must hold with the given polarity. For
// example, a false implication entails its antecedent and negated consequent.
func (p *UniversalPremises) collect(h ast.ASTHandle, positive bool, rule universalRule, depth int, work *int) error {
	if !p.spend(work, depth) {
		return nil
	}
	m := p.manager
	head, err := m.NodeHeader(h)
	if err != nil {
		return err
	}
	if head.Kind == ast.ASTQuantifier && rule.quantifier.IsNull() {
		all, err := m.IsForall(h)
		if err != nil || all != positive {
			return err
		}
		n, err := m.GetNumDeclarations(h)
		if err != nil || n > maxUniversalVariables {
			return err
		}
		body, err := m.GetQuantifierBody(h)
		if err != nil {
			return err
		}
		return p.collect(body, positive, universalRule{quantifier: h, variables: n}, depth+1, work)
	}
	if head.Kind != ast.ASTApp {
		return nil
	}
	basic := head.Family == m.BasicFamilyID()
	if basic && head.DeclKind == ast.ASTOpNot && head.NumArgs == 1 {
		child, err := m.GetArg(h, 0)
		if err != nil {
			return err
		}
		return p.collect(child, !positive, rule, depth+1, work)
	}
	if !rule.quantifier.IsNull() {
		if p.rules == nil {
			p.rules = make(map[signedDeclaration][]universalRule)
		}
		key := signedDeclaration{head.Decl, positive}
		rule.body = h
		p.rules[key] = append(p.rules[key], rule)
	}
	if !basic {
		return nil
	}
	implication := head.DeclKind == ast.ASTOpImplies && head.NumArgs == 2
	if positive && head.DeclKind == ast.ASTOpAnd || !positive && (head.DeclKind == ast.ASTOpOr || implication) {
		for i := uint32(0); i < head.NumArgs && *work > 0 && depth < 64; i++ {
			child, err := m.GetArg(h, i)
			if err != nil {
				return err
			}
			if err := p.collect(child, positive || implication && i == 0, rule, depth+1, work); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *UniversalPremises) entails(h ast.ASTHandle, positive bool, depth int, work *int) bool {
	if !p.spend(work, depth) {
		return false
	}
	m := p.manager
	head, err := m.NodeHeader(h)
	if err != nil || head.Kind != ast.ASTApp {
		return false
	}
	basic := head.Family == m.BasicFamilyID()
	if basic {
		switch head.DeclKind {
		case ast.ASTOpTrue, ast.ASTOpFalse:
			return positive == (head.DeclKind == ast.ASTOpTrue)
		case ast.ASTOpNot:
			if head.NumArgs == 1 {
				child, err := m.GetArg(h, 0)
				return err == nil && p.entails(child, !positive, depth+1, work)
			}
		}
	}
	for _, rule := range p.rules[signedDeclaration{head.Decl, positive}] {
		var bindings [maxUniversalVariables]ast.ASTHandle
		if p.match(rule.body, h, rule, &bindings, 0, work) {
			return true
		}
		if *work == 0 {
			return false
		}
	}
	// Look for one sufficient child. Different disjuncts can use different
	// instances. An unmatched conjunction cannot establish all its conjuncts.
	implication := head.DeclKind == ast.ASTOpImplies && head.NumArgs == 2
	if basic && (!positive && head.DeclKind == ast.ASTOpAnd || positive && (head.DeclKind == ast.ASTOpOr || implication)) {
		for i := uint32(0); i < head.NumArgs && *work > 0 && depth < 64; i++ {
			child, err := m.GetArg(h, i)
			if err == nil && p.entails(child, positive && !(implication && i == 0), depth+1, work) {
				return true
			}
		}
	}
	return false
}

// The target and all its descendants are ground. Only variables belonging to
// this binder may match a term. Repeated variables must match the same term.
func (p *UniversalPremises) match(pattern, target ast.ASTHandle, rule universalRule, bindings *[maxUniversalVariables]ast.ASTHandle, depth int, work *int) bool {
	if !p.spend(work, depth) {
		return false
	}
	if pattern == target {
		return true
	}
	m := p.manager
	head, err := m.NodeHeader(pattern)
	if err != nil {
		return false
	}
	if head.Kind == ast.ASTVar {
		i, err := m.GetVarIndex(pattern)
		if err != nil || i >= rule.variables {
			return false
		}
		formal, err := m.GetDeclarationSort(rule.quantifier, rule.variables-1-i)
		if err != nil {
			return false
		}
		sort, err := m.GetSort(pattern)
		if err != nil || sort != formal {
			return false
		}
		if !bindings[i].IsNull() {
			return bindings[i] == target
		}
		sort, err = m.GetSort(target)
		if err != nil || sort != formal {
			return false
		}
		bindings[i] = target
		return true
	}
	if head.Kind != ast.ASTApp || head.NumArgs == 0 || depth == 64 {
		return false
	}
	other, err := m.NodeHeader(target)
	if err != nil || other.Kind != ast.ASTApp || head.Decl != other.Decl || head.NumArgs != other.NumArgs {
		return false
	}
	for i := uint32(0); i < head.NumArgs; i++ {
		formal, err := m.GetArg(pattern, i)
		if err != nil {
			return false
		}
		actual, err := m.GetArg(target, i)
		if err != nil || !p.match(formal, actual, rule, bindings, depth+1, work) {
			return false
		}
	}
	return true
}
