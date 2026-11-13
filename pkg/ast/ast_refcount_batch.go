package ast

// Batched reference releases. Each is the corresponding sequence of DecRef
// calls, in the same order, under one manager lock. Between elements it
// resumes suspended deletions exactly as lock() does before every DecRef,
// so node deletion, id recycling and trace output happen at the same points.

// decRefLocked is DecRefConsumed's body with the manager already locked.
func (m *ASTManager) decRefLocked(h ASTHandle) error {
	if h.IsNull() {
		return nil
	}
	r, e := m.internal(h)
	if e != nil {
		return e
	}
	_, e = m.decNode(r)
	return e
}

// relockBetween reproduces the per-call lock() checks that may change while
// the lock is held: pending deletions are resumed before the next release.
func (m *ASTManager) relockBetween() error {
	if len(m.pendingDeletions) != 0 {
		return m.cleanupNodeDeletions()
	}
	return nil
}

// DecRefEach is DecRef on hs[0], hs[1], ... under one manager lock. It stops
// at the first failure and reports how many handles it processed before it
// (the failing element is not counted).
func (m *ASTManager) DecRefEach(hs []ASTHandle) (int, error) {
	if len(hs) == 0 {
		return 0, nil
	}
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	for i, h := range hs {
		if i != 0 {
			if e := m.relockBetween(); e != nil {
				return i, e
			}
		}
		if e := m.decRefLocked(h); e != nil {
			return i, e
		}
	}
	return len(hs), nil
}

// DecRefEachReverse is DecRef on hs[len-1], ..., hs[0] under one manager
// lock. Like a loop of DecRef calls that joins errors, it reports the first
// failure (counted from the back) and how many elements it processed before
// it; the caller continues with hs[:len(hs)-n-1].
func (m *ASTManager) DecRefEachReverse(hs []ASTHandle) (int, error) {
	if len(hs) == 0 {
		return 0, nil
	}
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	for i := len(hs) - 1; i >= 0; i-- {
		done := len(hs) - 1 - i
		if done != 0 {
			if e := m.relockBetween(); e != nil {
				return done, e
			}
		}
		if e := m.decRefLocked(hs[i]); e != nil {
			return done, e
		}
	}
	return len(hs), nil
}

// IncRefThenDecRef is IncRef on inc[0], inc[1], ... followed by DecRef on
// dec[0], dec[1], ..., under one manager lock. It stops at the first
// failure and reports how many increments and releases completed before it.
func (m *ASTManager) IncRefThenDecRef(inc, dec []ASTHandle) (incDone, decDone int, err error) {
	if len(inc) == 0 && len(dec) == 0 {
		return 0, 0, nil
	}
	if err = m.lock(); err != nil {
		return 0, 0, err
	}
	defer m.mu.Unlock()
	for i, h := range inc {
		if h.IsNull() {
			continue
		}
		r, e := m.internal(h)
		if e == nil {
			e = m.incNode(r)
		}
		if e != nil {
			return i, 0, e
		}
	}
	for i, h := range dec {
		if e := m.relockBetween(); e != nil {
			return len(inc), i, e
		}
		if e := m.decRefLocked(h); e != nil {
			return len(inc), i, e
		}
	}
	return len(inc), len(dec), nil
}
