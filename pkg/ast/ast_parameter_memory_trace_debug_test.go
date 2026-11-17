//go:build g3debug && g3trace

package ast

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestASTParameterRationalMemoryWrapperCopyMoveAssignAndDestroy(t *testing.T) {
	sy, q, b := astTestOwners(t)
	var output bytes.Buffer
	tags := udiag.NewTraceTagsWithWriter(&output)
	m, err := NewASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b, TraceTags: tags})
	astTestMust(t, err)
	tags.Enable("memory")
	v, err := q.NewValue()
	astTestMust(t, err)
	astTestMust(t, q.SetString(&v, "7/3"))
	a, err := m.NewParameterRational(&v)
	astTestMust(t, err)
	first := m.rationalTraceName(m.parameterAt(a.slot).rationalTraceID)
	copy, err := m.CopyParameter(a)
	astTestMust(t, err)
	secondID := m.parameterAt(copy.slot).rationalTraceID
	second := m.rationalTraceName(secondID)
	beforeMove := output.String()
	moved, err := m.MoveParameter(copy)
	astTestMust(t, err)
	if m.parameterAt(moved.slot).rationalTraceID != secondID || m.parameterAt(copy.slot).rationalTraceID != 0 || output.String() != beforeMove {
		t.Fatal("move changed source rational* allocation identity")
	}
	astTestMust(t, m.DelParameter(&copy))
	astTestMust(t, m.AssignParameter(a, moved))
	third := m.rationalTraceName(m.parameterAt(a.slot).rationalTraceID)
	astTestMust(t, m.DelParameter(&moved))
	astTestMust(t, m.DelParameter(&a))
	astTestMust(t, q.Del(&v))
	size := strconv.FormatUint(uint64(reflect.TypeOf(unumeral.OwnedRational{}).Size()), 10)
	alloc := func(id, file string, line int) string {
		return "-------- [memory] allocate src/util/memory_manager.cpp:209 ---------\n" +
			"alloc " + id + " " + file + ":" + strconv.Itoa(line) + " rational " + size + "\n" +
			"------------------------------------------------\n"
	}
	dealloc := func(id string, line int) string {
		return "-------- [memory] deallocate src/util/memory_manager.cpp:204 ---------\n" +
			"dealloc " + id + " src/ast/ast.cpp:" + strconv.Itoa(line) + "\n" +
			"------------------------------------------------\n"
	}
	want := alloc(first, "src/ast/ast.h", 124) +
		alloc(second, "src/ast/ast.cpp", 60) +
		dealloc(first, 53) +
		alloc(third, "src/ast/ast.cpp", 60) +
		dealloc(second, 38) +
		dealloc(third, 38)
	if got := output.String(); got != want || first == second || first == third || second == third {
		t.Fatal(fmt.Sprintf("source rational records:\n got %q\nwant %q", got, want))
	}
	tags.Disable("memory")
	astTestMust(t, m.Close())
}
