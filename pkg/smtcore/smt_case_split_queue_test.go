package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTActivityCaseSplitQueueUsesActivityAssignmentAndPhase(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "case-split-a")
	b := smtRelevancyBool(t, m, "case-split-b")
	astTestMust(t, state.Internalize(a, true))
	astTestMust(t, state.Internalize(b, true))
	av, found := state.BoolVar(a)
	if !found {
		t.Fatal("missing first variable")
	}
	bv, found := state.BoolVar(b)
	if !found {
		t.Fatal("missing second variable")
	}
	state.activity[bv] = 4
	state.booleanData[bv].SetTrueFirst()
	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	variable, phase, err := queue.NextCaseSplit()
	astTestMust(t, err)
	if variable != bv || phase != lbool.True {
		t.Fatal("activity/phase selection", variable, phase, bv)
	}
	astTestMust(t, state.AssignLiteral(NewSMTLiteral(bv, false), NewSMTBooleanAxiom()))
	variable, phase, err = queue.NextCaseSplit()
	astTestMust(t, err)
	if variable != av || phase != lbool.False {
		t.Fatal("assigned variable was not skipped", variable, phase, av)
	}
}

func TestSMTActivityCaseSplitQueueHonorsConfiguredAndCachedPhases(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.SetBinaryClauseOptimization(true))
	a := smtRelevancyBool(t, m, "case-split-configured-a")
	astTestMust(t, state.Internalize(a, true))
	variable, found := state.BoolVar(a)
	if !found {
		t.Fatal("missing configured-phase variable")
	}
	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	config := SMTParamsState{PhaseSelection: 1, PhaseCachingOn: 400, PhaseCachingOff: 100}
	astTestMust(t, queue.ConfigurePhases(config))
	selected, phase, err := queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != variable || phase != lbool.True {
		t.Fatal("always-true phase", selected, phase)
	}

	// A native activity selection consumes the entry; real unassignment
	// makes this variable available for the next independent phase check.
	astTestMust(t, state.SetAssignment(variable, lbool.True))
	astTestMust(t, state.SetAssignment(variable, lbool.Undef))
	config.PhaseSelection = 0
	astTestMust(t, queue.ConfigurePhases(config))
	selected, phase, err = queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != variable || phase != lbool.False {
		t.Fatal("always-false phase", selected, phase)
	}

	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(NewSMTLiteral(variable, false), NewSMTBooleanAxiom()))
	astTestMust(t, state.PopScopes(1))
	if !state.booleanData[variable].PhaseAvailable || !state.booleanData[variable].Phase {
		t.Fatal("assignment phase was not cached across rollback")
	}
	config.PhaseSelection = 2
	astTestMust(t, queue.ConfigurePhases(config))
	selected, phase, err = queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != variable || phase != lbool.True {
		t.Fatal("cached phase", selected, phase)
	}

	// A native activity selection consumes the entry; real unassignment
	// makes this variable available for the next independent phase check.
	astTestMust(t, state.SetAssignment(variable, lbool.True))
	astTestMust(t, state.SetAssignment(variable, lbool.Undef))
	config.PhaseSelection = 5
	config.RandomSeed = 0
	astTestMust(t, queue.ConfigurePhases(config))
	selected, phase, err = queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != variable || phase != lbool.False {
		t.Fatal("source-random phase", selected, phase)
	}

	b := smtRelevancyBool(t, m, "case-split-configured-b")
	astTestMust(t, state.Internalize(b, true))
	bVariable, found := state.BoolVar(b)
	if !found {
		t.Fatal("missing occurrence-phase peer")
	}
	state.activity[variable] = 2
	positive := NewSMTLiteral(variable, false)
	peer := NewSMTLiteral(bVariable, false)
	astTestMust(t, state.addClause([]SMTLiteral{positive, peer}, false, ast.ASTHandle{}))
	astTestMust(t, state.addClause([]SMTLiteral{positive, peer.Negated()}, false, ast.ASTHandle{}))
	// A native activity selection consumes the entry; real unassignment
	// makes this variable available for the next independent phase check.
	astTestMust(t, state.SetAssignment(variable, lbool.True))
	astTestMust(t, state.SetAssignment(variable, lbool.Undef))
	config.PhaseSelection = 6
	astTestMust(t, queue.ConfigurePhases(config))
	selected, phase, err = queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != variable || phase != lbool.True {
		t.Fatal("occurrence phase", selected, phase)
	}

	config.PhaseSelection = 7
	if err = queue.ConfigurePhases(config); err != ast.ErrASTDomain {
		t.Fatal("invalid phase strategy", err)
	}
}

func TestSMTActivityCaseSplitQueueForgetsConservativeConflictPhases(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	expression := smtRelevancyBool(t, m, "case-split-conservative")
	astTestMust(t, state.Internalize(expression, true))
	variable, _ := state.BoolVar(expression)
	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	config := SMTParamsState{PhaseSelection: 3, PhaseCachingOn: 400, PhaseCachingOff: 100}
	astTestMust(t, queue.ConfigurePhases(config))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(NewSMTLiteral(variable, false), NewSMTBooleanAxiom()))
	if !state.booleanData[variable].PhaseAvailable {
		t.Fatal("assignment did not cache its phase")
	}
	astTestMust(t, queue.PrepareConflict())
	if state.booleanData[variable].PhaseAvailable {
		t.Fatal("conservative conflict retained current-level phase")
	}

	state.booleanData[variable].PhaseAvailable = true
	config.PhaseSelection = 2
	astTestMust(t, queue.ConfigurePhases(config))
	astTestMust(t, queue.PrepareConflict())
	if !state.booleanData[variable].PhaseAvailable {
		t.Fatal("ordinary caching discarded current-level phase")
	}
}

func TestSMTActivityCaseSplitQueueHonorsRandomVariableFrequency(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	variables := make([]int32, 3)
	for index := range variables {
		expression := smtRelevancyBool(t, m, "random-variable-"+string(rune('a'+index)))
		astTestMust(t, state.Internalize(expression, true))
		variables[index], _ = state.BoolVar(expression)
	}
	state.activity[variables[0]] = 100
	params := SMTParamsState{InvDecay: 1.052, RandomInitialActivity: 0, RandomVarFreq: 1, PhaseSelection: 0}
	astTestMust(t, state.configureBooleanHeuristics(params))
	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	astTestMust(t, queue.ConfigurePhases(params))
	selected, _, err := queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != variables[2] {
		t.Fatal("random split did not override activity", selected, variables)
	}
}

func TestSMTActivityCaseSplitQueueDelaysSearchVariables(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	base := smtRelevancyBool(t, m, "delayed-variable-base")
	astTestMust(t, state.Internalize(base, true))
	baseVar, _ := state.BoolVar(base)
	params := SMTParamsState{InvDecay: 1.052, CaseSplitStrategy: 1, PhaseSelection: 0}
	astTestMust(t, state.configureBooleanHeuristics(params))
	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	astTestMust(t, queue.ConfigurePhases(params))
	astTestMust(t, state.beginSearchPersistence(0, params))
	defer func() { astTestMust(t, state.endSearchPersistence()) }()
	astTestMust(t, state.PushScope())
	search := smtRelevancyBool(t, m, "delayed-variable-search")
	astTestMust(t, state.Internalize(search, true))
	searchVar, _ := state.BoolVar(search)
	state.activity[searchVar] = 100

	selected, _, err := queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != baseVar {
		t.Fatal("search variable bypassed delayed pool", selected, baseVar)
	}
	astTestMust(t, state.AssignLiteral(NewSMTLiteral(baseVar, false), NewSMTBooleanAxiom()))
	selected, _, err = queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != searchVar {
		t.Fatal("delayed pool was not promoted", selected, searchVar)
	}

	astTestMust(t, state.PopScopes(1))
	astTestMust(t, state.PushScope())
	replacement := smtRelevancyBool(t, m, "delayed-variable-replacement")
	astTestMust(t, state.Internalize(replacement, true))
	replacementVar, _ := state.BoolVar(replacement)
	state.activity[replacementVar] = 200
	selected, _, err = queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != baseVar {
		t.Fatal("reused variable index bypassed delayed pool", selected, baseVar)
	}
	astTestMust(t, state.PopScopes(1))
}

func TestSMTActivityCaseSplitQueueCachesReinternalizedActivity(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	params := SMTParamsState{InvDecay: 1.052, CaseSplitStrategy: 2}
	astTestMust(t, state.configureBooleanHeuristics(params))
	expression := smtRelevancyBool(t, m, "cached-reinternalized-activity")
	astTestMust(t, state.beginSearchPersistence(state.scopeLevel, params))
	t.Cleanup(func() { astTestMust(t, state.endSearchPersistence()) })
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(expression, true))
	variable, _ := state.BoolVar(expression)
	state.activity[variable] = 17
	astTestMust(t, state.PopScopes(1))
	if _, err := m.GetKind(expression); err != nil {
		t.Fatal("activity cache did not retain expression", err)
	}
	astTestMust(t, state.Internalize(expression, true))
	variable, _ = state.BoolVar(expression)
	if state.activity[variable] != 17 {
		t.Fatal("reinternalized variable lost cached activity", state.activity[variable])
	}
}

func TestSMTActivityCaseSplitQueueCachesOnlyPositiveSearchActivity(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	params := SMTParamsState{InvDecay: 1.052, CaseSplitStrategy: 2}
	astTestMust(t, state.configureBooleanHeuristics(params))

	outside := smtRelevancyBool(t, m, "outside-search-cache")
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(outside, true))
	outsideVar, _ := state.BoolVar(outside)
	state.activity[outsideVar] = 23
	astTestMust(t, state.PopScopes(1))
	if len(state.activityCache) != 0 {
		t.Fatal("cached variable removed outside search")
	}

	astTestMust(t, state.beginSearchPersistence(state.scopeLevel, params))
	zero := smtRelevancyBool(t, m, "zero-search-cache")
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(zero, true))
	astTestMust(t, state.PopScopes(1))
	if len(state.activityCache) != 0 {
		t.Fatal("cached nonpositive search activity")
	}

	positive := smtRelevancyBool(t, m, "positive-search-cache")
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(positive, true))
	positiveVar, _ := state.BoolVar(positive)
	state.activity[positiveVar] = 9
	astTestMust(t, state.PopScopes(1))
	if len(state.activityCache) != 1 {
		t.Fatal("positive search activity was not cached")
	}
	astTestMust(t, state.endSearchPersistence())
	astTestMust(t, state.resetBooleanActivityCache())
	if len(state.activityCache) != 0 {
		t.Fatal("new search retained old activity cache")
	}
}

func TestSMTRelevantCaseSplitQueueFallbackAndFIFO(t *testing.T) {
	m := astTestManager(t)
	_, relevance, state := smtInternalizerFixture(t, m, nil)
	first := smtRelevancyBool(t, m, "relevant-fifo-first")
	second := smtRelevancyBool(t, m, "relevant-fifo-second")
	astTestMust(t, state.Internalize(first, true))
	astTestMust(t, state.Internalize(second, true))
	firstVar, _ := state.BoolVar(first)
	secondVar, _ := state.BoolVar(second)
	state.activity[secondVar] = 100
	astTestMust(t, relevance.MarkRelevant(first))

	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	params := SMTParamsState{CaseSplitStrategy: 3, RelevancyLvl: 2, AutoConfig: false, PhaseSelection: 0}
	astTestMust(t, queue.ConfigurePhases(params))
	selected, _, err := queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != firstVar {
		t.Fatal("relevance FIFO ignored relevant variable", selected, firstVar)
	}

	params.AutoConfig = true
	astTestMust(t, queue.ConfigurePhases(params))
	selected, _, err = queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != secondVar || queue.strategy != 0 {
		t.Fatal("automatic configuration did not fall back to activity", selected, queue.strategy)
	}
	params.AutoConfig = false
	params.RelevancyLvl = 1
	astTestMust(t, queue.ConfigurePhases(params))
	if queue.strategy != 0 {
		t.Fatal("low relevancy level did not fall back to activity", queue.strategy)
	}
}

func TestSMTRelevantCaseSplitQueueSelectsConnectiveChild(t *testing.T) {
	for _, order := range []uint32{0, 2} {
		t.Run(string(rune('0'+order)), func(t *testing.T) {
			m := astTestManager(t)
			_, relevance, state := smtInternalizerFixture(t, m, nil)
			first := smtRelevancyBool(t, m, "relevant-child-first")
			second := smtRelevancyBool(t, m, "relevant-child-second")
			parent, err := m.MkOr(first, second)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(parent, true))
			parentVar, _ := state.BoolVar(parent)
			firstVar, _ := state.BoolVar(first)
			secondVar, _ := state.BoolVar(second)
			astTestMust(t, state.SetAssignment(parentVar, lbool.True))
			astTestMust(t, relevance.MarkRelevant(parent))
			queue, err := NewSMTActivityCaseSplitQueue(state)
			astTestMust(t, err)
			params := SMTParamsState{CaseSplitStrategy: 3, RelevancyLvl: 2, AutoConfig: false, RelCaseSplitOrder: order}
			astTestMust(t, queue.ConfigurePhases(params))
			selected, phase, err := queue.NextCaseSplit()
			astTestMust(t, err)
			want := firstVar
			if order == 2 {
				want = secondVar
			}
			if selected != want || phase != lbool.True {
				t.Fatal("connective child order or phase", selected, phase, want)
			}
		})
	}
	t.Run("later-satisfying-child", func(t *testing.T) {
		m := astTestManager(t)
		_, relevance, state := smtInternalizerFixture(t, m, nil)
		first := smtRelevancyBool(t, m, "relevant-child-undef")
		second := smtRelevancyBool(t, m, "relevant-child-true")
		parent, err := m.MkOr(first, second)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(parent, true))
		parentVar, _ := state.BoolVar(parent)
		secondVar, _ := state.BoolVar(second)
		astTestMust(t, state.SetAssignment(parentVar, lbool.True))
		astTestMust(t, state.SetAssignment(secondVar, lbool.True))
		astTestMust(t, relevance.MarkRelevant(parent))
		queue, err := NewSMTActivityCaseSplitQueue(state)
		astTestMust(t, err)
		params := SMTParamsState{CaseSplitStrategy: 3, RelevancyLvl: 2, AutoConfig: false, RelCaseSplitOrder: 0}
		astTestMust(t, queue.ConfigurePhases(params))
		selected, _, err := queue.NextCaseSplit()
		astTestMust(t, err)
		if selected != SMTNullBoolVar {
			t.Fatal("OR with a true child produced a redundant split", selected)
		}
	})
}

func TestSMTRelevantCaseSplitQueueLooksAheadAtDisequality(t *testing.T) {
	m := astTestManager(t)
	_, values := smtUFConstants(t, m, "relevant-lookahead-", 2)
	equality, err := m.MkEq(values[0], values[1])
	astTestMust(t, err)
	context, relevance, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.Internalize(equality, false))
	variable, found := state.BoolVar(equality)
	if !found {
		t.Fatal("equality was not Boolean-internalized")
	}
	left, right := context.enodes.get(values[0]), context.enodes.get(values[1])
	if left == nil || right == nil {
		t.Fatal("equality arguments were not e-internalized")
	}
	astTestMust(t, context.SetDisequal(left, right, true))
	astTestMust(t, relevance.MarkRelevant(equality))
	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	params := SMTParamsState{CaseSplitStrategy: 3, RelevancyLvl: 2, AutoConfig: false, PhaseSelection: 1, LookaheadDiseq: true}
	astTestMust(t, queue.ConfigurePhases(params))
	selected, phase, err := queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != variable || phase != lbool.False {
		t.Fatal("known disequality did not force the equality false", selected, phase, variable)
	}
}

func TestSMTRelevantActivityCaseSplitDelaysSearchVariables(t *testing.T) {
	m := astTestManager(t)
	_, relevance, state := smtInternalizerFixture(t, m, nil)
	base := smtRelevancyBool(t, m, "relevant-activity-base")
	astTestMust(t, state.Internalize(base, true))
	baseVar, _ := state.BoolVar(base)
	params := SMTParamsState{CaseSplitStrategy: 4, RelevancyLvl: 2, AutoConfig: false, PhaseSelection: 0}
	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	astTestMust(t, queue.ConfigurePhases(params))
	astTestMust(t, state.beginSearchPersistence(state.scopeLevel, params))
	t.Cleanup(func() { astTestMust(t, state.endSearchPersistence()) })
	astTestMust(t, relevance.MarkRelevant(base))
	search := smtRelevancyBool(t, m, "relevant-activity-search")
	astTestMust(t, state.Internalize(search, true))
	searchVar, _ := state.BoolVar(search)
	state.activity[searchVar] = 100

	selected, _, err := queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != baseVar {
		t.Fatal("search activity bypassed relevant base variable", selected, baseVar)
	}
	astTestMust(t, state.SetAssignment(baseVar, lbool.False))
	selected, _, err = queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != searchVar {
		t.Fatal("search activity was not selected after base relevance", selected, searchVar)
	}
}

func TestSMTRelevantGoalCaseSplitUsesLowestGeneration(t *testing.T) {
	m := astTestManager(t)
	_, relevance, state := smtInternalizerFixture(t, m, nil)
	params := SMTParamsState{CaseSplitStrategy: 5, RelevancyLvl: 2, AutoConfig: false, PhaseSelection: 0}
	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	astTestMust(t, queue.ConfigurePhases(params))
	astTestMust(t, state.beginSearchPersistence(state.scopeLevel, params))
	t.Cleanup(func() { astTestMust(t, state.endSearchPersistence()) })

	astTestMust(t, state.SetGeneration(9))
	high := smtRelevancyBool(t, m, "relevant-goal-high")
	astTestMust(t, state.Internalize(high, true))
	_, err = state.MkEnode(high, false, false, true)
	astTestMust(t, err)
	highVar, _ := state.BoolVar(high)
	astTestMust(t, relevance.MarkRelevant(high))
	astTestMust(t, state.SetGeneration(2))
	low := smtRelevancyBool(t, m, "relevant-goal-low")
	astTestMust(t, state.Internalize(low, true))
	_, err = state.MkEnode(low, false, false, true)
	astTestMust(t, err)
	lowVar, _ := state.BoolVar(low)
	astTestMust(t, relevance.MarkRelevant(low))

	if state.context.enodes.get(high) == nil || state.context.enodes.get(low) == nil {
		t.Fatal("generation candidates were not e-internalized")
	}
	selected, _, err := queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != lowVar || selected == highVar {
		t.Fatal("goal relevance ignored lower generation", selected, lowVar, highVar)
	}
}

func TestSMTTheoryAwareCaseSplitWithoutHintProducerUsesActivity(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	first := smtRelevancyBool(t, m, "theory-aware-first")
	second := smtRelevancyBool(t, m, "theory-aware-second")
	astTestMust(t, state.Internalize(first, true))
	astTestMust(t, state.Internalize(second, true))
	firstVar, _ := state.BoolVar(first)
	secondVar, _ := state.BoolVar(second)
	state.activity[firstVar] = 1
	state.activity[secondVar] = 8
	queue, err := NewSMTActivityCaseSplitQueue(state)
	astTestMust(t, err)
	params := SMTParamsState{CaseSplitStrategy: 6, PhaseSelection: 1}
	astTestMust(t, queue.ConfigurePhases(params))
	selected, phase, err := queue.NextCaseSplit()
	astTestMust(t, err)
	if selected != secondVar || phase != lbool.True {
		t.Fatal("theory-aware queue without retained hints diverged from activity", selected, phase, secondVar)
	}
}
