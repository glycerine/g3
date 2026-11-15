package rewriter

// Source: src/ast/rewriter/expr_safe_replace.cpp;
// src/ast/rewriter/expr_safe_replace.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type safeReplacement struct {
	source, destination ast.ASTHandle
}

// ExprSafeReplace performs simultaneous expression replacement and shifts
// substitutions through quantifier binders.
type ExprSafeReplace struct {
	self         *ExprSafeReplace
	manager      *ast.ASTManager
	sourceRefs   []ast.ASTRef
	destRefs     []ast.ASTRef
	replacements []safeReplacement
	substitution map[ast.ASTHandle]ast.ASTHandle
	temporary    []ast.ASTRef
	closed       bool
}

func NewExprSafeReplace(m *ast.ASTManager) (*ExprSafeReplace, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	r := &ExprSafeReplace{manager: m, substitution: make(map[ast.ASTHandle]ast.ASTHandle)}
	r.self = r
	return r, nil
}

func (r *ExprSafeReplace) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (r *ExprSafeReplace) Insert(source, destination ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	sourceSort, err := r.manager.GetSort(source)
	if err != nil {
		return err
	}
	destinationSort, err := r.manager.GetSort(destination)
	if err != nil {
		return err
	}
	if sourceSort != destinationSort {
		return ast.ErrASTDomain
	}
	sourceRef, err := r.manager.NewRef(source)
	if err != nil {
		return err
	}
	r.sourceRefs = append(r.sourceRefs, sourceRef)
	destinationRef, err := r.manager.NewRef(destination)
	if err != nil {
		return err
	}
	r.destRefs = append(r.destRefs, destinationRef)
	r.replacements = append(r.replacements, safeReplacement{source: source, destination: destination})
	r.substitution[source] = destination
	return nil
}

func (r *ExprSafeReplace) Empty() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return len(r.substitution) == 0, nil
}

// clearTemporary is m_refs.reset(): front to back.
func (r *ExprSafeReplace) clearTemporary() error {
	return releaseRefsForward(r.manager, &r.temporary)
}

func (r *ExprSafeReplace) scratch() (ast.ASTRef, error) {
	ref, err := r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	r.temporary = append(r.temporary, ref)
	return ref, nil
}

func (r *ExprSafeReplace) retainTemporary(h ast.ASTHandle) error {
	ref, err := r.manager.NewRef(h)
	if err != nil {
		return err
	}
	r.temporary = append(r.temporary, ref)
	return nil
}

// rewriteQuantifier is the quantifier branch of operator(). Its locals
// (replace, shift, new_body, src, dst, tmp, pats, nopats) die when the
// branch ends, in reverse declaration order, after b joins m_refs.
func (r *ExprSafeReplace) rewriteQuantifier(q ast.ASTHandle) (result ast.ASTHandle, err error) {
	m := r.manager
	nested, err := NewExprSafeReplace(m)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, nested.Close()) }()
	shifter, err := newVarShifter(m)
	if shifter != nil {
		defer func() { err = errors.Join(err, shifter.Close()) }()
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// expr_ref new_body, src, dst, tmp; expr_ref_vector pats, nopats.
	var newBody, src, dst, tmp ast.ASTRef
	var pats, nopats []ast.ASTRef
	defer func() {
		err = errors.Join(err,
			releaseRefsForward(m, &nopats),
			releaseRefsForward(m, &pats),
			m.DelRef(&tmp), m.DelRef(&dst), m.DelRef(&src), m.DelRef(&newBody))
	}()
	for _, ref := range []*ast.ASTRef{&newBody, &src, &dst, &tmp} {
		if *ref, err = m.NewRef(ast.ASTHandle{}); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	declarations, err := m.GetNumDeclarations(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, replacement := range r.replacements {
		if err = shifter.ShiftBy(replacement.source, declarations, src); err != nil {
			return ast.ASTHandle{}, err
		}
		if err = shifter.ShiftBy(replacement.destination, declarations, dst); err != nil {
			return ast.ASTHandle{}, err
		}
		shiftedSource, e := m.RefGet(src)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		shiftedDestination, e := m.RefGet(dst)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if err = nested.Insert(shiftedSource, shiftedDestination); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	// replace(child, tmp); vector.push_back(tmp).
	rewriteInto := func(h ast.ASTHandle, vector *[]ast.ASTRef) (ast.ASTHandle, error) {
		if e := nested.Apply(h, tmp); e != nil {
			return ast.ASTHandle{}, e
		}
		value, e := m.RefGet(tmp)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		held, e := m.NewRef(value)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		*vector = append(*vector, held)
		return value, nil
	}
	patternCount, err := m.GetNumPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	patterns := make([]ast.ASTHandle, 0, patternCount)
	for i := uint32(0); i < patternCount; i++ {
		pattern, e := m.GetPattern(q, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if pattern, e = rewriteInto(pattern, &pats); e != nil {
			return ast.ASTHandle{}, e
		}
		patterns = append(patterns, pattern)
	}
	noPatternCount, err := m.GetNumNoPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	noPatterns := make([]ast.ASTHandle, 0, noPatternCount)
	for i := uint32(0); i < noPatternCount; i++ {
		noPattern, e := m.GetNoPattern(q, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if noPattern, e = rewriteInto(noPattern, &nopats); e != nil {
			return ast.ASTHandle{}, e
		}
		noPatterns = append(noPatterns, noPattern)
	}
	body, err := m.GetQuantifierBody(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = nested.Apply(body, newBody); err != nil {
		return ast.ASTHandle{}, err
	}
	if body, err = m.RefGet(newBody); err != nil {
		return ast.ASTHandle{}, err
	}
	b, err := m.UpdateQuantifierAnnotations(q, patterns, noPatterns, body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// m_refs.push_back(b) before the locals are destroyed.
	if err = r.retainTemporary(b); err != nil {
		return ast.ASTHandle{}, err
	}
	return b, nil
}

func (r *ExprSafeReplace) Apply(root ast.ASTHandle, out ast.ASTRef) (result error) {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.clearTemporary(); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.clearTemporary()) }()
	if _, err := r.manager.GetKind(root); err != nil {
		return err
	}
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	todo := []ast.ASTHandle{root}
	args := make([]ast.ASTHandle, 0)
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		if _, found := cache[current]; found {
			todo = todo[:last]
			continue
		}
		if replacement, found := r.substitution[current]; found {
			cache[current] = replacement
			todo = todo[:last]
			continue
		}
		kind, err := r.manager.GetKind(current)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTVar:
			cache[current] = current
			todo = todo[:last]
		case ast.ASTApp:
			n, e := r.manager.GetNumArgs(current)
			if e != nil {
				return e
			}
			args = args[:0]
			changed := false
			for i := uint32(0); i < n; i++ {
				child, childErr := r.manager.GetArg(current, i)
				if childErr != nil {
					return childErr
				}
				rewritten, found := cache[child]
				if !found {
					todo = append(todo, child)
					continue
				}
				if ast.ASTSourceDebug {
					left, sortErr := r.manager.GetSort(child)
					if sortErr != nil {
						return sortErr
					}
					right, sortErr := r.manager.GetSort(rewritten)
					if sortErr != nil {
						return sortErr
					}
					if left != right {
						return ast.ErrASTDomain
					}
				}
				args = append(args, rewritten)
				changed = changed || child != rewritten
			}
			if uint32(len(args)) != n {
				continue
			}
			rewritten := current
			if changed {
				decl, e := r.manager.GetDecl(current)
				if e != nil {
					return e
				}
				// Reserve the owning entry before constructing the replacement;
				// a failed reference allocation must not strand the fresh app.
				root, e := r.scratch()
				if e != nil {
					return e
				}
				rewritten, e = r.manager.MkApp(decl, args)
				if !rewritten.IsNull() {
					e = errors.Join(e, r.manager.SetRef(root, rewritten))
				}
				if e != nil {
					return e
				}
			}
			cache[current] = rewritten
			todo = todo[:last]
		case ast.ASTQuantifier:
			rewritten, e := r.rewriteQuantifier(current)
			if e != nil {
				return e
			}
			cache[current] = rewritten
			todo = todo[:last]
		default:
			return ast.ErrASTDomain
		}
	}
	rewritten, found := cache[root]
	if !found {
		return ast.ErrASTDomain
	}
	return r.manager.SetRef(out, rewritten)
}

func (r *ExprSafeReplace) ApplyRef(value ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	h, err := r.manager.RefGet(value)
	if err != nil {
		return err
	}
	return r.Apply(h, value)
}

func (r *ExprSafeReplace) ApplyRefSlice(values []ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	for _, value := range values {
		if err := r.ApplyRef(value); err != nil {
			return err
		}
	}
	return nil
}

func (r *ExprSafeReplace) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.clearTemporary(); err != nil {
		return err
	}
	// reset(): m_src.reset(); m_dst.reset(); each front to back.
	if err := releaseRefsForward(r.manager, &r.sourceRefs); err != nil {
		return err
	}
	if err := releaseRefsForward(r.manager, &r.destRefs); err != nil {
		return err
	}
	r.replacements = nil
	r.substitution = make(map[ast.ASTHandle]ast.ASTHandle)
	return nil
}

// releaseRefsForward is ref_vector::reset: front to back, keeping the
// unreleased suffix on an error.
func releaseRefsForward(m *ast.ASTManager, refs *[]ast.ASTRef) error {
	for len(*refs) != 0 {
		err := m.DelRef(&(*refs)[0])
		if (*refs)[0] == (ast.ASTRef{}) {
			*refs = (*refs)[1:]
		}
		if err != nil {
			return err
		}
	}
	*refs = nil
	return nil
}

func (r *ExprSafeReplace) ApplySubstitution(source, definition ast.ASTHandle, target ast.ASTRef) (result error) {
	if err := r.Reset(); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.Reset()) }()
	if err := r.Insert(source, definition); err != nil {
		return err
	}
	return r.ApplyRef(target)
}

func (r *ExprSafeReplace) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	// ~expr_safe_replace destroys m_refs, then m_dst, then m_src, each
	// front to back.
	if err := r.clearTemporary(); err != nil {
		return err
	}
	if err := releaseRefsForward(r.manager, &r.destRefs); err != nil {
		return err
	}
	if err := releaseRefsForward(r.manager, &r.sourceRefs); err != nil {
		return err
	}
	r.replacements = nil
	r.substitution = nil
	r.closed = true
	return nil
}
