package qe

// Source: src/qe/qe.cpp class nnf_normalize_literals.

import "github.com/glycerine/g3/pkg/ast"

// normalizeLiterals is nnf_normalize_literals::operator(). Rebuilt
// connectives are plain mk_app results owned by their parents; mk_atom
// results go to the literal trail, released by the caller after the formula
// took the result.
func (n *QENNFNormalizer) normalizeLiterals(expression ast.ASTHandle) (ast.ASTHandle, error) {
	m := n.manager
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	todo := []ast.ASTHandle{expression}
	for len(todo) > 0 {
		e := todo[len(todo)-1]
		if _, found := cache[e]; found {
			todo = todo[:len(todo)-1]
			continue
		}
		kind, err := m.GetKind(e)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if kind != ast.ASTApp {
			todo = todo[:len(todo)-1]
			cache[e] = e
			continue
		}
		relevant, err := n.relevance.IsRelevant(e)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !relevant {
			cache[e] = e
			todo = todo[:len(todo)-1]
			continue
		}
		family, err := m.GetFamilyID(e)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		operation, err := m.GetDeclKind(e)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		basic := family == m.BasicFamilyID()
		if basic && (operation == ast.ASTOpAnd || operation == ast.ASTOpOr) {
			arguments, err := n.args(e)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			values := make([]ast.ASTHandle, 0, len(arguments))
			all := true
			for _, argument := range arguments {
				if f, found := cache[argument]; found {
					values = append(values, f)
				} else {
					all = false
					todo = append(todo, argument)
				}
			}
			if all {
				decl, err := m.GetDecl(e)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				rebuilt, err := m.MkApp(decl, values)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				cache[e] = rebuilt
				todo = todo[:len(todo)-1]
			}
			continue
		}
		var value ast.ASTHandle
		if basic && operation == ast.ASTOpNot {
			atom, err := m.GetArg(e, 0)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			value, err = n.normalizeAtom(atom, false)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		} else {
			value, err = n.normalizeAtom(e, true)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		cache[e] = value
		todo = todo[:len(todo)-1]
	}
	return cache[expression], nil
}

// normalizeAtom calls mk_atom and pushes its result on the literal trail.
func (n *QENNFNormalizer) normalizeAtom(atom ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	if owner, ok := n.atoms.(qeNNFAtomRefNormalizer); ok {
		ref, err := owner.NormalizeAtomRef(atom, positive)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		n.literalTrail = append(n.literalTrail, ref)
		return n.manager.RefGet(ref)
	}
	result, err := n.atoms.NormalizeAtom(atom, positive)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := n.manager.NewRef(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	n.literalTrail = append(n.literalTrail, ref)
	return result, nil
}
