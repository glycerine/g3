package ast

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestASTNondeterminismTraceFollowsProcessDebugTag(t *testing.T) {
	m := astTestManager(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	m.traceTags = tags
	budget := arena.NewBudget(^uint64(0))
	region, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: budget}})
	astTestMust(t, err)
	debug, err := diagnostic.NewDebugContext(region, budget)
	astTestMust(t, err)
	inputFile, err := os.CreateTemp(t.TempDir(), "nondet-input-*")
	astTestMust(t, err)
	_, err = inputFile.WriteString("CCT")
	astTestMust(t, err)
	_, err = inputFile.Seek(0, 0)
	astTestMust(t, err)
	outputFile, err := os.CreateTemp(t.TempDir(), "nondet-output-*")
	astTestMust(t, err)
	streams, err := diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: budget, Stdout: outputFile, Stderr: outputFile})
	astTestMust(t, err)
	input, err := diagnostic.NewDebuggerCharInput(streams, inputFile)
	astTestMust(t, err)
	action, err := diagnostic.NewDebugActionContext(diagnostic.DebugActionConfig{Budget: budget, Streams: streams, Input: input})
	astTestMust(t, err)
	astTestMust(t, debug.BindAssertionStreams(streams))
	astTestMust(t, debug.BindAssertionActions(action))
	previous, err := diagnostic.BindProcessDebugContext(debug)
	astTestMust(t, err)
	t.Cleanup(func() {
		_, e := diagnostic.BindProcessDebugContext(previous)
		astTestMust(t, e)
		astTestMust(t, action.Close())
		astTestMust(t, input.Close())
		astTestMust(t, streams.Close())
		astTestMust(t, inputFile.Close())
		astTestMust(t, outputFile.Close())
		astTestMust(t, debug.Close())
		astTestMust(t, region.Close())
	})
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	existing, err := m.MkVar(17, sort)
	astTestMust(t, err)
	astTestPin(t, m, existing)
	id, err := m.GetASTID(existing)
	astTestMust(t, err)
	m.mu.Lock()
	candidate, err := m.allocateNode(ASTVar)
	if err != nil {
		m.mu.Unlock()
		t.Fatal(err)
	}
	m.varAt(candidate.slot).index = m.varAt(existing.slot).index
	m.varAt(candidate.slot).sort = m.varAt(existing.slot).sort
	hash, err := m.nodeHash(candidate)
	if err != nil {
		m.freeNodeStorage(candidate)
		m.mu.Unlock()
		t.Fatal(err)
	}
	m.nodeAt(candidate.slot).hash = hash
	var bucket []astNodeRef
	for _, s := range m.table.slots {
		if s.ref.slot != 0 && s.hash == hash {
			bucket = append(bucket, s.ref)
		}
	}
	for _, r := range bucket {
		m.table.unindex(r, hash)
	}
	defer func() {
		for _, r := range bucket {
			m.table.place(hash, r)
		}
		m.freeNodeStorage(candidate)
		m.mu.Unlock()
	}()
	check := func() error {
		_, e := m.registrationBefore(candidate)
		return e
	}
	if err = check(); err != nil || output.Len() != 0 {
		t.Errorf("disabled debug tag: %v, output %q", err, output.String())
	}
	astTestMust(t, debug.EnableDebug("nondet_bug"))
	if err = check(); err != nil {
		t.Errorf("enabled assertion without TRACE: %v", err)
	}
	if output.Len() != 0 {
		t.Errorf("disabled trace tag wrote %q", output.String())
	}
	tags.Enable("nondet_bug")
	astTestMust(t, debug.EnableAssertions(false))
	if err = check(); err != nil || output.Len() != 0 {
		t.Errorf("disabled assertion scanned canonical table: %v, output %q", err, output.String())
	}
	astTestMust(t, debug.EnableAssertions(true))
	err = check()
	if err != nil {
		t.Errorf("conditional debug scan: %v", err)
	}
	if ASTSourceDebug && tags.ShouldTrace("nondet_bug") {
		want := "-------- [nondet_bug] slow_not_contains src/ast/ast.cpp:1650 ---------\n" +
			fmt.Sprintf("id1:   %d, id2: 4294967295\nhash1: %d, hash2: %d\n", id, hash, hash) +
			"------------------------------------------------\n"
		if got := output.String(); got != want {
			t.Errorf("source nondeterminism trace: got %q, want %q", got, want)
		}
	} else if output.Len() != 0 {
		t.Errorf("erased nondeterminism trace wrote %q", output.String())
	}
	if ASTSourceDebug {
		var ex z3exception.DefaultException
		if err = check(); !errors.As(err, &ex) {
			t.Errorf("T action did not transport source exception: %v", err)
		}
		if tags.ShouldTrace("nondet_bug") && strings.Count(output.String(), "[nondet_bug] slow_not_contains") != 2 {
			t.Errorf("C and T actions did not both follow source TRACE: %q", output.String())
		}
		state, e := input.State()
		astTestMust(t, e)
		if state.Position != 3 {
			t.Errorf("debug action input position: %d", state.Position)
		}
		contents, e := os.ReadFile(outputFile.Name())
		astTestMust(t, e)
		want := "ASSERTION VIOLATION\nFile: src/ast/ast.cpp\nLine: 1671\ncontains || slow_not_contains(n)\n" +
			"(C)ontinue, (A)bort, (S)top, (T)hrow exception, Invoke (G)DB\n"
		if got := string(contents); got != strings.Repeat(want, 3) {
			t.Errorf("source assertion notifications/actions: got %q, want %q", got, strings.Repeat(want, 3))
		}
	}
}
