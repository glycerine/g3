package smtcore

// Source: src/smt/smt_enode.cpp context-independent deletion, diagnostics,
// invariant checks, and tmp_enode implementation.

import (
	"fmt"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

func (n *SMTEnode) Delete(updateChildrenParent bool) error {
	return n.delete(updateChildrenParent, true)
}

// deleteAfterEqualityUndo retires a node popped with its scope. Undoing a merge
// older than the node has already cut the node from that root's parent vector;
// the source avoids this by interleaving node creation on one undo trail.
func (n *SMTEnode) deleteAfterEqualityUndo() error {
	return n.delete(true, false)
}

func (n *SMTEnode) delete(updateChildrenParent, requireParentEntry bool) error {
	if err := n.valid(); err != nil {
		return err
	}
	if n.classSize != 1 || n.root != n || n.next != n {
		return fmt.Errorf("delete non-singleton enode (class %d root-self %t next-self %t): %w", n.classSize, n.root == n, n.next == n, ast.ErrASTDomain)
	}
	if updateChildrenParent {
		seenRoots := make(map[*SMTEnode]struct{}, len(n.arguments))
		for index, argument := range n.arguments {
			root := argument.root
			if _, found := seenRoots[root]; found {
				continue
			}
			seenRoots[root] = struct{}{}
			parents := root.parents
			kept := parents[:0]
			removed := 0
			for _, parent := range parents {
				if parent == n {
					removed++
					continue
				}
				kept = append(kept, parent)
			}
			if removed == 0 && requireParentEntry {
				operation, _ := n.manager.GetDeclKind(n.owner)
				family, _ := n.manager.GetFamilyID(n.owner)
				return fmt.Errorf("delete enode %d family %d operation %d argument %d missing from parents (parents %d): %w", n.owner.Slot(), family, operation, index, len(parents), ast.ErrASTDomain)
			}
			for i := len(kept); i < len(parents); i++ {
				parents[i] = nil
			}
			root.parents = kept
			// enode::del_eh pops this node from its argument root's m_parents.
			for i := len(root.sourceParents) - 1; i >= 0; i-- {
				if root.sourceParents[i] == n {
					copy(root.sourceParents[i:], root.sourceParents[i+1:])
					root.sourceParents[len(root.sourceParents)-1] = nil
					root.sourceParents = root.sourceParents[:len(root.sourceParents)-1]
					break
				}
			}
		}
	}
	n.manager = nil
	n.root, n.next, n.congruence = nil, nil, nil
	n.parents, n.arguments = nil, nil
	return nil
}

// TransReaches walks the proof tree. Brent's check catches a corrupted cycle
// without allocating, since this runs for every equality a conflict expands.
func (n *SMTEnode) TransReaches(target *SMTEnode) bool {
	checkpoint, steps, limit := n, 0, 1
	for current := n; current != nil; current = current.trans.Target {
		if current == target {
			return true
		}
		if steps == limit {
			checkpoint, steps, limit = current, 0, limit*2
		}
		steps++
		if next := current.trans.Target; next == checkpoint {
			return false
		}
	}
	return false
}

func (n *SMTEnode) ContainsParentCongruentTo(parent *SMTEnode) (bool, error) {
	if err := n.valid(); err != nil {
		return false, err
	}
	for _, candidate := range n.parents {
		yes, _, err := SMTEnodesCongruent(candidate, parent)
		if err != nil {
			return false, err
		}
		if yes {
			return true, nil
		}
	}
	return false, nil
}

func (n *SMTEnode) CheckInvariant() error {
	if err := n.valid(); err != nil {
		return err
	}
	seen := make(map[*SMTEnode]struct{})
	foundRoot, hasInterpreted := false, false
	current := n
	for {
		if current == nil || current.root != n.root {
			return ast.ErrASTDomain
		}
		if _, found := seen[current]; found {
			if current != n {
				return ast.ErrASTDomain
			}
			break
		}
		seen[current] = struct{}{}
		foundRoot = foundRoot || current == n.root
		hasInterpreted = hasInterpreted || current.interpreted
		current = current.next
	}
	if !foundRoot || n == n.root && uint32(len(seen)) != n.classSize || hasInterpreted && !n.root.interpreted || !n.TransReaches(n.root) {
		return ast.ErrASTDomain
	}
	if n == n.root {
		for _, parent := range n.parents {
			found := false
			for _, argument := range parent.arguments {
				if argument.root == n.root {
					found = true
					break
				}
			}
			if !found {
				return ast.ErrASTDomain
			}
		}
		for member := n.root.next; member != n.root; member = member.next {
			for _, parent := range member.parents {
				if parent.IsTrueEquality() {
					continue
				}
				found, err := n.root.ContainsParentCongruentTo(parent)
				if err != nil || !found {
					if err != nil {
						return err
					}
					return ast.ErrASTDomain
				}
			}
		}
	}
	return nil
}

func (n *SMTEnode) DisplayLabels(out io.Writer) error {
	if err := n.valid(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTDomain
	}
	ownerID, err := n.OwnerID()
	if err != nil {
		return err
	}
	rootID, err := n.root.OwnerID()
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "#%d  ->  #%d, lbls: %s, plbls: %s, root->lbls: %s, root->plbls: %s", ownerID, rootID, n.labels.String(), n.propagatedLabels.String(), n.root.labels.String(), n.root.propagatedLabels.String()); err != nil {
		return err
	}
	if n.labelHash >= 0 {
		if _, err = fmt.Fprintf(out, ", lbl-hash: %d", n.labelHash); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, "\n")
	return err
}

type SMTTempEnode struct {
	manager *ast.ASTManager
	node    SMTEnode
}

func NewSMTTempEnode(m *ast.ASTManager) (*SMTTempEnode, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	t := &SMTTempEnode{manager: m}
	t.node.manager = m
	t.node.root, t.node.next = &t.node, &t.node
	t.node.classSize = 1
	t.node.congruenceEnabled = true
	t.node.functionDeclID = math.MaxUint32
	t.node.labelHash = -1
	t.node.theoryVariables = *NewDefaultSMTTheoryVarList()
	return t, nil
}

func (t *SMTTempEnode) Set(declaration ast.ASTHandle, args []*SMTEnode) (*SMTEnode, error) {
	if t == nil || t.manager == nil {
		return nil, ast.ErrASTHandle
	}
	kind, err := t.manager.GetKind(declaration)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTFuncDecl {
		return nil, ast.ErrASTDomain
	}
	flags, err := t.manager.GetFuncDeclFlags(declaration)
	if err != nil {
		return nil, err
	}
	for _, argument := range args {
		if argument == nil || argument.root == nil {
			return nil, ast.ErrASTDomain
		}
	}
	if t.node.declaration != declaration {
		t.node.functionDeclID = math.MaxUint32
	}
	t.node.declaration = declaration
	t.node.arguments = append(t.node.arguments[:0], args...)
	t.node.commutative = len(args) == 2 && flags.Commutative
	return &t.node, nil
}

func (t *SMTTempEnode) Reset() error {
	if t == nil || t.manager == nil {
		return ast.ErrASTHandle
	}
	t.node.functionDeclID = math.MaxUint32
	return nil
}
