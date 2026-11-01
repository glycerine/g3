package sls

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSLSRandomizeLocalSourceTrace(t *testing.T) {
	for _, mode := range []string{"disabled", "enabled", "refusing"} {
		t.Run(mode, func(t *testing.T) {
			var output bytes.Buffer
			var writer io.Writer = &output
			if mode == "refusing" {
				writer = slsRefusingTrace{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			if mode != "disabled" {
				tags.Enable("sls")
			}
			_, search, evaluator, tracker, manager, numbers := slsEngineFixture(t, tags)
			if manager.BorrowSourceTraceTags() != tags {
				t.Fatal("trace fixture binding")
			}
			if mode != "disabled" && !tags.ShouldTrace("sls") {
				t.Skip("source trace sites are compiled out")
			}
			if mode == "disabled" && tags.ShouldTrace("sls") {
				t.Fatal("disabled tag unexpectedly enabled")
			}
			p := slsBoolConst(t, manager, "sls-random-trace-p")
			q := slsBoolConst(t, manager, "sls-random-trace-q")
			root, err := manager.MkAnd(p, q)
			if err != nil {
				t.Fatal(err)
			}
			slsPin(t, manager, p, q, root)
			if err := search.Initialize([]ast.ASTHandle{root}); err != nil {
				t.Fatal(err)
			}
			if err := search.Reset(); err != nil {
				t.Fatal(err)
			}
			decl, err := manager.GetDecl(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := evaluator.RandomizeLocal(search, []ast.ASTHandle{decl}); err != nil {
				t.Fatal(err)
			}
			if mode != "enabled" {
				if output.Len() != 0 {
					t.Fatal("unexpected trace output", output.String())
				}
				return
			}
			value, err := tracker.GetValue(p)
			if err != nil {
				t.Fatal(err)
			}
			text, err := numbers.String(value)
			if err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf("-------- [sls] randomize_local src/tactic/sls/sls_evaluator.h:808 ---------\nRandomization candidate: sls-random-trace-p\nLocally randomized model: \nsls-random-trace-p = %s\nsls-random-trace-q = 0\n------------------------------------------------\n", text)
			if output.String() != want {
				t.Fatalf("source trace %q, want %q", output.String(), want)
			}
		})
	}
}
