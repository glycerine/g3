package order

import (
	"slices"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/control"
)

// Add distinct constants related by positive or negative binary facts on one
// uninterpreted sort. A transition can constrain a witness absent from the goal.
func (p *GroundInstancePool) relationWitnesses(key [8]ast.ASTHandle, n int, limit *control.Reslimit) ([8]ast.ASTHandle, int, error) {
	m := p.manager
	for _, fact := range p.facts {
		if n == len(key) {
			break
		}
		if !limit.Inc() {
			return key, n, errBitLimit
		}
		h, err := m.NodeHeader(fact)
		if err != nil {
			return key, n, err
		}
		if h.Family == m.BasicFamilyID() && h.DeclKind == ast.ASTOpNot && h.NumArgs == 1 {
			fact, err = m.GetArg(fact, 0)
			if err != nil {
				return key, n, err
			}
			h, err = m.NodeHeader(fact)
			if err != nil {
				return key, n, err
			}
		}
		if h.Kind != ast.ASTApp || h.Family != ast.NullFamilyID || h.NumArgs != 2 {
			continue
		}
		var args [2]ast.ASTHandle
		var sorts [2]ast.ASTHandle
		for i := range args {
			args[i], err = m.GetArg(fact, uint32(i))
			if err != nil {
				return key, n, err
			}
			hd, err := m.NodeHeader(args[i])
			if err != nil {
				return key, n, err
			}
			if hd.Kind != ast.ASTApp || hd.Family != ast.NullFamilyID || hd.NumArgs != 0 {
				break
			}
			sorts[i], err = m.GetSort(args[i])
			if err != nil {
				return key, n, err
			}
		}
		if sorts[0].IsNull() || sorts[0] != sorts[1] || args[0] == args[1] {
			continue
		}
		ok, err := m.IsUninterpretedSort(sorts[0])
		if err != nil {
			return key, n, err
		}
		if !ok {
			continue
		}
		for _, arg := range args {
			if n < len(key) && !slices.Contains(key[:n], arg) {
				key[n] = arg
				n++
			}
		}
	}
	ast.SortHandles(key[:n])
	return key, n, nil
}
