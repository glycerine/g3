package fp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExternalIntervalRecursiveProgramSeedsDeltaRoundsAndWidening(t *testing.T) {
	for _, mode := range []struct {
		name         string
		widen        bool
		allOrNothing bool
		wantBound    string
	}{
		{name: "ordinary", wantBound: "1 in [0, 2]"},
		{name: "widening", widen: true, wantBound: "1 in [0, oo)"},
		{name: "all-or-nothing-widening", widen: true, allOrNothing: true, wantBound: "1 in [0, oo)"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			// The interval component is first so its exact delta emptiness
			// controls the fixpoint rounds independently of the mock external
			// relation's intentionally coarse empty/nonempty model.
			kinds := []string{"interval_relation", "external_relation"}
			p := declareExternalIntervalPredicate(t, manager, engine, "recursive-interval-P-"+mode.name, []ast.ASTHandle{integer, integer}, kinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "recursive-interval-Q-"+mode.name, []ast.ASTHandle{integer}, kinds)
			s := declareExternalIntervalPredicate(t, manager, engine, "recursive-interval-S-"+mode.name, []ast.ASTHandle{integer}, kinds)
			r := declareExternalIntervalPredicate(t, manager, engine, "recursive-interval-R-"+mode.name, []ast.ASTHandle{integer}, kinds)
			numeral := func(value string) ast.ASTHandle {
				h, cause := manager.ArithmeticIntegerNumeralText(value)
				fragmentMust(t, cause)
				return h
			}
			fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{numeral("0")}))
			fragmentMust(t, engine.AddFact(s, []ast.ASTHandle{numeral("0")}))
			fragmentMust(t, engine.AddFact(r, []ast.ASTHandle{numeral("1")}))
			fragmentMust(t, engine.AddFact(r, []ast.ASTHandle{numeral("2")}))

			x, err := manager.MkVar(0, integer)
			fragmentMust(t, err)
			y, err := manager.MkVar(1, integer)
			fragmentMust(t, err)
			z, err := manager.MkVar(2, integer)
			fragmentMust(t, err)
			app := func(declaration ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
				h, cause := manager.MkApp(declaration, args)
				fragmentMust(t, cause)
				return h
			}
			seedBody, err := manager.MkAnd(app(q, x), app(s, y))
			fragmentMust(t, err)
			seed, err := manager.MkImplies(seedBody, app(p, x, y))
			fragmentMust(t, err)
			fragmentMust(t, engine.AddRule(seed, "external-interval-recursive-seed-"+mode.name))
			loopBody, err := manager.MkAnd(app(p, x, y), app(r, z))
			fragmentMust(t, err)
			loop, err := manager.MkImplies(loopBody, app(p, x, z))
			fragmentMust(t, err)
			fragmentMust(t, engine.AddRule(loop, "external-interval-recursive-step-"+mode.name))
			fragmentMust(t, engine.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", modeValue(mode.widen)))
			fragmentMust(t, engine.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", modeValue(mode.allOrNothing)))
			resetExternalIntervalCartesianCallbacks(callbacks)

			status, err := engine.QueryPredicate(p)
			if err != nil || status != lbool.Undef {
				t.Fatalf("recursive interval query status=%v err=%v", status, err)
			}
			updates := operationCount(callbacks.operations, ast.DatalogOpUnion) + operationCount(callbacks.operations, ast.DatalogOpWiden)
			if operationCount(callbacks.operations, ast.DatalogOpJoin) < 2 || updates < 3 {
				t.Fatalf("seed evaluation and delta rounds did not execute: %v", callbacks.operations)
			}
			seedJoin, seedUnion, deltaClone, loopJoin := -1, -1, -1, -1
			for index, operation := range callbacks.operations {
				switch {
				case seedJoin < 0 && operation == ast.DatalogOpJoin:
					seedJoin = index
				case seedJoin >= 0 && seedUnion < 0 && (operation == ast.DatalogOpUnion || operation == ast.DatalogOpWiden):
					seedUnion = index
				case seedUnion >= 0 && deltaClone < 0 && operation == ast.DatalogOpClone:
					deltaClone = index
				case deltaClone >= 0 && operation == ast.DatalogOpJoin:
					loopJoin = index
				}
			}
			if seedJoin < 0 || seedUnion < seedJoin || deltaClone < seedUnion || loopJoin < deltaClone {
				t.Fatalf("seed rule did not finish before recursive deltas were cloned and consumed: %v", callbacks.operations)
			}
			var display bytes.Buffer
			fragmentMust(t, engine.externalIntervalDatalog.entries[p].product.Display(&display))
			if !strings.Contains(display.String(), mode.wantBound) {
				t.Fatalf("recursive result bounds %q; want %q", display.String(), mode.wantBound)
			}
		})
	}
}

func TestExternalIntervalRecursiveProgramConsumesDeltasAcrossRounds(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"interval_relation", "external_relation"}
	base := declareExternalIntervalPredicate(t, manager, engine, "recursive-rounds-base", []ast.ASTHandle{integer}, kinds)
	p := declareExternalIntervalPredicate(t, manager, engine, "recursive-rounds-P", []ast.ASTHandle{integer}, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "recursive-rounds-Q", []ast.ASTHandle{integer}, kinds)
	r := declareExternalIntervalPredicate(t, manager, engine, "recursive-rounds-R", []ast.ASTHandle{integer}, kinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(base, []ast.ASTHandle{one}))
	addExternalIntervalIdentityRule(t, manager, engine, base, p, []ast.ASTHandle{integer}, "recursive-rounds-seed")
	addExternalIntervalIdentityRule(t, manager, engine, p, q, []ast.ASTHandle{integer}, "recursive-rounds-P-to-Q")
	addExternalIntervalIdentityRule(t, manager, engine, q, r, []ast.ASTHandle{integer}, "recursive-rounds-Q-to-R")
	addExternalIntervalIdentityRule(t, manager, engine, r, p, []ast.ASTHandle{integer}, "recursive-rounds-R-to-P")
	resetExternalIntervalCartesianCallbacks(callbacks)

	status, err := engine.QueryPredicate(r)
	if err != nil || status != lbool.Undef {
		var displays []string
		if engine.externalIntervalDatalog != nil {
			for _, declaration := range []ast.ASTHandle{p, q, r} {
				var display bytes.Buffer
				_ = engine.externalIntervalDatalog.entries[declaration].product.Display(&display)
				displays = append(displays, display.String())
			}
		}
		t.Fatalf("recursive round query status=%v err=%v operations=%v displays=%v strata=%+v", status, err, callbacks.operations, displays, engine.externalIntervalDatalog.program.strata)
	}
	if unions := operationCount(callbacks.operations, ast.DatalogOpUnion); unions < 8 {
		t.Fatalf("three-predicate cycle did not propagate through multiple delta rounds: %d unions, operations %v", unions, callbacks.operations)
	}
	for _, declaration := range []ast.ASTHandle{p, q, r} {
		var display bytes.Buffer
		fragmentMust(t, engine.externalIntervalDatalog.entries[declaration].product.Display(&display))
		if !strings.Contains(display.String(), "0 in [1, 1]") {
			t.Fatalf("recursive delta did not reach predicate %v: %s", declaration, display.String())
		}
	}
}

func TestExternalIntervalRecursiveProgramEvaluatesSeedsWithoutRecursiveRounds(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"interval_relation", "external_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "seed-only-P", []ast.ASTHandle{integer, integer}, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "seed-only-Q", []ast.ASTHandle{integer}, kinds)
	s := declareExternalIntervalPredicate(t, manager, engine, "seed-only-S", []ast.ASTHandle{integer}, kinds)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{zero}))
	fragmentMust(t, engine.AddFact(s, []ast.ASTHandle{two}))
	x, err := manager.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := manager.MkVar(1, integer)
	fragmentMust(t, err)
	qApp, err := manager.MkApp(q, []ast.ASTHandle{x})
	fragmentMust(t, err)
	sApp, err := manager.MkApp(s, []ast.ASTHandle{y})
	fragmentMust(t, err)
	body, err := manager.MkAnd(qApp, sApp)
	fragmentMust(t, err)
	pHead, err := manager.MkApp(p, []ast.ASTHandle{x, y})
	fragmentMust(t, err)
	seed, err := manager.MkImplies(body, pHead)
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(seed, "seed-only-product"))
	resetExternalIntervalCartesianCallbacks(callbacks)

	status, err := engine.QueryPredicate(p)
	if err != nil || status != lbool.Undef {
		t.Fatalf("seed-only query status=%v err=%v", status, err)
	}
	if operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 ||
		operationCount(callbacks.operations, ast.DatalogOpUnion) == 0 {
		t.Fatalf("nonrecursive seed execution did not run exactly its seed rule: %v", callbacks.operations)
	}
	var display bytes.Buffer
	fragmentMust(t, engine.externalIntervalDatalog.entries[p].product.Display(&display))
	for _, want := range []string{"0 in [0, 0]", "1 in [2, 2]"} {
		if !strings.Contains(display.String(), want) {
			t.Fatalf("seed relation %q missing %q", display.String(), want)
		}
	}
}

func TestExternalIntervalRecursiveProgramOptionalWidening(t *testing.T) {
	for _, test := range []struct {
		name      string
		widen     bool
		wantBound string
	}{
		{name: "disabled", wantBound: "1 in [0, 2]"},
		{name: "enabled", widen: true, wantBound: "1 in [0, oo)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			kinds := []string{"interval_relation", "external_relation"}
			p := declareExternalIntervalPredicate(t, manager, engine, "widening-P-"+test.name, []ast.ASTHandle{integer, integer}, kinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "widening-Q-"+test.name, []ast.ASTHandle{integer}, kinds)
			s := declareExternalIntervalPredicate(t, manager, engine, "widening-S-"+test.name, []ast.ASTHandle{integer}, kinds)
			r := declareExternalIntervalPredicate(t, manager, engine, "widening-R-"+test.name, []ast.ASTHandle{integer}, kinds)
			numeral := func(value string) ast.ASTHandle {
				h, cause := manager.ArithmeticIntegerNumeralText(value)
				fragmentMust(t, cause)
				return h
			}
			fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{numeral("0")}))
			fragmentMust(t, engine.AddFact(s, []ast.ASTHandle{numeral("0")}))
			fragmentMust(t, engine.AddFact(r, []ast.ASTHandle{numeral("1")}))
			fragmentMust(t, engine.AddFact(r, []ast.ASTHandle{numeral("2")}))
			x, err := manager.MkVar(0, integer)
			fragmentMust(t, err)
			y, err := manager.MkVar(1, integer)
			fragmentMust(t, err)
			z, err := manager.MkVar(2, integer)
			fragmentMust(t, err)
			app := func(declaration ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
				h, cause := manager.MkApp(declaration, args)
				fragmentMust(t, cause)
				return h
			}
			seedBody, err := manager.MkAnd(app(q, x), app(s, y))
			fragmentMust(t, err)
			seed, err := manager.MkImplies(seedBody, app(p, x, y))
			fragmentMust(t, err)
			fragmentMust(t, engine.AddRule(seed, "widening-seed-"+test.name))
			loopBody, err := manager.MkAnd(app(p, x, y), app(r, z))
			fragmentMust(t, err)
			loop, err := manager.MkImplies(loopBody, app(p, x, z))
			fragmentMust(t, err)
			fragmentMust(t, engine.AddRule(loop, "widening-loop-"+test.name))
			fragmentMust(t, engine.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", modeValue(test.widen)))
			resetExternalIntervalCartesianCallbacks(callbacks)

			status, err := engine.QueryPredicate(p)
			if err != nil || status != lbool.Undef {
				t.Fatalf("widening query status=%v err=%v", status, err)
			}
			var display bytes.Buffer
			fragmentMust(t, engine.externalIntervalDatalog.entries[p].product.Display(&display))
			if !strings.Contains(display.String(), test.wantBound) {
				t.Fatalf("widen=%t produced %q; want bound %q", test.widen, display.String(), test.wantBound)
			}
			widenCalls := operationCount(callbacks.operations, ast.DatalogOpWiden)
			if test.widen && widenCalls == 0 {
				t.Fatalf("widening enabled but relation WIDEN was not invoked: %v", callbacks.operations)
			}
			if !test.widen && widenCalls != 0 {
				t.Fatalf("widening disabled but relation WIDEN was invoked: %v", callbacks.operations)
			}
		})
	}
}

// The planner is a subcomponent of recursive evaluation: source-only rules
// must run once as seeds, while each same-SCC tail must become its own delta
// action. Check this schedule directly, independently of callback execution.
func TestExternalIntervalProgramPlanSeparatesSeedsAndDeltaActions(t *testing.T) {
	manager, engine, _ := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"interval_relation", "external_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "plan-seed-delta-P", []ast.ASTHandle{integer}, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "plan-seed-delta-Q", []ast.ASTHandle{integer}, kinds)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	seed, err := manager.MkApp(p, []ast.ASTHandle{zero})
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(seed, "plan-ground-seed"))
	addExternalIntervalIdentityRule(t, manager, engine, p, q, []ast.ASTHandle{integer}, "plan-P-to-Q")
	addExternalIntervalIdentityRule(t, manager, engine, q, p, []ast.ASTHandle{integer}, "plan-Q-to-P")

	program, err := engine.externalIntervalProgramPlan()
	fragmentMust(t, err)
	t.Cleanup(func() {
		for index := len(program.rules) - 1; index >= 0; index-- {
			fragmentMust(t, program.rules[index].identity.close(manager))
			fragmentMust(t, program.rules[index].cartesian.close(manager))
			fragmentMust(t, program.rules[index].ground.close(manager))
		}
	})
	if len(program.strata) != 1 {
		t.Fatalf("mutually recursive predicates planned into %d strata, want 1", len(program.strata))
	}
	stage := program.strata[0]
	if !stage.cyclic || len(stage.heads) != 2 {
		t.Fatalf("recursive component classification: cyclic=%t heads=%v", stage.cyclic, stage.heads)
	}
	if len(stage.seeds) != 1 || stage.seeds[0] != 0 {
		t.Fatalf("seed schedule %v, want only the ground fact rule [0]", stage.seeds)
	}
	if len(stage.loop) != 2 {
		t.Fatalf("recursive delta actions %v, want one action per recursive tail", stage.loop)
	}
	seen := make(map[[2]int]bool, len(stage.loop))
	for _, action := range stage.loop {
		if action.rule < 1 || action.rule > 2 || action.inputTail != 0 {
			t.Fatalf("invalid recursive delta action %+v", action)
		}
		key := [2]int{action.rule, action.inputTail}
		if seen[key] {
			t.Fatalf("duplicate recursive delta action %+v", action)
		}
		seen[key] = true
	}
	if !seen[[2]int{1, 0}] || !seen[[2]int{2, 0}] {
		t.Fatalf("recursive tails missing from delta schedule: %v", stage.loop)
	}
	if len(stage.allLoop) != 3 {
		t.Fatalf("all-relations loop actions %v, want all three source rules", stage.allLoop)
	}
	for _, action := range stage.allLoop {
		if action.inputTail != -1 {
			t.Fatalf("all-or-nothing action unexpectedly selects a delta: %+v", action)
		}
	}
}

func modeValue(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
