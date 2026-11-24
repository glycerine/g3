package ast

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// The same application is both a left and a right child at every level.
// Building the expected DAG in the destination checks argument positions and
// sharing even for a translator whose memo table lasts for only one call.
func sharedTranslationFixture(t testing.TB, m *ASTManager, depth int) ASTHandle {
	t.Helper()
	st, e := m.MkBoolSort()
	astTestMust(t, e)
	a, e := m.MkConst(astTestSymbol(t, m, "a"), st)
	astTestMust(t, e)
	b, e := m.MkConst(astTestSymbol(t, m, "b"), st)
	astTestMust(t, e)
	f, e := m.MkFuncDecl(astTestSymbol(t, m, "f"), []ASTHandle{st, st, st}, st, ASTFuncDeclInfo{})
	astTestMust(t, e)
	h := a
	for i := 0; i < depth; i++ {
		extra := a
		if i%2 == 0 {
			extra = b
		}
		h, e = m.MkApp(f, []ASTHandle{h, extra, h})
		astTestMust(t, e)
	}
	return h
}

func TestASTTranslationSharedDAGDepth(t *testing.T) {
	for _, persist := range []bool{false, true} {
		t.Run(strconv.FormatBool(persist), func(t *testing.T) {
			from, to := astTranslationManagers(t)
			root := sharedTranslationFixture(t, from, 1536)
			astTestPin(t, from, root)
			translated, e := newASTTranslation(from, to, false, persist)
			astTestMust(t, e)
			defer func() { astTestMust(t, translated.Close()) }()
			result, e := translated.Translate(root)
			astTestMust(t, e)
			astTestPin(t, to, result)
			if want := sharedTranslationFixture(t, to, 1536); result != want {
				t.Fatal("translated a different DAG")
			}
			if (translated.cache.Len() != 0) != persist {
				t.Fatal("unexpected number of cached nodes", translated.cache.Len())
			}
			astTestMust(t, translated.ResetCache())
			again, e := translated.Translate(root)
			astTestMust(t, e)
			if again != result {
				t.Fatal("translation changed after cache reset")
			}
		})
	}
}

func TestASTTranslationRetryAfterDestinationFailure(t *testing.T) {
	for _, fault := range []int64{0, 1, 2, 4, 8, 16, 32} {
		t.Run(strconv.FormatInt(fault, 10), func(t *testing.T) {
			from, to := astTranslationManagers(t)
			root := sharedTranslationFixture(t, from, 20)
			astTestPin(t, from, root)
			tr, e := NewASTTranslation(from, to, false)
			astTestMust(t, e)
			defer func() { astTestMust(t, tr.Close()) }()
			to.Budget().FailAfter(fault)
			_, e = tr.Translate(root)
			to.Budget().FailAfter(-1)
			if fault == 0 && e == nil {
				t.Fatal("expected the first destination allocation to fail")
			}
			if e != nil && !errors.Is(e, arena.ErrInjectedFailure) {
				t.Fatal(e)
			}
			got, e := tr.Translate(root)
			astTestMust(t, e)
			astTestPin(t, to, got)
			if want := sharedTranslationFixture(t, to, 20); got != want {
				t.Fatal("retry returned a different DAG")
			}
		})
	}
}
