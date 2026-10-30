package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIParserReturnsAssertionConjunctionWithoutMutatingSession(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	parsed, err := f.c.ParseSMT2String("(assert true)(assert false)(check-sat)", SMT2Bindings{})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&parsed)) }()
	formula, err := f.m.RefGet(parsed)
	astTestMust(t, err)
	kind, err := f.m.GetDeclKind(formula)
	astTestMust(t, err)
	if kind != ast.ASTOpAnd {
		t.Fatal("parsed conjunction kind", kind)
	}
	second, err := f.m.GetArg(formula, 1)
	astTestMust(t, err)
	value, err := f.c.BooleanValue(second)
	astTestMust(t, err)
	if value != lbool.False {
		t.Fatal("parsed conjunction false operand", value)
	}
	assertions, err := f.c.Assertions()
	astTestMust(t, err)
	if len(assertions) != 0 {
		t.Fatal("one-shot parser mutated solver session", len(assertions))
	}
}

func TestAPIParserEmptyInputReturnsTrue(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	parsed, err := f.c.ParseSMT2String("", SMT2Bindings{})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&parsed)) }()
	formula, err := f.m.RefGet(parsed)
	astTestMust(t, err)
	value, err := f.c.BooleanValue(formula)
	astTestMust(t, err)
	if value != lbool.True {
		t.Fatal("empty parser conjunction", value)
	}
}

func TestAPIParserResetContinuesOnBorrowedManager(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	parsed, err := f.c.ParseSMT2String("(assert false)(reset)(assert true)", SMT2Bindings{})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&parsed)) }()
	formula, err := f.m.RefGet(parsed)
	astTestMust(t, err)
	value, err := f.c.BooleanValue(formula)
	astTestMust(t, err)
	if value != lbool.True {
		t.Fatal("post-reset parser assertion", value)
	}
}
