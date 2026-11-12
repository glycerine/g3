//go:build g3debug && g3trace

package ufbv

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRewriteBackSetMemoryWrapperRetainsEmptySetUntilClose(t *testing.T) {
	b := arena.NewBudget(1 << 30)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	sy, e := symbol.NewManager(symbol.Config{Budget: b})
	cacheMust(t, e)
	q, e := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	cacheMust(t, e)
	m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b, TraceTags: tags})
	cacheMust(t, e)
	t.Cleanup(func() {
		cacheMust(t, m.Close())
		cacheMust(t, q.Close())
		cacheMust(t, sy.Close())
		if b.Used() != 0 {
			t.Fatal("back-set owner leak", b.Used())
		}
	})
	boolSort, e := m.MkBoolSort()
	cacheMust(t, e)
	x, e := m.MkVar(0, boolSort)
	cacheMust(t, e)
	f, e := m.MkFuncDecl(symbol.Numerical(981), []ast.ASTHandle{boolSort}, boolSort, ast.ASTFuncDeclInfo{})
	cacheMust(t, e)
	fx, e := m.MkApp(f, []ast.ASTHandle{x})
	cacheMust(t, e)
	cacheMust(t, m.IncRef(fx))
	defer func() { cacheMust(t, m.DecRef(fx)) }()
	r, e := newRewriteBackIndex(m)
	cacheMust(t, e)
	start := b.Used()
	tags.Enable("memory")
	cacheMust(t, r.addProcessed(fx))
	set := r.byDecl[f]
	size := uint64(reflect.TypeOf(rewriteBackSet{}).Size())
	if set == nil || set.header != size || b.Used() != start+size {
		t.Fatal("source expr-set allocation", set, b.Used(), start, size)
	}
	cacheMust(t, r.rescheduleProcessed(f))
	if set.values.Len() != 0 || b.Used() != start+size {
		t.Fatal("source retains empty back set", set, b.Used())
	}
	cacheMust(t, r.close())
	if b.Used() != start {
		t.Fatal("back-set destructor did not release its header", b.Used(), start)
	}
	record := func(action string, line int, body string) string {
		return "-------- [memory] " + action + " src/util/memory_manager.cpp:" + strconv.Itoa(line) + " ---------\n" + body + "\n------------------------------------------------\n"
	}
	id := fmt.Sprintf("%p", set)
	want := record("allocate", 209, "alloc "+id+" src/tactic/ufbv/ufbv_rewriter.cpp:485 expr_set "+strconv.FormatUint(size, 10)) +
		record("deallocate", 204, "dealloc "+id+" src/util/obj_hashtable.h:220")
	if got := output.String(); got != want {
		t.Fatalf("back-set memory records:\ngot  %q\nwant %q", got, want)
	}
}
