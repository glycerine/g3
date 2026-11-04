package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
)

func TestManagerAndRefSourceBooleanReductionAndOwnership(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	p := pdrBool(t, manager, "manager_and_p")
	root, err := manager.NewRef(p)
	pdrMust(t, err)
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	notRoot, err := manager.NewRef(notP)
	pdrMust(t, err)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	before := manager.Stats().LiveReferences
	for _, tc := range []struct {
		name        string
		terms       []ast.ASTHandle
		want        ast.ASTHandle
		falseResult bool
	}{
		{"duplicate", []ast.ASTHandle{p, p, truth}, p, false},
		{"complement", []ast.ASTHandle{p, notP}, ast.ASTHandle{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, x := owner.AndRef(tc.terms)
			pdrMust(t, x)
			h, x := manager.RefGet(result)
			pdrMust(t, x)
			if tc.falseResult {
				if !manager.IsFalse(h) {
					t.Fatal("expected false", h)
				}
			} else if h != tc.want {
				t.Fatal("unexpected reduced conjunction", h, tc.want)
			}
			if got := manager.Stats().LiveReferences; got != before+1 {
				t.Fatalf("result references %d, want %d", got, before+1)
			}
			pdrMust(t, manager.DelRef(&result))
			if got := manager.Stats().LiveReferences; got != before {
				t.Fatalf("released references %d, want %d", got, before)
			}
		})
	}
	pdrMust(t, manager.DelRef(&notRoot))
	pdrMust(t, manager.DelRef(&root))
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}

func TestCoverDeltaUsesSourceRewrittenConjunction(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "cover_contradictory", nil, boolean)
	transformer, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableNoCache)
	pdrMust(t, err)
	p := pdrBool(t, manager, "cover_p")
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	_, err = transformer.AddProperty(p, pdr.InfinityLevel)
	pdrMust(t, err)
	_, err = transformer.AddProperty(notP, pdr.InfinityLevel)
	pdrMust(t, err)
	before := manager.Stats().LiveReferences
	result, err := transformer.CoverDelta(-1)
	pdrMust(t, err)
	h, err := manager.RefGet(result)
	pdrMust(t, err)
	if !manager.IsFalse(h) {
		t.Fatal("contradictory invariant cover did not reduce to false", h)
	}
	pdrMust(t, manager.DelRef(&result))
	if got := manager.Stats().LiveReferences; got != before {
		t.Fatalf("cover leaked references %d, want %d", got, before)
	}
	pdrMust(t, transformer.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}

func TestManagerAndRefFailureReleasesOutput(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	p := pdrBool(t, manager, "manager_and_error_p")
	root, err := manager.NewRef(p)
	pdrMust(t, err)
	before := manager.Stats().LiveReferences
	result, err := owner.AndRef([]ast.ASTHandle{p, {}})
	if err == nil || result != (ast.ASTRef{}) {
		t.Fatal("invalid conjunction accepted or retained a result", result, err)
	}
	if got := manager.Stats().LiveReferences; got != before {
		t.Fatalf("failed conjunction leaked refs %d, want %d", got, before)
	}
	pdrMust(t, manager.DelRef(&root))
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
