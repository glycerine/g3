package base_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"strings"
	"testing"
)

type engineFake struct {
	manager  *ast.ASTManager
	query    ast.ASTHandle
	queryRef ast.ASTRef
	answer   ast.ASTRef
}

func (f *engineFake) Query(q ast.ASTHandle) (lbool.LBool, error) {
	f.query = q
	var err error
	f.queryRef, err = f.manager.NewRef(q)
	if err != nil {
		return lbool.Undef, err
	}
	return lbool.True, nil
}
func (f *engineFake) Answer() (ast.ASTRef, error) { return f.manager.CopyRef(f.answer) }

func TestEngineBaseRelationQueryAndDefaults(t *testing.T) {
	f := newRuleSetFixture(t)
	truth, e := f.manager.MkTrue()
	ruleMust(t, e)
	answer, e := f.manager.NewRef(truth)
	ruleMust(t, e)
	defer f.manager.DelRef(&answer)
	core := &engineFake{manager: f.manager, answer: answer}
	engine, e := base.NewEngineBase(f.manager, "fake", core)
	ruleMust(t, e)
	defer engine.Close()
	b, e := f.manager.MkBoolSort()
	ruleMust(t, e)
	s, e := f.manager.Symbols.Intern("engine-rel")
	ruleMust(t, e)
	decl, e := f.manager.MkFuncDecl(s, []ast.ASTHandle{b, b}, b, ast.ASTFuncDeclInfo{})
	ruleMust(t, e)
	got, e := engine.QueryRelations([]ast.ASTHandle{decl})
	ruleMust(t, e)
	if got != lbool.True {
		t.Fatal(got)
	}
	kind, e := f.manager.GetKind(core.query)
	ruleMust(t, e)
	if kind != ast.ASTQuantifier {
		t.Fatal(kind)
	}
	ruleMust(t, f.manager.DelRef(&core.queryRef))
	if got, e = engine.QueryRelations([]ast.ASTHandle{decl, decl}); e != nil || got != lbool.Undef {
		t.Fatal(got, e)
	}
	if _, e = engine.GroundSATAnswer(); e == nil || !strings.Contains(e.Error(), "fake") {
		t.Fatal(e)
	}
	if _, e = engine.NumLevels(decl); e == nil || !strings.Contains(e.Error(), "get_num_levels") {
		t.Fatal(e)
	}
	copy, e := engine.Answer()
	ruleMust(t, e)
	ruleMust(t, f.manager.DelRef(&copy))
}
