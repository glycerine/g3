package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalIntervalProductCallbacksBoundsAndOwnership(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			f := newTableRelationFixture(t)
			family, err := f.ast.RegisterArithmeticPlugin()
			astTestMust(t, err)
			realSort, err := f.ast.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			one, err := f.ast.ArithmeticRealNumeralText("1")
			astTestMust(t, err)
			three, err := f.ast.ArithmeticRealNumeralText("3")
			astTestMust(t, err)
			five, err := f.ast.ArithmeticRealNumeralText("5")
			astTestMust(t, err)
			for _, value := range []ast.ASTHandle{one, three, five} {
				astTestPin(t, f.ast, value)
			}

			nonempty := make(map[ast.ASTHandle]bool)
			emptyCalls, isEmptyCalls, stores, clones := 0, 0, 0, 0
			storeRefusal := errors.New("mixed product STORE refused")
			failStore := false
			plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
				ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
					op, cause := f.ast.GetDeclKind(declaration)
					if cause != nil {
						return cause
					}
					switch op {
					case ast.DatalogOpEmpty:
						emptyCalls++
						nonempty[outputs[0]] = false
					case ast.DatalogOpStore:
						stores++
						if failStore {
							return storeRefusal
						}
						nonempty[outputs[0]] = true
					case ast.DatalogOpClone:
						clones++
						nonempty[outputs[0]] = nonempty[arguments[0]]
					default:
						t.Fatalf("unexpected product callback %d", op)
					}
					return nil
				},
				Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
					op, cause := f.ast.GetDeclKind(declaration)
					if cause != nil {
						return ast.ASTHandle{}, cause
					}
					if op != ast.DatalogOpIsEmpty {
						t.Fatalf("unexpected product reduction %d", op)
					}
					isEmptyCalls++
					if nonempty[arguments[0]] {
						return f.ast.MkFalse()
					}
					return f.ast.MkTrue()
				},
			})
			astTestMust(t, err)
			product, selected, err := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{realSort, realSort}, kinds)
			astTestMust(t, err)
			if !selected || product == nil || product.IsPrecise() || emptyCalls != 1 || plugin.users != 1 {
				t.Fatal("mixed product selection/precision/ownership", selected, product, emptyCalls, plugin.users)
			}
			gotKinds, gotSorts := product.Kinds(), []ast.ASTHandle(nil)
			gotSorts, err = product.Sorts()
			astTestMust(t, err)
			if len(gotKinds) != 2 || gotKinds[0] != kinds[0] || gotKinds[1] != kinds[1] || len(gotSorts) != 2 || gotSorts[0] != realSort || gotSorts[1] != realSort {
				t.Fatal("mixed product lost selection order/signature", gotKinds, gotSorts)
			}
			beforeEmptyCalls := isEmptyCalls
			empty, err := product.Empty()
			astTestMust(t, err)
			if !empty || isEmptyCalls-beforeEmptyCalls != map[bool]int{true: 1, false: 0}[kinds[0] == "external_relation"] {
				t.Fatal("mixed product empty short-circuit order", empty, isEmptyCalls-beforeEmptyCalls)
			}
			astTestMust(t, product.AddFact([]ast.ASTHandle{one, three}))
			empty, err = product.Empty()
			astTestMust(t, err)
			if empty || stores != 1 {
				t.Fatal("STORE did not update both product components", empty, stores)
			}
			assertBound := func(p *ExternalIntervalProduct, column uint32, want string) {
				t.Helper()
				lo, loOpen, hi, hiOpen, cause := p.interval.Bounds(column)
				astTestMust(t, cause)
				if lo == nil || hi == nil || lo.RatString() != want || hi.RatString() != want || loOpen || hiOpen {
					t.Fatal("interval component bound", column, lo, loOpen, hi, hiOpen, want)
				}
			}
			assertBound(product, 0, "1")
			assertBound(product, 1, "3")
			clone, err := product.Clone()
			astTestMust(t, err)
			if clones != 1 || plugin.users != 2 {
				t.Fatal("clone did not own independent components", clones, plugin.users)
			}
			failStore = true
			if err = product.AddFact([]ast.ASTHandle{five, five}); !errors.Is(err, storeRefusal) {
				t.Fatal("STORE refusal not propagated", err)
			}
			assertBound(product, 0, "1")
			assertBound(product, 1, "3")
			failStore = false
			astTestMust(t, product.AddFact([]ast.ASTHandle{five, five}))
			_, _, upper, _, err := product.interval.Bounds(0)
			astTestMust(t, err)
			if upper == nil || upper.RatString() != "5" {
				t.Fatal("interval component did not publish successful update", upper)
			}
			assertBound(clone, 0, "1")
			astTestMust(t, product.Close())
			if plugin.users != 1 {
				t.Fatal("source close consumed clone", plugin.users)
			}
			if empty, err = clone.Empty(); err != nil || empty {
				t.Fatal("clone did not survive source close", empty, err)
			}
			astTestMust(t, clone.Close())
			if plugin.users != 0 {
				t.Fatal("clone leaked external component", plugin.users)
			}
			astTestMust(t, plugin.Close())
		})
	}
}

func TestExternalIntervalProductSelectionAndConstructionRefusal(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integerSort, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	refusal := errors.New("mixed product EMPTY refused")
	failEmpty := true
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{ReduceAssign: func(declaration ast.ASTHandle, _, _ []ast.ASTHandle) error {
		op, cause := f.ast.GetDeclKind(declaration)
		if cause != nil {
			return cause
		}
		if failEmpty && op == ast.DatalogOpEmpty {
			return refusal
		}
		return nil
	}})
	astTestMust(t, err)
	if p, selected, err := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{integerSort}, []string{"external_relation"}); p != nil || selected || err != nil {
		t.Fatal("non-product selection intercepted", p, selected, err)
	}
	if p, selected, err := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{f.boolean}, []string{"external_relation", "interval_relation"}); p != nil || !selected || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-arithmetic mixed product accepted", p, selected, err)
	}
	p, selected, err := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{integerSort}, []string{"interval_relation", "external_relation"})
	if p != nil || !selected || !errors.Is(err, refusal) || plugin.users != 0 {
		t.Fatal("constructor refusal published prefix", p, selected, err, plugin.users)
	}
	failEmpty = false
	nullary, selected, err := NewSelectedExternalIntervalProduct(plugin, nil, []string{"external_relation", "interval_relation"})
	astTestMust(t, err)
	if !selected || nullary == nil {
		t.Fatal("nullary product not selected")
	}
	astTestMust(t, nullary.AddFact(nil))
	empty, err := nullary.Empty()
	astTestMust(t, err)
	if empty {
		t.Fatal("nullary product fact remained empty")
	}
	astTestMust(t, nullary.Close())
	astTestMust(t, plugin.Close())
}
