package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFilterExternalRuleInterpretedBindsJoinedColumns(t *testing.T) {
	f := newTableRelationFixture(t)
	rows := make(map[ast.ASTHandle][][]ast.ASTHandle)
	filters := 0
	theory := tableRelationTheoryRewriter(t, f.ast)
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
			case ast.DatalogOpFilter:
				filters++
				parameters, cause := f.ast.GetDeclParametersCopy(declaration)
				if cause != nil || len(parameters) != 1 {
					return cause
				}
				condition, cause := f.ast.ParameterAST(parameters[0])
				astTestMust(t, f.ast.DelParameter(&parameters[0]))
				if cause != nil {
					return cause
				}
				kept := rows[arguments[0]][:0]
				for _, tuple := range rows[arguments[0]] {
					out, rewriteErr := f.ast.NewRef(ast.ASTHandle{})
					if rewriteErr != nil {
						return rewriteErr
					}
					bindings := make([]ast.ASTHandle, len(tuple))
					for column := range tuple {
						bindings[len(tuple)-column-1] = tuple[column]
					}
					rewriteErr = theory.RewriteWithBindings(condition, bindings, out)
					answer, getErr := f.ast.RefGet(out)
					rewriteErr = errors.Join(rewriteErr, getErr)
					if rewriteErr == nil && f.ast.IsTrue(answer) {
						kept = append(kept, tuple)
					}
					rewriteErr = errors.Join(rewriteErr, f.ast.DelRef(&out))
					if rewriteErr != nil {
						return rewriteErr
					}
				}
				rows[outputs[0]] = kept
			default:
				t.Fatalf("unexpected assignment operation %d", kind)
			}
			return nil
		},
	})
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	relation, err := plugin.NewEmpty([]ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, relation.Close()) }()
	astTestMust(t, relation.AddFact([]ast.ASTHandle{f.truth, f.falsehood}))
	astTestMust(t, relation.AddFact([]ast.ASTHandle{f.falsehood, f.truth}))
	x, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	y, err := f.ast.MkVar(1, f.boolean)
	astTestMust(t, err)
	xTrue, err := f.ast.MkEq(x, f.truth)
	astTestMust(t, err)
	yFalse, err := f.ast.MkEq(y, f.falsehood)
	astTestMust(t, err)
	astTestMust(t, filterExternalRuleInterpreted(relation, []ast.ASTHandle{xTrue, yFalse}, map[uint32]uint32{0: 1, 1: 0}))
	handle, err := relation.Relation()
	astTestMust(t, err)
	if filters != 1 || len(rows[handle]) != 1 || rows[handle][0][0] != f.falsehood || rows[handle][0][1] != f.truth {
		t.Fatal("interpreted joined-column filter", filters, rows[handle])
	}
	before := filters
	if cause := filterExternalRuleInterpreted(relation, []ast.ASTHandle{xTrue}, nil); cause == nil {
		t.Fatal("unbound interpreted variable accepted")
	}
	if filters != before {
		t.Fatal("invalid interpreted binding invoked callback")
	}
}
