package rewriter

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	z3exception "github.com/glycerine/g3/pkg/util/z3exception"
)

func TestPB2BVPopSourceAssertionSites(t *testing.T) {
	t.Run("pending-lemmas", func(t *testing.T) {
		m, r := pb2BVFixture(t)
		astTestMust(t, r.context.Clause(0, ast.ASTLiteralVectorSpan{}))
		_, report := enum2BVDebugActions(t, "T")
		err := r.Pop(0)
		if diagnostic.DebugBuild {
			var ex z3exception.DefaultException
			if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
				t.Fatal("pending lemma assertion", err)
			}
			data, err := os.ReadFile(report)
			astTestMust(t, err)
			if !strings.Contains(string(data), "pb2bv_rewriter.cpp") || !strings.Contains(string(data), "439") || !strings.Contains(string(data), "m_lemmas.empty()") {
				t.Fatal("wrong assertion site", string(data))
			}
		} else {
			astTestMust(t, err)
		}
		clauses, err := r.FlushSideConstraints()
		astTestMust(t, err)
		for _, clause := range clauses {
			astTestMust(t, m.DecRef(clause))
		}
	})
	t.Run("scope-limit", func(t *testing.T) {
		_, r := pb2BVFixture(t)
		astTestMust(t, r.Push())
		_, report := enum2BVDebugActions(t, "T")
		err := r.Pop(2)
		if diagnostic.DebugBuild {
			var ex z3exception.DefaultException
			if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
				t.Fatal("scope assertion", err)
			}
			data, err := os.ReadFile(report)
			astTestMust(t, err)
			if !strings.Contains(string(data), "pb2bv_rewriter.cpp") || !strings.Contains(string(data), "441") || !strings.Contains(string(data), "num_scopes <= m_fresh_lim.size()") {
				t.Fatal("wrong assertion site", string(data))
			}
		} else if !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("invalid native scope", err)
		}
		if len(r.freshLimits) != 1 {
			t.Fatal("failed pop changed scope")
		}
	})
}
