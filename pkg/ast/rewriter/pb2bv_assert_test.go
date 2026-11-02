package rewriter

import (
	"errors"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestPB2BVDualBoundSourceAssertion(t *testing.T) {
	m, r := pb2BVFixture(t)
	a := goalTestAtom(t, m, "dual-bound-a")
	b := goalTestAtom(t, m, "dual-bound-b")
	values := pbTestRationals(t, m, "2", "3", "6")
	h, err := m.MkPseudoBooleanGE(values[:2], []ast.ASTHandle{a, b}, &values[2])
	astTestMust(t, err)
	astTestPin(t, m, h)
	decl, err := m.GetDecl(h)
	astTestMust(t, err)
	_, report := enum2BVDebugActions(t, "T")
	got, err := r.weightedPB(ast.ASTPBOpGE, decl, ast.SliceArguments([]ast.ASTHandle{a, b}), big.NewRat(6, 1))
	if diagnostic.DebugBuild {
		var ex z3exception.DefaultException
		if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
			t.Fatal("missing dual bound assertion", err)
		}
		data, e := os.ReadFile(report)
		astTestMust(t, e)
		if !strings.Contains(string(data), "pb2bv_rewriter.cpp") || !strings.Contains(string(data), "175") || !strings.Contains(string(data), "!k.is_neg()") {
			t.Fatal("wrong assertion site", string(data))
		}
	} else {
		astTestMust(t, err)
		if !m.IsFalse(got) {
			t.Fatal("impossible weighted bound did not become false", got)
		}
	}
}

func TestPB2BVNegativeCoefficientSourceAssertion(t *testing.T) {
	m, r := pb2BVFixture(t)
	a := goalTestAtom(t, m, "negative-weight-a")
	debug, report := enum2BVDebugActions(t, "T")
	_, err := r.weighted(pb2BVLE, []ast.ASTHandle{a}, []*big.Rat{big.NewRat(-1, 1)}, big.NewRat(2, 1))
	if diagnostic.DebugBuild {
		var ex z3exception.DefaultException
		if !errors.As(err, &ex) || ex.Message() != "assertion violation" {
			t.Fatal("missing coefficient assertion", err)
		}
		data, e := os.ReadFile(report)
		astTestMust(t, e)
		if !strings.Contains(string(data), "117") || !strings.Contains(string(data), "!m_coeffs[i].is_neg()") {
			t.Fatal("wrong coefficient assertion", string(data))
		}
	} else if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("negative coefficient guard", err)
	}
	astTestMust(t, debug.EnableAssertions(false))
	_, err = r.weighted(pb2BVLE, []ast.ASTHandle{a}, []*big.Rat{big.NewRat(-1, 1)}, big.NewRat(2, 1))
	if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("disabled assertions removed Go coefficient guard", err)
	}
}
