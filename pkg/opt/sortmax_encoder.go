package opt

// Source: src/opt/sortmax.cpp sorting-network callback owner.

type sortMaxEncoder struct {
	engine *sortMaxEngine
	// costs memoizes vc_merge and vc_smerge. The C++ recomputes them at
	// every merge node, which is quadratic in the merge size; they are pure
	// functions of their sizes, so caching changes no network decision.
	costs map[sortMaxCostKey]sortMaxNetworkCost
}

func newSortMaxEncoder(engine *sortMaxEngine) *sortMaxEncoder {
	return &sortMaxEncoder{engine: engine}
}
