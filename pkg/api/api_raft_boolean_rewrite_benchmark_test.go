package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func BenchmarkRaftBooleanRewrite(b *testing.B) {
	context, err := NewSMTContext()
	if err != nil {
		b.Fatal(err)
	}
	defer context.Close()
	m, err := context.Manager()
	if err != nil {
		b.Fatal(err)
	}
	services, err := context.command.ModelEvaluatorConfig()
	if err != nil {
		b.Fatal(err)
	}
	rw, err := rewriter.NewTheoryRewriter(m, services.Context, services.Globals, services.Params)
	if err != nil {
		b.Fatal(err)
	}
	defer rw.Close()
	value := func(h ast.ASTHandle, err error) ast.ASTHandle {
		b.Helper()
		if err != nil {
			b.Fatal(err)
		}
		return h
	}
	atom := value(m.MkFreshConst("rewrite", value(m.MkBoolSort())))
	truth, falsity := value(m.MkTrue()), value(m.MkFalse())
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		b.Fatal(err)
	}
	defer m.DelRef(&out)
	for _, test := range []struct {
		name       string
		term, want ast.ASTHandle
		args       []ast.ASTHandle
	}{
		{"Negation", value(m.MkNot(falsity)), truth, []ast.ASTHandle{falsity}},
		{"Conjunction", value(m.MkAnd(atom, truth)), atom, []ast.ASTHandle{atom, truth}},
		{"Equality", value(m.MkEq(atom, atom)), truth, []ast.ASTHandle{atom, atom}},
		{"Uninterpreted", atom, atom, nil},
	} {
		root, err := m.NewRef(test.term)
		if err != nil {
			b.Fatal(err)
		}
		defer m.DelRef(&root)
		decl := value(m.GetDecl(test.term))
		args := ast.SliceArguments(test.args)
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := rw.MakeApp(decl, args, out); err != nil {
					b.Fatal(err)
				}
				if h, err := m.RefGet(out); err != nil || h != test.want {
					b.Fatalf("result %v, want %v, err %v", h, test.want, err)
				}
			}
		})
	}
}
