package substitution

// Source: src/ast/substitution/substitution.cpp;
// src/ast/substitution/substitution.h.

import (
	"bytes"
	"errors"
	"math"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type substitutionState uint8

const (
	substitutionClean substitutionState = iota
	substitutionApply
	substitutionInsert
)

// VariableOffset identifies one substitution-domain binding.
type VariableOffset struct {
	Variable uint32
	Offset   uint32
}

// Substitution maps de Bruijn variable/offset pairs to expression/offset
// pairs and owns every expression stored in its range.
type Substitution struct {
	self      *Substitution
	manager   *ast.ASTManager
	bindings  varOffsetExprMap
	order     []VariableOffset
	refs      []ast.ASTRef
	scopes    []int
	cache     map[ExprOffset]ast.ASTHandle
	cacheRefs []ast.ASTRef
	todo      []ExprOffset
	state     substitutionState
	closed    bool
}

func New(m *ast.ASTManager) (*Substitution, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	s := &Substitution{manager: m, cache: make(map[ExprOffset]ast.ASTHandle)}
	s.self = s
	return s, nil
}

func (s *Substitution) open() error {
	if s == nil || s.self != s || s.manager == nil || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *Substitution) Reserve(offsets, variables uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if len(s.order) != 0 {
		return ast.ErrASTPending
	}
	return s.bindings.reserve(offsets, variables)
}

func (s *Substitution) ReserveOffsets(offsets uint32) error {
	return s.Reserve(offsets, s.bindings.variables)
}

func (s *Substitution) ReserveVariables(variables uint32) error {
	return s.Reserve(s.bindings.offsets, variables)
}

// release is expr_ref_vector::reset: front to back, keeping progress so a
// refused release can be retried.
func (s *Substitution) release(refs *[]ast.ASTRef) error {
	for len(*refs) != 0 {
		if err := s.manager.DelRef(&(*refs)[0]); err != nil {
			return err
		}
		*refs = (*refs)[1:]
	}
	return nil
}

func (s *Substitution) ResetCache() error {
	if err := s.open(); err != nil {
		return err
	}
	clear(s.cache)
	if err := s.release(&s.cacheRefs); err != nil {
		return err
	}
	s.state = substitutionClean
	return nil
}

func (s *Substitution) Reset() error {
	if err := s.open(); err != nil {
		return err
	}
	s.bindings.reset()
	s.order, s.scopes = nil, nil
	if err := s.release(&s.refs); err != nil {
		return err
	}
	return s.ResetCache()
}

func (s *Substitution) PushScope() error {
	if err := s.open(); err != nil {
		return err
	}
	s.scopes = append(s.scopes, len(s.order))
	return nil
}

func (s *Substitution) PopScope(count uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if count == 0 || uint64(count) > uint64(len(s.scopes)) {
		return ast.ErrASTDomain
	}
	newLevel := len(s.scopes) - int(count)
	oldSize := s.scopes[newLevel]
	for i := oldSize; i < len(s.order); i++ {
		v := s.order[i]
		if err := s.bindings.erase(v.Variable, v.Offset); err != nil {
			return err
		}
	}
	s.order = s.order[:oldSize]
	s.scopes = s.scopes[:newLevel]
	// m_refs.shrink(old_sz) releases front to back.
	for i := oldSize; i < len(s.refs); i++ {
		if s.refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := s.manager.DelRef(&s.refs[i]); err != nil {
			return err
		}
	}
	s.refs = s.refs[:oldSize]
	return s.ResetCache()
}

func (s *Substitution) ScopeLevel() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.scopes)), nil
}

func (s *Substitution) TopScopeHasBindings() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	start := 0
	if len(s.scopes) != 0 {
		start = s.scopes[len(s.scopes)-1]
	}
	return start < len(s.order), nil
}

func (s *Substitution) NumBindings() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.order)), nil
}

func (s *Substitution) Insert(variable, offset uint32, target ExprOffset) error {
	if err := s.open(); err != nil {
		return err
	}
	if target.expr.IsNull() {
		return ast.ErrASTHandle
	}
	r, err := s.manager.NewRef(target.expr)
	if err != nil {
		return err
	}
	if err = s.bindings.insert(variable, offset, target); err != nil {
		return errors.Join(err, s.manager.DelRef(&r))
	}
	s.order = append(s.order, VariableOffset{variable, offset})
	s.refs = append(s.refs, r)
	s.state = substitutionInsert
	return nil
}

func (s *Substitution) InsertVariable(variable ast.ASTHandle, offset uint32, target ExprOffset) error {
	i, err := s.manager.GetVarIndex(variable)
	if err != nil {
		return err
	}
	return s.Insert(i, offset, target)
}

func (s *Substitution) Find(variable, offset uint32) (ExprOffset, bool, error) {
	if err := s.open(); err != nil {
		return ExprOffset{}, false, err
	}
	return s.bindings.find(variable, offset)
}

func (s *Substitution) Contains(variable ast.ASTHandle, offset uint32) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	index, err := s.manager.GetVarIndex(variable)
	if err != nil {
		return false, err
	}
	_, found, err := s.Find(index, offset)
	return found, err
}

func (s *Substitution) Binding(index uint32) (VariableOffset, ExprOffset, error) {
	if err := s.open(); err != nil {
		return VariableOffset{}, ExprOffset{}, err
	}
	if uint64(index) >= uint64(len(s.order)) {
		return VariableOffset{}, ExprOffset{}, ast.ErrASTDomain
	}
	v := s.order[index]
	r, ok, err := s.bindings.find(v.Variable, v.Offset)
	if err != nil {
		return VariableOffset{}, ExprOffset{}, err
	}
	if !ok {
		return VariableOffset{}, ExprOffset{}, ast.ErrASTDomain
	}
	return v, r, nil
}

// reserveCacheRef records the owner before a constructor can create a fresh
// expression. ResetCache can then release partial work after any allocation
// failure, just as m_new_exprs and local expr_ref owners do in the source.
func (s *Substitution) reserveCacheRef() (ast.ASTRef, error) {
	r, err := s.manager.NewRef(ast.ASTHandle{})
	if err == nil {
		s.cacheRefs = append(s.cacheRefs, r)
	}
	return r, err
}

func (s *Substitution) keep(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := s.manager.NewRef(h)
	if err == nil {
		s.cacheRefs = append(s.cacheRefs, r)
	}
	return h, err
}

func add32(a, b uint32) (uint32, error) {
	if b > math.MaxUint32-a {
		return 0, ast.ErrASTDomain
	}
	return a + b, nil
}

func (s *Substitution) Apply(offsets uint32, deltas []uint32, n ExprOffset, out ast.ASTRef) (result error) {
	return s.ApplyModulo(offsets, deltas, n, ExprOffset{}, ExprOffset{}, out)
}

func (s *Substitution) ApplyExpression(n ast.ASTHandle, out ast.ASTRef) error {
	return s.Apply(1, []uint32{0}, NewExprOffset(n, 0), out)
}

// ApplyModulo is substitution::apply(num_actual_offsets, deltas, n, s, t,
// result): an iterative post-order walk over m_todo, caching each
// expression/offset's image in m_apply_cache and owning every new image in
// m_new_exprs. A quantifier's body is applied by a nested substitution whose
// bindings are var_shifter-shifted past the quantifier's declarations.
func (s *Substitution) ApplyModulo(offsets uint32, deltas []uint32, n, replace, replacement ExprOffset, out ast.ASTRef) (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if uint64(offsets) > uint64(len(deltas)) {
		return ast.ErrASTDomain
	}
	deltas = deltas[:offsets]
	// It is incorrect to cache results between different calls if we are
	// applying a substitution modulo a substitution s -> t.
	modulo := !replace.expr.IsNull()
	if s.state == substitutionInsert || modulo {
		if err := s.ResetCache(); err != nil {
			return err
		}
	}
	s.state = substitutionApply
	defer func() {
		if modulo {
			result = errors.Join(result, s.ResetCache())
		}
	}()
	s.todo = append(s.todo[:0], n)
	// expanding records the m_todo slot whose children are pending. The
	// source requires an acyclic substitution (it would not terminate); a
	// second uncached slot of the same term while the first is pending is
	// a cycle, refused here.
	expanding := make(map[ExprOffset]int)
	for len(s.todo) != 0 {
		top := len(s.todo) - 1
		current := s.todo[top]
		if _, cached := s.cache[current]; cached {
			s.todo = s.todo[:top]
			continue
		}
		if slot, pending := expanding[current]; pending && slot != top {
			return ast.ErrASTDomain
		}
		expanding[current] = top
		prime := current
		if modulo && current == replace {
			prime = replacement
		}
		visited := true
		e, off := prime.expr, prime.offset
		kind, err := s.manager.GetKind(e)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTVar:
			index, err := s.manager.GetVarIndex(e)
			if err != nil {
				return err
			}
			target, found, err := s.bindings.find(index, off)
			if err != nil {
				return err
			}
			if found {
				if _, cached := s.cache[target]; !cached {
					s.todo = append(s.todo, target)
					visited = false
				}
				if visited {
					s.todo = s.todo[:len(s.todo)-1]
					s.cache[current] = s.cache[target]
				}
				break
			}
			s.todo = s.todo[:len(s.todo)-1]
			if int(off) >= len(deltas) {
				return ast.ErrASTDomain
			}
			image := e
			if delta := deltas[off]; delta > 0 {
				shifted, err := add32(index, delta)
				if err != nil {
					return err
				}
				sort, err := s.manager.GetSort(e)
				if err != nil {
					return err
				}
				reference, err := s.reserveCacheRef()
				if err != nil {
					return err
				}
				image, err = s.manager.MkVar(shifted, sort)
				if err = errors.Join(err, s.manager.SetRef(reference, image)); err != nil {
					return err
				}
			}
			s.cache[current] = image
		case ast.ASTApp:
			count, err := s.manager.GetNumArgs(e)
			if err != nil {
				return err
			}
			for j := count; j > 0; j-- {
				arg, err := s.manager.GetArg(e, j-1)
				if err != nil {
					return err
				}
				child := NewExprOffset(arg, off)
				if _, cached := s.cache[child]; !cached {
					s.todo = append(s.todo, child)
					visited = false
				}
			}
			if !visited {
				break
			}
			s.todo = s.todo[:len(s.todo)-1]
			args := make([]ast.ASTHandle, count)
			changed := false
			for i := range args {
				arg, err := s.manager.GetArg(e, uint32(i))
				if err != nil {
					return err
				}
				image, cached := s.cache[NewExprOffset(arg, off)]
				if !cached {
					return ast.ErrASTDomain
				}
				args[i] = image
				changed = changed || image != arg
			}
			if !changed {
				s.cache[current] = e
				break
			}
			declaration, err := s.manager.GetDecl(e)
			if err != nil {
				return err
			}
			reference, err := s.reserveCacheRef()
			if err != nil {
				return err
			}
			image, err := s.manager.MkApp(declaration, args)
			if err = errors.Join(err, s.manager.SetRef(reference, image)); err != nil {
				return err
			}
			s.cache[current] = image
		case ast.ASTQuantifier:
			image, err := s.applyQuantifier(offsets, deltas, e, off, replace, replacement)
			if err != nil {
				return err
			}
			s.todo = s.todo[:len(s.todo)-1]
			s.cache[current] = image
		default:
			return ast.ErrASTDomain
		}
	}
	h, cached := s.cache[n]
	if !cached {
		return ast.ErrASTDomain
	}
	// m_new_exprs retains the result of every apply call, including an
	// unchanged input whose descendants are borrowed by the persistent cache.
	if _, err := s.keep(h); err != nil {
		return err
	}
	return s.manager.SetRef(out, h)
}

// applyQuantifier is apply's AST_QUANTIFIER case: a nested substitution
// holds every binding var_shifter-shifted by the quantifier's declarations,
// and applies itself, modulo the shifted s -> t, to the patterns, the
// no-patterns and the body. The new quantifier joins m_new_exprs; the block's
// locals die in reverse declaration order: no_pats, pats, t1_ref, s1_ref,
// var_sh, er, subst.
func (s *Substitution) applyQuantifier(offsets uint32, deltas []uint32, q ast.ASTHandle, off uint32, replace, replacement ExprOffset) (image ast.ASTHandle, resultErr error) {
	m := s.manager
	numVars, err := m.GetNumDeclarations(q)
	if err != nil {
		return image, err
	}
	nested, err := New(m)
	if err != nil {
		return image, err
	}
	defer func() { resultErr = errors.Join(resultErr, nested.Close()) }()
	variables, err := add32(s.bindings.variables, numVars)
	if err != nil {
		return image, err
	}
	if err = nested.Reserve(s.bindings.offsets, variables); err != nil {
		return image, err
	}
	er, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return image, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&er)) }()
	shifter, err := rewriter.NewVarShifter(m)
	if err != nil {
		if shifter != nil {
			err = errors.Join(err, shifter.Close())
		}
		return image, err
	}
	defer func() { resultErr = errors.Join(resultErr, shifter.Close()) }()
	for i := uint32(0); i < s.bindings.offsets; i++ {
		for j := uint32(0); j < s.bindings.variables; j++ {
			r, found, err := s.bindings.find(j, i)
			if err != nil {
				return image, err
			}
			if !found {
				continue
			}
			if err = shifter.ShiftBy(r.expr, numVars, er); err != nil {
				return image, err
			}
			shifted, err := m.RefGet(er)
			if err != nil {
				return image, err
			}
			if err = nested.Insert(j+numVars, i, NewExprOffset(shifted, r.offset)); err != nil {
				return image, err
			}
		}
	}
	body, err := m.GetQuantifierBody(q)
	if err != nil {
		return image, err
	}
	s1Ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return image, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&s1Ref)) }()
	t1Ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return image, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&t1Ref)) }()
	if !replace.expr.IsNull() {
		if err = shifter.ShiftBy(replace.expr, numVars, s1Ref); err != nil {
			return image, err
		}
	}
	if !replacement.expr.IsNull() {
		if err = shifter.ShiftBy(replacement.expr, numVars, t1Ref); err != nil {
			return image, err
		}
	}
	s1Expr, err := m.RefGet(s1Ref)
	if err != nil {
		return image, err
	}
	t1Expr, err := m.RefGet(t1Ref)
	if err != nil {
		return image, err
	}
	s1, t1 := NewExprOffset(s1Expr, replace.offset), NewExprOffset(t1Expr, replacement.offset)
	var pats, noPats []ast.ASTRef
	defer func() {
		for i := range noPats {
			resultErr = errors.Join(resultErr, m.DelRef(&noPats[i]))
		}
		for i := range pats {
			resultErr = errors.Join(resultErr, m.DelRef(&pats[i]))
		}
	}()
	applyChild := func(child ast.ASTHandle) (ast.ASTHandle, error) {
		if err := nested.ApplyModulo(offsets, deltas, NewExprOffset(child, off), s1, t1, er); err != nil {
			return ast.ASTHandle{}, err
		}
		return m.RefGet(er)
	}
	numPatterns, err := m.GetNumPatterns(q)
	if err != nil {
		return image, err
	}
	patterns := make([]ast.ASTHandle, numPatterns)
	for i := range patterns {
		pattern, err := m.GetPattern(q, uint32(i))
		if err != nil {
			return image, err
		}
		if patterns[i], err = applyChild(pattern); err != nil {
			return image, err
		}
		held, err := m.NewRef(patterns[i])
		if err != nil {
			return image, err
		}
		pats = append(pats, held)
	}
	numNoPatterns, err := m.GetNumNoPatterns(q)
	if err != nil {
		return image, err
	}
	noPatterns := make([]ast.ASTHandle, numNoPatterns)
	for i := range noPatterns {
		pattern, err := m.GetNoPattern(q, uint32(i))
		if err != nil {
			return image, err
		}
		if noPatterns[i], err = applyChild(pattern); err != nil {
			return image, err
		}
		held, err := m.NewRef(noPatterns[i])
		if err != nil {
			return image, err
		}
		noPats = append(noPats, held)
	}
	newBody, err := applyChild(body)
	if err != nil {
		return image, err
	}
	// er = update_quantifier(q, pats, no_pats, er), then m_new_exprs.
	updated, err := m.UpdateQuantifierAnnotations(q, patterns, noPatterns, newBody)
	if err != nil {
		return image, err
	}
	if err = m.SetRef(er, updated); err != nil {
		return image, err
	}
	return s.keep(updated)
}

func (s *Substitution) Acyclic() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	colors := make(map[ExprOffset]uint8)
	var visit func(ExprOffset) (bool, error)
	visit = func(p ExprOffset) (bool, error) {
		if colors[p] == 2 {
			return true, nil
		}
		if colors[p] == 1 {
			return false, nil
		}
		colors[p] = 1
		kind, e := s.manager.GetKind(p.expr)
		if e != nil {
			return false, e
		}
		children := []ExprOffset{}
		switch kind {
		case ast.ASTVar:
			i, e := s.manager.GetVarIndex(p.expr)
			if e != nil {
				return false, e
			}
			if q, ok, e := s.bindings.find(i, p.offset); e != nil {
				return false, e
			} else if ok && q != p {
				children = append(children, q)
			}
		case ast.ASTApp:
			n, e := s.manager.GetNumArgs(p.expr)
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < n; i++ {
				h, e := s.manager.GetArg(p.expr, i)
				if e != nil {
					return false, e
				}
				children = append(children, NewExprOffset(h, p.offset))
			}
		default:
			return false, ast.ErrASTDomain
		}
		for _, c := range children {
			ok, e := visit(c)
			if e != nil || !ok {
				return ok, e
			}
		}
		colors[p] = 2
		return true, nil
	}
	for _, v := range s.order {
		r, ok, e := s.bindings.find(v.Variable, v.Offset)
		if e != nil {
			return false, e
		}
		if ok {
			yes, e := visit(r)
			if e != nil || !yes {
				return yes, e
			}
		}
	}
	return true, nil
}

func (s *Substitution) format(h ast.ASTHandle) (value string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	doc, err := s.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return "", err
	}
	if err = s.manager.PrintFormatTransient(out, doc, nil, nil, nil); err != nil {
		return "", err
	}
	return out.String()
}

// Display returns the source normalized diagnostic form.
func (s *Substitution) Display(offsets uint32, deltas []uint32) (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	if uint64(offsets) > uint64(len(deltas)) {
		return "", ast.ErrASTDomain
	}
	if err := s.ResetCache(); err != nil {
		return "", err
	}
	var out bytes.Buffer
	ref, err := s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return "", err
	}
	defer s.manager.DelRef(&ref)
	for i := uint32(0); i < offsets; i++ {
		for j := uint32(0); j < s.bindings.variables; j++ {
			r, ok, e := s.bindings.find(j, i)
			if e != nil {
				return "", e
			}
			if !ok {
				continue
			}
			if e = s.Apply(offsets, deltas, r, ref); e != nil {
				return "", e
			}
			h, e := s.manager.RefGet(ref)
			if e != nil {
				return "", e
			}
			text, e := s.format(h)
			if e != nil {
				return "", e
			}
			out.WriteString("VAR ")
			out.WriteString(strconv.FormatUint(uint64(j), 10))
			out.WriteByte(':')
			out.WriteString(strconv.FormatUint(uint64(i), 10))
			out.WriteString(" -->\n")
			out.WriteString(text)
			out.WriteByte('\n')
		}
	}
	return out.String(), nil
}

// DisplayRaw returns the source diagnostic form without normalization.
func (s *Substitution) DisplayRaw() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	var out bytes.Buffer
	for i := uint32(0); i < s.bindings.offsets; i++ {
		for j := uint32(0); j < s.bindings.variables; j++ {
			r, ok, e := s.bindings.find(j, i)
			if e != nil {
				return "", e
			}
			if !ok {
				continue
			}
			text, e := s.format(r.expr)
			if e != nil {
				return "", e
			}
			out.WriteString("VAR ")
			out.WriteString(strconv.FormatUint(uint64(j), 10))
			out.WriteByte(':')
			out.WriteString(strconv.FormatUint(uint64(i), 10))
			out.WriteString(" --> ")
			out.WriteString(strconv.FormatUint(uint64(r.offset), 10))
			out.WriteByte('\n')
			out.WriteString(text)
			out.WriteByte('\n')
		}
	}
	return out.String(), nil
}

func (s *Substitution) Close() error {
	if s == nil || s.self != s || s.manager == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	// The destructor runs members in reverse declaration order:
	// m_new_exprs (the apply cache) before m_refs.
	if err := s.ResetCache(); err != nil {
		return err
	}
	if err := s.Reset(); err != nil {
		return err
	}
	s.closed = true
	return nil
}
