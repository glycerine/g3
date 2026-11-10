package smtcore

// Source: src/smt/mam.cpp code_tree_map, path, path_tree and mam_impl
// (lines 2833-4031).

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/approx"
)

const mamApproxSetCapacity = 64

// smtMAMPath is path: f.1:t.2 -> g.2 -> h.0:s.1.
type smtMAMPath struct {
	label        ast.ASTHandle
	argIdx       uint16
	groundArgIdx uint16
	groundArg    *SMTEnode
	patternIdx   uint32
	child        *smtMAMPath
}

// mamPathIsEqual is is_equal(path const*, path const*).
func mamPathIsEqual(p1, p2 *smtMAMPath) bool {
	for {
		if p1.label != p2.label || p1.argIdx != p2.argIdx || p1.patternIdx != p2.patternIdx || (p1.child == nil) != (p2.child == nil) {
			return false
		}
		if p1.child == nil && p2.child == nil {
			return true
		}
		p1 = p1.child
		p2 = p2.child
	}
}

// smtMAMPathTree is path_tree, a node of the inverted path index.
type smtMAMPathTree struct {
	label        ast.ASTHandle
	argIdx       uint16
	groundArgIdx uint16
	groundArg    *SMTEnode
	code         *smtMAMCodeTree
	filter       approx.UintSet
	sibling      *smtMAMPathTree
	firstChild   *smtMAMPathTree
	todo         []*SMTEnode // m_todo; hasTodo stands for a non-null vector
	hasTodo      bool
}

type smtMAMQP struct {
	qa, mp ast.ASTHandle
}

// smtMAMImpl is mam_impl.
type smtMAMImpl struct {
	state      *SMTInternalizerState
	registry   *SMTQuantifierRegistry
	m          *ast.ASTManager
	useFilters bool
	trail      smtMAMTrail
	hasher     smtMAMLabelHasher
	ct         smtMAMCodeTreeManager
	compiler   *smtMAMCompiler
	interp     *smtMAMInterpreter
	trees      []*smtMAMCodeTree // code_tree_map::m_trees, by decl id

	tmpTrees         []*smtMAMCodeTree
	tmpTreesToDelete []ast.ASTHandle
	toMatch          []*smtMAMCodeTree
	newPatterns      []smtMAMQP

	isPlbl []bool
	isClbl []bool

	pp [mamApproxSetCapacity][mamApproxSetCapacity][2]*smtMAMPathTree
	pc [mamApproxSetCapacity][mamApproxSetCapacity]*smtMAMPathTree

	varPaths [][]*smtMAMPath
	todo     []*smtMAMPathTree

	sharedEnodes map[*SMTEnode]struct{}

	r1, r2 *SMTEnode

	flatAssoc map[ast.ASTHandle]bool
	err       error
}

func newSMTMAMImpl(state *SMTInternalizerState, registry *SMTQuantifierRegistry, useFilters bool) *smtMAMImpl {
	r := &smtMAMImpl{state: state, registry: registry, m: state.manager, useFilters: useFilters,
		sharedEnodes: make(map[*SMTEnode]struct{}), flatAssoc: make(map[ast.ASTHandle]bool)}
	r.ct = smtMAMCodeTreeManager{hasher: &r.hasher, trail: &r.trail, m: state.manager}
	r.compiler = newSMTMAMCompiler(state, registry, &r.ct, &r.hasher, useFilters)
	r.interp = newSMTMAMInterpreter(state, r.onMatch, useFilters)
	return r
}

func (r *smtMAMImpl) fail(err error) {
	if err != nil && r.err == nil {
		r.err = err
	}
}

func (r *smtMAMImpl) takeErr() error {
	err := r.err
	r.err = nil
	return err
}

// ---- AST helpers (errors are recorded in r.err) ----

func (r *smtMAMImpl) declID(d ast.ASTHandle) uint32 {
	id, err := smtMAMDeclID(r.m, d)
	r.fail(err)
	return id
}

func (r *smtMAMImpl) lblHash(d ast.ASTHandle) uint32 { return uint32(r.hasher.hash(r.declID(d))) }

func (r *smtMAMImpl) numArgs(e ast.ASTHandle) uint32 {
	n, err := r.m.GetNumArgs(e)
	r.fail(err)
	return n
}

func (r *smtMAMImpl) arg(e ast.ASTHandle, i uint32) ast.ASTHandle {
	a, err := r.m.GetArg(e, i)
	r.fail(err)
	return a
}

func (r *smtMAMImpl) decl(e ast.ASTHandle) ast.ASTHandle {
	d, err := r.m.GetDecl(e)
	r.fail(err)
	return d
}

func (r *smtMAMImpl) isGround(e ast.ASTHandle) bool {
	g, err := r.m.IsGround(e)
	r.fail(err)
	return err == nil && g
}

func (r *smtMAMImpl) kind(e ast.ASTHandle) ast.ASTKind {
	k, err := r.m.GetKind(e)
	r.fail(err)
	return k
}

func (r *smtMAMImpl) isFlatAssociative(d ast.ASTHandle) bool {
	if v, ok := r.flatAssoc[d]; ok {
		return v
	}
	flags, err := r.m.GetFuncDeclFlags(d)
	r.fail(err)
	r.flatAssoc[d] = flags.FlatAssociative
	return flags.FlatAssociative
}

func (r *smtMAMImpl) mkEnode(qa, n ast.ASTHandle) *SMTEnode {
	if r.err != nil {
		return nil
	}
	return smtMAMMkEnode(r.state, r.registry, qa, n, &r.err)
}

func (r *smtMAMImpl) relevant(n *SMTEnode) bool { return r.state.isRelevant(n.owner) }

// ---- code_tree_map ----

// codeTreeFor is code_tree_map::get_code_tree_for.
func (r *smtMAMImpl) codeTreeFor(lbl ast.ASTHandle) *smtMAMCodeTree {
	id := r.declID(lbl)
	if int(id) < len(r.trees) {
		return r.trees[id]
	}
	return nil
}

// treesAddPattern is code_tree_map::add_pattern.
func (r *smtMAMImpl) treesAddPattern(qa, mp ast.ASTHandle, firstIdx uint32) {
	p := r.arg(mp, firstIdx)
	lbl := r.decl(p)
	lblID := r.declID(lbl)
	if r.err != nil {
		return
	}
	for uint32(len(r.trees)) <= lblID {
		r.trees = append(r.trees, nil)
	}
	if r.trees[lblID] == nil {
		t, err := r.compiler.mkTree(qa, mp, firstIdx, false)
		r.fail(err)
		r.trees[lblID] = t
		r.trail.push(func() {
			if int(lblID) < len(r.trees) {
				r.trees[lblID] = nil
			}
		})
	} else {
		tree := r.trees[lblID]
		// n-ary + and *: ignore a pattern without the expected arity.
		if tree.numArgs == r.numArgs(p) {
			r.fail(r.compiler.insert(tree, qa, mp, firstIdx, false))
		}
	}
}

// ---- mam_impl ----

func (r *smtMAMImpl) addCandidateTo(t *smtMAMCodeTree, app *SMTEnode) {
	if t != nil {
		if !t.hasCandidates() {
			r.toMatch = append(r.toMatch, t)
		}
		t.addCandidate(app)
	}
}

func (r *smtMAMImpl) addCandidate(app *SMTEnode) {
	r.addCandidateTo(r.codeTreeFor(app.declaration), app)
}

func (r *smtMAMImpl) isPlblDecl(lbl ast.ASTHandle) bool {
	id := r.declID(lbl)
	return int(id) < len(r.isPlbl) && r.isPlbl[id]
}

func (r *smtMAMImpl) isClblDecl(lbl ast.ASTHandle) bool {
	id := r.declID(lbl)
	return int(id) < len(r.isClbl) && r.isClbl[id]
}

func (r *smtMAMImpl) updateLbls(n *SMTEnode, elem uint32) {
	root := n.root
	if !root.labels.MayContain(elem) {
		old := root.labels
		r.trail.push(func() { root.labels = old })
		root.labels.Insert(elem)
	}
}

func mamReserveBools(v []bool, id uint32) []bool {
	for uint32(len(v)) <= id {
		v = append(v, false)
	}
	return v
}

func (r *smtMAMImpl) updateClbls(lbl ast.ASTHandle) {
	lblID := r.declID(lbl)
	if r.err != nil {
		return
	}
	r.isClbl = mamReserveBools(r.isClbl, lblID)
	if r.isClbl[lblID] {
		return
	}
	r.trail.push(func() { r.isClbl[lblID] = false })
	r.isClbl[lblID] = true
	h := r.lblHash(lbl)
	for _, app := range r.interp.enodesOf(lbl) {
		if r.relevant(app) {
			r.updateLbls(app, h)
		}
	}
}

func (r *smtMAMImpl) updateChildrenPlbls(app *SMTEnode, elem uint32) {
	for _, c := range app.arguments {
		root := c.root
		if !root.propagatedLabels.MayContain(elem) {
			old := root.propagatedLabels
			r.trail.push(func() { root.propagatedLabels = old })
			root.propagatedLabels.Insert(elem)
		}
	}
}

func (r *smtMAMImpl) updatePlbls(lbl ast.ASTHandle) {
	lblID := r.declID(lbl)
	if r.err != nil {
		return
	}
	r.isPlbl = mamReserveBools(r.isPlbl, lblID)
	if r.isPlbl[lblID] {
		return
	}
	r.trail.push(func() { r.isPlbl[lblID] = false })
	r.isPlbl[lblID] = true
	h := r.lblHash(lbl)
	for _, app := range r.interp.enodesOf(lbl) {
		if r.relevant(app) {
			r.updateChildrenPlbls(app, h)
		}
	}
}

func (r *smtMAMImpl) resetPPPC() {
	r.pp = [mamApproxSetCapacity][mamApproxSetCapacity][2]*smtMAMPathTree{}
	r.pc = [mamApproxSetCapacity][mamApproxSetCapacity]*smtMAMPathTree{}
}

func (r *smtMAMImpl) mkCode(qa, mp ast.ASTHandle, patIdx uint32) *smtMAMCodeTree {
	t, err := r.compiler.mkTree(qa, mp, patIdx, true)
	r.fail(err)
	return t
}

func (r *smtMAMImpl) insertCode(t *smtMAMPathTree, qa, mp ast.ASTHandle, patIdx uint32) {
	r.fail(r.compiler.insert(t.code, qa, mp, patIdx, false))
}

func (r *smtMAMImpl) newPathTree(p *smtMAMPath) *smtMAMPathTree {
	return &smtMAMPathTree{label: p.label, argIdx: p.argIdx, groundArgIdx: p.groundArgIdx, groundArg: p.groundArg,
		filter: approx.NewUintSet(r.lblHash(p.label))}
}

// mkPathTree is mk_path_tree.
func (r *smtMAMImpl) mkPathTree(p *smtMAMPath, qa, mp ast.ASTHandle) *smtMAMPathTree {
	patIdx := p.patternIdx
	var head, curr, prev *smtMAMPathTree
	for p != nil {
		curr = r.newPathTree(p)
		if prev != nil {
			prev.firstChild = curr
		}
		if head == nil {
			head = curr
		}
		prev = curr
		p = p.child
	}
	curr.code = r.mkCode(qa, mp, patIdx)
	return head
}

func (r *smtMAMImpl) setPathTreePtr(slot **smtMAMPathTree) {
	old := *slot
	r.trail.push(func() { *slot = old })
}

// insertPath is mam_impl::insert(path_tree*, path*, qa, mp).
func (r *smtMAMImpl) insertPath(t *smtMAMPathTree, p *smtMAMPath, qa, mp ast.ASTHandle) {
	head := t
	var prevSibling *smtMAMPathTree
	foundLabel := false
	for t != nil {
		if t.label == p.label {
			foundLabel = true
			if t.argIdx == p.argIdx && t.groundArg == p.groundArg && t.groundArgIdx == p.groundArgIdx {
				// found compatible node
				if t.firstChild == nil {
					if p.child == nil {
						r.insertCode(t, qa, mp, p.patternIdx)
					} else {
						r.setPathTreePtr(&t.firstChild)
						t.firstChild = r.mkPathTree(p.child, qa, mp)
					}
				} else {
					if p.child == nil {
						if t.code != nil {
							r.insertCode(t, qa, mp, p.patternIdx)
						} else {
							tt := t
							old := tt.code
							r.trail.push(func() { tt.code = old })
							t.code = r.mkCode(qa, mp, p.patternIdx)
						}
					} else {
						r.insertPath(t.firstChild, p.child, qa, mp)
					}
				}
				return
			}
		}
		prevSibling = t
		t = t.sibling
	}
	r.setPathTreePtr(&prevSibling.sibling)
	prevSibling.sibling = r.mkPathTree(p, qa, mp)
	if !foundLabel {
		old := head.filter
		r.trail.push(func() { head.filter = old })
		head.filter.Insert(r.lblHash(p.label))
	}
}

func (r *smtMAMImpl) updatePC(h1, h2 uint32, p *smtMAMPath, qa, mp ast.ASTHandle) {
	if r.pc[h1][h2] != nil {
		r.insertPath(r.pc[h1][h2], p, qa, mp)
	} else {
		r.setPathTreePtr(&r.pc[h1][h2])
		r.pc[h1][h2] = r.mkPathTree(p, qa, mp)
	}
}

func (r *smtMAMImpl) updatePP(h1, h2 uint32, p1, p2 *smtMAMPath, qa, mp ast.ASTHandle) {
	if h1 == h2 {
		slot := &r.pp[h1][h2]
		if slot[0] != nil {
			r.insertPath(slot[0], p1, qa, mp)
			if !mamPathIsEqual(p1, p2) {
				r.insertPath(slot[0], p2, qa, mp)
			}
		} else {
			r.setPathTreePtr(&slot[0])
			slot[0] = r.mkPathTree(p1, qa, mp)
			r.insertPath(slot[0], p2, qa, mp)
		}
		return
	}
	if h1 > h2 {
		h1, h2 = h2, h1
		p1, p2 = p2, p1
	}
	slot := &r.pp[h1][h2]
	if slot[0] != nil {
		r.insertPath(slot[0], p1, qa, mp)
		r.insertPath(slot[1], p2, qa, mp)
	} else {
		r.setPathTreePtr(&slot[0])
		r.setPathTreePtr(&slot[1])
		slot[0] = r.mkPathTree(p1, qa, mp)
		slot[1] = r.mkPathTree(p2, qa, mp)
	}
}

func (r *smtMAMImpl) updateVars(varID uint32, p *smtMAMPath, qa, mp ast.ASTHandle) {
	found := false
	for _, currPath := range r.varPaths[varID] {
		if mamPathIsEqual(p, currPath) {
			found = true
		}
		lbl1 := currPath.label
		lbl2 := p.label
		r.updatePlbls(lbl1)
		r.updatePlbls(lbl2)
		r.updatePP(r.lblHash(lbl1), r.lblHash(lbl2), currPath, p, qa, mp)
	}
	if !found {
		r.varPaths[varID] = append(r.varPaths[varID], p)
	}
}

func (r *smtMAMImpl) getGroundArg(pat, qa ast.ASTHandle) (*SMTEnode, uint16) {
	n := r.numArgs(pat)
	for i := uint32(0); i < n; i++ {
		a := r.arg(pat, i)
		if r.isGround(a) {
			return r.mkEnode(qa, a), uint16(i)
		}
	}
	return nil, 0
}

// updateFiltersPat is update_filters(pat, p, qa, mp, pat_idx).
func (r *smtMAMImpl) updateFiltersPat(pat ast.ASTHandle, p *smtMAMPath, qa, mp ast.ASTHandle, patIdx uint32) {
	numArgs := uint16(r.numArgs(pat))
	groundArg, groundArgPos := r.getGroundArg(pat, qa)
	plbl := r.decl(pat)
	for i := uint16(0); i < numArgs; i++ {
		if r.err != nil {
			return
		}
		child := r.arg(pat, uint32(i))
		newPath := &smtMAMPath{label: plbl, argIdx: i, groundArgIdx: groundArgPos, groundArg: groundArg, patternIdx: patIdx, child: p}
		k := r.kind(child)
		if k == ast.ASTVar {
			idx, err := r.m.GetVarIndex(child)
			r.fail(err)
			if err == nil {
				r.updateVars(idx, newPath, qa, mp)
			}
			continue
		}
		if r.isGround(child) {
			n := r.mkEnode(qa, child)
			if n == nil {
				return
			}
			r.updatePlbls(plbl)
			if !n.HasLabelHash() {
				r.fail(n.SetLabelHash(&r.state.enodeTrail))
			}
			r.updatePC(r.lblHash(plbl), uint32(n.labelHash), newPath, qa, mp)
			continue
		}
		clbl := r.decl(child)
		r.updatePlbls(plbl)
		r.updateClbls(clbl)
		r.updatePC(r.lblHash(plbl), r.lblHash(clbl), newPath, qa, mp)
		r.updateFiltersPat(child, newPath, qa, mp, patIdx)
	}
}

// updateFilters is update_filters(qa, mp).
func (r *smtMAMImpl) updateFilters(qa, mp ast.ASTHandle) {
	numVars, err := r.m.GetNumDeclarations(qa)
	r.fail(err)
	for uint32(len(r.varPaths)) < numVars+1 {
		r.varPaths = append(r.varPaths, nil)
	}
	for i := uint32(0); i < numVars; i++ {
		r.varPaths[i] = r.varPaths[i][:0]
	}
	numPatterns := r.numArgs(mp)
	for i := uint32(0); i < numPatterns; i++ {
		r.updateFiltersPat(r.arg(mp, i), nil, qa, mp, i)
	}
}

// isEq is equality modulo m_r1 = m_r2.
func (r *smtMAMImpl) isEq(n1, n2 *SMTEnode) bool {
	return n1.root == n2.root || (n1.root == r.r1 && n2.root == r.r2) || (n2.root == r.r1 && n1.root == r.r2)
}

// collectParents is collect_parents: new E-matching candidates from the
// inverted path index t.
func (r *smtMAMImpl) collectParents(root *SMTEnode, t *smtMAMPathTree) {
	if t == nil {
		return
	}
	r.todo = r.todo[:0]
	var toUnmark, toUnmark2 []*SMTEnode
	t.todo = append(t.todo[:0], root)
	t.hasTodo = true
	r.todo = append(r.todo, t)
	for head := 0; head < len(r.todo); head++ {
		t := r.todo[head]
		v := t.todo
		filter := t.filter
		for _, n := range v {
			// mark: processed parents; mark2: roots queued for the next level.
			currChild := n.root
			if r.useFilters && currChild.propagatedLabels.EmptyIntersection(filter) {
				continue
			}
			for _, currParent := range currChild.sourceParents {
				// Remark: equality is never in the inverted path index.
				if currParent.IsEquality() {
					continue
				}
				lbl := currParent.declaration
				isFlatAssoc := r.isFlatAssociative(lbl)
				currParentRoot := currParent.root
				currParentCg := currParent.congruence
				if filter.MayContain(r.lblHash(lbl)) && !currParent.mark &&
					(currParentCg == currParent || !r.isEq(currParentCg, currParentRoot)) &&
					r.relevant(currParent) {
					for currTree := t; currTree != nil; currTree = currTree.sibling {
						nargs := uint16(len(currParent.arguments))
						if currTree.label == lbl && (!isFlatAssoc || (currTree.argIdx < nargs && currTree.groundArgIdx < nargs)) {
							currParentChild := currParent.arguments[currTree.argIdx].root
							if currChild == currParentChild &&
								(currTree.groundArg == nil || r.isEq(currTree.groundArg, currParent.arguments[currTree.groundArgIdx])) {
								if currTree.code != nil {
									r.addCandidateTo(currTree.code, currParent)
								}
								if currTree.firstChild != nil {
									child := currTree.firstChild
									if !child.hasTodo {
										child.todo = child.todo[:0]
										child.hasTodo = true
										r.todo = append(r.todo, child)
									}
									if !currParentRoot.mark2 {
										child.todo = append(child.todo, currParentRoot)
									}
								}
							}
						}
					}
					currParent.mark = true
					toUnmark = append(toUnmark, currParent)
					if !currParentRoot.mark2 {
						currParentRoot.mark2 = true
						toUnmark2 = append(toUnmark2, currParentRoot)
					}
				}
			}
		}
		t.todo = t.todo[:0]
		t.hasTodo = false
		for _, n := range toUnmark {
			n.mark = false
		}
		for _, n := range toUnmark2 {
			n.mark2 = false
		}
		toUnmark = toUnmark[:0]
		toUnmark2 = toUnmark2[:0]
	}
}

func (r *smtMAMImpl) processPP(r1, r2 *SMTEnode) {
	plbls1 := r1.propagatedLabels
	plbls2 := r2.propagatedLabels
	if plbls1.Empty() || plbls2.Empty() {
		return
	}
	plbls1.ForEach(func(plbl1 uint32) bool {
		plbls2.ForEach(func(plbl2 uint32) bool {
			nPlbl1, nPlbl2 := plbl1, plbl2
			nR1, nR2 := r1, r2
			if nPlbl1 > nPlbl2 {
				nPlbl1, nPlbl2 = nPlbl2, nPlbl1
				nR1, nR2 = nR2, nR1
			}
			slot := &r.pp[nPlbl1][nPlbl2]
			if nPlbl1 == nPlbl2 {
				if len(nR1.sourceParents) <= len(nR2.sourceParents) {
					r.collectParents(nR1, slot[0])
				} else {
					r.collectParents(nR2, slot[0])
				}
			} else {
				if len(nR1.sourceParents) <= len(nR2.sourceParents) {
					r.collectParents(nR1, slot[0])
				} else {
					r.collectParents(nR2, slot[1])
				}
			}
			return true
		})
		return true
	})
}

func (r *smtMAMImpl) processPC(r1, r2 *SMTEnode) {
	plbls := r1.propagatedLabels
	clbls := r2.labels
	if plbls.Empty() || clbls.Empty() {
		return
	}
	plbls.ForEach(func(plbl1 uint32) bool {
		clbls.ForEach(func(lbl2 uint32) bool {
			r.collectParents(r1, r.pc[plbl1][lbl2])
			return true
		})
		return true
	})
}

func (r *smtMAMImpl) matchNewPatterns() {
	r.tmpTreesToDelete = r.tmpTreesToDelete[:0]
	for _, qp := range r.newPatterns {
		p := r.arg(qp.mp, 0)
		lbl := r.decl(p)
		if r.err != nil {
			return
		}
		if len(r.interp.enodesOf(lbl)) > 0 {
			lblID := r.declID(lbl)
			for uint32(len(r.tmpTrees)) <= lblID {
				r.tmpTrees = append(r.tmpTrees, nil)
			}
			if r.tmpTrees[lblID] == nil {
				t, err := r.compiler.mkTree(qp.qa, qp.mp, 0, false)
				r.fail(err)
				r.tmpTrees[lblID] = t
				r.tmpTreesToDelete = append(r.tmpTreesToDelete, lbl)
			} else {
				r.fail(r.compiler.insert(r.tmpTrees[lblID], qp.qa, qp.mp, 0, true))
			}
		}
	}
	for _, lbl := range r.tmpTreesToDelete {
		lblID := r.declID(lbl)
		tmpTree := r.tmpTrees[lblID]
		if r.err == nil {
			r.interp.init(tmpTree)
			for _, app := range r.interp.enodesOf(lbl) {
				if r.relevant(app) {
					r.interp.executeCore(tmpTree, app)
				}
			}
			r.fail(r.interp.takeErr())
		}
		r.tmpTrees[lblID] = nil
	}
	r.newPatterns = r.newPatterns[:0]
}

func (r *smtMAMImpl) collectGroundExprs(qa, mp ast.ASTHandle) {
	var todo []ast.ASTHandle
	numPatterns := r.numArgs(mp)
	for i := uint32(0); i < numPatterns; i++ {
		todo = append(todo, r.arg(mp, i))
	}
	for len(todo) != 0 && r.err == nil {
		n := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if r.isGround(n) {
			e := r.mkEnode(qa, n)
			if e == nil {
				return
			}
			r.trail.push(func() { delete(r.sharedEnodes, e) })
			r.sharedEnodes[e] = struct{}{}
		} else {
			numArgs := r.numArgs(n)
			for i := uint32(0); i < numArgs; i++ {
				a := r.arg(n, i)
				if r.kind(a) == ast.ASTApp {
					todo = append(todo, a)
				}
			}
		}
	}
}

// AddPattern is mam_impl::add_pattern.
func (r *smtMAMImpl) AddPattern(qa, mp ast.ASTHandle) error {
	numPatterns := r.numArgs(mp)
	for i := uint32(0); i < numPatterns; i++ {
		if r.isGround(r.arg(mp, i)) {
			return r.takeErr() // ignore multi-pattern containing ground pattern.
		}
	}
	r.updateFilters(qa, mp)
	r.collectGroundExprs(qa, mp)
	r.newPatterns = append(r.newPatterns, smtMAMQP{qa: qa, mp: mp})
	// Incremental e-matching: for [p_1, ..., p_n] make n insertions, the
	// i-th with p_i first.
	for i := uint32(0); i < numPatterns && r.err == nil; i++ {
		r.treesAddPattern(qa, mp, i)
	}
	return r.takeErr()
}

func (r *smtMAMImpl) PushScope() { r.trail.pushScope() }

func (r *smtMAMImpl) PopScope(numScopes uint32) {
	if len(r.toMatch) != 0 {
		for _, t := range r.toMatch {
			t.resetCandidates()
		}
		r.toMatch = r.toMatch[:0]
	}
	r.newPatterns = r.newPatterns[:0]
	r.trail.popScope(int(numScopes))
}

func (r *smtMAMImpl) Reset() {
	r.trail.reset()
	r.trees = nil
	r.toMatch = r.toMatch[:0]
	r.newPatterns = r.newPatterns[:0]
	r.isPlbl = nil
	r.isClbl = nil
	r.resetPPPC()
}

// Match is mam_impl::match.
func (r *smtMAMImpl) Match() error {
	for _, t := range r.toMatch {
		r.fail(r.interp.execute(t))
		t.resetCandidates()
	}
	r.toMatch = r.toMatch[:0]
	if len(r.newPatterns) != 0 {
		r.matchNewPatterns()
		r.newPatterns = r.newPatterns[:0]
	}
	return r.takeErr()
}

// Rematch is mam_impl::rematch.
func (r *smtMAMImpl) Rematch(useIrrelevant bool) error {
	for _, t := range r.trees {
		if t == nil {
			continue
		}
		r.interp.init(t)
		for _, curr := range r.interp.enodesOf(t.rootLbl) {
			if useIrrelevant || r.relevant(curr) {
				r.interp.executeCore(t, curr)
			}
		}
		r.fail(r.interp.takeErr())
		if r.err != nil {
			break
		}
	}
	return r.takeErr()
}

// onMatch is mam_impl::on_match.
func (r *smtMAMImpl) onMatch(qa, pat ast.ASTHandle, bindings []*SMTEnode, maxGeneration uint32, used []*SMTEnode) error {
	minGen, maxGen := r.interp.getMinMaxTopGeneration()
	_, err := r.registry.AddInstanceUsed(qa, pat, bindings, used, maxGeneration, minGen, maxGen)
	return err
}

func (r *smtMAMImpl) IsShared(n *SMTEnode) bool {
	if len(r.sharedEnodes) == 0 {
		return false
	}
	_, ok := r.sharedEnodes[n]
	return ok
}

// RelevantEh is relevant_eh: with lazy, n only updates the labels and is
// not added as a candidate.
func (r *smtMAMImpl) RelevantEh(n *SMTEnode, lazy bool) {
	if n.HasLabelHash() {
		r.updateLbls(n, uint32(n.labelHash))
	}
	if len(n.arguments) > 0 {
		lbl := n.declaration
		h := r.lblHash(lbl)
		if r.isClblDecl(lbl) {
			r.updateLbls(n, h)
		}
		if r.isPlblDecl(lbl) {
			r.updateChildrenPlbls(n, h)
		}
		if !lazy {
			r.addCandidate(n)
		}
	}
}

func (r *smtMAMImpl) HasWork() bool { return len(r.toMatch) != 0 || len(r.newPatterns) != 0 }

// AddEqEh is add_eq_eh, called before r1's class merges into r2's.
func (r *smtMAMImpl) AddEqEh(r1, r2 *SMTEnode) {
	savedR1, savedR2 := r.r1, r.r2
	r.r1, r.r2 = r1, r2
	r.processPC(r1, r2)
	r.processPC(r2, r1)
	r.processPP(r1, r2)
	r.r1, r.r2 = savedR1, savedR2

	r1Plbls := r1.propagatedLabels
	r1Lbls := r1.labels
	oldLbls := r2.labels
	r.trail.push(func() { r2.labels = oldLbls })
	oldPlbls := r2.propagatedLabels
	r.trail.push(func() { r2.propagatedLabels = oldPlbls })
	r2.labels.UnionWith(r1Lbls)
	r2.propagatedLabels.UnionWith(r1Plbls)
}
