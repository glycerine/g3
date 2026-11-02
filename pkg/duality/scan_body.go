package duality

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

// ScannedBody owns the rewritten formula and positional relation declarations.
// Children are borrowed from the source graph in the same order as Parameters.
type ScannedBody struct {
	Term       *Expr
	Parameters []*FuncDecl
	Children   []*Node
}

func (s *ScannedBody) Close() error {
	if s == nil {
		return nil
	}
	var result error
	if s.Term != nil {
		if err := s.Term.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.Term = nil
		}
	}
	for i, p := range s.Parameters {
		if p == nil {
			continue
		}
		if err := p.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.Parameters[i] = nil
		}
	}
	return result
}

// ScanBody replaces each distinct predicate application by a positional
// relation parameter. Shared applications share one parameter; different
// applications of the same predicate produce repeated child positions.
// Quantifier patterns are left untouched, as in the source FromClauses path.
// Source: duality_rpfp.cpp, ScanBody and SuffixFuncDecl.
func (g *RPFP) ScanBody(term *Expr, predicates map[ast.ASTHandle]*Node) (out *ScannedBody, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := term.open(); err != nil {
		return nil, err
	}
	if term.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	for _, node := range predicates {
		if node == nil || node.closed || node.Owner != g {
			return nil, ErrWrapperOwner
		}
	}
	out = &ScannedBody{}
	m := g.context.manager
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			result = errors.Join(result, m.DelRef(&pins[i]))
		}
		if result != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var visit func(ast.ASTHandle) (ast.ASTHandle, error)
	visit = func(h ast.ASTHandle) (ast.ASTHandle, error) {
		if v, ok := memo[h]; ok {
			return v, nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		changed := h
		switch kind {
		case ast.ASTApp:
			decl, err := m.GetDecl(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			n, err := m.GetNumArgs(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if node, ok := predicates[decl]; ok {
				name, err := m.GetASTName(decl)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				symbol, err := newSymbol(g.context, name)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				text, err := symbol.String()
				if err != nil {
					return ast.ASTHandle{}, err
				}
				suffix, err := m.Symbols.Intern(text + "_" + strconv.Itoa(len(out.Parameters)))
				if err != nil {
					return ast.ASTHandle{}, err
				}
				domain := make([]ast.ASTHandle, n)
				for i := range domain {
					arg, err := m.GetArg(h, uint32(i))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					domain[i], err = m.GetSort(arg)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
				sort, err := m.GetSort(h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				decl, err = m.MkFuncDecl(suffix, domain, sort, ast.ASTFuncDeclInfo{})
				if err != nil {
					return ast.ASTHandle{}, err
				}
				parameter, err := NewFuncDecl(g.context, decl)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				out.Parameters = append(out.Parameters, parameter)
				out.Children = append(out.Children, node)
			}
			args := make([]ast.ASTHandle, n)
			for i := range args {
				arg, err := m.GetArg(h, uint32(i))
				if err != nil {
					return ast.ASTHandle{}, err
				}
				args[i], err = visit(arg)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			changed, err = m.MkApp(decl, args)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			body, err = visit(body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			changed, err = m.UpdateQuantifierBody(h, body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		ref, err := m.NewRef(changed)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		pins = append(pins, ref)
		memo[h] = changed
		return changed, nil
	}
	root, err := term.Raw()
	if err != nil {
		return out, err
	}
	h, err := visit(root)
	if err != nil {
		return out, err
	}
	out.Term, err = NewExpr(g.context, h)
	return out, err
}
