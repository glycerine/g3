package spacer

// Source: src/muz/spacer/spacer_context.cpp check_reachability restart pops
// only the obligations before the root, preserving entries queued after it.

import "testing"

func TestSpacerRestartKeepsObligationsBehindRoot(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	c, m := f.context, f.context.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	p := c.transformers[f.query]
	must(p.SetProofObligationReuse(false))
	boolean, err := m.MkBoolSort()
	must(err)
	a, err := m.MkFreshConst("restart-root-a", boolean)
	must(err)
	b, err := m.MkFreshConst("restart-root-b", boolean)
	must(err)
	rootPost, err := m.MkAnd(a, b)
	must(err)
	root, err := p.NewProofObligation(nil, 0, 0, rootPost, nil)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	beforeRoot, err := p.NewProofObligation(root, 0, 0, truth, nil)
	must(err)
	deeper, err := p.NewProofObligation(root, 0, 1, a, nil)
	must(err)
	higher, err := p.NewProofObligation(root, 1, 0, b, nil)
	must(err)
	must(c.queue.SetRoot(root))
	must(c.queue.Push(beforeRoot))
	must(c.queue.Push(deeper))
	must(c.queue.Push(higher))
	if top, err := c.queue.Top(); err != nil || top != beforeRoot {
		t.Fatal("restart setup must put the smaller cube before the root", top, err)
	}
	reads := 0
	lemmaCount := func() (uint64, error) {
		reads++
		if reads == 1 {
			return 0, nil
		}
		return 1, nil
	}
	reachable, err := c.queue.checkReachability(false, false, InfinityLevel, true, 0, lemmaCount)
	must(err)
	if reachable || c.queue.restarts != 1 {
		t.Fatal("expected one restart followed by blocking the level-zero root", reachable, c.queue.restarts)
	}
	if len(c.queue.items) != 2 || c.queue.items[0] != deeper || c.queue.items[1] != higher {
		t.Fatal("restart discarded obligations behind the root", len(c.queue.items))
	}
	if closed, err := beforeRoot.IsClosed(); err != nil || closed {
		t.Fatal("restart expanded an obligation that should only have been popped", closed, err)
	}
	if root.level != 1 {
		t.Fatal("root was not processed after restarting", root.level)
	}
	if got := spacerContextStatistic(t, c, "SPACER restarts"); got != 1 {
		t.Fatal("context restart count", got)
	}
	must(c.Reset())
	if got := spacerContextStatistic(t, c, "SPACER restarts"); got != 1 {
		t.Fatal("reset lost context restart count", got)
	}
	must(c.ResetStatistics())
	if got := spacerContextStatistic(t, c, "SPACER restarts"); got != 0 {
		t.Fatal("statistics reset left restart count", got)
	}
}
