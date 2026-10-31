package api

import (
	"errors"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/control"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type apiSourceTactic struct {
	tactic.Tactic
	params  *uparams.ParamsContext
	manager *ast.ASTManager
	events  []string
	timeout uint32
	body    func(*tactic.Goal, *tactic.TacticResult) error
}

func (t *apiSourceTactic) UpdateParams(p *uparams.ParamsRef) error {
	t.events = append(t.events, "update")
	key := "timeout"
	var err error
	t.timeout, err = t.params.GetUIntName(p, &key, math.MaxUint32)
	return err
}
func (t *apiSourceTactic) ResetStatistics() error {
	t.events = append(t.events, "reset")
	return nil
}
func (t *apiSourceTactic) Apply(g *tactic.Goal, r *tactic.TacticResult) error {
	t.events = append(t.events, "apply")
	if err := r.Prepare(t.manager); err != nil {
		return err
	}
	return t.body(g, r)
}
func (t *apiSourceTactic) Cleanup() error {
	t.events = append(t.events, "cleanup")
	return nil
}
func (t *apiSourceTactic) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := d.InsertName("timeout", uparams.ParamUInt, nil, nil, nil); err != nil {
		return err
	}
	return d.InsertName("ctrl_c", uparams.ParamBool, nil, nil, nil)
}

func apiSourceTacticFixture(t *testing.T, f *smtAPIFixture) (*apiSourceTactic, *APITactic, *APIGoal) {
	t.Helper()
	globals, err := f.c.command.GlobalParameters()
	astTestMust(t, err)
	recorder := &apiSourceTactic{Tactic: tactic.NewSkipTactic(), params: globals.Context, manager: f.m}
	value, err := newAPITactic(f.c.smtContextState, recorder)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, value.Close()) })
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	return recorder, value, goal
}

func TestAPITacticSourceExecutionLifecycleAndInterruptScope(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	recorder, value, goal := apiSourceTacticFixture(t, f)
	for _, fail := range []bool{false, true} {
		var executed *tactic.Goal
		recorder.events = nil
		recorder.body = func(g *tactic.Goal, r *tactic.TacticResult) error {
			executed = g
			if f.c.interruptable == nil {
				t.Fatal("API apply did not install context interrupt handler")
			}
			if fail {
				return tactic.NewTacticException("source failure")
			}
			return r.AddGoal(g)
		}
		result, err := value.Apply(goal)
		if fail {
			if result != nil || err == nil {
				t.Fatal("failed apply result", result, err)
			}
		} else {
			astTestMust(t, err)
			if result.goals[0] != executed {
				t.Fatal("API result copied rather than shared the source output goal")
			}
			astTestMust(t, result.Close())
		}
		if !reflect.DeepEqual(recorder.events, []string{"update", "reset", "apply", "cleanup"}) {
			t.Fatal("source execution prefix/cleanup", recorder.events)
		}
		if recorder.timeout != math.MaxUint32 || f.c.interruptable != nil {
			t.Fatal("default timeout or released interrupt scope", recorder.timeout)
		}
	}
}

func TestAPITacticSourceParameterTimeoutAndDefaultReset(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	recorder, value, goal := apiSourceTacticFixture(t, f)
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	defer func() { astTestMust(t, parameters.Close()) }()
	astTestMust(t, parameters.SetUInt("timeout", 1))
	recorder.body = func(g *tactic.Goal, r *tactic.TacticResult) error {
		deadline := time.Now().Add(time.Second)
		for !f.m.ResourceLimit().CancelFlag() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if !f.m.ResourceLimit().CancelFlag() {
			t.Fatal("local tactic timeout never canceled the AST limit")
		}
		return r.AddGoal(g)
	}
	result, err := value.ApplyWithParameters(goal, parameters)
	astTestMust(t, err)
	astTestMust(t, result.Close())
	if recorder.timeout != 1 {
		t.Fatal("apply_ex did not pass the local parameter set", recorder.timeout)
	}
	f.m.ResourceLimit().ResetCancel()
	astTestMust(t, f.c.SetUIntOption("timeout", 1))
	recorder.body = func(g *tactic.Goal, r *tactic.TacticResult) error {
		time.Sleep(5 * time.Millisecond)
		if f.m.ResourceLimit().CancelFlag() {
			t.Fatal("plain apply used context timeout instead of local UINT_MAX default")
		}
		return r.AddGoal(g)
	}
	result, err = value.Apply(goal)
	astTestMust(t, err)
	astTestMust(t, result.Close())
	if recorder.timeout != math.MaxUint32 {
		t.Fatal("plain apply did not update the tactic with empty parameters", recorder.timeout)
	}
}

func TestAPITacticNestedScopeRefusalStillUpdatesParameters(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	recorder, value, goal := apiSourceTacticFixture(t, f)
	recorder.body = func(*tactic.Goal, *tactic.TacticResult) error {
		t.Fatal("tactic executed after interrupt scope refusal")
		return nil
	}
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	astTestMust(t, err)
	result, err := value.Apply(goal)
	astTestMust(t, scope.Close())
	if result != nil || !errors.Is(err, errSMTContextInterruptScope) || !reflect.DeepEqual(recorder.events, []string{"update"}) {
		t.Fatal("source update-before-scope prefix", result, err, recorder.events)
	}
}

func TestAPITacticSourceSharedSubgoalAndRetainedResult(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	skip, err := f.c.SkipTactic()
	astTestMust(t, err)
	defer func() { astTestMust(t, skip.Close()) }()
	result, err := skip.Apply(goal)
	astTestMust(t, err)
	retained, err := result.Retain()
	astTestMust(t, err)
	first, err := result.Subgoal(0)
	astTestMust(t, err)
	second, err := retained.Subgoal(0)
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	atom := f.pin(f.c.Constant("shared-subgoal", boolean))
	astTestMust(t, first.Assert(atom))
	if size, err := second.Size(); err != nil || size != 1 {
		t.Fatal("source subgoal wrappers did not share mutations", size, err)
	}
	if size, err := goal.Size(); err != nil || size != 0 {
		t.Fatal("apply mutated its original input goal", size, err)
	}
	text, err := result.String()
	astTestMust(t, err)
	if text != "(goals\n(goal\n  shared-subgoal)\n)" {
		t.Fatal("source result serialization", text)
	}
	astTestMust(t, result.Close())
	astTestMust(t, first.Close())
	astTestMust(t, retained.Close())
	if size, err := second.Size(); err != nil || size != 1 {
		t.Fatal("last subgoal owner did not outlive both result handles", size, err)
	}
	astTestMust(t, second.Close())
}

func TestAPITacticResultOwnsProofConverterAndCore(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", func(c *SMTContext) { astTestMust(t, c.SetBoolOption("produce-proofs", true)) })
	recorder, value, goal := apiSourceTacticFixture(t, f)
	boolean := f.pin(f.c.BoolSort())
	label := f.pin(f.c.Constant("result-core-label", boolean))
	proof := f.pin(f.m.MkProofAsserted(label))
	before, err := f.m.GetRefCount(label)
	astTestMust(t, err)
	recorder.body = func(g *tactic.Goal, r *tactic.TacticResult) error {
		if err := r.AddGoal(g); err != nil {
			return err
		}
		converter, err := tactic.NewStoredProofConverter(f.m, proof)
		if err != nil {
			return err
		}
		if err = r.SetProofConverter(converter); err != nil {
			return err
		}
		core, err := f.m.DependencyLeaf(label)
		if err != nil {
			return err
		}
		return r.SetCore(core)
	}
	result, err := value.Apply(goal)
	astTestMust(t, err)
	retained, err := result.Retain()
	astTestMust(t, err)
	astTestMust(t, result.Close())
	if retained.proofConverter == nil || retained.core.Get().IsNull() {
		t.Fatal("apply result dropped source proof/core owners")
	}
	out, err := f.m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	astTestMust(t, retained.proofConverter.Convert(f.m, nil, out))
	actual, err := f.m.RefGet(out)
	astTestMust(t, err)
	if actual != proof {
		t.Fatal("proof converter lost its stored proof", actual)
	}
	astTestMust(t, f.m.DelRef(&out))
	if contains, err := f.m.DependencyContains(retained.core.Get(), label); err != nil || !contains {
		t.Fatal("result core lifetime", contains, err)
	}
	astTestMust(t, retained.Close())
	after, err := f.m.GetRefCount(label)
	astTestMust(t, err)
	if after != before {
		t.Fatal("result core reference leak", before, after)
	}
}

func TestAPITacticParallelSourceFallbackAndInputOwnership(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	boolean := f.pin(f.c.BoolSort())
	atom := f.pin(f.c.Constant("parallel-undecided", boolean))
	astTestMust(t, goal.Assert(atom))
	fail, err := f.c.FailTactic()
	astTestMust(t, err)
	skip, err := f.c.SkipTactic()
	astTestMust(t, err)
	choice, err := f.c.ParallelOr(fail, skip)
	astTestMust(t, err)
	sequence, err := skip.ParallelAndThen(fail)
	astTestMust(t, err)
	astTestMust(t, fail.Close())
	astTestMust(t, skip.Close())
	result, err := choice.Apply(goal)
	astTestMust(t, err)
	if count, err := result.NumSubgoals(); err != nil || count != 1 {
		t.Fatal("ordered parallel fallback", count, err)
	}
	astTestMust(t, result.Close())
	if result, err := sequence.Apply(goal); err == nil || result != nil {
		t.Fatal("parallel sequence did not propagate second tactic failure", result, err)
	}
	astTestMust(t, sequence.Close())
	astTestMust(t, choice.Close())
	if _, err = f.c.ParallelOr(); err == nil {
		t.Fatal("empty parallel alternative accepted")
	}
	foreign := newSMTAPIFixture(t, "ALL", nil)
	child, err := foreign.c.SkipTactic()
	astTestMust(t, err)
	defer func() { astTestMust(t, child.Close()) }()
	if _, err = f.c.ParallelOr(child); err == nil {
		t.Fatal("parallel alternative accepted foreign tactic")
	}
}

func TestAPITacticAndProbeCStringNamesAndIndependentHandles(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	value, err := f.c.Tactic("skip\x00ignored")
	astTestMust(t, err)
	description, err := f.c.TacticDescription("skip\x00ignored")
	astTestMust(t, err)
	want, err := f.c.TacticDescription("skip")
	astTestMust(t, err)
	if description != want {
		t.Fatal("source tactic C-string lookup", description, want)
	}
	probe, err := f.c.NamedProbe("size\x00ignored")
	astTestMust(t, err)
	description, err = f.c.ProbeDescription("size\x00ignored")
	astTestMust(t, err)
	want, err = f.c.ProbeDescription("size")
	astTestMust(t, err)
	if description != want {
		t.Fatal("source probe C-string lookup", description, want)
	}
	keptTactic, err := value.Retain()
	astTestMust(t, err)
	keptProbe, err := probe.Retain()
	astTestMust(t, err)
	astTestMust(t, value.Close())
	astTestMust(t, probe.Close())
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	result, err := keptTactic.Apply(goal)
	astTestMust(t, err)
	astTestMust(t, result.Close())
	if size, err := keptProbe.Apply(goal); err != nil || size != 0 {
		t.Fatal("probe handle did not outlive its original owner", size, err)
	}
	astTestMust(t, keptTactic.Close())
	astTestMust(t, keptProbe.Close())
}

func TestAPITacticSourceCopiedOwnersCannotReleaseSharedState(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	value, err := f.c.SkipTactic()
	astTestMust(t, err)
	defer func() { astTestMust(t, value.Close()) }()
	probe, err := f.c.ConstantProbe(1)
	astTestMust(t, err)
	defer func() { astTestMust(t, probe.Close()) }()
	result, err := value.Apply(goal)
	astTestMust(t, err)
	defer func() { astTestMust(t, result.Close()) }()
	subgoal, err := result.Subgoal(0)
	astTestMust(t, err)
	defer func() { astTestMust(t, subgoal.Close()) }()
	copiedTactic, copiedProbe, copiedResult, copiedGoal := *value, *probe, *result, *subgoal
	for _, close := range []func() error{copiedTactic.Close, copiedProbe.Close, copiedResult.Close, copiedGoal.Close} {
		if err := close(); !errors.Is(err, ErrSMTContextOwner) {
			t.Fatal("copied owner released live shared state", err)
		}
	}
	final, err := value.Apply(subgoal)
	astTestMust(t, err)
	astTestMust(t, final.Close())
}

func TestAPITacticConvertModelSourceEqualSizeBoundaryWithoutConverter(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	skip, err := f.c.SkipTactic()
	astTestMust(t, err)
	defer func() { astTestMust(t, skip.Close()) }()
	result, err := skip.Apply(goal)
	astTestMust(t, err)
	defer func() { astTestMust(t, result.Close()) }()
	source, err := f.c.NewAPIModel()
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	size, err := result.NumSubgoals()
	astTestMust(t, err)
	converted, err := result.ConvertModel(size, source)
	astTestMust(t, err)
	if converted.core == source.core {
		t.Fatal("source boundary did not copy the model")
	}
	astTestMust(t, converted.Close())
	if model, err := result.ConvertModel(size+1, source); model != nil || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("source strict upper bound", model, err)
	}
}

func TestAPITacticSourceControlCOptionAndHandlerRestoration(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	recorder, value, goal := apiSourceTacticFixture(t, f)
	outerCalls := make(chan control.EventCaller, 2)
	outer, err := control.NewScopedCtrlC(control.EventHandlerFunc(func(c control.EventCaller) { outerCalls <- c }), false, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, outer.Close()) }()
	process, err := os.FindProcess(os.Getpid())
	astTestMust(t, err)
	defer process.Release()
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	defer func() { astTestMust(t, parameters.Close()) }()
	for _, enabled := range []bool{false, true} {
		astTestMust(t, parameters.SetBool("ctrl_c", enabled))
		recorder.body = func(g *tactic.Goal, r *tactic.TacticResult) error {
			if err := process.Signal(os.Interrupt); err != nil {
				return err
			}
			if enabled {
				deadline := time.Now().Add(time.Second)
				for !f.m.ResourceLimit().CancelFlag() && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if !f.m.ResourceLimit().CancelFlag() {
					t.Fatal("enabled tactic ctrl_c handler did not cancel the AST limit")
				}
			} else {
				select {
				case caller := <-outerCalls:
					if caller != control.EventCallerControlC || f.m.ResourceLimit().CancelFlag() {
						t.Fatal("disabled ctrl_c replaced its outer handler", caller)
					}
				case <-time.After(time.Second):
					t.Fatal("disabled tactic ctrl_c did not preserve outer handler")
				}
			}
			return r.AddGoal(g)
		}
		result, err := value.ApplyWithParameters(goal, parameters)
		astTestMust(t, err)
		astTestMust(t, result.Close())
		f.m.ResourceLimit().ResetCancel()
	}
	astTestMust(t, process.Signal(os.Interrupt))
	select {
	case caller := <-outerCalls:
		if caller != control.EventCallerControlC || f.m.ResourceLimit().CancelFlag() {
			t.Fatal("tactic ctrl_c scope did not restore the outer handler", caller)
		}
	case <-time.After(time.Second):
		t.Fatal("outer handler was not restored after tactic apply")
	}
}
