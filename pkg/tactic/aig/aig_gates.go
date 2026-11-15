package aig

// Source: src/tactic/aig/aig.cpp:1341-1472,1699-1722.

import (
	"errors"
	"sort"
)

type gateTemps struct{ refs []*Ref }

func (t *gateTemps) keep(r *Ref, err error) (*Ref, error) {
	if err == nil {
		t.refs = append(t.refs, r)
	}
	return r, err
}

func (m *Manager) composeGate(build func(*gateTemps) (*Ref, error)) (*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	t := &gateTemps{}
	result, err := build(t)
	// mk_ite/mk_iff dec_ref their temporaries in creation order (n1, then
	// n2); id_gen recycles freed ids LIFO, so the order decides later ids.
	for i := 0; i < len(t.refs); i++ {
		err = errors.Join(err, t.refs[i].Close())
	}
	if err != nil && result != nil {
		err = errors.Join(err, result.Close())
		result = nil
	}
	return result, err
}

// Or follows the source's De Morgan gate construction.
func (m *Manager) Or(left, right *Ref) (*Ref, error) {
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nl, err := t.keep(left.Not())
		if err != nil {
			return nil, err
		}
		nr, err := t.keep(right.Not())
		if err != nil {
			return nil, err
		}
		conj, err := t.keep(m.And(nl, nr))
		if err != nil {
			return nil, err
		}
		return conj.Not()
	})
}

// Ite selects the source's default or alternate gate encoding.
func (m *Manager) Ite(condition, then, otherwise *Ref) (*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if !m.defaultGateEncoding {
		return m.composeGate(func(t *gateTemps) (*Ref, error) {
			first, err := t.keep(m.And(condition, then))
			if err != nil {
				return nil, err
			}
			nc, err := t.keep(condition.Not())
			if err != nil {
				return nil, err
			}
			second, err := t.keep(m.And(nc, otherwise))
			if err != nil {
				return nil, err
			}
			return m.Or(first, second)
		})
	}
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nc, err := t.keep(condition.Not())
		if err != nil {
			return nil, err
		}
		first, err := t.keep(m.Or(nc, then))
		if err != nil {
			return nil, err
		}
		second, err := t.keep(m.Or(condition, otherwise))
		if err != nil {
			return nil, err
		}
		return m.And(first, second)
	})
}

// Iff selects the source's default or alternate gate encoding.
func (m *Manager) Iff(left, right *Ref) (*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if !m.defaultGateEncoding {
		return m.composeGate(func(t *gateTemps) (*Ref, error) {
			first, err := t.keep(m.And(left, right))
			if err != nil {
				return nil, err
			}
			nl, err := t.keep(left.Not())
			if err != nil {
				return nil, err
			}
			nr, err := t.keep(right.Not())
			if err != nil {
				return nil, err
			}
			second, err := t.keep(m.And(nl, nr))
			if err != nil {
				return nil, err
			}
			return m.Or(first, second)
		})
	}
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nl, err := t.keep(left.Not())
		if err != nil {
			return nil, err
		}
		nr, err := t.keep(right.Not())
		if err != nil {
			return nil, err
		}
		first, err := t.keep(m.Or(nl, right))
		if err != nil {
			return nil, err
		}
		second, err := t.keep(m.Or(left, nr))
		if err != nil {
			return nil, err
		}
		return m.And(first, second)
	})
}

func (m *Manager) Xor(left, right *Ref) (*Ref, error) {
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nl, err := t.keep(left.Not())
		if err != nil {
			return nil, err
		}
		return m.Iff(nl, right)
	})
}

func (m *Manager) Implies(left, right *Ref) (*Ref, error) {
	return m.composeGate(func(t *gateTemps) (*Ref, error) {
		nl, err := t.keep(left.Not())
		if err != nil {
			return nil, err
		}
		return m.Or(nl, right)
	})
}

// sortArgs is std::sort(args, args+num, aig_lit_lt()) on the caller's
// slice: aig_lit_lt is a total order, so equal elements are identical
// literals and the unstable sort is indistinguishable from std::sort.
func (m *Manager) sortArgs(args []*Ref) error {
	if err := m.open(); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := arg.literal(m); err != nil {
			return err
		}
	}
	sort.Slice(args, func(i, j int) bool {
		a, b := args[i].lit, args[j].lit
		if a.node.id != b.node.id {
			return a.node.id < b.node.id
		}
		return a.inverted && !b.inverted
	})
	return nil
}

// AndArgs is mk_and(unsigned, aig_lit*) on a copy of args.
func (m *Manager) AndArgs(args []*Ref) (*Ref, error) {
	return m.andArgsInPlace(append([]*Ref(nil), args...))
}

// andArgsInPlace is mk_and(unsigned, aig_lit*): two operands are not sorted;
// three or more are sorted in place, as the source sorts m_result_stack, so
// restore_result_stack later releases them in sorted order.
func (m *Manager) andArgsInPlace(args []*Ref) (*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	switch len(args) {
	case 0:
		return m.True()
	case 1:
		return args[0].Clone()
	case 2:
		return m.And(args[0], args[1])
	}
	if err := m.sortArgs(args); err != nil {
		return nil, err
	}
	acc, err := m.And(args[0], args[1])
	if err != nil {
		return nil, err
	}
	for _, next := range args[2:] {
		replacement, e := m.And(acc, next)
		closeErr := acc.Close()
		if e != nil || closeErr != nil {
			if replacement != nil {
				closeErr = errors.Join(closeErr, replacement.Close())
			}
			return nil, errors.Join(e, closeErr)
		}
		acc = replacement
	}
	return acc, nil
}

// OrArgs is mk_or(unsigned, aig_lit*) on a copy of args.
func (m *Manager) OrArgs(args []*Ref) (*Ref, error) {
	return m.orArgsInPlace(append([]*Ref(nil), args...))
}

// orArgsInPlace is mk_or(unsigned, aig_lit*), sorting like andArgsInPlace.
func (m *Manager) orArgsInPlace(args []*Ref) (*Ref, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	switch len(args) {
	case 0:
		return m.False()
	case 1:
		return args[0].Clone()
	case 2:
		return m.Or(args[0], args[1])
	}
	if err := m.sortArgs(args); err != nil {
		return nil, err
	}
	acc, err := m.Or(args[0], args[1])
	if err != nil {
		return nil, err
	}
	for _, next := range args[2:] {
		replacement, e := m.Or(acc, next)
		closeErr := acc.Close()
		if e != nil || closeErr != nil {
			if replacement != nil {
				closeErr = errors.Join(closeErr, replacement.Close())
			}
			return nil, errors.Join(e, closeErr)
		}
		acc = replacement
	}
	return acc, nil
}
