package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

func TestInterpolationReadSequenceTheoryPrefix(t *testing.T) {
	f := newInterpolationFixture(t)
	problem, err := f.c.ReadInterpolationProblem(strings.NewReader(`;! THEORY=1
(declare-const theory-p Bool)
(declare-const a Bool)
(declare-const b Bool)
(assert theory-p)
(assert a)
(assert (not b))
`), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(problem.Theory) != 1 || len(problem.Constraints) != 2 || problem.Parents != nil {
		t.Fatalf("wrong sequence layout: theory=%d constraints=%d parents=%v", len(problem.Theory), len(problem.Constraints), problem.Parents)
	}
	if got := interpolationReadName(t, f.m, problem.Theory[0]); got != "theory-p" {
		t.Fatalf("theory formula = %q", got)
	}
	if got := interpolationReadName(t, f.m, problem.Constraints[0]); got != "a" {
		t.Fatalf("first sequence formula = %q", got)
	}
	kind, err := f.m.GetDeclKind(problem.Constraints[1])
	if err != nil || kind != ast.ASTOpNot {
		t.Fatalf("second sequence formula kind=%d, err=%v", kind, err)
	}
}

func TestInterpolationReadTreeParentsAndLocalConstraints(t *testing.T) {
	f := newInterpolationFixture(t)
	problem, err := f.c.ReadInterpolationProblem(strings.NewReader(`(declare-const local-a Bool)
(declare-const node-a Bool)
(declare-const local-root Bool)
(assert (=> local-a node-a))
(assert (=> (and node-a local-root) false))
`), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(problem.Constraints) != 2 || len(problem.Parents) != 2 || problem.Parents[0] != 1 || problem.Parents[1] != interp.ScopeTop {
		t.Fatalf("wrong tree layout: constraints=%d parents=%v", len(problem.Constraints), problem.Parents)
	}
	if got := interpolationReadName(t, f.m, problem.Constraints[0]); got != "local-a" {
		t.Fatalf("leaf local constraint = %q", got)
	}
	if got := interpolationReadName(t, f.m, problem.Constraints[1]); got != "local-root" {
		t.Fatalf("root local constraint = %q", got)
	}
}

func TestInterpolationReadRejectsInvalidTreeAndTheoryHeader(t *testing.T) {
	f := newInterpolationFixture(t)
	unknown := `(declare-const local Bool)
(declare-const root Bool)
(declare-const missing Bool)
(assert (=> local root))
(assert (=> (and missing local) false))
`
	if _, err := f.c.ReadInterpolationProblem(strings.NewReader(unknown), true); err == nil || !strings.Contains(err.Error(), "unknown child") {
		t.Fatalf("unknown tree child error = %v", err)
	}
	if _, err := f.c.ReadInterpolationProblem(strings.NewReader(";! THEORY=not-a-number\n(assert true)\n"), false); err == nil {
		t.Fatal("invalid theory count was accepted")
	}
}

func TestInterpolationReadEmptySequenceAndTheoryCountClamp(t *testing.T) {
	f := newInterpolationFixture(t)
	problem, err := f.c.ReadInterpolationProblem(strings.NewReader(";! THEORY=3\n"), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(problem.Constraints) != 0 || len(problem.Theory) != 0 || problem.Parents != nil {
		t.Fatalf("empty sequence decoded as constraints=%v theory=%v parents=%v", problem.Constraints, problem.Theory, problem.Parents)
	}
}

func interpolationReadName(t *testing.T, manager *ast.ASTManager, expression ast.ASTHandle) string {
	t.Helper()
	declaration, err := manager.GetDecl(expression)
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.GetASTName(declaration)
	if err != nil {
		t.Fatal(err)
	}
	text, err := manager.Symbols.String(name)
	if err != nil {
		t.Fatal(err)
	}
	return text
}
