package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalIntervalProductJoinCallbacksSemanticsAndRollback(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := f.ast.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	integerSort, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	astTestPin(t, f.ast, realSort)
	astTestPin(t, f.ast, integerSort)
	one, err := f.ast.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	three, err := f.ast.ArithmeticRealNumeralText("3")
	astTestMust(t, err)
	astTestPin(t, f.ast, one)
	astTestPin(t, f.ast, three)

	type joinEvent struct {
		inputs     int
		parameters []uint32
	}
	var events []joinEvent
	refusal := errors.New("mixed product JOIN refused")
	failJoin := false
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
		op, cause := f.ast.GetDeclKind(declaration)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		if op != ast.DatalogOpJoin {
			return ast.ASTHandle{}, nil
		}
		parameters, cause := f.ast.GetDeclParametersCopy(declaration)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		event := joinEvent{inputs: len(arguments)}
		for i := range parameters {
			value, parameterErr := f.ast.ParameterInt(parameters[i])
			astTestMust(t, f.ast.DelParameter(&parameters[i]))
			if parameterErr != nil {
				return ast.ASTHandle{}, parameterErr
			}
			event.parameters = append(event.parameters, uint32(value))
		}
		events = append(events, event)
		if failJoin {
			return ast.ASTHandle{}, refusal
		}
		return ast.ASTHandle{}, nil
	}})
	astTestMust(t, err)
	kinds := []string{"external_relation", "interval_relation"}
	makeProduct := func(sorts, values []ast.ASTHandle, selectedKinds []string) *ExternalIntervalProduct {
		t.Helper()
		product, selected, cause := NewSelectedExternalIntervalProduct(plugin, sorts, selectedKinds)
		astTestMust(t, cause)
		if !selected {
			t.Fatal("mixed join product not selected")
		}
		if values != nil {
			astTestMust(t, product.AddFact(values))
		}
		return product
	}
	left := makeProduct([]ast.ASTHandle{realSort, realSort}, []ast.ASTHandle{one, three}, kinds)
	right := makeProduct([]ast.ASTHandle{realSort}, []ast.ASTHandle{three}, kinds)
	defer func() {
		astTestMust(t, right.Close())
		astTestMust(t, left.Close())
		astTestMust(t, plugin.Close())
	}()
	events = nil
	joined, err := JoinExternalIntervalProducts(left, right, []uint32{1}, []uint32{0})
	astTestMust(t, err)
	if len(events) != 1 || events[0].inputs != 2 || len(events[0].parameters) != 2 || events[0].parameters[0] != 1 || events[0].parameters[1] != 0 {
		t.Fatal("mixed join callback parameters/order", events)
	}
	joinedSorts, err := joined.Sorts()
	astTestMust(t, err)
	if len(joinedSorts) != 3 || joinedSorts[0] != realSort || joinedSorts[1] != realSort || joinedSorts[2] != realSort || joined.Kinds()[0] != kinds[0] {
		t.Fatal("mixed join lost concatenated signature/kinds", joinedSorts, joined.Kinds())
	}
	for column, want := range []string{"1", "3", "3"} {
		lo, _, hi, _, cause := joined.interval.Bounds(uint32(column))
		astTestMust(t, cause)
		if lo == nil || hi == nil || lo.RatString() != want || hi.RatString() != want {
			t.Fatal("mixed join interval result", column, lo, hi, want)
		}
	}
	astTestMust(t, joined.Close())
	if plugin.users != 2 {
		t.Fatal("mixed join result close consumed sources", plugin.users)
	}
	if empty, cause := left.Empty(); cause != nil || empty {
		t.Fatal("mixed join changed left source", empty, cause)
	}

	failJoin = true
	events = nil
	beforeUsers := plugin.users
	failed, err := JoinExternalIntervalProducts(left, right, []uint32{1}, []uint32{0})
	if failed != nil || !errors.Is(err, refusal) || len(events) != 1 || plugin.users != beforeUsers {
		t.Fatal("mixed join callback refusal published/leaked result", failed, err, events, plugin.users, beforeUsers)
	}
	failJoin = false

	reversedKinds := []string{"interval_relation", "external_relation"}
	reversed := makeProduct([]ast.ASTHandle{realSort}, []ast.ASTHandle{three}, reversedKinds)
	defer func() { astTestMust(t, reversed.Close()) }()
	events = nil
	reversedJoin, err := JoinExternalIntervalProducts(left, reversed, []uint32{1}, []uint32{0})
	astTestMust(t, err)
	if len(events) != 1 || events[0].inputs != 2 || len(events[0].parameters) != 2 ||
		events[0].parameters[0] != 1 || events[0].parameters[1] != 0 {
		t.Fatal("opposite-order mixed join callback parameters/order", events)
	}
	if got := reversedJoin.Kinds(); len(got) != len(kinds) || got[0] != kinds[0] || got[1] != kinds[1] {
		t.Fatal("opposite-order mixed join did not keep left kinds", got)
	}
	if got := reversed.Kinds(); len(got) != len(reversedKinds) || got[0] != reversedKinds[0] || got[1] != reversedKinds[1] {
		t.Fatal("opposite-order mixed join changed right kinds", got)
	}
	if got := left.Kinds(); len(got) != len(kinds) || got[0] != kinds[0] || got[1] != kinds[1] {
		t.Fatal("opposite-order mixed join changed left kinds", got)
	}
	astTestMust(t, reversedJoin.Close())
	events = nil
	reversedLeftJoin, err := JoinExternalIntervalProducts(reversed, right, nil, nil)
	astTestMust(t, err)
	if len(events) != 1 || events[0].inputs != 2 || len(events[0].parameters) != 0 {
		t.Fatal("interval-first mixed join callback shape", events)
	}
	if got := reversedLeftJoin.Kinds(); len(got) != len(reversedKinds) || got[0] != reversedKinds[0] || got[1] != reversedKinds[1] {
		t.Fatal("interval-first mixed join did not keep left kinds", got)
	}
	if got := right.Kinds(); len(got) != len(kinds) || got[0] != kinds[0] || got[1] != kinds[1] {
		t.Fatal("interval-first mixed join changed right kinds", got)
	}
	astTestMust(t, reversedLeftJoin.Close())
	integerProduct := makeProduct([]ast.ASTHandle{integerSort}, nil, kinds)
	defer func() { astTestMust(t, integerProduct.Close()) }()
	forged := makeProduct([]ast.ASTHandle{realSort}, nil, kinds)
	forged.kinds[1] = "external_relation"
	defer func() { astTestMust(t, forged.Close()) }()
	otherPlugin, err := NewExternalRelationPlugin(f.relations, plugin.callbacks)
	astTestMust(t, err)
	defer func() { astTestMust(t, otherPlugin.Close()) }()
	otherProduct, selected, err := NewSelectedExternalIntervalProduct(otherPlugin, []ast.ASTHandle{realSort}, kinds)
	astTestMust(t, err)
	if !selected {
		t.Fatal("other-plugin mixed join product not selected")
	}
	defer func() { astTestMust(t, otherProduct.Close()) }()
	events = nil
	for _, attempt := range []func() error{
		func() error { _, cause := JoinExternalIntervalProducts(left, forged, nil, nil); return cause },
		func() error { _, cause := JoinExternalIntervalProducts(left, otherProduct, nil, nil); return cause },
		func() error {
			_, cause := JoinExternalIntervalProducts(left, integerProduct, []uint32{0}, []uint32{0})
			return cause
		},
		func() error {
			_, cause := JoinExternalIntervalProducts(left, right, []uint32{2}, []uint32{0})
			return cause
		},
		func() error { _, cause := JoinExternalIntervalProducts(left, right, []uint32{0}, nil); return cause },
	} {
		if cause := attempt(); cause == nil {
			t.Fatal("incompatible mixed join accepted")
		}
	}
	if len(events) != 0 {
		t.Fatal("mixed join preflight invoked callback", events)
	}
}
