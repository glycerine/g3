package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Exercise the call used on a cached_var_subst miss. One case retains all
// expected instances in the AST table. The other retires previous instances.
func BenchmarkSMTQuantifierBodySubstitution(b *testing.B) {
	for _, retain := range []bool{true, false} {
		name := "RetireInstances"
		if retain {
			name = "RetainInstances"
		}
		b.Run(name, func(b *testing.B) {
			m := astTestManager(b)
			sort, err := m.MkBoolSort()
			benchmarkMust(b, err)
			decl, err := m.MkFuncDecl(astTestSymbol(b, m, "predicate"), []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{})
			benchmarkMust(b, err)
			body := func(a, z ast.ASTHandle) ast.ASTHandle {
				left, err := m.MkApp(decl, []ast.ASTHandle{a, z})
				benchmarkMust(b, err)
				right, err := m.MkApp(decl, []ast.ASTHandle{z, a})
				benchmarkMust(b, err)
				neg, err := m.MkNot(left)
				benchmarkMust(b, err)
				result, err := m.MkIte(left, right, neg)
				benchmarkMust(b, err)
				return result
			}
			pin := func(h ast.ASTHandle) ast.ASTRef {
				r, err := m.NewRef(h)
				benchmarkMust(b, err)
				b.Cleanup(func() { benchmarkMust(b, m.DelRef(&r)) })
				return r
			}
			x, err := m.MkVar(0, sort)
			benchmarkMust(b, err)
			y, err := m.MkVar(1, sort)
			benchmarkMust(b, err)
			formula := body(x, y)
			pin(formula)
			var args [64]ast.ASTHandle
			for i := range args {
				args[i], err = m.MkConst(astTestSymbol(b, m, "c"+strconv.Itoa(i)), sort)
				benchmarkMust(b, err)
				pin(args[i])
			}
			var expected [64]ast.ASTHandle
			if retain {
				for i := range args {
					expected[i] = body(args[(i+1)%len(args)], args[i])
					pin(expected[i])
				}
			}
			out := pin(ast.ASTHandle{})
			sub, err := rewriter.NewVarSubstitution(m, true)
			benchmarkMust(b, err)
			b.Cleanup(func() { benchmarkMust(b, sub.Close()) })
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				k := i % len(args)
				bindings := [2]ast.ASTHandle{args[k], args[(k+1)%len(args)]}
				if err := sub.Substitute(formula, bindings[:], out); err != nil {
					b.Fatal(err)
				}
				got, err := m.RefGet(out)
				if err != nil || got.IsNull() || retain && got != expected[k] {
					b.Fatalf("unexpected substitution %v (%v)", got, err)
				}
			}
			b.StopTimer()
			if b.N > 0 && !retain {
				k := (b.N - 1) % len(args)
				want := body(args[(k+1)%len(args)], args[k])
				got, err := m.RefGet(out)
				if err != nil || got != want {
					b.Fatalf("last instance %v, want %v (%v)", got, want, err)
				}
			}
		})
	}
}

// BenchmarkSMTQuantifierInstanceStream runs the normal quantifier-cache miss
// path on unique ordered tuples. Each group of 1,024 tuples shares the same
// body and 32 binding terms. Reset discards the batch of retained instances.
func BenchmarkSMTQuantifierInstanceStream(b *testing.B) {
	benchmarkSMTQuantifierInstanceStream(b, 32, false)
}

func BenchmarkSMTBoundedQuantifierInstanceStream(b *testing.B) {
	benchmarkSMTQuantifierInstanceStream(b, 32, true)
}

func BenchmarkSMTQuantifierInstanceBatches(b *testing.B) {
	for _, width := range []int{4, 8, 16, 32} {
		b.Run(strconv.Itoa(width*width), func(b *testing.B) {
			benchmarkSMTQuantifierInstanceStream(b, width, false)
		})
	}
}

func benchmarkSMTQuantifierInstanceStream(b *testing.B, width int, bounded bool) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	benchmarkMust(b, err)
	x, err := m.MkVar(0, sort)
	benchmarkMust(b, err)
	y, err := m.MkVar(1, sort)
	benchmarkMust(b, err)
	decl, err := m.MkFuncDecl(astTestSymbol(b, m, "relation"), []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{})
	benchmarkMust(b, err)
	app := func(a, binding ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(decl, []ast.ASTHandle{a, binding})
		benchmarkMust(b, err)
		return h
	}
	xy, yx := app(x, y), app(y, x)
	shared, err := m.MkIte(xy, yx, x)
	benchmarkMust(b, err)
	body, err := m.MkIte(yx, app(shared, x), app(y, shared))
	benchmarkMust(b, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort, sort}, Names: []symbol.Symbol{symbol.Numerical(1), symbol.Numerical(2)}, Body: body})
	benchmarkMust(b, err)
	pin := func(h ast.ASTHandle) {
		ref, err := m.NewRef(h)
		benchmarkMust(b, err)
		b.Cleanup(func() { benchmarkMust(b, m.DelRef(&ref)) })
	}
	pin(q)
	atoms := make([]*SMTEnode, width)
	for i := range atoms {
		h, node := smtEnodeConstant(b, m, uint32(i+10), sort, 0)
		pin(h)
		atoms[i] = node
	}
	cache, err := NewSMTCachedVarSubstitution(m)
	benchmarkMust(b, err)
	b.Cleanup(func() { benchmarkMust(b, cache.Close()) })
	out, err := m.NewRef(ast.ASTHandle{})
	benchmarkMust(b, err)
	b.Cleanup(func() { benchmarkMust(b, m.DelRef(&out)) })
	limit := m.ResourceLimit()
	if bounded {
		benchmarkMust(b, limit.Push(6_000_000))
		defer limit.Pop()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := i % (len(atoms) * len(atoms))
		if k == 0 && i != 0 {
			benchmarkMust(b, cache.Reset())
			if bounded {
				limit.Pop()
				benchmarkMust(b, limit.Push(6_000_000))
			}
		}
		tuple := [2]*SMTEnode{atoms[k/len(atoms)], atoms[k%len(atoms)]}
		hit, err := cache.Instantiate(q, tuple[:], out)
		if err != nil || hit {
			b.Fatalf("tuple %d: cached=%v, err=%v", k, hit, err)
		}
	}
	b.StopTimer()
	if b.N > 0 {
		last := (b.N - 1) % (width * width)
		x, y := atoms[last%width].owner, atoms[last/width].owner
		xy, yx := app(x, y), app(y, x)
		shared, err := m.MkIte(xy, yx, x)
		benchmarkMust(b, err)
		want, err := m.MkIte(yx, app(shared, x), app(y, shared))
		benchmarkMust(b, err)
		got, err := m.RefGet(out)
		if err != nil || got != want {
			b.Fatalf("last instance %v, want %v (%v)", got, want, err)
		}
	}
}
