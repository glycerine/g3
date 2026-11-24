package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	uhash "github.com/glycerine/g3/pkg/util/hash"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTEnodeTrailGenerationAndLabelHash(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2050), ast.ASTSortInfo{})
	astTestMust(t, err)
	_, node := smtEnodeConstant(t, m, 2051, sort, 7)
	ownerID, err := node.OwnerID()
	astTestMust(t, err)
	wantHash := uint8(uhash.Uint(ownerID) & 63)

	var trail SMTEnodeTrail
	mark := trail.Mark()
	astTestMust(t, node.SetGeneration(&trail, 7))
	if trail.Len() != mark {
		t.Fatal("unchanged generation was trailed")
	}
	astTestMust(t, node.SetGeneration(&trail, 11))
	if node.Generation() != 11 || trail.Len() != mark+1 {
		t.Fatal("generation update was not trailed")
	}
	if node.HasLabelHash() {
		t.Fatal("fresh enode has a label hash")
	}
	astTestMust(t, node.SetLabelHash(&trail))
	gotHash, err := node.LabelHash()
	astTestMust(t, err)
	if gotHash != wantHash || !node.Root().Labels().MayContain(uint32(wantHash)) || trail.Len() != mark+3 {
		t.Fatal("label hash or root label propagation", gotHash, wantHash, trail.Len())
	}
	if err := node.SetLabelHash(&trail); err != ast.ErrASTDomain {
		t.Fatal("duplicate label hash guard", err)
	}

	beforeUndo := trail.entries
	astTestMust(t, trail.Undo(mark))
	for _, entry := range beforeUndo[mark:] {
		if entry.node != nil {
			t.Fatal("rolled-back label trail still holds node")
		}
	}
	if node.Generation() != 7 || node.HasLabelHash() || !node.Root().Labels().Empty() || trail.Len() != mark {
		t.Fatal("trail did not restore enode fields")
	}
	if _, err := node.LabelHash(); err != ast.ErrASTDomain {
		t.Fatal("unset label hash guard", err)
	}
}

func TestSMTEnodeTrailNestedMarksAndExistingRootLabel(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2060), ast.ASTSortInfo{})
	astTestMust(t, err)
	_, root := smtEnodeConstant(t, m, 2061, sort, 1)
	_, member := smtEnodeConstant(t, m, 2062, sort, 2)
	member.root = root
	ownerID, err := member.OwnerID()
	astTestMust(t, err)
	wantHash := uint32(uhash.Uint(ownerID) & 63)
	root.labels.Insert(wantHash)

	var trail SMTEnodeTrail
	astTestMust(t, root.SetGeneration(&trail, 3))
	nested := trail.Mark()
	astTestMust(t, member.SetGeneration(&trail, 4))
	astTestMust(t, member.SetLabelHash(&trail))
	if trail.Len() != nested+2 {
		t.Fatal("existing root label should not be trailed again", trail.Len())
	}
	astTestMust(t, trail.Undo(nested))
	if root.Generation() != 3 || member.Generation() != 2 || member.HasLabelHash() || !root.labels.MayContain(wantHash) {
		t.Fatal("nested undo restored the wrong scope")
	}
	astTestMust(t, trail.Undo(0))
	if root.Generation() != 1 || trail.Len() != 0 {
		t.Fatal("outer undo failed")
	}
	if err := trail.Undo(1); err != ast.ErrASTDomain {
		t.Fatal("invalid mark guard", err)
	}
	if err := root.SetGeneration(nil, 9); err != ast.ErrASTDomain {
		t.Fatal("nil trail guard", err)
	}
}
