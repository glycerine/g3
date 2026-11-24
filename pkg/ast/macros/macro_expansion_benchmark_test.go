package macros_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/macros"
)

type macroExpansionCase struct {
	name           string
	term           ast.ASTHandle
	want           ast.ASTHandle
	unchangedSteps uint32
}

func macroExpansionCases(t testing.TB) (*ast.ASTManager, *macros.Manager, []macroExpansionCase) {
	t.Helper()
	m := fixture(t)
	manager, err := macros.NewManager(m)
	must(t, err)
	t.Cleanup(func() { must(t, manager.Close()) })
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(t, err)
		ref, err := m.NewRef(h)
		must(t, err)
		t.Cleanup(func() { must(t, m.DelRef(&ref)) })
		return h
	}
	b := pin(m.MkBoolSort())
	f := declaration(t, m, "macro-f", []ast.ASTHandle{b}, b)
	g := declaration(t, m, "opaque-g", []ast.ASTHandle{b}, b)
	pair := declaration(t, m, "opaque-pair", []ast.ASTHandle{b, b}, b)
	leaf := pin(m.MkApp(declaration(t, m, "a", nil, b), nil))
	x := pin(m.MkVar(0, b))
	def := quantifiedMacro(t, m, f, x, b)
	ok, err := manager.Insert(f, def, ast.ASTHandle{}, ast.ASTDependency{})
	must(t, err)
	if !ok {
		t.Fatal("failed to install identity macro")
	}
	chain := leaf
	for range 5 {
		chain = pin(m.MkApp(g, []ast.ASTHandle{chain}))
	}
	shared := pin(m.MkApp(pair, []ast.ASTHandle{chain, chain}))
	match := pin(m.MkApp(f, []ast.ASTHandle{chain}))
	truth := pin(m.MkTrue())
	conditional := pin(m.MkIte(truth, chain, leaf))
	return m, manager, []macroExpansionCase{
		{"Leaf", leaf, leaf, 1},
		{"Chain", chain, chain, 10},
		{"SharedSubterm", shared, shared, 12},
		{"Substitution", match, chain, 0},
		{"ConstantConditional", conditional, chain, 0},
	}
}

// The macro finder visits every pending formula when any declaration has a
// macro. Measure misses, a real substitution and a built-in ITE reduction.
func BenchmarkMacroExpansion(b *testing.B) {
	m, manager, cases := macroExpansionCases(b)
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			out, err := m.NewRef(ast.ASTHandle{})
			must(b, err)
			defer func() { must(b, m.DelRef(&out)) }()
			proof, err := m.NewRef(ast.ASTHandle{})
			must(b, err)
			defer func() { must(b, m.DelRef(&proof)) }()
			b.ReportAllocs()
			for b.Loop() {
				_, err = manager.ExpandMacros(tc.term, ast.ASTHandle{}, ast.ASTDependency{}, out, proof)
				if err != nil {
					b.Fatal(err)
				}
			}
			got, err := m.RefGet(out)
			must(b, err)
			if got != tc.want {
				b.Fatalf("expanded term %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMacroExpansionResourceSteps(t *testing.T) {
	m, manager, cases := macroExpansionCases(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := m.NewRef(ast.ASTHandle{})
			must(t, err)
			defer func() { must(t, m.DelRef(&out)) }()
			proof, err := m.NewRef(ast.ASTHandle{})
			must(t, err)
			defer func() { must(t, m.DelRef(&proof)) }()
			limit := m.ResourceLimit()
			if tc.unchangedSteps != 0 {
				must(t, limit.Push(tc.unchangedSteps))
				defer limit.Pop()
			}
			before := limit.Count()
			dependency, err := manager.ExpandMacros(tc.term, ast.ASTHandle{}, ast.ASTDependency{}, out, proof)
			must(t, err)
			got, err := m.RefGet(out)
			must(t, err)
			if got != tc.want || !dependency.IsNull() {
				t.Fatalf("expanded term %v, dependency %v, want %v and a null dependency", got, dependency, tc.want)
			}
			if cost := limit.Count() - before; tc.unchangedSteps != 0 && cost != uint64(tc.unchangedSteps) {
				t.Fatalf("charged %d steps, want %d", cost, tc.unchangedSteps)
			}
		})
	}
}
