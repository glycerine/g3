package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFMInputParserForbiddenVariableDefersConstraint(t *testing.T) {
	p, m := fmParserFixture(t, false, false)
	x := fixDLConst(t, m, false, "fm_forbidden_x")
	equationMust(t, p.Forbid(x))
	bound := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "2"))
	c, registration, matched, e := p.Parse(bound, ast.ASTDependency{})
	equationMust(t, e)
	if matched || c != nil || registration != FMRegistrationDeferred {
		t.Fatalf("all-forbidden formula should not be OCC: %#v %v %v", c, registration, matched)
	}
}

func TestFMInputParserCloseIsIdempotent(t *testing.T) {
	_, m := fixDLFixture(t, false)
	store, e := NewFMConstraintStore(m)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, store.Close()) })
	p, e := NewFMInputParser(m, store, false, false)
	equationMust(t, e)
	equationMust(t, p.Close())
	equationMust(t, p.Close())
}
