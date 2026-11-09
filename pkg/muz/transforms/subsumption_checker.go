package transforms

// Source: src/muz/transforms/dl_mk_subsumption_checker.cpp/.h and
// src/muz/base/dl_rule_subsumption_index.cpp. The retained consumer (Spacer)
// has no rel_context, so scan_for_relations_total_due_to_facts returns at
// once and the ground unconditional heads it would read are not collected.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

type SubsumptionCheckerPlugin struct {
	priority uint32
	enabled  bool
	// m_total_relations (func_decl_set, iterated in hash-table order) and
	// m_total_relation_defining_rules (a nil rule: total due to facts).
	totalRelations *ast.ObjMap
	totalDefining  map[ast.ASTHandle]*base.Rule
	// m_ref_holder keeps the defining rules alive with the plugin.
	refHolder        []*base.Rule
	haveNewTotalRule bool
	noModelConverter bool
	// fixUnbound is rule_manager::fix_unbound_vars: reduce_unbound_vars with
	// the rule manager's qe_lite, then the xform.fix_unbound_vars rewrite.
	fixUnbound func(*base.Rule) (*base.Rule, error)
	modelSteps []ConeModelStep
	closed     bool
	closing    bool
}

type subsumptionConstructionFailure struct {
	manager        *ast.ASTManager
	sets           []*base.RuleSet
	steps          []ConeModelStep
	nested         memory.ConstructionFailure
	cause, cleanup error
	reported       error
	text           string
}

var _ memory.ConstructionFailure = (*subsumptionConstructionFailure)(nil)

func (f *subsumptionConstructionFailure) Error() string        { return f.text }
func (f *subsumptionConstructionFailure) Unwrap() error        { return f.reported }
func (f *subsumptionConstructionFailure) OriginalCause() error { return f.cause }
func (f *subsumptionConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *subsumptionConstructionFailure) RuntimeOwner() *memory.Runtime {
	if f.manager == nil || f.manager.NumeralService() == nil {
		return nil
	}
	return f.manager.NumeralService().RuntimeOwner()
}
func (f *subsumptionConstructionFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	for len(f.sets) != 0 {
		last := len(f.sets) - 1
		f.cleanup = f.sets[last].Close()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.sets[last] = nil
		f.sets = f.sets[:last]
	}
	f.cleanup = closeConeModelSteps(f.steps)
	if f.cleanup == nil {
		f.steps = nil
	}
	return f.cleanup
}

func closeSubsumptionConstruction(manager *ast.ASTManager, cause error, sets []*base.RuleSet, steps []ConeModelStep) error {
	if len(sets) == 0 && len(steps) == 0 {
		return cause
	}
	failure := &subsumptionConstructionFailure{manager: manager, sets: sets, steps: steps, cause: cause}
	_ = errors.As(cause, &failure.nested)
	cleanup := failure.Cleanup()
	if cleanup == nil {
		return cause
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	failure.cause = origin
	failure.reported = cleanup
	if cause != nil {
		failure.reported = errors.Join(cause, cleanup)
	}
	failure.text = failure.reported.Error()
	return failure
}

func NewSubsumptionCheckerPlugin(enabled bool, priority uint32) *SubsumptionCheckerPlugin {
	if priority == 0 {
		priority = 31000
	}
	return &SubsumptionCheckerPlugin{priority: priority, enabled: enabled, totalDefining: make(map[ast.ASTHandle]*base.Rule)}
}

func (p *SubsumptionCheckerPlugin) Priority() uint32 { return p.priority }

func (p *SubsumptionCheckerPlugin) CanDestratifyNegation() bool { return false }

func (p *SubsumptionCheckerPlugin) Cancel() {}

// SetModelConverter reports whether the context has a model converter;
// without one transform_rules records no extension_model_converter.
func (p *SubsumptionCheckerPlugin) SetModelConverter(enabled bool) { p.noModelConverter = !enabled }

// SetUnboundFixer supplies rule_manager::fix_unbound_vars.
func (p *SubsumptionCheckerPlugin) SetUnboundFixer(fix func(*base.Rule) (*base.Rule, error)) {
	p.fixUnbound = fix
}

// Close is ~mk_subsumption_checker: m_ref_holder releases its rules forward.
func (p *SubsumptionCheckerPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	if p.closed {
		return nil
	}
	p.closing = true
	if err := closeConeModelSteps(p.modelSteps); err != nil {
		return err
	}
	p.modelSteps = nil
	for len(p.refHolder) != 0 {
		if err := p.refHolder[0].Close(); err != nil {
			return err
		}
		p.refHolder = p.refHolder[1:]
	}
	p.totalDefining = nil
	p.totalRelations = nil
	p.closed = true
	return nil
}

// TakeModelSteps transfers the ordered total-relation interpretations emitted
// by this subsumption stage.
func (p *SubsumptionCheckerPlugin) TakeModelSteps() ([]ConeModelStep, error) {
	if p == nil || p.closed || p.closing {
		return nil, base.ErrRuleTransformerOwner
	}
	steps := p.modelSteps
	p.modelSteps = nil
	return steps, nil
}

func (p *SubsumptionCheckerPlugin) isTotalRelation(declaration ast.ASTHandle) (bool, error) {
	if p.totalRelations == nil {
		return false, nil
	}
	return p.totalRelations.Contains(declaration)
}

// isTotalRule is mk_subsumption_checker::is_total_rule: a fact whose head
// arguments are pairwise distinct variables.
func isTotalRule(rule *base.Rule) (bool, error) {
	tail, err := rule.TailSize()
	if err != nil || tail != 0 {
		return false, err
	}
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	manager, err := rule.ASTManager()
	if err != nil {
		return false, err
	}
	arity, err := manager.GetNumArgs(head)
	if err != nil {
		return false, err
	}
	seen := make(map[uint32]bool, arity)
	for index := uint32(0); index < arity; index++ {
		argument, e := manager.GetArg(head, index)
		if e != nil {
			return false, e
		}
		kind, e := manager.GetKind(argument)
		if e != nil || kind != ast.ASTVar {
			return false, e
		}
		variable, e := manager.GetVarIndex(argument)
		if e != nil || seen[variable] {
			return false, e
		}
		seen[variable] = true
	}
	return true, nil
}

// onDiscoveredTotalRelation records pred as total; a non-nil defining rule
// is retained in m_ref_holder.
func (p *SubsumptionCheckerPlugin) onDiscoveredTotalRelation(manager *ast.ASTManager, declaration ast.ASTHandle, rule *base.Rule) error {
	if p.totalRelations == nil {
		p.totalRelations = ast.NewObjMap(manager)
	}
	if _, err := p.totalRelations.InsertIfNotThere(declaration); err != nil {
		return err
	}
	p.haveNewTotalRule = true
	if rule == nil {
		p.totalDefining[declaration] = nil
		return nil
	}
	held, err := rule.Retain()
	if err != nil {
		return err
	}
	p.totalDefining[declaration] = held
	p.refHolder = append(p.refHolder, held)
	return nil
}

func (p *SubsumptionCheckerPlugin) scanForTotalRules(source *base.RuleSet) error {
	rules, err := source.RuleManager()
	if err != nil {
		return err
	}
	manager, err := rules.ASTManager()
	if err != nil {
		return err
	}
	count, err := source.Len()
	if err != nil {
		return err
	}
	for discovered := true; discovered; {
		discovered = false
		for index := 0; index < count; index++ {
			rule, e := source.Rule(index)
			if e != nil {
				return e
			}
			declaration, e := rule.HeadDeclaration()
			total := false
			if e == nil {
				total, e = isTotalRule(rule)
			}
			known := false
			if e == nil && total {
				known, e = p.isTotalRelation(declaration)
			}
			if e == nil && total && !known {
				e = p.onDiscoveredTotalRelation(manager, declaration, rule)
				discovered = true
			}
			if e = errors.Join(e, rule.Close()); e != nil {
				return e
			}
		}
	}
	return nil
}

// ruleSubsumptionIndex is rule_subsumption_index.
type ruleSubsumptionIndex struct {
	refHolder          []*base.Rule
	unconditionedHeads map[ast.ASTHandle]bool
	ruleSet            map[uint32][]*base.Rule
}

func newRuleSubsumptionIndex() *ruleSubsumptionIndex {
	return &ruleSubsumptionIndex{unconditionedHeads: make(map[ast.ASTHandle]bool), ruleSet: make(map[uint32][]*base.Rule)}
}

func (i *ruleSubsumptionIndex) add(rule *base.Rule) error {
	held, err := rule.Retain()
	if err != nil {
		return err
	}
	i.refHolder = append(i.refHolder, held)
	tail, err := held.TailSize()
	if err != nil {
		return err
	}
	if tail == 0 {
		head, e := held.Head()
		if e != nil {
			return e
		}
		i.unconditionedHeads[head] = true
	}
	hash, err := held.Hash()
	if err != nil {
		return err
	}
	i.ruleSet[hash] = append(i.ruleSet[hash], held)
	return nil
}

func (i *ruleSubsumptionIndex) atomSubsumed(atom ast.ASTHandle) bool {
	return i.unconditionedHeads[atom]
}

func (i *ruleSubsumptionIndex) ruleSubsumed(rule *base.Rule) (bool, error) {
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	if i.atomSubsumed(head) {
		return true, nil
	}
	hash, err := rule.Hash()
	if err != nil {
		return false, err
	}
	for _, prior := range i.ruleSet[hash] {
		same, e := prior.Equal(rule)
		if e != nil || same {
			return same, e
		}
	}
	return false, nil
}

// close is ~rule_subsumption_index: m_ref_holder releases forward.
func (i *ruleSubsumptionIndex) close() (result error) {
	for _, rule := range i.refHolder {
		result = errors.Join(result, rule.Close())
	}
	i.refHolder = nil
	i.ruleSet = nil
	return result
}

// transformRule is mk_subsumption_checker::transform_rule. It returns the
// rule itself (fresh false) or a new owned replacement; ok false means the
// rule is unsatisfiable or never adds facts.
func (p *SubsumptionCheckerPlugin) transformRule(manager *base.RuleManager, rule *base.Rule, index *ruleSubsumptionIndex) (_ *base.Rule, fresh, ok bool, resultErr error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, false, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, false, false, err
	}
	if uninterpreted == 0 {
		return rule, false, true, nil
	}
	head, err := rule.Head()
	if err != nil {
		return nil, false, false, err
	}
	tail := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for position := 0; position < uninterpreted; position++ {
		atom, negative, e := rule.Tail(position)
		if e != nil {
			return nil, false, false, e
		}
		declaration, e := rule.TailDeclaration(position)
		if e != nil {
			return nil, false, false, e
		}
		totalTail, e := p.isTotalRelation(declaration)
		if e != nil {
			return nil, false, false, e
		}
		if totalTail || index.atomSubsumed(atom) {
			if negative {
				return nil, false, false, nil
			}
			continue
		}
		if !negative && atom == head {
			return nil, false, false, nil
		}
		tail = append(tail, atom)
		negated = append(negated, negative)
	}
	if len(tail) == uninterpreted {
		return rule, false, true, nil
	}
	for position := uninterpreted; position < total; position++ {
		atom, negative, e := rule.Tail(position)
		if e != nil {
			return nil, false, false, e
		}
		tail = append(tail, atom)
		negated = append(negated, negative)
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, false, false, err
	}
	replacement, err := manager.NewRuleWithSymbol(head, tail, negated, name)
	if err != nil {
		return nil, false, false, err
	}
	if err = replacement.SetAccountingParentRule(rule); err != nil {
		return nil, false, false, errors.Join(err, replacement.Close())
	}
	// m_context.get_rule_manager().fix_unbound_vars(res, true).
	if p.fixUnbound != nil {
		fixed, e := p.fixUnbound(replacement)
		if e = errors.Join(e, replacement.Close()); e != nil {
			if fixed != nil {
				e = errors.Join(e, fixed.Close())
			}
			return nil, false, false, e
		}
		replacement = fixed
	}
	if err = manager.MkRuleRewriteProof(rule, replacement); err != nil {
		return nil, false, false, errors.Join(err, replacement.Close())
	}
	return replacement, true, true, nil
}

// transformRules is mk_subsumption_checker::transform_rules.
func (p *SubsumptionCheckerPlugin) transformRules(source, target *base.RuleSet) (modified bool, resultErr error) {
	manager, err := source.RuleManager()
	if err != nil {
		return false, err
	}
	astManager, err := manager.ASTManager()
	if err != nil {
		return false, err
	}
	includedTotal := make(map[ast.ASTHandle]bool)
	index := newRuleSubsumptionIndex()
	count, err := source.Len()
	if err != nil {
		return false, err
	}
	original := make([]*base.Rule, 0, count)
	// Locals die in reverse: orig_rules (forward), then subs_index.
	defer func() {
		for _, rule := range original {
			resultErr = errors.Join(resultErr, rule.Close())
		}
		resultErr = errors.Join(resultErr, index.close())
	}()
	sizes := make([]int, 0, count)
	for position := 0; position < count; position++ {
		rule, e := source.Rule(position)
		if e != nil {
			return false, e
		}
		original = append(original, rule)
		size, e := rule.TailSize()
		if e != nil {
			return false, e
		}
		sizes = append(sizes, size)
	}
	// rule_size_comparator: shortest tails first.
	order := make([]int, count)
	for position := range order {
		order[position] = position
	}
	sort.SliceStable(order, func(a, b int) bool { return sizes[order[a]] < sizes[order[b]] })
	for _, position := range order {
		rule := original[position]
		declaration, e := rule.HeadDeclaration()
		if e != nil {
			return false, e
		}
		total, e := p.isTotalRelation(declaration)
		if e != nil {
			return false, e
		}
		if total {
			output, x := source.IsOutputPredicate(declaration)
			if x != nil {
				return false, x
			}
			if !output || includedTotal[declaration] {
				modified = true
				continue
			}
			if defining := p.totalDefining[declaration]; defining != nil {
				totality, fresh, _, x := p.transformRule(manager, defining, index)
				if x != nil {
					return false, x
				}
				if fresh {
					modified = true
				}
				x = target.Add(totality)
				if fresh {
					x = errors.Join(x, totality.Close())
				}
				if x != nil {
					return false, x
				}
			} else {
				modified = true
			}
			includedTotal[declaration] = true
			continue
		}
		replacement, fresh, ok, x := p.transformRule(manager, rule, index)
		if x != nil {
			return false, x
		}
		if !ok {
			modified = true
			continue
		}
		x = p.addTransformedRule(astManager, source, target, index, declaration, replacement, fresh, &modified)
		if fresh {
			x = errors.Join(x, replacement.Close())
		}
		if x != nil {
			return false, x
		}
	}
	if err = target.InheritPredicates(source); err != nil {
		return false, err
	}
	if p.totalRelations != nil && p.totalRelations.Len() != 0 && !p.noModelConverter {
		entries := p.totalRelations.Entries()
		declarations := make([]ast.ASTHandle, 0, len(entries))
		for _, entry := range entries {
			declarations = append(declarations, entry.Key)
		}
		step, x := totalRelationModelStep(astManager, declarations)
		if x != nil {
			return false, errors.Join(x, closeConeModelSteps([]ConeModelStep{step}))
		}
		p.modelSteps = append(p.modelSteps, step)
	}
	return modified, nil
}

func (p *SubsumptionCheckerPlugin) addTransformedRule(manager *ast.ASTManager, source, target *base.RuleSet, index *ruleSubsumptionIndex, declaration ast.ASTHandle, rule *base.Rule, fresh bool, modified *bool) error {
	total, err := isTotalRule(rule)
	if err != nil {
		return err
	}
	if total {
		if err = p.onDiscoveredTotalRelation(manager, declaration, rule); err != nil {
			return err
		}
	}
	subsumed, err := index.ruleSubsumed(rule)
	if err != nil {
		return err
	}
	if subsumed {
		*modified = true
		return nil
	}
	if fresh {
		*modified = true
	}
	if err = target.Add(rule); err != nil {
		return err
	}
	return index.add(rule)
}

func totalRelationModelStep(manager *ast.ASTManager, declarations []ast.ASTHandle) (ConeModelStep, error) {
	if len(declarations) == 0 {
		return nil, nil
	}
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	step := make(ConeModelStep, 0, len(declarations))
	truth, err := manager.MkTrue()
	if err != nil {
		return nil, err
	}
	for _, declaration := range declarations {
		formula, getErr := manager.NewRef(truth)
		if getErr != nil {
			return step, getErr
		}
		step = append(step, ConeModelDefinition{Declaration: declaration, Formula: formula, manager: manager})
	}
	return step, nil
}

// Transform is mk_subsumption_checker::operator().
func (p *SubsumptionCheckerPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.closing {
		return nil, base.ErrRuleTransformerOwner
	}
	if !p.enabled {
		return nil, nil
	}
	rules, err := source.RuleManager()
	if err != nil {
		return nil, err
	}
	manager, err := rules.ASTManager()
	if err != nil {
		return nil, err
	}
	p.haveNewTotalRule = false
	if err = p.scanForTotalRules(source); err != nil {
		return nil, err
	}
	p.haveNewTotalRule = false
	result, err := base.NewRuleSet(rules)
	if err != nil {
		return nil, err
	}
	modified, err := p.transformRules(source, result)
	if err != nil {
		return nil, closeSubsumptionConstruction(manager, err, []*base.RuleSet{result}, nil)
	}
	if !p.haveNewTotalRule && !modified {
		return nil, result.Close()
	}
	for p.haveNewTotalRule {
		p.haveNewTotalRule = false
		old := result
		if result, err = base.NewRuleSet(rules); err != nil {
			return nil, closeSubsumptionConstruction(manager, err, []*base.RuleSet{old}, nil)
		}
		if _, err = p.transformRules(old, result); err != nil {
			return nil, closeSubsumptionConstruction(manager, err, []*base.RuleSet{old, result}, nil)
		}
		if err = old.Close(); err != nil {
			return nil, closeSubsumptionConstruction(manager, err, []*base.RuleSet{result}, nil)
		}
	}
	return result, nil
}
