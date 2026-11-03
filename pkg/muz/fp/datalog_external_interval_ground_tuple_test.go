package fp

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func addExternalIntervalNaryGroundRule(t *testing.T, manager *ast.ASTManager, engine *PropositionalEngine, head ast.ASTHandle, values []ast.ASTHandle) {
	t.Helper()
	application, err := manager.MkApp(head, values)
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(application, "external-interval-nary-ground"))
}

func TestExternalIntervalDatalogNaryGroundSingletonCacheAndHeadOrder(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, mixed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/mixed=%t", kinds[0], mixed), func(t *testing.T) {
				manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
				family, err := manager.RegisterArithmeticPlugin()
				fragmentMust(t, err)
				integer, err := manager.MkFamilySort(family, 1, nil)
				fragmentMust(t, err)
				real, err := manager.MkFamilySort(family, 0, nil)
				fragmentMust(t, err)
				seven, err := manager.ArithmeticIntegerNumeralText("7")
				fragmentMust(t, err)
				last, err := manager.ArithmeticIntegerNumeralText("9")
				fragmentMust(t, err)
				values := []ast.ASTHandle{seven, seven, last}
				sorts := []ast.ASTHandle{integer, integer, integer}
				if mixed {
					sevenReal, cause := manager.ArithmeticRealNumeralText("7")
					fragmentMust(t, cause)
					values = []ast.ASTHandle{seven, sevenReal, seven}
					sorts = []ast.ASTHandle{integer, real, integer}
				}
				q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-nary-cache", sorts, kinds)
				addExternalIntervalNaryGroundRule(t, manager, engine, q, values)
				selected, err := engine.ensureExternalIntervalDatalog()
				fragmentMust(t, err)
				if !selected {
					t.Fatal("n-ary ground rule was not selected")
				}
				plan := engine.externalIntervalDatalog.ground
				planRoot := plan.applicationRoot
				root, err := manager.RefGet(planRoot)
				fragmentMust(t, err)
				for i, value := range values {
					child, cause := manager.GetArg(root, uint32(i))
					fragmentMust(t, cause)
					if child != value {
						t.Fatal("ground plan did not pin its numeral column", i)
					}
				}
				for run := 0; run < 2; run++ {
					resetExternalIntervalCartesianCallbacks(callbacks)
					status, cause := engine.QueryPredicate(q)
					fragmentMust(t, cause)
					if status != lbool.Undef {
						t.Fatal("n-ary ground result", status)
					}
					var want []int32
					if run != 0 || kinds[0] == "external_relation" {
						want = append(want, ast.DatalogOpIsEmpty)
					}
					if run != 0 {
						want = append(want, ast.DatalogOpClone)
					}
					want = append(want, ast.DatalogOpEmpty, ast.DatalogOpStore)
					if mixed {
						want = append(want, ast.DatalogOpEmpty, ast.DatalogOpStore, ast.DatalogOpJoin, ast.DatalogOpJoin)
					} else {
						want = append(want, ast.DatalogOpJoin, ast.DatalogOpEmpty, ast.DatalogOpStore, ast.DatalogOpJoin)
					}
					if run == 0 {
						want = append(want, ast.DatalogOpEmpty)
					}
					want = append(want, ast.DatalogOpUnion, ast.DatalogOpIsEmpty)
					assertExternalIntervalRegisterOperations(t, callbacks.operations, want)
					if len(callbacks.joinArguments) != 2 || len(callbacks.joinParameters) != 2 || len(callbacks.joinParameters[0]) != 0 || len(callbacks.joinParameters[1]) != 0 {
						t.Fatal("ground tuple JOIN callback shape", callbacks.joinArguments, callbacks.joinParameters)
					}
					first, second := callbacks.joinArguments[0], callbacks.joinArguments[1]
					if mixed {
						if first[0] == first[1] || second[1] != first[0] {
							t.Fatal("ground cache merged Int/Real values or lost later reuse", first, second)
						}
					} else if first[0] != first[1] || second[1] == first[0] {
						t.Fatal("ground cache did not reuse repeated numeral", first, second)
					}
					product := engine.externalIntervalDatalog.entries[q].product
					actualSorts, cause := product.Sorts()
					fragmentMust(t, cause)
					for i, sort := range sorts {
						if actualSorts[i] != sort {
							t.Fatal("ground tuple changed column sort", i)
						}
					}
					var display bytes.Buffer
					fragmentMust(t, product.Display(&display))
					lastValue := "9"
					if mixed {
						lastValue = "7"
					}
					for _, interval := range []string{"0 in [7, 7]", "1 in [7, 7]", "2 in [" + lastValue + ", " + lastValue + "]"} {
						if !strings.Contains(display.String(), interval) {
							t.Fatal("ground tuple changed interval columns", interval, display.String())
						}
					}
					if engine.externalIntervalDatalog.ground != plan || plan.applicationRoot != planRoot || !externalIntervalKindsEqual(product.Kinds(), kinds) {
						t.Fatal("ground query replaced the rooted plan or representation")
					}
				}
			})
		}
	}
}

func TestExternalIntervalDatalogNaryGroundRefusalsRollbackAndRetry(t *testing.T) {
	for _, stop := range []struct {
		op int32
		at int
	}{{ast.DatalogOpEmpty, 1}, {ast.DatalogOpEmpty, 2}, {ast.DatalogOpEmpty, 3}, {ast.DatalogOpStore, 1}, {ast.DatalogOpStore, 2}, {ast.DatalogOpJoin, 1}, {ast.DatalogOpJoin, 2}, {ast.DatalogOpUnion, 1}} {
		t.Run(fmt.Sprintf("operation-%d/%d", stop.op, stop.at), func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			seven, err := manager.ArithmeticIntegerNumeralText("7")
			fragmentMust(t, err)
			nine, err := manager.ArithmeticIntegerNumeralText("9")
			fragmentMust(t, err)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-nary-refusal", []ast.ASTHandle{integer, integer, integer}, []string{"external_relation", "interval_relation"})
			addExternalIntervalNaryGroundRule(t, manager, engine, q, []ast.ASTHandle{seven, seven, nine})
			_, err = engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			old := engine.externalIntervalDatalog.entries[q].product
			refusal := errors.New("n-ary ground callback refused")
			callbacks.operations = nil
			callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = stop.op, stop.at, refusal
			status, err := engine.QueryPredicate(q)
			if status != lbool.Undef || !errors.Is(err, refusal) {
				t.Fatal("n-ary ground refusal not reported", status, err)
			}
			if engine.externalIntervalDatalog.entries[q].product != old {
				t.Fatal("failed n-ary ground published its target")
			}
			callbacks.refusal = nil
			empty, err := old.Empty()
			fragmentMust(t, err)
			if !empty {
				t.Fatal("failed n-ary ground changed stored facts")
			}
			callbacks.operations = nil
			status, err = engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpStore) != 2 || operationCount(callbacks.operations, ast.DatalogOpJoin) != 2 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
				t.Fatal("n-ary ground retry did not reevaluate once", status, callbacks.operations)
			}
		})
	}
}
