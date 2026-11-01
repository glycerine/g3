package fp

// Source: src/muz/base/dl_context.cpp add_rule/query orchestration and
// src/muz/base/dl_engine_base.h query contract. This is the retained stratified
// finite-domain Horn execution path.

import (
	"errors"
	"fmt"
	"io"
	"math/bits"
	"sort"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/ddnf"
	muzpdr "github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/muz/rel"
	muzspacer "github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var (
	ErrPropositionalEngine = errors.New("muz/fp: invalid or closed propositional engine")
	ErrPropositionalHorn   = errors.New("muz/fp: positive finite-domain Horn fragment required")
)

type predicateState struct {
	sorts []ast.ASTHandle
	table *rel.HashTable
}

type ddnfEngineState struct {
	output           io.Writer
	registry         *ddnf.Registry
	registryBorrowed bool
	trail            []ast.ASTRef
}

func (e *PropositionalEngine) predicateState(declaration ast.ASTHandle) (*predicateState, error) {
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, arity)
	for index := uint32(0); index < arity; index++ {
		sorts[index], err = e.manager.GetDomain(declaration, index)
		if err != nil {
			return nil, err
		}
	}
	table, tableErr := e.relations.GetRelation(declaration)
	if tableErr != nil && !errors.Is(tableErr, rel.ErrTable) {
		return nil, tableErr
	}
	return &predicateState{sorts: sorts, table: table}, nil
}

// PropositionalEngine owns the retained rule, finite-relation, and optional
// ground-guard rewriting services.
type PropositionalEngine struct {
	manager             *ast.ASTManager
	rules               *base.RuleManager
	set                 *base.RuleSet
	binder              *base.VariableBinder
	relations           *rel.RelationManager
	guard               *astrewriter.TheoryRewriter
	unboundCooperation  *cooperate.Cooperation
	unboundLite         *qe.Lite
	background          ast.ASTRef
	params              *uparams.ParamsContext
	globals             *uparams.GlobalParamsService
	backend             string
	pdr                 *muzpdr.Context
	pdrRules            *base.RuleSet
	pdrRuleRevision     uint64
	pdrQuery            ast.ASTHandle
	pdrDerived          map[ast.ASTHandle]ast.ASTHandle
	pdrPredicates       map[ast.ASTHandle]ast.ASTHandle
	pdrSlices           map[ast.ASTHandle][]bool
	pdrQuantified       map[ast.ASTHandle]ast.ASTHandle
	pdrInstantiated     map[ast.ASTHandle]ast.ASTHandle
	pdrBitBlasted       map[ast.ASTHandle]ast.ASTHandle
	pdrScaled           map[ast.ASTHandle]ast.ASTHandle
	pdrMagic            map[ast.ASTHandle]ast.ASTHandle
	pdrKarr             map[ast.ASTHandle]rel.KarrMatrix
	pdrConversions      []hornModelConversion
	pdrStatistics       statistics.Statistics
	bmcStatistics       statistics.Statistics
	bmcTrace            []*base.Rule
	bmcProof            ast.ASTRef
	tabAnswer           ast.ASTRef
	tabUnfold           uint32
	tabUnfoldFail       uint32
	tabSubsumed         uint32
	spacer              *muzspacer.Context
	scopes              []*base.RuleSet
	autoRules           []autoRuleEntry
	autoRuleScopes      [][]autoRuleEntry
	assertions          []ast.ASTRef
	assertionScopes     []int
	predicates          map[ast.ASTHandle]*predicateState
	predicateOrder      []ast.ASTHandle
	auxiliary           map[ast.ASTHandle]bool
	compiled            bool
	queried             bool
	queryReady          bool
	queryClone          *PropositionalEngine
	queryClonePredicate ast.ASTHandle
	queryRelations      []ast.ASTHandle
	queryRelationsSet   bool
	queryRevision       uint64
	queryEngine         EngineKind
	lastResult          lbool.LBool
	revision            uint64
	pdrRevision         uint64
	spacerRules         *base.RuleSet
	spacerRevision      uint64
	spacerPredicates    map[ast.ASTHandle]ast.ASTHandle
	spacerSlices        map[ast.ASTHandle][]bool
	spacerQuantified    map[ast.ASTHandle]ast.ASTHandle
	spacerInstantiated  map[ast.ASTHandle]ast.ASTHandle
	spacerBitBlasted    map[ast.ASTHandle]ast.ASTHandle
	spacerScaled        map[ast.ASTHandle]ast.ASTHandle
	spacerMagic         map[ast.ASTHandle]ast.ASTHandle
	spacerKarr          map[ast.ASTHandle]rel.KarrMatrix
	spacerConversions   []hornModelConversion
	lastStatus          string
	forceProofTrace     bool
	queryParams         uparams.ParamsRef
	queryParamsReady    bool
	datalogStats        statistics.Statistics
	datalogElapsed      float64
	closed              bool
	ddnf                ddnfEngineState
}

// NewPropositionalEngineWithServices adds the retained AST theory rewriter
// used to evaluate ground interpreted guards after finite variable binding.
func NewPropositionalEngineWithServices(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*PropositionalEngine, error) {
	engine, err := NewPropositionalEngine(manager)
	if err != nil {
		return nil, err
	}
	engine.guard, err = astrewriter.NewTheoryRewriter(manager, context, globals, nil)
	if err != nil {
		return nil, errors.Join(err, engine.Close())
	}
	engine.params = context
	engine.globals = globals
	return engine, nil
}

func NewPropositionalEngine(manager *ast.ASTManager) (*PropositionalEngine, error) {
	if manager == nil {
		return nil, ErrPropositionalEngine
	}
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		return nil, err
	}
	set, err := base.NewRuleSet(rules)
	if err != nil {
		return nil, errors.Join(err, rules.Close())
	}
	binder, err := base.NewVariableBinder(manager)
	if err != nil {
		return nil, errors.Join(err, set.Close(), rules.Close())
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		return nil, errors.Join(err, binder.Close(), set.Close(), rules.Close())
	}
	truth, err := manager.MkTrue()
	if err != nil {
		return nil, errors.Join(err, relations.Close(), binder.Close(), set.Close(), rules.Close())
	}
	background, err := manager.NewRef(truth)
	if err != nil {
		return nil, errors.Join(err, relations.Close(), binder.Close(), set.Close(), rules.Close())
	}
	return &PropositionalEngine{
		manager: manager, rules: rules, set: set, binder: binder, relations: relations,
		background: background, predicates: make(map[ast.ASTHandle]*predicateState), auxiliary: make(map[ast.ASTHandle]bool), backend: "datalog", revision: 1, lastStatus: "ok",
	}, nil
}

func (e *PropositionalEngine) open() error {
	if e == nil || e.closed || e.manager == nil || e.rules == nil || e.set == nil || e.binder == nil || e.relations == nil {
		return ErrPropositionalEngine
	}
	return nil
}

func (e *PropositionalEngine) RegisterPredicate(declaration ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	kind, err := e.manager.GetKind(declaration)
	if err != nil {
		return err
	}
	if kind != ast.ASTFuncDecl {
		return ErrPropositionalHorn
	}
	rangeSort, err := e.manager.GetRange(declaration)
	if err != nil {
		return err
	}
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if rangeSort != boolean {
		return ErrPropositionalHorn
	}
	if err = e.rules.RegisterPredicate(declaration); err != nil {
		return err
	}
	state, err := e.predicateState(declaration)
	if err != nil {
		return err
	}
	if e.predicates[declaration] == nil {
		e.predicateOrder = append(e.predicateOrder, declaration)
	}
	e.predicates[declaration] = state
	e.revision++
	return nil
}

// SetPredicateRepresentation accepts the native finite-table implementation
// names. The Go backend uses one exact tuple-set
// representation for all four storage-equivalent native table plugins.
func (e *PropositionalEngine) SetPredicateRepresentation(declaration ast.ASTHandle, kinds []string) error {
	if err := e.open(); err != nil {
		return err
	}
	state := e.predicates[declaration]
	if state == nil {
		return ErrPropositionalHorn
	}
	if len(kinds) == 0 {
		return nil
	}
	if e.queried {
		return fmt.Errorf("muz/fp: predicate representation must be selected before querying")
	}
	signature, err := state.table.Signature()
	if err != nil {
		return err
	}
	for _, kind := range kinds {
		switch kind {
		case "tr_hashtable":
		case "tr_sparse", "tr_lazy_sparse":
			if signature.Len() == 0 {
				return fmt.Errorf("muz/fp: relation plugin %s cannot represent a zero-arity predicate", kind)
			}
		case "tr_bitvector":
			shift := 0
			for _, domain := range signature.Domains() {
				if domain > uint64(^uint32(0)) || domain&(domain-1) != 0 {
					return fmt.Errorf("muz/fp: relation plugin bitvector cannot represent the predicate signature")
				}
				shift += bits.Len64(domain)
				if shift >= 32 {
					return fmt.Errorf("muz/fp: relation plugin bitvector cannot represent the predicate signature")
				}
			}
		default:
			return fmt.Errorf("muz/fp: relation plugin %s does not exist", kind)
		}
	}
	e.revision++
	return nil
}

// RegisterVariable retains an implicit Datalog variable application.
func (e *PropositionalEngine) RegisterVariable(variable ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if err := e.binder.AddVariable(variable); err != nil {
		return err
	}
	e.revision++
	return nil
}

// SetBackground replaces the background constraint used by subsequent PDR
// queries. The engine retains the formula independently of the caller.
func (e *PropositionalEngine) SetBackground(background ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	current, err := e.manager.RefGet(e.background)
	if err != nil {
		return err
	}
	if current == background {
		return nil
	}
	if err = e.manager.SetRef(e.background, background); err != nil {
		return err
	}
	e.revision++
	return nil
}

// AssertBackground retains a fixedpoint background assertion in the current
// fixedpoint scope.
func (e *PropositionalEngine) AssertBackground(formula ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	reference, err := e.manager.NewRef(formula)
	if err != nil {
		return err
	}
	e.assertions = append(e.assertions, reference)
	e.revision++
	return nil
}

// BackgroundAssertions returns owning references in insertion order.
func (e *PropositionalEngine) BackgroundAssertions() (result []ast.ASTRef, resultErr error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	result = make([]ast.ASTRef, 0, len(e.assertions))
	defer func() {
		if resultErr != nil {
			for i := len(result) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, e.manager.DelRef(&result[i]))
			}
			result = nil
		}
	}()
	for _, assertion := range e.assertions {
		handle, err := e.manager.RefGet(assertion)
		if err != nil {
			return nil, err
		}
		reference, err := e.manager.NewRef(handle)
		if err != nil {
			return nil, err
		}
		result = append(result, reference)
	}
	return result, nil
}

// ParametersUpdated invalidates a retained backend context so the next query
// reconstructs it from current fixedpoint parameters.
func (e *PropositionalEngine) ParametersUpdated() error {
	if err := e.open(); err != nil {
		return err
	}
	e.revision++
	return nil
}

func (e *PropositionalEngine) AddRule(formula ast.ASTHandle, name string) error {
	if err := e.open(); err != nil {
		return err
	}
	autoKind, err := e.autoFormulaKind(formula)
	if err != nil {
		return err
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return err
	}
	if finalized {
		if err = e.set.Reopen(); err != nil {
			return err
		}
	}
	bound, err := e.binder.Bind(formula, true)
	if err != nil {
		return err
	}
	boundFormula, getErr := e.manager.RefGet(bound)
	if getErr == nil {
		getErr = e.addFormula(e.set, boundFormula, name)
	}
	added := getErr == nil
	getErr = errors.Join(getErr, e.manager.DelRef(&bound))
	if added {
		e.autoRules = append(e.autoRules, autoRuleEntry{name: name, kind: autoKind})
		e.revision++
	}
	return getErr
}

// AddFact adds a ground predicate application as an unnamed rule.
func (e *PropositionalEngine) AddFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if e.predicates[declaration] == nil {
		return ErrPropositionalHorn
	}
	fact, err := e.manager.MkApp(declaration, values)
	if err != nil {
		return err
	}
	return e.AddRule(fact, "")
}

// AddTableFact decodes native finite-table element numbers and adds the
// corresponding ground predicate application.
func (e *PropositionalEngine) AddTableFact(declaration ast.ASTHandle, values []uint32) (resultErr error) {
	if err := e.open(); err != nil {
		return err
	}
	state := e.predicates[declaration]
	if state == nil || len(values) != len(state.sorts) {
		return fmt.Errorf("%w: table fact arity mismatch", ErrPropositionalHorn)
	}
	fact := make(rel.Fact, len(values))
	for i, value := range values {
		fact[i] = rel.Element(value)
	}
	references, err := e.relations.DecodeFact(state.sorts, fact)
	if err != nil {
		return err
	}
	defer func() {
		for i := len(references) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&references[i]))
		}
	}()
	handles := make([]ast.ASTHandle, len(references))
	for i := range references {
		handles[i], err = e.manager.RefGet(references[i])
		if err != nil {
			return err
		}
	}
	return e.AddFact(declaration, handles)
}

// UpdateRule adds a named rule or replaces the unique existing rule with that
// name. A replacement must preserve the head and may only strengthen the body.
func (e *PropositionalEngine) UpdateRule(formula ast.ASTHandle, name string) (resultErr error) {
	if err := e.open(); err != nil {
		return err
	}
	autoKind, err := e.autoFormulaKind(formula)
	if err != nil {
		return err
	}
	candidateSet, err := base.NewRuleSet(e.rules)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, candidateSet.Close()) }()
	bound, err := e.binder.Bind(formula, true)
	if err != nil {
		return err
	}
	boundFormula, err := e.manager.RefGet(bound)
	if err == nil {
		err = e.addFormula(candidateSet, boundFormula, name)
	}
	err = errors.Join(err, e.manager.DelRef(&bound))
	if err != nil {
		return err
	}
	candidateCount, err := candidateSet.Len()
	if err != nil {
		return err
	}
	if candidateCount != 1 {
		return fmt.Errorf("muz/fp: rule %s has a non-trivial body; it cannot be modified", name)
	}
	candidate, err := candidateSet.Rule(0)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, candidate.Close()) }()

	count, err := e.set.Len()
	if err != nil {
		return err
	}
	oldIndex := -1
	for i := 0; i < count; i++ {
		rule, x := e.set.Rule(i)
		if x != nil {
			return x
		}
		ruleName, x := rule.Name()
		x = errors.Join(x, rule.Close())
		if x != nil {
			return x
		}
		if ruleName != name {
			continue
		}
		if oldIndex >= 0 {
			return fmt.Errorf("muz/fp: rule %s occurs twice; it cannot be modified", name)
		}
		oldIndex = i
	}
	if oldIndex >= 0 {
		old, x := e.set.Rule(oldIndex)
		if x != nil {
			return x
		}
		subsumes, x := base.BasicRuleSubsumes(old, candidate)
		x = errors.Join(x, old.Close())
		if x != nil {
			return x
		}
		if !subsumes {
			return fmt.Errorf("muz/fp: old rule %s does not subsume its replacement", name)
		}
	}

	replacement, err := base.NewRuleSet(e.rules)
	if err != nil {
		return err
	}
	keepReplacement := false
	defer func() {
		if !keepReplacement {
			resultErr = errors.Join(resultErr, replacement.Close())
		}
	}()
	for i := 0; i < count; i++ {
		if i == oldIndex {
			continue
		}
		rule, x := e.set.Rule(i)
		if x == nil {
			x = replacement.Add(rule)
		}
		if rule != nil {
			x = errors.Join(x, rule.Close())
		}
		if x != nil {
			return x
		}
	}
	if err = replacement.Add(candidate); err != nil {
		return err
	}
	if err = replacement.InheritPredicates(e.set); err != nil {
		return err
	}
	prior := e.set
	e.set = replacement
	keepReplacement = true
	if oldIndex >= 0 {
		kept := e.autoRules[:0]
		for _, entry := range e.autoRules {
			if entry.name != name {
				kept = append(kept, entry)
			}
		}
		e.autoRules = kept
	}
	e.autoRules = append(e.autoRules, autoRuleEntry{name: name, kind: autoKind})
	e.revision++
	e.pdrRuleRevision = e.revision
	return prior.Close()
}

// Push retains a snapshot of the current rules and background assertions.
// Predicate registrations are intentionally not scoped, matching
// dl_context::push.
func (e *PropositionalEngine) Push() error {
	if err := e.open(); err != nil {
		return err
	}
	snapshot, err := base.NewRuleSet(e.rules)
	if err != nil {
		return err
	}
	if err = snapshot.AddRules(e.set); err != nil {
		return errors.Join(err, snapshot.Close())
	}
	e.scopes = append(e.scopes, snapshot)
	e.autoRuleScopes = append(e.autoRuleScopes, append([]autoRuleEntry(nil), e.autoRules...))
	e.assertionScopes = append(e.assertionScopes, len(e.assertions))
	return nil
}

// Pop restores the most recent rule snapshot. The native Datalog engine only
// permits this before a query creates its execution engine.
func (e *PropositionalEngine) Pop() error {
	if err := e.open(); err != nil {
		return err
	}
	if len(e.scopes) == 0 {
		return fmt.Errorf("muz/fp: there are no backtracking points to pop to")
	}
	if e.queried {
		return fmt.Errorf("muz/fp: pop operation is only supported before a query")
	}
	if len(e.assertionScopes) != len(e.scopes) || len(e.autoRuleScopes) != len(e.scopes) {
		return ErrPropositionalEngine
	}
	last := len(e.scopes) - 1
	snapshot := e.scopes[last]
	if err := e.set.ReplaceRules(snapshot); err != nil {
		return err
	}
	e.revision++
	e.scopes = e.scopes[:last]
	e.autoRules = e.autoRuleScopes[last]
	e.autoRuleScopes = e.autoRuleScopes[:last]
	assertionSize := e.assertionScopes[last]
	e.assertionScopes = e.assertionScopes[:last]
	var result error
	for i := len(e.assertions) - 1; i >= assertionSize; i-- {
		result = errors.Join(result, e.manager.DelRef(&e.assertions[i]))
	}
	e.assertions = e.assertions[:assertionSize]
	return errors.Join(result, snapshot.Close())
}

func (e *PropositionalEngine) validateFragment() error {
	if err := validateRuleSetProperties(e.rules, e.set, e.backend); err != nil {
		return err
	}
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		rule, x := e.set.Rule(i)
		if x != nil {
			return x
		}
		head, x := rule.HeadDeclaration()
		if x == nil && e.predicates[head] == nil {
			x = ErrPropositionalHorn
		}
		positive, pErr := rule.PositiveTailSize()
		uninterpreted, uErr := rule.UninterpretedTailSize()
		tailSize, tErr := rule.TailSize()
		x = errors.Join(x, pErr, uErr, tErr)
		if x == nil && (positive > uninterpreted || uninterpreted > tailSize) {
			x = ErrPropositionalHorn
		}
		for j := 0; x == nil && j < uninterpreted; j++ {
			declaration, y := rule.TailDeclaration(j)
			if y != nil {
				x = y
				break
			}
			if e.predicates[declaration] == nil {
				x = ErrPropositionalHorn
			}
		}
		x = errors.Join(x, rule.Close())
		if x != nil {
			return x
		}
	}
	return nil
}

func (e *PropositionalEngine) normalizeSimpleJoins() (resultErr error) {
	filter := transforms.NewFilterRulesPlugin(e.manager, e.rules, 0)
	plugin := transforms.NewSimpleJoinsPlugin(e.manager, e.rules, 0, e.relations)
	transformer := base.NewRuleTransformer()
	if err := transformer.Register(filter); err != nil {
		return errors.Join(err, filter.Close(), plugin.Close(), transformer.Close())
	}
	if err := transformer.Register(plugin); err != nil {
		return errors.Join(err, transformer.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, transformer.Close()) }()
	modified, err := transformer.Apply(e.set)
	if err != nil || !modified {
		return err
	}
	for _, declaration := range append(filter.FreshPredicates(), plugin.FreshPredicates()...) {
		state, x := e.predicateState(declaration)
		if x != nil {
			return x
		}
		e.predicates[declaration] = state
		e.auxiliary[declaration] = true
	}
	e.revision++
	return nil
}

type variableBinding map[uint32]rel.Element

func cloneBinding(binding variableBinding) variableBinding {
	result := make(variableBinding, len(binding))
	for variable, value := range binding {
		result[variable] = value
	}
	return result
}

func (e *PropositionalEngine) unify(application ast.ASTHandle, state *predicateState, fact rel.Fact, binding variableBinding) (variableBinding, bool, error) {
	result := cloneBinding(binding)
	for i, value := range fact {
		argument, err := e.manager.GetArg(application, uint32(i))
		if err != nil {
			return nil, false, err
		}
		kind, err := e.manager.GetKind(argument)
		if err != nil {
			return nil, false, err
		}
		if kind == ast.ASTVar {
			index, x := e.manager.GetVarIndex(argument)
			if x != nil {
				return nil, false, x
			}
			if previous, found := result[index]; found && previous != value {
				return nil, false, nil
			}
			result[index] = value
			continue
		}
		constant, err := e.relations.EncodeValue(state.sorts[i], argument)
		if err != nil {
			return nil, false, ErrPropositionalHorn
		}
		if constant != value {
			return nil, false, nil
		}
	}
	return result, true, nil
}

func (e *PropositionalEngine) instantiate(application ast.ASTHandle, state *predicateState, binding variableBinding) (rel.Fact, error) {
	fact := make(rel.Fact, len(state.sorts))
	for i, sort := range state.sorts {
		argument, err := e.manager.GetArg(application, uint32(i))
		if err != nil {
			return nil, err
		}
		kind, err := e.manager.GetKind(argument)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTVar {
			index, x := e.manager.GetVarIndex(argument)
			if x != nil {
				return nil, x
			}
			value, found := binding[index]
			if !found {
				return nil, fmt.Errorf("%w: head variable is not bound by a positive tail", ErrPropositionalHorn)
			}
			fact[i] = value
			continue
		}
		fact[i], err = e.relations.EncodeValue(sort, argument)
		if err != nil {
			return nil, ErrPropositionalHorn
		}
	}
	return fact, nil
}

func (e *PropositionalEngine) negativeMatch(application ast.ASTHandle, state *predicateState, fact rel.Fact, binding variableBinding) (bool, error) {
	for i, value := range fact {
		argument, err := e.manager.GetArg(application, uint32(i))
		if err != nil {
			return false, err
		}
		kind, err := e.manager.GetKind(argument)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTVar {
			index, x := e.manager.GetVarIndex(argument)
			if x != nil {
				return false, x
			}
			bound, found := binding[index]
			if !found {
				return false, fmt.Errorf("%w: negative-tail variable is not bound by a positive tail", ErrPropositionalHorn)
			}
			if bound != value {
				return false, nil
			}
			continue
		}
		constant, err := e.relations.EncodeValue(state.sorts[i], argument)
		if err != nil {
			return false, ErrPropositionalHorn
		}
		if constant != value {
			return false, nil
		}
	}
	return true, nil
}

func (e *PropositionalEngine) expandUnbound(application ast.ASTHandle, state *predicateState, bindings []variableBinding) ([]variableBinding, error) {
	signature, err := state.table.Signature()
	if err != nil {
		return nil, err
	}
	for i := range state.sorts {
		argument, x := e.manager.GetArg(application, uint32(i))
		if x != nil {
			return nil, x
		}
		kind, x := e.manager.GetKind(argument)
		if x != nil {
			return nil, x
		}
		if kind != ast.ASTVar {
			continue
		}
		index, x := e.manager.GetVarIndex(argument)
		if x != nil {
			return nil, x
		}
		domain, x := signature.Domain(i)
		if x != nil {
			return nil, x
		}
		next := make([]variableBinding, 0)
		for _, binding := range bindings {
			if _, found := binding[index]; found {
				next = append(next, binding)
				continue
			}
			for value := uint64(0); value < domain; value++ {
				expanded := cloneBinding(binding)
				expanded[index] = rel.Element(value)
				next = append(next, expanded)
			}
		}
		bindings = next
	}
	return bindings, nil
}

func (e *PropositionalEngine) expressionVariableSorts(root ast.ASTHandle) (map[uint32]ast.ASTHandle, error) {
	variables := make(map[uint32]ast.ASTHandle)
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		kind, err := e.manager.GetKind(current)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTVar {
			index, x := e.manager.GetVarIndex(current)
			if x != nil {
				return nil, x
			}
			if _, found := variables[index]; !found {
				variables[index], x = e.manager.GetSort(current)
				if x != nil {
					return nil, x
				}
			}
			continue
		}
		if kind != ast.ASTApp {
			return nil, ErrPropositionalHorn
		}
		count, err := e.manager.GetNumArgs(current)
		if err != nil {
			return nil, err
		}
		for i := count; i > 0; i-- {
			argument, x := e.manager.GetArg(current, i-1)
			if x != nil {
				return nil, x
			}
			stack = append(stack, argument)
		}
	}
	return variables, nil
}

func (e *PropositionalEngine) expandExpressionVariables(root ast.ASTHandle, bindings []variableBinding) ([]variableBinding, error) {
	variables, err := e.expressionVariableSorts(root)
	if err != nil {
		return nil, err
	}
	indices := make([]uint32, 0, len(variables))
	for index := range variables {
		indices = append(indices, index)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	for _, index := range indices {
		size, err := e.manager.GetSortSize(variables[index])
		if err != nil || !size.IsFinite() {
			return nil, ErrPropositionalHorn
		}
		domain, err := size.Size()
		if err != nil {
			return nil, err
		}
		next := make([]variableBinding, 0)
		for _, binding := range bindings {
			if _, found := binding[index]; found {
				next = append(next, binding)
				continue
			}
			for value := uint64(0); value < domain; value++ {
				expanded := cloneBinding(binding)
				expanded[index] = rel.Element(value)
				next = append(next, expanded)
			}
		}
		bindings = next
	}
	return bindings, nil
}

func (e *PropositionalEngine) finiteValue(expression ast.ASTHandle, binding variableBinding) (rel.Element, error) {
	kind, err := e.manager.GetKind(expression)
	if err != nil {
		return 0, err
	}
	if kind == ast.ASTVar {
		index, x := e.manager.GetVarIndex(expression)
		if x != nil {
			return 0, x
		}
		value, found := binding[index]
		if !found {
			return 0, ErrPropositionalHorn
		}
		return value, nil
	}
	sort, err := e.manager.GetSort(expression)
	if err != nil {
		return 0, err
	}
	value, err := e.relations.EncodeValue(sort, expression)
	if err != nil {
		return 0, ErrPropositionalHorn
	}
	return value, nil
}

func (e *PropositionalEngine) rewriteCondition(expression ast.ASTHandle, binding variableBinding) (value bool, result error) {
	if e.guard == nil {
		return false, ErrPropositionalHorn
	}
	variables, err := e.expressionVariableSorts(expression)
	if err != nil {
		return false, err
	}
	var maximum uint32
	for index := range variables {
		if index > maximum {
			maximum = index
		}
	}
	bindings := make([]ast.ASTHandle, 0)
	if len(variables) != 0 {
		bindings = make([]ast.ASTHandle, int(maximum)+1)
	}
	references := make([]ast.ASTRef, 0, len(variables)+1)
	defer func() {
		for i := len(references) - 1; i >= 0; i-- {
			result = errors.Join(result, e.manager.DelRef(&references[i]))
		}
	}()
	for index, sort := range variables {
		element, found := binding[index]
		if !found {
			return false, ErrPropositionalHorn
		}
		reference, x := e.relations.DecodeValue(sort, element)
		if x != nil {
			return false, x
		}
		references = append(references, reference)
		handle, x := e.manager.RefGet(reference)
		if x != nil {
			return false, x
		}
		bindings[len(bindings)-int(index)-1] = handle
	}
	out, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	references = append(references, out)
	if err = e.guard.RewriteWithBindings(expression, bindings, out); err != nil {
		return false, err
	}
	rewritten, err := e.manager.RefGet(out)
	if err != nil {
		return false, err
	}
	if e.manager.IsTrue(rewritten) {
		return true, nil
	}
	if e.manager.IsFalse(rewritten) {
		return false, nil
	}
	return false, ErrPropositionalHorn
}

func (e *PropositionalEngine) evaluateCondition(expression ast.ASTHandle, binding variableBinding) (bool, error) {
	value, err := e.evaluateConditionDirect(expression, binding)
	if errors.Is(err, ErrPropositionalHorn) && e.guard != nil {
		return e.rewriteCondition(expression, binding)
	}
	return value, err
}

func (e *PropositionalEngine) evaluateConditionDirect(expression ast.ASTHandle, binding variableBinding) (bool, error) {
	if e.manager.IsTrue(expression) {
		return true, nil
	}
	if e.manager.IsFalse(expression) {
		return false, nil
	}
	kind, err := e.manager.GetKind(expression)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		value, x := e.finiteValue(expression, binding)
		return value != 0, x
	}
	if kind != ast.ASTApp {
		return false, ErrPropositionalHorn
	}
	operator, err := e.manager.GetDeclKind(expression)
	if err != nil {
		return false, err
	}
	count, err := e.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	argument := func(index uint32) (ast.ASTHandle, error) { return e.manager.GetArg(expression, index) }
	switch operator {
	case ast.ASTOpNot:
		if count != 1 {
			return false, ErrPropositionalHorn
		}
		child, x := argument(0)
		if x != nil {
			return false, x
		}
		value, x := e.evaluateCondition(child, binding)
		return !value, x
	case ast.ASTOpAnd, ast.ASTOpOr:
		result := operator == ast.ASTOpAnd
		for i := uint32(0); i < count; i++ {
			child, x := argument(i)
			if x != nil {
				return false, x
			}
			value, x := e.evaluateCondition(child, binding)
			if x != nil {
				return false, x
			}
			if operator == ast.ASTOpAnd {
				result = result && value
			} else {
				result = result || value
			}
		}
		return result, nil
	case ast.ASTOpEq, ast.ASTOpOEq:
		if count != 2 {
			return false, ErrPropositionalHorn
		}
		left, x := argument(0)
		if x != nil {
			return false, x
		}
		right, x := argument(1)
		if x != nil {
			return false, x
		}
		leftSort, x := e.manager.GetSort(left)
		if x != nil {
			return false, x
		}
		boolean, x := e.manager.MkBoolSort()
		if x != nil {
			return false, x
		}
		if leftSort == boolean {
			leftValue, y := e.evaluateCondition(left, binding)
			if y != nil {
				return false, y
			}
			rightValue, y := e.evaluateCondition(right, binding)
			return leftValue == rightValue, y
		}
		leftValue, x := e.finiteValue(left, binding)
		if x != nil {
			return false, x
		}
		rightValue, x := e.finiteValue(right, binding)
		return leftValue == rightValue, x
	case ast.ASTOpDistinct:
		seen := make(map[rel.Element]bool)
		for i := uint32(0); i < count; i++ {
			child, x := argument(i)
			if x != nil {
				return false, x
			}
			value, x := e.finiteValue(child, binding)
			if x != nil {
				return false, x
			}
			if seen[value] {
				return false, nil
			}
			seen[value] = true
		}
		return true, nil
	case ast.ASTOpIff, ast.ASTOpXor, ast.ASTOpImplies:
		if count != 2 {
			return false, ErrPropositionalHorn
		}
		left, x := argument(0)
		if x != nil {
			return false, x
		}
		right, x := argument(1)
		if x != nil {
			return false, x
		}
		leftValue, x := e.evaluateCondition(left, binding)
		if x != nil {
			return false, x
		}
		rightValue, x := e.evaluateCondition(right, binding)
		if x != nil {
			return false, x
		}
		if operator == ast.ASTOpIff {
			return leftValue == rightValue, nil
		}
		if operator == ast.ASTOpXor {
			return leftValue != rightValue, nil
		}
		return !leftValue || rightValue, nil
	case ast.ASTOpIte:
		if count != 3 {
			return false, ErrPropositionalHorn
		}
		condition, x := argument(0)
		if x != nil {
			return false, x
		}
		branch, x := e.evaluateCondition(condition, binding)
		if x != nil {
			return false, x
		}
		index := uint32(2)
		if branch {
			index = 1
		}
		selected, x := argument(index)
		if x != nil {
			return false, x
		}
		return e.evaluateCondition(selected, binding)
	}
	sort, err := e.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	boolean, err := e.manager.MkBoolSort()
	if err == nil && sort == boolean {
		value, x := e.finiteValue(expression, binding)
		return value != 0, x
	}
	return false, ErrPropositionalHorn
}

func (e *PropositionalEngine) evaluateRule(rule *base.Rule) (changed bool, result error) {
	defer func() { result = errors.Join(result, rule.Close()) }()
	bindings := []variableBinding{{}}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return false, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return false, err
	}
	for i := 0; i < positive; i++ {
		application, negated, x := rule.Tail(i)
		if x != nil {
			return false, x
		}
		if negated {
			return false, ErrPropositionalHorn
		}
		declaration, x := e.manager.GetDecl(application)
		if x != nil {
			return false, x
		}
		state := e.predicates[declaration]
		if state == nil {
			return false, ErrPropositionalHorn
		}
		facts, x := state.table.Facts()
		if x != nil {
			return false, x
		}
		next := make([]variableBinding, 0)
		for _, binding := range bindings {
			for _, fact := range facts {
				unified, matches, y := e.unify(application, state, fact, binding)
				if y != nil {
					return false, y
				}
				if matches {
					next = append(next, unified)
				}
			}
		}
		bindings = next
	}
	for i := positive; i < uninterpreted; i++ {
		application, negated, x := rule.Tail(i)
		if x != nil {
			return false, x
		}
		if !negated {
			return false, ErrPropositionalHorn
		}
		declaration, x := e.manager.GetDecl(application)
		if x != nil {
			return false, x
		}
		state := e.predicates[declaration]
		if state == nil {
			return false, ErrPropositionalHorn
		}
		bindings, x = e.expandUnbound(application, state, bindings)
		if x != nil {
			return false, x
		}
		facts, x := state.table.Facts()
		if x != nil {
			return false, x
		}
		next := make([]variableBinding, 0, len(bindings))
		for _, binding := range bindings {
			excluded := false
			for _, fact := range facts {
				matches, y := e.negativeMatch(application, state, fact, binding)
				if y != nil {
					return false, y
				}
				if matches {
					excluded = true
					break
				}
			}
			if !excluded {
				next = append(next, binding)
			}
		}
		bindings = next
	}
	tails, err := rule.TailSize()
	if err != nil {
		return false, err
	}
	for i := uninterpreted; i < tails; i++ {
		condition, negated, x := rule.Tail(i)
		if x != nil {
			return false, x
		}
		if negated {
			return false, ErrPropositionalHorn
		}
		bindings, x = e.expandExpressionVariables(condition, bindings)
		if x != nil {
			return false, x
		}
		next := make([]variableBinding, 0, len(bindings))
		for _, binding := range bindings {
			accepted, y := e.evaluateCondition(condition, binding)
			if y != nil {
				return false, y
			}
			if accepted {
				next = append(next, binding)
			}
		}
		bindings = next
	}
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	declaration, err := e.manager.GetDecl(head)
	if err != nil {
		return false, err
	}
	state := e.predicates[declaration]
	if state == nil {
		return false, ErrPropositionalHorn
	}
	bindings, err = e.expandUnbound(head, state, bindings)
	if err != nil {
		return false, err
	}
	for _, binding := range bindings {
		fact, x := e.instantiate(head, state, binding)
		if x != nil {
			return false, x
		}
		added, x := state.table.Add(fact)
		if x != nil {
			return false, x
		}
		changed = changed || added
	}
	return changed, nil
}

// resetNegationDependentTables mirrors rel_context::reset_negated_tables:
// only populated predicates that directly or transitively depend on negation
// are cleared between saturations. Positive-only relation contents survive.
func (e *PropositionalEngine) resetNegationDependentTables() error {
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	depends := make(map[ast.ASTHandle]bool)
	for changed := true; changed; {
		changed = false
		for i := 0; i < count; i++ {
			rule, e2 := e.set.Rule(i)
			if e2 != nil {
				return e2
			}
			head, e2 := rule.HeadDeclaration()
			positive, pErr := rule.PositiveTailSize()
			uninterpreted, uErr := rule.UninterpretedTailSize()
			e2 = errors.Join(e2, pErr, uErr)
			mark := e2 == nil && positive < uninterpreted && !depends[head]
			for j := 0; e2 == nil && !mark && j < uninterpreted; j++ {
				tail, x := rule.TailDeclaration(j)
				e2 = x
				if depends[tail] && !depends[head] {
					mark = true
				}
			}
			e2 = errors.Join(e2, rule.Close())
			if e2 != nil {
				return e2
			}
			if mark {
				depends[head] = true
				changed = true
			}
		}
	}
	for declaration := range depends {
		state := e.predicates[declaration]
		if state == nil {
			continue
		}
		rows, x := state.table.Len()
		if x != nil {
			return x
		}
		if rows != 0 {
			if x = state.table.Reset(); x != nil {
				return x
			}
		}
	}
	return nil
}

func (e *PropositionalEngine) saturate() error {
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	rulesByStratum := make(map[uint32][]int)
	var maximum uint32
	for i := 0; i < count; i++ {
		rule, x := e.set.Rule(i)
		if x != nil {
			return x
		}
		head, x := rule.HeadDeclaration()
		if x == nil {
			var stratum uint32
			stratum, x = e.set.PredicateStratum(head)
			if x == nil {
				rulesByStratum[stratum] = append(rulesByStratum[stratum], i)
				if stratum > maximum {
					maximum = stratum
				}
			}
		}
		x = errors.Join(x, rule.Close())
		if x != nil {
			return x
		}
	}
	for stratum := uint32(0); stratum <= maximum; stratum++ {
		for {
			changed := false
			for _, index := range rulesByStratum[stratum] {
				rule, x := e.set.Rule(index)
				if x != nil {
					return x
				}
				added, x := e.evaluateRule(rule)
				if x != nil {
					return x
				}
				changed = changed || added
			}
			if !changed {
				break
			}
		}
	}
	return nil
}

func (e *PropositionalEngine) tryCompiledSaturation() (used bool, resultErr error) {
	execution, termination, err := rel.CompileFiniteRulesWithGuard(e.relations, e.set, e.guard)
	if errors.Is(err, rel.ErrCompilerUnsupported) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	context, err := rel.NewExecutionContextWithRelations(e.relations, nil)
	if err != nil {
		return true, errors.Join(err, execution.Close(), termination.Close())
	}
	defer func() {
		resultErr = errors.Join(resultErr, execution.Close(), termination.Close(), context.Close())
	}()
	complete, err := execution.Perform(context)
	if err != nil {
		return true, err
	}
	terminated, terminationErr := termination.Perform(context)
	if terminationErr != nil {
		return true, terminationErr
	}
	if !complete || !terminated {
		return true, ErrPropositionalEngine
	}
	if err = execution.ProcessAllCosts(); err != nil {
		return true, err
	}
	e.datalogStats.Reset()
	if err = execution.CollectStatistics(&e.datalogStats); err != nil {
		return true, err
	}
	if err = context.CollectStatistics(&e.datalogStats); err != nil {
		return true, err
	}
	for declaration, state := range e.predicates {
		table, found, x := e.relations.TryGetRelation(declaration)
		if x != nil {
			return true, x
		}
		if found {
			state.table = table
		}
	}
	return true, nil
}

// LastQueryUsedCompiledCode reports whether the most recent query ran the
// relational instruction compiler instead of the direct finite evaluator.
func (e *PropositionalEngine) LastQueryUsedCompiledCode() (bool, error) {
	if err := e.open(); err != nil {
		return false, err
	}
	return e.compiled, nil
}

func (e *PropositionalEngine) CollectStatistics(destination *statistics.Statistics) error {
	if err := e.open(); err != nil {
		return err
	}
	if destination == nil {
		return ErrPropositionalEngine
	}
	if e.backend == "pdr" || e.backend == "qpdr" {
		destination.Copy(e.pdrStatistics)
		if e.pdr != nil {
			return e.pdr.CollectStatistics(destination)
		}
		return nil
	}
	if e.backend == "spacer" && e.spacer != nil {
		return e.spacer.CollectStatistics(destination)
	}
	if e.backend == "bmc" {
		destination.Copy(e.bmcStatistics)
		return nil
	}
	if e.backend == "tab" {
		destination.UpdateUint("tab.num_unfold", e.tabUnfold)
		destination.UpdateUint("tab.num_unfold_fail", e.tabUnfoldFail)
		destination.UpdateUint("tab.num_subsumed", e.tabSubsumed)
		return nil
	}
	if e.backend == "datalog" {
		destination.UpdateDouble("saturation time", e.datalogElapsed)
		destination.Copy(e.datalogStats)
	}
	return nil
}

func (e *PropositionalEngine) ResetStatistics() error {
	if err := e.open(); err != nil {
		return err
	}
	if e.backend == "pdr" || e.backend == "qpdr" {
		e.pdrStatistics.Reset()
		if e.pdr != nil {
			return e.pdr.ResetStatistics()
		}
		return nil
	}
	if e.backend == "spacer" && e.spacer != nil {
		return e.spacer.ResetStatistics()
	}
	if e.backend == "bmc" {
		e.bmcStatistics.Reset()
		return nil
	}
	if e.backend == "tab" {
		e.tabUnfold, e.tabUnfoldFail, e.tabSubsumed = 0, 0, 0
		return nil
	}
	return nil
}

// Rules returns owning formula references for the current scoped Horn rule
// set in insertion order. The caller releases every reference.
func (e *PropositionalEngine) Rules() (result []ast.ASTRef, resultErr error) {
	result, _, resultErr = e.NamedRules()
	return result, resultErr
}

// NamedRules returns owning formulas and their copied names in insertion
// order.
func (e *PropositionalEngine) NamedRules() (result []ast.ASTRef, names []string, resultErr error) {
	if err := e.open(); err != nil {
		return nil, nil, err
	}
	count, err := e.set.Len()
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if resultErr != nil {
			for index := len(result) - 1; index >= 0; index-- {
				resultErr = errors.Join(resultErr, e.manager.DelRef(&result[index]))
			}
			result = nil
			names = nil
		}
	}()
	result = make([]ast.ASTRef, 0, count)
	names = make([]string, 0, count)
	for index := 0; index < count; index++ {
		rule, getErr := e.set.Rule(index)
		if getErr != nil {
			return nil, nil, getErr
		}
		formula, formulaErr := rule.ToFormula()
		name, nameErr := rule.Name()
		formulaErr = errors.Join(formulaErr, nameErr)
		formulaErr = errors.Join(formulaErr, rule.Close())
		if formulaErr != nil {
			if _, referenceErr := e.manager.RefGet(formula); referenceErr == nil {
				formulaErr = errors.Join(formulaErr, e.manager.DelRef(&formula))
			}
			return nil, nil, formulaErr
		}
		result = append(result, formula)
		names = append(names, name)
	}
	return result, names, nil
}

// Predicates returns borrowed registered relation declarations in declaration
// order.
func (e *PropositionalEngine) Predicates() ([]ast.ASTHandle, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), e.predicateOrder...), nil
}

// QueryPredicate returns true exactly when the registered predicate relation
// contains at least one tuple after positive Horn saturation.
func (e *PropositionalEngine) QueryPredicate(declaration ast.ASTHandle) (result lbool.LBool, resultErr error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	if err := e.clearQueryResources(); err != nil {
		return lbool.Undef, err
	}
	defer func() { e.recordQueryStatus(result, resultErr) }()
	state := e.predicates[declaration]
	if state == nil {
		return lbool.Undef, ErrPropositionalHorn
	}
	e.queried = true
	if e.backend == "pdr" || e.backend == "qpdr" {
		e.compiled = false
		return e.queryPDR(declaration)
	}
	if e.backend == "spacer" {
		e.compiled = false
		return e.querySpacer(declaration)
	}
	if e.backend == "ddnf" {
		e.compiled = false
		return e.queryDDNFPredicate(declaration)
	}
	if e.backend == "clp" {
		e.compiled = false
		return e.queryCLPPredicate(declaration)
	}
	if e.backend == "bmc" {
		e.compiled = false
		if !e.auxiliary[declaration] {
			return e.queryBMCRelation(declaration)
		}
		return e.queryBMCPredicate(declaration)
	}
	if e.backend == "tab" {
		e.compiled = false
		return e.queryTabPredicate(declaration)
	}
	return e.queryDatalogPredicates([]ast.ASTHandle{declaration})
}

// QueryPredicates returns true when at least one requested relation is
// nonempty. Native PDR and Spacer only support a single relation in this form.
func (e *PropositionalEngine) QueryPredicates(declarations []ast.ASTHandle) (result lbool.LBool, resultErr error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	if err := e.clearQueryResources(); err != nil {
		return lbool.Undef, err
	}
	defer func() { e.recordQueryStatus(result, resultErr) }()
	defer func() {
		if resultErr == nil && e.backend == "datalog" {
			e.queryRelations = append([]ast.ASTHandle(nil), declarations...)
			e.queryRelationsSet = true
		}
	}()
	for _, declaration := range declarations {
		if e.predicates[declaration] == nil {
			return lbool.Undef, ErrPropositionalHorn
		}
	}
	if e.backend != "datalog" {
		if len(declarations) != 1 {
			return lbool.Undef, nil
		}
		return e.QueryPredicate(declarations[0])
	}
	e.queried = true
	return e.queryDatalogPredicates(declarations)
}

func (e *PropositionalEngine) queryDatalogPredicates(declarations []ast.ASTHandle) (lbool.LBool, error) {
	count, err := e.set.Len()
	if err != nil {
		return lbool.Undef, err
	}
	if count == 0 && len(declarations) > 0 {
		hasInfinite, nonempty := false, false
		for _, declaration := range declarations {
			table := e.predicates[declaration].table
			if table == nil {
				hasInfinite = true
				continue
			}
			size, cause := table.Len()
			if cause != nil {
				return lbool.Undef, cause
			}
			nonempty = nonempty || size != 0
		}
		if hasInfinite {
			e.compiled = false
			e.datalogStats.Reset()
			return lbool.FromBool(nonempty), nil
		}
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, x := e.set.Finalize()
		if x != nil {
			return lbool.Undef, x
		}
		if !valid {
			return lbool.Undef, fmt.Errorf("muz/fp: negation is not stratified")
		}
	}
	if err = e.normalizeSimpleJoins(); err != nil {
		return lbool.Undef, err
	}
	// Clear old query marks before transformations can publish saturated
	// auxiliary relations for this query.
	if err = e.relations.ResetSaturatedMarks(); err != nil {
		return lbool.Undef, err
	}
	if err = e.applyDatalogSimilarity(declarations); err != nil {
		return lbool.Undef, err
	}

	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	for _, declaration := range declarations {
		if e.predicates[declaration].table == nil {
			return lbool.Undef, ErrPropositionalHorn
		}
	}
	e.datalogStats.Reset()
	// rel_context::query resets negation-dependent tables before choosing and
	// running the relational saturation implementation. Keep this outside the
	// direct evaluator so compiled and interpreted execution see the same state.
	if err = e.resetNegationDependentTables(); err != nil {
		return lbool.Undef, err
	}
	started := time.Now()
	e.compiled, err = e.tryCompiledSaturation()
	if err != nil {
		e.datalogElapsed += time.Since(started).Seconds()
		return lbool.Undef, err
	}
	if !e.compiled {
		err = e.saturate()
	}
	e.datalogElapsed += time.Since(started).Seconds()
	if err != nil {
		return lbool.Undef, err
	}
	for _, declaration := range declarations {
		count, x := e.predicates[declaration].table.Len()
		if x != nil {
			return lbool.Undef, x
		}
		if count != 0 {
			return lbool.True, nil
		}
	}
	return lbool.FromBool(len(declarations) == 0), nil
}

// QueryPredicateFromLevel runs a Spacer query beginning at the requested proof
// level.
func (e *PropositionalEngine) QueryPredicateFromLevel(declaration ast.ASTHandle, level uint32) (result lbool.LBool, resultErr error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	if err := e.clearQueryResources(); err != nil {
		return lbool.Undef, err
	}
	defer func() { e.recordQueryStatus(result, resultErr) }()
	if e.backend != "spacer" || e.predicates[declaration] == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	e.queried = true
	e.compiled = false
	return e.querySpacerFromLevel(declaration, level)
}

func (e *PropositionalEngine) recordQueryStatus(result lbool.LBool, err error) {
	e.recordResultSnapshot(result, err)
	if err != nil {
		e.lastStatus = "input error"
		return
	}
	if result == lbool.Undef {
		e.lastStatus = "approximated"
		return
	}
	e.lastStatus = "ok"
}

// ReasonUnknown returns the native fixedpoint execution-status spelling for
// the latest query. Decided queries report "ok"; an undecided retained query
// reports "approximated".
func (e *PropositionalEngine) ReasonUnknown() (string, error) {
	if err := e.open(); err != nil {
		return "", err
	}
	return e.lastStatus, nil
}

func (e *PropositionalEngine) inductiveModelAnswer(inductive *model.Model) (answer ast.ASTRef, result error) {
	if inductive == nil {
		return answer, ast.ErrASTHandle
	}
	defer func() { result = errors.Join(result, inductive.Close()) }()
	builder, err := model.NewModel2ExprBuilder(inductive.Core())
	if err != nil {
		return answer, err
	}
	defer func() { result = errors.Join(result, builder.Close()) }()
	answer, err = e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return answer, err
	}
	defer func() {
		if result != nil && answer != (ast.ASTRef{}) {
			result = errors.Join(result, e.manager.DelRef(&answer))
			answer = ast.ASTRef{}
		}
	}()
	if err = builder.Build(answer); err != nil {
		return answer, err
	}
	return answer, nil
}

// Answer returns an owning formula describing the current finite relation.
// Each free variable corresponds to the predicate column with the same index.
func (e *PropositionalEngine) Answer(declaration ast.ASTHandle) (answer ast.ASTRef, result error) {
	if err := e.open(); err != nil {
		return answer, err
	}
	kind, err := parseEngineKind(e.backend)
	if err != nil {
		return answer, err
	}
	if err = e.requireResultSnapshot(kind); err != nil {
		return answer, err
	}
	if kind == EngineCLP {
		value, err := e.manager.MkTrue()
		if err != nil {
			return answer, err
		}
		return e.manager.NewRef(value)
	}
	if kind == EngineBMC {
		if e.bmcProof == (ast.ASTRef{}) {
			return answer, ErrPropositionalEngine
		}
		proof, err := e.manager.RefGet(e.bmcProof)
		if err != nil {
			return answer, err
		}
		return e.manager.NewRef(proof)
	}
	if kind == EngineTab {
		if e.tabAnswer == (ast.ASTRef{}) {
			return answer, ErrPropositionalEngine
		}
		return e.manager.CopyRef(e.tabAnswer)
	}
	if e.backend == "pdr" || e.backend == "qpdr" {
		status, err := e.pdr.LastResult()
		if err != nil {
			return answer, err
		}
		if status == lbool.False {
			inductive, modelErr := e.Model(declaration)
			if modelErr != nil {
				return answer, modelErr
			}
			return e.inductiveModelAnswer(inductive)
		}
		// Relation-query answers intentionally retain sliced predicate names,
		// matching pdr_dl_interface's converter-free reachable-trace surface.
		return e.pdr.Answer(e.pdrPredicate(declaration))
	}
	if e.backend == "spacer" {
		status, err := e.spacer.LastResult()
		if err != nil {
			return answer, err
		}
		if status == lbool.False {
			inductive, modelErr := e.Model(declaration)
			if modelErr != nil {
				return answer, modelErr
			}
			return e.inductiveModelAnswer(inductive)
		}
		answer, result = e.spacer.Answer(e.spacerPredicate(declaration))
		if result != nil {
			return answer, result
		}
		return e.expandSpacerFormula(declaration, answer)
	}
	// DDNF's source query path dumps the compiled rules and returns unknown;
	// its get_answer implementation is UNREACHABLE. Do not fall through to
	// the Datalog relation table, which was never populated by DDNF execution.
	if e.backend == "ddnf" {
		return answer, ErrPropositionalEngine
	}
	state := e.predicates[declaration]
	if state == nil || !e.queried {
		return answer, ErrPropositionalEngine
	}
	if state.table == nil {
		count, cause := e.set.Len()
		if cause != nil {
			return answer, cause
		}
		if count == 0 {
			value, cause := e.manager.MkFalse()
			if cause != nil {
				return answer, cause
			}
			return e.manager.NewRef(value)
		}
		return answer, ErrPropositionalEngine
	}
	facts, err := state.table.Facts()
	if err != nil {
		return answer, err
	}
	temporaries := make([]ast.ASTRef, 0)
	defer func() {
		for i := len(temporaries) - 1; i >= 0; i-- {
			result = errors.Join(result, e.manager.DelRef(&temporaries[i]))
		}
		if result != nil && answer != (ast.ASTRef{}) {
			result = errors.Join(result, e.manager.DelRef(&answer))
			answer = ast.ASTRef{}
		}
	}()
	retain := func(handle ast.ASTHandle) (ast.ASTHandle, error) {
		reference, x := e.manager.NewRef(handle)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		temporaries = append(temporaries, reference)
		return handle, nil
	}
	rows := make([]ast.ASTHandle, 0, len(facts))
	for _, fact := range facts {
		values, x := e.relations.DecodeFact(state.sorts, fact)
		if x != nil {
			return answer, x
		}
		temporaries = append(temporaries, values...)
		equalities := make([]ast.ASTHandle, len(fact))
		for i := range fact {
			variable, y := e.manager.MkVar(uint32(i), state.sorts[i])
			if y != nil {
				return answer, y
			}
			if _, y = retain(variable); y != nil {
				return answer, y
			}
			value, y := e.manager.RefGet(values[i])
			if y != nil {
				return answer, y
			}
			equalities[i], y = e.manager.MkEq(variable, value)
			if y != nil {
				return answer, y
			}
			if _, y = retain(equalities[i]); y != nil {
				return answer, y
			}
		}
		row, x := e.manager.MkAndArray(equalities)
		if x != nil {
			return answer, x
		}
		if row, x = retain(row); x != nil {
			return answer, x
		}
		rows = append(rows, row)
	}
	formula, err := e.manager.MkOrArray(rows)
	if err != nil {
		return answer, err
	}
	return e.manager.NewRef(formula)
}

// Proof returns the owning reachable PDR proof trace selected by
// fixedpoint.generate_proof_trace. The caller releases it through
// ASTManager.DelRef.
func (e *PropositionalEngine) Proof(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.requirePDRSnapshot(); err != nil {
		return ast.ASTRef{}, err
	}
	return e.pdr.Proof(e.pdrPredicate(declaration))
}

// WriteCertificate emits the PDR certificate selected by the source
// fixedpoint query command. Unreachable queries use the converted inductive
// model; reachable queries use the retained trace expression. Spacer owns a
// separate certificate presentation path and reports no output here.
func (e *PropositionalEngine) WriteCertificate(out io.Writer, declaration ast.ASTHandle, writeExpression func(io.Writer, ast.ASTHandle) error) (found bool, resultErr error) {
	if err := e.open(); err != nil {
		return false, err
	}
	if e.backend == "spacer" {
		if err := e.requireResultSnapshot(EngineSpacer); err != nil {
			return false, err
		}
		return false, e.spacer.DisplayCertificate(out)
	}
	if e.backend == "ddnf" {
		return false, ErrPropositionalEngine
	}
	if e.backend == "bmc" {
		if err := e.requireResultSnapshot(EngineBMC); err != nil {
			return false, err
		}
		status, err := e.LastResult()
		if err != nil {
			return false, err
		}
		if status != lbool.True {
			_, err = fmt.Fprintln(out, "null")
			return true, err
		}
		answer, err := e.Answer(declaration)
		if err != nil {
			return false, err
		}
		defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&answer)) }()
		proof, err := e.manager.RefGet(answer)
		if err != nil {
			return false, err
		}
		if err = writeExpression(out, proof); err != nil {
			return true, err
		}
		_, err = fmt.Fprintln(out)
		return true, err
	}
	if e.backend != "pdr" && e.backend != "qpdr" {
		return false, nil
	}
	if err := e.requirePDRSnapshot(); err != nil {
		return false, err
	}
	status, err := e.pdr.LastResult()
	if err != nil {
		return false, err
	}
	if status == lbool.False {
		inductive, x := e.Model(declaration)
		if x != nil {
			return false, x
		}
		defer func() { resultErr = errors.Join(resultErr, inductive.Close()) }()
		return true, inductive.Core().WriteSMT2(out, e.params, e.globals, 0)
	}
	if status != lbool.True {
		return false, nil
	}
	answer, err := e.Answer(declaration)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&answer)) }()
	handle, err := e.manager.RefGet(answer)
	if err != nil {
		return false, err
	}
	return true, writeExpression(out, handle)
}

// Model returns the finite least-fixed-point or inductive PDR/Spacer model for
// the latest unreachable query. The caller owns and closes the returned model.
func (e *PropositionalEngine) Model(declaration ast.ASTHandle) (*model.Model, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	kind, err := parseEngineKind(e.backend)
	if err != nil {
		return nil, err
	}
	if err = e.requireResultSnapshot(kind); err != nil {
		return nil, err
	}
	if e.backend == "datalog" {
		return e.datalogModel(declaration)
	}
	if (e.backend == "pdr" || e.backend == "qpdr") && e.pdr != nil {
		result, err := e.pdr.Model(e.pdrPredicate(declaration))
		if err != nil {
			return nil, err
		}
		return e.expandHornModel(result, e.pdrConversions)
	}
	if e.backend == "spacer" && e.spacer != nil && e.spacerRevision == e.revision {
		result, err := e.spacer.Model(e.spacerPredicate(declaration))
		if err != nil {
			return nil, err
		}
		return e.expandSpacerModel(result)
	}
	return nil, ErrPropositionalEngine
}

// Query accepts the zero-arity predicate application used by the original
// propositional engine API. General declaration queries use QueryPredicate.
func (e *PropositionalEngine) Query(query ast.ASTHandle) (lbool.LBool, error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	kind, err := e.manager.GetKind(query)
	if err != nil {
		return lbool.Undef, err
	}
	if kind != ast.ASTApp {
		return e.QueryExpression(query)
	}
	arguments, err := e.manager.GetNumArgs(query)
	if err != nil {
		return lbool.Undef, err
	}
	if arguments != 0 {
		return e.QueryExpression(query)
	}
	declaration, err := e.manager.GetDecl(query)
	if err != nil {
		return lbool.Undef, err
	}
	if e.predicates[declaration] == nil {
		return e.QueryExpression(query)
	}
	return e.QueryPredicate(declaration)
}

func (e *PropositionalEngine) Close() (result error) {
	if e == nil {
		return ErrPropositionalEngine
	}
	if e.closed {
		if e.ddnf.registry != nil && !e.ddnf.registryBorrowed {
			e.ddnf.registry.Close()
			e.ddnf.registry = nil
		}
		result = e.ddnf.closeTrail(e.manager)
		if result == nil {
			e.ddnf = ddnfEngineState{}
		}
		return result
	}
	result = errors.Join(result, e.clearQueryResources())
	if e.queryParamsReady {
		result = errors.Join(result, e.params.Del(&e.queryParams))
		e.queryParamsReady = false
	}
	result = errors.Join(result, e.clearBMCTrace())
	result = errors.Join(result, e.clearTabAnswer())
	if e.pdr != nil {
		result = errors.Join(result, e.pdr.Close())
		e.pdr = nil
		e.pdrRevision = 0
		e.pdrQuery = ast.ASTHandle{}
		result = errors.Join(result, e.clearPDRPreprocessing())
	}
	if e.pdrRules != nil {
		result = errors.Join(result, e.pdrRules.Close())
		e.pdrRules = nil
	}
	e.pdrRuleRevision = 0
	if e.spacer != nil {
		result = errors.Join(result, e.spacer.Close())
		e.spacer = nil
		e.spacerRevision = 0
		e.spacerPredicates = nil
		e.spacerSlices = nil
		e.spacerQuantified = nil
		e.spacerInstantiated = nil
		e.spacerBitBlasted = nil
		e.spacerScaled = nil
		e.spacerMagic = nil
		e.spacerKarr = nil
	}
	if e.spacerRules != nil {
		result = errors.Join(result, e.spacerRules.Close())
		e.spacerRules = nil
	}
	result = errors.Join(result, closeHornModelConversions(e.spacerConversions))
	e.spacerConversions = nil
	for i := len(e.scopes) - 1; i >= 0; i-- {
		result = errors.Join(result, e.scopes[i].Close())
	}
	e.scopes = nil
	e.autoRules = nil
	e.autoRuleScopes = nil
	e.assertionScopes = nil
	for i := len(e.assertions) - 1; i >= 0; i-- {
		result = errors.Join(result, e.manager.DelRef(&e.assertions[i]))
	}
	e.assertions = nil
	if e.guard != nil {
		result = errors.Join(result, e.guard.Close())
		e.guard = nil
	}
	if e.unboundLite != nil {
		result = errors.Join(result, e.unboundLite.Close())
		e.unboundLite = nil
	}
	if e.unboundCooperation != nil {
		result = errors.Join(result, e.unboundCooperation.Close())
		e.unboundCooperation = nil
	}
	if e.ddnf.registry != nil && !e.ddnf.registryBorrowed {
		e.ddnf.registry.Close()
		e.ddnf.registry = nil
	}
	trailErr := e.ddnf.closeTrail(e.manager)
	result = errors.Join(result, trailErr)
	if trailErr == nil {
		e.ddnf = ddnfEngineState{}
	}
	if e.set != nil {
		result = errors.Join(result, e.set.Close())
		e.set = nil
	}
	if e.rules != nil {
		result = errors.Join(result, e.rules.Close())
		e.rules = nil
	}
	if e.binder != nil {
		result = errors.Join(result, e.binder.Close())
		e.binder = nil
	}
	if e.relations != nil {
		result = errors.Join(result, e.relations.Close())
		e.relations = nil
	}
	result = errors.Join(result, e.manager.DelRef(&e.background))
	e.predicates = nil
	e.predicateOrder = nil
	e.auxiliary = nil
	e.params, e.globals = nil, nil
	e.closed = true
	return result
}
