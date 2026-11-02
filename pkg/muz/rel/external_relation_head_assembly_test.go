package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAssembleExternalRuleHeadConstantsRepeatsAndOrder(t *testing.T) {
	f := newTableRelationFixture(t)
	rows := make(map[ast.ASTHandle][][]ast.ASTHandle)
	copyRows := func(input [][]ast.ASTHandle) [][]ast.ASTHandle {
		result := make([][]ast.ASTHandle, len(input))
		for i := range input {
			result[i] = append([]ast.ASTHandle(nil), input[i]...)
		}
		return result
	}
	parameters := func(declaration ast.ASTHandle) []uint32 {
		values, err := f.ast.GetDeclParametersCopy(declaration)
		astTestMust(t, err)
		result := make([]uint32, len(values))
		for i := range values {
			value, cause := f.ast.ParameterInt(values[i])
			astTestMust(t, cause)
			result[i] = uint32(value)
			astTestMust(t, f.ast.DelParameter(&values[i]))
		}
		return result
	}
	reductions := 0
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return cause
			}
			switch kind {
			case ast.DatalogOpEmpty:
				rows[outputs[0]] = nil
			case ast.DatalogOpStore:
				rows[outputs[0]] = append(rows[outputs[0]], append([]ast.ASTHandle(nil), arguments[1:]...))
			case ast.DatalogOpClone:
				reductions++
				rows[outputs[0]] = copyRows(rows[arguments[0]])
			default:
				t.Fatalf("unexpected assignment operation %d", kind)
			}
			return nil
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			sort, cause := f.ast.GetRange(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			output, cause := f.ast.MkFreshConst("external-head-assembly", sort)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			reductions++
			switch kind {
			case ast.DatalogOpProject:
				removed := make(map[uint32]bool)
				for _, column := range parameters(declaration) {
					removed[column] = true
				}
				for _, row := range rows[arguments[0]] {
					projected := make([]ast.ASTHandle, 0, len(row)-len(removed))
					for column, value := range row {
						if !removed[uint32(column)] {
							projected = append(projected, value)
						}
					}
					rows[output] = append(rows[output], projected)
				}
			case ast.DatalogOpJoin:
				indices := parameters(declaration)
				for _, left := range rows[arguments[0]] {
					for _, right := range rows[arguments[1]] {
						match := true
						for i := 0; i < len(indices); i += 2 {
							match = match && left[indices[i]] == right[indices[i+1]]
						}
						if match {
							row := append(append([]ast.ASTHandle(nil), left...), right...)
							rows[output] = append(rows[output], row)
						}
					}
				}
			case ast.DatalogOpRename:
				cycle := parameters(declaration)
				for _, row := range rows[arguments[0]] {
					renamed := append([]ast.ASTHandle(nil), row...)
					for i, destination := range cycle {
						renamed[destination] = row[cycle[(i+1)%len(cycle)]]
					}
					rows[output] = append(rows[output], renamed)
				}
			default:
				t.Fatalf("unexpected reduction operation %d", kind)
			}
			return output, nil
		},
	})
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	source, err := plugin.NewEmpty([]ast.ASTHandle{f.boolean, f.boolean, f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth, f.falsehood, f.truth}))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood, f.truth, f.falsehood}))
	constant, err := plugin.NewEmpty([]ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, constant.Close()) }()
	astTestMust(t, constant.AddFact([]ast.ASTHandle{f.truth}))
	columns := []externalHeadColumn{
		externalSourceHeadColumn(1),
		externalSourceHeadColumn(1),
		externalConstantHeadColumn(f.truth),
		externalSourceHeadColumn(0),
	}
	result, err := assembleExternalRuleHead(source, columns, map[ast.ASTHandle]*ExternalRelation{f.truth: constant})
	astTestMust(t, err)
	defer func() { astTestMust(t, result.Close()) }()
	handle, err := result.Relation()
	astTestMust(t, err)
	want := [][]ast.ASTHandle{
		{f.falsehood, f.falsehood, f.truth, f.truth},
		{f.truth, f.truth, f.truth, f.falsehood},
	}
	if len(rows[handle]) != len(want) {
		t.Fatal("assembled tuple count", rows[handle])
	}
	for i := range want {
		for column := range want[i] {
			if rows[handle][i][column] != want[i][column] {
				t.Fatal("assembled tuples", rows[handle], "want", want)
			}
		}
	}
	sourceHandle, err := source.Relation()
	astTestMust(t, err)
	if len(rows[sourceHandle]) != 2 || len(rows[sourceHandle][0]) != 3 {
		t.Fatal("assembly mutated source", rows[sourceHandle])
	}
	before := reductions
	if invalid, cause := assembleExternalRuleHead(source, []externalHeadColumn{externalConstantHeadColumn(f.falsehood)}, nil); cause == nil || invalid != nil {
		t.Fatal("missing preflight singleton accepted", invalid, cause)
	}
	if reductions != before {
		t.Fatal("invalid assembly invoked callbacks before validation")
	}
}
