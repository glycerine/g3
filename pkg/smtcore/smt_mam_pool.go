package smtcore

// Source: src/util/pool.h (pool<T>), as mam.cpp's interpreter and mam_impl
// use it for enode_vector: mk() pops a recycled vector or allocates one,
// recycle() pushes it back.

// smtMAMEnodeVectorPool is pool<enode_vector>. A recycled vector is cleared
// before it is pooled, so the pool retains no enodes.
type smtMAMEnodeVectorPool struct {
	objs []*[]*SMTEnode
}

// mk is m_pool.mk() followed by r->reset() (mk_enode_vector /
// mk_tmp_vector).
func (p *smtMAMEnodeVectorPool) mk() *[]*SMTEnode {
	n := len(p.objs)
	if n == 0 {
		return new([]*SMTEnode)
	}
	r := p.objs[n-1]
	p.objs[n-1] = nil
	p.objs = p.objs[:n-1]
	return r
}

// recycle is m_pool.recycle(v).
func (p *smtMAMEnodeVectorPool) recycle(v *[]*SMTEnode) {
	if v == nil {
		return
	}
	clear(*v)
	*v = (*v)[:0]
	p.objs = append(p.objs, v)
}
