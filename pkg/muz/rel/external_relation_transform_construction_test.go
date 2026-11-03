package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalRelationTransformsSuppressCallbackFailedOutputs(t *testing.T) {
	f := newTableRelationFixture(t)
	callbackFailure := errors.New("external transform callback failed")
	failing := int32(-1)
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(_ ast.ASTHandle, _, _ []ast.ASTHandle) error { return nil },
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			if kind == failing {
				return ast.ASTHandle{}, callbackFailure
			}
			return arguments[0], nil
		},
	})
	astTestMust(t, err)
	left, err := plugin.NewEmpty([]ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	right, err := plugin.NewEmpty([]ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	if plugin.users != 2 {
		t.Fatal("transform fixture ownership", plugin.users)
	}

	failing = ast.DatalogOpProject
	projected, err := left.Project([]uint32{1})
	if projected != nil || !errors.Is(err, callbackFailure) || plugin.users != 2 {
		t.Fatal("Project published or leaked a callback-failed output", projected, err, plugin.users)
	}
	if _, err = left.Relation(); err != nil {
		t.Fatal("Project cleanup consumed its borrowed input", err)
	}

	failing = ast.DatalogOpRename
	renamed, err := left.Rename([]uint32{0, 1})
	if renamed != nil || !errors.Is(err, callbackFailure) || plugin.users != 2 {
		t.Fatal("Rename published or leaked a callback-failed output", renamed, err, plugin.users)
	}
	if _, err = left.Relation(); err != nil {
		t.Fatal("Rename cleanup consumed its borrowed input", err)
	}

	failing = ast.DatalogOpJoin
	joined, err := JoinExternalRelations(left, right, []uint32{0}, []uint32{0})
	if joined != nil || !errors.Is(err, callbackFailure) || plugin.users != 2 {
		t.Fatal("Join published or leaked a callback-failed output", joined, err, plugin.users)
	}
	if _, err = left.Relation(); err != nil {
		t.Fatal("Join cleanup consumed its left input", err)
	}
	if _, err = right.Relation(); err != nil {
		t.Fatal("Join cleanup consumed its right input", err)
	}

	astTestMust(t, right.Close())
	astTestMust(t, left.Close())
	astTestMust(t, plugin.Close())
}
