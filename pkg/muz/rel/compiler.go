package rel

// Source: src/muz/rel/dl_compiler.cpp retained load, register allocation,
// finite rule assembly, dependency ordering, recursive fixpoint, and store paths.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

var ErrCompilerUnsupported = errors.New("muz/rel: rule is outside the finite compiler fragment")

// FiniteCompilerOptions selects source rule evaluation modes.
type FiniteCompilerOptions struct {
	// AllOrNothingDeltas uses deltas only to detect changes, evaluating each
	// rule in a recursive stratum against full relations on every iteration.
	AllOrNothingDeltas bool
}

func onlyCompilerUnsupported(err error) bool {
	if err == nil {
		return false
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		children := many.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyCompilerUnsupported(child) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return onlyCompilerUnsupported(one.Unwrap())
	}
	return err == ErrCompilerUnsupported
}

type finiteRuleKind uint8

const (
	finiteGroundRule finiteRuleKind = iota
	finiteSingleTailRule
	finiteTwoTailRule
)

type finiteCompiledRule struct {
	rule        *base.Rule
	kind        finiteRuleKind
	head        ast.ASTHandle
	tail        ast.ASTHandle
	tail2       ast.ASTHandle
	negative    []ast.ASTHandle
	interpreted []ast.ASTHandle
}

type constantRegisterKey struct {
	sort  ast.ASTHandle
	value Element
}

type totalRegisterKey struct {
	sort      ast.ASTHandle
	predicate ast.ASTHandle
}

type finiteCompiler struct {
	manager       *ast.ASTManager
	relations     *RelationManager
	execution     *InstructionBlock
	termination   *InstructionBlock
	current       *InstructionBlock
	guard         *astrewriter.TheoryRewriter
	options       FiniteCompilerOptions
	predicateRegs map[ast.ASTHandle]uint32
	signatures    []Signature
	// regSorts is m_reg_signatures' relation sorts (nil when unknown).
	regSorts [][]ast.ASTHandle
	// predicateLayout is m_pred_regs' obj_map layout: the termination
	// stores follow its table order.
	predicateLayout *ast.ObjMap
	constants       map[constantRegisterKey]uint32
	totals          map[totalRegisterKey]uint32
	emptyTotals     map[ast.ASTHandle]uint32
}

func newFiniteCompiler(relations *RelationManager, guard *astrewriter.TheoryRewriter) (*finiteCompiler, error) {
	if err := relations.open(); err != nil {
		return nil, err
	}
	if guard != nil {
		// The compiling context's rewriter (context::get_rewriter).
		relations.SetContextRewriter(guard)
	}
	return &finiteCompiler{
		manager: relations.manager, relations: relations, guard: guard,
		execution: NewInstructionBlock(), termination: NewInstructionBlock(),
		predicateRegs: make(map[ast.ASTHandle]uint32),
		constants:     make(map[constantRegisterKey]uint32),
		totals:        make(map[totalRegisterKey]uint32),
		emptyTotals:   make(map[ast.ASTHandle]uint32),
	}, nil
}

func (c *finiteCompiler) closeBlocks() error {
	return errors.Join(c.execution.Close(), c.termination.Close())
}

func (c *finiteCompiler) fresh(signature Signature) uint32 {
	register := uint32(len(c.signatures))
	c.signatures = append(c.signatures, Signature{domains: signature.Domains()})
	c.regSorts = append(c.regSorts, nil)
	return register
}

func (c *finiteCompiler) append(block *InstructionBlock, instruction Instruction, rule *base.Rule) error {
	if instruction == nil {
		return ErrInstruction
	}
	if rule != nil {
		if err := instruction.Accounting().SetAccountingParentRule(rule); err != nil {
			return errors.Join(err, instruction.Close())
		}
	}
	if err := block.Append(instruction); err != nil {
		return errors.Join(err, instruction.Close())
	}
	return nil
}

func (c *finiteCompiler) emit(instruction Instruction, rule *base.Rule) error {
	block := c.execution
	if c.current != nil {
		block = c.current
	}
	return c.append(block, instruction, rule)
}

func (c *finiteCompiler) ensureLoaded(predicate ast.ASTHandle) error {
	if _, found := c.predicateRegs[predicate]; found {
		return nil
	}
	sorts, signature, err := c.relations.RelationPredicateSignature(predicate)
	if err != nil {
		return err
	}
	register := c.fresh(signature)
	c.regSorts[register] = sorts
	instruction, err := NewLoadInstruction(c.relations, predicate, register)
	if err != nil {
		return err
	}
	if err = c.append(c.execution, instruction, nil); err != nil {
		return err
	}
	c.predicateRegs[predicate] = register
	if c.predicateLayout == nil {
		c.predicateLayout = ast.NewObjMap(c.manager)
	}
	if err = c.predicateLayout.InsertNum(predicate, uint64(register)); err != nil {
		return err
	}
	return nil
}

func (c *finiteCompiler) classify(rule *base.Rule) (finiteCompiledRule, error) {
	result := finiteCompiledRule{rule: rule}
	var err error
	result.head, err = rule.Head()
	if err != nil {
		return result, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return result, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return result, err
	}
	tails, err := rule.TailSize()
	if err != nil {
		return result, err
	}
	validArguments := func(application ast.ASTHandle) error {
		count, x := c.manager.GetNumArgs(application)
		if x != nil {
			return x
		}
		for index := uint32(0); index < count; index++ {
			argument, y := c.manager.GetArg(application, index)
			if y != nil {
				return y
			}
			kind, y := c.manager.GetKind(argument)
			if y != nil {
				return y
			}
			if kind == ast.ASTVar {
				continue
			}
			value, y := c.manager.IsValue(argument)
			if y != nil {
				return y
			}
			if !value {
				return ErrCompilerUnsupported
			}
		}
		return nil
	}
	if err = validArguments(result.head); err != nil {
		return result, err
	}
	if positive < 0 || positive > 2 || uninterpreted < positive || tails < uninterpreted {
		return result, ErrCompilerUnsupported
	}
	result.kind = finiteGroundRule
	if positive >= 1 {
		result.tail, _, err = rule.Tail(0)
		if err != nil {
			return result, err
		}
		if err = validArguments(result.tail); err != nil {
			return result, err
		}
		result.kind = finiteSingleTailRule
	}
	if positive == 2 {
		result.tail2, _, err = rule.Tail(1)
		if err != nil {
			return result, err
		}
		if err = validArguments(result.tail2); err != nil {
			return result, err
		}
		result.kind = finiteTwoTailRule
	}
	for index := positive; index < uninterpreted; index++ {
		application, negated, x := rule.Tail(index)
		if x != nil {
			return result, x
		}
		if !negated {
			return result, ErrCompilerUnsupported
		}
		if x = validArguments(application); x != nil {
			return result, x
		}
		result.negative = append(result.negative, application)
	}
	if tails != uninterpreted && c.guard == nil {
		return result, ErrCompilerUnsupported
	}
	for index := uninterpreted; index < tails; index++ {
		condition, negated, x := rule.Tail(index)
		if x != nil || negated {
			return result, errors.Join(x, ErrCompilerUnsupported)
		}
		result.interpreted = append(result.interpreted, condition)
	}
	return result, nil
}

func closeCompiledRules(rules []finiteCompiledRule) (result error) {
	for _, rule := range rules {
		result = errors.Join(result, rule.rule.Close())
	}
	return result
}

func (c *finiteCompiler) readRules(set *base.RuleSet) ([]finiteCompiledRule, error) {
	count, err := set.Len()
	if err != nil {
		return nil, err
	}
	rules := make([]finiteCompiledRule, 0, count)
	for index := 0; index < count; index++ {
		rule, x := set.Rule(index)
		if x != nil {
			return nil, errors.Join(x, closeCompiledRules(rules))
		}
		compiled, x := c.classify(rule)
		if x != nil {
			return nil, errors.Join(x, rule.Close(), closeCompiledRules(rules))
		}
		rules = append(rules, compiled)
	}
	return rules, nil
}

type finitePredicateOrderKey struct {
	origin           uint64
	slot, generation uint32
}

func finitePredicateKey(predicate ast.ASTHandle) finitePredicateOrderKey {
	return finitePredicateOrderKey{origin: predicate.Origin(), slot: predicate.Slot(), generation: predicate.Generation()}
}

// sourceDependencyOrder mirrors dl_compiler's cycle_breaker followed by
// rule_dependencies::sort_deps: remove a DFS back-edge source, topologically
// order the remaining dependency graph, then append removed predicates.
func sourceDependencyOrder(heads []finitePredicateOrderKey, dependencies map[finitePredicateOrderKey][]finitePredicateOrderKey) ([]finitePredicateOrderKey, []finitePredicateOrderKey) {
	allowed := make(map[finitePredicateOrderKey]bool, len(heads))
	for _, head := range heads {
		allowed[head] = true
	}
	restricted := make(map[finitePredicateOrderKey][]finitePredicateOrderKey, len(heads))
	for _, head := range heads {
		for _, dependency := range dependencies[head] {
			if allowed[dependency] {
				restricted[head] = append(restricted[head], dependency)
			}
		}
	}
	dependencies = restricted
	visited := make(map[finitePredicateOrderKey]bool, len(heads))
	onStack := make(map[finitePredicateOrderKey]bool, len(heads))
	removed := make(map[finitePredicateOrderKey]bool)
	var visit func(finitePredicateOrderKey)
	visit = func(head finitePredicateOrderKey) {
		if visited[head] || removed[head] {
			return
		}
		visited[head] = true
		onStack[head] = true
		for _, dependency := range dependencies[head] {
			if onStack[dependency] {
				removed[head] = true
				break
			}
			visit(dependency)
		}
		delete(onStack, head)
	}
	for _, head := range heads {
		visit(head)
	}

	indegree := make(map[finitePredicateOrderKey]int, len(heads))
	activeCount := 0
	for _, head := range heads {
		if removed[head] {
			continue
		}
		activeCount++
		for _, dependency := range dependencies[head] {
			if !removed[dependency] {
				indegree[head]++
			}
		}
	}
	ordered := make([]finitePredicateOrderKey, 0, len(heads))
	for _, head := range heads {
		if !removed[head] && indegree[head] == 0 {
			ordered = append(ordered, head)
		}
	}
	for current := 0; current < len(ordered); current++ {
		dependency := ordered[current]
		for _, head := range heads {
			if removed[head] || indegree[head] == 0 {
				continue
			}
			for _, candidate := range dependencies[head] {
				if candidate == dependency {
					indegree[head]--
					if indegree[head] == 0 {
						ordered = append(ordered, head)
					}
					break
				}
			}
		}
	}
	if len(ordered) != activeCount {
		return nil, nil
	}
	removedOrder := make([]finitePredicateOrderKey, 0, len(heads)-activeCount)
	for _, head := range heads {
		if removed[head] {
			removedOrder = append(removedOrder, head)
			ordered = append(ordered, head)
		}
	}
	return ordered, removedOrder
}

func (c *finiteCompiler) orderRecursiveRules(rules []finiteCompiledRule) ([]finiteCompiledRule, error) {
	byHead := make(map[finitePredicateOrderKey][]finiteCompiledRule)
	heads := make([]finitePredicateOrderKey, 0)
	dependencies := make(map[finitePredicateOrderKey][]finitePredicateOrderKey)
	seenHeads := make(map[finitePredicateOrderKey]bool)
	for _, rule := range rules {
		head, err := c.manager.GetDecl(rule.head)
		if err != nil {
			return nil, err
		}
		headKey := finitePredicateKey(head)
		if !seenHeads[headKey] {
			seenHeads[headKey] = true
			heads = append(heads, headKey)
		}
		byHead[headKey] = append(byHead[headKey], rule)
		if rule.kind != finiteSingleTailRule && rule.kind != finiteTwoTailRule {
			continue
		}
		for _, application := range []ast.ASTHandle{rule.tail, rule.tail2} {
			if application.IsNull() {
				continue
			}
			dependency, x := c.manager.GetDecl(application)
			if x != nil {
				return nil, x
			}
			dependencyKey := finitePredicateKey(dependency)
			duplicate := false
			for _, existing := range dependencies[headKey] {
				duplicate = duplicate || existing == dependencyKey
			}
			if !duplicate {
				dependencies[headKey] = append(dependencies[headKey], dependencyKey)
			}
		}
	}
	order, _ := sourceDependencyOrder(heads, dependencies)
	if len(order) != len(heads) {
		return nil, ErrCompilerUnsupported
	}
	orderedRules := make([]finiteCompiledRule, 0, len(rules))
	for _, key := range order {
		orderedRules = append(orderedRules, byHead[key]...)
	}
	return orderedRules, nil
}

func (c *finiteCompiler) constantRegister(rule *base.Rule, predicate, sort ast.ASTHandle, domain uint64, value Element) (uint32, error) {
	key := constantRegisterKey{sort: sort, value: value}
	if register, found := c.constants[key]; found {
		return register, nil
	}
	register := c.fresh(Signature{domains: []uint64{domain}})
	c.regSorts[register] = []ast.ASTHandle{sort}
	instruction, err := NewUnarySingletonInstructionForPredicate(domain, value, register, predicate)
	if err != nil {
		return 0, err
	}
	instruction.(*constructInstruction).SetDisplaySorts(c.manager, []ast.ASTHandle{sort})
	// m_top_level_code.push_back(...): outside any loop body being built.
	if err = c.append(c.execution, instruction, rule); err != nil {
		return 0, err
	}
	c.constants[key] = register
	return register, nil
}

func (c *finiteCompiler) totalRegister(rule *base.Rule, predicate, sort ast.ASTHandle, domain uint64) (uint32, error) {
	key := totalRegisterKey{sort: sort, predicate: predicate}
	if register, found := c.totals[key]; found {
		return register, nil
	}
	signature := Signature{domains: []uint64{domain}}
	register := c.fresh(signature)
	c.regSorts[register] = []ast.ASTHandle{sort}
	name, err := c.predicateNameText(predicate)
	if err != nil {
		return 0, err
	}
	// m_top_level_code.push_back(instruction::mk_total(...)).
	total := NewTotalInstructionForPredicateKind(signature, register, predicate, name)
	total.(*constructInstruction).SetDisplaySorts(c.manager, []ast.ASTHandle{sort})
	if err := c.append(c.execution, total, rule); err != nil {
		return 0, err
	}
	c.totals[key] = register
	return register, nil
}

// predicateNameText is the text of func_decl::get_name() as a symbol is
// streamed (operator<<(ostream, symbol)).
func (c *finiteCompiler) predicateNameText(predicate ast.ASTHandle) (string, error) {
	name, err := c.manager.GetASTName(predicate)
	if err != nil {
		return "", err
	}
	return c.manager.Symbols.Display(name)
}

func (c *finiteCompiler) emptyTotalRegister(rule *base.Rule, predicate ast.ASTHandle) (uint32, error) {
	if register, found := c.emptyTotals[predicate]; found {
		return register, nil
	}
	signature := Signature{}
	register := c.fresh(signature)
	c.regSorts[register] = []ast.ASTHandle{}
	name, err := c.predicateNameText(predicate)
	if err != nil {
		return 0, err
	}
	// m_top_level_code.push_back(instruction::mk_total(...)).
	if err := c.append(c.execution, NewTotalInstructionForPredicateKind(signature, register, predicate, name), rule); err != nil {
		return 0, err
	}
	c.emptyTotals[predicate] = register
	return register, nil
}

func renamedSignature(signature Signature, cycle []int) (Signature, error) {
	if len(cycle) < 2 {
		return signature, nil
	}
	domains := signature.Domains()
	original := signature.Domains()
	seen := make(map[int]bool)
	for _, column := range cycle {
		if column < 0 || column >= len(domains) || seen[column] {
			return Signature{}, ErrCompilerUnsupported
		}
		seen[column] = true
	}
	for index := 1; index < len(cycle); index++ {
		domains[cycle[index-1]] = original[cycle[index]]
	}
	domains[cycle[len(cycle)-1]] = original[cycle[0]]
	return NewSignature(domains)
}

func (c *finiteCompiler) conditionVariables(root ast.ASTHandle, variables map[uint32]ast.ASTHandle) error {
	kind, err := c.manager.GetKind(root)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		index, x := c.manager.GetVarIndex(root)
		if x != nil {
			return x
		}
		variables[index], x = c.manager.GetSort(root)
		return x
	}
	if kind != ast.ASTApp {
		return nil
	}
	count, err := c.manager.GetNumArgs(root)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		child, x := c.manager.GetArg(root, index)
		if x != nil {
			return x
		}
		if x = c.conditionVariables(child, variables); x != nil {
			return x
		}
	}
	return nil
}

// renamedConditionColumns lists the column variables of a renamed condition;
// each variable index is its column.
func (c *finiteCompiler) renamedConditionColumns(renamed ast.ASTHandle) (map[uint32]int, map[uint32]ast.ASTHandle, error) {
	sorts := make(map[uint32]ast.ASTHandle)
	if err := c.conditionVariables(renamed, sorts); err != nil {
		return nil, nil, err
	}
	columns := make(map[uint32]int, len(sorts))
	for variable := range sorts {
		columns[variable] = int(variable)
	}
	return columns, sorts, nil
}

// compileRule is compiler::compile_rule_evaluation for one run: the head
// register and the positive tails' registers, a tail source other than
// VoidRegister standing for that tail's delta register.
func (c *finiteCompiler) compileRule(rule finiteCompiledRule, delta uint32, tailSources [2]uint32) error {
	predicate, err := c.manager.GetDecl(rule.head)
	if err != nil {
		return err
	}
	headReg := c.predicateRegs[predicate]
	tailRegs := make([]uint32, 0, 2)
	for index, application := range []ast.ASTHandle{rule.tail, rule.tail2} {
		if application.IsNull() {
			continue
		}
		tailPred, x := c.manager.GetDecl(application)
		if x != nil {
			return x
		}
		register := c.predicateRegs[tailPred]
		if tailSources[index] != VoidRegister {
			register = tailSources[index]
			if int(register) >= len(c.signatures) || !c.signatures[register].equal(c.signatures[c.predicateRegs[tailPred]]) {
				return ErrCompilerUnsupported
			}
		}
		tailRegs = append(tailRegs, register)
	}
	return c.compileRuleEvaluationRun(rule, headReg, tailRegs, delta)
}

// compileRecursive emits a finite fixpoint loop. By default each rule run
// substitutes one changed tail and reads accumulated relations for its other
// tails; all-or-nothing mode reads full relations. Both record new delta rows.
func (c *finiteCompiler) compileRecursive(set *base.RuleSet, rules []finiteCompiledRule) (resultErr error) {
	orderedRules, err := c.orderRecursiveRules(rules)
	if err != nil {
		return err
	}
	rules = orderedRules
	fullSources := [2]uint32{VoidRegister, VoidRegister}
	headOrder := make([]ast.ASTHandle, 0)
	seen := make(map[ast.ASTHandle]bool)
	rulesByHead := make(map[ast.ASTHandle][]finiteCompiledRule)
	currentDeltas := make(map[ast.ASTHandle]uint32)
	nextDeltas := make(map[ast.ASTHandle]uint32)
	for _, rule := range rules {
		head, err := c.manager.GetDecl(rule.head)
		if err != nil {
			return err
		}
		rulesByHead[head] = append(rulesByHead[head], rule)
		if seen[head] {
			continue
		}
		seen[head] = true
		headOrder = append(headOrder, head)
		signature := c.signatures[c.predicateRegs[head]]
		currentDeltas[head] = c.fresh(signature)
		nextDeltas[head] = c.fresh(signature)
		c.regSorts[currentDeltas[head]] = c.regSorts[c.predicateRegs[head]]
		c.regSorts[nextDeltas[head]] = c.regSorts[c.predicateRegs[head]]
	}
	// compile_preds_init evaluates only lower-stratum inputs before entering
	// the recursive loop. Every existing head row, including preloaded facts,
	// then participates in the first iteration through a full relation clone.
	for _, head := range headOrder {
		stratum, err := set.PredicateStratum(head)
		if err != nil {
			return err
		}
		for _, rule := range rulesByHead[head] {
			count, err := rule.rule.UninterpretedTailSize()
			if err != nil {
				return err
			}
			initial := true
			for index := 0; index < count; index++ {
				tail, _, err := rule.rule.Tail(index)
				if err != nil {
					return err
				}
				predicate, err := c.manager.GetDecl(tail)
				if err != nil {
					return err
				}
				tailStratum, err := set.PredicateStratum(predicate)
				if err != nil {
					return err
				}
				if tailStratum >= stratum {
					initial = false
					break
				}
			}
			if initial {
				if err = c.compileRule(rule, VoidRegister, fullSources); err != nil {
					return err
				}
			}
		}
		if err = c.emit(NewCloneInstruction(c.predicateRegs[head], currentDeltas[head]), nil); err != nil {
			return err
		}
	}
	body := NewInstructionBlock()
	transferred := false
	defer func() {
		if !transferred {
			resultErr = errors.Join(resultErr, body.Close())
		}
	}()
	c.current = body
	defer func() { c.current = nil }()
	for _, rule := range rules {
		head, err := c.manager.GetDecl(rule.head)
		if err != nil {
			return err
		}
		if c.options.AllOrNothingDeltas {
			if err = c.compileRule(rule, nextDeltas[head], fullSources); err != nil {
				return err
			}
			continue
		}
		tails := []ast.ASTHandle{rule.tail}
		if rule.kind == finiteTwoTailRule {
			tails = append(tails, rule.tail2)
		}
		if rule.kind == finiteGroundRule {
			continue
		}
		for index, application := range tails {
			predicate, x := c.manager.GetDecl(application)
			if x != nil {
				return x
			}
			delta, found := currentDeltas[predicate]
			if !found {
				continue
			}
			sources := fullSources
			sources[index] = delta
			if err = c.compileRule(rule, nextDeltas[head], sources); err != nil {
				return err
			}
		}
	}
	controls := make([]uint32, 0, len(headOrder))
	for _, head := range headOrder {
		current := currentDeltas[head]
		controls = append(controls, current)
		if err := c.emit(NewMoveInstruction(nextDeltas[head], current), nil); err != nil {
			return err
		}
	}
	c.current = nil
	loop := NewWhileLoopInstruction(controls, body)
	if err := c.append(c.execution, loop, nil); err != nil {
		return err
	}
	transferred = true
	return nil
}

func (c *finiteCompiler) compile(set *base.RuleSet) (resultErr error) {
	rules, err := c.readRules(set)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeCompiledRules(rules)) }()
	if len(rules) == 0 {
		return nil
	}
	for _, rule := range rules {
		head, x := c.manager.GetDecl(rule.head)
		if x != nil {
			return x
		}
		if x = c.ensureLoaded(head); x != nil {
			return x
		}
		if rule.kind == finiteSingleTailRule || rule.kind == finiteTwoTailRule {
			for _, application := range []ast.ASTHandle{rule.tail, rule.tail2} {
				if application.IsNull() {
					continue
				}
				tail, y := c.manager.GetDecl(application)
				if y != nil {
					return y
				}
				if y = c.ensureLoaded(tail); y != nil {
					return y
				}
			}
		}
		for _, application := range rule.negative {
			negative, y := c.manager.GetDecl(application)
			if y != nil {
				return y
			}
			if y = c.ensureLoaded(negative); y != nil {
				return y
			}
		}
	}
	if err = c.compileStrats(set, rules); err != nil {
		return err
	}
	return c.storePredicates()
}

// compileStrats is compiler::compile_strats over the stratifier's strata with
// no input deltas, empty output deltas and saturation marks.
func (c *finiteCompiler) compileStrats(set *base.RuleSet, rules []finiteCompiledRule) error {
	rulesByHead := make(map[ast.ASTHandle][]finiteCompiledRule)
	for _, rule := range rules {
		head, x := c.manager.GetDecl(rule.head)
		if x != nil {
			return x
		}
		// m_rule_set.get_predicate_rules(head) keeps insertion order.
		rulesByHead[head] = append(rulesByHead[head], rule)
	}
	strats, err := set.Strata()
	if err != nil {
		return err
	}
	for _, stratPreds := range strats {
		allSaturated, x := c.allSaturated(stratPreds)
		if x != nil {
			return x
		}
		if allSaturated {
			//all predicates in stratum are saturated, so no need to compile rules for them
			continue
		}
		_ = c.manager.EmitTrace("dl", "compile_strats", "src/muz/rel/dl_compiler.cpp", 1272, func(out io.Writer) error {
			if _, err := io.WriteString(out, "Stratum: "); err != nil {
				return err
			}
			for _, pred := range stratPreds {
				name, err := c.manager.GetASTName(pred)
				if err != nil {
					return err
				}
				text, err := c.manager.Symbols.String(name)
				if err != nil {
					return err
				}
				if _, err = fmt.Fprintf(out, "%s ", text); err != nil {
					return err
				}
			}
			_, err := io.WriteString(out, "\n")
			return err
		})
		nonrecursive, x := c.isNonrecursiveStratum(stratPreds, rulesByHead)
		if x != nil {
			return x
		}
		if nonrecursive {
			//this stratum contains just a single non-recursive rule
			x = c.compileNonrecursiveStratum(stratPreds[0], rulesByHead[stratPreds[0]])
		} else {
			x = c.compileDependentRules(set, stratPreds, rulesByHead)
		}
		if x != nil {
			return x
		}
	}
	return nil
}

// allSaturated is compiler::all_saturated.
func (c *finiteCompiler) allSaturated(preds []ast.ASTHandle) (bool, error) {
	for _, pred := range preds {
		saturated, err := c.relations.IsSaturated(pred)
		if err != nil || !saturated {
			return false, err
		}
	}
	return true, nil
}

// isNonrecursiveStratum is compiler::is_nonrecursive_stratum.
func (c *finiteCompiler) isNonrecursiveStratum(preds []ast.ASTHandle, rulesByHead map[ast.ASTHandle][]finiteCompiledRule) (bool, error) {
	if len(preds) > 1 {
		return false, nil
	}
	headPred := preds[0]
	for _, rule := range rulesByHead[headPred] {
		//it is sufficient to check just for presence of the first head predicate,
		//since if the rules are recursive and their heads are strongly connected by dependence,
		//this predicate must appear in some tail
		inTail, err := rule.rule.IsInTail(headPred, false)
		if err != nil || inTail {
			return false, err
		}
	}
	return true, nil
}

// markSaturated is acc.push_back(instruction::mk_mark_saturated(m, pred)).
func (c *finiteCompiler) markSaturated(pred ast.ASTHandle) error {
	instruction, err := NewMarkSaturatedInstruction(c.relations, pred)
	if err != nil {
		return err
	}
	return c.append(c.execution, instruction, nil)
}

// compileNonrecursiveStratum is compiler::compile_nonrecursive_stratum with
// no output delta and saturation marks.
func (c *finiteCompiler) compileNonrecursiveStratum(headPred ast.ASTHandle, rules []finiteCompiledRule) error {
	for _, rule := range rules {
		if err := c.compileRule(rule, VoidRegister, [2]uint32{VoidRegister, VoidRegister}); err != nil {
			return err
		}
	}
	//now the predicate is saturated, so we may mark it as such
	return c.markSaturated(headPred)
}

// compileDependentRules is compiler::compile_dependent_rules with saturation
// marks: the recursive loop, then a mark for every predicate of the stratum.
func (c *finiteCompiler) compileDependentRules(set *base.RuleSet, headPreds []ast.ASTHandle, rulesByHead map[ast.ASTHandle][]finiteCompiledRule) error {
	var group []finiteCompiledRule
	for _, pred := range headPreds {
		group = append(group, rulesByHead[pred]...)
	}
	if len(group) != 0 {
		if err := c.compileRecursive(set, group); err != nil {
			return err
		}
	}
	//after the loop finishes, all predicates in the group are saturated,
	//so we may mark them as such
	for _, pred := range headPreds {
		if err := c.markSaturated(pred); err != nil {
			return err
		}
	}
	return nil
}

// storePredicates is the store loop of compiler::do_compilation, in
// m_pred_regs' (obj_map) iteration order.
func (c *finiteCompiler) storePredicates() error {
	if c.predicateLayout == nil {
		return nil
	}
	for _, entry := range c.predicateLayout.Entries() {
		predicate := entry.Key
		instruction, x := NewStoreInstruction(c.relations, predicate, c.predicateRegs[predicate])
		if x != nil {
			return x
		}
		if x = c.append(c.termination, instruction, nil); x != nil {
			return x
		}
	}
	return nil
}

// CompileFiniteRules compiles the currently retained stratified finite rule
// slice with up to two positive tails into execution and termination bytecode.
// Ownership of both returned blocks transfers to the caller.
func CompileFiniteRules(relations *RelationManager, set *base.RuleSet) (*InstructionBlock, *InstructionBlock, error) {
	return CompileFiniteRulesWithGuard(relations, set, nil)
}

// CompileFiniteRulesWithGuard additionally compiles interpreted finite guards.
func CompileFiniteRulesWithGuard(relations *RelationManager, set *base.RuleSet, guard *astrewriter.TheoryRewriter) (*InstructionBlock, *InstructionBlock, error) {
	return CompileFiniteRulesWithOptions(relations, set, guard, FiniteCompilerOptions{})
}

// CompileFiniteRulesWithOptions compiles finite rules and interpreted guards
// using the selected recursive evaluation mode.
func CompileFiniteRulesWithOptions(relations *RelationManager, set *base.RuleSet, guard *astrewriter.TheoryRewriter, options FiniteCompilerOptions) (*InstructionBlock, *InstructionBlock, error) {
	if relations == nil {
		return nil, nil, ErrCompilerUnsupported
	}
	if err := relations.SupportsCompiledFiniteTables(); err != nil {
		return nil, nil, err
	}
	compiler, err := newFiniteCompiler(relations, guard)
	if err != nil {
		return nil, nil, err
	}
	compiler.options = options
	if set == nil {
		if closeErr := compiler.closeBlocks(); closeErr != nil {
			return nil, nil, closeErr
		}
		return nil, nil, ErrCompilerUnsupported
	}
	if err = compiler.compile(set); err != nil {
		closeErr := compiler.closeBlocks()
		if onlyCompilerUnsupported(err) && closeErr != nil {
			return nil, nil, closeErr
		}
		return nil, nil, errors.Join(err, closeErr)
	}
	return compiler.execution, compiler.termination, nil
}
