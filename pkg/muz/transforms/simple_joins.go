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

type simpleJoinPlan struct {
	outputs    map[normalizedPairKey][]uint32
	costs      map[normalizedPairKey]float32
	consumers  map[normalizedPairKey]uint32
	stratified map[normalizedPairKey]bool
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

func (p *SimpleJoinsPlugin) auxiliary(pair normalizedPair, outside []ast.ASTHandle, prefix usymbol.Symbol, parent *base.Rule, shared map[sharedJoinKey]sharedJoinDefinition, globalOutputs map[normalizedPairKey][]uint32) (ast.ASTHandle, *base.Rule, error) {
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
	if union, found := globalOutputs[pair.key]; found {
		outputs = append(outputs[:0], union...)
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

// collectPairPlan performs the source planner's pre-registration pass: every
// rule contributes non-local variables, the first estimated pair cost, and
// stratification/consumer accounting before pair selection starts.
func (p *SimpleJoinsPlugin) collectPairPlan(source *base.RuleSet) (_ *simpleJoinPlan, resultErr error) {
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	sets := make(map[normalizedPairKey]map[uint32]struct{})
	plan := &simpleJoinPlan{
		outputs: make(map[normalizedPairKey][]uint32), costs: make(map[normalizedPairKey]float32),
		consumers: make(map[normalizedPairKey]uint32), stratified: make(map[normalizedPairKey]bool),
	}
	for ruleIndex := 0; ruleIndex < count; ruleIndex++ {
		rule, x := source.Rule(ruleIndex)
		if x != nil {
			return nil, x
		}
		funcErr := func() error {
			positive, x := rule.PositiveTailSize()
			if x != nil {
				return x
			}
			head, x := rule.Head()
			if x != nil {
				return x
			}
			tailCount, x := rule.TailSize()
			if x != nil {
				return x
			}
			tails := make([]ast.ASTHandle, tailCount)
			for i := range tails {
				tails[i], _, x = rule.Tail(i)
				if x != nil {
					return x
				}
			}
			for left := 0; left < positive; left++ {
				for right := left + 1; right < positive; right++ {
					pair, y := p.normalizePair(tails[left], tails[right])
					if y != nil {
						return y
					}
					outside := []ast.ASTHandle{head}
					for i, tail := range tails {
						if i != left && i != right {
							outside = append(outside, tail)
						}
					}
					live := make(map[uint32]ast.ASTHandle)
					for _, expression := range outside {
						if y = p.variables(expression, live); y != nil {
							return y
						}
					}
					if _, seen := plan.costs[pair.key]; !seen {
						cost, costErr := p.estimatedPairCost(tails[:positive], outside, left, right)
						if costErr != nil {
							return costErr
						}
						plan.costs[pair.key] = cost
						plan.stratified[pair.key] = true
					}
					if positive > 2 {
						plan.consumers[pair.key]++
					}
					leftDecl, y := p.manager.GetDecl(tails[left])
					if y != nil {
						return y
					}
					rightDecl, y := p.manager.GetDecl(tails[right])
					if y != nil {
						return y
					}
					headDecl, y := p.manager.GetDecl(head)
					if y != nil {
						return y
					}
					leftStratum, leftErr := source.PredicateStratum(leftDecl)
					rightStratum, rightErr := source.PredicateStratum(rightDecl)
					headStratum, headErr := source.PredicateStratum(headDecl)
					if leftErr != nil || rightErr != nil || headErr != nil {
						plan.stratified[pair.key] = false
					} else {
						sourceStratum := leftStratum
						if rightStratum > sourceStratum {
							sourceStratum = rightStratum
						}
						if headStratum <= sourceStratum {
							plan.stratified[pair.key] = false
						}
					}
					output := sets[pair.key]
					if output == nil {
						output = make(map[uint32]struct{})
						sets[pair.key] = output
					}
					for variable, canonical := range pair.actualToCanonical {
						if _, needed := live[variable]; needed {
							output[canonical] = struct{}{}
						}
					}
				}
			}
			return nil
		}()
		funcErr = errors.Join(funcErr, rule.Close())
		if funcErr != nil {
			return nil, funcErr
		}
	}
	result := make(map[normalizedPairKey][]uint32, len(sets))
	for key, set := range sets {
		outputs := make([]uint32, 0, len(set))
		for output := range set {
			outputs = append(outputs, output)
		}
		sort.Slice(outputs, func(i, j int) bool { return outputs[i] < outputs[j] })
		result[key] = outputs
	}
	plan.outputs = result
	return plan, nil
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
	plan, err := p.collectPairPlan(source)
	if err != nil {
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
			leftIndex, rightIndex, y := p.bestPair(tails[:positive], outside, plan)
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
			auxiliary, definition, y := p.auxiliary(pair, outside, prefix, rule, shared, plan.outputs)
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
