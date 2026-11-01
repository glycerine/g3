package duality

import "testing"

func TestExpansionHeuristicUsesMappedUpdatesAndSourceCutoff(t *testing.T) {
	ctx, _, _ := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("heuristic-R")
	must(err)
	closeExpr(t, term)
	original, err := NewRPFP(ctx)
	must(err)
	defer func() { must(original.Close()) }()
	instances, err := NewRPFP(ctx)
	must(err)
	defer func() { must(instances.Close()) }()
	heuristic, err := NewExpansionHeuristic(original)
	must(err)
	var nodes, candidates []*Node
	for _, score := range []int{0, 1, 4, 8} {
		node, err := original.CreateNode(term)
		must(err)
		nodes = append(nodes, node)
		candidate, err := instances.CloneNode(node)
		must(err)
		candidates = append(candidates, candidate)
		for n := 0; n < score; n++ {
			must(heuristic.Update(node))
		}
	}
	var best []*Node
	must(heuristic.ChooseExpand(candidates, &best, false, false))
	if len(best) != 3 || best[0] != candidates[0] || best[1] != candidates[1] || best[2] != candidates[2] {
		t.Fatal("midpoint cutoff", best)
	}
	best = nil
	must(heuristic.ChooseExpand(candidates, &best, false, true))
	if len(best) != 1 || best[0] != candidates[0] {
		t.Fatal("minimum-only selection")
	}
	// All mapped instances of the same original node share its score.
	duplicate, err := instances.CloneNode(nodes[0])
	must(err)
	choices := append(append([]*Node(nil), candidates...), duplicate, candidates[0])
	best = nil
	must(heuristic.ChooseExpand(choices, &best, false, true))
	if len(best) != 2 || best[0] != candidates[0] || best[1] != duplicate {
		t.Fatal("mapped instance score or set identity")
	}
	// High-priority calls preserve previously accumulated selections.
	must(heuristic.ChooseExpand(nil, &best, true, true))
	if len(best) != 2 {
		t.Fatal("high priority changed best")
	}
	best = []*Node{candidates[3]}
	must(heuristic.ChooseExpand(candidates, &best, false, true))
	if len(best) != 2 || best[0] != candidates[3] || best[1] != candidates[0] {
		t.Fatal("discarded previous best entries")
	}
	// Updating the original changes every mapped instance's priority.
	for n := 0; n < 9; n++ {
		must(heuristic.Update(nodes[0]))
	}
	best = nil
	must(heuristic.ChooseExpand(choices, &best, false, true))
	if len(best) != 1 || best[0] != candidates[1] {
		t.Fatal("score updates did not affect mapped instances")
	}
	snapshot := len(best)
	must(heuristic.ChooseExpand(nil, &best, false, false))
	if len(best) != snapshot {
		t.Fatal("empty choices changed selection")
	}
	if heuristic.UseNode(nodes[0]) != 0 {
		t.Fatal("base heuristic is not neutral")
	}
	heuristic.Done()
}
