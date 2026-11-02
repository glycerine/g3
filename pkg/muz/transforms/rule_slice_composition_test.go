package transforms_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestRuleSliceCoalesceUnfoldProofComposition(t *testing.T) {
	for _, value := range []int{0, 1} {
		t.Run(fmt.Sprint(value), func(t *testing.T) { testRuleProofComposition(t, true, true, -1, value) })
	}
}

func TestRuleSliceUnfoldProofComposition(t *testing.T) {
	testRuleProofComposition(t, true, false, -1, 0)
}

func TestRuleCoalesceUnfoldProofComposition(t *testing.T) {
	testRuleProofComposition(t, false, true, -1, 0)
}

func TestRuleUnfoldCompactsProofSubstitutions(t *testing.T) {
	testRuleProofComposition(t, false, false, -1, 0)
}

func testRuleProofComposition(t *testing.T, withSlice, withCoalesce bool, stop int64, queryValue int) bool {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("composition fixture leaked bytes", budget.Used())
		}
	})
	f := newConeFixture(t, "Goal")
	budget = f.manager.Budget()
	m := f.manager
	coneMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	zero, e := m.ArithmeticIntegerNumeralText("0")
	coneMust(t, e)
	one, e := m.ArithmeticIntegerNumeralText("1")
	coneMust(t, e)
	integer, e := m.GetSort(zero)
	coneMust(t, e)
	boolean, e := m.MkBoolSort()
	coneMust(t, e)
	declare := func(name string) ast.ASTHandle {
		n, e := m.Symbols.Intern(name)
		coneMust(t, e)
		d, e := m.MkFuncDecl(n, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, e)
		coneMust(t, f.rules.RegisterPredicate(d))
		return d
	}
	p, q := declare("composition-P"), declare("composition-Q")
	x, e := m.MkVar(0, integer)
	coneMust(t, e)
	y, e := m.MkVar(1, integer)
	coneMust(t, e)
	pa, e := m.MkApp(p, []ast.ASTHandle{x, y})
	coneMust(t, e)
	qa, e := m.MkApp(q, []ast.ASTHandle{x, y})
	coneMust(t, e)
	sources := make(map[ast.ASTHandle]bool)
	add := func(h ast.ASTHandle, tail []ast.ASTHandle) {
		r, e := f.rules.NewRule(h, tail, nil, "")
		coneMust(t, e)
		coneMust(t, f.rules.MkRuleAssertedProof(r))
		proof, found, e := r.Proof()
		coneMust(t, e)
		if !found {
			t.Fatal("missing source proof")
		}
		sources[proof] = true
		coneMust(t, f.set.Add(r))
		coneMust(t, r.Close())
	}
	add(pa, nil)
	for _, v := range []ast.ASTHandle{zero, one} {
		guard, e := m.MkEq(y, v)
		coneMust(t, e)
		add(qa, []ast.ASTHandle{pa, guard})
	}
	queryTerm := zero
	if queryValue == 1 {
		queryTerm = one
	}
	q0, e := m.MkApp(q, []ast.ASTHandle{x, queryTerm})
	coneMust(t, e)
	add(f.applications["Goal"], []ast.ASTHandle{q0})
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, e := f.set.Finalize()
	coneMust(t, e)
	if !valid {
		t.Fatal("invalid composition fixture")
	}
	apply := func(plugin base.RuleTransformPlugin) {
		transformer := base.NewRuleTransformer()
		coneMust(t, transformer.Register(plugin))
		changed, e := transformer.Apply(f.set)
		coneMust(t, e)
		coneMust(t, transformer.Close())
		if !changed {
			t.Fatal("composition stage made no change")
		}
	}
	var converter *transforms.SliceProofConverter
	if withSlice {
		slice := transforms.NewRuleSlicePlugin(m, f.rules, 1)
		slice.SetGenerateProofTrace(true)
		// TakeProofConverter must precede closing its plugin.
		transformer := base.NewRuleTransformer()
		coneMust(t, transformer.Register(slice))
		changed, e := transformer.Apply(f.set)
		coneMust(t, e)
		if !changed {
			t.Fatal("expected slicing")
		}
		converter = slice.TakeProofConverter()
		if converter == nil {
			t.Fatal("missing converter")
		}
		coneMust(t, converter.SetNormalizationServices(f.globals.Context, f.globals))
		defer func() { coneMust(t, converter.Close()) }()
		coneMust(t, transformer.Close())
	}
	if withCoalesce {
		apply(transforms.NewRuleCoalescePluginWithProofTrace(m, f.rules, 50, true))
	}
	apply(transforms.NewRuleUnfoldPlugin(m, f.rules, f.globals.Context, f.globals, 100))
	rules, e := f.set.PredicateRules(f.declarations["Goal"])
	coneMust(t, e)
	defer func() {
		for _, r := range rules {
			coneMust(t, r.Close())
		}
	}()
	if len(rules) != 1 {
		t.Fatal("expected one unfolded goal", len(rules))
	}
	proof, found, e := rules[0].Proof()
	coneMust(t, e)
	if !found {
		t.Fatal("lost unfolded proof")
	}
	succeeded := false
	if stop >= 0 {
		before := m.Stats().LiveReferences
		count, e := m.GetRefCount(proof)
		coneMust(t, e)
		mode, e := m.ProofMode()
		coneMust(t, e)
		m.Budget().FailAfter(stop)
		attempt, cause := converter.Convert(proof)
		m.Budget().FailAfter(-1)
		if cause != nil {
			if attempt != (ast.ASTRef{}) {
				t.Fatal("failed composition published proof")
			}
			var cleanup memory.ConstructionFailure
			if errors.As(cause, &cleanup) {
				coneMust(t, cleanup.Cleanup())
			}
			var failure memory.Failure
			if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &failure) || failure.Kind != memory.FailureMemout) {
				t.Fatal("unexpected composition failure", stop, cause)
			}
		} else {
			succeeded = true
			coneMust(t, m.DelRef(&attempt))
		}
		if m.Stats().LiveReferences != before {
			t.Fatal("composition failure changed reference accounting", stop, m.Stats().LiveReferences, before)
		}
		current, e := m.GetRefCount(proof)
		coneMust(t, e)
		if current != count {
			t.Fatal("composition changed input proof ownership")
		}
		currentMode, e := m.ProofMode()
		coneMust(t, e)
		if currentMode != mode {
			t.Fatal("composition changed proof mode")
		}
	}
	var converted ast.ASTRef
	if converter != nil {
		converted, e = converter.Convert(proof)
	} else {
		converted, e = m.NewRef(proof)
	}
	coneMust(t, e)
	defer func() { coneMust(t, m.DelRef(&converted)) }()
	node, e := m.RefGet(converted)
	coneMust(t, e)
	// The restored theorem must prove exactly the original P-at-zero goal
	// rule, including its independent column removed by slicing.
	p0, e := m.MkApp(p, []ast.ASTHandle{x, queryTerm})
	coneMust(t, e)
	expected, e := f.rules.NewRule(f.applications["Goal"], []ast.ASTHandle{p0}, nil, "")
	coneMust(t, e)
	expectedFormula, e := expected.ToFormula()
	coneMust(t, e)
	expectedFact, e := m.RefGet(expectedFormula)
	coneMust(t, e)
	actual, e := m.GetProofFact(node)
	coneMust(t, e)
	equal, e := m.AreEqual(actual, expectedFact)
	coneMust(t, e)
	coneMust(t, m.DelRef(&expectedFormula))
	coneMust(t, expected.Close())
	if !equal {
		var b bytes.Buffer
		coneMust(t, ast.DisplayLegacySMT(&b, m, actual, 0, nil))
		t.Fatal("wrong restored conclusion", b.String())
	}
	checker, e := proofs.NewChecker(m)
	coneMust(t, e)
	valid, side, e := checker.Check(node)
	coneMust(t, e)
	defer func() { coneMust(t, checker.ReleaseSideConditions(side)) }()
	if !valid {
		t.Fatal("invalid composed proof")
	}
	solver, e := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: f.globals.Context, Globals: f.globals})
	coneMust(t, e)
	defer func() { coneMust(t, solver.Close()) }()
	for i, condition := range side {
		coneMust(t, solver.Push())
		vars, e := ast.NewASTUsedVars(m)
		coneMust(t, e)
		coneMust(t, vars.Collect(condition))
		var pins []ast.ASTRef
		args := make([]ast.ASTHandle, vars.MaxFoundVarIndexPlusOne())
		for j := range args {
			sort := vars.Contains(uint32(j))
			if sort.IsNull() {
				continue
			}
			args[j], e = m.MkFreshConst("proof-arbitrary", sort)
			coneMust(t, e)
			r, e := m.NewRef(args[j])
			coneMust(t, e)
			pins = append(pins, r)
		}
		subst, e := rewriter.NewVarSubstitution(m, false)
		coneMust(t, e)
		grounded, e := m.NewRef(ast.ASTHandle{})
		coneMust(t, e)
		coneMust(t, subst.Substitute(condition, args, grounded))
		coneMust(t, subst.Close())
		condition, e = m.RefGet(grounded)
		coneMust(t, e)
		neg, e := m.MkNot(condition)
		coneMust(t, e)
		coneMust(t, solver.Assert(neg))
		status, e := solver.CheckSat(nil)
		coneMust(t, e)
		coneMust(t, solver.Pop(1))
		coneMust(t, m.DelRef(&grounded))
		for j := range pins {
			coneMust(t, m.DelRef(&pins[j]))
		}
		if status != lbool.False {
			var b bytes.Buffer
			coneMust(t, ast.DisplayLegacySMT(&b, m, side[i], 0, nil))
			t.Fatal("invalid composed side condition", i, status, b.String())
		}
	}
	todo := []ast.ASTHandle{node}
	seen := make(map[ast.ASTHandle]bool)
	leaves := 0
	for len(todo) > 0 {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		var fact ast.ASTHandle
		asserted, e := m.MatchProofAsserted(h, &fact)
		coneMust(t, e)
		if asserted {
			leaves++
		}
		if asserted && !sources[h] {
			var b bytes.Buffer
			coneMust(t, ast.DisplayLegacySMT(&b, m, fact, 0, nil))
			t.Fatal("composed proof contains nonoriginal assertion", b.String())
		}
		n, e := m.GetProofNumParents(h)
		coneMust(t, e)
		for i := uint32(0); i < n; i++ {
			parent, e := m.GetProofParent(h, i)
			coneMust(t, e)
			todo = append(todo, parent)
		}
	}
	wantLeaves := 2
	if withCoalesce {
		wantLeaves = 3
	}
	if leaves != wantLeaves {
		t.Fatal("wrong original clause count", leaves, wantLeaves)
	}

	return succeeded
}

func TestRuleSliceCompositionAllocationFailureAndRetry(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 4096 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) { succeeded = testRuleProofComposition(t, true, true, stop, 0) })
		if t.Failed() {
			return
		}
		if succeeded {
			if stop == 0 {
				t.Fatal("missing composition allocation failure")
			}
			t.Log("first successful allocation allowance", stop)
		}
	}
	if !succeeded {
		t.Fatal("missing composition allocation success frontier")
	}
}
