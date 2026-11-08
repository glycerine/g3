package tactic

// Source: src/util/parray.h parray_manager, as ast_manager instantiates it
// for goal::m_forms/m_proofs (expr_array_config) and goal::m_dependencies
// (expr_dependency_array_config): ref_count, preserve_roots, max_trail_sz 16,
// factor 2.
//
// A goal copy shares its arrays with the original. Updating the original
// afterwards turns the shared root into a diff cell that keeps the replaced
// value alive for the copy, so a value removed from a goal is released only
// when every older version that contains it is gone. or_else_tactical's
// `goal orig(*in)` relies on this: formulas replaced inside a branch live
// until orig is destroyed.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

const (
	goalArrayMaxTrail = 16 // array_config::max_trail_sz
	goalArrayFactor   = 2  // array_config::factor
)

type goalArrayKind uint8

const (
	goalArrayExprs goalArrayKind = iota
	goalArrayDependencies
)

// goalArrayValue is one element: an expression (formulas, proofs) or a
// dependency, according to the array's kind.
type goalArrayValue struct {
	h ast.ASTHandle
	d ast.ASTDependency
}

type goalCellKind uint8

const (
	goalCellSet goalCellKind = iota
	goalCellPushBack
	goalCellPopBack
	goalCellRoot
)

type goalCell struct {
	refs uint32
	kind goalCellKind
	// idx is m_idx for diff cells and m_size for a root.
	idx  uint32
	elem goalArrayValue
	next *goalCell
	// values has len == capacity; the first idx entries of a root are live.
	values []goalArrayValue
}

// goalArray is parray_manager::ref.
type goalArray struct {
	cell        *goalCell
	updtCounter uint32
}

// goalArrays is the parray_manager of one array kind.
type goalArrays struct {
	manager *ast.ASTManager
	kind    goalArrayKind
}

func (a goalArrays) incValue(v goalArrayValue) error {
	if a.kind == goalArrayDependencies {
		return a.manager.DependencyInc(v.d)
	}
	return goalIncExpr(a.manager, v.h)
}

func (a goalArrays) decValue(v goalArrayValue) error {
	if a.kind == goalArrayDependencies {
		return a.manager.DependencyDec(v.d)
	}
	return a.manager.DecRef(v.h)
}

func (a goalArrays) decValues(vs []goalArrayValue, n uint32) (err error) {
	for i := uint32(0); i < n; i++ {
		err = errors.Join(err, a.decValue(vs[i]))
	}
	return err
}

func goalNewCell(k goalCellKind) *goalCell { return &goalCell{refs: 1, kind: k} }

// del is parray_manager::del(cell*).
func (a goalArrays) del(c *goalCell) (err error) {
	for {
		var next *goalCell
		switch c.kind {
		case goalCellSet, goalCellPushBack:
			err = errors.Join(err, a.decValue(c.elem))
			next = c.next
		case goalCellPopBack:
			next = c.next
		case goalCellRoot:
			err = errors.Join(err, a.decValues(c.values, c.idx))
			c.values = nil
		}
		c.next = nil
		if next == nil {
			return err
		}
		next.refs--
		if next.refs > 0 {
			return err
		}
		c = next
	}
}

func goalIncCell(c *goalCell) {
	if c != nil {
		c.refs++
	}
}

func (a goalArrays) decCell(c *goalCell) error {
	if c == nil {
		return nil
	}
	c.refs--
	if c.refs == 0 {
		return a.del(c)
	}
	return nil
}

// expand grows the values array as parray_manager::expand does.
func goalExpandValues(vs []goalArrayValue) []goalArrayValue {
	capacity := len(vs)
	next := 2
	if capacity != 0 {
		next = (3*capacity + 1) >> 1
	}
	grown := make([]goalArrayValue, next)
	copy(grown, vs)
	return grown
}

func (a goalArrays) rset(vs []goalArrayValue, i uint32, v goalArrayValue) error {
	if err := a.incValue(v); err != nil {
		return err
	}
	err := a.decValue(vs[i])
	vs[i] = v
	return err
}

func (a goalArrays) rpushBack(vs *[]goalArrayValue, sz *uint32, v goalArrayValue) error {
	if int(*sz) == len(*vs) {
		*vs = goalExpandValues(*vs)
	}
	if err := a.incValue(v); err != nil {
		return err
	}
	(*vs)[*sz] = v
	*sz++
	return nil
}

func (a goalArrays) rpopBack(vs []goalArrayValue, sz *uint32) error {
	*sz--
	err := a.decValue(vs[*sz])
	vs[*sz] = goalArrayValue{}
	return err
}

// copyValues is copy_values: a new array of the same capacity holding
// references to the first sz values.
func (a goalArrays) copyValues(s []goalArrayValue, sz uint32) ([]goalArrayValue, error) {
	t := make([]goalArrayValue, len(s))
	for i := uint32(0); i < sz; i++ {
		t[i] = s[i]
		if err := a.incValue(t[i]); err != nil {
			return t, err
		}
	}
	return t, nil
}

// getValues is get_values: the values of the version at s.
func (a goalArrays) getValues(s *goalCell) ([]goalArrayValue, uint32, error) {
	var cs []*goalCell
	r := s
	for r.kind != goalCellRoot {
		cs = append(cs, r)
		r = r.next
	}
	sz := r.idx
	vs, err := a.copyValues(r.values, sz)
	if err != nil {
		return vs, sz, err
	}
	for i := len(cs); i > 0; {
		i--
		curr := cs[i]
		switch curr.kind {
		case goalCellSet:
			err = a.rset(vs, curr.idx, curr.elem)
		case goalCellPopBack:
			err = a.rpopBack(vs, &sz)
		case goalCellPushBack:
			err = a.rpushBack(&vs, &sz, curr.elem)
		}
		if err != nil {
			return vs, sz, err
		}
	}
	return vs, sz, nil
}

// unfoldCell is unfold(cell*).
func (a goalArrays) unfoldCell(c *goalCell) error {
	if c.kind == goalCellRoot {
		return nil
	}
	vs, sz, err := a.getValues(c)
	if err != nil {
		return err
	}
	next := c.next
	err = a.decCell(next)
	if c.kind == goalCellSet || c.kind == goalCellPushBack {
		err = errors.Join(err, a.decValue(c.elem))
	}
	c.next = nil
	c.elem = goalArrayValue{}
	c.kind = goalCellRoot
	c.idx = sz
	c.values = vs
	return err
}

func (r *goalArray) root() bool     { return r.cell.kind == goalCellRoot }
func (r *goalArray) unshared() bool { return r.cell.refs == 1 }

func (a goalArrays) mk(r *goalArray) error {
	err := a.decCell(r.cell)
	r.cell = goalNewCell(goalCellRoot)
	r.updtCounter = 0
	return err
}

// release is parray_manager::del(ref&).
func (a goalArrays) release(r *goalArray) error {
	err := a.decCell(r.cell)
	r.cell = nil
	r.updtCounter = 0
	return err
}

// copyRef is parray_manager::copy(s, t).
func (a goalArrays) copyRef(s goalArray, t *goalArray) error {
	goalIncCell(s.cell)
	err := a.decCell(t.cell)
	t.cell = s.cell
	t.updtCounter = 0
	return err
}

func goalArraySize(c *goalCell) uint32 {
	if c == nil {
		return 0
	}
	for {
		switch c.kind {
		case goalCellSet:
			c = c.next
		case goalCellPushBack:
			return c.idx + 1
		case goalCellPopBack:
			return c.idx - 1
		default:
			return c.idx
		}
	}
}

func (r *goalArray) size() uint32 { return goalArraySize(r.cell) }

// get is parray_manager::get, which reroots a long trail.
func (a goalArrays) get(r *goalArray, i uint32) (goalArrayValue, error) {
	if i >= r.size() {
		return goalArrayValue{}, arena.ErrBounds
	}
	trail := uint32(0)
	c := r.cell
	for {
		if trail > goalArrayMaxTrail {
			if err := a.reroot(r); err != nil {
				return goalArrayValue{}, err
			}
			return r.cell.values[i], nil
		}
		switch c.kind {
		case goalCellSet, goalCellPushBack:
			if i == c.idx {
				return c.elem, nil
			}
			trail++
			c = c.next
		case goalCellPopBack:
			trail++
			c = c.next
		default:
			return c.values[i], nil
		}
	}
}

// set is parray_manager::set(ref&, i, v).
func (a goalArrays) set(r *goalArray, i uint32, v goalArrayValue) error {
	if i >= r.size() {
		return arena.ErrBounds
	}
	if r.root() {
		if r.unshared() {
			return a.rset(r.cell.values, i, v)
		}
		if r.updtCounter > r.size() {
			if err := a.unshare(r); err != nil {
				return err
			}
			return a.rset(r.cell.values, i, v)
		}
		r.updtCounter++
		c := r.cell
		nc := goalNewCell(goalCellRoot)
		nc.idx, nc.values = c.idx, c.values
		goalIncCell(nc)
		c.kind = goalCellSet
		c.idx = i
		c.elem = c.values[i]
		if err := a.incValue(c.elem); err != nil {
			return err
		}
		c.next = nc
		c.values = nil
		err := a.decCell(c)
		r.cell = nc
		return errors.Join(err, a.rset(nc.values, i, v))
	}
	nc := goalNewCell(goalCellSet)
	nc.idx = i
	if err := a.incValue(v); err != nil {
		return err
	}
	nc.elem = v
	nc.next = r.cell
	r.cell = nc
	return nil
}

// pushBack is parray_manager::push_back(ref&, v).
func (a goalArrays) pushBack(r *goalArray, v goalArrayValue) error {
	if r.cell == nil {
		if err := a.mk(r); err != nil {
			return err
		}
	}
	if r.root() {
		if r.unshared() {
			return a.rpushBack(&r.cell.values, &r.cell.idx, v)
		}
		if r.updtCounter > r.size() {
			if err := a.unshare(r); err != nil {
				return err
			}
			return a.rpushBack(&r.cell.values, &r.cell.idx, v)
		}
		r.updtCounter++
		c := r.cell
		nc := goalNewCell(goalCellRoot)
		nc.idx, nc.values = c.idx, c.values
		goalIncCell(nc)
		c.kind = goalCellPopBack
		c.idx = nc.idx + 1
		c.next = nc
		c.values = nil
		err := a.decCell(c)
		r.cell = nc
		return errors.Join(err, a.rpushBack(&nc.values, &nc.idx, v))
	}
	nc := goalNewCell(goalCellPushBack)
	nc.idx = r.size()
	if err := a.incValue(v); err != nil {
		return err
	}
	nc.elem = v
	nc.next = r.cell
	r.cell = nc
	return nil
}

// popBack is parray_manager::pop_back(ref&).
func (a goalArrays) popBack(r *goalArray) error {
	if r.size() == 0 {
		return arena.ErrBounds
	}
	if r.root() {
		if r.unshared() {
			return a.rpopBack(r.cell.values, &r.cell.idx)
		}
		if r.updtCounter > r.size() {
			if err := a.unshare(r); err != nil {
				return err
			}
			return a.rpopBack(r.cell.values, &r.cell.idx)
		}
		r.updtCounter++
		c := r.cell
		nc := goalNewCell(goalCellRoot)
		nc.idx, nc.values = c.idx, c.values
		goalIncCell(nc)
		c.kind = goalCellPushBack
		c.idx = nc.idx - 1
		c.elem = nc.values[c.idx]
		if err := a.incValue(c.elem); err != nil {
			return err
		}
		c.next = nc
		c.values = nil
		err := a.decCell(c)
		r.cell = nc
		return errors.Join(err, a.rpopBack(nc.values, &nc.idx))
	}
	nc := goalNewCell(goalCellPopBack)
	nc.idx = r.size()
	nc.next = r.cell
	r.cell = nc
	return nil
}

// unshare is parray_manager::unshare.
func (a goalArrays) unshare(r *goalArray) error {
	if r.root() && r.unshared() {
		return nil
	}
	c := r.cell
	nc := goalNewCell(goalCellRoot)
	vs, sz, err := a.getValues(c)
	nc.values, nc.idx = vs, sz
	err = errors.Join(err, a.decCell(c))
	r.cell = nc
	r.updtCounter = 0
	return err
}

// reroot is parray_manager::reroot.
func (a goalArrays) reroot(r *goalArray) error {
	if r.root() {
		return nil
	}
	var cs []*goalCell
	split := r.size() / goalArrayFactor
	c := r.cell
	for i := uint32(0); c.kind != goalCellRoot && i < split; i++ {
		cs = append(cs, c)
		c = c.next
	}
	if c.kind != goalCellRoot {
		if err := a.unfoldCell(c); err != nil {
			return err
		}
	}
	var err error
	for i := len(cs); i > 0; {
		i--
		p := cs[i]
		sz := c.idx
		vs := c.values
		switch p.kind {
		case goalCellSet:
			c.kind = goalCellSet
			c.idx = p.idx
			c.elem = vs[c.idx]
			vs[p.idx] = p.elem
		case goalCellPushBack:
			c.kind = goalCellPopBack
			if int(sz) == len(vs) {
				vs = goalExpandValues(vs)
			}
			c.idx = sz
			vs[sz] = p.elem
			sz++
		case goalCellPopBack:
			c.kind = goalCellPushBack
			sz--
			c.idx = sz
			c.elem = vs[sz]
		}
		goalIncCell(p)
		c.next = p
		c.values = nil
		err = errors.Join(err, a.decCell(c))
		p.kind = goalCellRoot
		p.idx = sz
		p.values = vs
		p.elem = goalArrayValue{}
		p.next = nil
		c = p
	}
	r.updtCounter = 0
	return err
}

func (g *Goal) exprArrays() goalArrays { return goalArrays{manager: g.manager, kind: goalArrayExprs} }
func (g *Goal) depArrays() goalArrays {
	return goalArrays{manager: g.manager, kind: goalArrayDependencies}
}

func (g *Goal) formAt(i uint32) (ast.ASTHandle, error) {
	v, err := g.exprArrays().get(&g.forms, i)
	return v.h, err
}

func (g *Goal) proofAt(i uint32) (ast.ASTHandle, error) {
	v, err := g.exprArrays().get(&g.proofs, i)
	return v.h, err
}

func (g *Goal) dependencyAt(i uint32) (ast.ASTDependency, error) {
	v, err := g.depArrays().get(&g.dependencies, i)
	return v.d, err
}

// formSlice reads every formula in order, as loops over goal::form(i) do.
func (g *Goal) formSlice() []ast.ASTHandle {
	n := g.forms.size()
	out := make([]ast.ASTHandle, 0, n)
	for i := uint32(0); i < n; i++ {
		h, _ := g.formAt(i)
		out = append(out, h)
	}
	return out
}

func (g *Goal) proofSlice() []ast.ASTHandle {
	n := g.proofs.size()
	out := make([]ast.ASTHandle, 0, n)
	for i := uint32(0); i < n; i++ {
		h, _ := g.proofAt(i)
		out = append(out, h)
	}
	return out
}

func (g *Goal) dependencySlice() []ast.ASTDependency {
	n := g.dependencies.size()
	out := make([]ast.ASTDependency, 0, n)
	for i := uint32(0); i < n; i++ {
		d, _ := g.dependencyAt(i)
		out = append(out, d)
	}
	return out
}
