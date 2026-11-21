package smtcore

import "github.com/glycerine/g3/pkg/ast"

// smtCandidateBody is immutable syntax for repeated qi_queue checks of one
// registered quantifier. Its AST owns the nodes borrowed here. Solver state
// and the per-candidate truth and enode caches belong to the checker.
type smtCandidateBody struct {
	body         ast.ASTHandle
	nodes        []smtCandidateNode
	arguments    []uint16
	declarations []smtCandidateDecl
	memoSize     int
}

type smtCandidateDecl struct {
	handle      ast.ASTHandle
	commutative bool
}

type smtCandidateOp uint8

const (
	candidateOpaque smtCandidateOp = iota
	candidateVariable
	candidateApplication
	candidateTrue
	candidateFalse
	candidateNot
	candidateAnd
	candidateOr
	candidateIff
	candidateIte
	candidateEqual
)

type smtCandidateNode struct {
	expression ast.ASTHandle
	operand    uint32 // de Bruijn index or index into declarations
	first, end uint16 // half-open range in arguments
	memo       uint16 // one plus the memo slot, zero when a second visit is impossible
	op         smtCandidateOp
}

func prepareCandidateBody(m *ast.ASTManager, body ast.ASTHandle, size uint32) *smtCandidateBody {
	// Larger bodies keep the ordinary lazy checker. The bounds also limit
	// compile time and storage per registration when many solvers are live.
	const maxNodes, maxArguments, maxDepth = 256, 1024, 128
	if size > maxNodes {
		return nil
	}
	p := &smtCandidateBody{body: body, nodes: make([]smtCandidateNode, 0, size), arguments: make([]uint16, 0, size)}
	ids := make(map[ast.ASTHandle]uint16)
	decls := make(map[ast.ASTHandle]uint32)
	var repeated []uint16
	var visit func(ast.ASTHandle, int) (uint16, bool)
	visit = func(h ast.ASTHandle, depth int) (uint16, bool) {
		if id, found := ids[h]; found {
			repeated = append(repeated, id)
			return id, true
		}
		if len(p.nodes) >= maxNodes || depth >= maxDepth {
			return 0, false
		}
		header, err := m.NodeHeader(h)
		if err != nil || header.NumArgs > maxNodes || int(header.NumArgs)+len(p.arguments) > maxArguments {
			return 0, false
		}
		node := smtCandidateNode{expression: h, first: uint16(len(p.arguments))}
		node.end = node.first + uint16(header.NumArgs)
		switch header.Kind {
		case ast.ASTVar:
			node.op = candidateVariable
			node.operand, err = m.GetVarIndex(h)
			if err != nil {
				return 0, false
			}
		case ast.ASTApp:
			node.op = candidateApplication
			if header.Family == m.BasicFamilyID() {
				switch header.DeclKind {
				case ast.ASTOpTrue:
					node.op = candidateTrue
				case ast.ASTOpFalse:
					// get_assignment treats only the manager's false specially.
					if !m.IsFalse(h) {
						return 0, false
					}
					node.op = candidateFalse
				case ast.ASTOpNot:
					if header.NumArgs != 1 {
						return 0, false
					}
					node.op = candidateNot
				case ast.ASTOpAnd:
					node.op = candidateAnd
				case ast.ASTOpOr:
					node.op = candidateOr
				case ast.ASTOpIff:
					if header.NumArgs != 2 {
						return 0, false
					}
					node.op = candidateIff
				case ast.ASTOpIte:
					if header.NumArgs != 3 {
						return 0, false
					}
					node.op = candidateIte
				case ast.ASTOpEq:
					if header.NumArgs != 2 {
						return 0, false
					}
					node.op = candidateEqual
				}
			}
			if header.NumArgs != 0 {
				id, found := decls[header.Decl]
				if !found {
					flags, err := m.GetFuncDeclFlags(header.Decl)
					if err != nil {
						return 0, false
					}
					id = uint32(len(p.declarations))
					decls[header.Decl] = id
					p.declarations = append(p.declarations, smtCandidateDecl{handle: header.Decl, commutative: flags.Commutative})
				}
				node.operand = id
			}
		}
		id := uint16(len(p.nodes))
		ids[h] = id
		p.nodes = append(p.nodes, node)

		p.arguments = append(p.arguments, make([]uint16, header.NumArgs)...)
		for i := uint32(0); i < header.NumArgs; i++ {
			child, err := m.GetArg(h, i)
			if err != nil {
				return 0, false
			}
			arg, ok := visit(child, depth+1)
			if !ok {
				return 0, false
			}
			p.arguments[int(node.first)+int(i)] = arg
		}
		if node.op == candidateIff {
			repeated = append(repeated, p.arguments[node.first:node.end]...)
		}
		return id, true
	}
	if _, ok := visit(body, 0); !ok {
		return nil
	}

	// Multiple syntax edges and IFF's two polarity probes can revisit a
	// subexpression. Assign memo slots to just those nodes and their children.
	// The current AST reference count still determines whether each visit caches.
	var mark func(uint16)
	mark = func(id uint16) {
		n := &p.nodes[id]
		if n.memo != 0 {
			return
		}
		p.memoSize++
		n.memo = uint16(p.memoSize)
		for _, child := range p.arguments[n.first:n.end] {
			mark(child)
		}
	}
	for _, id := range repeated {
		mark(id)
	}

	return p
}
