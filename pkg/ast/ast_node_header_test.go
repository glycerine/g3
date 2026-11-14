package ast

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func headerTerm(t *testing.T, m *ASTManager, sort ASTHandle, name string) ASTHandle {
	t.Helper()
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	value, err := m.MkConst(symbol, sort)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

var _ = usymbol.Symbol{}

// A header must agree with the single accessors it replaces.
func TestNodeHeaderMatchesSingleAccessors(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	a, b := headerTerm(t, m, boolean, "header-a"), headerTerm(t, m, boolean, "header-b")
	or, err := m.MkOr(a, b)
	if err != nil {
		t.Fatal(err)
	}
	not, err := m.MkNot(or)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []ASTHandle{a, b, or, not} {
		header, err := m.NodeHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		kind, _ := m.GetKind(h)
		count, _ := m.GetNumArgs(h)
		family, _ := m.GetFamilyID(h)
		op, _ := m.GetDeclKind(h)
		decl, _ := m.GetDecl(h)
		refs, _ := m.GetRefCount(h)
		if header.Kind != kind || header.NumArgs != count || header.Family != family || header.DeclKind != op || header.Decl != decl || header.RefCount != refs {
			t.Fatalf("header %+v differs: kind %v count %d family %d op %d decl %v refs %d", header, kind, count, family, op, decl, refs)
		}
	}
}

func TestNodeHeaderForNonApplications(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	variable, err := m.MkVar(0, boolean)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []ASTHandle{boolean, variable} {
		header, err := m.NodeHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		kind, _ := m.GetKind(h)
		if header.Kind != kind || header.NumArgs != 0 || !header.Decl.IsNull() || header.Family != -1 || header.DeclKind != -1 {
			t.Fatalf("non-application header %+v", header)
		}
	}
}

func TestNodeHeaderRefusesBadHandlesAndClosedManagers(t *testing.T) {
	if !ASTValidateHandles {
		t.Skip("handle validation is compiled out of normal builds (ast_validate_handles_off.go)")
	}
	first, second := astTestManager(t), astTestManager(t)
	boolean, err := first.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	value := headerTerm(t, first, boolean, "header-foreign")
	if _, err = second.NodeHeader(value); !errors.Is(err, ErrASTHandle) {
		t.Fatal("foreign handle", err)
	}
	if _, err = first.NodeHeader(ASTHandle{}); !errors.Is(err, ErrASTHandle) {
		t.Fatal("null handle", err)
	}
	var missing *ASTManager
	if _, err = missing.NodeHeader(value); !errors.Is(err, ErrASTHandle) {
		t.Fatal("nil manager", err)
	}
	ref, err := first.NewRef(value)
	if err != nil {
		t.Fatal(err)
	}
	stale := value
	if err = first.DelRef(&ref); err != nil {
		t.Fatal(err)
	}
	if _, err = first.NodeHeader(stale); !errors.Is(err, ErrASTHandle) {
		t.Fatal("retired handle", err)
	}
	closing := astTestManager(t)
	survivor, err := closing.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	closing.Closed = true
	defer func() { closing.Closed = false }()
	if _, err = closing.NodeHeader(survivor); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed manager", err)
	}
}

func TestNodeHeaderAndArgumentReadsDoNotAllocate(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	a, b := headerTerm(t, m, boolean, "alloc-a"), headerTerm(t, m, boolean, "alloc-b")
	or, err := m.MkOr(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if allocs := testing.AllocsPerRun(200, func() {
		_, _ = m.NodeHeader(or)
		_, _ = m.GetArg(or, 1)
	}); allocs != 0 {
		t.Fatalf("%v allocations", allocs)
	}
}

// The argument read now validates once for both words of a reference.
func TestGetArgStillReturnsEachArgument(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	terms := []ASTHandle{headerTerm(t, m, boolean, "arg-a"), headerTerm(t, m, boolean, "arg-b"), headerTerm(t, m, boolean, "arg-c")}
	or, err := m.MkOrArgs(terms)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range terms {
		got, err := m.GetArg(or, uint32(i))
		if err != nil || got != want {
			t.Fatal(i, got, want, err)
		}
	}
	if _, err = m.GetArg(or, 3); !errors.Is(err, ErrASTDomain) {
		t.Fatal("out of range argument", err)
	}
}

func TestSortHandlesOrdersBySlotThenGeneration(t *testing.T) {
	handles := []ASTHandle{{slot: 9, generation: 1}, {slot: 2, generation: 7}, {slot: 2, generation: 3}, {slot: 5, generation: 1}, {}}
	SortHandles(handles)
	want := []ASTHandle{{}, {slot: 2, generation: 3}, {slot: 2, generation: 7}, {slot: 5, generation: 1}, {slot: 9, generation: 1}}
	for i := range want {
		if handles[i] != want[i] {
			t.Fatal(i, handles)
		}
	}
	SortHandles(nil)
}
