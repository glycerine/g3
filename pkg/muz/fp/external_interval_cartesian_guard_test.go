package fp

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// The final two conjunction arguments become the first two normalized
// positive tails in reverse order. Put q before p so JOIN consumes p then q.
func newExternalIntervalCartesianGuardFixture(t *testing.T, kinds []string, head []uint32, leftFacts, rightFacts []string, real bool, guards []externalIntervalUnaryGuardSpec) (*ast.ASTManager, *PropositionalEngine, *externalIntervalCallbackState, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	sortKind := int32(1)
	if real {
		sortKind = 0
	}
	sort, err := manager.MkFamilySort(family, sortKind, nil)
	fragmentMust(t, err)
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			fragmentMust(t, manager.DelRef(&roots[i]))
		}
	}()
	own := func(value ast.ASTHandle, cause error) ast.ASTHandle {
		fragmentMust(t, cause)
		root, cause := manager.NewRef(value)
		fragmentMust(t, cause)
		roots = append(roots, root)
		return value
	}
	numeral := func(value string) ast.ASTHandle {
		if real {
			return own(manager.ArithmeticRealNumeralText(value))
		}
		return own(manager.ArithmeticIntegerNumeralText(value))
	}
	sorts := func(count int) []ast.ASTHandle {
		result := make([]ast.ASTHandle, count)
		for i := range result {
			result[i] = sort
		}
		return result
	}
	p := declareExternalIntervalPredicate(t, manager, engine, "cartesian-guard-p", sorts(3), kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "cartesian-guard-q", sorts(3), kinds)
	h := declareExternalIntervalPredicate(t, manager, engine, "cartesian-guard-h", sorts(len(head)), kinds)
	for _, fact := range []struct {
		predicate ast.ASTHandle
		values    []string
	}{{p, leftFacts}, {q, rightFacts}} {
		if fact.values != nil {
			values := make([]ast.ASTHandle, len(fact.values))
			for column, value := range fact.values {
				values[column] = numeral(value)
			}
			fragmentMust(t, engine.AddFact(fact.predicate, values))
		}
	}
	application := func(predicate ast.ASTHandle, variables []uint32) ast.ASTHandle {
		arguments := make([]ast.ASTHandle, len(variables))
		for i, variable := range variables {
			arguments[i] = own(manager.MkVar(variable, sort))
		}
		return own(manager.MkApp(predicate, arguments))
	}
	var conditions []ast.ASTHandle
	for _, guard := range guards {
		left := own(manager.MkVar(guard.left, sort))
		var right ast.ASTHandle
		if guard.bound != "" {
			right = numeral(guard.bound)
		} else {
			right = own(manager.MkVar(guard.right, sort))
		}
		switch guard.op {
		case "<":
			conditions = append(conditions, own(manager.MkArithmeticLT(left, right)))
		case ">=":
			conditions = append(conditions, own(manager.MkArithmeticGE(left, right)))
		default:
			t.Fatal("unknown test guard", guard.op)
		}
	}
	// p(x,k,u), q(k,z,y): k joins, u is unused, z may occur only in a guard.
	conditions = append(conditions, application(q, []uint32{1, 3, 2}), application(p, []uint32{0, 1, 4}))
	body := own(manager.MkAndArgs(conditions))
	formula := own(manager.MkImplies(body, application(h, head)))
	fragmentMust(t, engine.AddRule(formula, "cartesian-guard-rule"))
	return manager, engine, callbacks, p, q, h, sort
}

func externalIntervalCartesianGuardOperations(operations []int32) []int32 {
	var result []int32
	for _, operation := range operations {
		switch operation {
		case ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpUnion:
			result = append(result, operation)
		}
	}
	return result
}

func TestExternalIntervalDatalogCartesianGuardBoundComparison(t *testing.T) {
	_, engine, callbacks, p, q, h, _ := newExternalIntervalCartesianGuardFixture(t,
		[]string{"external_relation", "interval_relation"}, []uint32{2, 0}, []string{"1", "7", "99"}, []string{"7", "3", "5"}, false,
		[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 2}})
	resetExternalIntervalUnaryGuardCallbacks(callbacks)
	status, err := engine.QueryPredicate(h)
	fragmentMust(t, err)
	if status != lbool.Undef || !reflect.DeepEqual(callbacks.guardConditions, []string{"(< v0 v1)"}) || !reflect.DeepEqual(callbacks.projectParameters, [][]uint32{{1, 2, 3, 4}}) {
		t.Fatal("head-bound guard mapping", status, callbacks.guardConditions, callbacks.projectParameters)
	}
	if plan := engine.externalIntervalDatalog.cartesian; plan.left != p || plan.right != q {
		t.Fatal("normalized positive-tail order changed", plan)
	}
	assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), []int32{
		ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject,
		ast.DatalogOpFilter, ast.DatalogOpRename, ast.DatalogOpUnion,
	})
}

func TestExternalIntervalDatalogCartesianGuardOriginalNoKeyShape(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, contradiction := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/contradiction=%t", kinds[0], contradiction), func(t *testing.T) {
				manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
				family, err := manager.RegisterArithmeticPlugin()
				fragmentMust(t, err)
				integer, err := manager.MkFamilySort(family, 1, nil)
				fragmentMust(t, err)
				p := declareExternalIntervalPredicate(t, manager, engine, "cartesian-no-key-guard-p", []ast.ASTHandle{integer}, kinds)
				q := declareExternalIntervalPredicate(t, manager, engine, "cartesian-no-key-guard-q", []ast.ASTHandle{integer}, kinds)
				h := declareExternalIntervalPredicate(t, manager, engine, "cartesian-no-key-guard-h", []ast.ASTHandle{integer, integer}, kinds)
				leftValue, rightValue := "1", "3"
				if contradiction {
					leftValue, rightValue = rightValue, leftValue
				}
				for _, fact := range []struct {
					predicate ast.ASTHandle
					value     string
				}{{p, leftValue}, {q, rightValue}} {
					value, cause := manager.ArithmeticIntegerNumeralText(fact.value)
					fragmentMust(t, cause)
					fragmentMust(t, engine.AddFact(fact.predicate, []ast.ASTHandle{value}))
				}
				x, err := manager.MkVar(0, integer)
				fragmentMust(t, err)
				y, err := manager.MkVar(1, integer)
				fragmentMust(t, err)
				left, err := manager.MkApp(p, []ast.ASTHandle{x})
				fragmentMust(t, err)
				right, err := manager.MkApp(q, []ast.ASTHandle{y})
				fragmentMust(t, err)
				body, err := manager.MkAnd(left, right)
				fragmentMust(t, err)
				guard, err := manager.MkArithmeticLT(x, y)
				fragmentMust(t, err)
				body, err = manager.MkAnd(body, guard)
				fragmentMust(t, err)
				head, err := manager.MkApp(h, []ast.ASTHandle{x, y})
				fragmentMust(t, err)
				formula, err := manager.MkImplies(body, head)
				fragmentMust(t, err)
				fragmentMust(t, engine.AddRule(formula, "cartesian-no-key-guard-rule"))
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err := engine.QueryPredicate(h)
				fragmentMust(t, err)
				plan := engine.externalIntervalDatalog.cartesian
				// HNF flattens the nested conjunction, then extract_horn
				// flattens its rebuilt body; these source passes leave p,q.
				if plan.left != p || plan.right != q || len(callbacks.joinParameters) != 1 || len(callbacks.joinParameters[0]) != 0 || len(callbacks.projectParameters) != 0 || len(callbacks.renameParameters) != 0 ||
					!reflect.DeepEqual(callbacks.guardConditions, []string{"(< v0 v1)"}) {
					t.Fatal("original no-key guard source operands", plan, callbacks.joinParameters, callbacks.projectParameters, callbacks.guardConditions)
				}
				wantStatus := lbool.Undef
				wantOps := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpFilter}
				if contradiction {
					wantStatus = lbool.False
				} else {
					wantOps = append(wantOps, ast.DatalogOpUnion)
					assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 0, "1", "1")
					assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 1, "3", "3")
				}
				if status != wantStatus {
					t.Fatal("original no-key guard result", status, wantStatus)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), wantOps)
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, 0, leftValue, leftValue)
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, 0, rightValue, rightValue)
			})
		}
	}
}

func TestExternalIntervalDatalogCartesianGuardColumnsAndRepeat(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, real := range []bool{false, true} {
			for _, shape := range []string{"head-bound", "guard-only", "shared-key", "nullary-head"} {
				t.Run(fmt.Sprintf("%s/real=%t/%s", kinds[0], real, shape), func(t *testing.T) {
					x, z, y := "1", "3", "5"
					if real {
						x, z, y = "1/2", "3/2", "5/2"
					}
					head := []uint32{2, 0}
					guard := externalIntervalUnaryGuardSpec{op: "<", left: 0, right: 2}
					condition := "(< v0 v1)"
					projects, renames := [][]uint32{{1, 2, 3, 4}}, [][]uint32{{0, 1}}
					wantValues := []string{y, x}
					fused := false
					switch shape {
					case "guard-only":
						guard.right, fused = 3, true
						projects = [][]uint32{{1, 2, 3}, {1}}
					case "shared-key":
						guard.left, guard.right, condition, fused = 0, 1, "(< v0 v1)", true
						projects = [][]uint32{{1, 2, 4}, {1}}
					case "nullary-head":
						head, wantValues, renames, fused = nil, nil, nil, true
						guard.right, projects = 3, [][]uint32{{1, 2, 3, 5}, {0, 1}}
					}
					leftFacts, rightFacts := []string{x, "7", "99"}, []string{"7", z, y}
					_, engine, callbacks, p, q, h, _ := newExternalIntervalCartesianGuardFixture(t, kinds, head, leftFacts, rightFacts, real, []externalIntervalUnaryGuardSpec{guard})
					for run := 0; run < 2; run++ {
						resetExternalIntervalUnaryGuardCallbacks(callbacks)
						status, err := engine.QueryPredicate(h)
						fragmentMust(t, err)
						if status != lbool.Undef || !reflect.DeepEqual(callbacks.guardConditions, []string{condition}) || !reflect.DeepEqual(callbacks.joinParameters, [][]uint32{{1, 0}}) ||
							!reflect.DeepEqual(callbacks.projectParameters, projects) || !reflect.DeepEqual(callbacks.renameParameters, renames) {
							t.Fatal("two-tail guard source columns", status, callbacks.guardConditions, callbacks.joinParameters, callbacks.projectParameters, callbacks.renameParameters)
						}
						want := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject}
						if run > 0 {
							want = append([]int32{ast.DatalogOpClone}, want...)
						}
						if fused {
							want = append(want, ast.DatalogOpClone)
						}
						want = append(want, ast.DatalogOpFilter)
						if fused {
							want = append(want, ast.DatalogOpProject)
						}
						if renames != nil {
							want = append(want, ast.DatalogOpRename)
						}
						want = append(want, ast.DatalogOpUnion)
						assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), want)
						for column, value := range wantValues {
							assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, column, value, value)
						}
						for column, value := range leftFacts {
							assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
						}
						for column, value := range rightFacts {
							assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, column, value, value)
						}
					}
				})
			}
		}
	}
}

func TestExternalIntervalDatalogCartesianGuardEmptyFrontiers(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, shape := range []string{"empty-source", "disjoint-join", "in-place-contradiction", "fused-contradiction"} {
			t.Run(kinds[0]+"/"+shape, func(t *testing.T) {
				left, right := []string{"5", "7", "99"}, []string{"7", "3", "1"}
				guard := externalIntervalUnaryGuardSpec{op: "<", left: 0, right: 2}
				want := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject}
				switch shape {
				case "empty-source":
					right, want = nil, []int32{ast.DatalogOpClone}
				case "disjoint-join":
					right[0] = "8"
				case "in-place-contradiction":
					want = append(want, ast.DatalogOpFilter)
				case "fused-contradiction":
					guard.right = 3
					want = append(want, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject)
				}
				_, engine, callbacks, p, _, h, _ := newExternalIntervalCartesianGuardFixture(t, kinds, []uint32{2, 0}, left, right, false, []externalIntervalUnaryGuardSpec{guard})
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err := engine.QueryPredicate(h)
				fragmentMust(t, err)
				wantStatus := lbool.False
				if shape == "fused-contradiction" {
					wantStatus = lbool.Undef
				}
				if status != wantStatus {
					t.Fatal("two-tail guard result", status, wantStatus)
				}
				if shape == "disjoint-join" {
					want = append(want, ast.DatalogOpFilter)
				} else if shape == "fused-contradiction" {
					want = append(want, ast.DatalogOpRename, ast.DatalogOpUnion)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), want)
				if shape == "fused-contradiction" && !reflect.DeepEqual(callbacks.projectParameters, [][]uint32{{1, 2, 3}, {1}}) {
					t.Fatal("empty fused filter skipped its projection", callbacks.projectParameters)
				}
				for column, value := range left {
					assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
				}
			})
		}
	}
}

func TestExternalIntervalDatalogCartesianGuardOrderedConjunctionStaysCoarse(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, firstFalse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/first-false=%t", kinds[0], firstFalse), func(t *testing.T) {
				x, z := "0", "1"
				if firstFalse {
					x, z = "3", "2"
				}
				_, engine, callbacks, _, _, h, _ := newExternalIntervalCartesianGuardFixture(t, kinds, []uint32{2, 0}, []string{x, "7", "99"}, []string{"7", z, "5"}, false,
					[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 3}, {op: ">=", left: 3, bound: "2"}})
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err := engine.QueryPredicate(h)
				fragmentMust(t, err)
				// The source interval operation does not recognize AND, so a
				// false child cannot be promoted to an independent interval filter.
				if status != lbool.Undef || !reflect.DeepEqual(callbacks.guardConditions, []string{"(and (>= v1 2) (< v0 v1))"}) {
					t.Fatal("two-tail conjunction was reordered or strengthened", status, callbacks.guardConditions)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), []int32{
					ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject,
					ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpUnion,
				})
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 1, x, x)
			})
		}
	}
}

func TestExternalIntervalDatalogCartesianGuardProgramRefusalAndRetry(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, stop := range []struct {
			name string
			op   int32
			at   int
		}{{"guard", ast.DatalogOpFilter, 1}, {"fused-project", ast.DatalogOpProject, 2}} {
			t.Run(kinds[0]+"/"+stop.name, func(t *testing.T) {
				manager, engine, callbacks, p, q, h, sort := newExternalIntervalCartesianGuardFixture(t, kinds, []uint32{2, 0}, []string{"1", "7", "99"}, []string{"7", "3", "5"}, false,
					[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 3}})
				r := declareExternalIntervalPredicate(t, manager, engine, "cartesian-guard-consumer", []ast.ASTHandle{sort, sort}, kinds)
				addExternalIntervalIdentityRule(t, manager, engine, h, r, []ast.ASTHandle{sort, sort}, "cartesian-guard-consumer-rule")
				_, err := engine.ensureExternalIntervalDatalog()
				fragmentMust(t, err)
				activation := engine.externalIntervalDatalog
				oldP, oldQ, oldH, oldR := activation.entries[p].product, activation.entries[q].product, activation.entries[h].product, activation.entries[r].product
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				refusal := errors.New("two-tail guard callback refused")
				callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = stop.op, stop.at, refusal
				status, err := engine.QueryPredicate(r)
				if status != lbool.Undef || !errors.Is(err, refusal) || activation.entries[p].product != oldP || activation.entries[q].product != oldQ || activation.entries[h].product != oldH || activation.entries[r].product != oldR || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
					t.Fatal("two-tail guard refusal published partial registers", status, err, callbacks.operations)
				}
				callbacks.refusal = nil
				for _, predicate := range []ast.ASTHandle{h, r} {
					empty, cause := activation.entries[predicate].product.Empty()
					fragmentMust(t, cause)
					if !empty {
						t.Fatal("failed two-tail program changed a derived relation")
					}
				}
				assertExternalIntervalGroundProgramInterval(t, oldP, 0, "1", "1")
				assertExternalIntervalGroundProgramInterval(t, oldQ, 1, "3", "3")
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err = engine.QueryPredicate(r)
				fragmentMust(t, err)
				if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpUnion) != 2 {
					t.Fatal("two-tail program retry", status, callbacks.operations)
				}
				assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, 0, "5", "5")
				assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, 1, "1", "1")
			})
		}
	}
}

func TestExternalIntervalDatalogCartesianGuardMappedCallbackSMTEquivalence(t *testing.T) {
	manager, engine, callbacks, _, _, h, sort := newExternalIntervalCartesianGuardFixture(t,
		[]string{"external_relation", "interval_relation"}, []uint32{2, 0}, []string{"1", "7", "99"}, []string{"7", "3", "5"}, false,
		[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 3}})
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			fragmentMust(t, manager.DelRef(&roots[i]))
		}
	}()
	own := func(value ast.ASTHandle, cause error) ast.ASTHandle {
		fragmentMust(t, cause)
		root, cause := manager.NewRef(value)
		fragmentMust(t, cause)
		roots = append(roots, root)
		return value
	}
	var observed ast.ASTHandle
	installed := callbacks.callbacks()
	assign := installed.ReduceAssign
	installed.ReduceAssign = func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) (result error) {
		kind, err := manager.GetDeclKind(declaration)
		if err != nil {
			return err
		}
		if kind == ast.DatalogOpFilter {
			parameters, err := manager.GetDeclParametersCopy(declaration)
			if err != nil {
				return err
			}
			defer func() {
				for i := range parameters {
					result = errors.Join(result, manager.DelParameter(&parameters[i]))
				}
			}()
			condition, err := manager.ParameterAST(parameters[0])
			if err != nil {
				return err
			}
			observed = own(condition, nil)
		}
		return assign(declaration, arguments, outputs)
	}
	fragmentMust(t, engine.SetExternalRelationCallbacks(installed))
	status, err := engine.QueryPredicate(h)
	fragmentMust(t, err)
	if status != lbool.Undef || observed.IsNull() {
		t.Fatal("missing actual two-tail FILTER condition", status)
	}
	// JOIN_PROJECT removes k,u,k, leaving x,z,y; the guard is x<z.
	bindings := make([]ast.ASTHandle, 3)
	for column := range bindings {
		bindings[column] = own(manager.MkFreshConst(fmt.Sprintf("cartesian-guard-column-%d", column), sort))
	}
	substitution, err := astrewriter.NewVarSubstitution(manager, false)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, substitution.Close()) }()
	bound, err := manager.NewRef(ast.ASTHandle{})
	fragmentMust(t, err)
	roots = append(roots, bound)
	fragmentMust(t, substitution.Substitute(observed, bindings, bound))
	actual, err := manager.RefGet(bound)
	fragmentMust(t, err)
	expected := own(manager.MkArithmeticLT(bindings[0], bindings[1]))
	equivalent := own(manager.MkEq(actual, expected))
	different := own(manager.MkNot(equivalent))
	solver, err := smtcore.NewSMTSolver(manager, nil, "", smtcore.SMTSolverConfig{Params: engine.params, Globals: engine.globals})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, solver.Close()) }()
	fragmentMust(t, solver.AssertFormula(different))
	checked, err := solver.CheckSat(nil)
	fragmentMust(t, err)
	if checked != lbool.False {
		t.Fatal("actual FILTER condition differs from post-JOIN_PROJECT guard semantics", checked)
	}
}
