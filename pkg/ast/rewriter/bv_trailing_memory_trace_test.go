package rewriter

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestBVTrailingMemoryWrappersFollowLazyCacheLifetime(t *testing.T) {
	symbols, numerals, budget := astTestOwners(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget, TraceTags: tags})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.Close()) }()
	_, err = m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	sort, err := m.BitVectorSort(8)
	astTestMust(t, err)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	root, err := m.NewRef(x)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&root)) }()
	extract, err := NewMkExtractProc(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, extract.Close()) }()
	tags.Enable("memory")
	p, err := NewBVTrailing(extract)
	astTestMust(t, err)
	if _, _, err = p.Count(x); err != nil {
		t.Fatal(err)
	}
	first := p.tables[bvTrailingDepth]
	if first == nil {
		t.Fatal("source lazy cache missing")
	}
	if _, _, err = p.Count(x); err != nil {
		t.Fatal(err)
	}
	astTestMust(t, p.ResetCache(0))
	if _, _, err = p.Count(x); err != nil {
		t.Fatal(err)
	}
	second := p.tables[bvTrailingDepth]
	if second == nil || second == first {
		t.Fatal("source reset must construct a new cache")
	}
	astTestMust(t, p.Close())
	astTestMust(t, p.Close())
	got := output.String()
	if !diagnostic.DebugBuild || !tags.ShouldTrace("memory") {
		if got != "" {
			t.Fatal("source Z3DEBUG/_TRACE memory wrappers must erase", got)
		}
		return
	}
	if strings.Count(got, "[memory] allocate ") != 3 || strings.Count(got, "[memory] deallocate ") != 3 {
		t.Fatal("one wrapper record per actual imp/map lifetime", got)
	}
	impID := fmt.Sprintf("%p", p)
	mapSize := strconv.FormatUint(uint64(reflect.TypeOf(bvTrailingTable{}).Size()), 10)
	// The source charge includes all three call records even before first use.
	header := reflect.TypeOf(BVTrailing{}).Size() - reflect.TypeOf(p.calls).Size()
	impSize := strconv.FormatUint(uint64(header+3*reflect.TypeOf(bvTrailingCall{}).Size()), 10)
	parts := []string{
		"alloc " + impID + " src/ast/rewriter/bv_trailing.cpp:384 imp " + impSize + "\n",
		"alloc " + fmt.Sprintf("%p", first) + " src/ast/rewriter/bv_trailing.cpp:342 map " + mapSize + "\n",
		"dealloc " + fmt.Sprintf("%p", first) + " src/ast/rewriter/bv_trailing.cpp:376\n",
		"alloc " + fmt.Sprintf("%p", second) + " src/ast/rewriter/bv_trailing.cpp:342 map " + mapSize + "\n",
		"dealloc " + fmt.Sprintf("%p", second) + " src/ast/rewriter/bv_trailing.cpp:376\n",
		"dealloc " + impID + " src/ast/rewriter/bv_trailing.cpp:388\n",
	}
	previous := -1
	for _, part := range parts {
		at := strings.Index(got, part)
		if at <= previous || strings.Count(got, part) != 1 {
			t.Fatalf("source wrapper order for %q in %q", part, got)
		}
		previous = at
	}
}
