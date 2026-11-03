package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalIntervalProductTransformsAndFilterAtomicity(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := f.ast.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	astTestPin(t, f.ast, realSort)
	one, err := f.ast.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	three, err := f.ast.ArithmeticRealNumeralText("3")
	astTestMust(t, err)
	five, err := f.ast.ArithmeticRealNumeralText("5")
	astTestMust(t, err)
	x, err := f.ast.MkVar(0, realSort)
	astTestMust(t, err)
	condition, err := f.ast.MkArithmeticLE(x, three)
	astTestMust(t, err)
	for _, value := range []ast.ASTHandle{one, three, five, x, condition} {
		astTestPin(t, f.ast, value)
	}

	type callbackEvent struct {
		op             int32
		parameters     []uint32
		condition      ast.ASTHandle
		conditionArgs  [2]ast.ASTHandle
		conditionVars  [2]uint32
		conditionIsVar [2]bool
		conditionArity uint32
	}
	var events []callbackEvent
	filterRefusal := errors.New("mixed product FILTER refused")
	failFilter := false
	readEvent := func(declaration ast.ASTHandle) (callbackEvent, error) {
		op, cause := f.ast.GetDeclKind(declaration)
		if cause != nil {
			return callbackEvent{}, cause
		}
		event := callbackEvent{op: op}
		if op != ast.DatalogOpProject && op != ast.DatalogOpRename && op != ast.DatalogOpFilter {
			return event, nil
		}
		parameters, cause := f.ast.GetDeclParametersCopy(declaration)
		if cause != nil {
			return callbackEvent{}, cause
		}
		defer func() {
			for i := range parameters {
				astTestMust(t, f.ast.DelParameter(&parameters[i]))
			}
		}()
		for _, parameter := range parameters {
			if op == ast.DatalogOpFilter {
				event.condition, cause = f.ast.ParameterAST(parameter)
				if cause == nil {
					event.conditionArity, cause = f.ast.GetNumArgs(event.condition)
				}
				for i := uint32(0); cause == nil && i < event.conditionArity && i < 2; i++ {
					event.conditionArgs[i], cause = f.ast.GetArg(event.condition, i)
					var kind ast.ASTKind
					if cause == nil {
						kind, cause = f.ast.GetKind(event.conditionArgs[i])
					}
					if cause == nil && kind == ast.ASTVar {
						event.conditionIsVar[i] = true
						event.conditionVars[i], cause = f.ast.GetVarIndex(event.conditionArgs[i])
					}
				}
			} else {
				var value int32
				value, cause = f.ast.ParameterInt(parameter)
				event.parameters = append(event.parameters, uint32(value))
			}
			if cause != nil {
				return callbackEvent{}, cause
			}
		}
		return event, nil
	}
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		Reduce: func(declaration ast.ASTHandle, _ []ast.ASTHandle) (ast.ASTHandle, error) {
			event, cause := readEvent(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			events = append(events, event)
			return ast.ASTHandle{}, nil
		},
		ReduceAssign: func(declaration ast.ASTHandle, _, _ []ast.ASTHandle) error {
			event, cause := readEvent(declaration)
			if cause != nil {
				return cause
			}
			if event.op == ast.DatalogOpFilter {
				events = append(events, event)
				if failFilter {
					return filterRefusal
				}
			}
			return nil
		},
	})
	astTestMust(t, err)
	product, selected, err := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{realSort, realSort}, []string{"interval_relation", "external_relation"})
	astTestMust(t, err)
	if !selected {
		t.Fatal("mixed transform product not selected")
	}
	astTestMust(t, product.AddFact([]ast.ASTHandle{one, three}))
	events = nil

	projected, err := product.Project([]uint32{0})
	astTestMust(t, err)
	projectSorts, err := projected.Sorts()
	astTestMust(t, err)
	lo, _, hi, _, err := projected.interval.Bounds(0)
	astTestMust(t, err)
	if len(projectSorts) != 1 || projectSorts[0] != realSort || lo.RatString() != "3" || hi.RatString() != "3" || len(events) != 1 || events[0].op != ast.DatalogOpProject || len(events[0].parameters) != 1 || events[0].parameters[0] != 0 {
		t.Fatal("componentwise project mismatch", projectSorts, lo, hi, events)
	}
	astTestMust(t, projected.Close())
	events = nil
	nullary, err := product.Project([]uint32{0, 1})
	astTestMust(t, err)
	nullarySorts, err := nullary.Sorts()
	astTestMust(t, err)
	if len(nullarySorts) != 0 || nullary.IsPrecise() || len(events) != 1 || events[0].op != ast.DatalogOpProject || len(events[0].parameters) != 2 || events[0].parameters[0] != 0 || events[0].parameters[1] != 1 {
		t.Fatal("componentwise nullary projection mismatch", nullarySorts, events)
	}
	astTestMust(t, nullary.Close())

	events = nil
	renamed, err := product.Rename([]uint32{0, 1})
	astTestMust(t, err)
	r0, _, _, _, err := renamed.interval.Bounds(0)
	astTestMust(t, err)
	r1, _, _, _, err := renamed.interval.Bounds(1)
	astTestMust(t, err)
	if r0.RatString() != "3" || r1.RatString() != "1" || len(events) != 1 || events[0].op != ast.DatalogOpRename || len(events[0].parameters) != 2 || events[0].parameters[0] != 0 || events[0].parameters[1] != 1 {
		t.Fatal("componentwise rename mismatch", r0, r1, events)
	}
	astTestMust(t, renamed.Close())

	events = nil
	equalProduct, err := product.Clone()
	astTestMust(t, err)
	astTestMust(t, equalProduct.FilterEqual(0, one))
	if len(events) != 1 || events[0].op != ast.DatalogOpFilter {
		t.Fatal("equality filter callback missing", events)
	}
	if !events[0].conditionIsVar[0] || events[0].conditionVars[0] != 0 || events[0].conditionArgs[1] != one {
		t.Fatal("equality filter parameter order", events[0])
	}
	astTestMust(t, equalProduct.Close())

	events = nil
	identicalProduct, err := product.Clone()
	astTestMust(t, err)
	astTestMust(t, identicalProduct.FilterIdentical([]uint32{1, 0}))
	if len(events) != 1 || events[0].op != ast.DatalogOpFilter {
		t.Fatal("identical filter callback missing", events)
	}
	if !events[0].conditionIsVar[0] || !events[0].conditionIsVar[1] || events[0].conditionVars[0] != 1 || events[0].conditionVars[1] != 0 {
		t.Fatal("identical filter parameter order", events[0])
	}
	if !identicalProduct.interval.Empty() {
		t.Fatal("identical interval filter did not reject unequal fact")
	}
	astTestMust(t, identicalProduct.Close())

	events = nil
	interpretedProduct, err := product.Clone()
	astTestMust(t, err)
	astTestMust(t, interpretedProduct.FilterInterpreted(condition))
	if len(events) != 1 || events[0].op != ast.DatalogOpFilter || events[0].condition != condition {
		t.Fatal("interpreted filter condition changed", events)
	}
	astTestMust(t, interpretedProduct.Close())
	before, _, _, _, err := product.interval.Bounds(0)
	astTestMust(t, err)
	failFilter = true
	if err = product.FilterEqual(0, five); !errors.Is(err, filterRefusal) {
		t.Fatal("filter refusal not propagated", err)
	}
	after, _, _, _, err := product.interval.Bounds(0)
	astTestMust(t, err)
	if before.Cmp(after) != 0 || product.interval.Empty() {
		t.Fatal("failed external filter published interval candidate", before, after)
	}

	astTestMust(t, product.Close())
	astTestMust(t, plugin.Close())
}
