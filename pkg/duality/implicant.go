package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrUnresolvedITE = errors.New("unresolved ite in model")
var ErrUnresolvedIff = errors.New("disaster in SubtermTruth processing equivalence")

// implicantState holds the locals of UnderapproxFormula and Implicant. The
// done sets (Implicant's hash_set<ast> done[2]) own their members and die
// before the conjunction is built; lits owns the negated literals until the
// conjunction exists; the memo owns the temporaries SubtermTruth recorded.
// ResolveIte results live in the graph's resolve_ite_memo.
type implicantState struct {
	graph     *RPFP
	truth     map[ast.ASTHandle]int
	done      [2]*iz3Table
	ignore    map[ast.ASTHandle]bool
	pins      []*Expr
	memoTemps []*Expr
	literals  []ast.ASTHandle
}

func (s *implicantState) keep(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pin, err := NewExpr(s.graph.context, h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.pins = append(s.pins, pin)
	return h, nil
}
func (s *implicantState) close() (result error) {
	result = s.closeDone()
	for _, pin := range s.pins {
		result = errors.Join(result, pin.Close())
	}
	s.pins = nil
	for _, pin := range s.memoTemps {
		result = errors.Join(result, pin.Close())
	}
	s.memoTemps = nil
	return
}

// closeDone is ~hash_set<ast> done[2] at Implicant's return: done[1] first.
func (s *implicantState) closeDone() (result error) {
	m := s.graph.context.manager
	for i := 1; i >= 0; i-- {
		if s.done[i] != nil {
			result = errors.Join(result, s.done[i].clear(m))
		}
	}
	return
}

func (s *implicantState) isDone(polarity int, h ast.ASTHandle) (bool, error) {
	id, err := s.graph.context.manager.GetASTID(h)
	if err != nil {
		return false, err
	}
	return s.done[polarity].find(uint64(id)) != nil, nil
}

func (s *implicantState) markDone(polarity int, h ast.ASTHandle) error {
	m := s.graph.context.manager
	id, err := m.GetASTID(h)
	if err != nil {
		return err
	}
	e, inserted := s.done[polarity].lookup(uint64(id))
	if !inserted {
		return nil
	}
	e.key, err = m.NewRef(h)
	return err
}

// UnderapproxFormula follows ImplicantRed, including the source equivalence
// rule that records only the right operand's selected polarity.
// Source: duality_rpfp.cpp:1746-1820,1853-1889,1940-1963.
func (g *RPFP) UnderapproxFormula(formula *Expr, dontCares []*Expr) (out *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	// Both source callers clear resolve_ite_memo just before this call.
	if err := g.clearResolveIteMemo(); err != nil {
		return nil, err
	}
	state := &implicantState{graph: g, truth: make(map[ast.ASTHandle]int), ignore: make(map[ast.ASTHandle]bool)}
	state.done[0] = newIZ3Table()
	state.done[1] = newIZ3Table()
	defer func() {
		result = errors.Join(result, state.close())
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	for _, term := range dontCares {
		if err := term.open(); err != nil {
			return nil, err
		}
		if term.value.object.context != g.context {
			return nil, ErrContextMismatch
		}
		h, err := term.Raw()
		if err != nil {
			return nil, err
		}
		state.ignore[h] = true
	}
	h, err := formula.Raw()
	if err != nil {
		return nil, err
	}
	if err = state.reduce(h, true); err != nil {
		return nil, err
	}
	if err = state.closeDone(); err != nil {
		return nil, err
	}
	m := g.context.manager
	var combined ast.ASTHandle
	// Z3User::conjoin is ctx.make(And,args): (and) and (and x) included.
	combined, err = m.MkAndArgs(state.literals)
	if err != nil {
		return nil, err
	}
	return NewExpr(g.context, combined)
}
func (s *implicantState) reduce(h ast.ASTHandle, positive bool) (result error) {
	polarity := 0
	if positive {
		polarity = 1
	}
	if done, err := s.isDone(polarity, h); err != nil || done {
		return err
	}
	defer func() {
		if result == nil {
			result = s.markDone(polarity, h)
		}
	}()
	m := s.graph.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	if kind == ast.ASTApp {
		family, err := m.GetFamilyID(h)
		if err != nil {
			return err
		}
		if family == m.BasicFamilyID() {
			op, err := m.GetDeclKind(h)
			if err != nil {
				return err
			}
			switch op {
			case ast.ASTOpImplies:
				a, err := m.GetArg(h, 0)
				if err != nil {
					return err
				}
				b, err := m.GetArg(h, 1)
				if err != nil {
					return err
				}
				// ImplicantRed(memo, f.arg(1) || !f.arg(0), ...): both
				// temporaries die after the call; the done set (and the
				// memo, if SubtermTruth saw them) keep them alive.
				return s.reduceImpliesTemp(a, b, positive)
			case ast.ASTOpIff:
				a, err := m.GetArg(h, 0)
				if err != nil {
					return err
				}
				value, err := s.graph.subtermTruth(s.truth, a)
				if err != nil {
					return err
				}
				if value == 2 {
					return ErrUnresolvedIff
				}
				b, err := m.GetArg(h, 1)
				if err != nil {
					return err
				}
				selected := value == 1
				if !positive {
					selected = !selected
				}
				return s.reduce(b, selected)
			case ast.ASTOpAnd, ast.ASTOpOr:
				all := (positive && op == ast.ASTOpAnd) || (!positive && op == ast.ASTOpOr)
				n, err := m.GetNumArgs(h)
				if err != nil {
					return err
				}
				for i := uint32(0); i < n; i++ {
					child, err := m.GetArg(h, i)
					if err != nil {
						return err
					}
					if !all {
						value, err := s.graph.timedSubtermTruth(s.truth, child)
						if err != nil {
							return err
						}
						if (positive && value != 1) || (!positive && value != 0) {
							continue
						}
					}
					if err = s.reduce(child, positive); err != nil {
						return err
					}
					if !all {
						return nil
					}
				}
				// Source silently emits no literal if no branch matches the model.
				return nil
			case ast.ASTOpNot:
				child, err := m.GetArg(h, 0)
				if err != nil {
					return err
				}
				return s.reduce(child, !positive)
			}
		}
	}
	if s.ignore[h] {
		return nil
	}
	resolved, err := s.resolve(h)
	if err != nil {
		return err
	}
	if !positive {
		resolved, err = s.keep(m.MkNot(resolved))
		if err != nil {
			return err
		}
	}
	s.literals = append(s.literals, resolved)
	return nil
}
func (s *implicantState) reduceImpliesTemp(a, b ast.ASTHandle, positive bool) (result error) {
	m := s.graph.context.manager
	notA, err := m.MkNot(a)
	if err != nil {
		return err
	}
	neg, err := NewExpr(s.graph.context, notA)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.releaseTemp(neg)) }()
	or, err := m.MkOr(b, notA)
	if err != nil {
		return err
	}
	rewritten, err := NewExpr(s.graph.context, or)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.releaseTemp(rewritten)) }()
	return s.reduce(or, positive)
}

// releaseTemp ends a temporary; a SubtermTruth memo entry keeps it until the
// memo dies with UnderapproxFormula's locals.
func (s *implicantState) releaseTemp(temp *Expr) error {
	h, err := temp.Raw()
	if err != nil {
		return err
	}
	if _, ok := s.truth[h]; ok {
		s.memoTemps = append(s.memoTemps, temp)
		return nil
	}
	return temp.Close()
}

func (s *implicantState) resolve(h ast.ASTHandle) (ast.ASTHandle, error) {
	m0 := s.graph.context.manager
	id, err := m0.GetASTID(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if e := s.graph.resolveIteMemo.find(uint64(id)); e != nil {
		return m0.RefGet(e.value)
	}
	m := s.graph.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result := h
	if kind == ast.ASTApp {
		family, err := m.GetFamilyID(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if family == m.BasicFamilyID() && op == ast.ASTOpIte {
			cond, err := m.GetArg(h, 0)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			value, err := s.graph.timedSubtermTruth(s.truth, cond)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if value == 2 {
				return ast.ASTHandle{}, ErrUnresolvedITE
			}
			if err = s.reduce(cond, value == 1); err != nil {
				return ast.ASTHandle{}, err
			}
			index := uint32(2)
			if value == 1 {
				index = 1
			}
			branch, err := m.GetArg(h, index)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = s.resolve(branch)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		} else {
			n, err := m.GetNumArgs(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				arg, err := m.GetArg(h, i)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				args[i], err = s.resolve(arg)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			decl, err := m.GetDecl(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = m.MkApp(decl, args)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	}
	// resolve_ite_memo[t] = res: the memo owns both, past this call.
	e, inserted := s.graph.resolveIteMemo.lookup(uint64(id))
	if inserted {
		if e.key, err = m.NewRef(h); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if e.value != (ast.ASTRef{}) {
		if err = m.DelRef(&e.value); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if e.value, err = m.NewRef(result); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

// clearResolveIteMemo is resolve_ite_memo.clear().
func (g *RPFP) clearResolveIteMemo() error {
	if g.resolveIteMemo == nil {
		g.resolveIteMemo = newIZ3Table()
		return nil
	}
	return g.resolveIteMemo.clear(g.context.manager)
}

// FixCurrentState records model-selected edge decisions in the current graph
// scope; the caller pushes that scope before expanding the next search level.
// Source: duality_rpfp.cpp:2519-2527.
func (g *RPFP) FixCurrentState(edge *Edge) (result error) {
	if err := g.solverReady(); err != nil {
		return err
	}
	if edge == nil || edge.Owner != g || edge.closed {
		return ErrWrapperOwner
	}
	m := g.context.manager
	var h ast.ASTHandle
	var err error
	if edge.dual == (ast.ASTRef{}) {
		h, err = m.MkTrue()
	} else {
		h, err = m.RefGet(edge.dual)
	}
	if err != nil {
		return err
	}
	formula, err := NewExpr(g.context, h)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, formula.Close()) }()
	// Underapproximation flags are temporary query controls. Fixing their
	// model polarity can contradict the next expansion-selection assumptions.
	// RPFP::FixCurrentState passes an empty dont_cares set; that is a z3
	// 4.7.1 bug (the next ExpansionChoices check is unsat and z3 aborts with
	// "error in ExpansionChoices"), see z3_bug_exclusions.txt and AUDIT.md
	// duality_02-F2.
	var controls []*Expr
	defer func() {
		for _, control := range controls {
			result = errors.Join(result, control.Close())
		}
	}()
	for _, ref := range g.underapproxFlagRefs {
		h, err := m.RefGet(ref)
		if err != nil {
			return err
		}
		control, err := NewExpr(g.context, h)
		if err != nil {
			return err
		}
		controls = append(controls, control)
	}
	timer, err := startProfile(g.context, "UnderapproxFormula")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, timer.Close()) }()
	decision, err := g.UnderapproxFormula(formula, controls)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, decision.Close()) }()
	if err = timer.Close(); err != nil {
		return err
	}
	return g.ConstrainEdgeLocalized(edge, decision)
}
