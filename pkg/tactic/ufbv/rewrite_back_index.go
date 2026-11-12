package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:468-537.

import "github.com/glycerine/g3/pkg/ast"

// rewriteBackIndex is the concrete UFBV m_back_idx/m_processed/m_todo state.
// m_processed and each expr_set are obj_hashtable<expr>; their table order
// (AST hash, insertion and removal history) fixes the rescheduling order and
// the order of the rewriter's output formulas, so they use ast.ObjMap.
// byDecl is the lookup side of m_back_idx; declOrder keeps its table order.
type rewriteBackSet struct {
	values *ast.ObjMap
	header uint64
}

type rewriteBackIndex struct {
	manager   *ast.ASTManager
	byDecl    map[ast.ASTHandle]*rewriteBackSet
	declOrder *ast.ObjMap
	processed *ast.ObjMap
	todo      []ast.ASTHandle
	owned     []ast.ASTHandle
	closed    bool
}

func newRewriteBackIndex(m *ast.ASTManager) (*rewriteBackIndex, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &rewriteBackIndex{
		manager:   m,
		byDecl:    make(map[ast.ASTHandle]*rewriteBackSet),
		declOrder: ast.NewObjMap(m),
		processed: ast.NewObjMap(m),
	}, nil
}

// objSetKeys lists an obj_hashtable's keys in iteration (table) order.
func objSetKeys(set *ast.ObjMap) []ast.ASTHandle {
	if set == nil {
		return nil
	}
	entries := set.Entries()
	keys := make([]ast.ASTHandle, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}
	return keys
}

func (r *rewriteBackIndex) check() error {
	if r == nil || r.manager == nil || r.closed {
		return ast.ErrASTHandle
	}
	return nil
}

// declarations follows for_each_expr's postorder: application arguments from
// left to right, and quantifier body, patterns, then no-patterns. The first
// occurrence of each declaration fixes the back-set allocation order.
func (r *rewriteBackIndex) declarations(root ast.ASTHandle) ([]ast.ASTHandle, error) {
	type frame struct {
		h    ast.ASTHandle
		exit bool
	}
	stack := []frame{{h: root}}
	seen := make(map[ast.ASTHandle]struct{})
	seenDecls := make(map[ast.ASTHandle]struct{})
	var decls []ast.ASTHandle
	for len(stack) != 0 {
		i := len(stack) - 1
		fr := stack[i]
		stack = stack[:i]
		kind, e := r.manager.GetKind(fr.h)
		if e != nil {
			return nil, e
		}
		if fr.exit {
			if kind == ast.ASTApp {
				n, e := r.manager.GetNumArgs(fr.h)
				if e != nil {
					return nil, e
				}
				if n > 0 {
					family, e := r.manager.GetFamilyID(fr.h)
					if e != nil {
						return nil, e
					}
					if family == ast.NullFamilyID {
						f, e := r.manager.GetDecl(fr.h)
						if e != nil {
							return nil, e
						}
						if _, present := seenDecls[f]; !present {
							seenDecls[f] = struct{}{}
							decls = append(decls, f)
						}
					}
				}
			}
			continue
		}
		if _, present := seen[fr.h]; present {
			continue
		}
		seen[fr.h] = struct{}{}
		switch kind {
		case ast.ASTVar:
		case ast.ASTApp:
			n, e := r.manager.GetNumArgs(fr.h)
			if e != nil {
				return nil, e
			}
			stack = append(stack, frame{h: fr.h, exit: true})
			for j := n; j > 0; j-- {
				child, e := r.manager.GetArg(fr.h, j-1)
				if e != nil {
					return nil, e
				}
				stack = append(stack, frame{h: child})
			}
		case ast.ASTQuantifier:
			body, e := r.manager.GetQuantifierBody(fr.h)
			if e != nil {
				return nil, e
			}
			stack = append(stack, frame{h: fr.h, exit: true})
			n, e := r.manager.GetNumNoPatterns(fr.h)
			if e != nil {
				return nil, e
			}
			for j := n; j > 0; j-- {
				child, e := r.manager.GetNoPattern(fr.h, j-1)
				if e != nil {
					return nil, e
				}
				stack = append(stack, frame{h: child})
			}
			n, e = r.manager.GetNumPatterns(fr.h)
			if e != nil {
				return nil, e
			}
			for j := n; j > 0; j-- {
				child, e := r.manager.GetPattern(fr.h, j-1)
				if e != nil {
					return nil, e
				}
				stack = append(stack, frame{h: child})
			}
			stack = append(stack, frame{h: body})
		default:
			return nil, ast.ErrASTDomain
		}
	}
	return decls, nil
}

func (r *rewriteBackIndex) addProcessed(expr ast.ASTHandle) error {
	if e := r.check(); e != nil {
		return e
	}
	decls, e := r.declarations(expr)
	if e != nil {
		return e
	}
	if present, e := r.processed.Contains(expr); e != nil || present {
		return e
	}
	for _, f := range decls {
		if _, e := r.ensureBackSet(f); e != nil {
			return e
		}
	}
	// The source borrows m_processed roots from the enclosing rewriter's
	// argument/result owners. This concrete owner retains them until queued.
	if e := r.manager.IncRef(expr); e != nil {
		return e
	}
	r.owned = append(r.owned, expr)
	if e := r.processed.Insert(expr, [2]ast.ASTHandle{}); e != nil {
		return e
	}
	for _, f := range decls {
		if e := r.byDecl[f].values.Insert(expr, [2]ast.ASTHandle{}); e != nil {
			return e
		}
	}
	return nil
}

// rescheduleProcessed copies the selected f-occurrence set before removing
// all back-index memberships, since removal may mutate that same set.
func (r *rewriteBackIndex) rescheduleProcessed(f ast.ASTHandle) error {
	if e := r.check(); e != nil {
		return e
	}
	set, found := r.byDecl[f]
	if !found {
		return nil
	}
	if set == nil {
		if e := assertBackIndexSetPresent(518); e != nil {
			return e
		}
		return ast.ErrASTDomain
	}
	// expr_set temp: the processed members of f's set, in that set's order,
	// then visited in temp's own table order.
	temp := ast.NewObjMap(r.manager)
	for _, expr := range objSetKeys(set.values) {
		active, e := r.processed.Contains(expr)
		if e != nil {
			return e
		}
		if active {
			if e := temp.Insert(expr, [2]ast.ASTHandle{}); e != nil {
				return e
			}
		}
	}
	for _, expr := range objSetKeys(temp) {
		decls, e := r.declarations(expr)
		if e != nil {
			return e
		}
		// m_todo takes a reference before m_processed's concrete owner is
		// retired, so the queued expression remains live across transfer.
		if e := r.manager.IncRef(expr); e != nil {
			return e
		}
		r.todo = append(r.todo, expr)
		if e := r.processed.Erase(expr); e != nil {
			return e
		}
		for _, d := range decls {
			if e := r.removeBackOccurrence(d, expr); e != nil {
				return e
			}
		}
		for i, h := range r.owned {
			if h == expr {
				r.owned[i] = ast.ASTHandle{}
				consumed, e := r.manager.DecRefConsumed(h)
				if !consumed {
					r.owned[i] = h
				}
				if e != nil {
					return e
				}
				if !consumed {
					return ast.ErrASTPending
				}
				break
			}
		}
	}
	return nil
}

func (r *rewriteBackIndex) close() error {
	if r == nil || r.closed {
		return nil
	}
	for len(r.todo) != 0 {
		h := r.todo[0]
		consumed, e := r.manager.DecRefConsumed(h)
		if consumed {
			r.todo[0] = ast.ASTHandle{}
			r.todo = r.todo[1:]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	for len(r.owned) != 0 {
		h := r.owned[0]
		if h.IsNull() {
			r.owned = r.owned[1:]
			continue
		}
		consumed, e := r.manager.DecRefConsumed(h)
		if consumed {
			r.owned[0] = ast.ASTHandle{}
			r.owned = r.owned[1:]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	// ~ufbv_rewriter: reset_dealloc_values(m_back_idx) in table order.
	for _, f := range objSetKeys(r.declOrder) {
		r.releaseBackSet(r.byDecl[f])
	}
	r.byDecl = nil
	r.declOrder = nil
	r.processed = nil
	r.closed = true
	return nil
}
