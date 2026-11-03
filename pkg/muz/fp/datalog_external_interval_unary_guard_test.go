package fp

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func describeExternalIntervalGuard(manager *ast.ASTManager, expression ast.ASTHandle) (string, error) {
	kind, err := manager.GetKind(expression)
	if err != nil {
		return "", err
	}
	if kind == ast.ASTVar {
		index, cause := manager.GetVarIndex(expression)
		return fmt.Sprintf("v%d", index), cause
	}
	value, _, matched, err := manager.ArithmeticNumeralText(expression)
	if err != nil || matched {
		return value, err
	}
	declaration, err := manager.GetDecl(expression)
	if err != nil {
		return "", err
	}
	name, err := manager.GetASTName(declaration)
	if err != nil {
		return "", err
	}
	text, err := manager.Symbols.String(name)
	if err != nil {
		return "", err
	}
	count, err := manager.GetNumArgs(expression)
	if err != nil {
		return "", err
	}
	parts := []string{text}
	for column := uint32(0); column < count; column++ {
		argument, cause := manager.GetArg(expression, column)
		if cause != nil {
			return "", cause
		}
		child, cause := describeExternalIntervalGuard(manager, argument)
		if cause != nil {
			return "", cause
		}
		parts = append(parts, child)
	}
	return "(" + strings.Join(parts, " ") + ")", nil
}

type externalIntervalUnaryGuardSpec struct {
	op          string
	left, right uint32
	bound       string
}

func newExternalIntervalUnaryGuardFixture(t *testing.T, kinds []string, tail []int, head []uint32, facts []string, real, negated bool, guards []externalIntervalUnaryGuardSpec) (*ast.ASTManager, *PropositionalEngine, *externalIntervalCallbackState, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
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
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-guard-p", sorts(len(tail)), kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-guard-q", sorts(len(head)), kinds)
	if facts != nil {
		values := make([]ast.ASTHandle, len(facts))
		for column, value := range facts {
			values[column] = numeral(value)
		}
		fragmentMust(t, engine.AddFact(p, values))
	}
	arguments := make([]ast.ASTHandle, len(tail))
	for column, variable := range tail {
		if variable < 0 {
			arguments[column] = numeral("7")
		} else {
			arguments[column] = own(manager.MkVar(uint32(variable), sort))
		}
	}
	predicate := own(manager.MkApp(p, arguments))
	if negated {
		predicate = own(manager.MkNot(predicate))
	}
	conditions := []ast.ASTHandle{predicate}
	for _, guard := range guards {
		left := own(manager.MkVar(guard.left, sort))
		var right ast.ASTHandle
		if guard.bound != "" {
			right = numeral(guard.bound)
		} else {
			right = own(manager.MkVar(guard.right, sort))
		}
		var condition ast.ASTHandle
		switch guard.op {
		case "<":
			condition = own(manager.MkArithmeticLT(left, right))
		case ">=":
			condition = own(manager.MkArithmeticGE(left, right))
		default:
			t.Fatal("unknown test guard", guard.op)
		}
		conditions = append(conditions, condition)
	}
	body := own(manager.MkAndArgs(conditions))
	headArguments := make([]ast.ASTHandle, len(head))
	for column, variable := range head {
		headArguments[column] = own(manager.MkVar(variable, sort))
	}
	conclusion := own(manager.MkApp(q, headArguments))
	formula := own(manager.MkImplies(body, conclusion))
	fragmentMust(t, engine.AddRule(formula, "external-interval-guard-rule"))
	return manager, engine, callbacks, p, q, sort
}

func resetExternalIntervalUnaryGuardCallbacks(callbacks *externalIntervalCallbackState) {
	resetExternalIntervalUnarySelectCallbacks(callbacks)
	callbacks.guardConditions = nil
}

func TestExternalIntervalDatalogUnaryGuardBoundComparison(t *testing.T) {
	_, engine, callbacks, _, q, _ := newExternalIntervalUnaryGuardFixture(t,
		[]string{"external_relation", "interval_relation"}, []int{0}, []uint32{0}, []string{"2"}, false, false,
		[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, bound: "3"}})
	resetExternalIntervalUnaryGuardCallbacks(callbacks)
	status, err := engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef || len(callbacks.guardConditions) != 1 || callbacks.guardConditions[0] != "(< v0 3)" {
		t.Fatal("bound guard", status, callbacks.guardConditions)
	}
	assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
		ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpUnion,
	})
}

func TestExternalIntervalDatalogUnaryGuardMappingOwnershipAndRepeat(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, real := range []bool{false, true} {
			for _, shape := range []string{"aligned", "swap", "repeated", "selected-repeated"} {
				t.Run(fmt.Sprintf("%s/real=%t/%s", kinds[0], real, shape), func(t *testing.T) {
					x, y := "1", "3"
					if real {
						x, y = "1/2", "3/2"
					}
					tail, facts := []int{0, 1}, []string{x, y}
					wantCondition := "(< v0 v1)"
					var projects, identical, renames [][]uint32
					switch shape {
					case "swap":
						tail, facts = []int{1, 0}, []string{y, x}
						wantCondition = "(< v1 v0)"
						renames = [][]uint32{{0, 1}}
					case "repeated", "selected-repeated":
						tail, facts = []int{1, 0, 1, 0}, []string{y, x, y, x}
						wantCondition = "(< v3 v2)"
						identical = [][]uint32{{1, 3}, {0, 2}}
						projects, renames = [][]uint32{{0, 1}}, [][]uint32{{0, 1}}
						if shape == "selected-repeated" {
							tail, facts = append([]int{-1}, tail...), append([]string{"7"}, facts...)
							projects = append([][]uint32{{0}}, projects...)
						}
					}
					_, engine, callbacks, p, q, _ := newExternalIntervalUnaryGuardFixture(t, kinds, tail, []uint32{0, 1}, facts, real, false, []externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}})
					for run := 0; run < 2; run++ {
						resetExternalIntervalUnaryGuardCallbacks(callbacks)
						status, err := engine.QueryPredicate(q)
						fragmentMust(t, err)
						if status != lbool.Undef || !reflect.DeepEqual(callbacks.guardConditions, []string{wantCondition}) || !reflect.DeepEqual(callbacks.filterColumns, identical) ||
							!reflect.DeepEqual(callbacks.projectParameters, projects) || !reflect.DeepEqual(callbacks.renameParameters, renames) {
							t.Fatal("guard binding or head order", status, callbacks.guardConditions, callbacks.filterColumns, callbacks.projectParameters, callbacks.renameParameters)
						}
						want := []int32{ast.DatalogOpClone}
						if run != 0 {
							want = append(want, ast.DatalogOpClone)
						}
						want = append(want, ast.DatalogOpClone)
						if shape == "selected-repeated" {
							want = append(want, ast.DatalogOpFilter, ast.DatalogOpProject)
						}
						for range identical {
							want = append(want, ast.DatalogOpFilter)
						}
						want = append(want, ast.DatalogOpFilter)
						if len(identical) != 0 {
							want = append(want, ast.DatalogOpProject)
						}
						for range renames {
							want = append(want, ast.DatalogOpRename)
						}
						want = append(want, ast.DatalogOpUnion)
						assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), want)
						assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, 0, x, x)
						assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, 1, y, y)
						for column, value := range facts {
							assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
						}
					}
				})
			}
		}
	}
}

func TestExternalIntervalDatalogUnaryGuardsUseOneOrderedConjunction(t *testing.T) {
	_, engine, callbacks, _, q, _ := newExternalIntervalUnaryGuardFixture(t,
		[]string{"external_relation", "interval_relation"}, []int{0, 1}, []uint32{0, 1}, []string{"3", "3"}, false, false,
		[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, bound: "2"}, {op: ">=", left: 1, bound: "4"}})
	resetExternalIntervalUnaryGuardCallbacks(callbacks)
	status, err := engine.QueryPredicate(q)
	fragmentMust(t, err)
	// flatten_and moves the last child to the front before rule_manager::mk
	// fills interpreted tails downward, yielding this exact guard order. The
	// interval plugin ignores the conjunction; splitting its false children
	// would change the source approximation and wrongly report emptiness.
	if status != lbool.Undef || !reflect.DeepEqual(callbacks.guardConditions, []string{"(and (< v0 2) (>= v1 4))"}) {
		t.Fatal("guard conjunction or rule-tail order", status, callbacks.guardConditions)
	}
	assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
		ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpUnion,
	})
}

func TestExternalIntervalDatalogUnaryGuardContradictionSkipsHeadAssembly(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			_, engine, callbacks, p, q, _ := newExternalIntervalUnaryGuardFixture(t, kinds,
				[]int{1, 0, 1, 0}, []uint32{0, 1}, []string{"1", "3", "1", "3"}, false, false,
				[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}})
			resetExternalIntervalUnaryGuardCallbacks(callbacks)
			status, err := engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.False || !reflect.DeepEqual(callbacks.guardConditions, []string{"(< v3 v2)"}) {
				t.Fatal("contradictory bound guard", status, callbacks.guardConditions)
			}
			assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
				ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpFilter, ast.DatalogOpFilter,
			})
			for column, value := range []string{"1", "3", "1", "3"} {
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
			}
		})
	}
}

func TestExternalIntervalDatalogUnaryGuardRefusalAndProgramRetry(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			manager, engine, callbacks, p, q, sort := newExternalIntervalUnaryGuardFixture(t, kinds, []int{1, 0}, []uint32{0, 1}, []string{"3", "1"}, false, false,
				[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}})
			r := declareExternalIntervalPredicate(t, manager, engine, "external-interval-guard-consumer", []ast.ASTHandle{sort, sort}, kinds)
			addExternalIntervalIdentityRule(t, manager, engine, q, r, []ast.ASTHandle{sort, sort}, "external-interval-guard-consumer-rule")
			_, err := engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			activation := engine.externalIntervalDatalog
			oldP, oldQ, oldR := activation.entries[p].product, activation.entries[q].product, activation.entries[r].product
			resetExternalIntervalUnaryGuardCallbacks(callbacks)
			refusal := errors.New("interpreted FILTER refused")
			callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = ast.DatalogOpFilter, 1, refusal
			status, err := engine.QueryPredicate(r)
			if status != lbool.Undef || !errors.Is(err, refusal) || activation.entries[p].product != oldP || activation.entries[q].product != oldQ || activation.entries[r].product != oldR {
				t.Fatal("guard refusal published partial program", status, err)
			}
			assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter})
			callbacks.refusal = nil
			resetExternalIntervalUnaryGuardCallbacks(callbacks)
			status, err = engine.QueryPredicate(r)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("guard program retry", status)
			}
			assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
				ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpRename, ast.DatalogOpUnion, ast.DatalogOpUnion,
			})
			assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, 0, "1", "1")
			assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, 1, "3", "3")
			assertExternalIntervalGroundProgramInterval(t, activation.entries[p].product, 0, "3", "3")
		})
	}
}

func TestExternalIntervalDatalogUnaryGuardRejectsUnsupportedBeforeCallbacks(t *testing.T) {
	for _, shape := range []string{"unbound-guard", "negative-tail"} {
		t.Run(shape, func(t *testing.T) {
			tail, head := []int{0}, []uint32{0}
			guard := externalIntervalUnaryGuardSpec{op: "<", left: 0, bound: "3"}
			switch shape {
			case "unbound-guard":
				guard.left = 1
			}
			_, engine, callbacks, _, q, _ := newExternalIntervalUnaryGuardFixture(t,
				[]string{"external_relation", "interval_relation"}, tail, head, nil, false, shape == "negative-tail", []externalIntervalUnaryGuardSpec{guard})
			status, err := engine.QueryPredicate(q)
			if status != lbool.Undef || !errors.Is(err, ErrExternalIntervalDatalogFragment) || engine.externalIntervalDatalog != nil || len(callbacks.operations) != 0 {
				t.Fatal("unsupported guard reached activation or callbacks", status, err, callbacks.operations)
			}
		})
	}
}

func TestExternalIntervalDatalogUnaryGuardMappedCallbackSMTEquivalence(t *testing.T) {
	manager, engine, callbacks, _, q, sort := newExternalIntervalUnaryGuardFixture(t,
		[]string{"external_relation", "interval_relation"}, []int{1, 0, 1, 0}, []uint32{0, 1}, []string{"3", "1", "3", "1"}, false, false,
		[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}})
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
	status, err := engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef || observed.IsNull() {
		t.Fatal("missing mapped FILTER condition", status)
	}
	bindings := make([]ast.ASTHandle, 4)
	for column := range bindings {
		bindings[column] = own(manager.MkFreshConst(fmt.Sprintf("guard-column-%d", column), sort))
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
	expected := own(manager.MkArithmeticLT(bindings[3], bindings[2]))
	equivalent := own(manager.MkEq(actual, expected))
	different := own(manager.MkNot(equivalent))
	solver, err := smtcore.NewSMTSolver(manager, nil, "", smtcore.SMTSolverConfig{Params: engine.params, Globals: engine.globals})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, solver.Close()) }()
	fragmentMust(t, solver.AssertFormula(different))
	checked, err := solver.CheckSat(nil)
	fragmentMust(t, err)
	if checked != lbool.False {
		t.Fatal("actual FILTER callback differs from last-occurrence guard semantics", checked)
	}
}
