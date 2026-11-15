package base

// Source: src/muz/base/dl_rule_set.h rule_dependencies and rule_set;
// src/muz/base/dl_rule_set.cpp grouped storage, dependency population,
// reopen/finalize lifecycle, and stratified-negation validation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrRuleSetOwner = errors.New("muz/base: invalid or closed rule set")

// RuleSet owns retained references to a Horn program's rules. Finalize builds
// dependencies and validates stratified negation; Reopen discards that derived
// state before mutation.
type RuleSet struct {
	manager      *RuleManager
	contextOwner *DatalogContext
	rules        []*Rule
	byHead       map[ast.ASTHandle][]*Rule
	headOrder    []ast.ASTHandle
	// headTable is m_head2rules' key layout (obj_map<func_decl, rule_vector*>):
	// display walks it in table order, and del_rule never removes a key, so
	// an emptied head still counts as a predicate until the set is reset.
	headTable           *ast.ObjMap
	groupBuckets        []ruleGroupBucket
	groupCount          int
	dependencies        map[ast.ASTHandle][]ast.ASTHandle
	strata              map[ast.ASTHandle]uint32
	strats              [][]ast.ASTHandle
	outputs             map[ast.ASTHandle]ast.ASTRef
	outputOrder         []ast.ASTHandle
	originalToPredicate map[ast.ASTHandle]ast.ASTHandle
	predicateToOriginal map[ast.ASTHandle]ast.ASTHandle
	mappingReferences   []ast.ASTRef
	finalized           bool
	closed              bool
	cleanupPending      bool
}

func NewRuleSet(manager *RuleManager) (*RuleSet, error) {
	if manager == nil {
		return nil, ErrRuleSetOwner
	}
	if err := manager.open(); err != nil {
		return nil, err
	}
	return &RuleSet{
		manager: manager, byHead: make(map[ast.ASTHandle][]*Rule),
		outputs:             make(map[ast.ASTHandle]ast.ASTRef),
		originalToPredicate: make(map[ast.ASTHandle]ast.ASTHandle),
		predicateToOriginal: make(map[ast.ASTHandle]ast.ASTHandle),
	}, nil
}

func (s *RuleSet) openForCleanup() error {
	if s == nil || s.closed || s.manager == nil {
		return ErrRuleSetOwner
	}
	if owner := s.contextOwner; owner != nil && (owner.scopeRecovery || owner.closing || len(owner.scopes) != 0 && owner.scopes[len(owner.scopes)-1].popping) {
		return ErrRuleSetOwner
	}
	return s.manager.open()
}

func (s *RuleSet) open() error {
	if err := s.openForCleanup(); err != nil {
		return err
	}
	if s.cleanupPending {
		return ErrRuleSetOwner
	}
	return nil
}

// RuleManager returns the borrowed manager used to construct rules in this set.
func (s *RuleSet) RuleManager() (*RuleManager, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.manager, nil
}

func (s *RuleSet) ruleHeadDeclaration(rule *Rule) (ast.ASTHandle, error) {
	state, err := rule.open()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if state.manager != s.manager.manager {
		return ast.ASTHandle{}, fmt.Errorf("muz/base: rule belongs to another AST manager")
	}
	return state.manager.GetDecl(state.head)
}

// Add retains rule and indexes it under its head predicate.
func (s *RuleSet) Add(rule *Rule) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.finalized {
		return fmt.Errorf("muz/base: reopen rule set before mutation")
	}
	declaration, err := s.ruleHeadDeclaration(rule)
	if err != nil {
		return err
	}
	hash, err := s.manager.manager.GetHash(declaration)
	if err != nil {
		return err
	}
	owned, err := rule.Retain()
	if err != nil {
		return err
	}
	s.rules = append(s.rules, owned)
	s.insertGroup(declaration, hash)
	if len(s.byHead[declaration]) == 0 {
		s.headOrder = append(s.headOrder, declaration)
	}
	if s.headTable == nil {
		s.headTable = ast.NewObjMap(s.manager.manager)
	}
	if found, err := s.headTable.Contains(declaration); err != nil {
		return err
	} else if !found {
		if err = s.headTable.Insert(declaration, [2]ast.ASTHandle{}); err != nil {
			return err
		}
	}
	s.byHead[declaration] = append(s.byHead[declaration], owned)
	return nil
}

// Remove drops the first rule with the same native rule state. Like the
// source vectors, removal swaps in the final entry and does not preserve order.
func (s *RuleSet) Remove(rule *Rule) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.finalized {
		return fmt.Errorf("muz/base: reopen rule set before mutation")
	}
	state, err := rule.open()
	if err != nil {
		return err
	}
	declaration, err := s.ruleHeadDeclaration(rule)
	if err != nil {
		return err
	}
	index := -1
	for i, candidate := range s.rules {
		if candidate.state == state {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("muz/base: rule is not in rule set")
	}
	owned := s.rules[index]
	s.rules[index] = s.rules[len(s.rules)-1]
	s.rules = s.rules[:len(s.rules)-1]
	bucket := s.byHead[declaration]
	for i, candidate := range bucket {
		if candidate.state == state {
			bucket[i] = bucket[len(bucket)-1]
			bucket = bucket[:len(bucket)-1]
			break
		}
	}
	if len(bucket) == 0 {
		delete(s.byHead, declaration)
		for i, head := range s.headOrder {
			if head == declaration {
				copy(s.headOrder[i:], s.headOrder[i+1:])
				s.headOrder = s.headOrder[:len(s.headOrder)-1]
				break
			}
		}
	} else {
		s.byHead[declaration] = bucket
	}
	return owned.Close()
}

// AddRules retains every rule and predicate mapping from another set.
func (s *RuleSet) AddRules(other *RuleSet) error {
	if err := s.open(); err != nil {
		return err
	}
	if other == nil || other.manager != s.manager {
		return ErrRuleSetOwner
	}
	if err := other.open(); err != nil {
		return err
	}
	if other == s {
		return nil
	}
	if s.finalized {
		return fmt.Errorf("muz/base: reopen rule set before mutation")
	}
	for _, rule := range other.rules {
		if err := s.Add(rule); err != nil {
			return err
		}
	}
	return s.InheritPredicates(other)
}

func (s *RuleSet) Len() (int, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return len(s.rules), nil
}

// Rule returns a new owning reference to the indexed rule.
func (s *RuleSet) Rule(index int) (*Rule, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if index < 0 || index >= len(s.rules) {
		return nil, fmt.Errorf("muz/base: rule index out of bounds")
	}
	return s.rules[index].Retain()
}

// Contains is rule_set::contains: whether some rule ever had the predicate
// as its head (m_head2rules keeps a key after its rules are deleted).
func (s *RuleSet) Contains(declaration ast.ASTHandle) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if s.headTable == nil {
		return false, nil
	}
	return s.headTable.Contains(declaration)
}

// PredicateRules returns new owning references in the rule set's current
// grouped order. The caller must close each result.
func (s *RuleSet) PredicateRules(declaration ast.ASTHandle) ([]*Rule, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	source := s.byHead[declaration]
	result := make([]*Rule, 0, len(source))
	for _, rule := range source {
		owned, err := rule.Retain()
		if err != nil {
			for _, prior := range result {
				err = errors.Join(err, prior.Close())
			}
			return nil, err
		}
		result = append(result, owned)
	}
	return result, nil
}

func (s *RuleSet) retainMappingDeclaration(declaration ast.ASTHandle) error {
	reference, err := s.manager.manager.NewRef(declaration)
	if err != nil {
		return err
	}
	s.mappingReferences = append(s.mappingReferences, reference)
	return nil
}

// SetOutputPredicate retains a predicate declaration as a program output.
func (s *RuleSet) SetOutputPredicate(declaration ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if _, registered := s.manager.predicates[declaration]; !registered {
		return fmt.Errorf("muz/base: output predicate is not registered")
	}
	if _, found := s.outputs[declaration]; found {
		return nil
	}
	reference, err := s.manager.manager.NewRef(declaration)
	if err != nil {
		return err
	}
	s.outputs[declaration] = reference
	s.outputOrder = append(s.outputOrder, declaration)
	return nil
}

func (s *RuleSet) IsOutputPredicate(declaration ast.ASTHandle) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	_, found := s.outputs[declaration]
	return found, nil
}

func (s *RuleSet) OutputPredicates() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), s.outputOrder...), nil
}

func (s *RuleSet) OutputPredicate() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(s.outputOrder) != 1 {
		return ast.ASTHandle{}, fmt.Errorf("muz/base: expected exactly one output predicate")
	}
	return s.outputOrder[0], nil
}

func (s *RuleSet) OriginalPredicate(predicate ast.ASTHandle) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if original, found := s.predicateToOriginal[predicate]; found {
		return original, nil
	}
	return predicate, nil
}

func (s *RuleSet) DerivedPredicate(original ast.ASTHandle) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if predicate, found := s.originalToPredicate[original]; found {
		return predicate, nil
	}
	return original, nil
}

// InheritPredicate records a transformed predicate's original declaration and
// propagates output status from the source program.
func (s *RuleSet) InheritPredicate(other *RuleSet, original, predicate ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if other == nil || other.manager != s.manager {
		return ErrRuleSetOwner
	}
	if err := other.open(); err != nil {
		return err
	}
	isOutput, err := other.IsOutputPredicate(original)
	if err != nil {
		return err
	}
	if isOutput {
		if err = s.SetOutputPredicate(predicate); err != nil {
			return err
		}
	}
	original, err = other.OriginalPredicate(original)
	if err != nil {
		return err
	}
	if err = s.retainMappingDeclaration(original); err != nil {
		return err
	}
	if err = s.retainMappingDeclaration(predicate); err != nil {
		return err
	}
	s.originalToPredicate[original] = predicate
	s.predicateToOriginal[predicate] = original
	return nil
}

// InheritPredicates copies output predicates and original/derived mappings.
func (s *RuleSet) InheritPredicates(other *RuleSet) error {
	if err := s.open(); err != nil {
		return err
	}
	if other == nil || other.manager != s.manager {
		return ErrRuleSetOwner
	}
	if err := other.open(); err != nil {
		return err
	}
	for _, declaration := range other.outputOrder {
		if err := s.SetOutputPredicate(declaration); err != nil {
			return err
		}
	}
	for original, predicate := range other.originalToPredicate {
		if err := s.retainMappingDeclaration(original); err != nil {
			return err
		}
		if err := s.retainMappingDeclaration(predicate); err != nil {
			return err
		}
		s.originalToPredicate[original] = predicate
		s.predicateToOriginal[predicate] = original
	}
	return nil
}

// Finalize is rule_set::close: rule_dependencies::populate over
// m_head2rules, rule_stratifier::process (Gabow's components ordered by
// Kahn's algorithm, then reversed) and stratified_negation. It returns false,
// discarding the derived state, when a negated tail shares its head's stratum.
func (s *RuleSet) Finalize() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if s.finalized {
		return false, fmt.Errorf("muz/base: rule set is already finalized")
	}
	deps := newSourceDependencies(s.manager.manager)
	if s.headTable != nil {
		for _, entry := range s.headTable.Entries() {
			for _, rule := range s.byHead[entry.Key] {
				if err := s.populateRuleDependencies(deps, rule); err != nil {
					return false, err
				}
			}
		}
	}
	stratifier := &sourceStratifier{manager: s.manager.manager, deps: deps, preorder: make(map[ast.ASTHandle]uint32), componentNums: make(map[ast.ASTHandle]int)}
	strats, err := stratifier.process()
	if err != nil {
		return false, err
	}
	// m_pred_strat_nums; a predicate absent from the program has stratum 0.
	strata := make(map[ast.ASTHandle]uint32)
	for index, stratum := range strats {
		for _, declaration := range stratum {
			strata[declaration] = uint32(index)
		}
	}
	// rule_set::stratified_negation.
	for _, rule := range s.rules {
		state, err := rule.open()
		if err != nil {
			return false, err
		}
		head, err := state.manager.GetDecl(state.head)
		if err != nil {
			return false, err
		}
		for i := state.positive; i < state.uninterpreted; i++ {
			tail, err := state.manager.GetDecl(state.tail[i].handle)
			if err != nil {
				return false, err
			}
			if strata[head] == strata[tail] {
				return false, nil
			}
		}
	}
	// get_deps(f): the item_set of masters in table order.
	dependencies := make(map[ast.ASTHandle][]ast.ASTHandle, deps.keys.Len())
	for _, key := range deps.keys.Entries() {
		masters := deps.masters(key.Key)
		list := make([]ast.ASTHandle, len(masters))
		for i := range masters {
			list[i] = masters[i].Key
		}
		dependencies[key.Key] = list
	}
	s.dependencies = dependencies
	s.strata = strata
	s.strats = strats
	s.finalized = true
	return true, nil
}

func (s *RuleSet) Reopen() error {
	if err := s.open(); err != nil {
		return err
	}
	s.dependencies = nil
	s.strata = nil
	s.strats = nil
	s.finalized = false
	return nil
}

func (s *RuleSet) IsFinalized() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	return s.finalized, nil
}

func (s *RuleSet) Dependencies(declaration ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if !s.finalized {
		return nil, fmt.Errorf("muz/base: finalize rule set before reading dependencies")
	}
	return append([]ast.ASTHandle(nil), s.dependencies[declaration]...), nil
}

func (s *RuleSet) PredicateStratum(declaration ast.ASTHandle) (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if !s.finalized {
		return 0, fmt.Errorf("muz/base: finalize rule set before reading strata")
	}
	return s.strata[declaration], nil
}

// SplitFounded partitions head predicates using the native least fixed point:
// a predicate is founded when one of its rules depends only on founded
// predicate tails.
func (s *RuleSet) SplitFounded() (founded, nonFounded []ast.ASTHandle, err error) {
	if err = s.open(); err != nil {
		return nil, nil, err
	}
	foundedSet := make(map[ast.ASTHandle]bool, len(s.headOrder))
	remaining := make(map[ast.ASTHandle]bool, len(s.headOrder))
	for _, declaration := range s.headOrder {
		remaining[declaration] = true
	}
	changed := true
	for changed {
		changed = false
		for _, declaration := range s.headOrder {
			if !remaining[declaration] {
				continue
			}
			predicateFounded := false
			for _, rule := range s.byHead[declaration] {
				state, e := rule.open()
				if e != nil {
					return nil, nil, e
				}
				predicateFounded = true
				for i := 0; i < state.uninterpreted; i++ {
					dependency, x := state.manager.GetDecl(state.tail[i].handle)
					if x != nil {
						return nil, nil, x
					}
					if !foundedSet[dependency] {
						predicateFounded = false
						break
					}
				}
				if predicateFounded {
					break
				}
			}
			if predicateFounded {
				foundedSet[declaration] = true
				remaining[declaration] = false
				founded = append(founded, declaration)
				changed = true
			}
		}
	}
	for _, declaration := range s.headOrder {
		if remaining[declaration] {
			nonFounded = append(nonFounded, declaration)
		}
	}
	return founded, nonFounded, nil
}

func (s *RuleSet) resetContents() error {
	if err := s.openForCleanup(); err != nil {
		return err
	}
	return s.resetContentsUnchecked()
}

func (s *RuleSet) resetContentsUnchecked() error {
	if _, err := s.manager.manager.ProofMode(); err != nil {
		return err
	}
	s.cleanupPending = true
	// m_rules.reset(): rule_ref_vector releases front to back.
	for len(s.rules) > 0 {
		if err := s.rules[0].Close(); err != nil {
			return err
		}
		s.rules[0] = nil
		s.rules = s.rules[1:]
	}
	for _, declaration := range s.outputOrder {
		reference, held := s.outputs[declaration]
		if !held {
			continue
		}
		ref := reference
		err := s.manager.manager.DelRef(&ref)
		if ref == (ast.ASTRef{}) {
			delete(s.outputs, declaration)
		} else {
			s.outputs[declaration] = ref
		}
		if err != nil {
			return err
		}
	}
	// m_refs.reset(): front to back.
	for len(s.mappingReferences) > 0 {
		err := s.manager.manager.DelRef(&s.mappingReferences[0])
		if s.mappingReferences[0] == (ast.ASTRef{}) {
			s.mappingReferences = s.mappingReferences[1:]
		}
		if err != nil {
			return err
		}
	}
	s.rules = nil
	s.byHead = make(map[ast.ASTHandle][]*Rule)
	s.headOrder = nil
	s.headTable = nil
	s.resetGroups()
	s.dependencies = nil
	s.strata = nil
	s.strats = nil
	s.outputs = make(map[ast.ASTHandle]ast.ASTRef)
	s.outputOrder = nil
	s.originalToPredicate = make(map[ast.ASTHandle]ast.ASTHandle)
	s.predicateToOriginal = make(map[ast.ASTHandle]ast.ASTHandle)
	s.mappingReferences = nil
	s.finalized = false
	s.cleanupPending = false
	return nil
}

// Reset releases all program state while keeping the set reusable. If a
// release fails after cleanup starts, only Reset, ReplaceRules and Close may
// resume the set until the remaining owners have been released.
func (s *RuleSet) Reset() error {
	if err := s.openForCleanup(); err != nil {
		return err
	}
	return s.resetContents()
}

// ReplaceRules resets this set and retains all rules and predicate mappings
// from other.
func (s *RuleSet) ReplaceRules(other *RuleSet) error {
	if err := s.openForCleanup(); err != nil {
		return err
	}
	if other == s {
		if s.cleanupPending {
			return ErrRuleSetOwner
		}
		return nil
	}
	if other == nil || other.manager != s.manager {
		return ErrRuleSetOwner
	}
	if err := other.open(); err != nil {
		return err
	}
	if err := s.resetContents(); err != nil {
		return err
	}
	return s.AddRules(other)
}

func (s *RuleSet) Close() error {
	if s == nil || s.manager == nil {
		return ErrRuleSetOwner
	}
	if s.closed {
		return nil
	}
	if err := s.openForCleanup(); err != nil {
		return err
	}
	return s.closeContents()
}

// closeContents is used by the owning context during guarded scope recovery.
func (s *RuleSet) closeContents() error {
	if err := s.resetContentsUnchecked(); err != nil {
		return err
	}
	s.closed = true
	return nil
}
