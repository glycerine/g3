package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func externalIntervalIdentityFormula(t *testing.T, manager *ast.ASTManager, tail, head ast.ASTHandle, sorts []ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	arguments := make([]ast.ASTHandle, len(sorts))
	for i := range sorts {
		variable, err := manager.MkVar(uint32(i), sorts[i])
		fragmentMust(t, err)
		arguments[i] = variable
	}
	tailApplication, err := manager.MkApp(tail, arguments)
	fragmentMust(t, err)
	headApplication, err := manager.MkApp(head, arguments)
	fragmentMust(t, err)
	formula, err := manager.MkImplies(tailApplication, headApplication)
	fragmentMust(t, err)
	return formula
}

func addExternalIntervalIdentityRule(t *testing.T, manager *ast.ASTManager, engine *PropositionalEngine, tail, head ast.ASTHandle, sorts []ast.ASTHandle, name string) {
	t.Helper()
	fragmentMust(t, engine.AddRule(externalIntervalIdentityFormula(t, manager, tail, head, sorts), name))
}

func TestExternalIntervalDatalogAlignedIdentityRuleAndRepeat(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-p-"+kinds[0], []ast.ASTHandle{integer}, kinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-q-"+kinds[0], []ast.ASTHandle{integer}, kinds)
			seven, err := manager.ArithmeticIntegerNumeralText("7")
			fragmentMust(t, err)
			// A direct seed may precede rule installation; the later query must
			// reuse the persistent activation instead of rebuilding it.
			fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{seven}))
			addExternalIntervalIdentityRule(t, manager, engine, p, q, []ast.ASTHandle{integer}, "external-interval-identity")

			callbacks.operations = nil
			status, err := engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("nonempty product identity result", status)
			}
			wantFirst := []int32{
				ast.DatalogOpIsEmpty, ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpEmpty, ast.DatalogOpUnion, ast.DatalogOpIsEmpty,
			}
			if kinds[0] != "external_relation" {
				wantFirst = []int32{
					ast.DatalogOpIsEmpty, ast.DatalogOpClone,
					ast.DatalogOpEmpty, ast.DatalogOpUnion, ast.DatalogOpIsEmpty,
				}
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, wantFirst)
			reason, err := engine.ReasonUnknown()
			fragmentMust(t, err)
			if reason != "approximated" {
				t.Fatal("identity result did not preserve product imprecision", reason)
			}

			callbacks.operations = nil
			status, err = engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("repeated product identity result", status)
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpUnion, ast.DatalogOpIsEmpty,
			})
		})
	}
}

func TestExternalIntervalDatalogAlignedIdentityEmptySource(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-empty-p-"+kinds[0], []ast.ASTHandle{integer}, kinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-empty-q-"+kinds[0], []ast.ASTHandle{integer}, kinds)
			addExternalIntervalIdentityRule(t, manager, engine, p, q, []ast.ASTHandle{integer}, "external-interval-empty-identity")
			selected, err := engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			if !selected {
				t.Fatal("identity products were not selected")
			}

			for iteration := 0; iteration < 2; iteration++ {
				callbacks.operations = nil
				status, queryErr := engine.QueryPredicate(q)
				fragmentMust(t, queryErr)
				if status != lbool.False {
					t.Fatal("empty identity source became reachable", iteration, status)
				}
				want := []int32{
					ast.DatalogOpIsEmpty, ast.DatalogOpIsEmpty,
					ast.DatalogOpEmpty, ast.DatalogOpEmpty, ast.DatalogOpIsEmpty,
				}
				if kinds[0] != "external_relation" {
					want = []int32{ast.DatalogOpEmpty, ast.DatalogOpEmpty}
				}
				assertExternalIntervalRegisterOperations(t, callbacks.operations, want)
			}
		})
	}
}

func TestExternalIntervalDatalogAlignedIdentityUnionRefusalRollsBack(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-refuse-p", []ast.ASTHandle{integer}, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-refuse-q", []ast.ASTHandle{integer}, kinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
	addExternalIntervalIdentityRule(t, manager, engine, p, q, []ast.ASTHandle{integer}, "external-interval-refused-identity")
	refusal := errors.New("external identity union refused")
	callbacks.operations = nil
	callbacks.refusalOperation = ast.DatalogOpUnion
	callbacks.refusalAt = 1
	callbacks.refusal = refusal
	status, queryErr := engine.QueryPredicate(q)
	if status != lbool.Undef || !errors.Is(queryErr, refusal) {
		t.Fatal("UNION refusal did not fail identity query", status, queryErr)
	}
	assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
		ast.DatalogOpIsEmpty, ast.DatalogOpIsEmpty, ast.DatalogOpClone,
		ast.DatalogOpEmpty, ast.DatalogOpUnion,
	})
	callbacks.refusal = nil
	empty, err := engine.externalIntervalDatalog.entries[q].product.Empty()
	fragmentMust(t, err)
	if !empty {
		t.Fatal("refused UNION published the working target")
	}

	callbacks.operations = nil
	status, err = engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("identity query did not recover after UNION refusal", status)
	}
}

func TestExternalIntervalDatalogOppositeOrderIdentityUnionRefusalRollsBack(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	pKinds := []string{"external_relation", "interval_relation"}
	qKinds := []string{"interval_relation", "external_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-refuse-p", []ast.ASTHandle{integer}, pKinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-refuse-q", []ast.ASTHandle{integer}, qKinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
	addExternalIntervalIdentityRule(t, manager, engine, p, q, []ast.ASTHandle{integer}, "external-interval-refused-identity")
	refusal := errors.New("external identity union refused")
	callbacks.operations = nil
	callbacks.refusalOperation = ast.DatalogOpUnion
	callbacks.refusalAt = 1
	callbacks.refusal = refusal
	status, queryErr := engine.QueryPredicate(q)
	if status != lbool.Undef || !errors.Is(queryErr, refusal) {
		t.Fatal("UNION refusal did not fail identity query", status, queryErr)
	}
	assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
		ast.DatalogOpIsEmpty, ast.DatalogOpClone, ast.DatalogOpEmpty, ast.DatalogOpUnion,
	})
	callbacks.refusal = nil
	empty, err := engine.externalIntervalDatalog.entries[q].product.Empty()
	fragmentMust(t, err)
	if !empty {
		t.Fatal("refused UNION published the working target")
	}
	if got := engine.externalIntervalDatalog.entries[q].product.Kinds(); len(got) != 2 || got[0] != qKinds[0] || got[1] != qKinds[1] {
		t.Fatal("refused unaligned UNION changed the persistent target spec", got)
	}

	callbacks.operations = nil
	status, err = engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("identity query did not recover after UNION refusal", status)
	}
	if got := engine.externalIntervalDatalog.entries[q].product.Kinds(); len(got) != 2 || got[0] != "external_relation" || got[1] != "interval_relation" {
		t.Fatal("successful unaligned UNION did not publish the common spec", got)
	}
}

func TestExternalIntervalDatalogIdentityLoadCleanupRefusalRetriesPublicly(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-load-p", []ast.ASTHandle{integer}, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-load-q", []ast.ASTHandle{integer}, kinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
	fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{two}))
	addExternalIntervalIdentityRule(t, manager, engine, p, q, []ast.ASTHandle{integer}, "external-interval-load-cleanup-identity")

	refusal := errors.New("external identity load cleanup pending")
	nested := &externalIntervalNestedConstructionFailure{
		runtime: manager.NumeralService().RuntimeOwner(),
		refusal: refusal,
	}
	callbacks.operations = nil
	callbacks.refusalOperation = ast.DatalogOpClone
	callbacks.refusalAt = 2
	callbacks.refusal = nested
	status, queryErr := engine.QueryPredicate(q)
	if status != lbool.Undef || !errors.Is(queryErr, refusal) {
		t.Fatal("LOAD cleanup refusal did not fail the public query", status, queryErr)
	}
	activation := engine.externalIntervalDatalog
	if activation.pending == nil || activation.pendingCompletesRule || nested.calls != 1 {
		t.Fatal("LOAD cleanup refusal was not adopted by the activation", nested.calls)
	}
	if operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
		t.Fatal("failed LOAD reached UNION", callbacks.operations)
	}

	callbacks.operations = nil
	callbacks.refusal = nil
	status, err = engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef || activation.pending != nil || nested.calls != 2 {
		t.Fatal("public LOAD cleanup retry did not recover", status, nested.calls)
	}
	assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
		ast.DatalogOpIsEmpty, ast.DatalogOpClone,
		ast.DatalogOpIsEmpty, ast.DatalogOpClone,
		ast.DatalogOpUnion, ast.DatalogOpIsEmpty,
	})
}

func TestExternalIntervalDatalogIdentityCommittedContinuationStoresWithoutReplay(t *testing.T) {
	externalFirst := []string{"external_relation", "interval_relation"}
	intervalFirst := []string{"interval_relation", "external_relation"}
	for _, test := range []struct {
		name           string
		pKinds, qKinds []string
	}{
		{name: "aligned-external-first", pKinds: externalFirst, qKinds: externalFirst},
		{name: "aligned-interval-first", pKinds: intervalFirst, qKinds: intervalFirst},
		{name: "opposite-head-external-first", pKinds: intervalFirst, qKinds: externalFirst},
		{name: "opposite-head-interval-first", pKinds: externalFirst, qKinds: intervalFirst},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-continuation-p-"+test.name, []ast.ASTHandle{integer}, test.pKinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-continuation-q-"+test.name, []ast.ASTHandle{integer}, test.qKinds)
			five, err := manager.ArithmeticIntegerNumeralText("5")
			fragmentMust(t, err)
			fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{five}))
			addExternalIntervalIdentityRule(t, manager, engine, p, q, []ast.ASTHandle{integer}, "external-interval-continuation-identity")
			selected, err := engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			if !selected {
				t.Fatal("identity products were not selected")
			}

			activation := engine.externalIntervalDatalog
			oldTarget := activation.entries[q].product
			callbacks.operations = nil
			registers, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{q, p})
			fragmentMust(t, err)
			source, err := registers.relation(p)
			fragmentMust(t, err)
			target, err := registers.ensureRelation(q)
			fragmentMust(t, err)
			fragmentMust(t, target.Union(source, nil, false))
			want := []int32{
				ast.DatalogOpIsEmpty, ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpEmpty, ast.DatalogOpUnion,
			}
			if test.qKinds[0] != "external_relation" {
				want = []int32{ast.DatalogOpIsEmpty, ast.DatalogOpClone, ast.DatalogOpEmpty, ast.DatalogOpUnion}
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, want)

			refusal := errors.New("external identity committed cleanup pending")
			nested := &externalIntervalNestedConstructionFailure{
				runtime: manager.NumeralService().RuntimeOwner(),
				refusal: refusal,
			}
			pending := newExternalIntervalDatalogIdentityFailure(nested, registers, nested)
			activation.pending = pending
			activation.pendingCompletesRule = true
			activation.pendingIdentity = activation.identity
			callbacksBeforeCleanup := len(callbacks.operations)
			if err = pending.Cleanup(); !errors.Is(err, refusal) || activation.entries[q].product != oldTarget {
				t.Fatal("first cleanup refusal published staged registers", err)
			}
			status, err := engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.Undef || activation.entries[q].product == oldTarget || len(callbacks.operations) != callbacksBeforeCleanup+1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
				t.Fatal("public retry replayed committed UNION or missed STORE", status, callbacks.operations)
			}
			if activation.pending != nil || activation.pendingCompletesRule || nested.calls != 2 {
				t.Fatal("activation did not consume the completed continuation", nested.calls)
			}
			empty, err := activation.entries[q].product.Empty()
			fragmentMust(t, err)
			if empty || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
				t.Fatal("committed continuation did not publish its union target", empty, callbacks.operations)
			}
			wantKinds := test.qKinds
			if test.pKinds[0] != test.qKinds[0] {
				wantKinds = externalFirst
			}
			if got := activation.entries[q].product.Kinds(); len(got) != 2 || got[0] != wantKinds[0] || got[1] != wantKinds[1] {
				t.Fatal("committed continuation published the wrong product spec", got, wantKinds)
			}

			callbacks.operations = nil
			status, err = engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("next query did not execute the next rule evaluation", status)
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpUnion, ast.DatalogOpIsEmpty,
			})

			// A mutation cannot race a pre-STORE register snapshot. Its first
			// attempt observes the pending cleanup refusal; the retry publishes
			// that snapshot before adding the new fact to the replacement.
			oldSource := activation.entries[p].product
			callbacks.operations = nil
			registers, err = activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{q, p})
			fragmentMust(t, err)
			source, err = registers.relation(p)
			fragmentMust(t, err)
			target, err = registers.relation(q)
			fragmentMust(t, err)
			fragmentMust(t, target.Union(source, nil, false))
			mutationRefusal := errors.New("external identity cleanup blocks mutation")
			mutationNested := &externalIntervalNestedConstructionFailure{
				runtime: manager.NumeralService().RuntimeOwner(),
				refusal: mutationRefusal,
			}
			activation.pending = newExternalIntervalDatalogIdentityFailure(mutationNested, registers, mutationNested)
			activation.pendingCompletesRule = true
			activation.pendingIdentity = activation.identity
			six, err := manager.ArithmeticIntegerNumeralText("6")
			fragmentMust(t, err)
			if err = engine.AddFact(p, []ast.ASTHandle{six}); !errors.Is(err, mutationRefusal) || activation.entries[p].product != oldSource {
				t.Fatal("fact mutation bypassed pending identity cleanup", err)
			}
			fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{six}))
			if activation.pending != nil || activation.pendingCompletesRule || activation.entries[p].product == oldSource || mutationNested.calls != 2 {
				t.Fatal("fact retry did not publish before mutation", mutationNested.calls)
			}
			if operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 || operationCount(callbacks.operations, ast.DatalogOpStore) != 1 {
				t.Fatal("fact retry replayed the staged rule or missed the mutation", callbacks.operations)
			}
			status, err = engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("post-mutation identity result", status)
			}
		})
	}
}

func TestExternalIntervalDatalogPendingCompletionDoesNotSkipReplacedPlan(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-replace-p", []ast.ASTHandle{integer}, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-replace-q", []ast.ASTHandle{integer}, kinds)
	r := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-replace-r", []ast.ASTHandle{integer}, kinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
	addExternalIntervalIdentityRule(t, manager, engine, p, q, []ast.ASTHandle{integer}, "external-interval-replaced-identity")
	engine.flushRawDisplayRules()
	selected, err := engine.ensureExternalIntervalDatalog()
	fragmentMust(t, err)
	if !selected {
		t.Fatal("identity products were not selected")
	}

	activation := engine.externalIntervalDatalog
	callbacks.operations = nil
	registers, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{q, p})
	fragmentMust(t, err)
	source, err := registers.relation(p)
	fragmentMust(t, err)
	target, err := registers.ensureRelation(q)
	fragmentMust(t, err)
	fragmentMust(t, target.Union(source, nil, false))
	refusal := errors.New("old identity completion cleanup pending")
	nested := &externalIntervalNestedConstructionFailure{
		runtime: manager.NumeralService().RuntimeOwner(),
		refusal: refusal,
	}
	pending := newExternalIntervalDatalogIdentityFailure(nested, registers, nested)
	activation.pending = pending
	activation.pendingCompletesRule = true
	activation.pendingIdentity = activation.identity
	if err = pending.Cleanup(); !errors.Is(err, refusal) {
		t.Fatal("old identity cleanup did not pause", err)
	}

	// Public UpdateRule rejects a different head because the old rule cannot
	// subsume it. Exercise the activation's revision boundary directly so a
	// future admissible plan replacement cannot inherit this stale completion.
	activation.identity = &externalIntervalDatalogIdentityRule{head: r, tail: p}
	fragmentMust(t, activation.executeExternalIntervalIdentity())
	status, err := activation.query([]ast.ASTHandle{r})
	fragmentMust(t, err)
	if status != lbool.Undef || activation.pending != nil || nested.calls != 2 {
		t.Fatal("replacement identity was not evaluated after old cleanup", status, nested.calls)
	}
	if operationCount(callbacks.operations, ast.DatalogOpUnion) != 2 {
		t.Fatal("old completion incorrectly short-circuited replacement identity", callbacks.operations)
	}
}

func TestExternalIntervalDatalogIdentityRejectsUnsupportedShapesBeforeCallbacks(t *testing.T) {
	for _, shape := range []string{"self", "constant", "reordered", "repeated", "nullary", "two-rules"} {
		t.Run(shape, func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			sorts := []ast.ASTHandle{integer}
			if shape == "reordered" || shape == "repeated" {
				sorts = []ast.ASTHandle{integer, integer}
			}
			if shape == "nullary" {
				sorts = nil
			}
			pKinds := []string{"external_relation", "interval_relation"}
			qKinds := append([]string(nil), pKinds...)
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-invalid-p-"+shape, sorts, pKinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-invalid-q-"+shape, sorts, qKinds)
			headDeclaration := q
			if shape == "self" {
				headDeclaration = p
			}
			arguments := make([]ast.ASTHandle, len(sorts))
			for i := range sorts {
				arguments[i], err = manager.MkVar(uint32(i), sorts[i])
				fragmentMust(t, err)
			}
			tailArguments := append([]ast.ASTHandle(nil), arguments...)
			headArguments := append([]ast.ASTHandle(nil), arguments...)
			switch shape {
			case "constant":
				value, valueErr := manager.ArithmeticIntegerNumeralText("9")
				fragmentMust(t, valueErr)
				tailArguments[0], headArguments[0] = value, value
			case "reordered":
				headArguments[0], headArguments[1] = headArguments[1], headArguments[0]
			case "repeated":
				tailArguments[1], headArguments[1] = tailArguments[0], headArguments[0]
			}
			tail, err := manager.MkApp(p, tailArguments)
			fragmentMust(t, err)
			head, err := manager.MkApp(headDeclaration, headArguments)
			fragmentMust(t, err)
			formula, err := manager.MkImplies(tail, head)
			fragmentMust(t, err)
			fragmentMust(t, engine.AddRule(formula, "external-interval-invalid-"+shape))
			if shape == "two-rules" {
				r := declareExternalIntervalPredicate(t, manager, engine, "external-interval-identity-invalid-r", sorts, pKinds)
				rTail, ruleErr := manager.MkApp(r, arguments)
				fragmentMust(t, ruleErr)
				second, ruleErr := manager.MkImplies(rTail, head)
				fragmentMust(t, ruleErr)
				fragmentMust(t, engine.AddRule(second, "external-interval-invalid-second"))
			}

			status, queryErr := engine.QueryPredicate(q)
			if status != lbool.Undef || !errors.Is(queryErr, ErrExternalIntervalDatalogFragment) || engine.externalIntervalDatalog != nil || len(callbacks.operations) != 0 {
				t.Fatal("unsupported identity shape reached product callbacks", status, queryErr, callbacks.operations)
			}
		})
	}
}
