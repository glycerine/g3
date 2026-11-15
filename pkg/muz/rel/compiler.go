package rel

// Source: src/muz/rel/dl_compiler.cpp retained load, register allocation,
// finite rule assembly, dependency ordering, recursive fixpoint, and store paths.

import (
	"errors"
	"fmt"
	"io"
	"sort"

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
	loaded        []ast.ASTHandle
	constants     map[constantRegisterKey]uint32
	totals        map[totalRegisterKey]uint32
	emptyTotals   map[ast.ASTHandle]uint32
}

func newFiniteCompiler(relations *RelationManager, guard *astrewriter.TheoryRewriter) (*finiteCompiler, error) {
	if err := relations.open(); err != nil {
		return nil, err
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
	_, signature, err := c.relations.PredicateSignature(predicate)
	if err != nil {
		return err
	}
	register := c.fresh(signature)
	instruction, err := NewLoadInstruction(c.relations, predicate, register)
	if err != nil {
		return err
	}
	if err = c.append(c.execution, instruction, nil); err != nil {
		return err
	}
	c.predicateRegs[predicate] = register
	c.loaded = append(c.loaded, predicate)
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

func (c *finiteCompiler) project(register uint32, signature Signature, removed []int, owned bool, rule *base.Rule) (uint32, Signature, bool, error) {
	projected, err := projectedSignature(signature, removed)
	if err != nil {
		return 0, Signature{}, false, err
	}
	result := register
	if owned {
		c.signatures[result] = projected
	} else {
		result = c.fresh(projected)
	}
	if err = c.emit(NewProjectionInstruction(register, removed, result), rule); err != nil {
		return 0, Signature{}, false, err
	}
	return result, projected, true, nil
}

func (c *finiteCompiler) appendColumn(current uint32, signature Signature, owned, present bool, columnRegister uint32, domain uint64, rule *base.Rule) (uint32, Signature, bool, bool, error) {
	if !present {
		return columnRegister, Signature{domains: []uint64{domain}}, false, true, nil
	}
	joined, err := NewSignature(append(signature.Domains(), domain))
	if err != nil {
		return 0, Signature{}, false, false, err
	}
	result := current
	if owned {
		c.signatures[result] = joined
	} else {
		result = c.fresh(joined)
	}
	if err = c.emit(NewJoinInstruction(current, columnRegister, nil, nil, result), rule); err != nil {
		return 0, Signature{}, false, false, err
	}
	return result, joined, true, true, nil
}

func (c *finiteCompiler) duplicateColumn(current uint32, signature Signature, column int, owned bool, rule *base.Rule) (uint32, Signature, bool, error) {
	if column < 0 || column >= signature.Len() {
		return 0, Signature{}, false, ErrCompilerUnsupported
	}
	single := current
	temporary := false
	if signature.Len() != 1 {
		removed := make([]int, 0, signature.Len()-1)
		for index := 0; index < signature.Len(); index++ {
			if index != column {
				removed = append(removed, index)
			}
		}
		one, err := projectedSignature(signature, removed)
		if err != nil {
			return 0, Signature{}, false, err
		}
		single = c.fresh(one)
		if err = c.emit(NewProjectionInstruction(current, removed, single), rule); err != nil {
			return 0, Signature{}, false, err
		}
		temporary = true
	}
	domain := signature.domains[column]
	joined, err := NewSignature(append(signature.Domains(), domain))
	if err != nil {
		return 0, Signature{}, false, err
	}
	result := current
	if owned {
		c.signatures[result] = joined
	} else {
		result = c.fresh(joined)
	}
	if err = c.emit(NewJoinInstruction(current, single, []int{column}, []int{0}, result), rule); err != nil {
		return 0, Signature{}, false, err
	}
	if temporary {
		if err = c.emit(NewDeallocateInstruction(single), rule); err != nil {
			return 0, Signature{}, false, err
		}
	}
	return result, joined, true, nil
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

func (c *finiteCompiler) reorder(current uint32, signature Signature, order []int, owned bool, rule *base.Rule) (uint32, Signature, bool, error) {
	if len(order) != signature.Len() {
		return 0, Signature{}, false, ErrCompilerUnsupported
	}
	seen := make([]bool, len(order))
	for _, source := range order {
		if source < 0 || source >= len(order) || seen[source] {
			return 0, Signature{}, false, ErrCompilerUnsupported
		}
		seen[source] = true
	}
	visited := make([]bool, len(order))
	for start := range order {
		if visited[start] || order[start] == start {
			visited[start] = true
			continue
		}
		cycle := make([]int, 0, 2)
		for column := start; !visited[column]; column = order[column] {
			visited[column] = true
			cycle = append(cycle, column)
		}
		renamed, err := renamedSignature(signature, cycle)
		if err != nil {
			return 0, Signature{}, false, err
		}
		result := current
		if owned {
			c.signatures[result] = renamed
		} else {
			result = c.fresh(renamed)
		}
		if err = c.emit(NewRenameInstruction(current, cycle, result), rule); err != nil {
			return 0, Signature{}, false, err
		}
		current, signature, owned = result, renamed, true
	}
	return current, signature, owned, nil
}

func copyVariableColumns(source map[uint32][]int) map[uint32][]int {
	result := make(map[uint32][]int, len(source))
	for variable, columns := range source {
		result[variable] = append([]int(nil), columns...)
	}
	return result
}

func (c *finiteCompiler) assembleHead(rule finiteCompiledRule, predicate ast.ASTHandle, current uint32, signature Signature, owned, present bool, variableColumns map[uint32][]int) (uint32, bool, error) {
	sorts, target, err := c.relations.PredicateSignature(predicate)
	if err != nil {
		return 0, false, err
	}
	count := target.Len()
	if count == 0 && !present {
		register, x := c.emptyTotalRegister(rule.rule, predicate)
		return register, false, x
	}
	columns := make([]int, count)
	for index := range columns {
		columns[index] = -1
	}
	boundVariables := make([]uint32, count)
	isBound := make([]bool, count)
	available := copyVariableColumns(variableColumns)
	referenced := make([]bool, signature.Len())
	for index := 0; index < count; index++ {
		argument, x := c.manager.GetArg(rule.head, uint32(index))
		if x != nil {
			return 0, false, x
		}
		kind, x := c.manager.GetKind(argument)
		if x != nil {
			return 0, false, x
		}
		if kind != ast.ASTVar {
			continue
		}
		variable, x := c.manager.GetVarIndex(argument)
		if x != nil {
			return 0, false, x
		}
		if source := available[variable]; len(source) != 0 {
			columns[index] = source[0]
			available[variable] = source[1:]
			referenced[source[0]] = true
			boundVariables[index], isBound[index] = variable, true
		} else if len(variableColumns[variable]) != 0 {
			boundVariables[index], isBound[index] = variable, true
		}
	}
	if present {
		removed := make([]int, 0)
		offset := make([]int, signature.Len())
		for column := 0; column < signature.Len(); column++ {
			offset[column] = len(removed)
			if !referenced[column] {
				removed = append(removed, column)
			}
		}
		if len(removed) != 0 {
			current, signature, owned, err = c.project(current, signature, removed, owned, rule.rule)
			if err != nil {
				return 0, false, err
			}
			for index, column := range columns {
				if column >= 0 {
					columns[index] = column - offset[column]
				}
			}
		}
	}
	primary := make(map[uint32]int)
	for index, variable := range boundVariables {
		if isBound[index] && columns[index] >= 0 {
			if _, found := primary[variable]; !found {
				primary[variable] = columns[index]
			}
		}
	}
	for index := 0; index < count; index++ {
		if !isBound[index] || columns[index] >= 0 {
			continue
		}
		column, found := primary[boundVariables[index]]
		if !found {
			return 0, false, ErrCompilerUnsupported
		}
		current, signature, owned, err = c.duplicateColumn(current, signature, column, owned, rule.rule)
		if err != nil {
			return 0, false, err
		}
		columns[index] = signature.Len() - 1
	}
	unbound := make(map[uint32]int)
	for index := 0; index < count; index++ {
		if columns[index] >= 0 {
			continue
		}
		argument, x := c.manager.GetArg(rule.head, uint32(index))
		if x != nil {
			return 0, false, x
		}
		kind, x := c.manager.GetKind(argument)
		if x != nil {
			return 0, false, x
		}
		if kind == ast.ASTVar {
			variable, y := c.manager.GetVarIndex(argument)
			if y != nil {
				return 0, false, y
			}
			if column, found := unbound[variable]; found {
				current, signature, owned, err = c.duplicateColumn(current, signature, column, owned, rule.rule)
				if err != nil {
					return 0, false, err
				}
				columns[index] = signature.Len() - 1
				continue
			}
			domain, y := target.Domain(index)
			if y != nil {
				return 0, false, y
			}
			if y = unsafeRuleColumnVerbose(c.manager, rule.rule, index); y != nil {
				return 0, false, y
			}
			total, y := c.totalRegister(rule.rule, predicate, sorts[index], domain)
			if y != nil {
				return 0, false, y
			}
			column := signature.Len()
			current, signature, owned, present, err = c.appendColumn(current, signature, owned, present, total, domain, rule.rule)
			if err != nil {
				return 0, false, err
			}
			columns[index], unbound[variable] = column, column
			continue
		}
		value, x := c.relations.EncodeValue(sorts[index], argument)
		if x != nil {
			return 0, false, x
		}
		domain, x := target.Domain(index)
		if x != nil {
			return 0, false, x
		}
		singleton, x := c.constantRegister(rule.rule, predicate, sorts[index], domain, value)
		if x != nil {
			return 0, false, x
		}
		column := signature.Len()
		current, signature, owned, present, err = c.appendColumn(current, signature, owned, present, singleton, domain, rule.rule)
		if err != nil {
			return 0, false, err
		}
		columns[index] = column
	}
	current, signature, owned, err = c.reorder(current, signature, columns, owned, rule.rule)
	if err != nil {
		return 0, false, err
	}
	if !signature.equal(target) {
		return 0, false, ErrCompilerUnsupported
	}
	return current, owned, nil
}

func (c *finiteCompiler) prepareTail(rule *base.Rule, application ast.ASTHandle, source uint32) (uint32, Signature, bool, map[uint32][]int, error) {
	tailPredicate, err := c.manager.GetDecl(application)
	if err != nil {
		return 0, Signature{}, false, nil, err
	}
	sorts, signature, err := c.relations.PredicateSignature(tailPredicate)
	if err != nil {
		return 0, Signature{}, false, nil, err
	}
	current := source
	if current == VoidRegister {
		current = c.predicateRegs[tailPredicate]
	} else if int(current) >= len(c.signatures) || !c.signatures[current].equal(signature) {
		return 0, Signature{}, false, nil, ErrCompilerUnsupported
	}
	owned := false
	variableColumns := make(map[uint32][]int)
	tailCount, err := c.manager.GetNumArgs(application)
	if err != nil {
		return 0, Signature{}, false, nil, err
	}
	column := 0
	for index := uint32(0); index < tailCount; index++ {
		argument, x := c.manager.GetArg(application, index)
		if x != nil {
			return 0, Signature{}, false, nil, x
		}
		kind, x := c.manager.GetKind(argument)
		if x != nil {
			return 0, Signature{}, false, nil, x
		}
		if kind == ast.ASTVar {
			variable, y := c.manager.GetVarIndex(argument)
			if y != nil {
				return 0, Signature{}, false, nil, y
			}
			variableColumns[variable] = append(variableColumns[variable], column)
			column++
			continue
		}
		value, x := c.relations.EncodeValue(sorts[index], argument)
		if x != nil {
			return 0, Signature{}, false, nil, x
		}
		projected, x := projectedSignature(signature, []int{column})
		if x != nil {
			return 0, Signature{}, false, nil, x
		}
		result := current
		if owned {
			c.signatures[result] = projected
		} else {
			result = c.fresh(projected)
		}
		if x = c.emit(NewSelectEqualProjectInstruction(current, value, column, result), rule); x != nil {
			return 0, Signature{}, false, nil, x
		}
		current, signature, owned = result, projected, true
	}
	// dl_compiler.cpp compile_rule_evaluation_run walks its int2ints
	// var_indexes (u_map) in table order, the keys inserted in column order.
	for _, variable := range uMapOrder(variablesByFirstColumn(variableColumns)) {
		columns := variableColumns[variable]
		if len(columns) < 2 {
			continue
		}
		if !owned {
			result := c.fresh(signature)
			if err = c.emit(NewCloneInstruction(current, result), rule); err != nil {
				return 0, Signature{}, false, nil, err
			}
			current, owned = result, true
		}
		if err = c.emit(NewFilterIdenticalInstruction(current, columns), rule); err != nil {
			return 0, Signature{}, false, nil, err
		}
	}
	return current, signature, owned, variableColumns, nil
}

func (c *finiteCompiler) twoTailBody(rule finiteCompiledRule, sources [2]uint32) (uint32, Signature, bool, map[uint32][]int, error) {
	left, leftSignature, leftOwned, leftVariables, err := c.prepareTail(rule.rule, rule.tail, sources[0])
	if err != nil {
		return 0, Signature{}, false, nil, err
	}
	right, rightSignature, rightOwned, rightVariables, err := c.prepareTail(rule.rule, rule.tail2, sources[1])
	if err != nil {
		return 0, Signature{}, false, nil, err
	}
	leftJoin := make([]int, 0)
	rightJoin := make([]int, 0)
	// variable_intersection::populate scans the left columns in order.
	for _, variable := range variablesByFirstColumn(leftVariables) {
		leftColumns, rightColumns := leftVariables[variable], rightVariables[variable]
		if len(leftColumns) != 0 && len(rightColumns) != 0 {
			leftJoin = append(leftJoin, leftColumns[0])
			rightJoin = append(rightJoin, rightColumns[0])
		}
	}
	domains := append(leftSignature.Domains(), rightSignature.domains...)
	joinedSignature, err := NewSignature(domains)
	if err != nil {
		return 0, Signature{}, false, nil, err
	}
	variableColumns := copyVariableColumns(leftVariables)
	offset := leftSignature.Len()
	for variable, columns := range rightVariables {
		for _, column := range columns {
			variableColumns[variable] = append(variableColumns[variable], offset+column)
		}
	}
	removed, err := c.joinProjectionColumns(rule, joinedSignature.Len(), variableColumns)
	if err != nil {
		return 0, Signature{}, false, nil, err
	}
	if len(removed) != 0 {
		joinedSignature, err = projectedSignature(joinedSignature, removed)
		if err != nil {
			return 0, Signature{}, false, nil, err
		}
	}
	joined := c.fresh(joinedSignature)
	var join Instruction
	if len(removed) != 0 {
		join = NewJoinProjectInstruction(left, right, leftJoin, rightJoin, removed, joined)
	} else {
		join = NewJoinInstruction(left, right, leftJoin, rightJoin, joined)
	}
	if err = c.emit(join, rule.rule); err != nil {
		return 0, Signature{}, false, nil, err
	}
	if leftOwned {
		if err = c.emit(NewDeallocateInstruction(left), rule.rule); err != nil {
			return 0, Signature{}, false, nil, err
		}
	}
	if rightOwned && right != left {
		if err = c.emit(NewDeallocateInstruction(right), rule.rule); err != nil {
			return 0, Signature{}, false, nil, err
		}
	}
	return joined, joinedSignature, true, variableColumns, nil
}

// joinProjectionColumns follows get_local_indexes_for_projection: preserve
// one column per head occurrence, or one for a later negative/interpreted use,
// and remove the earliest surplus occurrences from the joined relation.
// prepareTail has already filtered repeated variables and selected constants.
// Join keys still refer to the input columns; only the result map is remapped.
func (c *finiteCompiler) joinProjectionColumns(rule finiteCompiledRule, width int, variables map[uint32][]int) ([]int, error) {
	needed := make(map[uint32]int)
	count, err := c.manager.GetNumArgs(rule.head)
	if err != nil {
		return nil, err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := c.manager.GetArg(rule.head, index)
		if err != nil {
			return nil, err
		}
		kind, err := c.manager.GetKind(argument)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTVar {
			variable, err := c.manager.GetVarIndex(argument)
			if err != nil {
				return nil, err
			}
			needed[variable]++
		}
	}
	later := make(map[uint32]ast.ASTHandle)
	for _, terms := range [][]ast.ASTHandle{rule.negative, rule.interpreted} {
		for _, term := range terms {
			if err := c.conditionVariables(term, later); err != nil {
				return nil, err
			}
		}
	}
	for variable := range later {
		if needed[variable] == 0 {
			needed[variable] = 1
		}
	}
	remove := make([]bool, width)
	for variable, columns := range variables {
		for index := 0; index < len(columns)-needed[variable]; index++ {
			remove[columns[index]] = true
		}
	}
	removed := make([]int, 0)
	positions := make([]int, width)
	for column := 0; column < width; column++ {
		positions[column] = column - len(removed)
		if remove[column] {
			removed = append(removed, column)
		}
	}
	for variable, columns := range variables {
		kept := columns[:0]
		for _, column := range columns {
			if !remove[column] {
				kept = append(kept, positions[column])
			}
		}
		if len(kept) == 0 {
			delete(variables, variable)
		} else {
			variables[variable] = kept
		}
	}
	return removed, nil
}

func (c *finiteCompiler) filterNegatives(rule finiteCompiledRule, predicate ast.ASTHandle, current uint32, signature Signature, owned, present bool, variableColumns map[uint32][]int) (uint32, Signature, bool, bool, map[uint32][]int, error) {
	if !present {
		seed, err := c.emptyTotalRegister(rule.rule, predicate)
		if err != nil {
			return 0, Signature{}, false, false, nil, err
		}
		current, present = seed, true
	}
	for _, application := range rule.negative {
		negativePredicate, err := c.manager.GetDecl(application)
		if err != nil {
			return 0, Signature{}, false, false, nil, err
		}
		sorts, negativeSignature, err := c.relations.PredicateSignature(negativePredicate)
		if err != nil {
			return 0, Signature{}, false, false, nil, err
		}
		count, err := c.manager.GetNumArgs(application)
		if err != nil {
			return 0, Signature{}, false, false, nil, err
		}
		for index := uint32(0); index < count; index++ {
			argument, x := c.manager.GetArg(application, index)
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			kind, x := c.manager.GetKind(argument)
			if x != nil || kind != ast.ASTVar {
				if x != nil {
					return 0, Signature{}, false, false, nil, x
				}
				continue
			}
			variable, x := c.manager.GetVarIndex(argument)
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			if len(variableColumns[variable]) != 0 {
				continue
			}
			domain, x := negativeSignature.Domain(int(index))
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			if x = unsafeRuleColumnVerbose(c.manager, rule.rule, int(variable)); x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			total, x := c.totalRegister(rule.rule, predicate, sorts[index], domain)
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			column := signature.Len()
			current, signature, owned, present, x = c.appendColumn(current, signature, owned, present, total, domain, rule.rule)
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			variableColumns[variable] = []int{column}
		}
		// neg_intersection.populate(single_res_expr, neg_tail): every pair of
		// a result column and a negated-tail argument holding the same
		// variable, result columns outermost.
		columnVariables := make(map[int]uint32)
		for variable, columns := range variableColumns {
			for _, column := range columns {
				columnVariables[column] = variable
			}
		}
		var targetColumns, negativeColumns []int
		for column := 0; column < signature.Len(); column++ {
			variable, found := columnVariables[column]
			if !found {
				continue
			}
			for index := uint32(0); index < count; index++ {
				argument, x := c.manager.GetArg(application, index)
				if x != nil {
					return 0, Signature{}, false, false, nil, x
				}
				kind, x := c.manager.GetKind(argument)
				if x != nil {
					return 0, Signature{}, false, false, nil, x
				}
				if kind != ast.ASTVar {
					continue
				}
				argVariable, x := c.manager.GetVarIndex(argument)
				if x != nil {
					return 0, Signature{}, false, false, nil, x
				}
				if argVariable == variable {
					targetColumns = append(targetColumns, column)
					negativeColumns = append(negativeColumns, int(index))
				}
			}
		}
		// Constant arguments: make_add_constant_column joins a singleton
		// column onto the result and filters on it.
		for index := uint32(0); index < count; index++ {
			argument, x := c.manager.GetArg(application, index)
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			kind, x := c.manager.GetKind(argument)
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			if kind == ast.ASTVar {
				continue
			}
			value, x := c.relations.EncodeValue(sorts[index], argument)
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			domain, x := negativeSignature.Domain(int(index))
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			singleton, x := c.constantRegister(rule.rule, predicate, sorts[index], domain, value)
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			column := signature.Len()
			current, signature, owned, present, x = c.appendColumn(current, signature, owned, present, singleton, domain, rule.rule)
			if x != nil {
				return 0, Signature{}, false, false, nil, x
			}
			targetColumns = append(targetColumns, column)
			negativeColumns = append(negativeColumns, int(index))
		}
		negative := c.predicateRegs[negativePredicate]
		if !owned {
			clone := c.fresh(signature)
			if err = c.emit(NewCloneInstruction(current, clone), rule.rule); err != nil {
				return 0, Signature{}, false, false, nil, err
			}
			current, owned = clone, true
		}
		if err = c.emit(NewFilterByNegationInstruction(current, negative, targetColumns, negativeColumns), rule.rule); err != nil {
			return 0, Signature{}, false, false, nil, err
		}
	}
	return current, signature, owned, present, variableColumns, nil
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

func (c *finiteCompiler) ruleVariableSorts(rule finiteCompiledRule) (map[uint32]ast.ASTHandle, error) {
	result := make(map[uint32]ast.ASTHandle)
	roots := []ast.ASTHandle{rule.head, rule.tail, rule.tail2}
	roots = append(roots, rule.negative...)
	roots = append(roots, rule.interpreted...)
	for _, root := range roots {
		if root.IsNull() {
			continue
		}
		if err := c.conditionVariables(root, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (c *finiteCompiler) relationSortsForVariables(signature Signature, variableColumns map[uint32][]int, variableSorts map[uint32]ast.ASTHandle) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, signature.Len())
	assigned := make([]bool, signature.Len())
	for variable, columns := range variableColumns {
		sort, found := variableSorts[variable]
		if !found || sort.IsNull() {
			return nil, ErrCompilerUnsupported
		}
		domain, err := c.relations.sortDomain(sort)
		if err != nil {
			return nil, err
		}
		for _, column := range columns {
			if column < 0 || column >= signature.Len() || signature.domains[column] != domain {
				return nil, ErrCompilerUnsupported
			}
			result[column], assigned[column] = sort, true
		}
	}
	for _, found := range assigned {
		if !found {
			return nil, ErrCompilerUnsupported
		}
	}
	return result, nil
}

// interpretedBinding holds compile_rule's binding vector: binding[n-v] is
// the column variable for rule variable v. The vector lives until the end of
// compile_rule, so its references are released by release.
type interpretedBinding struct {
	binding []ast.ASTHandle
	refs    []ast.ASTRef
}

func (b *interpretedBinding) release(m *ast.ASTManager) (result error) {
	if b == nil {
		return nil
	}
	for index := len(b.refs) - 1; index >= 0; index-- {
		result = errors.Join(result, m.DelRef(&b.refs[index]))
	}
	b.refs = nil
	return result
}

// prepareInterpreted ports compile_rule's "add unbound columns for
// interpreted filter" block (dl_compiler.cpp 602-640): the interpreted tail
// is conjoined, unbound condition variables receive total columns, and every
// condition variable is bound to a variable naming its (last) column.
func (c *finiteCompiler) prepareInterpreted(rule finiteCompiledRule, predicate ast.ASTHandle, current uint32, signature Signature, owned, present bool, variableColumns map[uint32][]int, binding *interpretedBinding) (_ uint32, _ Signature, _ bool, _ bool, resultErr error) {
	condition, err := c.manager.MkAndArray(rule.interpreted)
	if err != nil {
		return 0, Signature{}, false, false, err
	}
	conditionRef, err := c.manager.NewRef(condition)
	if err != nil {
		return 0, Signature{}, false, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&conditionRef)) }()
	freeVars, err := astrewriter.NewExprFreeVars(c.manager)
	if err != nil {
		return 0, Signature{}, false, false, err
	}
	if err = freeVars.Collect(condition); err != nil {
		return 0, Signature{}, false, false, err
	}
	size := freeVars.Size()
	binding.binding = make([]ast.ASTHandle, size+1)
	for variable := uint32(0); variable < size; variable++ {
		if !freeVars.Contains(variable) {
			continue
		}
		variableSort, err := freeVars.Get(variable)
		if err != nil {
			return 0, Signature{}, false, false, err
		}
		var column int
		if columns := variableColumns[variable]; len(columns) != 0 {
			column = columns[len(columns)-1]
		} else {
			sortSize, err := c.manager.GetSortSize(variableSort)
			if err != nil || !sortSize.IsFinite() {
				return 0, Signature{}, false, false, errors.Join(err, ErrCompilerUnsupported)
			}
			domain, err := sortSize.Size()
			if err != nil {
				return 0, Signature{}, false, false, err
			}
			if err = unsafeRuleColumnVerbose(c.manager, rule.rule, 0); err != nil {
				return 0, Signature{}, false, false, err
			}
			total, err := c.totalRegister(rule.rule, predicate, variableSort, domain)
			if err != nil {
				return 0, Signature{}, false, false, err
			}
			column = signature.Len()
			if !present {
				column = 0
			}
			current, signature, owned, present, err = c.appendColumn(current, signature, owned, present, total, domain, rule.rule)
			if err != nil {
				return 0, Signature{}, false, false, err
			}
			variableColumns[variable] = []int{column}
		}
		renamed, err := c.manager.MkVar(uint32(column), variableSort)
		if err != nil {
			return 0, Signature{}, false, false, err
		}
		reference, err := c.manager.NewRef(renamed)
		if err != nil {
			return 0, Signature{}, false, false, err
		}
		binding.refs = append(binding.refs, reference)
		binding.binding[size-variable] = renamed
	}
	return current, signature, owned, present, nil
}

// filterInterpreted ports compile_rule's "enforce interpreted tail
// predicates" block (dl_compiler.cpp 682-740): one conjoined condition is
// renamed to column variables and filtered, projecting away the columns of
// variables absent from the head.
func (c *finiteCompiler) filterInterpreted(rule finiteCompiledRule, predicate ast.ASTHandle, current uint32, signature Signature, owned, present bool, variableColumns map[uint32][]int, binding *interpretedBinding) (_ uint32, _ Signature, _ bool, _ bool, _ map[uint32][]int, resultErr error) {
	if !present {
		seed, err := c.emptyTotalRegister(rule.rule, predicate)
		if err != nil {
			return 0, Signature{}, false, false, nil, err
		}
		current, present = seed, true
	}
	condition, err := c.manager.MkAndArray(rule.interpreted)
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	conditionRef, err := c.manager.NewRef(condition)
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&conditionRef)) }()
	sourceVariableColumns := copyVariableColumns(variableColumns)
	headVariables := make(map[uint32]ast.ASTHandle)
	if err := c.conditionVariables(rule.head, headVariables); err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	removed := make([]int, 0)
	for variable, sourceColumns := range variableColumns {
		if _, found := headVariables[variable]; found {
			continue
		}
		removed = append(removed, sourceColumns...)
		delete(variableColumns, variable)
	}
	if len(removed) != 0 {
		offsets := make([]int, signature.Len())
		for _, removedColumn := range removed {
			for column := removedColumn; column < len(offsets); column++ {
				offsets[column]++
			}
		}
		for variable, survivingColumns := range variableColumns {
			for index, column := range survivingColumns {
				variableColumns[variable][index] = column - offsets[column]
			}
		}
	}
	renamedRef, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&renamedRef)) }()
	if c.relations.varSubst == nil {
		return 0, Signature{}, false, false, nil, ErrCompilerUnsupported
	}
	err = c.relations.varSubst.Substitute(condition, binding.binding, renamedRef)
	err = errors.Join(err, c.relations.varSubst.Reset())
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	renamed, err := c.manager.RefGet(renamedRef)
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	columns, sorts, err := c.renamedConditionColumns(renamed)
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	if len(removed) == 0 {
		if !owned {
			clone := c.fresh(signature)
			if err := c.emit(NewCloneInstruction(current, clone), rule.rule); err != nil {
				return 0, Signature{}, false, false, nil, err
			}
			current, owned = clone, true
		}
		instruction, err := NewFilterInterpretedInstruction(c.manager, c.relations, c.guard, current, []ast.ASTHandle{renamed}, columns, sorts)
		if err != nil {
			return 0, Signature{}, false, false, nil, err
		}
		if err = c.emit(instruction, rule.rule); err != nil {
			return 0, Signature{}, false, false, nil, err
		}
		return current, signature, owned, present, variableColumns, nil
	}
	sort.Ints(removed)
	allVariableSorts, err := c.ruleVariableSorts(rule)
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	relationSorts, err := c.relationSortsForVariables(signature, sourceVariableColumns, allVariableSorts)
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	projected, err := projectedSignature(signature, removed)
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	result := current
	if owned {
		c.signatures[result] = projected
	} else {
		result = c.fresh(projected)
	}
	instruction, err := NewFilterInterpretedAndProjectInstruction(c.manager, c.relations, c.guard, current, []ast.ASTHandle{renamed}, columns, sorts, relationSorts, removed, result)
	if err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	if err = c.emit(instruction, rule.rule); err != nil {
		return 0, Signature{}, false, false, nil, err
	}
	return result, projected, true, present, variableColumns, nil
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

func (c *finiteCompiler) compileRule(rule finiteCompiledRule, delta uint32, tailSources [2]uint32) (resultErr error) {
	predicate, err := c.manager.GetDecl(rule.head)
	if err != nil {
		return err
	}
	target := c.predicateRegs[predicate]
	var current uint32
	var signature Signature
	owned, present := false, false
	variableColumns := make(map[uint32][]int)
	switch rule.kind {
	case finiteGroundRule:
		// The body starts as the true zero-column relation when filters need it.
	case finiteSingleTailRule:
		current, signature, owned, variableColumns, err = c.prepareTail(rule.rule, rule.tail, tailSources[0])
		present = true
	case finiteTwoTailRule:
		current, signature, owned, variableColumns, err = c.twoTailBody(rule, tailSources)
		present = true
	default:
		err = ErrCompilerUnsupported
	}
	if err != nil {
		return err
	}
	binding := &interpretedBinding{}
	defer func() { resultErr = errors.Join(resultErr, binding.release(c.manager)) }()
	if len(rule.interpreted) != 0 {
		current, signature, owned, present, err = c.prepareInterpreted(rule, predicate, current, signature, owned, present, variableColumns, binding)
		if err != nil {
			return err
		}
	}
	if len(rule.negative) != 0 {
		current, signature, owned, present, variableColumns, err = c.filterNegatives(rule, predicate, current, signature, owned, present, variableColumns)
		if err != nil {
			return err
		}
	}
	if len(rule.interpreted) != 0 {
		current, signature, owned, present, variableColumns, err = c.filterInterpreted(rule, predicate, current, signature, owned, present, variableColumns, binding)
		if err != nil {
			return err
		}
	}
	source, sourceOwned, err := c.assembleHead(rule, predicate, current, signature, owned, present, variableColumns)
	if err != nil {
		return err
	}
	owned = sourceOwned
	if !c.signatures[source].equal(c.signatures[target]) {
		return fmt.Errorf("%w: compiled rule signatures differ", ErrCompilerUnsupported)
	}
	if err = c.emit(NewUnionInstruction(source, target, delta), rule.rule); err != nil {
		return err
	}
	if owned {
		return c.emit(NewDeallocateInstruction(source), rule.rule)
	}
	return nil
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

// storePredicates is the store loop of compiler::do_compilation.
func (c *finiteCompiler) storePredicates() error {
	for _, predicate := range c.loaded {
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
