package core

// Source: src/tactic/core/dom_simplify_tactic.cpp:25-161 and
// dom_simplify_tactic.h:29-61.

import "github.com/glycerine/g3/pkg/ast"

type expressionDominators struct {
	manager   *ast.ASTManager
	root      ast.ASTHandle
	rootRef   ast.ASTRef // m_root (expr_ref)
	post      []ast.ASTHandle
	postIndex map[ast.ASTHandle]uint32
	parents   map[ast.ASTHandle][]ast.ASTHandle
	// doms is m_doms. extract_tree iterates it, so it keeps the source
	// obj_map layout (and its capacity across reset()).
	doms *ast.ObjMap
	tree map[ast.ASTHandle][]ast.ASTHandle
}

func newExpressionDominators(m *ast.ASTManager) *expressionDominators {
	return &expressionDominators{manager: m, doms: ast.NewObjMap(m)}
}
func (d *expressionDominators) reset() error {
	d.post = nil
	d.postIndex = make(map[ast.ASTHandle]uint32)
	d.parents = make(map[ast.ASTHandle][]ast.ASTHandle)
	d.doms.Reset()
	d.tree = make(map[ast.ASTHandle][]ast.ASTHandle)
	d.root = ast.ASTHandle{}
	if d.rootRef == (ast.ASTRef{}) {
		return nil
	}
	return d.manager.DelRef(&d.rootRef) // m_root.reset()
}

// idom is expr_dominators::idom (m_doms[e]).
func (d *expressionDominators) idom(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	v, ok, err := d.doms.Find(h)
	return v[0], ok, err
}

func (d *expressionDominators) compile(root ast.ASTHandle) (bool, error) {
	if err := d.reset(); err != nil {
		return false, err
	}
	ref, err := d.manager.NewRef(root) // m_root = e
	if err != nil {
		return false, err
	}
	d.root, d.rootRef = root, ref
	if err := d.computePostOrder(); err != nil {
		return false, err
	}
	ok, err := d.computeDominators()
	if err != nil || !ok {
		return ok, err
	}
	// extract_tree: for (auto const& kv : m_doms) add_edge(m_tree, value, key).
	for _, entry := range d.doms.Entries() {
		parent := entry.Value[0]
		d.tree[parent] = append(d.tree[parent], entry.Key)
	}
	return true, nil
}

// computePostOrder is expr_dominators::compute_post_order: every unmarked
// argument is pushed, so the last argument is numbered first.
func (d *expressionDominators) computePostOrder() error {
	marked := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{d.root}
	for len(todo) > 0 {
		e := todo[len(todo)-1]
		if marked[e] {
			todo = todo[:len(todo)-1]
			continue
		}
		kind, err := d.manager.GetKind(e)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			marked[e] = true
			todo = todo[:len(todo)-1]
			continue
		}
		n, err := d.manager.GetNumArgs(e)
		if err != nil {
			return err
		}
		args := make([]ast.ASTHandle, n)
		done := true
		for i := uint32(0); i < n; i++ {
			args[i], err = d.manager.GetArg(e, i)
			if err != nil {
				return err
			}
			if !marked[args[i]] {
				todo = append(todo, args[i])
				done = false
			}
		}
		if done {
			marked[e] = true
			d.postIndex[e] = uint32(len(d.post))
			d.post = append(d.post, e)
			todo = todo[:len(todo)-1]
			for _, arg := range args {
				d.parents[arg] = append(d.parents[arg], e)
			}
		}
	}
	return nil
}

func (d *expressionDominators) intersect(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	n1, xok := d.postIndex[x]
	n2, yok := d.postIndex[y]
	if !xok || !yok {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	for n1 != n2 {
		var ok bool
		var err error
		if n1 < n2 {
			if x, ok, err = d.idom(x); err != nil || !ok {
				return ast.ASTHandle{}, firstDomError(err)
			}
			if n1, ok = d.postIndex[x]; !ok {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
		} else {
			if y, ok, err = d.idom(y); err != nil || !ok {
				return ast.ASTHandle{}, firstDomError(err)
			}
			if n2, ok = d.postIndex[y]; !ok {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
		}
	}
	return x, nil
}

func firstDomError(err error) error {
	if err != nil {
		return err
	}
	return ast.ErrASTDomain
}

func (d *expressionDominators) computeDominators() (bool, error) {
	if d.root.IsNull() {
		return false, ast.ErrASTHandle
	}
	if err := d.doms.Insert(d.root, [2]ast.ASTHandle{d.root}); err != nil {
		return false, err
	}
	change := true
	iterations := uint32(1)
	for change {
		change = false
		for i := 0; i+1 < len(d.post); i++ {
			child := d.post[i]
			p := d.parents[child]
			var newIdom ast.ASTHandle
			for _, pred := range p {
				known, err := d.doms.Contains(pred)
				if err != nil {
					return false, err
				}
				if !known {
					continue
				}
				if newIdom.IsNull() {
					newIdom = pred
				} else if newIdom, err = d.intersect(newIdom, pred); err != nil {
					return false, err
				}
			}
			if newIdom.IsNull() {
				if len(p) == 0 {
					return false, ast.ErrASTDomain
				}
				if err := d.doms.Insert(child, [2]ast.ASTHandle{p[0]}); err != nil {
					return false, err
				}
				change = true
				continue
			}
			old, ok, err := d.idom(child)
			if err != nil {
				return false, err
			}
			if !ok || old != newIdom {
				if err = d.doms.Insert(child, [2]ast.ASTHandle{newIdom}); err != nil {
					return false, err
				}
				change = true
			}
		}
		iterations *= 2
		if change && iterations > uint32(len(d.post)) {
			return false, nil
		}
	}
	return true, nil
}
