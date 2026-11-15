package smtcore

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func smtPBNetworkTraceRecord(function string, line int, body string) string {
	return fmt.Sprintf("-------- [pb] %s src/util/sorting_network.h:%d ---------\n%s------------------------------------------------\n", function, line, body)
}

func TestSMTPBLiteralNetworkSourceCardinalityCostOrder(t *testing.T) {
	_, _, pb, _, _ := smtPBWatchFixture(t, []int64{1, 1}, 2, false)
	network := smtPBSort{theory: pb, mode: smtPBSortGE}
	// Source vc_card_rec calls vc_smerge(k,l,n-l), whose parameters are
	// (a,b,c). The Go executable helper is ordered (c,a,b), so k=1,n=5
	// maps to (3,1,2) and costs 5 vertices + 6 clauses = 31.
	if got := network.costCardRecursive(1, 5); got != 31 {
		t.Fatal("source vc_smerge cardinality cost order", got)
	}
	if got := network.costDirectSort(1, 5); got != 21 || !network.useDirectCard(1, 5) {
		t.Fatal("source direct-cardinality selection", got)
	}
	astTestMust(t, network.err)
}

func TestSMTPBLiteralNetworkTraceActualComparatorClauseOrder(t *testing.T) {
	_, s, pb, _, inputs := smtPBWatchFixture(t, []int64{1, 1}, 2, false)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("pb")
	pb.trace = tags
	network := smtPBSort{theory: pb, mode: smtPBSortGEFull}
	out := network.merge(inputs[:1], inputs[1:])
	astTestMust(t, network.err)
	if len(out) != 2 || network.variables != 2 || network.clauses != 3 || len(s.testClauses()) != 3 {
		t.Fatal("actual comparator publication", out, network.variables, network.clauses, len(s.testClauses()))
	}
	want := smtPBNetworkTraceRecord("merge", 640, "merge a: 1 b: 1\n") +
		smtPBNetworkTraceRecord("add_clause", 561, fmt.Sprintf("(not p%d) p%d \n", out[1].Var(), inputs[0].Var())) +
		smtPBNetworkTraceRecord("add_clause", 561, fmt.Sprintf("(not p%d) p%d \n", out[1].Var(), inputs[1].Var())) +
		smtPBNetworkTraceRecord("add_clause", 561, fmt.Sprintf("(not p%d) p%d p%d \n", out[0].Var(), inputs[0].Var(), inputs[1].Var())) +
		smtPBNetworkTraceRecord("merge", 675, fmt.Sprintf("merge a: 1 b: 1\na:p%d \nb:p%d \nout:p%d p%d \n", inputs[0].Var(), inputs[1].Var(), out[0].Var(), out[1].Var()))
	if tags.ShouldTrace("pb") {
		if output.String() != want {
			t.Fatal("source comparator trace order/spacing", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native network trace was not erased")
	}
}

func TestSMTPBLiteralNetworkTraceDualCardAndSubsetFamilies(t *testing.T) {
	_, _, pb, _, xs := smtPBWatchFixture(t, []int64{1, 1, 1, 1, 1}, 3, false)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("pb")
	pb.trace = tags
	network := smtPBSort{theory: pb}
	network.ge(true, 4, xs)
	astTestMust(t, network.err)
	if tags.ShouldTrace("pb") {
		dual := smtPBNetworkTraceRecord("dualize", 516, fmt.Sprintf("5: (not p%d) (not p%d) (not p%d) (not p%d) (not p%d)  ~ 1\n", xs[0].Var(), xs[1].Var(), xs[2].Var(), xs[3].Var(), xs[4].Var()))
		if !strings.HasPrefix(output.String(), dual) || !strings.Contains(output.String(), "[pb] mk_at_most_1 src/util/sorting_network.h:340") {
			t.Fatal("source dualization/at-most-one entry", output.String())
		}
	}
	output.Reset()
	network = smtPBSort{theory: pb, mode: smtPBSortGEFull}
	network.card(1, xs[:1])
	network.directSort(2, xs[:3])
	network.smallMerge(1, xs[:1], xs[1:2])
	network.interleave(xs[:1], nil)
	network.directMerge(2, xs[:1], xs[1:2])
	astTestMust(t, network.err)
	if tags.ShouldTrace("pb") {
		last := -1
		for _, site := range []string{"card src/util/sorting_network.h:598", "sorting src/util/sorting_network.h:744", "sorting src/util/sorting_network.h:769", "card src/util/sorting_network.h:612", "dsorting src/util/sorting_network.h:982", "add_subset src/util/sorting_network.h:1017", "smerge src/util/sorting_network.h:805", "smerge src/util/sorting_network.h:879", "interleave src/util/sorting_network.h:712", "interleave src/util/sorting_network.h:732", "dsmerge src/util/sorting_network.h:923"} {
			i := strings.Index(output.String(), site)
			if i <= last {
				t.Fatal("missing or reordered family trace", site, output.String())
			}
			last = i
		}
	} else if output.Len() != 0 {
		t.Fatal("native trace operands were not erased")
	}
}

func TestSMTPBLiteralNetworkTraceBadStreamPreservesClauses(t *testing.T) {
	_, s, pb, _, xs := smtPBWatchFixture(t, []int64{1, 1}, 2, false)
	tags := diagnostic.NewTraceTagsWithWriter(smtPBDisplayFailure{})
	tags.Enable("pb")
	pb.trace = tags
	network := smtPBSort{theory: pb, mode: smtPBSortGEFull}
	out := network.sort(xs)
	astTestMust(t, network.err)
	if len(out) != 2 || len(s.testClauses()) != 3 || network.variables != 2 || network.clauses != 3 {
		t.Fatal("sticky ostream failure interrupted the actual network")
	}
}

func TestSMTPBLiteralNetworkRecursiveDiagnosticsPreserveThreshold(t *testing.T) {
	coefficients := make([]int64, 12)
	for i := range coefficients {
		coefficients[i] = 1
	}
	_, s, pb, _, xs := smtPBWatchFixture(t, coefficients, 6, false)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("pb")
	pb.trace = tags
	network := smtPBSort{theory: pb, mode: smtPBSortGEFull}
	out := network.card(5, xs)
	astTestMust(t, network.err)
	if len(out) != 5 || network.clauses == 0 {
		t.Fatal("recursive threshold network shape", len(out), network.clauses)
	}
	clauses := make([][]SMTLiteral, len(s.testClauses()))
	for i, c := range s.testClauses() {
		clauses[i] = append([]SMTLiteral{}, c.Literals...)
	}
	// Independent existential SAT checks around the threshold, with the
	// output forced true. No PB propagation or AST evaluator is consulted.
	for count := 0; count <= len(xs); count++ {
		values := make([]int8, len(s.boolExpr))
		values[0] = 1 // source Boolean variable zero is true
		for i, l := range xs {
			values[l.Var()] = -1
			if i < count {
				values[l.Var()] = 1
			}
		}
		values[out[4].Var()] = 1
		if got := smtPBNetworkSAT(clauses, values); got != (count >= 5) {
			t.Fatal("recursive network threshold", count, got)
		}
	}
	if tags.ShouldTrace("pb") {
		for _, site := range []string{"card src/util/sorting_network.h:598", "card src/util/sorting_network.h:612", "smerge src/util/sorting_network.h:805", "smerge src/util/sorting_network.h:879", "dsmerge src/util/sorting_network.h:923"} {
			if !strings.Contains(output.String(), site) {
				t.Fatal("missing recursive source packet", site)
			}
		}
	}
}

func TestSMTPBLiteralNetworkDebugGuardsPrecedeEffects(t *testing.T) {
	if !smtcoreSourceDebug {
		t.Skip("source SASSERT guards are erased in native builds")
	}
	for _, operation := range []string{"interleave", "direct-merge", "direct-sort", "subset", "cost-sort", "cost-recursion", "cost-direct"} {
		t.Run(operation, func(t *testing.T) {
			_, s, pb, _, xs := smtPBWatchFixture(t, []int64{1, 1}, 2, false)
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			tags.Enable("pb")
			pb.trace = tags
			network := smtPBSort{theory: pb, mode: smtPBSortGEFull}
			before := len(s.boolExpr)
			switch operation {
			case "interleave":
				network.interleave(nil, xs)
			case "direct-merge":
				network.directMerge(0, xs[:1], xs[1:])
			case "direct-sort":
				network.directSort(3, xs)
			case "subset":
				network.subset(false, 2, 1, nil, xs)
			case "cost-sort":
				network.useDirectSort(2)
			case "cost-recursion":
				network.costSortRecursive(2)
			case "cost-direct":
				network.costDirectSort(3, 2)
			}
			if network.err != ast.ErrASTDomain || len(s.boolExpr) != before || len(s.testClauses()) != 0 || network.variables != 0 || network.clauses != 0 {
				t.Fatal("source guard did not precede side effects", network.err, network.variables, network.clauses)
			}
			if strings.Contains(output.String(), "[pb] add_clause") {
				t.Fatal("source guard published a clause")
			}
		})
	}
}
