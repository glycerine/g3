package api

// Source: src/api/api_context.cpp save_ast_trail and generated API AST
// result/reference commands. Replay IDs are concrete Go-side object tokens.

import (
	"github.com/glycerine/g3/pkg/ast"
)

type replayASTKey struct {
	owner  *replayContextOwner
	handle ast.ASTHandle
}
type replayASTOwner struct {
	owner      *replayContextOwner
	handle     ast.ASTHandle
	references uint32
}

func (r *Z3Replayer) storeAST(owner *replayContextOwner, handle ast.ASTHandle) error {
	if owner == nil || owner.results == nil {
		return ErrZ3ReplayReference
	}
	if err := owner.results.saveASTTrail(handle); err != nil {
		return err
	}
	return r.storeBorrowedAST(owner, handle)
}

func (r *Z3Replayer) storeBorrowedAST(owner *replayContextOwner, handle ast.ASTHandle) error {
	if owner == nil || owner.results == nil {
		return ErrZ3ReplayReference
	}
	key := replayASTKey{owner: owner, handle: handle}
	if id := r.astIDs[key]; id != 0 {
		return r.StoreResult(id)
	}
	if r.nextObjectID == ^uint64(0) {
		return ErrZ3ReplayState
	}
	if r.asts == nil {
		r.asts = make(map[uint64]*replayASTOwner)
		r.astIDs = make(map[replayASTKey]uint64)
	}
	r.nextObjectID++
	id := r.nextObjectID
	r.asts[id] = &replayASTOwner{owner: owner, handle: handle}
	r.astIDs[key] = id
	owner.astIDs = append(owner.astIDs, id)
	return r.StoreResult(id)
}

func (r *Z3Replayer) astObject(owner *replayContextOwner, position uint32) (*replayASTOwner, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	return r.astByID(owner, id)
}

func (r *Z3Replayer) astByID(owner *replayContextOwner, id uint64) (*replayASTOwner, error) {
	entry := r.asts[id]
	if entry == nil || entry.owner != owner || owner == nil || owner.results == nil {
		return nil, ErrZ3ReplayReference
	}
	if _, err := owner.results.manager.GetKind(entry.handle); err != nil {
		return nil, err
	}
	return entry, nil
}

func (owner *replayContextOwner) close(r *Z3Replayer) error {
	if owner == nil {
		return nil
	}
	for len(owner.astIDs) > 0 {
		i := len(owner.astIDs) - 1
		id := owner.astIDs[i]
		entry := r.asts[id]
		if entry != nil {
			for entry.references > 0 {
				consumed, err := owner.results.manager.DecRefConsumed(entry.handle)
				if consumed {
					entry.references--
				}
				if err != nil {
					return err
				}
				if !consumed {
					return ErrZ3ReplayState
				}
			}
			delete(r.astIDs, replayASTKey{owner: owner, handle: entry.handle})
			delete(r.asts, id)
		}
		owner.astIDs = owner.astIDs[:i]
	}
	if owner.results != nil {
		if err := owner.results.Close(); err != nil {
			return err
		}
		owner.results = nil
	}
	for _, id := range owner.parameterIDs {
		delete(r.parameterSets, id)
	}
	owner.parameterIDs = nil
	for _, id := range owner.descriptionIDs {
		delete(r.descriptions, id)
	}
	owner.descriptionIDs = nil
	for _, id := range owner.solverIDs {
		delete(r.solvers, id)
	}
	owner.solverIDs = nil
	for _, id := range owner.modelIDs {
		delete(r.models, id)
	}
	owner.modelIDs = nil
	for _, id := range owner.goalIDs {
		delete(r.goals, id)
	}
	owner.goalIDs = nil
	for _, id := range owner.vectorIDs {
		delete(r.vectors, id)
	}
	owner.vectorIDs = nil
	for _, id := range owner.functionIDs {
		delete(r.functions, id)
	}
	owner.functionIDs = nil
	if owner.context != nil {
		if err := owner.context.Close(); err != nil {
			return err
		}
	}
	return nil
}
