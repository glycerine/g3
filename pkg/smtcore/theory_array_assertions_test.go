package smtcore

// Source: src/smt/theory_array.cpp:73-74,94.

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestSMTOrdinaryArraySourceAssertionGateAndShape(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	astTestMust(t, d.EnableAssertions(false))
	evaluated := false
	if err := arraySourceAssert(94, "is_select(s)", func() bool {
		evaluated = true
		return false
	}); err != nil || evaluated {
		t.Fatal("disabled SASSERT evaluated its condition", evaluated, err)
	}

	manager := astTestManager(t)
	family, err := manager.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, manager, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	value, err := manager.MkTrue()
	astTestMust(t, err)
	constant, err := manager.MkConstArray(arraySort, value)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(constant, false))
	node := state.context.enodes.get(constant)
	variable := node.TheoryVar(family)
	if err = theory.addParentSelect(variable, node); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("disabled source assertion changed the physical shape guard", err)
	}

	astTestMust(t, d.EnableAssertions(true))
	err = theory.addParentSelect(variable, node)
	if diagnostic.DebugBuild {
		var assertion z3exception.DefaultException
		if !errors.As(err, &assertion) || assertion.Message() != "assertion violation" {
			t.Fatal("enabled source assertion did not reject a non-select parent", err)
		}
	} else if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("release shape guard changed", err)
	}
	reportText, readErr := os.ReadFile(report)
	astTestMust(t, readErr)
	if diagnostic.DebugBuild {
		if !strings.Contains(string(reportText), "File: src/smt/theory_array.cpp\nLine: 94\nis_select(s)\n") {
			t.Fatal("ordinary-array source assertion report", string(reportText))
		}
	} else if len(reportText) != 0 {
		t.Fatal("release SASSERT was not erased", string(reportText))
	}
}

func TestSMTOrdinaryArraySourceVerifyIgnoresAssertionSwitch(t *testing.T) {
	d, report := newLRASourceAssertionContext(t)
	astTestMust(t, d.EnableAssertions(false))
	evaluated := false
	err := arraySourceVerify(73, "r == static_cast<theory_var>(m_find.mk_var())", func() bool {
		evaluated = true
		return false
	})
	if !evaluated {
		t.Fatal("VERIFY did not evaluate its condition")
	}
	if diagnostic.DebugBuild {
		var assertion z3exception.DefaultException
		if !errors.As(err, &assertion) || assertion.Message() != "assertion violation" {
			t.Fatal("VERIFY throw consulted the assertion switch", err)
		}
	} else if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
		t.Fatal("release VERIFY did not preserve its native unreachable guard", err)
	}
	reportText, readErr := os.ReadFile(report)
	astTestMust(t, readErr)
	if !strings.Contains(string(reportText), "Failed to verify: r == static_cast<theory_var>(m_find.mk_var())\n") {
		t.Fatal("ordinary-array VERIFY prefix", string(reportText))
	}
	if diagnostic.DebugBuild && !strings.Contains(string(reportText), "File: src/smt/theory_array.cpp\nLine: 73\nUNREACHABLE CODE WAS REACHED.\n") {
		t.Fatal("ordinary-array VERIFY source location", string(reportText))
	}
}
