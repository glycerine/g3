package ast

import "errors"

// IsolatedCheck runs a debug-only check (a source SASSERT or DEBUG_CODE
// block whose condition builds and releases terms, such as qsat's
// validate_defs) so that it leaves no observable trace in the manager.
// Release builds never run such checks; debug builds must still allocate the
// same IDs, in the same order, and log the same trace.
//
// While the check runs the trace stream ([mk-app] and the SMT records) is
// muted. Afterwards, small arithmetic numerals that the check first created
// and cached (arith_decl_plugin m_small_ints/m_small_reals) are dropped from
// the cache again, and, when every expression and declaration the check
// created has been released, the expression and declaration ID generators
// (free-list order included) and the fresh-name counter are put back exactly
// as they were. restored reports whether that happened; when the check left
// a new term alive the IDs are left as they are.
func (m *ASTManager) IsolatedCheck(check func() error) (restored bool, err error) {
	if err = m.lock(); err != nil {
		return false, err
	}
	exprs, decls, fresh := m.ExprIDs.Snapshot(), m.declIDs.Snapshot(), m.freshID
	var small [2][]astNodeRef
	if p := m.arithmeticPlugin; p != nil {
		for j := range p.small {
			small[j] = append([]astNodeRef(nil), p.small[j]...)
		}
	}
	trace := m.TraceStream
	m.TraceStream = nil
	m.mu.Unlock()
	err = check()
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	m.TraceStream = trace
	if p := m.arithmeticPlugin; p != nil {
		for j := range p.small {
			v := &p.small[j]
			for i, r := range *v {
				if r.slot == 0 || (i < len(small[j]) && small[j][i] == r) {
					continue
				}
				consumed, e := m.decNode(r)
				if consumed {
					(*v)[i] = astNodeRef{}
				}
				if e != nil {
					return false, errors.Join(err, e)
				}
			}
			if len(*v) > len(small[j]) {
				*v = (*v)[:len(small[j])]
			}
		}
	}
	// Both generators must be restorable; check before changing either.
	e2, d2 := m.ExprIDs.Snapshot(), m.declIDs.Snapshot()
	if !e2.RestoreTransient(exprs) || !d2.RestoreTransient(decls) {
		return false, err
	}
	m.ExprIDs.RestoreTransient(exprs)
	m.declIDs.RestoreTransient(decls)
	// Fresh names the check made belong to declarations that are gone.
	m.freshID = fresh
	return true, err
}
