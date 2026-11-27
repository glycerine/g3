package smtcore

import (
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// A caller may keep enode pointers after the scope that created them is popped.
func BenchmarkSMTRetiredEnodeReferences(b *testing.B) {
	const width, roots, stride = 2048, 64, 32
	f := newSMTEqualityUndoFixture(b, "retired-enode-references", roots)
	terms := make([]ast.ASTHandle, width)
	for i := range terms {
		h, err := f.m.MkApp(f.binary, []ast.ASTHandle{f.base[i%roots], f.base[i/roots]})
		astTestMust(b, err)
		astTestPin(b, f.m, h)
		terms[i] = h
	}
	refs := make([]*SMTEnode, width/stride)
	var retained int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		astTestMust(b, f.s.PushScope())
		for j, h := range terms {
			astTestMust(b, f.s.Internalize(h, false))
			if j%stride == 0 {
				refs[j/stride] = f.s.context.enodes.get(h)
			}
		}
		astTestMust(b, f.s.PopScopes(1))
		b.StopTimer()
		for _, node := range refs {
			if node == nil || node.root != nil {
				b.Fatal("expected a retired enode")
			}
		}
		held := scopeBenchmarkLiveHeap()
		clear(refs)
		retained += held - scopeBenchmarkLiveHeap()
		b.StartTimer()
	}
	b.StopTimer()
	b.ReportMetric(float64(retained)/float64(b.N), "retained-B/solver")
	runtime.KeepAlive(refs)
	runtime.KeepAlive(f)
}
