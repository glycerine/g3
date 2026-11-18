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
	// memo holds the ScanBody memo's references (FromClauses' scan_memo,
	// a hash_space::hash_map<ast,Term>): the source keeps them until the end
	// of the FromClauses loop iteration.
	memo *iz3Table
	m    *ast.ASTManager
}

func (s *ScannedBody) Close() error {
	if s == nil {
		return nil
	}
	var result error
	// The source destroys labeled (Term), then scan_memo, then Relparams.
	if s.Term != nil {
		if err := s.Term.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			s.Term = nil
		}
	}
	if s.memo != nil {
		if err := s.memo.clear(s.m); err != nil {
			return errors.Join(result, err)
		}
		s.memo = nil
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
	m := g.context.manager
	out = &ScannedBody{memo: newIZ3Table(), m: m}
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
		key := h
		id, err := m.GetASTID(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if out.memo.find(uint64(id)) != nil {
			// memo[t] on a hit is still an inserting lookup.
			out.memo.resize()
		}
		if v, ok := memo[key]; ok {
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
		if entry, inserted := out.memo.lookup(uint64(id)); !inserted {
			pins = append(pins, ref)
		} else {
			// memo[t] = res: the pair holds t and then res.
			entry.value = ref
			entry.key, err = m.NewRef(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		memo[key] = changed
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
