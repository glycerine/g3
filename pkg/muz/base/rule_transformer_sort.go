package base

// libstdc++ std::sort (introsort, final insertion sort with threshold 16)
// over the plugin vector with rule_transformer::plugin_comparator. The source
// relies on this unstable order for plugins of equal priority.

const stdSortThreshold = 16

func pluginBefore(a, b RuleTransformPlugin) bool { return a.Priority() > b.Priority() }

func stdSortPlugins(v []RuleTransformPlugin) {
	n := len(v)
	if n < 2 {
		return
	}
	depth := 0
	for k := n; k > 1; k >>= 1 {
		depth++
	}
	pluginIntrosortLoop(v, 0, n, 2*depth)
	pluginFinalInsertionSort(v, 0, n)
}

func pluginIntrosortLoop(v []RuleTransformPlugin, first, last, depth int) {
	for last-first > stdSortThreshold {
		if depth == 0 {
			pluginHeapSort(v, first, last)
			return
		}
		depth--
		cut := pluginPartitionPivot(v, first, last)
		pluginIntrosortLoop(v, cut, last, depth)
		last = cut
	}
}

func pluginMoveMedianToFirst(v []RuleTransformPlugin, result, a, b, c int) {
	if pluginBefore(v[a], v[b]) {
		if pluginBefore(v[b], v[c]) {
			v[result], v[b] = v[b], v[result]
		} else if pluginBefore(v[a], v[c]) {
			v[result], v[c] = v[c], v[result]
		} else {
			v[result], v[a] = v[a], v[result]
		}
	} else if pluginBefore(v[a], v[c]) {
		v[result], v[a] = v[a], v[result]
	} else if pluginBefore(v[b], v[c]) {
		v[result], v[c] = v[c], v[result]
	} else {
		v[result], v[b] = v[b], v[result]
	}
}

func pluginPartitionPivot(v []RuleTransformPlugin, first, last int) int {
	mid := first + (last-first)/2
	pluginMoveMedianToFirst(v, first, first+1, mid, last-1)
	// __unguarded_partition(first + 1, last, first)
	lo, hi, pivot := first+1, last, first
	for {
		for pluginBefore(v[lo], v[pivot]) {
			lo++
		}
		hi--
		for pluginBefore(v[pivot], v[hi]) {
			hi--
		}
		if !(lo < hi) {
			return lo
		}
		v[lo], v[hi] = v[hi], v[lo]
		lo++
	}
}

func pluginUnguardedLinearInsert(v []RuleTransformPlugin, last int) {
	value := v[last]
	next := last - 1
	for pluginBefore(value, v[next]) {
		v[last] = v[next]
		last = next
		next--
	}
	v[last] = value
}

func pluginInsertionSort(v []RuleTransformPlugin, first, last int) {
	if first == last {
		return
	}
	for i := first + 1; i < last; i++ {
		if pluginBefore(v[i], v[first]) {
			value := v[i]
			copy(v[first+1:i+1], v[first:i])
			v[first] = value
		} else {
			pluginUnguardedLinearInsert(v, i)
		}
	}
}

func pluginFinalInsertionSort(v []RuleTransformPlugin, first, last int) {
	if last-first > stdSortThreshold {
		pluginInsertionSort(v, first, first+stdSortThreshold)
		for i := first + stdSortThreshold; i < last; i++ {
			pluginUnguardedLinearInsert(v, i)
		}
		return
	}
	pluginInsertionSort(v, first, last)
}

// pluginHeapSort is std::__partial_sort(first, last, last): make_heap then
// sort_heap with the same comparator.
func pluginHeapSort(v []RuleTransformPlugin, first, last int) {
	n := last - first
	for parent := (n - 2) / 2; parent >= 0; parent-- {
		pluginAdjustHeap(v, first, parent, n, v[first+parent])
	}
	for n > 1 {
		n--
		value := v[first+n]
		v[first+n] = v[first]
		pluginAdjustHeap(v, first, 0, n, value)
	}
}

func pluginAdjustHeap(v []RuleTransformPlugin, first, hole, length int, value RuleTransformPlugin) {
	top := hole
	child := hole
	for child < (length-1)/2 {
		child = 2 * (child + 1)
		if pluginBefore(v[first+child], v[first+child-1]) {
			child--
		}
		v[first+hole] = v[first+child]
		hole = child
	}
	if length&1 == 0 && child == (length-2)/2 {
		child = 2*(child+1) - 1
		v[first+hole] = v[first+child]
		hole = child
	}
	parent := (hole - 1) / 2
	for hole > top && pluginBefore(v[first+parent], value) {
		v[first+hole] = v[first+parent]
		hole = parent
		parent = (hole - 1) / 2
	}
	v[first+hole] = value
}
