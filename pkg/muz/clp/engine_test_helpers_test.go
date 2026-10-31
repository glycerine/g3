package clp_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

type testSolver struct {
	checks       []lbool.LBool
	asserted     []ast.ASTHandle
	pushes, pops int
	fail         error
}

func (s *testSolver) Push() error                  { s.pushes++; return s.fail }
func (s *testSolver) Pop(uint32) error             { s.pops++; return s.fail }
func (s *testSolver) Assert(h ast.ASTHandle) error { s.asserted = append(s.asserted, h); return s.fail }
func (s *testSolver) CheckSat([]ast.ASTHandle) (lbool.LBool, error) {
	if s.fail != nil {
		return lbool.Undef, s.fail
	}
	if len(s.checks) == 0 {
		return lbool.True, nil
	}
	r := s.checks[0]
	s.checks = s.checks[1:]
	return r, nil
}

type fixture struct {
	m      *ast.ASTManager
	rm     *base.RuleManager
	set    *base.RuleSet
	app    map[string]ast.ASTHandle
	solver *testSolver
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	c, e := cmd_context.NewStandaloneContext()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = c.Close() })
	m, e := c.Manager()
	if e != nil {
		t.Fatal(e)
	}
	rm, e := base.NewRuleManager(m)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = rm.Close() })
	set, e := base.NewRuleSet(rm)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = set.Close() })
	b, _ := m.MkBoolSort()
	f := &fixture{m: m, rm: rm, set: set, app: map[string]ast.ASTHandle{}, solver: &testSolver{}}
	for _, n := range []string{"P", "Q"} {
		sym, _ := m.Symbols.Intern("clp-" + n)
		d, x := m.MkFuncDecl(sym, nil, b, ast.ASTFuncDeclInfo{})
		if x != nil {
			t.Fatal(x)
		}
		_ = rm.RegisterPredicate(d)
		f.app[n], _ = m.MkApp(d, nil)
	}
	f.add(t, "P", nil)
	f.add(t, "Q", []string{"P"})
	ok, e := set.Finalize()
	if e != nil || !ok {
		t.Fatal(ok, e)
	}
	return f
}
func (f *fixture) add(t *testing.T, h string, tail []string) {
	xs := make([]ast.ASTHandle, len(tail))
	for i, n := range tail {
		xs[i] = f.app[n]
	}
	r, e := f.rm.NewRule(f.app[h], xs, nil, h)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.set.Add(r); e != nil {
		t.Fatal(e)
	}
	_ = r.Close()
}
