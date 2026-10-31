package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestRecover01CoefficientsTruthTables(t *testing.T) {
	_, m := fixDLFixture(t, false)
	sort, e := m.MkBoolSort()
	equationMust(t, e)
	b := func(name string) ast.ASTHandle {
		sym, e := m.Symbols.Intern(name)
		equationMust(t, e)
		h, e := m.MkConst(sym, sort)
		equationMust(t, e)
		fixDLPin(t, m, h)
		return h
	}
	p, q, r := b("p"), b("q"), b("r")
	neg := func(h ast.ASTHandle) ast.ASTHandle {
		v, e := m.MkNot(h)
		equationMust(t, e)
		fixDLPin(t, m, v)
		return v
	}
	x := fixDLConst(t, m, true, "x")
	clause := func(a, b ast.ASTHandle, n string) ast.ASTHandle {
		eq := collectorEq(t, m, x, fixDLNumber(t, m, true, n))
		h, e := m.MkOrArgs([]ast.ASTHandle{a, b, eq})
		equationMust(t, e)
		fixDLPin(t, m, h)
		return h
	}
	c0 := clause(p, q, "0")
	c1 := clause(neg(p), q, "2")
	c2 := clause(p, neg(q), "-3")
	c3 := clause(neg(p), neg(q), "-1")
	tests := []struct {
		name    string
		clauses []ast.ASTHandle
		valid   bool
	}{
		{"complete", []ast.ASTHandle{c3, c1, c0, c2}, true},
		{"duplicate", []ast.ASTHandle{c0, c1, c2, c3, c1}, true},
		{"missing", []ast.ASTHandle{c0, c1, c2, c2}, false},
		{"inconsistent sum", []ast.ASTHandle{c0, c1, c2, clause(neg(p), neg(q), "5")}, false},
		{"conflicting duplicate", []ast.ASTHandle{c0, c1, c2, c3, clause(neg(p), q, "8")}, false},
		{"different literal", []ast.ASTHandle{c0, c1, c2, clause(neg(p), r, "-1")}, false},
		{"no zero", []ast.ASTHandle{c1, c2, c3, c3}, false},
	}
	decl, e := m.GetDecl(x)
	equationMust(t, e)
	groups, e := recover01Clauses(m, []ast.ASTHandle{c3, c1, c0, c2}, 10)
	equationMust(t, e)
	_, integrated, ok, e := recover01Coefficients(m, groups[decl])
	equationMust(t, e)
	if !ok || integrated[0].RatString() != "2" || integrated[1].RatString() != "-3" {
		t.Fatal("collector integration", integrated)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lits, coeffs, ok, e := recover01Coefficients(m, tc.clauses)
			equationMust(t, e)
			if ok != tc.valid {
				t.Fatalf("valid=%v", ok)
			}
			if ok && (len(lits) != 2 || lits[0] != p || lits[1] != q || coeffs[0].RatString() != "2" || coeffs[1].RatString() != "-3") {
				t.Fatalf("literals %v coefficients %v", lits, coeffs)
			}
		})
	}
}
