package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func addExternalIntervalCartesianRule(t *testing.T, manager *ast.ASTManager, engine *PropositionalEngine, left, right, head ast.ASTHandle, leftSort, rightSort ast.ASTHandle, name string) {
	t.Helper()
	leftVariable, err := manager.MkVar(0, leftSort)
	fragmentMust(t, err)
	rightVariable, err := manager.MkVar(1, rightSort)
	fragmentMust(t, err)
	leftApplication, err := manager.MkApp(left, []ast.ASTHandle{leftVariable})
	fragmentMust(t, err)
	rightApplication, err := manager.MkApp(right, []ast.ASTHandle{rightVariable})
	fragmentMust(t, err)
	body, err := manager.MkAnd(leftApplication, rightApplication)
	fragmentMust(t, err)
	headApplication, err := manager.MkApp(head, []ast.ASTHandle{leftVariable, rightVariable})
	fragmentMust(t, err)
	formula, err := manager.MkImplies(body, headApplication)
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(formula, name))
}

func resetExternalIntervalCartesianCallbacks(callbacks *externalIntervalCallbackState) {
	callbacks.operations = nil
	callbacks.cloneOutputs = nil
	callbacks.joinArguments = nil
	callbacks.joinParameters = nil
	callbacks.projectParameters = nil
	callbacks.renameParameters = nil
}

func assertExternalIntervalCartesianJoin(t *testing.T, callbacks *externalIntervalCallbackState, leftClone, rightClone int) {
	t.Helper()
	if len(callbacks.joinArguments) != 1 || len(callbacks.joinArguments[0]) != 2 ||
		len(callbacks.joinParameters) != 1 || len(callbacks.joinParameters[0]) != 0 {
		t.Fatal("Cartesian JOIN callback shape", callbacks.joinArguments, callbacks.joinParameters)
	}
	if leftClone >= len(callbacks.cloneOutputs) || rightClone >= len(callbacks.cloneOutputs) ||
		callbacks.joinArguments[0][0] != callbacks.cloneOutputs[leftClone] ||
		callbacks.joinArguments[0][1] != callbacks.cloneOutputs[rightClone] {
		t.Fatal("Cartesian JOIN did not receive LOAD clones in tail order", callbacks.cloneOutputs, callbacks.joinArguments)
	}
}

func TestExternalIntervalDatalogCartesianRuleBothLeftOrdersAndRepeat(t *testing.T) {
	for _, externalLeft := range []bool{true, false} {
		label := "interval-left"
		leftKinds := []string{"interval_relation", "external_relation"}
		rightKinds := []string{"external_relation", "interval_relation"}
		if externalLeft {
			label = "external-left"
			leftKinds, rightKinds = rightKinds, leftKinds
		}
		// The normalized left JOIN input is q, so keep h in p's opposite
		// component order to exercise source product UNION alignment.
		headKinds := append([]string(nil), leftKinds...)
		t.Run(label, func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			real, err := manager.MkFamilySort(family, 0, nil)
			fragmentMust(t, err)
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-p-"+label, []ast.ASTHandle{integer}, leftKinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-q-"+label, []ast.ASTHandle{real}, rightKinds)
			h := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-h-"+label, []ast.ASTHandle{integer, real}, headKinds)
			one, err := manager.ArithmeticIntegerNumeralText("1")
			fragmentMust(t, err)
			fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
			half, err := manager.ArithmeticRealNumeralText("1/2")
			fragmentMust(t, err)
			halfRoot, err := manager.NewRef(half)
			fragmentMust(t, err)
			fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{half}))
			fragmentMust(t, manager.DelRef(&halfRoot))
			addExternalIntervalCartesianRule(t, manager, engine, p, q, h, integer, real, "external-interval-cartesian-"+label)

			resetExternalIntervalCartesianCallbacks(callbacks)
			status, err := engine.QueryPredicate(h)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("nonempty Cartesian product result", status)
			}
			want := []int32{
				ast.DatalogOpIsEmpty,
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpJoin, ast.DatalogOpIsEmpty, ast.DatalogOpRename, ast.DatalogOpEmpty, ast.DatalogOpUnion,
				ast.DatalogOpIsEmpty,
			}
			if headKinds[0] != "external_relation" {
				want = want[1:]
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, want)
			assertExternalIntervalCartesianJoin(t, callbacks, 0, 1)
			if got := engine.externalIntervalDatalog.entries[p].product.Kinds(); !externalIntervalKindsEqual(got, leftKinds) {
				t.Fatal("Cartesian JOIN changed left source order", got, leftKinds)
			}
			if got := engine.externalIntervalDatalog.entries[q].product.Kinds(); !externalIntervalKindsEqual(got, rightKinds) {
				t.Fatal("Cartesian JOIN changed right source order", got, rightKinds)
			}
			if got := engine.externalIntervalDatalog.entries[h].product.Kinds(); !externalIntervalKindsEqual(got, []string{"external_relation", "interval_relation"}) {
				t.Fatal("unaligned Cartesian UNION did not canonicalize the writable head", got)
			}

			resetExternalIntervalCartesianCallbacks(callbacks)
			status, err = engine.QueryPredicate(h)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("repeated Cartesian product result", status)
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpJoin, ast.DatalogOpIsEmpty, ast.DatalogOpRename, ast.DatalogOpUnion, ast.DatalogOpIsEmpty,
			})
			assertExternalIntervalCartesianJoin(t, callbacks, 1, 2)

			two, err := manager.ArithmeticIntegerNumeralText("2")
			fragmentMust(t, err)
			fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{two}))
			resetExternalIntervalCartesianCallbacks(callbacks)
			status, err = engine.QueryPredicate(h)
			fragmentMust(t, err)
			if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
				t.Fatal("updated Cartesian source was not reevaluated once", status, callbacks.operations)
			}
		})
	}
}

func TestExternalIntervalDatalogCartesianEmptySourceThenUpdate(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-empty-p", []ast.ASTHandle{integer}, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-empty-q", []ast.ASTHandle{integer}, kinds)
	h := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-empty-h", []ast.ASTHandle{integer, integer}, kinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
	addExternalIntervalCartesianRule(t, manager, engine, p, q, h, integer, integer, "external-interval-cartesian-empty")

	resetExternalIntervalCartesianCallbacks(callbacks)
	status, err := engine.QueryPredicate(h)
	fragmentMust(t, err)
	if status != lbool.False || operationCount(callbacks.operations, ast.DatalogOpJoin) != 0 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
		t.Fatal("empty Cartesian source reached JOIN or UNION", status, callbacks.operations)
	}

	fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{one}))
	resetExternalIntervalCartesianCallbacks(callbacks)
	status, err = engine.QueryPredicate(h)
	fragmentMust(t, err)
	if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
		t.Fatal("late Cartesian source fact did not reach head", status, callbacks.operations)
	}
}

func TestExternalIntervalDatalogCartesianJoinAndUnionRefusalsRollback(t *testing.T) {
	for _, test := range []struct {
		name      string
		operation int32
		unions    int
	}{
		{name: "join", operation: ast.DatalogOpJoin},
		{name: "union", operation: ast.DatalogOpUnion, unions: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			kinds := []string{"external_relation", "interval_relation"}
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-refuse-p-"+test.name, []ast.ASTHandle{integer}, kinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-refuse-q-"+test.name, []ast.ASTHandle{integer}, kinds)
			h := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-refuse-h-"+test.name, []ast.ASTHandle{integer, integer}, kinds)
			one, err := manager.ArithmeticIntegerNumeralText("1")
			fragmentMust(t, err)
			fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
			fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{one}))
			addExternalIntervalCartesianRule(t, manager, engine, p, q, h, integer, integer, "external-interval-cartesian-refuse-"+test.name)
			selected, err := engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			if !selected {
				t.Fatal("Cartesian products were not selected")
			}
			activation := engine.externalIntervalDatalog
			headBefore := activation.entries[h].product
			leftBefore := activation.entries[p].product
			rightBefore := activation.entries[q].product
			refusal := errors.New("Cartesian operation refused")
			resetExternalIntervalCartesianCallbacks(callbacks)
			callbacks.refusalOperation = test.operation
			callbacks.refusalAt = 1
			callbacks.refusal = refusal
			status, queryErr := engine.QueryPredicate(h)
			if status != lbool.Undef || !errors.Is(queryErr, refusal) {
				t.Fatal("Cartesian refusal did not fail query", status, queryErr)
			}
			if activation.entries[h].product != headBefore || activation.entries[p].product != leftBefore || activation.entries[q].product != rightBefore {
				t.Fatal("Cartesian refusal published working registers")
			}
			if operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != test.unions {
				t.Fatal("Cartesian refusal callback frontier", callbacks.operations)
			}
			callbacks.refusal = nil
			resetExternalIntervalCartesianCallbacks(callbacks)
			status, err = engine.QueryPredicate(h)
			fragmentMust(t, err)
			if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
				t.Fatal("Cartesian query did not recover after refusal", status, callbacks.operations)
			}
		})
	}
}

func TestExternalIntervalDatalogCartesianCommittedUnionCleanupNoReplay(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-commit-p", []ast.ASTHandle{integer}, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-commit-q", []ast.ASTHandle{integer}, kinds)
	h := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-commit-h", []ast.ASTHandle{integer, integer}, kinds)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
	fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{one}))
	addExternalIntervalCartesianRule(t, manager, engine, p, q, h, integer, integer, "external-interval-cartesian-commit")
	selected, err := engine.ensureExternalIntervalDatalog()
	fragmentMust(t, err)
	if !selected {
		t.Fatal("Cartesian products were not selected")
	}
	activation := engine.externalIntervalDatalog
	oldHead := activation.entries[h].product
	resetExternalIntervalCartesianCallbacks(callbacks)
	registers, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{h, p, q})
	fragmentMust(t, err)
	left, err := registers.relation(p)
	fragmentMust(t, err)
	right, err := registers.relation(q)
	fragmentMust(t, err)
	joined, err := rel.JoinExternalIntervalProducts(left, right, nil, nil)
	fragmentMust(t, err)
	target, err := registers.ensureRelation(h)
	fragmentMust(t, err)
	fragmentMust(t, target.Union(joined, nil, false))
	refusal := errors.New("committed Cartesian UNION cleanup pending")
	nested := &externalIntervalNestedConstructionFailure{
		runtime: manager.NumeralService().RuntimeOwner(),
		refusal: refusal,
	}
	owner := &externalIntervalDatalogCartesianContinuation{
		registers: registers,
		temporary: joined,
		nested:    nested,
		runtime:   registers.runtime,
		publish:   true,
	}
	pending := newExternalIntervalDatalogCartesianFailure(nested, owner)
	activation.pending = pending
	activation.pendingCompletesRule = true
	activation.pendingCartesian = activation.cartesian
	if err = pending.Cleanup(); !errors.Is(err, refusal) || activation.entries[h].product != oldHead {
		t.Fatal("committed Cartesian cleanup did not stop before STORE", err, nested.calls)
	}

	status, err := engine.QueryPredicate(h)
	fragmentMust(t, err)
	if status != lbool.Undef || activation.entries[h].product == oldHead || activation.pending != nil || nested.calls != 2 {
		t.Fatal("committed Cartesian cleanup did not publish exactly once", status, nested.calls)
	}
	if operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
		t.Fatal("committed Cartesian continuation replayed JOIN or UNION", callbacks.operations)
	}
}

func TestExternalIntervalDatalogCartesianRejectsInvalidShapesBeforeCallbacks(t *testing.T) {
	shapes := []string{
		"compound-head", "repeated-tail",
		"negative", "unbound-guard", "cycle", "unsafe-head", "nullary-tail",
		"repeated-tail-column", "third-tail",
		"mutual-recursion",
	}
	for _, shape := range shapes {
		t.Run(shape, func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			kinds := []string{"external_relation", "interval_relation"}
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-invalid-p-"+shape, []ast.ASTHandle{integer}, kinds)
			qSorts := []ast.ASTHandle{integer}
			if shape == "nullary-tail" {
				qSorts = nil
			} else if shape == "repeated-tail-column" {
				qSorts = []ast.ASTHandle{integer, integer}
			}
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-invalid-q-"+shape, qSorts, kinds)
			headSorts := []ast.ASTHandle{integer, integer}
			if shape == "cycle" {
				headSorts = []ast.ASTHandle{integer}
			}
			h := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-invalid-h-"+shape, headSorts, kinds)
			x, err := manager.MkVar(0, integer)
			fragmentMust(t, err)
			y, err := manager.MkVar(1, integer)
			fragmentMust(t, err)
			z, err := manager.MkVar(2, integer)
			fragmentMust(t, err)
			app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
				result, cause := manager.MkApp(declaration, arguments)
				fragmentMust(t, cause)
				return result
			}
			leftApplication := app(p, x)
			var rightApplication ast.ASTHandle
			switch shape {
			case "nullary-tail":
				rightApplication = app(q)
			case "repeated-tail-column":
				rightApplication = app(q, y, y)
			case "repeated-tail":
				rightApplication = app(p, y)
			default:
				rightApplication = app(q, y)
			}
			body, err := manager.MkAnd(leftApplication, rightApplication)
			fragmentMust(t, err)
			if shape == "negative" {
				negated, cause := manager.MkNot(rightApplication)
				fragmentMust(t, cause)
				body, err = manager.MkAnd(leftApplication, negated)
				fragmentMust(t, err)
			}
			if shape == "unbound-guard" {
				guard, cause := manager.MkArithmeticLT(x, z)
				fragmentMust(t, cause)
				body, err = manager.MkAnd(body, guard)
				fragmentMust(t, err)
			}
			if shape == "third-tail" {
				r := declareExternalIntervalPredicate(t, manager, engine, "external-interval-cartesian-invalid-r", []ast.ASTHandle{integer}, kinds)
				body, err = manager.MkAnd(body, app(r, z))
				fragmentMust(t, err)
			}

			headDeclaration := h
			headArguments := []ast.ASTHandle{x, y}
			switch shape {
			case "compound-head":
				one, cause := manager.ArithmeticIntegerNumeralText("1")
				fragmentMust(t, cause)
				sum, cause := manager.MkArithmeticAdd(x, one)
				fragmentMust(t, cause)
				sumRoot, cause := manager.NewRef(sum)
				fragmentMust(t, cause)
				defer func() { fragmentMust(t, manager.DelRef(&sumRoot)) }()
				headArguments = []ast.ASTHandle{x, sum}
			case "cycle":
				headDeclaration = p
				headArguments = []ast.ASTHandle{x}
			case "unsafe-head":
				headArguments = []ast.ASTHandle{x, z}
			}
			headApplication := app(headDeclaration, headArguments...)
			formula, err := manager.MkImplies(body, headApplication)
			fragmentMust(t, err)
			fragmentMust(t, engine.AddRule(formula, "external-interval-cartesian-invalid-"+shape))
			if shape == "mutual-recursion" {
				// Both joins are individually supported; the back-edge makes
				// the complete program recursive and must fail preflight.
				backBody, cause := manager.MkAnd(app(h, x, y), app(q, y))
				fragmentMust(t, cause)
				second, cause := manager.MkImplies(backBody, app(p, x))
				fragmentMust(t, cause)
				fragmentMust(t, engine.AddRule(second, "external-interval-cartesian-invalid-second"))
			}

			status, queryErr := engine.QueryPredicate(headDeclaration)
			if status != lbool.Undef || !errors.Is(queryErr, ErrExternalIntervalDatalogFragment) || engine.externalIntervalDatalog != nil || len(callbacks.operations) != 0 {
				t.Fatal("invalid Cartesian rule reached callbacks or activation", status, queryErr, engine.externalIntervalDatalog, callbacks.operations)
			}
		})
	}
}
