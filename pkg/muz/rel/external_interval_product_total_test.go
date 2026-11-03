package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalIntervalNullaryTotalConstructionAndRefusal(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			f := newTableRelationFixture(t)
			var operations []int32
			refusal := errors.New("nullary total callback refusal")
			refuse := int32(-1)
			plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
				ReduceAssign: func(declaration ast.ASTHandle, _, _ []ast.ASTHandle) error {
					op, cause := f.ast.GetDeclKind(declaration)
					if cause != nil {
						return cause
					}
					operations = append(operations, op)
					if op == refuse {
						return refusal
					}
					return nil
				},
				Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
					op, cause := f.ast.GetDeclKind(declaration)
					if cause != nil {
						return ast.ASTHandle{}, cause
					}
					operations = append(operations, op)
					if op == refuse {
						return ast.ASTHandle{}, refusal
					}
					if len(arguments) != 1 {
						return ast.ASTHandle{}, ErrTableRelation
					}
					switch op {
					case ast.DatalogOpComplement:
						return arguments[0], nil
					case ast.DatalogOpIsEmpty:
						return f.ast.MkFalse()
					default:
						return ast.ASTHandle{}, ErrTableRelation
					}
				},
			})
			astTestMust(t, err)

			total, selected, err := NewSelectedExternalIntervalNullaryTotal(plugin, kinds)
			astTestMust(t, err)
			if !selected || total == nil || len(operations) != 2 || operations[0] != ast.DatalogOpEmpty || operations[1] != ast.DatalogOpComplement {
				t.Fatal("nullary total did not use Empty().Complement()", selected, operations)
			}
			gotKinds := total.Kinds()
			if len(gotKinds) != len(kinds) || gotKinds[0] != kinds[0] || gotKinds[1] != kinds[1] {
				t.Fatal("nullary total changed requested kind order", gotKinds, kinds)
			}
			if sorts, cause := total.Sorts(); cause != nil || len(sorts) != 0 {
				t.Fatal("nullary total signature", sorts, cause)
			}
			if empty, cause := total.Empty(); cause != nil || empty {
				t.Fatal("selected nullary total was empty", empty, cause)
			}
			astTestMust(t, total.Close())
			operations = nil
			if invalid, selected, cause := NewSelectedExternalIntervalNullaryTotal(plugin, []string{"external_relation"}); invalid != nil || selected || cause != nil || len(operations) != 0 {
				t.Fatal("invalid nullary total selection reached callbacks", invalid, selected, cause, operations)
			}

			for _, operation := range []int32{ast.DatalogOpEmpty, ast.DatalogOpComplement} {
				operations, refuse = nil, operation
				total, selected, err = NewSelectedExternalIntervalNullaryTotal(plugin, kinds)
				if !selected || total != nil || !errors.Is(err, refusal) || plugin.users != 0 {
					t.Fatal("nullary total refusal published an owner", operation, total, selected, err, plugin.users)
				}
			}
			astTestMust(t, plugin.Close())
		})
	}
}
