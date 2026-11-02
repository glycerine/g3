package rel

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExplanationRelationInstructionsLoadCloneStoreAndReset(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		n, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		d, e := f.ast.MkFuncDecl(n, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestPin(t, f.ast, d)
		return d
	}
	explanation := func(name string) ast.ASTHandle {
		n, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		h, e := f.ast.MkConst(n, ruleSort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	predicate := declare("typed-explanation-load", []ast.ASTHandle{f.boolean, ruleSort})
	first, second := explanation("typed-first"), explanation("typed-second")
	stored, err := f.relations.GetExplanationRelation(predicate)
	astTestMust(t, err)
	astTestMust(t, stored.AddFact([]ast.ASTHandle{f.truth}, first))
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	load, err := NewLoadInstruction(f.relations, predicate, 3)
	astTestMust(t, err)
	defer func() { astTestMust(t, load.Close()) }()
	store, err := NewStoreInstruction(f.relations, predicate, 3)
	astTestMust(t, err)
	defer func() { astTestMust(t, store.Close()) }()
	complete, err := load.Perform(context)
	astTestMust(t, err)
	if !complete {
		t.Fatal("explanation load incomplete")
	}
	loaded, err := context.ExplanationRegister(3)
	astTestMust(t, err)
	if loaded == nil || loaded == stored {
		t.Fatal("explanation load did not clone")
	}
	count, err := context.RegisterCount()
	astTestMust(t, err)
	if count != 4 {
		t.Fatal("typed register count", count)
	}
	if _, err = context.Register(3); err == nil {
		t.Fatal("table reader accepted explanation register")
	}
	astTestMust(t, loaded.AddFact([]ast.ASTHandle{f.falsehood}, second))
	if n, err := stored.Len(); err != nil || n != 1 {
		t.Fatal("load clone changed manager source", n, err)
	}
	astTestMust(t, load.MakeAnnotations(context))
	var profile bytes.Buffer
	astTestMust(t, context.ReportBigRelations(1, &profile))
	if strings.HasPrefix(profile.String(), "bytes 0\n") {
		t.Fatal("typed register omitted from size profile", profile.String())
	}
	annotation, found, err := context.RegisterAnnotation(3)
	astTestMust(t, err)
	if !found || annotation != "typed-explanation-load" {
		t.Fatal("typed load annotation", annotation, found)
	}
	complete, err = store.Perform(context)
	astTestMust(t, err)
	if !complete {
		t.Fatal("explanation store incomplete")
	}
	if value, err := context.ExplanationRegister(3); err != nil || value != nil {
		t.Fatal("store kept register owner", value, err)
	}
	published, found, err := f.relations.TryGetExplanationRelation(predicate)
	astTestMust(t, err)
	if !found || published != loaded {
		t.Fatal("store did not transfer explanation owner")
	}
	if _, err = stored.Len(); err == nil {
		t.Fatal("replaced explanation product remained live")
	}
	for _, spec := range []struct{ value, want ast.ASTHandle }{{f.truth, first}, {f.falsehood, second}} {
		got, found, err := published.Explanation([]ast.ASTHandle{spec.value})
		astTestMust(t, err)
		if !found || got != spec.want {
			t.Fatal("stored explanation tuple", got, found, spec.want)
		}
	}
	// Invalid store restores the register owner and preserves the managed one.
	wrong, err := newFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean}, 2)
	astTestMust(t, err)
	astTestMust(t, context.SetExplanationRegister(3, wrong))
	if complete, err = store.Perform(context); err == nil || complete {
		t.Fatal("wrong explanation signature stored", complete, err)
	}
	if got, err := context.ExplanationRegister(3); err != nil || got != wrong {
		t.Fatal("failed store lost register owner", got, err)
	}
	if got, _, err := f.relations.TryGetExplanationRelation(predicate); err != nil || got != published {
		t.Fatal("failed store changed managed owner", got, err)
	}
	astTestMust(t, context.MakeEmpty(3))
	if _, err = wrong.Len(); err == nil {
		t.Fatal("MakeEmpty did not close typed owner")
	}
	// Empty stores publish the correctly typed empty relation.
	complete, err = store.Perform(context)
	astTestMust(t, err)
	if !complete {
		t.Fatal("empty typed store incomplete")
	}
	empty, err := f.relations.GetExplanationRelation(predicate)
	astTestMust(t, err)
	if n, err := empty.Len(); err != nil || n != 0 {
		t.Fatal("empty typed store", n, err)
	}
	if _, err = published.Len(); err == nil {
		t.Fatal("empty store did not retire previous product")
	}
	astTestMust(t, empty.AddFact([]ast.ASTHandle{f.truth}, first))
	_, err = load.Perform(context)
	astTestMust(t, err)
	retained, err := context.ExplanationRegister(3)
	astTestMust(t, err)
	astTestMust(t, f.relations.ResetRelations())
	if _, err = empty.Len(); err == nil {
		t.Fatal("manager reset did not release product")
	}
	if got, found, err := retained.Explanation([]ast.ASTHandle{f.truth}); err != nil || !found || got != first {
		t.Fatal("manager reset invalidated register clone", got, found, err)
	}
	astTestMust(t, context.Reset())
	if _, err = retained.Len(); err == nil {
		t.Fatal("execution reset did not release typed register")
	}
	bad := declare("bad-explanation-layout", []ast.ASTHandle{ruleSort, f.boolean})
	if instruction, err := NewLoadInstruction(f.relations, bad, 0); err == nil || instruction != nil {
		t.Fatal("interleaved unsupported explanation layout accepted", instruction, err)
	}
}
