package normal_forms

// Source: src/ast/normal_forms/nnf.cpp:69-195.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Skolemizer eliminates one quantifier using fresh functions over its free
// variables. Results and optional proofs remain in its act_caches m_cache
// and m_cache_pr until Close.
type Skolemizer struct {
	self          *Skolemizer
	manager       *ast.ASTManager
	skHackName    usymbol.Symbol
	skHackEnabled bool
	cache         *ast.ActivityCache // m_cache
	cachePr       *ast.ActivityCache // m_cache_pr
	closed        bool
}

func NewSkolemizer(m *ast.ASTManager) (*Skolemizer, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	name, e := m.Symbols.Intern("sk_hack")
	if e != nil {
		return nil, e
	}
	cache, e := ast.NewActivityCache(m, nil)
	if e != nil {
		return nil, e
	}
	cachePr, e := ast.NewActivityCache(m, nil)
	if e != nil {
		return nil, errors.Join(e, cache.Close())
	}
	s := &Skolemizer{manager: m, skHackName: name, cache: cache, cachePr: cachePr}
	s.self = s
	return s, nil
}

func (s *Skolemizer) open() error {
	if s == nil || s.self != s || s.manager == nil || s.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *Skolemizer) SetSkolemHack(enabled bool) error {
	if e := s.open(); e != nil {
		return e
	}
	s.skHackEnabled = enabled
	return nil
}

// IsSkolemHack recognizes the source's single-term pattern whose declaration
// is named sk_hack, has arity one, and returns Boolean.
func (s *Skolemizer) IsSkolemHack(pattern ast.ASTHandle) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	m := s.manager
	yes, e := m.IsPattern(pattern)
	if e != nil || !yes {
		if e == nil && ast.ASTSourceDebug {
			return false, ast.ErrASTDomain
		}
		return false, e
	}
	n, e := m.GetNumArgs(pattern)
	if e != nil || n != 1 {
		return false, e
	}
	body, e := m.GetArg(pattern, 0)
	if e != nil {
		return false, e
	}
	if kind, x := m.GetKind(body); x != nil || kind != ast.ASTApp {
		return false, x
	}
	decl, e := m.GetDecl(body)
	if e != nil {
		return false, e
	}
	name, e := m.GetASTName(decl)
	if e != nil || name != s.skHackName {
		return false, e
	}
	if arity, x := m.GetArity(decl); x != nil || arity != 1 {
		return false, x
	}
	sort, e := m.GetSort(body)
	if e != nil {
		return false, e
	}
	boolSort, e := m.MkBoolSort()
	if e != nil {
		return false, e
	}
	if sort != boolSort {
		_ = diagnostic.BorrowProcessWarningService().Warning("sk_hack constant must return a Boolean")
		return false, nil
	}
	return true, nil
}

// Apply publishes the cached or newly-created skolemized body and proof.
func (s *Skolemizer) Apply(q ast.ASTHandle, out, proofOut ast.ASTRef) (result error) {
	if e := s.open(); e != nil {
		return e
	}
	m := s.manager
	// r = m_cache.find(q)
	if value, e := s.cache.Find(q); e != nil {
		return e
	} else if !value.IsNull() {
		if e = m.SetRef(out, value); e != nil {
			return e
		}
		proofs, e := m.ProofsEnabled()
		if e != nil || !proofs {
			return e
		}
		proof, e := s.cachePr.Find(q)
		if e != nil {
			return e
		}
		return m.SetRef(proofOut, proof)
	}
	if kind, e := m.GetKind(q); e != nil || kind != ast.ASTQuantifier {
		if e != nil {
			return e
		}
		return ast.ErrASTDomain
	}

	var held []ast.ASTRef
	hold := func(h ast.ASTHandle) error {
		if h.IsNull() {
			return nil
		}
		r, e := m.NewRef(h)
		if e != nil {
			return e
		}
		held = append(held, r)
		return nil
	}
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&held[i]))
		}
	}()

	used, e := ast.NewASTUsedVars(m)
	if e != nil {
		return e
	}
	if e = used.Collect(q); e != nil {
		return e
	}
	count := used.MaxFoundVarIndexPlusOne()
	sorts := make([]ast.ASTHandle, 0, used.NumVars())
	args := make([]ast.ASTHandle, 0, used.NumVars())
	for i := uint32(0); i < count; i++ {
		sort := used.Contains(i)
		if sort.IsNull() {
			continue
		}
		sorts = append(sorts, sort)
		v, x := m.MkVar(i, sort)
		if x != nil {
			return x
		}
		if x = hold(v); x != nil {
			return x
		}
		args = append(args, v)
	}
	_, skid, e := m.GetQuantifierIDs(q)
	if e != nil {
		return e
	}
	declarations, e := m.GetNumDeclarations(q)
	if e != nil {
		return e
	}
	substitution := make([]ast.ASTHandle, 0, declarations+count)
	for i := declarations; i > 0; {
		i--
		rangeSort, x := m.GetDeclarationSort(q, i)
		if x != nil {
			return x
		}
		name, x := m.GetDeclarationName(q, i)
		if x != nil {
			return x
		}
		decl, x := m.MkFreshFuncDecl(name, skid, sorts, rangeSort)
		if x != nil {
			return x
		}
		if x = hold(decl); x != nil {
			return x
		}
		skolem, x := m.MkApp(decl, args)
		if x != nil {
			return x
		}
		if x = hold(skolem); x != nil {
			return x
		}
		substitution = append(substitution, skolem)
	}
	for i := uint32(0); i < count; i++ {
		sort := used.Contains(i)
		if sort.IsNull() {
			substitution = append(substitution, ast.ASTHandle{})
			continue
		}
		v, x := m.MkVar(i, sort)
		if x != nil {
			return x
		}
		if x = hold(v); x != nil {
			return x
		}
		substitution = append(substitution, v)
	}
	for i, j := 0, len(substitution)-1; i < j; i, j = i+1, j-1 {
		substitution[i], substitution[j] = substitution[j], substitution[i]
	}
	body, e := m.GetQuantifierBody(q)
	if e != nil {
		return e
	}
	if s.skHackEnabled {
		patterns, x := m.GetNumPatterns(q)
		if x != nil {
			return x
		}
		for i := uint32(0); i < patterns; i++ {
			pattern, x := m.GetPattern(q, i)
			if x != nil {
				return x
			}
			hack, x := s.IsSkolemHack(pattern)
			if x != nil {
				return x
			}
			if !hack {
				continue
			}
			hackExpr, x := m.GetArg(pattern, 0)
			if x != nil {
				return x
			}
			forall, x := m.IsForall(q)
			if x != nil {
				return x
			}
			if forall {
				negated, x := m.MkNot(hackExpr)
				if x != nil {
					return x
				}
				if x = hold(negated); x != nil {
					return x
				}
				body, x = m.MkOr(body, negated)
				if x != nil {
					return x
				}
			} else {
				body, x = m.MkAnd(body, hackExpr)
				if x != nil {
					return x
				}
			}
			if x = hold(body); x != nil {
				return x
			}
		}
	}
	subst, e := rewriter.NewVarSubstitution(m, true)
	if e != nil {
		if subst != nil {
			return errors.Join(e, subst.Close())
		}
		return e
	}
	defer func() { result = errors.Join(result, subst.Close()) }()
	tmp, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, m.DelRef(&tmp)) }()
	if e = subst.Substitute(body, substitution, tmp); e != nil {
		return e
	}
	value, e := m.RefGet(tmp)
	if e != nil {
		return e
	}
	proofs, e := m.ProofsEnabled()
	if e != nil {
		return e
	}
	var proof ast.ASTHandle
	if proofs {
		forall, x := m.IsForall(q)
		if x != nil {
			return x
		}
		left, right := q, value
		if forall {
			// mk_skolemization(mk_not(q), mk_not(r)): GCC evaluates the
			// arguments right to left, so (not r) is built first.
			right, x = m.MkNot(value)
			if x != nil {
				return x
			}
			if x = hold(right); x != nil {
				return x
			}
			left, x = m.MkNot(q)
			if x != nil {
				return x
			}
			if x = hold(left); x != nil {
				return x
			}
		}
		proof, e = m.MkProofSkolemization(left, right)
		if e != nil {
			return e
		}
		if e = hold(proof); e != nil {
			return e
		}
	}
	if e = s.cache.Insert(q, value); e != nil {
		return e
	}
	if proofs {
		if e = s.cachePr.Insert(q, proof); e != nil {
			return e
		}
	}
	if e = m.SetRef(out, value); e != nil {
		return e
	}
	if proofs {
		return m.SetRef(proofOut, proof)
	}
	return nil
}

// Reset is act_cache::reset of m_cache and m_cache_pr.
func (s *Skolemizer) Reset() error {
	if e := s.open(); e != nil {
		return e
	}
	return errors.Join(s.cache.Reset(), s.cachePr.Reset())
}

// Close is ~skolemizer: m_cache_pr, then m_cache.
func (s *Skolemizer) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if e := s.cachePr.Close(); e != nil {
		return e
	}
	if e := s.cache.Close(); e != nil {
		return e
	}
	s.closed = true
	return nil
}
