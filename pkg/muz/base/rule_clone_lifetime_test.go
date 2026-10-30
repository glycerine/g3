package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleCloneOwnsASTsIndependentlyOfSource(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	source, err := f.rules.NewRule(f.head, []ast.ASTHandle{f.head}, nil, "lifetime")
	ruleMust(t, err)
	clone, err := f.rules.CloneRule(source)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, clone.Close()) })
	ruleMust(t, source.Close())
	if head, err := clone.Head(); err != nil || head != f.head {
		t.Fatal("clone head after source close", head, err)
	}
	if tail, _, err := clone.Tail(0); err != nil || tail != f.head {
		t.Fatal("clone tail after source close", tail, err)
	}
}
