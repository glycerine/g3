package transforms

// Source: src/muz/transforms/dl_mk_array_blast.cpp/.h retained local-store
// substitution and Ackermann select elimination.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

var errArrayBlastRefused = errors.New("muz/transforms: array blast refused")

type ArrayBlastPlugin struct {
	manager    *ast.ASTManager
	rules      *base.RuleManager
	rewriter   *astrewriter.TheoryRewriter
	simplifier *InterpTailSimplifierPlugin
	mkRule     ArrayInstantiationRuleMaker
	priority   uint32
	closed     bool
	closing    bool
}

func NewArrayBlastPlugin(manager *ast.ASTManager, rules *base.RuleManager, contextRewriter *astrewriter.TheoryRewriter, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (_ *ArrayBlastPlugin, resultErr error) {
	if manager == nil || rules == nil || context == nil || globals == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	local, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Del(&local)) }()
	name := "expand_select_store"
	if err = context.SetBoolName(&local, &name, true); err != nil {
		return nil, err
	}
	rewriter, err := astrewriter.NewTheoryRewriter(manager, context, globals, &local)
	if err != nil {
		return nil, err
	}
	simplifier, err := newNestedInterpTailSimplifier(manager, rules, contextRewriter, context, globals, priority)
	if err != nil {
		return nil, errors.Join(err, rewriter.Close())
	}
	return &ArrayBlastPlugin{manager: manager, rules: rules, rewriter: rewriter, simplifier: simplifier, priority: priority}, nil
}

func (p *ArrayBlastPlugin) Priority() uint32            { return p.priority }
func (p *ArrayBlastPlugin) CanDestratifyNegation() bool { return false }
func (p *ArrayBlastPlugin) Cancel()                     {}

func (p *ArrayBlastPlugin) arrayApplication(expression ast.ASTHandle, operation int32) (bool, error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	arrayFamily, err := p.manager.ArrayFamilyID()
	if err != nil || family != arrayFamily {
		return false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	return op == operation, err
}

func (p *ArrayBlastPlugin) equality(expression ast.ASTHandle) (left, right ast.ASTHandle, yes bool, err error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return left, right, false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil || family != p.manager.BasicFamilyID() {
		return left, right, false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	if err != nil || (op != ast.ASTOpEq && op != ast.ASTOpIff) {
		return left, right, false, err
	}
	count, err := p.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return left, right, false, err
	}
	left, err = p.manager.GetArg(expression, 0)
	if err == nil {
		right, err = p.manager.GetArg(expression, 1)
	}
	return left, right, err == nil, err
}

func (p *ArrayBlastPlugin) storeDefinition(expression ast.ASTHandle) (variable, store ast.ASTHandle, yes bool, err error) {
	left, right, equal, err := p.equality(expression)
	if err != nil || !equal {
		return variable, store, false, err
	}
	rightStore, err := p.arrayApplication(right, ast.ASTArrayOpStore)
	if err != nil {
		return variable, store, false, err
	}
	if !rightStore {
		left, right = right, left
	}
	leftKind, err := p.manager.GetKind(left)
	if err != nil {
		return variable, store, false, err
	}
	rightStore, err = p.arrayApplication(right, ast.ASTArrayOpStore)
	return left, right, leftKind == ast.ASTVar && rightStore, err
}

func (p *ArrayBlastPlugin) variables(expression ast.ASTHandle) (map[uint32]bool, error) {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return nil, err
	}
	if err = used.Process(expression); err != nil {
		return nil, err
	}
	result := make(map[uint32]bool)
	for i := uint32(0); i < used.MaxFoundVarIndexPlusOne(); i++ {
		if !used.Contains(i).IsNull() {
			result[i] = true
		}
	}
	return result, nil
}

func (p *ArrayBlastPlugin) disjoint(left, right ast.ASTHandle) (bool, error) {
	lhs, err := p.variables(left)
	if err != nil {
		return false, err
	}
	rhs, err := p.variables(right)
	if err != nil {
		return false, err
	}
	for variable := range lhs {
		if rhs[variable] {
			return false, nil
		}
	}
	return true, nil
}

func (p *ArrayBlastPlugin) rewrite(expression ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	result, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = p.rewriter.Rewrite(expression, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&result))
	}
	return result, nil
}

func (p *ArrayBlastPlugin) maximumVariable(rule *base.Rule) (uint32, error) {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return 0, err
	}
	head, err := rule.Head()
	if err == nil {
		err = used.Process(head)
	}
	count, x := rule.TailSize()
	if err == nil {
		err = x
	}
	for i := 0; err == nil && i < count; i++ {
		tail, _, getErr := rule.Tail(i)
		if getErr != nil {
			err = getErr
		} else {
			err = used.Process(tail)
		}
	}
	return used.MaxFoundVarIndexPlusOne(), err
}

func (p *ArrayBlastPlugin) selectBase(expression ast.ASTHandle) (ast.ASTHandle, error) {
	for {
		selectTerm, err := p.arrayApplication(expression, ast.ASTArrayOpSelect)
		if err != nil || !selectTerm {
			return expression, err
		}
		expression, err = p.manager.GetArg(expression, 0)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
}

func (p *ArrayBlastPlugin) selectIndices(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	var result []ast.ASTHandle
	for {
		selectTerm, err := p.arrayApplication(expression, ast.ASTArrayOpSelect)
		if err != nil || !selectTerm {
			return result, err
		}
		count, err := p.manager.GetNumArgs(expression)
		if err != nil {
			return nil, err
		}
		for i := uint32(1); i < count; i++ {
			index, getErr := p.manager.GetArg(expression, i)
			if getErr != nil {
				return nil, getErr
			}
			result = append(result, index)
		}
		expression, err = p.manager.GetArg(expression, 0)
		if err != nil {
			return nil, err
		}
	}
}

func (p *ArrayBlastPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.closing || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
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
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	modified := false
	for i := 0; i < count; i++ {
		canceled, cancelErr := p.manager.Canceled()
		if cancelErr != nil {
			return nil, cancelErr
		}
		if canceled {
			break
		}
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		changed, transformErr := p.blast(rule, result)
		modified = modified || changed
		transformErr = errors.Join(transformErr, rule.Close())
		if transformErr != nil {
			return nil, fmt.Errorf("muz/transforms: array blast rule %d: %w", i, transformErr)
		}
	}
	if !modified {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *ArrayBlastPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	if p.closed {
		return nil
	}
	p.closing = true
	if p.simplifier != nil {
		if err := p.simplifier.Close(); err != nil {
			return err
		}
		p.simplifier = nil
	}
	if p.rewriter != nil {
		if err := p.rewriter.Close(); err != nil {
			return err
		}
		p.rewriter = nil
	}
	p.closed = true
	return nil
}
