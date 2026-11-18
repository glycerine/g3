package arith

// Source: src/tactic/arith/eq2bv_tactic.cpp, collect_fd/is_fd/is_bound.

import (
	"github.com/glycerine/g3/pkg/ast"
)

// eq2bvCandidates returns observed equality maxima, before cleanup_fd adjusts
// domain sizes from bounds and creates bit-vector constants. Handles are borrowed.
func eq2bvCandidates(m *ast.ASTManager, roots []ast.ASTHandle) (map[ast.ASTHandle]uint32, error) {
	return eq2bvCollect(m, roots, make(map[ast.ASTHandle]bool))
}

func eq2bvCollect(m *ast.ASTManager, roots []ast.ASTHandle, nonfd map[ast.ASTHandle]bool) (map[ast.ASTHandle]uint32, error) {
	maxima, _, err := eq2bvCollectOrdered(m, roots, nonfd)
	return maxima, err
}

// eq2bvCollectOrdered also returns the keys in m_max (obj_map<expr,
// unsigned>) table order: add_fd inserts new keys, cleanup_fd erases the
// non-fd ones (collected in table order).
func eq2bvCollectOrdered(m *ast.ASTManager, roots []ast.ASTHandle, nonfd map[ast.ASTHandle]bool) (_ map[ast.ASTHandle]uint32, _ []ast.ASTHandle, err error) {
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, nil, err
	}
	maxima := make(map[ast.ASTHandle]uint32)
	table := ast.NewObjMap(m)
	pair := func(v, c ast.ASTHandle, requireInt bool) (uint32, bool, error) {
		constant, e := arithmeticUninterpretedConstant(m, v)
		if e != nil || !constant || nonfd[v] {
			return 0, false, e
		}
		if requireInt {
			integer, found, e := arithmeticConstant(m, v, family)
			if e != nil || !found || !integer {
				return 0, false, e
			}
		}
		value, found, unsigned, e := numeralUnsigned(m, c)
		if e != nil || !found || !unsigned {
			return 0, false, e
		}
		return value, true, nil
	}
	for _, root := range roots {
		kind, e := m.GetKind(root)
		if e != nil {
			return nil, nil, e
		}
		if kind == ast.ASTApp {
			fid, e := m.GetFamilyID(root)
			if e != nil {
				return nil, nil, e
			}
			op, e := m.GetDeclKind(root)
			if e != nil {
				return nil, nil, e
			}
			if fid == family && (op == 2 || op == 3) {
				left, e := m.GetArg(root, 0)
				if e != nil {
					return nil, nil, e
				}
				right, e := m.GetArg(root, 1)
				if e != nil {
					return nil, nil, e
				}
				_, lower, e := pair(left, right, false)
				if e != nil {
					return nil, nil, e
				}
				_, upper, e := pair(right, left, false)
				if e != nil {
					return nil, nil, e
				}
				if lower || upper {
					continue
				}
			}
		}
		todo := []ast.ASTHandle{root}
		for len(todo) > 0 {
			h := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if nonfd[h] {
				continue
			}
			nonfd[h] = true
			kind, e := m.GetKind(h)
			if e != nil {
				return nil, nil, e
			}
			if kind == ast.ASTQuantifier {
				body, e := m.GetQuantifierBody(h)
				if e != nil {
					return nil, nil, e
				}
				todo = append(todo, body)
			} else if kind == ast.ASTApp {
				eq, e := arithmeticBasicApp(m, h, ast.ASTOpEq)
				if e != nil {
					return nil, nil, e
				}
				if eq {
					left, e := m.GetArg(h, 0)
					if e != nil {
						return nil, nil, e
					}
					right, e := m.GetArg(h, 1)
					if e != nil {
						return nil, nil, e
					}
					value, found, e := pair(left, right, true)
					if e != nil {
						return nil, nil, e
					}
					if !found {
						left, right = right, left
						value, found, e = pair(left, right, true)
						if e != nil {
							return nil, nil, e
						}
					}
					if found {
						// add_fd: m_max.insert(c, val) also re-inserts a
						// known key with a larger value, and
						// core_hashtable::insert may expand the table first.
						old, exists := maxima[left]
						if !exists || old < value {
							if e = table.Insert(left, [2]ast.ASTHandle{}); e != nil {
								return nil, nil, e
							}
							maxima[left] = value
						}
						continue
					}
				}
				n, e := m.GetNumArgs(h)
				if e != nil {
					return nil, nil, e
				}
				for i := uint32(0); i < n; i++ {
					child, e := m.GetArg(h, i)
					if e != nil {
						return nil, nil, e
					}
					todo = append(todo, child)
				}
			} else if kind != ast.ASTVar {
				return nil, nil, ast.ErrASTDomain
			}
		}
	}
	var rm []ast.ASTHandle
	for _, slot := range table.Entries() {
		if nonfd[slot.Key] {
			rm = append(rm, slot.Key)
		}
	}
	for _, h := range rm {
		if err = table.Erase(h); err != nil {
			return nil, nil, err
		}
		delete(maxima, h)
	}
	entries := table.Entries()
	order := make([]ast.ASTHandle, len(entries))
	for i := range entries {
		order[i] = entries[i].Key
	}
	return maxima, order, nil
}

// These concrete expression predicates are shared by the arithmetic collectors.
func arithmeticUninterpretedConstant(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	kind, e := m.GetKind(h)
	if e != nil || kind != ast.ASTApp {
		return false, e
	}
	fid, e := m.GetFamilyID(h)
	if e != nil || fid != ast.NullFamilyID {
		return false, e
	}
	n, e := m.GetNumArgs(h)
	return n == 0, e
}

func arithmeticBasicApp(m *ast.ASTManager, h ast.ASTHandle, op int32) (bool, error) {
	kind, e := m.GetKind(h)
	if e != nil || kind != ast.ASTApp {
		return false, e
	}
	fid, e := m.GetFamilyID(h)
	if e != nil || fid != m.BasicFamilyID() {
		return false, e
	}
	k, e := m.GetDeclKind(h)
	return k == op, e
}
