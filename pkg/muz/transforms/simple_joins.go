package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp and .h retained rule-shape
// normalization to at most two positive predicate tails.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SimpleJoinsPlugin struct {
	manager   *ast.ASTManager
	rules     *base.RuleManager
	relations *rel.RelationManager
	priority  uint32
	fresh     []ast.ASTHandle
	pins      []ast.ASTRef
	closed    bool
}

type sharedJoinKey struct {
	pair    normalizedPairKey
	outputs string
}

type sharedJoinDefinition struct {
	declaration ast.ASTHandle
	outputs     []uint32
}

func NewSimpleJoinsPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32, relations ...*rel.RelationManager) *SimpleJoinsPlugin {
	if priority == 0 {
		priority = 1000
	}
	plugin := &SimpleJoinsPlugin{manager: manager, rules: rules, priority: priority}
	if len(relations) > 0 {
		plugin.relations = relations[0]
	}
	return plugin
}

func (p *SimpleJoinsPlugin) Priority() uint32            { return p.priority }
func (p *SimpleJoinsPlugin) CanDestratifyNegation() bool { return false }
func (p *SimpleJoinsPlugin) Cancel()                     {}

func (p *SimpleJoinsPlugin) FreshPredicates() []ast.ASTHandle {
	return append([]ast.ASTHandle(nil), p.fresh...)
}

func (p *SimpleJoinsPlugin) variables(root ast.ASTHandle, result map[uint32]ast.ASTHandle) error {
	kind, err := p.manager.GetKind(root)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		index, x := p.manager.GetVarIndex(root)
		if x != nil {
			return x
		}
		result[index], x = p.manager.GetSort(root)
		return x
	}
	if kind != ast.ASTApp {
		return nil
	}
	count, err := p.manager.GetNumArgs(root)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		child, x := p.manager.GetArg(root, index)
		if x != nil {
			return x
		}
		if x = p.variables(child, result); x != nil {
			return x
		}
	}
	return nil
}

func (p *SimpleJoinsPlugin) auxiliary(pair normalizedPair, outside []ast.ASTHandle, prefix usymbol.Symbol, parent *base.Rule, shared map[sharedJoinKey]sharedJoinDefinition) (ast.ASTHandle, *base.Rule, error) {
	var err error
	live := make(map[uint32]ast.ASTHandle)
	for _, expression := range outside {
		if err := p.variables(expression, live); err != nil {
			return ast.ASTHandle{}, nil, err
		}
	}
	outputs := make([]uint32, 0)
	for variable := range pair.actualToCanonical {
		if _, found := live[variable]; found {
			outputs = append(outputs, pair.actualToCanonical[variable])
		}
	}
	sort.Slice(outputs, func(i, j int) bool { return outputs[i] < outputs[j] })
	key := sharedJoinKey{pair: pair.key, outputs: fmt.Sprint(outputs)}
	entry, found := shared[key]
	var definition *base.Rule
	if !found {
		domain := make([]ast.ASTHandle, len(outputs))
		canonicalArguments := make([]ast.ASTHandle, len(outputs))
		for i, output := range outputs {
			domain[i] = pair.canonicalSorts[output]
			canonicalArguments[i], err = p.manager.MkVar(output, domain[i])
			if err != nil {
				return ast.ASTHandle{}, nil, err
			}
		}
		boolean, err := p.manager.MkBoolSort()
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		suffix, err := p.manager.Symbols.Intern("split")
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		declaration, err := p.manager.MkFreshFuncDecl(prefix, suffix, domain, boolean)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if err = p.rules.RegisterPredicate(declaration); err != nil {
			return ast.ASTHandle{}, nil, err
		}
		p.fresh = append(p.fresh, declaration)
		canonicalHead, err := p.manager.MkApp(declaration, canonicalArguments)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		definition, err = p.rules.NewRule(canonicalHead, pair.terms[:], nil, "")
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if parent != nil {
			if err = definition.SetAccountingParentRule(parent); err != nil {
				return ast.ASTHandle{}, nil, errors.Join(err, definition.Close())
			}
		}
		if err = p.attachAssertedProof(definition); err != nil {
			return ast.ASTHandle{}, nil, errors.Join(err, definition.Close())
		}
		entry = sharedJoinDefinition{declaration: declaration, outputs: append([]uint32(nil), outputs...)}
		shared[key] = entry
	}
	arguments := make([]ast.ASTHandle, len(entry.outputs))
	for i, output := range entry.outputs {
		actual := pair.canonicalToActual[output]
		arguments[i], err = p.manager.MkVar(actual, pair.canonicalSorts[output])
		if err != nil {
			return ast.ASTHandle{}, definition, err
		}
	}
	head, err := p.manager.MkApp(entry.declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, definition, err
	}
	pin, err := p.manager.NewRef(head)
	if err != nil {
		return ast.ASTHandle{}, definition, err
	}
	p.pins = append(p.pins, pin)
	return head, definition, nil
}

func (p *SimpleJoinsPlugin) attachAssertedProof(rule *base.Rule) (resultErr error) {
	enabled, err := p.manager.ProofsEnabled()
	if err != nil || !enabled {
		return err
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&formula)) }()
	formulaHandle, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	proof, err := p.manager.MkProofAsserted(formulaHandle)
	if err != nil {
		return err
	}
	return rule.SetProof(proof)
}

func (p *SimpleJoinsPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	changed := false
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	var introduced []*base.Rule
	shared := make(map[sharedJoinKey]sharedJoinDefinition)
	defer func() {
		for _, definition := range introduced {
			if definition != nil {
				resultErr = errors.Join(resultErr, definition.Close())
			}
		}
	}()
	for index := 0; index < count; index++ {
		rule, x := source.Rule(index)
		if x != nil {
			return nil, x
		}
		positive, x := rule.PositiveTailSize()
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		if positive <= 2 {
			x = result.Add(rule)
			x = errors.Join(x, rule.Close())
			if x != nil {
				return nil, x
			}
			continue
		}
		changed = true
		head, x := rule.Head()
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		name, x := rule.Name()
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		headDeclaration, x := p.manager.GetDecl(head)
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		prefix, x := p.manager.GetASTName(headDeclaration)
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		tailCount, x := rule.TailSize()
		if x != nil {
			return nil, errors.Join(x, rule.Close())
		}
		tails := make([]ast.ASTHandle, tailCount)
		negated := make([]bool, tailCount)
		for i := range tails {
			tails[i], negated[i], x = rule.Tail(i)
			if x != nil {
				return nil, errors.Join(x, rule.Close())
			}
		}
		for positive > 2 {
			outside := []ast.ASTHandle{head}
			for i, tail := range tails {
				if i >= positive {
					outside = append(outside, tail)
				}
			}
			leftIndex, rightIndex, y := p.bestPair(source, head, tails[:positive], outside)
			if y != nil {
				return nil, errors.Join(y, rule.Close())
			}
			pair, y := p.normalizePair(tails[leftIndex], tails[rightIndex])
			if y != nil {
				return nil, errors.Join(y, rule.Close())
			}
			for i, tail := range tails[:positive] {
				if i != leftIndex && i != rightIndex {
					outside = append(outside, tail)
				}
			}
			auxiliary, definition, y := p.auxiliary(pair, outside, prefix, rule, shared)
			if y != nil {
				return nil, errors.Join(y, rule.Close())
			}
			if definition != nil {
				introduced = append(introduced, definition)
			}
			tails[leftIndex] = auxiliary
			negated[leftIndex] = false
			lastPositive := positive - 1
			if rightIndex != lastPositive {
				tails[rightIndex] = tails[lastPositive]
				negated[rightIndex] = negated[lastPositive]
			}
			tails = tails[:len(tails)-1]
			negated = negated[:len(negated)-1]
			positive--
		}
		replacement, x := p.rules.NewRule(head, tails, negated, name)
		if x == nil {
			x = replacement.SetAccountingParentRule(rule)
		}
		if x == nil {
			x = p.rules.MkRuleRewriteProof(rule, replacement)
		}
		if x == nil {
			x = result.Add(replacement)
			x = errors.Join(x, replacement.Close())
		}
		x = errors.Join(x, rule.Close())
		if x != nil {
			return nil, x
		}
	}
	for i := len(introduced) - 1; i >= 0; i-- {
		definition := introduced[i]
		x := result.Add(definition)
		x = errors.Join(x, definition.Close())
		introduced[i] = nil
		if x != nil {
			return nil, x
		}
	}
	if !changed {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *SimpleJoinsPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	var result error
	for index := len(p.pins) - 1; index >= 0; index-- {
		result = errors.Join(result, p.manager.DelRef(&p.pins[index]))
	}
	p.pins = nil
	p.fresh = nil
	return result
}
