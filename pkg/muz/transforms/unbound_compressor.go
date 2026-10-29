package transforms

// Source: src/muz/transforms/dl_mk_unbound_compressor.{h,cpp}.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type compressionKey struct {
	predicate ast.ASTHandle
	argument  uint32
}

type UnboundCompressorPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	enabled  bool
	priority uint32
	mapping  map[compressionKey]ast.ASTHandle
	closed   bool
}

func NewUnboundCompressorPlugin(manager *ast.ASTManager, rules *base.RuleManager, enabled bool, priority uint32) *UnboundCompressorPlugin {
	if priority == 0 {
		priority = 500
	}
	return &UnboundCompressorPlugin{manager: manager, rules: rules, enabled: enabled, priority: priority, mapping: make(map[compressionKey]ast.ASTHandle)}
}

func (p *UnboundCompressorPlugin) Priority() uint32            { return p.priority }
func (p *UnboundCompressorPlugin) CanDestratifyNegation() bool { return false }
func (p *UnboundCompressorPlugin) Cancel()                     {}

func (p *UnboundCompressorPlugin) CompressedPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle)
	for key, declaration := range p.mapping {
		result[key.predicate] = declaration
	}
	return result
}

func (p *UnboundCompressorPlugin) headTask(source *base.RuleSet, rule *base.Rule) (compressionKey, bool, error) {
	declaration, err := rule.HeadDeclaration()
	if err != nil {
		return compressionKey{}, false, err
	}
	output, err := source.IsOutputPredicate(declaration)
	if err != nil || output {
		return compressionKey{}, false, err
	}
	head, err := rule.Head()
	if err != nil {
		return compressionKey{}, false, err
	}
	tailVariables, err := p.rules.TailVariables(rule)
	if err != nil {
		return compressionKey{}, false, err
	}
	count, err := p.manager.GetNumArgs(head)
	if err != nil {
		return compressionKey{}, false, err
	}
	occurrences := make(map[uint32]uint32)
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(head, index)
		if getErr != nil {
			return compressionKey{}, false, getErr
		}
		if kind, x := p.manager.GetKind(argument); x == nil && kind == ast.ASTVar {
			variable, x := p.manager.GetVarIndex(argument)
			if x != nil {
				return compressionKey{}, false, x
			}
			occurrences[variable]++
		}
	}
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(head, index)
		if getErr != nil {
			return compressionKey{}, false, getErr
		}
		kind, getErr := p.manager.GetKind(argument)
		if getErr != nil || kind != ast.ASTVar {
			continue
		}
		variable, getErr := p.manager.GetVarIndex(argument)
		if getErr != nil {
			return compressionKey{}, false, getErr
		}
		if tailVariables[variable].IsNull() && occurrences[variable] == 1 {
			return compressionKey{predicate: declaration, argument: index}, true, nil
		}
	}
	return compressionKey{}, false, nil
}

func (p *UnboundCompressorPlugin) declare(key compressionKey) (ast.ASTHandle, error) {
	if declaration := p.mapping[key]; !declaration.IsNull() {
		return declaration, nil
	}
	arity, err := p.manager.GetArity(key.predicate)
	if err != nil || arity == 0 || key.argument >= arity {
		return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
	}
	domain := make([]ast.ASTHandle, 0, arity-1)
	for index := uint32(0); index < arity; index++ {
		if index == key.argument {
			continue
		}
		sort, getErr := p.manager.GetDomain(key.predicate, index)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		domain = append(domain, sort)
	}
	name, err := p.manager.GetASTName(key.predicate)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	suffixText := fmt.Sprintf("compr_arg_%d", key.argument)
	suffix, err := p.manager.Symbols.Intern(suffixText)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := p.manager.MkFreshFuncDecl(name, suffix, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.rules.RegisterPredicate(declaration); err != nil {
		return ast.ASTHandle{}, err
	}
	p.mapping[key] = declaration
	return declaration, nil
}

func (p *UnboundCompressorPlugin) compressedApplication(application ast.ASTHandle, key compressionKey) (ast.ASTHandle, error) {
	declaration := p.mapping[key]
	if declaration.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	count, err := p.manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0, count-1)
	for index := uint32(0); index < count; index++ {
		if index == key.argument {
			continue
		}
		argument, getErr := p.manager.GetArg(application, index)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		arguments = append(arguments, argument)
	}
	return p.manager.MkApp(declaration, arguments)
}

func (p *UnboundCompressorPlugin) rewriteRule(rule *base.Rule) (*base.Rule, bool, error) {
	head, err := rule.Head()
	if err != nil {
		return nil, false, err
	}
	headDeclaration, err := rule.HeadDeclaration()
	if err != nil {
		return nil, false, err
	}
	changed := false
	for key := range p.mapping {
		if key.predicate == headDeclaration {
			head, err = p.compressedApplication(head, key)
			if err != nil {
				return nil, false, err
			}
			changed = true
			break
		}
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, false, err
	}
	tails := make([]ast.ASTHandle, 0, total+1)
	negated := make([]bool, 0, total+1)
	for index := 0; index < total; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, getErr
		}
		declaration, getErr := p.manager.GetDecl(tail)
		if getErr != nil {
			return nil, false, getErr
		}
		rewritten := false
		for key := range p.mapping {
			if key.predicate != declaration {
				continue
			}
			compressed, x := p.compressedApplication(tail, key)
			if x != nil {
				return nil, false, x
			}
			if negative {
				tails, negated = append(tails, tail, compressed), append(negated, true, true)
			} else {
				tails, negated = append(tails, compressed), append(negated, false)
			}
			changed, rewritten = true, true
			break
		}
		if !rewritten {
			tails, negated = append(tails, tail), append(negated, negative)
		}
	}
	if !changed {
		retained, retainErr := rule.Retain()
		return retained, false, retainErr
	}
	name, err := rule.Name()
	if err != nil {
		return nil, false, err
	}
	replacement, err := p.rules.NewRule(head, tails, negated, name)
	if err != nil {
		return nil, false, err
	}
	if err = replacement.SetAccountingParentRule(rule); err != nil {
		return nil, false, errors.Join(err, replacement.Close())
	}
	return replacement, true, nil
}

func (p *UnboundCompressorPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	if !p.enabled {
		return nil, nil
	}
	p.mapping = make(map[compressionKey]ast.ASTHandle)
	count, err := source.Len()
	for index := 0; err == nil && index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		key, found, getErr := p.headTask(source, rule)
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
		if found {
			_, err = p.declare(key)
		}
	}
	if err != nil || len(p.mapping) == 0 {
		return nil, err
	}
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
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		replacement, _, getErr := p.rewriteRule(rule)
		if getErr == nil {
			getErr = result.Add(replacement)
		}
		if replacement != nil {
			getErr = errors.Join(getErr, replacement.Close())
		}
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	for key, declaration := range p.mapping {
		if err = result.InheritPredicate(source, key.predicate, declaration); err != nil {
			return nil, err
		}
	}
	keep = true
	return result, nil
}

func (p *UnboundCompressorPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed, p.mapping = true, nil
	return nil
}
