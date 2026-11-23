package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/util/approx"
)

func TestSMTMAMMergeRestoresLabelsInTrailOrder(t *testing.T) {
	_, _, _, mam := smtMAMFixture(t)
	node := &SMTEnode{labels: approx.NewUintSet(1), propagatedLabels: approx.NewUintSet(2)}
	empty := new(SMTEnode)
	baseLabels, basePropagated := node.labels, node.propagatedLabels
	mam.AddEqEh(empty, node)
	node.Labels().Insert(3)
	node.PropagatedLabels().Insert(4)
	outerLabels, outerPropagated := node.labels, node.propagatedLabels

	mam.PushScope()
	calls := 0
	mam.trail.push(func() {
		if calls != 1 || node.labels != outerLabels || node.propagatedLabels != outerPropagated {
			t.Fatal("outer callback saw the wrong merge state")
		}
		calls++
	})
	mam.AddEqEh(&SMTEnode{labels: approx.NewUintSet(5), propagatedLabels: approx.NewUintSet(6)}, node)
	mergedLabels, mergedPropagated := node.labels, node.propagatedLabels
	if !mergedLabels.MayContain(5) || !mergedPropagated.MayContain(6) {
		t.Fatal("merge lost source labels")
	}
	mam.PushScope()
	mam.trail.push(func() {
		if calls != 0 || node.labels != mergedLabels || node.propagatedLabels != mergedPropagated {
			t.Fatal("inner callback saw the wrong merge state")
		}
		calls++
	})
	// This merge adds no bits. The public label accessors still allow the
	// caller to change both sets before pop.
	mam.AddEqEh(empty, node)
	node.Labels().Reset()
	node.PropagatedLabels().Reset()
	mam.PopScope(2)
	if calls != 2 || node.labels != outerLabels || node.propagatedLabels != outerPropagated {
		t.Fatal("multi-scope pop lost the outer state")
	}
	mam.Reset()
	if node.labels != baseLabels || node.propagatedLabels != basePropagated {
		t.Fatal("reset lost the level-zero label snapshot")
	}
}

func TestSMTMAMLabelRunAcrossCallbackAndScope(t *testing.T) {
	var trail smtMAMTrail
	base := approx.NewUintSet(7)
	left, right := &SMTEnode{labels: base}, &SMTEnode{labels: base}
	called := false
	trail.saveLabels(left)
	left.labels.Reset()
	trail.push(func() {
		if trail.numScopes() != 0 || !left.labels.Empty() || right.labels != base {
			t.Fatal("base callback observed an incorrect scope or restoration order")
		}
		called = true
	})
	trail.pushScope()
	trail.saveLabels(right)
	right.labels.Reset()
	trail.pushScope()
	trail.saveLabels(right)
	right.propagatedLabels.Insert(11)
	trail.popScope(1)
	if !right.labels.Empty() || !right.propagatedLabels.Empty() {
		t.Fatal("inner pop lost the intermediate state of the same enode")
	}
	trail.popScope(1)
	if left.labels == base || right.labels != base || called {
		t.Fatal("a shared run restored more than the popped scope")
	}
	for _, node := range trail.nodes[len(trail.nodes):cap(trail.nodes)] {
		if node != nil {
			t.Fatal("trail retained a popped enode")
		}
	}
	trail.reset()
	if left.labels != base || !called || cap(trail.nodes) != 0 || cap(trail.labelRuns) != 0 {
		t.Fatal("reset did not restore and release the level-zero trail")
	}
}

func BenchmarkSMTMAMMergeTrail(b *testing.B) {
	for _, width := range []int{1, 256} {
		b.Run(strconv.Itoa(width), func(b *testing.B) {
			mam := new(smtMAMImpl)
			src := SMTEnode{labels: approx.NewUintSet(1)}
			dst := make([]SMTEnode, width)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mam.PushScope()
				for j := range dst {
					mam.AddEqEh(&src, &dst[j])
				}
				mam.PopScope(1)
			}
			if !dst[0].labels.Empty() {
				b.Fatal("merge survived scope pop")
			}
		})
	}
}
