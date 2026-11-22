package rewriter

import (
	"errors"
	"unsafe"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// GroundInstancePlans serves one quantifier instantiator. A body handle
// includes its owner and generation. Remembering its traversal keeps no AST
// alive. The caller holds the quantified formula and bindings during a call.
type GroundInstancePlans struct {
	manager *ast.ASTManager
	bodies  map[ast.ASTHandle]groundInstanceEntry
	held    []ast.ASTHandle
	args    []ast.ASTHandle
	size    int
}

type groundInstanceEntry struct {
	uses uint8
	plan *groundInstancePlan
}

type groundInstancePlan struct {
	values    []ast.ASTHandle
	variables []groundVariable
	apps      []groundApplication
	edges     []uint16
	maxArgs   int
}

type groundVariable struct {
	sort      ast.ASTHandle
	index     uint32
	dest      uint16
	condition bool
}

type groundApplication struct {
	source, decl       ast.ASTHandle
	first, arity, dest uint16
}

func (c *GroundInstancePlans) release() error {
	if len(c.held) == 0 {
		return nil
	}
	var err error
	c.held, err = c.manager.ShrinkHeldRefs(c.held, 0)
	return err
}

// Clear discards weak traversal metadata before the instantiator drops its
// quantified formulas. The source var_subst retains its own cache until reset.
func (c *GroundInstancePlans) Clear() error {
	if err := c.release(); err != nil {
		return err
	}
	*c = GroundInstancePlans{}
	return nil
}

// Substitute uses the ordinary var_subst for cold terms, symbolic bindings,
// folding conditionals, binders, traces and bounded resource checks.
func (c *GroundInstancePlans) Substitute(s *VarSubstitution, body ast.ASTHandle, bindings []ast.ASTHandle, out ast.ASTRef) error {
	if s == nil {
		return ast.ErrASTHandle
	}
	r := &s.reducer.engine
	if err := r.open(); err != nil {
		return err
	}
	if err := c.release(); err != nil {
		return err
	}
	m := r.core.manager
	if c.manager != nil && c.manager != m {
		return ast.ErrASTHandle
	}
	if !r.substituteVariables || !s.standardOrder || diagnostic.TraceCompiled() || m.TraceStream != nil ||
		m.Budget().InjectionArmed() || m.ResourceLimit().Limit != 0 || m.ResourceLimit().CancelFlag() {
		return s.Substitute(body, bindings, out)
	}
	plan := c.plan(m, body)
	if plan == nil || !plan.bind(m, bindings) {
		return s.Substitute(body, bindings, out)
	}
	// An earlier short-circuit may have left the rewriter's result ref live.
	// The ordinary traversal releases that value at its original assignment.
	previous, err := m.RefGetHeld(r.r)
	if err != nil || !previous.IsNull() {
		return s.Substitute(body, bindings, out)
	}
	if err := s.Reset(); err != nil {
		return err
	}
	return c.apply(r, plan, out)
}

func (c *GroundInstancePlans) plan(m *ast.ASTManager, body ast.ASTHandle) *groundInstancePlan {
	entry, seen := c.bodies[body]
	if entry.uses == 4 {
		return entry.plan
	}
	if !seen && len(c.bodies) == 128 {
		clear(c.bodies)
		c.size = 0
	}
	if c.bodies == nil {
		c.manager = m
		c.bodies = make(map[ast.ASTHandle]groundInstanceEntry)
	}
	entry.uses++
	if entry.uses == 4 {
		entry.plan = compileGroundInstance(m, body)
		if p := entry.plan; p != nil {
			// Bound the backing arrays held by each active solver. The table
			// and the scratch for evaluating one body have separate fixed bounds.
			bytes := cap(p.values)*int(unsafe.Sizeof(ast.ASTHandle{})) +
				cap(p.variables)*int(unsafe.Sizeof(groundVariable{})) +
				cap(p.apps)*int(unsafe.Sizeof(groundApplication{})) + cap(p.edges)*2
			if c.size+bytes > 128<<10 {
				clear(c.bodies)
				c.size = 0
			}
			c.size += bytes
		} else if m.ResourceLimit().CancelFlag() {
			return nil
		}
	}
	c.bodies[body] = entry
	return entry.plan
}

func literalBoolean(m *ast.ASTManager, h ast.ASTHandle) bool {
	app, family, op, err := m.AppIdentity(h)
	return err == nil && app && family == m.BasicFamilyID() && (op == ast.ASTOpTrue || op == ast.ASTOpFalse)
}

func compileGroundInstance(m *ast.ASTManager, root ast.ASTHandle) *groundInstancePlan {
	p := new(groundInstancePlan)
	ids := make(map[ast.ASTHandle]uint16)
	conditions := make(map[ast.ASTHandle]bool)
	var visit func(ast.ASTHandle, int) (uint16, bool)
	visit = func(h ast.ASTHandle, depth int) (uint16, bool) {
		if id, ok := ids[h]; ok {
			return id, true
		}
		if depth >= 64 || len(p.values) == 384 || m.ResourceLimit().CancelFlag() {
			return 0, false
		}
		kind, _, n, ground, err := m.SubstitutionShape(h)
		if err != nil {
			return 0, false
		}
		id := uint16(len(p.values))
		ids[h] = id
		p.values = append(p.values, h)
		if ground {
			return id, true
		}
		if kind == ast.ASTVar {
			sort, err := m.GetSort(h)
			if err != nil {
				return 0, false
			}
			p.variables = append(p.variables, groundVariable{sort: sort, index: n, dest: id})
			return id, true
		}
		if kind != ast.ASTApp || n == 0 || n > 64 {
			return 0, false
		}
		header, err := m.NodeHeader(h)
		if err != nil {
			return 0, false
		}
		first := len(p.edges)
		p.edges = append(p.edges, make([]uint16, n)...)
		for i := uint32(0); i < n; i++ {
			child, err := m.GetArg(h, i)
			if err != nil {
				return 0, false
			}
			if i == 0 && header.Family == m.BasicFamilyID() && header.DeclKind == ast.ASTOpIte {
				if n != 3 || literalBoolean(m, child) {
					return 0, false
				}
				conditions[child] = true
			}
			childID, ok := visit(child, depth+1)
			if !ok {
				return 0, false
			}
			p.edges[first+int(i)] = childID
		}
		p.apps = append(p.apps, groundApplication{source: h, decl: header.Decl, first: uint16(first), arity: uint16(n), dest: id})
		p.maxArgs = max(p.maxArgs, int(n))
		return id, true
	}
	if _, ok := visit(root, 0); !ok || len(p.apps) == 0 {
		return nil
	}
	for i := range p.variables {
		v := &p.variables[i]
		v.condition = conditions[p.values[v.dest]]
	}
	return p
}

func (p *groundInstancePlan) bind(m *ast.ASTManager, bindings []ast.ASTHandle) bool {
	for _, v := range p.variables {
		if uint64(v.index) >= uint64(len(bindings)) {
			return false
		}
		h := bindings[len(bindings)-1-int(v.index)]
		if h.IsNull() || v.condition && literalBoolean(m, h) {
			return false
		}
		sort, err := m.GetSort(h)
		if err != nil || sort != v.sort {
			return false
		}
		if ground, err := m.IsGround(h); err != nil || !ground {
			return false
		}
		p.values[v.dest] = h
	}
	return true
}

func (c *GroundInstancePlans) apply(r *rewriterEngine, p *groundInstancePlan, out ast.ASTRef) (err error) {
	m := r.core.manager
	if len(c.args) < p.maxArgs {
		c.args = make([]ast.ASTHandle, p.maxArgs)
	}
	defer func() {
		if releaseErr := c.release(); releaseErr != nil {
			err = errors.Join(err, releaseErr)
		}
	}()
	for i, op := range p.apps {
		// With no folding or binders, each distinct non-ground application
		// pushes one frame. A second visit to a shared node is a cache hit.
		// The initial rewrite poll and the frame polls total 2*N. The plan
		// has fewer nodes than the activity cache's minimum eviction limit.
		if err := r.checkCancel(); err != nil {
			return err
		}
		if err := r.checkCancel(); err != nil {
			return err
		}
		retain := false
		if i+1 < len(p.apps) {
			rc, err := m.GetRefCount(op.source)
			if err != nil {
				return err
			}
			retain = rc > 1
		}
		args := c.args[:op.arity]
		for j, child := range p.edges[int(op.first) : int(op.first)+int(op.arity)] {
			args[j] = p.values[child]
		}
		result, err := m.RewriterMakeApp(op.decl, ast.SliceArguments(args))
		if err != nil {
			return err
		}
		if err := m.IncRefHeld(result); err != nil {
			return err
		}
		c.held = append(c.held, result)
		p.values[op.dest] = result
		if retain {
			if err := r.core.cacheResult(op.source, result); err != nil {
				return err
			}
		}
	}
	return m.SetRef(out, p.values[0])
}
