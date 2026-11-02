package duality

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestStratifiedLeavesAndOverapproximationsPreserveSourceStates(t *testing.T) {
	for _, stratified := range []bool{false, true} {
		t.Run(fmt.Sprint(stratified), func(t *testing.T) {
			ctx, _, aux := dualityNativeSolverFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			term, err := ctx.BoolConst("leaf-R")
			must(err)
			closeExpr(t, term)
			source, err := NewRPFP(ctx)
			must(err)
			defer source.Close()
			relation, err := source.CreateNode(term)
			must(err)
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			var report bytes.Buffer
			graph.streamReporter = &StreamReporter{Format: ConjectureReporter{Output: &report}}
			covering, err := NewCovering(graph, aux)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			u.StratifiedInlining = stratified
			leaf, err := u.CreateLeaf(relation)
			must(err)
			if !strings.Contains(report.String(), "]node 1: ") {
				t.Fatal("missing leaf event", report.String())
			}
			empty, err := leaf.Annotation.IsEmpty()
			must(err)
			edgeEmpty, err := leaf.Outgoing.F.IsEmpty()
			must(err)
			if !empty || !edgeEmpty || leaf.Outgoing.Map != nil || leaf.Map != relation {
				t.Fatal("false leaf source state")
			}
			if u.updatedNodes[leaf] == stratified {
				t.Fatal("leaf update tracking switch")
			}
			if len(u.PendingNodes()) != 1 || len(covering.instances[relation]) != 0 {
				t.Fatal("leaf prematurely activated")
			}
			must(u.MarkExpanded(leaf))
			must(u.MarkExpanded(leaf))
			if len(u.PendingNodes()) != 0 || len(covering.instances[relation]) != 1 {
				t.Fatal("mark expanded not idempotent")
			}
			over, err := u.CreateNodeInstance(relation, 0)
			must(err)
			must(over.Annotation.SetEmpty())
			must(u.MakeOverapprox(over))
			full, err := over.Annotation.IsFull()
			must(err)
			edgeFull, err := over.Outgoing.F.IsFull()
			must(err)
			if !full || !edgeFull || over.Outgoing.Map != nil || !u.overapproximations[over] {
				t.Fatal("overapproximation source state")
			}
			if len(u.PendingNodes()) != 1 || len(covering.instances[relation]) != 1 {
				t.Fatal("overapproximation activation changed")
			}
			// Edge transformers own their formula independently of subsequent facts.
			must(over.Annotation.SetEmpty())
			edgeFull, err = over.Outgoing.F.IsFull()
			must(err)
			if !edgeFull {
				t.Fatal("annotation mutation changed approximation edge")
			}
		})
	}
}
