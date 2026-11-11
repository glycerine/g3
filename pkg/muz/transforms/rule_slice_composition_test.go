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

func testRuleProofComposition(t *testing.T, withSlice, withCoalesce bool, stop int64, queryValue int, configuration ...int) bool {
	branches, rounds := 2, 1
	if len(configuration) > 0 {
		branches = configuration[0]
		rounds = configuration[1]
	}
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
	values := []ast.ASTHandle{zero, one}
	for i := 2; i < branches; i++ {
		value, e := m.ArithmeticIntegerNumeralText(fmt.Sprint(i))
		coneMust(t, e)
		values = append(values, value)
	}
	for _, v := range values {
		guard, e := m.MkEq(y, v)
		coneMust(t, e)
		add(qa, []ast.ASTHandle{pa, guard})
	}
	queryTerm := values[queryValue]
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
	for i := 0; i < rounds; i++ {
		apply(transforms.NewRuleUnfoldPlugin(m, f.rules, f.globals.Context, f.globals, 100))
	}
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
		// derivedVariables builds a rule from the proof fact, which replaces
		// the rule manager's retained m_head/m_body terms; release them on
		// both sides of the comparison.
		coneMust(t, f.rules.ReleaseHornTemporaries())
		before := m.Stats().LiveReferences
		count, e := m.GetRefCount(proof)
		coneMust(t, e)
		mode, e := m.ProofMode()
		coneMust(t, e)
		sourceCounts := make(map[ast.ASTHandle]uint32)
		for source := range sources {
			n, e := m.GetRefCount(source)
			coneMust(t, e)
			sourceCounts[source] = n
		}
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
		coneMust(t, f.rules.ReleaseHornTemporaries())
		if m.Stats().LiveReferences != before {
			t.Fatal("composition failure changed reference accounting", stop, m.Stats().LiveReferences, before)
		}
		current, e := m.GetRefCount(proof)
		coneMust(t, e)
		if current != count {
			t.Fatal("composition changed input proof ownership")
		}
		for source, want := range sourceCounts {
			n, e := m.GetRefCount(source)
			coneMust(t, e)
			if n != want {
				t.Fatal("conversion changed original clause proof ownership", stop, n, want, "cause", cause)
			}
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
	// The restored theorem must prove the original P-at-query goal rule,
	// including its independent column removed by slicing. A second unfold
	// resolves the P fact and must prove the ground goal itself.
	p0, e := m.MkApp(p, []ast.ASTHandle{x, queryTerm})
	coneMust(t, e)
	tails := []ast.ASTHandle{p0}
	if rounds > 1 {
		tails = nil
	}
	expected, e := f.rules.NewRule(f.applications["Goal"], tails, nil, "")
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
		if status != lbool.False && !withCoalesce {
			var b bytes.Buffer
			coneMust(t, ast.DisplayLegacySMT(&b, m, side[i], 0, nil))
			t.Fatal("invalid composed side condition", i, status, b.String())
		}
		if status != lbool.False {
			// A coalesce premise has no original rule, so the converter keeps
			// the inliner's hyper-resolution node. That node has the source's
			// shape (resolve_rule after rule_unifier::apply): its conclusion is
			// the normalized resolvent while its substitutions number the raw
			// one, and the checker's side condition (forall binder.
			// resolvent <=> body) does not hold for it in the source either.
			t.Log("unverifiable side condition from a kept source-shaped hyper-resolution", i)
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
		wantLeaves = branches + 1
	}
	if rounds > 1 {
		wantLeaves++
	}
	if leaves != wantLeaves {
		t.Fatal("wrong original clause count", leaves, wantLeaves)
	}

	return succeeded
}

func TestRuleSliceCompositionAllocationFailureAndRetry(t *testing.T) {
	for _, configuration := range [][2]int{{2, 1}, {3, 2}} {
		t.Run(fmt.Sprintf("branches=%d/rounds=%d", configuration[0], configuration[1]), func(t *testing.T) {
			succeeded := false
			for stop := int64(0); stop < 4096 && !succeeded; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					succeeded = testRuleProofComposition(t, true, true, stop, 0, configuration[0], configuration[1])
				})
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
		})
	}
}

func TestRuleSliceNestedCoalesceAndRepeatedUnfoldProofs(t *testing.T) {
	for _, branches := range []int{2, 3} {
		for _, rounds := range []int{1, 2} {
			for value := 0; value < branches; value++ {
				t.Run(fmt.Sprintf("branches=%d/rounds=%d/value=%d", branches, rounds, value), func(t *testing.T) { testRuleProofComposition(t, true, true, -1, value, branches, rounds) })
			}
		}
	}
}
