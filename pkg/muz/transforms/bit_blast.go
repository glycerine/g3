package transforms

// Source: src/muz/transforms/dl_mk_bit_blast.cpp/.h retained Horn-rule
// bit blasting, predicate-signature expansion, and predicate mapping.

import (
	"errors"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	bitblaster "github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type bitBlastExpansion struct {
	astrewriter.DefaultConfiguration
	manager  *ast.ASTManager
	rules    *base.RuleManager
	source   *base.RuleSet
	target   *base.RuleSet
	oldToNew map[ast.ASTHandle]ast.ASTHandle
	// args, fVars, gVars are expand_mkbv_cfg's m_args, m_f_vars, m_g_vars:
	// reset on each reduce_app, released when the configuration dies.
	args, fVars, gVars []ast.ASTRef
}

func releaseExpansionVector(m *ast.ASTManager, refs *[]ast.ASTRef) (result error) {
	for i := range *refs {
		result = errors.Join(result, m.DelRef(&(*refs)[i]))
	}
	*refs = (*refs)[:0]
	return result
}

// release runs the member destructors in reverse declaration order.
func (c *bitBlastExpansion) release() error {
	err := releaseExpansionVector(c.manager, &c.gVars)
	err = errors.Join(err, releaseExpansionVector(c.manager, &c.fVars))
	return errors.Join(err, releaseExpansionVector(c.manager, &c.args))
}

func (c *bitBlastExpansion) push(refs *[]ast.ASTRef, h ast.ASTHandle) error {
	ref, err := c.manager.NewRef(h)
	if err == nil {
		*refs = append(*refs, ref)
	}
	return err
}

func (c *bitBlastExpansion) ReduceApp(declaration ast.ASTHandle, arguments ast.AppArguments, out, _ ast.ASTRef) (astrewriter.RewriteStatus, error) {
	if arguments.Count() == 0 {
		return astrewriter.RewriteFailed, nil
	}
	for i := uint32(0); i < arguments.Count(); i++ {
		argument, err := arguments.At(i)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		assembled, err := c.manager.IsBitVectorOperation(argument, 56)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		if !assembled {
			return astrewriter.RewriteFailed, nil
		}
	}
	err := errors.Join(releaseExpansionVector(c.manager, &c.args), releaseExpansionVector(c.manager, &c.gVars), releaseExpansionVector(c.manager, &c.fVars))
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	flat := make([]ast.ASTHandle, 0)
	var index uint32
	for i := uint32(0); i < arguments.Count(); i++ {
		argument, err := arguments.At(i)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		count, err := c.manager.GetNumArgs(argument)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		family, err := c.manager.GetFamilyID(argument)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		vars := make([]ast.ASTHandle, 0, count)
		for j := uint32(0); j < count; j++ {
			bit, getErr := c.manager.GetArg(argument, j)
			if getErr != nil {
				return astrewriter.RewriteFailed, getErr
			}
			flat = append(flat, bit)
			if getErr = c.push(&c.args, bit); getErr != nil {
				return astrewriter.RewriteFailed, getErr
			}
			variable, getErr := c.manager.MkVar(index, boolean)
			index++
			if getErr == nil {
				getErr = c.push(&c.gVars, variable)
			}
			if getErr != nil {
				return astrewriter.RewriteFailed, getErr
			}
			vars = append(vars, variable)
		}
		// m_f_vars.push_back(m_util.mk_bv(sz, ...)): unused by the result.
		assembled, err := c.manager.MkFamilyApp(family, 56, nil, vars)
		if err == nil {
			err = c.push(&c.fVars, assembled)
		}
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
	}
	fresh := c.oldToNew[declaration]
	if fresh.IsNull() {
		name, err := c.manager.GetASTName(declaration)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		suffix, err := c.manager.Symbols.Intern("bv")
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		domain := make([]ast.ASTHandle, len(flat))
		for i, bit := range flat {
			domain[i], err = c.manager.GetSort(bit)
			if err != nil {
				return astrewriter.RewriteFailed, err
			}
		}
		rangeSort, err := c.manager.GetRange(declaration)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		fresh, err = c.manager.MkFreshFuncDecl(name, suffix, domain, rangeSort)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		if err = c.rules.RegisterPredicate(fresh); err != nil {
			return astrewriter.RewriteFailed, err
		}
		if err = c.target.InheritPredicate(c.source, declaration, fresh); err != nil {
			return astrewriter.RewriteFailed, err
		}
		c.oldToNew[declaration] = fresh
	}
	application, err := c.manager.MkApp(fresh, flat)
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	if err = c.manager.SetRef(out, application); err != nil {
		return astrewriter.RewriteFailed, err
	}
	return astrewriter.RewriteDone, nil
}

type BitBlastPlugin struct {
	manager    *ast.ASTManager
	rules      *base.RuleManager
	simplifier *InterpTailSimplifierPlugin
	blaster    *bitblaster.BitBlasterRewriter
	expansion  bitBlastExpansion
	// expansionTraversal is m_rewriter during Transform.
	expansionTraversal *astrewriter.Traversal
	mkRule             func(destination *base.RuleSet, formula, proof ast.ASTHandle, name usymbol.Symbol) error
	priority           uint32
	closed             bool
	closing            bool
}

func bitBlastAssertedProof(manager *ast.ASTManager, formula ast.ASTHandle) (proof ast.ASTHandle, resultErr error) {
	mode, err := manager.ProofMode()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	changed := mode == ast.ASTProofDisabled
	if changed {
		if err = manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return ast.ASTHandle{}, err
		}
		defer func() { resultErr = errors.Join(resultErr, manager.ToggleProofMode(mode)) }()
	}
	return manager.MkProofAsserted(formula)
}

func NewBitBlastPlugin(manager *ast.ASTManager, rules *base.RuleManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (_ *BitBlastPlugin, resultErr error) {
	params := bitblaster.DefaultRewriterParams()
	params.BlastFull, params.BlastQuant = true, true
	blaster, err := bitblaster.NewBitBlasterRewriter(manager, &params)
	if err != nil {
		return nil, err
	}
	simplifier, err := NewInterpTailSimplifierPlugin(manager, rules, context, globals, priority)
	if err != nil {
		return nil, errors.Join(err, blaster.Close())
	}
	p := &BitBlastPlugin{manager: manager, rules: rules, simplifier: simplifier, blaster: blaster, priority: priority}
	p.expansion.manager, p.expansion.rules = manager, rules
	p.expansion.oldToNew = make(map[ast.ASTHandle]ast.ASTHandle)
	return p, nil
}

func (p *BitBlastPlugin) Priority() uint32            { return p.priority }
func (p *BitBlastPlugin) CanDestratifyNegation() bool { return false }
func (p *BitBlastPlugin) Cancel()                     {}

func (p *BitBlastPlugin) FreshPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle, len(p.expansion.oldToNew))
	for old, fresh := range p.expansion.oldToNew {
		result[old] = fresh
	}
	return result
}

func (p *BitBlastPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.closing || p.manager == nil || p.rules == nil || p.blaster == nil {
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
	p.expansion.source, p.expansion.target = source, result
	traversal, err := astrewriter.NewTraversal(p.manager, &p.expansion)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, traversal.Close()) }()
	p.expansionTraversal = traversal
	defer func() { p.expansionTraversal = nil }()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	// operator()'s fml lives across the loop: to_formula(*r, fml) replaces
	// it, and after a change it holds fml3 through mk_rule.
	var fml ast.ASTRef
	defer func() {
		if fml != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&fml))
		}
	}()
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
		changed, transformErr := p.blastRule(rule, &fml)
		if transformErr == nil {
			if changed {
				transformErr = p.mkBlastedRule(rule, fml, result)
			} else {
				transformErr = result.Add(rule)
			}
		}
		transformErr = errors.Join(transformErr, rule.Close())
		if transformErr != nil {
			return nil, transformErr
		}
	}
	keep = true
	return result, nil
}

// SetRuleMaker supplies rule_manager::mk_rule(fml, pr, rules, name).
func (p *BitBlastPlugin) SetRuleMaker(mk func(destination *base.RuleSet, formula, proof ast.ASTHandle, name usymbol.Symbol) error) {
	p.mkRule = mk
}

func (p *BitBlastPlugin) assignFormula(slot *ast.ASTRef, h ast.ASTHandle) error {
	fresh, err := p.manager.NewRef(h)
	if err != nil {
		return err
	}
	if *slot != (ast.ASTRef{}) {
		if err = p.manager.DelRef(slot); err != nil {
			return errors.Join(err, p.manager.DelRef(&fresh))
		}
	}
	*slot = fresh
	return nil
}

// blastRule is to_formula(*r, fml) followed by blast(r, fml).
func (p *BitBlastPlugin) blastRule(rule *base.Rule, fml *ast.ASTRef) (changed bool, resultErr error) {
	original, err := rule.ToFormula()
	if err != nil {
		return false, err
	}
	originalHandle, err := p.manager.RefGet(original)
	if err == nil {
		err = p.assignFormula(fml, originalHandle)
	}
	if err = errors.Join(err, p.manager.DelRef(&original)); err != nil {
		return false, err
	}
	// blast's locals die in reverse: r2, fml3, fml2, fml1.
	var fml1, fml2, fml3 ast.ASTRef
	var r2 *base.Rule
	defer func() {
		if r2 != nil && r2 != rule {
			resultErr = errors.Join(resultErr, r2.Close())
		}
		for _, r := range []*ast.ASTRef{&fml3, &fml2, &fml1} {
			if *r != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, p.manager.DelRef(r))
			}
		}
	}()
	simplified, dropped, simplifiedChanged, err := p.simplifier.transformRule(rule)
	if err != nil {
		return false, err
	}
	r2 = rule
	if !dropped && simplifiedChanged && simplified != nil {
		r2 = simplified
	} else if simplified != nil && simplified != rule {
		if err = simplified.Close(); err != nil {
			return false, err
		}
	}
	if fml1, err = r2.ToFormula(); err != nil {
		return false, err
	}
	if fml2, err = p.manager.NewRef(ast.ASTHandle{}); err != nil {
		return false, err
	}
	first, err := p.manager.RefGet(fml1)
	if err == nil {
		err = p.blaster.Rewrite(first, fml2)
	}
	if err != nil {
		return false, err
	}
	if fml3, err = p.manager.NewRef(ast.ASTHandle{}); err != nil {
		return false, err
	}
	second, err := p.manager.RefGet(fml2)
	if err == nil {
		err = p.expansionTraversal.Rewrite(second, fml3)
	}
	if err != nil {
		return false, err
	}
	third, err := p.manager.RefGet(fml3)
	if err != nil {
		return false, err
	}
	current, err := p.manager.RefGet(*fml)
	if err != nil {
		return false, err
	}
	if third == current {
		return false, nil
	}
	return true, p.assignFormula(fml, third)
}

func (p *BitBlastPlugin) mkBlastedRule(rule *base.Rule, fml ast.ASTRef, result *base.RuleSet) error {
	formula, err := p.manager.RefGet(fml)
	if err != nil {
		return err
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return err
	}
	var proof ast.ASTHandle
	_, hasProof, err := rule.Proof()
	if err != nil {
		return err
	}
	if hasProof {
		// pr = m.mk_asserted(fml): loses the original proof of r.
		if proof, err = bitBlastAssertedProof(p.manager, formula); err != nil {
			return err
		}
	}
	if p.mkRule != nil {
		return p.mkRule(result, formula, proof, name)
	}
	return p.rules.AddFormulaWithProofAndSymbol(result, formula, proof, name)
}

func (p *BitBlastPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closing = true
	// impl members die in reverse declaration order: m_rewriter (the mkbv
	// expansion), m_blaster, then m_simplifier.
	if err := p.expansion.release(); err != nil {
		return err
	}
	if p.blaster != nil {
		if err := p.blaster.Close(); err != nil {
			return err
		}
		p.blaster = nil
	}
	if p.simplifier != nil {
		if err := p.simplifier.Close(); err != nil {
			return err
		}
		p.simplifier = nil
	}
	p.expansion.source, p.expansion.target = nil, nil
	p.expansion.oldToNew = nil
	p.closed = true
	return nil
}
