package smtcore

// Source: src/smt/smt_internalizer.cpp Boolean-variable, enode, and literal
// registration methods.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uheap "github.com/glycerine/g3/pkg/util/heap"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

type smtInternalizerScope struct {
	enodeTrailMark       int // context trail: enode label hashes
	goalQueue            smtRelevanceGoalScope
	relActLimit          int
	relActHead           int
	relevanceFIFO        smtRelevanceFIFOScope
	trueFirstLimit       int
	theoryCaseSplitLimit int
	booleanLimit         int
	enodeLimit           int
	pendingLimit         int
	pendingHead          int
	equalityLimit        int
	assignmentLimit      int
	clauseLimit          int
	justificationLimit   int
	theoryLimit          int
	assignedLimit        int
	propagationHead      int
	relevancyBatch       smtRelevancyAssignmentBatch
	theoryEqLimit        int
	theoryEqHead         int
	theoryDiseqLimit     int
	theoryDiseqHead      int
	notifyLimit          int
	enodeFlagLimit       int
	mergeTFLimit         int
	atomNotifiedLimit    int
	simplifyHead         int
	inconsistent         bool
	conflict             SMTBooleanJustification
	conflictLiteral      SMTLiteral
	disequalityLimit     int
}

type smtInternalizerAssignmentTrail struct {
	variable      int32
	value         lbool.LBool
	justification SMTBooleanJustification
	scopeLevel    uint32
}

type smtInternalizerTheoryTrail struct {
	node      *SMTEnode
	root      *SMTEnode
	theory    SMTTheoryID
	old       SMTTheoryVar
	rootAdded bool
}

type smtInternalizerNotifyTrail struct {
	variable int32
	theory   SMTTheoryID
}

// smtDisequalityUndo restores one root's disequality list length.
type smtDisequalityUndo struct {
	root   *SMTEnode
	length int
}

// smtEqualityUndo is one source add_eq_trail entry.
type smtEqualityUndo struct {
	r1, left, r2       *SMTEnode
	r2NumParents       int
	r2NumSourceParents int
	r2NumDisequalities int
	theoryLimit        int
	r2TheoryVariables  SMTTheoryVarList
	merged             bool
	stage              uint8
}

type SMTTheoryPropagation struct {
	Theory      SMTTheoryID
	Left, Right SMTTheoryVar
}

type SMTInternalizedClause struct {
	Literals []SMTLiteral
	Root     bool
	Binary   bool
	// Lemma marks a binary lemma: its watches persist across pops, as the
	// source keeps binary lemma watches (atoms internalized at level 0).
	Lemma        bool
	Proof        ast.ASTHandle
	clause       *SMTClause
	owner        SMTJustification
	closeWatches uint8
}

type SMTInternalizerCongruence struct {
	Node     *SMTEnode
	Existing *SMTEnode
	Commuted bool
}

// Source context::new_eq: explicit theory equalities and congruences share
// one FIFO and retain their concrete tagged equality justification.
type smtPendingEquality struct {
	left, right   *SMTEnode
	justification SMTEqualityJustification
}

type SMTInternalizerState struct {
	// registeredJustification is a wrapper already in ownedJustifications
	// (mk_justification), which mk_clause must not register again.
	registeredJustification SMTJustification
	// reinitSeq orders pushes onto m_clauses_to_reinit.
	reinitSeq uint64
	// afterFlush runs in Close between context::flush and the destruction
	// of m_theories: ~context destroys m_model and m_proto_model there.
	afterFlush func() error
	// onQuantifierFlush runs when context::flush sets m_qmanager = nullptr:
	// ~qi_queue releases its cost function's variables there.
	onQuantifierFlush func() error
	// resourceLimitsExceeded is context::resource_limits_exceeded (it
	// records the search failure).
	resourceLimitsExceeded func() (bool, error)
	// onScopePush and onScopePop are the asserted_formulas scope work that
	// context::push_scope and pop_scope_core do for every scope.
	onScopePush func() error
	onScopePop  func(count uint32) error
	// paramsContext and globals are the kernel's parameter services, borrowed
	// by theories that build their own rewriters (theory_fpa's converter).
	paramsContext         *uparams.ParamsContext
	globals               *uparams.GlobalParamsService
	queueTrace            *diagnostic.TraceTags // Borrowed control; nil uses process trace tags.
	delayedActivity       *uheap.Int
	activityCacheRetiring bool
	cachedActivity        bool
	activityCacheDomain   []ast.ASTHandle
	activityCacheDrop     int
	basicActivity         *uheap.Int
	relevanceGoal         smtRelevanceGoalQueue
	relevanceActivity     smtRelevanceActivityQueue
	relevanceFIFO         smtRelevanceFIFO
	lemmaProblem          *smtLemmaProblem // Interrupted lexical roots, retired before the manager.
	theoryProgress        uint64
	manager               *ast.ASTManager
	theoryRewriter        *rewriter.TheoryRewriter // borrowed from the kernel
	flushQuantifiersInPop bool                     // Close: popTo runs flushQuantifiers after the trail
	// enodeTrail holds context value trails on enodes (set_lbl_hash).
	enodeTrail SMTEnodeTrail
	// assertedPopFlush is asserted_formulas::pop_scope's flush_cache, which
	// pop_scope_core runs on every context pop, search backjumps included.
	assertedPopFlush func() error
	// baseScopePop is pop_scope_core's base-scope step for a scope that was
	// not inconsistent at its push: the context drops m_unsat_proof.
	baseScopePop func() error
	// userPop is set while SMTKernel.Pop drives PopScopes; the kernel then
	// flushes after the asserted formulas shrink, where pop_scope does.
	userPop                    bool
	smtParams                  *SMTParams         // borrowed context::m_fparams
	contextParams              *uparams.ParamsRef // borrowed context::m_params
	context                    *SMTCheckerContext
	congruence                 *SMTCongruenceTable
	quantifiers                *SMTQuantifierRegistry
	quantifierManager          *SMTQuantifierManager
	assertedHasQuantifiers     *bool                 // borrowed context::has_quantifiers state
	assertedInconsistent       *bool                 // borrowed asserted-formula inconsistency state
	assertedFormulas           *[]SMTKernelAssertion // borrowed context asserted-formula owner
	relevancy                  *SMTRelevancyPropagator
	theories                   map[ast.FamilyID]SMTTheoryInternalizer
	theoryOrder                []SMTTheoryInternalizer
	boolVar                    smtBoolVarTable
	boolExpr                   []ast.ASTHandle
	booleanData                []SMTBoolVarData
	booleanCreation            []uint64
	nextBooleanCreation        uint64
	activity                   []float64
	activityIncrement          float64
	activityInverseDecay       float64
	random                     z3rand.Generator
	randomInitialActivity      uint32
	caseSplitStrategy          uint32
	theoryAwareBranching       *smtTheoryAwareBranchingQueue
	theoryCaseSplit            bool
	theoryCaseSplitSets        [][]SMTLiteral
	allTheoryCaseSplitLiterals map[SMTLiteral]struct{}
	literalTheoryCaseSplitSets map[SMTLiteral][][]SMTLiteral
	activityCache              map[ast.ASTHandle]float64
	assignment                 []lbool.LBool
	watches                    []SMTWatchList
	booleanStack               []ast.ASTHandle
	enodeStack                 []*SMTEnode
	// booleanSeq and enodeSeq number each mk_bool_var and mk_enode in one
	// sequence: the source records both on m_trail_stack, so a pop undoes
	// them interleaved, newest first.
	booleanSeq            []uint64
	enodeSeq              []uint64
	internalizeSeq        uint64
	scopes                []smtInternalizerScope
	pendingPop            *smtInternalizerPendingPop
	popActive             bool
	closing               bool
	retiringEnode         *SMTEnode
	retiringEnodeRoot     ast.ASTHandle
	retiringBoolean       bool
	retiringBooleanRoot   ast.ASTHandle
	cachedGeneration      map[ast.ASTHandle]uint32
	pending               []smtPendingEquality
	pendingHead           int
	equalityTrail         []smtEqualityUndo
	equalityAtomsByTerm   map[ast.ASTHandle][]int32
	equalityAtomCursor    int
	dynamicAck            *smtDynAckManager
	assignmentTrail       []smtInternalizerAssignmentTrail
	theoryTrail           []smtInternalizerTheoryTrail
	theoryEqualities      []SMTTheoryPropagation
	theoryDisequalities   []SMTTheoryPropagation
	theoryEqualityHead    int
	theoryDisequalityHead int
	// atomQueue is m_atom_propagation_queue: assign_core appends a literal
	// whose atom is relevant (or relevancy is off) when it is assigned, and
	// relevant_eh appends an assigned atom when it becomes relevant.
	atomQueue                []SMTLiteral
	notifyTrail              []smtInternalizerNotifyTrail
	enodeFlagTrail           []int32
	mergeTFTrail             []*SMTEnode // set_merge_tf_trail
	trueFirstTrail           []int32
	atomNotified             map[int32]bool
	atomNotifiedTrail        []int32
	relevancyLevel           uint32
	newCore2thEq             bool
	clauses                  []SMTInternalizedClause
	ownedJustifications      []SMTJustification
	assignedLiterals         []SMTLiteral
	propagationHead          int
	relevancyBatch           smtRelevancyAssignmentBatch
	conflict                 SMTBooleanJustification
	conflictLiteral          SMTLiteral
	inconsistent             bool
	generation               uint32
	initEnodes               int // enodes created by context::init (0 until then)
	scopeLevel               uint32
	statistics               SMTContextStatistics
	finalCheckIndex          int
	incompleteTheories       []SMTTheoryInternalizer
	theoriesSetup            bool
	binaryClauseOptimization bool
	clauseMaintenance        SMTClauseMaintenanceConfig
	conflictsSinceLemmaGC    uint32
	lemmaGCThreshold         uint32
	simplifyHead             int
	simplifyCounter          int
	learnedUnits             []smtPersistentLearnedUnit
	searchBaseLevel          uint32
	// baseLevel is context::m_base_lvl: the number of user scopes. Search
	// scopes above it stay until popToBaseLevel, as in pop_to_base_lvl.
	baseLevel             uint32
	searchUserBaseLevel   uint32
	searchAgility         float64
	searchAgilityFactor   float64
	searchRestartAdaptive bool
	searching             bool
	closed                bool
}

func NewSMTInternalizerState(manager *ast.ASTManager, context *SMTCheckerContext, quantifiers *SMTQuantifierRegistry) (*SMTInternalizerState, error) {
	state, err := newSMTInternalizerState(manager, context, quantifiers)
	if err != nil {
		return nil, err
	}
	if err = state.initContextEnodes(); err != nil {
		return nil, errors.Join(err, state.Close())
	}
	return state, nil
}

// newSMTInternalizerState leaves context::init's true and false enodes to the
// owner. The kernel creates them after its asserted-formula and quantifier
// members, as the source context constructor body does.
func newSMTInternalizerState(manager *ast.ASTManager, context *SMTCheckerContext, quantifiers *SMTQuantifierRegistry) (*SMTInternalizerState, error) {
	if manager == nil || context == nil || context.manager != manager || quantifiers != nil && quantifiers.manager != manager {
		return nil, ast.ErrASTHandle
	}
	table, err := NewSMTCongruenceTable(manager)
	if err != nil {
		return nil, err
	}
	state := &SMTInternalizerState{
		manager: manager, context: context, congruence: table, quantifiers: quantifiers,
		boolVar: smtBoolVarTable{context.exprs}, boolExpr: make([]ast.ASTHandle, 1), booleanData: make([]SMTBoolVarData, 1), booleanCreation: make([]uint64, 1),
		activity: make([]float64, 1), assignment: []lbool.LBool{lbool.True, lbool.False}, watches: make([]SMTWatchList, 2),
		cachedGeneration: make(map[ast.ASTHandle]uint32), theories: make(map[ast.FamilyID]SMTTheoryInternalizer),
		equalityAtomsByTerm:   make(map[ast.ASTHandle][]int32),
		conflictLiteral:       SMTNullLiteral,
		activityIncrement:     1,
		activityInverseDecay:  1.052,
		randomInitialActivity: 1,
		caseSplitStrategy:     1,
		relevanceFIFO:         smtRelevanceFIFO{baseBooleanLimit: -1},
		activityCache:         make(map[ast.ASTHandle]float64),
		clauseMaintenance:     defaultSMTClauseMaintenanceConfig(),
		lemmaGCThreshold:      5000,
		newCore2thEq:          true,
	}
	state.booleanData[0].Initialize(0)
	// context::init creates the true variable through mk_bool_var, which
	// counts it in m_num_mk_bool_var.
	state.statistics.NumMkBoolVar = 1
	if smtTraceEnabled("mk_bool_var") {
		if t, err := manager.MkTrue(); err == nil {
			state.traceMkBoolVar(0, t)
		}
	}
	dynamicAck, err := newSMTDynAckManager(state, defaultSMTDynAckState())
	if err != nil {
		_ = table.Close()
		return nil, err
	}
	state.dynamicAck = dynamicAck
	return state, nil
}

// initContextEnodes is the end of context::init: the true and false enodes.
func (s *SMTInternalizerState) initContextEnodes() error {
	if s.initEnodes != 0 {
		return nil
	}
	for _, mk := range []func() (ast.ASTHandle, error){s.manager.MkTrue, s.manager.MkFalse} {
		value, err := mk()
		if err != nil {
			return err
		}
		if node := s.context.enodes.get(value); node == nil {
			if _, err = s.MkEnode(value, true, true, false); err != nil {
				return err
			}
		}
	}
	s.initEnodes = smtInitEnodes
	return nil
}

// ConfigureDynamicAck installs source dynamic-Ackermann parameters before any
// expressions or scopes are created.
func (s *SMTInternalizerState) ConfigureDynamicAck(config SMTDynAckState) error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.booleanStack) != 0 || len(s.enodeStack) != s.initEnodes || len(s.scopes) != 0 {
		return ast.ErrASTDomain
	}
	replacement, err := newSMTDynAckManager(s, config)
	if err != nil {
		return err
	}
	if err = s.dynamicAck.close(); err != nil {
		_ = replacement.close()
		return err
	}
	s.dynamicAck = replacement
	return nil
}

// UpdateDynamicAck changes the source scalar parameters without discarding
// occurrence counts, pending candidates, or installed dynamic-Ack clauses.
func (s *SMTInternalizerState) UpdateDynamicAck(config SMTDynAckState) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.dynamicAck.updateConfig(config)
}

func (s *SMTInternalizerState) RegisterTheory(theory SMTTheoryInternalizer) error {
	if err := s.open(); err != nil {
		return err
	}
	if theory == nil || theory.FamilyID() == ast.NullFamilyID || theory.FamilyID() == s.manager.BasicFamilyID() || s.theories[theory.FamilyID()] != nil || len(s.scopes) != 0 || s.theoriesSetup {
		return ast.ErrASTDomain
	}
	family := theory.FamilyID()
	s.theories[family] = theory
	s.theoryOrder = append(s.theoryOrder, theory)
	if err := theory.Initialize(s); err != nil {
		delete(s.theories, family)
		s.theoryOrder = s.theoryOrder[:len(s.theoryOrder)-1]
		return err
	}
	return nil
}

// Source context::is_relevant includes all internalized expressions when
// filtering is disabled. A standalone read-side context without a relevancy
// service retains its explicitly marked view.
func (s *SMTInternalizerState) isRelevant(expression ast.ASTHandle) bool {
	return s.context.isRelevant(expression)
}

func (s *SMTInternalizerState) Theory(family ast.FamilyID) SMTTheoryInternalizer {
	if s == nil {
		return nil
	}
	return s.theories[family]
}

// SetBinaryClauseOptimization selects the source compact representation for
// eligible two-literal clauses. It must be configured before internalization.
func (s *SMTInternalizerState) SetBinaryClauseOptimization(enabled bool) error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.booleanStack) != 0 || len(s.enodeStack) != s.initEnodes || len(s.clauses) != 0 || len(s.scopes) != 0 {
		return ast.ErrASTDomain
	}
	if enabled {
		proofs, err := s.manager.ProofsEnabled()
		if err != nil {
			return err
		}
		if proofs {
			return ast.ErrASTDomain
		}
	}
	s.binaryClauseOptimization = enabled
	return nil
}

func (s *SMTInternalizerState) SetVariableTheory(variable int32, theory SMTTheoryID) error {
	if err := s.open(); err != nil {
		return err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(s.booleanData) || theory == SMTNullTheoryID || s.theories[theory] == nil || s.booleanData[variable].IsTheoryAtom() {
		return ast.ErrASTDomain
	}
	if s.scopeLevel > s.booleanData[variable].InternalLevel() {
		s.notifyTrail = append(s.notifyTrail, smtInternalizerNotifyTrail{variable: variable, theory: s.booleanData[variable].Theory()})
	}
	return s.booleanData[variable].SetNotifyTheory(theory)
}

func (s *SMTInternalizerState) attachTheoryVariable(node *SMTEnode, theory SMTTheoryID, variable SMTTheoryVar) error {
	if err := s.open(); err != nil {
		return err
	}
	plugin := s.theories[theory]
	if node == nil || node.manager != s.manager || theory == SMTNullTheoryID || variable == SMTNullTheoryVar || plugin == nil {
		return ast.ErrASTDomain
	}
	old := node.TheoryVar(theory)
	trail := smtInternalizerTheoryTrail{node: node, theory: theory, old: old}
	if old == SMTNullTheoryVar {
		root := node.root
		if root == nil {
			return ast.ErrASTDomain
		}
		rootVariable := root.TheoryVar(theory)
		if err := node.AddTheoryVar(theory, variable); err != nil {
			return err
		}
		if root != node && rootVariable == SMTNullTheoryVar {
			if err := root.AddTheoryVar(theory, variable); err != nil {
				_ = node.DeleteTheoryVar(theory)
				return err
			}
			trail.root, trail.rootAdded = root, true
		}
		if rootVariable == SMTNullTheoryVar {
			if err := s.enqueueExistingDisequalities(root, variable, plugin); err != nil {
				if trail.rootAdded {
					_ = root.DeleteTheoryVar(theory)
				}
				_ = node.DeleteTheoryVar(theory)
				return err
			}
		} else if root != node {
			if err := s.enqueueTheoryEquality(theory, rootVariable, variable); err != nil {
				_ = node.DeleteTheoryVar(theory)
				return err
			}
		}
	} else if err := node.ReplaceTheoryVar(theory, variable); err != nil {
		return err
	} else if err := s.enqueueTheoryEquality(theory, variable, old); err != nil {
		_ = node.ReplaceTheoryVar(theory, old)
		return err
	}
	s.theoryTrail = append(s.theoryTrail, trail)
	return nil
}

// undoTheoryAttachments must run before restoring an older equality snapshot,
// which also contains copies of the affected nodes' theory-variable lists.
func (s *SMTInternalizerState) undoTheoryAttachments(limit int) error {
	if limit < 0 || limit > len(s.theoryTrail) {
		return ast.ErrASTDomain
	}
	for len(s.theoryTrail) > limit {
		index := len(s.theoryTrail) - 1
		entry := s.theoryTrail[index]
		s.theoryTrail = s.theoryTrail[:index]
		if entry.old == SMTNullTheoryVar {
			if err := entry.node.DeleteTheoryVar(entry.theory); err != nil {
				return fmt.Errorf("delete node theory variable: %w", err)
			}
			if entry.rootAdded {
				if err := entry.root.DeleteTheoryVar(entry.theory); err != nil {
					return fmt.Errorf("delete root theory variable: %w", err)
				}
			}
		} else if err := entry.node.ReplaceTheoryVar(entry.theory, entry.old); err != nil {
			return fmt.Errorf("restore node theory variable: %w", err)
		}
	}
	return nil
}

func (s *SMTInternalizerState) open() error {
	if err := s.openOwner(); err != nil {
		return err
	}
	if (s.pendingPop != nil || s.closing) && !s.popActive {
		return ast.ErrASTPending
	}
	return nil
}

func (s *SMTInternalizerState) openOwner() error {
	if s == nil || s.manager == nil || s.context == nil || s.congruence == nil || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *SMTInternalizerState) SetGeneration(generation uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	s.generation = generation
	if generation > s.statistics.MaxGeneration {
		s.statistics.MaxGeneration = generation
	}
	return nil
}

func (s *SMTInternalizerState) SetCachedGeneration(expression ast.ASTHandle, generation uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if _, err := s.manager.GetKind(expression); err != nil {
		return err
	}
	s.cachedGeneration[expression] = generation
	return nil
}

func (s *SMTInternalizerState) BoolVar(expression ast.ASTHandle) (int32, bool) {
	if s == nil {
		return SMTNullBoolVar, false
	}
	variable, found := s.boolVar.lookup(expression)
	return variable, found
}

func (s *SMTInternalizerState) BoolExpression(variable int32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(s.boolExpr) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return s.boolExpr[variable], nil
}

func (s *SMTInternalizerState) BooleanData(variable int32) (*SMTBoolVarData, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(s.booleanData) {
		return nil, ast.ErrASTDomain
	}
	return &s.booleanData[variable], nil
}

func (s *SMTInternalizerState) MkBoolVar(expression ast.ASTHandle) (int32, error) {
	if err := s.open(); err != nil {
		return SMTNullBoolVar, err
	}
	if _, found := s.boolVar.lookup(expression); found {
		return SMTNullBoolVar, ast.ErrASTDomain
	}
	sort, err := s.manager.GetSort(expression)
	if err != nil {
		return SMTNullBoolVar, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return SMTNullBoolVar, err
	}
	if sort != boolean || s.manager.IsTrue(expression) || s.manager.IsFalse(expression) {
		return SMTNullBoolVar, ast.ErrASTDomain
	}
	data := SMTBoolVarData{}
	data.Initialize(s.scopeLevel)
	data.basicOp = -1
	header, err := s.manager.NodeHeader(expression)
	if err != nil {
		return SMTNullBoolVar, err
	}
	if header.Kind == ast.ASTApp {
		if header.Family == s.manager.BasicFamilyID() {
			data.basicOp = header.DeclKind
			if data.basicOp == ast.ASTOpNot {
				data.notChild, err = s.manager.GetArg(expression, 0)
				if err != nil {
					return SMTNullBoolVar, err
				}
			}
			if data.basicOp == ast.ASTOpAnd || data.basicOp == ast.ASTOpOr {
				data.gateArgs = make([]ast.ASTHandle, header.NumArgs)
				for i := range data.gateArgs {
					data.gateArgs[i], err = s.manager.GetArg(expression, uint32(i))
					if err != nil {
						return SMTNullBoolVar, err
					}
				}
			}
		}
	}
	if err = s.manager.IncRef(expression); err != nil {
		return SMTNullBoolVar, err
	}
	variable := int32(len(s.boolExpr))
	s.boolVar.set(expression, variable)
	s.boolExpr = append(s.boolExpr, expression)
	s.booleanData = append(s.booleanData, data)
	s.nextBooleanCreation++
	s.booleanCreation = append(s.booleanCreation, s.nextBooleanCreation)
	initialActivity := 0.0
	if s.randomInitialActivity == 2 || s.randomInitialActivity == 1 && s.searching {
		initialActivity = -float64(s.random.Next()%1000) / 1000
	}
	if s.cachedActivity || s.caseSplitStrategy == 2 {
		if cached, found := s.activityCache[expression]; found {
			initialActivity = cached
		}
	}
	s.traceMkBoolVar(variable, expression)
	s.activity = append(s.activity, initialActivity)
	s.assignment = append(s.assignment, lbool.Undef, lbool.Undef)
	s.watches = append(s.watches, SMTWatchList{}, SMTWatchList{})
	s.booleanStack = append(s.booleanStack, expression)
	s.internalizeSeq++
	s.booleanSeq = append(s.booleanSeq, s.internalizeSeq)
	s.statistics.NumMkBoolVar++
	if err := s.basicActivityCreated(variable); err != nil {
		return SMTNullBoolVar, err
	}
	if err := s.relevanceActivityCreated(variable); err != nil {
		return SMTNullBoolVar, err
	}
	if s.theoryAwareBranching != nil {
		if err := s.theoryAwareBranching.created(variable); err != nil {
			return SMTNullBoolVar, err
		}
	}
	return variable, nil
}

func (s *SMTInternalizerState) Literal(expression ast.ASTHandle) (SMTLiteral, error) {
	if err := s.open(); err != nil {
		return SMTNullLiteral, err
	}
	if s.manager.IsTrue(expression) {
		return SMTTrueLiteral, nil
	}
	if s.manager.IsFalse(expression) {
		return SMTFalseLiteral, nil
	}
	header, err := s.manager.NodeHeader(expression)
	if err != nil {
		return SMTNullLiteral, err
	}
	if header.Kind == ast.ASTApp && header.Family == s.manager.BasicFamilyID() && header.DeclKind == ast.ASTOpNot {
		child, err := s.manager.GetArg(expression, 0)
		if err != nil {
			return SMTNullLiteral, err
		}
		// Reserved truth literals have no ordinary boolVar table entry.
		if s.manager.IsTrue(child) {
			return SMTFalseLiteral, nil
		}
		if s.manager.IsFalse(child) {
			return SMTTrueLiteral, nil
		}
		variable, found := s.boolVar.lookup(child)
		if !found {
			return SMTNullLiteral, ast.ErrASTDomain
		}
		return NewSMTLiteral(variable, true), nil
	}
	variable, found := s.boolVar.lookup(expression)
	if !found {
		return SMTNullLiteral, ast.ErrASTDomain
	}
	return NewSMTLiteral(variable, false), nil
}

func (s *SMTInternalizerState) SetAssignment(variable int32, value lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(s.boolExpr) || value < lbool.False || value > lbool.True {
		return ast.ErrASTDomain
	}
	positive := NewSMTLiteral(variable, false)
	old := s.assignment[positive.Index()]
	if old == value {
		return nil
	}
	if len(s.scopes) != 0 {
		s.assignmentTrail = append(s.assignmentTrail, smtInternalizerAssignmentTrail{
			variable: variable, value: old, justification: s.booleanData[variable].Justification(), scopeLevel: s.booleanData[variable].ScopeLevel,
		})
	}
	s.assignment[positive.Index()] = value
	s.assignment[positive.Negated().Index()] = value.Negated()
	if value == lbool.Undef {
		if err := s.basicActivityUnassigned(variable); err != nil {
			return err
		}
	}
	if value == lbool.Undef && s.theoryAwareBranching != nil {
		if err := s.theoryAwareBranching.unassigned(variable); err != nil {
			return err
		}
	}
	return s.context.SetAssignment(s.boolExpr[variable], value)
}

func (s *SMTInternalizerState) Assignment(literal SMTLiteral) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	if literal.Index() < 0 || int(literal.Index()) >= len(s.assignment) {
		return lbool.Undef, ast.ErrASTDomain
	}
	return s.assignment[literal.Index()], nil
}

func (s *SMTInternalizerState) WatchList(literal SMTLiteral) (*SMTWatchList, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if literal.Index() < 0 || int(literal.Index()) >= len(s.watches) {
		return nil, ast.ErrASTDomain
	}
	return &s.watches[literal.Index()], nil
}

// smtInitEnodes counts the true and false enodes context::init creates.
const smtInitEnodes = 2

func (s *SMTInternalizerState) MkEnode(expression ast.ASTHandle, suppressArguments, mergeTruthValue, congruenceEnabled bool) (*SMTEnode, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.context.enodes.get(expression) != nil {
		return nil, ast.ErrASTDomain
	}
	generation := s.generation
	if cached, found := s.cachedGeneration[expression]; found {
		generation = cached
	}
	if err := s.manager.IncRef(expression); err != nil {
		return nil, err
	}
	node, err := NewSMTEnode(s.manager, expression, s.context.enodes.get, SMTEnodeOptions{
		Generation: generation, InternalScopeLevel: s.scopeLevel, SuppressArguments: suppressArguments,
		MergeTruthValue: mergeTruthValue, CongruenceEnabled: congruenceEnabled, UpdateChildrenParent: true,
	})
	if err != nil {
		_ = s.manager.DecRef(expression)
		return nil, err
	}
	count := node.NumArgs()
	if count == 0 {
		unique, err := s.manager.IsUniqueValue(expression)
		if err != nil {
			_ = node.Delete(true)
			_ = s.manager.DecRef(expression)
			return nil, err
		}
		if unique {
			if err = node.MarkAsInterpreted(); err != nil {
				_ = node.Delete(true)
				_ = s.manager.DecRef(expression)
				return nil, err
			}
		}
	} else if node.IsTrueEquality() {
		node.congruence = node
	} else if congruenceEnabled {
		inserted, err := s.congruence.Insert(node)
		if err != nil {
			_ = node.Delete(true)
			_ = s.manager.DecRef(expression)
			return nil, err
		}
		node.congruence = inserted.Node
		if inserted.Node != node {
			s.pending = append(s.pending, smtPendingEquality{node, inserted.Node, NewSMTEqualityCongruence(inserted.Commuted)})
		}
	} else {
		node.congruence = node
	}
	// Source mk_enode does not reset an expression's existing relevance.
	// Gate-context Boolean atoms can be marked before a term-context enode
	// is needed, including during a later search scope.
	if err = s.context.RegisterEnode(node, s.context.relevant.get(expression)); err != nil {
		if node.congruence == node && count != 0 && congruenceEnabled && !node.IsTrueEquality() {
			_, _ = s.congruence.Erase(node)
		}
		_ = node.Delete(true)
		_ = s.manager.DecRef(expression)
		return nil, err
	}
	s.enodeStack = append(s.enodeStack, node)
	s.internalizeSeq++
	s.enodeSeq = append(s.enodeSeq, s.internalizeSeq)
	s.statistics.NumMkEnode++
	if smtTraceEnabled("mk_enode") {
		s.traceMkEnode(node, count, congruenceEnabled)
	}
	// Source mk_enode assigns a newly created true equality immediately.
	// Its operands may already have merged before this atom was internalized.
	if node.IsTrueEquality() {
		if err := s.propagateTrueEquality(node); err != nil {
			return nil, err
		}
	}
	s.traceAttachEnode(node)
	return node, nil
}

func (s *SMTInternalizerState) PendingCongruences() []SMTInternalizerCongruence {
	if s == nil {
		return nil
	}
	var result []SMTInternalizerCongruence
	for _, entry := range s.pending {
		if commuted, congruence := entry.justification.UsedCommutativity(); congruence {
			result = append(result, SMTInternalizerCongruence{entry.left, entry.right, commuted})
		}
	}
	return result
}

func (s *SMTInternalizerState) PushScope() error {
	if err := s.open(); err != nil {
		return err
	}
	s.tracePushStream()
	s.scopes = append(s.scopes, smtInternalizerScope{
		enodeTrailMark: s.enodeTrail.Mark(),
		goalQueue:      s.relevanceGoal.snapshot(s.scopeLevel),
		relevanceFIFO:  s.relevanceFIFO.snapshot(),
		relActLimit:    len(s.relevanceActivity.entries), relActHead: s.relevanceActivity.head,
		trueFirstLimit:       len(s.trueFirstTrail),
		theoryCaseSplitLimit: len(s.theoryCaseSplitSets),
		booleanLimit:         len(s.booleanStack), enodeLimit: len(s.enodeStack), pendingLimit: len(s.pending), pendingHead: s.pendingHead,
		equalityLimit:   len(s.equalityTrail),
		assignmentLimit: len(s.assignmentTrail), clauseLimit: len(s.clauses), justificationLimit: len(s.ownedJustifications), theoryLimit: len(s.theoryTrail), inconsistent: s.inconsistent,
		assignedLimit: len(s.assignedLiterals), propagationHead: s.propagationHead, relevancyBatch: s.relevancyBatch, conflict: s.conflict,
		conflictLiteral:  s.conflictLiteral,
		disequalityLimit: len(s.context.disequalityTrail),
		theoryEqLimit:    len(s.theoryEqualities), theoryEqHead: s.theoryEqualityHead,
		theoryDiseqLimit: len(s.theoryDisequalities), theoryDiseqHead: s.theoryDisequalityHead,
		notifyLimit: len(s.notifyTrail), simplifyHead: s.simplifyHead,
		enodeFlagLimit:    len(s.enodeFlagTrail),
		mergeTFLimit:      len(s.mergeTFTrail),
		atomNotifiedLimit: len(s.atomNotifiedTrail),
	})
	if s.quantifiers != nil {
		if err := s.quantifiers.PushScope(); err != nil {
			s.scopes = s.scopes[:len(s.scopes)-1]
			return err
		}
	}
	if s.quantifierManager != nil {
		if err := s.quantifierManager.PushScope(); err != nil {
			_ = s.quantifiers.PopScopes(1)
			s.scopes = s.scopes[:len(s.scopes)-1]
			return err
		}
	}
	if s.relevancy != nil {
		if err := s.relevancy.PushScope(); err != nil {
			if s.quantifierManager != nil {
				_ = s.quantifierManager.PopScopes(1)
			}
			if s.quantifiers != nil {
				_ = s.quantifiers.PopScopes(1)
			}
			s.scopes = s.scopes[:len(s.scopes)-1]
			return err
		}
	}
	if err := s.traceCaseSplitScope(false); err != nil {
		return err
	}
	// context::push_scope: m_asserted_formulas.push_scope() precedes the
	// theories' push_scope_eh, for every scope (assumptions and decisions
	// included).
	if s.onScopePush != nil {
		if err := s.onScopePush(); err != nil {
			return err
		}
	}
	for index, theory := range s.theoryOrder {
		if err := theory.PushScope(); err != nil {
			for undo := index - 1; undo >= 0; undo-- {
				_ = s.theoryOrder[undo].PopScopes(1)
			}
			if s.relevancy != nil {
				_ = s.relevancy.PopScopes(1)
			}
			if s.quantifierManager != nil {
				_ = s.quantifierManager.PopScopes(1)
			}
			if s.quantifiers != nil {
				_ = s.quantifiers.PopScopes(1)
			}
			s.scopes = s.scopes[:len(s.scopes)-1]
			return err
		}
	}
	s.scopeLevel++
	return nil
}

func (s *SMTInternalizerState) unregisterEnode(node *SMTEnode) error {
	if s.retiringEnode != nil {
		if s.retiringEnode != node {
			return ast.ErrASTPending
		}
		return s.finishUnregisterEnode()
	}
	if s.context.enodes.get(node.owner) != node {
		return fmt.Errorf("enode owner %d is not current: %w", node.owner.Slot(), ast.ErrASTDomain)
	}
	bucket := s.context.byDeclaration[node.declaration]
	if len(bucket) == 0 || bucket[len(bucket)-1] != node {
		return fmt.Errorf("enode declaration %d is not bucket tail (size %d): %w", node.declaration.Slot(), len(bucket), ast.ErrASTDomain)
	}
	if node.IsCongruenceRoot() && node.NumArgs() != 0 && node.congruenceEnabled && !node.IsTrueEquality() {
		erased, err := s.congruence.Erase(node)
		if err != nil || !erased {
			if err != nil {
				return fmt.Errorf("erase congruence entry: %w", err)
			}
			return fmt.Errorf("congruence entry was absent: %w", ast.ErrASTDomain)
		}
	}
	s.context.enodes.del(node.owner)
	s.context.byDeclaration[node.declaration] = bucket[:len(bucket)-1]
	// Source relevancy marks belong to the expression's relevancy scope.
	// A base mark can precede an enode created only in this retiring scope.
	if s.relevancy == nil {
		s.context.relevant.del(node.owner)
	}
	if err := node.deleteAfterEqualityUndo(); err != nil {
		return fmt.Errorf("delete enode: %w", err)
	}
	s.retiringEnode, s.retiringEnodeRoot = node, node.owner
	return s.finishUnregisterEnode()
}

func (s *SMTInternalizerState) finishUnregisterEnode() error {
	if !s.retiringEnodeRoot.IsNull() {
		consumed, err := s.manager.DecRefConsumed(s.retiringEnodeRoot)
		if consumed {
			s.retiringEnodeRoot = ast.ASTHandle{}
		}
		if err != nil {
			return fmt.Errorf("release enode owner: %w", err)
		}
	}
	s.retiringEnode = nil
	s.statistics.NumDelEnode++
	return nil
}

// flushQuantifiers is context::flush's m_qmanager = nullptr.
func (s *SMTInternalizerState) flushQuantifiers() error {
	if s.quantifierManager != nil {
		if err := s.quantifierManager.flush(); err != nil {
			return err
		}
	}
	if s.quantifiers != nil {
		if err := s.quantifiers.schedule.releaseInstancesForFlush(); err != nil {
			return err
		}
	}
	if s.closing && s.onQuantifierFlush != nil {
		if err := s.onQuantifierFlush(); err != nil {
			return err
		}
		s.onQuantifierFlush = nil
	}
	s.flushQuantifiersInPop = false
	return nil
}

func (s *SMTInternalizerState) popTo(scope smtInternalizerScope, relevancyPops uint32) error {
	// context::pop_scope's undo_trail_stack for enode value trails.
	if err := s.enodeTrail.Undo(scope.enodeTrailMark); err != nil {
		return err
	}
	if err := s.relevanceGoal.restore(scope.goalQueue, s.caseSplitStrategy == 5); err != nil {
		return err
	}
	if scope.relActLimit < 0 || scope.relActLimit > len(s.relevanceActivity.entries) || scope.relActHead < 0 {
		return ast.ErrASTDomain
	}
	clear(s.relevanceActivity.entries[scope.relActLimit:])
	s.relevanceActivity.entries = s.relevanceActivity.entries[:scope.relActLimit]
	s.relevanceActivity.head = scope.relActHead
	if s.caseSplitStrategy == 4 {
		if err := smtCaseSplitAssert(591, "m_head <= m_queue.size()", func() bool { return s.relevanceActivity.head <= len(s.relevanceActivity.entries) }); err != nil {
			return err
		}
	}
	if s.relevanceActivity.head > len(s.relevanceActivity.entries) {
		return ast.ErrASTDomain
	}
	if err := s.relevanceFIFO.restore(scope.relevanceFIFO, s.caseSplitStrategy == 3); err != nil {
		return err
	}
	if err := s.traceCaseSplitScope(true); err != nil {
		return err
	}
	if err := s.undoTheoryCaseSplits(scope.theoryCaseSplitLimit); err != nil {
		return err
	}
	if scope.trueFirstLimit < 0 || scope.trueFirstLimit > len(s.trueFirstTrail) {
		return ast.ErrASTDomain
	}
	for len(s.trueFirstTrail) > scope.trueFirstLimit {
		i := len(s.trueFirstTrail) - 1
		s.booleanData[s.trueFirstTrail[i]].ResetTrueFirst()
		s.trueFirstTrail = s.trueFirstTrail[:i]
	}
	if scope.equalityLimit < 0 || scope.equalityLimit > len(s.equalityTrail) {
		return ast.ErrASTDomain
	}
	for len(s.equalityTrail) > scope.equalityLimit {
		index := len(s.equalityTrail) - 1
		entry := &s.equalityTrail[index]
		if err := s.undoTheoryAttachments(entry.theoryLimit); err != nil {
			return err
		}
		if err := s.undoEquality(entry); err != nil {
			return fmt.Errorf("undo equality: %w", err)
		}
		*entry = smtEqualityUndo{}
		s.equalityTrail = s.equalityTrail[:index]
	}
	if err := s.context.undoDisequalities(scope.disequalityLimit); err != nil {
		return err
	}
	if err := s.undoTheoryAttachments(scope.theoryLimit); err != nil {
		return err
	}
	if scope.assignmentLimit < 0 || scope.assignmentLimit > len(s.assignmentTrail) {
		return ast.ErrASTDomain
	}
	for len(s.assignmentTrail) > scope.assignmentLimit {
		entry := s.assignmentTrail[len(s.assignmentTrail)-1]
		positive := NewSMTLiteral(entry.variable, false)
		s.assignment[positive.Index()] = entry.value
		s.assignment[positive.Negated().Index()] = entry.value.Negated()
		s.booleanData[entry.variable].SetJustification(entry.justification)
		s.booleanData[entry.variable].ScopeLevel = entry.scopeLevel
		if entry.value == lbool.Undef {
			if err := s.basicActivityUnassigned(entry.variable); err != nil {
				return err
			}
			if err := s.relevanceActivityUnassigned(entry.variable); err != nil {
				return err
			}
		}
		if entry.value == lbool.Undef && s.theoryAwareBranching != nil {
			if err := s.theoryAwareBranching.unassigned(entry.variable); err != nil {
				return err
			}
		}
		if err := s.context.SetAssignment(s.boolExpr[entry.variable], entry.value); err != nil {
			return fmt.Errorf("restore Boolean assignment: %w", err)
		}
		s.assignmentTrail = s.assignmentTrail[:len(s.assignmentTrail)-1]
	}
	if scope.assignedLimit < 0 || scope.assignedLimit > len(s.assignedLiterals) {
		return ast.ErrASTDomain
	}
	s.assignedLiterals = s.assignedLiterals[:scope.assignedLimit]
	if scope.propagationHead < 0 || scope.propagationHead > len(s.assignedLiterals) {
		return ast.ErrASTDomain
	}
	s.propagationHead = scope.propagationHead
	s.relevancyBatch = scope.relevancyBatch
	s.simplifyHead = scope.simplifyHead
	if scope.theoryEqLimit < 0 || scope.theoryEqLimit > len(s.theoryEqualities) || scope.theoryEqHead < 0 || scope.theoryEqHead > scope.theoryEqLimit {
		return ast.ErrASTDomain
	}
	s.theoryEqualities = s.theoryEqualities[:scope.theoryEqLimit]
	s.theoryEqualityHead = scope.theoryEqHead
	if scope.theoryDiseqLimit < 0 || scope.theoryDiseqLimit > len(s.theoryDisequalities) || scope.theoryDiseqHead < 0 || scope.theoryDiseqHead > scope.theoryDiseqLimit {
		return ast.ErrASTDomain
	}
	s.theoryDisequalities = s.theoryDisequalities[:scope.theoryDiseqLimit]
	s.theoryDisequalityHead = scope.theoryDiseqHead
	if scope.notifyLimit < 0 || scope.notifyLimit > len(s.notifyTrail) {
		return ast.ErrASTDomain
	}
	s.atomQueue = s.atomQueue[:0] // pop_scope: m_atom_propagation_queue.reset()
	if scope.atomNotifiedLimit < 0 || scope.atomNotifiedLimit > len(s.atomNotifiedTrail) {
		return ast.ErrASTDomain
	}
	for len(s.atomNotifiedTrail) > scope.atomNotifiedLimit {
		i := len(s.atomNotifiedTrail) - 1
		delete(s.atomNotified, s.atomNotifiedTrail[i])
		s.atomNotifiedTrail = s.atomNotifiedTrail[:i]
	}
	if scope.enodeFlagLimit < 0 || scope.enodeFlagLimit > len(s.enodeFlagTrail) {
		return ast.ErrASTDomain
	}
	for len(s.enodeFlagTrail) > scope.enodeFlagLimit {
		i := len(s.enodeFlagTrail) - 1
		s.booleanData[s.enodeFlagTrail[i]].ResetENode()
		s.enodeFlagTrail = s.enodeFlagTrail[:i]
	}
	if scope.mergeTFLimit < 0 || scope.mergeTFLimit > len(s.mergeTFTrail) {
		return ast.ErrASTDomain
	}
	for len(s.mergeTFTrail) > scope.mergeTFLimit {
		i := len(s.mergeTFTrail) - 1
		s.mergeTFTrail[i].mergeTruthValue = false
		s.mergeTFTrail[i] = nil
		s.mergeTFTrail = s.mergeTFTrail[:i]
	}
	for len(s.notifyTrail) > scope.notifyLimit {
		index := len(s.notifyTrail) - 1
		entry := s.notifyTrail[index]
		s.notifyTrail = s.notifyTrail[:index]
		s.booleanData[entry.variable].ResetNotifyTheory()
		if entry.theory != SMTNullTheoryID {
			if err := s.booleanData[entry.variable].SetNotifyTheory(entry.theory); err != nil {
				return err
			}
		}
	}
	if scope.clauseLimit < 0 || scope.clauseLimit > len(s.clauses) {
		return ast.ErrASTDomain
	}
	flushing := s.closing
	for index := len(s.clauses); index > scope.clauseLimit; index-- {
		record := &s.clauses[index-1]
		if s.keepClauseAcrossPop(record) {
			if record.clause == nil {
				continue // binary lemma: watches only
			}
			retired := false
			for _, literal := range record.clause.literals {
				retired = retired || int(literal.Var()) >= scope.booleanLimit+int(SMTFirstBoolVar)
			}
			if retired {
				// Detach before retiring Boolean variables. closeWatches also
				// makes destruction safe if a later owner refuses the pop.
				for record.closeWatches < 2 {
					literal := record.clause.literals[record.closeWatches]
					if !s.watches[literal.Negated().Index()].RemoveClause(record.clause) {
						return ast.ErrASTDomain
					}
					record.closeWatches++
				}
			}
			continue
		}
		if flushing && !record.Binary && record.clause != nil {
			// context::flush runs undo_trail_stack(0) before del_clauses: the
			// watches go now, the clause (its proof, then its atoms) after the
			// trail, auxiliary clauses before lemmas, newest first. The record
			// keeps the detached clause until then, so a refused step can
			// resume here.
			if err := s.detachClause(record); err != nil {
				return fmt.Errorf("close clause: %w", err)
			}
			continue
		}
		if err := s.closeClause(&s.clauses[index-1]); err != nil {
			return fmt.Errorf("close clause: %w", err)
		}
	}
	compactClauses := func() {
		write := scope.clauseLimit
		for index := scope.clauseLimit; index < len(s.clauses); index++ {
			if s.keepClauseAcrossPop(&s.clauses[index]) {
				s.clauses[write] = s.clauses[index]
				write++
			} else if !s.clauses[index].Binary {
				// del_clause deletes allocated clauses; binary clauses live
				// only in the watch lists and are not counted.
				s.statistics.NumDelClause++
			}
		}
		clear(s.clauses[write:])
		s.clauses = s.clauses[:write]
	}
	if !flushing {
		compactClauses()
	}
	if scope.justificationLimit < 0 || scope.justificationLimit > len(s.ownedJustifications) {
		return ast.ErrASTDomain
	}
	// pop_scope_core: m_relevancy_propagator->pop follows del_clauses and
	// precedes undo_trail_stack.
	if relevancyPops != 0 && s.relevancy != nil && (s.pendingPop == nil || !s.pendingPop.relevancyPopped) {
		if err := s.relevancy.PopScopes(relevancyPops); err != nil {
			return err
		}
		if s.pendingPop != nil {
			s.pendingPop.relevancyPopped = true
		}
	}
	// undo_trail_stack: mk_enode and mk_bool_var entries, newest first.
	for {
		enodeMore := len(s.enodeStack) > scope.enodeLimit
		booleanMore := len(s.booleanStack) > scope.booleanLimit
		if !enodeMore && !booleanMore {
			break
		}
		if enodeMore && !s.retiringBoolean && (!booleanMore || s.enodeSeq[len(s.enodeSeq)-1] > s.booleanSeq[len(s.booleanSeq)-1]) {
			node := s.enodeStack[len(s.enodeStack)-1]
			if err := s.unregisterEnode(node); err != nil {
				return fmt.Errorf("unregister enode: %w", err)
			}
			s.enodeStack = s.enodeStack[:len(s.enodeStack)-1]
			s.enodeSeq = s.enodeSeq[:len(s.enodeSeq)-1]
			continue
		}
		expression := s.booleanStack[len(s.booleanStack)-1]
		if !s.retiringBoolean {
			variable := s.boolVar.get(expression)
			if (s.cachedActivity || s.caseSplitStrategy == 2) && s.searching && s.activity[variable] > 0 {
				if err := s.cacheBooleanActivity(expression, s.activity[variable]); err != nil {
					return err
				}
			}
			if err := s.basicActivityDeleted(variable); err != nil {
				return err
			}
			if err := s.relevanceActivityDeleted(variable); err != nil {
				return err
			}
			if s.theoryAwareBranching != nil {
				if err := s.theoryAwareBranching.deleted(variable); err != nil {
					return err
				}
			}
			if s.quantifiers != nil {
				kind, err := s.manager.GetKind(expression)
				if err != nil {
					return err
				}
				if kind == ast.ASTQuantifier {
					if _, statErr := s.quantifiers.Stat(expression); statErr == nil {
						if err = s.quantifiers.Delete(expression); err != nil {
							return err
						}
					} else if !errors.Is(statErr, ast.ErrASTDomain) {
						return statErr
					}
				}
			}
			s.unindexEqualityAtom(variable, expression)
			s.boolVar.del(expression)
			s.context.assignments.del(expression)
			if s.relevancy == nil {
				s.context.relevant.del(expression)
			}
			s.boolExpr[variable] = ast.ASTHandle{}
			s.boolExpr = s.boolExpr[:variable]
			s.booleanData = s.booleanData[:variable]
			s.booleanCreation = s.booleanCreation[:variable]
			s.activity = s.activity[:variable]
			s.assignment = s.assignment[:2*variable]
			s.watches = s.watches[:2*variable]
			s.retiringBoolean, s.retiringBooleanRoot = true, expression
		}
		if !s.retiringBooleanRoot.IsNull() {
			consumed, err := s.manager.DecRefConsumed(s.retiringBooleanRoot)
			if consumed {
				s.retiringBooleanRoot = ast.ASTHandle{}
			}
			if err != nil {
				return fmt.Errorf("release Boolean expression: %w", err)
			}
		}
		s.retiringBoolean = false
		s.booleanStack = s.booleanStack[:len(s.booleanStack)-1]
		s.booleanSeq = s.booleanSeq[:len(s.booleanSeq)-1]
		s.statistics.NumDelBoolVar++
	}
	if s.flushQuantifiersInPop {
		if err := s.flushQuantifiers(); err != nil {
			return err
		}
	}
	if flushing {
		// del_clauses(m_aux_clauses, 0); del_clauses(m_lemmas, 0): newest first.
		for _, lemmas := range []bool{false, true} {
			for index := len(s.clauses); index > scope.clauseLimit; index-- {
				record := &s.clauses[index-1]
				if record.Binary || record.clause == nil || record.clause.IsLemma() != lemmas {
					continue
				}
				if err := record.clause.Close(); err != nil {
					return fmt.Errorf("close clause: %w", err)
				}
				record.clause = nil
			}
		}
		compactClauses()
	}
	if scope.pendingLimit < 0 || scope.pendingLimit > len(s.pending) {
		return ast.ErrASTDomain
	}
	clear(s.pending[scope.pendingLimit:])
	s.pending = s.pending[:scope.pendingLimit]
	if scope.pendingHead < 0 || scope.pendingHead > len(s.pending) {
		return ast.ErrASTDomain
	}
	s.pendingHead = scope.pendingHead
	s.inconsistent = scope.inconsistent
	s.conflict = scope.conflict
	s.conflictLiteral = scope.conflictLiteral
	return nil
}

// releaseJustificationsTo is del_justifications: pop_scope_core runs it
// after undo_trail_stack and the theories' pop_scope_eh.
func (s *SMTInternalizerState) releaseJustificationsTo(limit int) error {
	if limit < 0 || limit > len(s.ownedJustifications) {
		return ast.ErrASTDomain
	}
	for len(s.ownedJustifications) > limit {
		index := len(s.ownedJustifications) - 1
		if err := s.ownedJustifications[index].Release(s.manager); err != nil {
			return fmt.Errorf("release owned justification: %w", err)
		}
		s.ownedJustifications = s.ownedJustifications[:index]
	}
	return nil
}

// A source scope pop visits several owners. Keep its cursor across Go native
// cleanup errors so a completed child scope is never consumed a second time.
type smtInternalizerPendingPop struct {
	count    uint32
	level    int
	stage    uint8
	theory   int
	preserve bool
	learned  []smtPreservedLearnedClause
	// quantifiersPopped and assertedPopped make stage 5 resumable.
	quantifiersPopped, assertedPopped bool
	// qmanagerPopped marks pop_scope_core's leading m_qmanager->pop: the
	// plugin, then the qi_queue instances.
	qmanagerPopped bool
	// relevancyPopped makes popTo's relevancy pop run once across retries.
	relevancyPopped bool
}

// smtPopOwnerReleaser is a theory that keeps Go owners of atoms the context
// also internalized; it releases those of the popped scopes before the
// context unwinds.
type smtPopOwnerReleaser interface {
	releasePoppedOwners(count uint32) error
}

func (s *SMTInternalizerState) PopScopes(count uint32) error {
	if err := s.openOwner(); err != nil {
		return err
	}
	if s.closing && !s.popActive {
		return ast.ErrASTPending
	}
	if s.pendingPop != nil {
		if s.pendingPop.count != count {
			return ast.ErrASTPending
		}
	} else {
		if err := s.assertCaseSplitPopCount(count); err != nil {
			return err
		}
		if count == 0 || count > uint32(len(s.scopes)) {
			return ast.ErrASTDomain
		}
		s.tracePopStream(count)
		level := len(s.scopes) - int(count)
		s.pendingPop = &smtInternalizerPendingPop{count: count, level: level, theory: 0,
			preserve: uint32(level) >= s.baseLevel}
	}
	pop := s.pendingPop
	active := s.popActive
	s.popActive = true
	defer func() { s.popActive = active }()
	if pop.stage == 0 && !pop.qmanagerPopped {
		// pop_scope_core begins with m_qmanager->pop(num_scopes):
		// default_qm_plugin::pop, then m_qi_queue.pop_scope.
		if s.quantifierManager != nil {
			if err := s.quantifierManager.PopScopes(count); err != nil {
				return err
			}
		}
		if s.quantifiers != nil && !s.quantifiers.popScheduled {
			if err := s.quantifiers.schedule.PopScopes(count); err != nil {
				return err
			}
			s.quantifiers.popScheduled = true
		}
		pop.qmanagerPopped = true
	}
	if pop.stage == 0 {
		if !pop.preserve && !s.scopes[pop.level].inconsistent && s.baseScopePop != nil {
			// new_lvl < m_base_lvl && !bs.m_inconsistent: m_unsat_proof = nullptr.
			if err := s.baseScopePop(); err != nil {
				return err
			}
		}
		if pop.preserve {
			if len(pop.learned) != 0 {
				if err := s.releasePreservedLearnedClauses(pop.learned); err != nil {
					return err
				}
				pop.learned = nil
			}
			var err error
			pop.learned, err = s.preserveLearnedClauses(s.scopes[pop.level].clauseLimit)
			if err != nil {
				return err
			}
		}
		pop.stage++
	}
	if pop.stage == 1 {
		// The source's theories hold no references to their internalized
		// atoms, so undo_trail_stack frees them in context order: drop the
		// theories' extra owners of the popped atoms first.
		for _, th := range s.theoryOrder {
			if r, ok := th.(smtPopOwnerReleaser); ok {
				if err := r.releasePoppedOwners(count); err != nil {
					return err
				}
			}
		}
		if err := s.popTo(s.scopes[pop.level], count); err != nil {
			return err
		}
		pop.stage++
	}
	if pop.stage == 2 {
		// The relevancy pop ran inside popTo, in pop_scope_core order.
		pop.stage++
	}
	if pop.stage == 3 {
		// for (theory* th : m_theory_set) th->pop_scope_eh(num_scopes), in
		// registration order.
		for pop.theory < len(s.theoryOrder) {
			if err := s.theoryOrder[pop.theory].PopScopes(count); err != nil {
				return err
			}
			pop.theory++
		}
		if err := s.releaseJustificationsTo(s.scopes[pop.level].justificationLimit); err != nil {
			return err
		}
		if s.assertedPopFlush != nil && !s.userPop {
			if err := s.assertedPopFlush(); err != nil {
				return err
			}
		}
		pop.stage++
	}
	if pop.stage == 4 {
		// m_qmanager->pop ran first (stage 0).
		pop.stage++
	}
	if pop.stage == 5 {
		if s.quantifiers != nil && !pop.quantifiersPopped {
			if err := s.quantifiers.PopScopes(count); err != nil {
				return err
			}
			pop.quantifiersPopped = true
		}
		// m_asserted_formulas.pop_scope(num_scopes) in pop_scope_core.
		if s.onScopePop != nil && !pop.assertedPopped {
			if err := s.onScopePop(count); err != nil {
				return err
			}
			pop.assertedPopped = true
		}
		s.scopes = s.scopes[:pop.level]
		s.scopeLevel = uint32(pop.level)
		if s.scopeLevel < s.baseLevel {
			s.baseLevel = s.scopeLevel
		}
		pop.stage++
	}
	if pop.stage == 6 {
		if pop.preserve {
			if err := s.restoreLearnedClauses(pop.learned); err != nil {
				return err
			}
			pop.learned = nil
		}
		pop.stage++
	}
	if pop.preserve {
		if err := s.restoreLearnedUnits(uint32(pop.level)); err != nil {
			return err
		}
	} else if err := s.discardLearnedUnitsAbove(uint32(pop.level)); err != nil {
		return err
	}
	s.pendingPop = nil
	return nil
}

func (s *SMTInternalizerState) Statistics() SMTContextStatistics {
	if s == nil {
		return SMTContextStatistics{}
	}
	return s.statistics
}

func (s *SMTInternalizerState) Close() error {
	if s == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if s.manager == nil {
		return ast.ErrASTHandle
	}
	s.closing = true
	active := s.popActive
	s.popActive = true
	defer func() { s.popActive = active }()
	if s.lemmaProblem != nil {
		if err := s.lemmaProblem.close(); err != nil {
			return err
		} else {
			s.lemmaProblem = nil
		}
	}
	if s.pendingPop != nil {
		// context::flush destroys owners instead of completing an operational
		// pop. A numeric theory may already have consumed its scope and allow
		// only Close after failure. Retire the pop's lexical roots without
		// restoring learned clauses or invoking that theory a second time.
		if err := s.releasePreservedLearnedClauses(s.pendingPop.learned); err != nil {
			return fmt.Errorf("close preserved SMT clauses: %w", err)
		}
		s.pendingPop = nil
	}
	// context::flush: undo_trail_stack(0) releases the enodes and Boolean
	// variables, then m_qmanager = nullptr (plugin, the qi_queue instances
	// and substitution cache); popTo runs flushQuantifiers between them.
	s.flushQuantifiersInPop = true
	if err := s.popTo(smtInternalizerScope{}, 0); err != nil {
		return fmt.Errorf("close SMT root scope: %w", err)
	}
	if err := s.releaseJustificationsTo(0); err != nil {
		return fmt.Errorf("close SMT root scope: %w", err)
	}
	if s.relevancy != nil && len(s.relevancy.scopes) != 0 {
		if err := s.relevancy.PopScopes(uint32(len(s.relevancy.scopes))); err != nil {
			return err
		}
	}
	s.scopes = nil
	s.scopeLevel = 0
	s.baseLevel = 0
	if s.afterFlush != nil {
		if err := s.afterFlush(); err != nil {
			return err
		}
		s.afterFlush = nil
	}
	for len(s.theoryOrder) != 0 {
		i := len(s.theoryOrder) - 1
		if closer, ok := s.theoryOrder[i].(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				return fmt.Errorf("close SMT theory %T: %w", s.theoryOrder[i], err)
			}
		}
		s.theoryOrder = s.theoryOrder[:i]
	}
	if err := s.releaseLearnedUnits(); err != nil {
		return err
	}
	if err := s.releaseBooleanActivityCache(); err != nil {
		return err
	}
	if s.theoryAwareBranching != nil {
		if err := s.theoryAwareBranching.heap.Close(); err != nil {
			return err
		}
		s.theoryAwareBranching = nil
	}
	if s.delayedActivity != nil {
		if err := s.delayedActivity.Close(); err != nil {
			return err
		} else {
			s.delayedActivity = nil
		}
	}
	if s.basicActivity != nil {
		if err := s.basicActivity.Close(); err != nil {
			return err
		} else {
			s.basicActivity = nil
		}
	}
	if s.relevanceGoal.heap != nil {
		if err := s.relevanceGoal.heap.Close(); err != nil {
			return err
		} else {
			s.relevanceGoal.heap = nil
		}
	}
	if s.relevanceActivity.delayed != nil {
		if err := s.relevanceActivity.delayed.Close(); err != nil {
			return err
		} else {
			s.relevanceActivity.delayed = nil
		}
	}
	if err := s.dynamicAck.close(); err != nil {
		return err
	}
	if s.quantifierManager != nil {
		if err := s.quantifierManager.Close(); err != nil {
			return err
		}
		s.quantifierManager = nil
	}
	if s.quantifiers != nil {
		if err := s.quantifiers.schedule.Reset(); err != nil {
			return err
		}
		// The registry is borrowed. Its queued instances and scope bookkeeping
		// belong to this context; other registrations are left with its owner.
		if s.quantifiers.fingerprints != nil {
			s.quantifiers.fingerprints.Reset()
			s.quantifiers.fingerprints.scopes = nil
		}
		s.quantifiers.scopeLevel = 0
		s.quantifiers.pendingPop = 0
		s.quantifiers.popScheduled = false
		s.quantifiers = nil
	}
	if s.context != nil {
		if err := s.context.releaseDiseqTmp(); err != nil {
			return err
		}
	}
	if err := s.congruence.Close(); err != nil {
		return err
	}
	if s.relevancy != nil && s.relevancy.internalizer == s {
		s.relevancy.internalizer = nil
	}
	s.closed = true
	s.theoryRewriter = nil
	s.assertedPopFlush = nil
	s.baseScopePop = nil
	s.smtParams = nil
	s.context = nil
	s.manager = nil
	return nil
}

// initTrueProof is the proof branch of context::init: with proofs enabled
// the true variable is justified by a proof wrapper of mk_true_proof(),
// held until flush. The source runs init in the context constructor body,
// after asserted_formulas and its members are built.
func (s *SMTInternalizerState) initTrueProof() error {
	proofs, err := s.manager.ProofsEnabled()
	if err != nil || !proofs {
		return err
	}
	proof, err := s.manager.MkProofTrue()
	if err != nil {
		return err
	}
	owner, err := NewSMTJustificationProofWrapper(s.manager, proof, true)
	if err != nil {
		return err
	}
	s.ownedJustifications = append(s.ownedJustifications, owner)
	return nil
}
