package cmd_context

import "testing"

func TestContextUserTacticsReplacementOrderResetAndClose(t *testing.T) {
	context, err := NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = context.DeclareUserTactic("first", "simplify"); err != nil {
		t.Fatal(err)
	}
	if err = context.DeclareUserTactic("second", "skip"); err != nil {
		t.Fatal(err)
	}
	if err = context.DeclareUserTactic("first", "fail"); err != nil {
		t.Fatal(err)
	}
	declarations := context.UserTacticDeclarations()
	if len(declarations) != 2 || declarations[0] != (UserTacticDeclaration{Name: "first", Body: "fail"}) || declarations[1] != (UserTacticDeclaration{Name: "second", Body: "skip"}) {
		t.Fatal("replacement changed user tactic order or body", declarations)
	}
	if err = context.Reset(); err != nil {
		t.Fatal(err)
	}
	if body, found := context.UserTacticBody("first"); !found || body != "fail" {
		t.Fatal("reset lost user tactic declaration", body, found)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
	if declarations := context.UserTacticDeclarations(); len(declarations) != 0 {
		t.Fatal("closed context retained user tactic declarations", declarations)
	}
}
