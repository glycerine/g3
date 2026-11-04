package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestManagerNotAndSourceRewriteAndOwnership(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	p := pdrBool(t, manager, "not_and_p")
	pRoot, err := manager.NewRef(p)
	pdrMust(t, err)
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	notPRoot, err := manager.NewRef(notP)
	pdrMust(t, err)
	before := manager.Stats().LiveReferences
	for _, tc := range []struct {
		name string
		args []ast.ASTHandle
		want ast.ASTHandle
	}{
		{"duplicate", []ast.ASTHandle{p, p}, notP},
		{"complement", []ast.ASTHandle{p, notP}, ast.ASTHandle{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref, x := owner.NotAndRef(tc.args)
			pdrMust(t, x)
			got, x := manager.RefGet(ref)
			pdrMust(t, x)
			if tc.name == "complement" {
				if !manager.IsTrue(got) {
					t.Fatal("complementary literals did not simplify to true", got)
				}
			} else if got != tc.want {
				t.Fatal("duplicate literals did not collapse", got, tc.want)
			}
			if gotRefs := manager.Stats().LiveReferences; gotRefs != before+1 {
				t.Fatalf("live references = %d, want %d", gotRefs, before+1)
			}
			pdrMust(t, manager.DelRef(&ref))
			if gotRefs := manager.Stats().LiveReferences; gotRefs != before {
				t.Fatalf("after release references = %d, want %d", gotRefs, before)
			}
		})
	}
	pdrMust(t, manager.DelRef(&notPRoot))
	pdrMust(t, manager.DelRef(&pRoot))
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}

func TestManagerNotAndFailureReleasesTemporaryReferences(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	p := pdrBool(t, manager, "not_and_failure_p")
	root, err := manager.NewRef(p)
	pdrMust(t, err)
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	notRoot, err := manager.NewRef(notP)
	pdrMust(t, err)
	before := manager.Stats().LiveReferences
	// Enough rooted terms force the reference arena to grow after earlier
	// negations have been constructed, allowing a mid-operation failure.
	args := make([]ast.ASTHandle, 10000)
	for i := range args {
		args[i] = p
	}
	manager.Budget().FailAfter(0)
	ref, failure := owner.NotAndRef(args)
	manager.Budget().FailAfter(-1)
	if failure == nil {
		pdrMust(t, manager.DelRef(&ref))
		t.Fatal("expected injected allocation failure")
	}
	if ref != (ast.ASTRef{}) {
		t.Fatal("failed rewrite returned an owned result")
	}
	if got := manager.Stats().LiveReferences; got != before {
		t.Fatalf("failed rewrite leaked references: got %d want %d (%v)", got, before, failure)
	}
	pdrMust(t, manager.DelRef(&notRoot))
	pdrMust(t, manager.DelRef(&root))
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}

func TestManagerNotAndLegacyAndNilGlobals(t *testing.T) {
	command, manager, base := pdrManagerFixture(t, 1)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	config := smtcore.SMTSolverConfig{Params: globals.Context}
	context, err := pdr.NewContext(manager, nil, 1, pdr.ReachableNoCache, config)
	pdrMust(t, err)
	pdrMust(t, context.Close())
	owner, err := pdr.NewManager(manager, nil, 1, config)
	pdrMust(t, err)
	p := pdrBool(t, manager, "not_and_legacy")
	root, err := manager.NewRef(p)
	pdrMust(t, err)
	andRef, err := owner.AndRef([]ast.ASTHandle{p, p})
	pdrMust(t, err)
	andResult, err := manager.RefGet(andRef)
	pdrMust(t, err)
	if andResult != p {
		t.Fatal("nil-globals conjunction was not simplified", andResult)
	}
	pdrMust(t, manager.DelRef(&andRef))
	result, err := owner.NotAnd([]ast.ASTHandle{p, p})
	pdrMust(t, err)
	isNot, arg, err := manager.IsNot(result)
	pdrMust(t, err)
	if !isNot || arg != p {
		t.Fatal("legacy result", result)
	}
	// Borrow remains valid after an independent owned rewrite and its release.
	ref, err := owner.NotAndRef([]ast.ASTHandle{p, p})
	pdrMust(t, err)
	pdrMust(t, manager.DelRef(&ref))
	isNot, _, err = manager.IsNot(result)
	pdrMust(t, err)
	if !isNot {
		t.Fatal("legacy borrowed result was invalidated")
	}
	pdrMust(t, owner.Close())
	pdrMust(t, manager.DelRef(&root))
	pdrMust(t, base.Close())
	pdrMust(t, command.Close())
}

func TestManagerNotAndOwnsFlattenedNegations(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	p := pdrBool(t, manager, "not_and_flat_p")
	q := pdrBool(t, manager, "not_and_flat_q")
	disjunction, err := manager.MkOr(p, q)
	pdrMust(t, err)
	input, err := manager.MkNot(disjunction)
	pdrMust(t, err)
	inputRoot, err := manager.NewRef(input)
	pdrMust(t, err)
	before := manager.Stats().LiveReferences
	ref, err := owner.NotAndRef([]ast.ASTHandle{input})
	pdrMust(t, err)
	result, err := manager.RefGet(ref)
	pdrMust(t, err)
	terms, err := owner.GetOr(result)
	pdrMust(t, err)
	if len(terms) != 2 || !((terms[0] == p && terms[1] == q) || (terms[0] == q && terms[1] == p)) {
		t.Fatal("flattened negation lost its terms", terms)
	}
	pdrMust(t, manager.DelRef(&ref))
	if got := manager.Stats().LiveReferences; got != before {
		t.Fatalf("flattened terms leaked refs: %d != %d", got, before)
	}
	pdrMust(t, manager.DelRef(&inputRoot))
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
