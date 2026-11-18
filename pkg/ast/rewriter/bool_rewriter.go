package rewriter

// Source: src/ast/rewriter/bool_rewriter.cpp:89-280;
// src/ast/rewriter/bool_rewriter.h:60-69; src/util/buffer.h:25-48.

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Original Boolean rewriter's AND/OR core fragment. This inline helper borrows
// its AST manager; boolRewriterStar supplies the original traversal.
type boolRewriter struct {
	self                                *boolRewriter
	params                              uparams.ModuleParamsOwner
	header                              uint64
	ready, closed                       bool
	elimAnd                             bool
	blastDistinct                       bool
	blastDistinctThreshold              uint32
	manager                             *ast.ASTManager
	flat, iteExtraRules                 bool
	localContext                        bool
	localContextCost, localContextLimit uint32
	localOld, localNew                  []ast.ASTHandle
	localArg                            ast.ASTRef
	localActive                         bool
	andArgs                             [][]ast.ASTHandle
	andDepth                            uint32
	// nflatNeg/nflatPos are mk_nflat_*_core's expr_fast_mark1/2 (neg_lits,
	// pos_lits), keyed by node; nflatBuffer backs its ptr_buffer, used as
	// a stack because local-context simplification can re-enter.
	nflatNeg, nflatPos map[uint64]bool
	nflatBuffer        []ast.ASTHandle
}

func boolRewriterMarkKey(h ast.ASTHandle) uint64 {
	return uint64(h.Slot())<<32 | uint64(h.Generation())
}

func (rw *boolRewriter) isOperator(h ast.ASTHandle, op int32) (bool, error) {
	app, fid, kind, e := rw.manager.AppIdentity(h)
	if e != nil || !app || fid != 0 {
		return false, e
	}
	return kind == op, nil
}

func (rw *boolRewriter) mkNFlatAndCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkNFlatCore(astSliceArguments(args), out, false)
}
func (rw *boolRewriter) mkNFlatOrCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkNFlatCore(astSliceArguments(args), out, true)
}

// The two original loops are dual except for OR's optional local-context call.
// Keep that source branch after mark reset and before the changed-result test.
func (rw *boolRewriter) mkNFlatCore(args ast.AppArguments, out ast.ASTRef, disjunction bool) (status brStatus, err error) {
	m := rw.manager
	if rw.nflatNeg == nil {
		rw.nflatNeg, rw.nflatPos = make(map[uint64]bool), make(map[uint64]bool)
	}
	neg, pos := rw.nflatNeg, rw.nflatPos
	// The marks are reset on every exit, as the source's fast marks are.
	defer func() {
		clear(neg)
		clear(pos)
	}()
	base := len(rw.nflatBuffer)
	defer func() {
		clear(rw.nflatBuffer[base:])
		rw.nflatBuffer = rw.nflatBuffer[:base]
	}()
	unit, e := m.MkBoolVal(!disjunction)
	if e != nil {
		return brFailed, e
	}
	absorb, e := m.MkBoolVal(disjunction)
	if e != nil {
		return brFailed, e
	}
	changed := false
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return brFailed, e
		}
		if arg == unit {
			changed = true
			continue
		}
		if arg == absorb {
			return brDone, m.SetRef(out, absorb)
		}
		negative, atom, e := m.IsNot(arg)
		if e != nil {
			return brFailed, e
		}
		if negative {
			key := boolRewriterMarkKey(atom)
			if neg[key] {
				changed = true
				continue
			}
			if pos[key] {
				return brDone, m.SetRef(out, absorb)
			}
			neg[key] = true
		} else {
			key := boolRewriterMarkKey(arg)
			if pos[key] {
				changed = true
				continue
			}
			if neg[key] {
				return brDone, m.SetRef(out, absorb)
			}
			pos[key] = true
		}
		rw.nflatBuffer = append(rw.nflatBuffer, arg)
	}
	remaining := rw.nflatBuffer[base:len(rw.nflatBuffer):len(rw.nflatBuffer)]
	switch len(remaining) {
	case 0:
		return brDone, m.SetRef(out, unit)
	case 1:
		return brDone, m.SetRef(out, remaining[0])
	}
	if disjunction && rw.localContext && rw.localContextCost <= rw.localContextLimit {
		clear(neg)
		clear(pos)
		simplified, e := rw.localContextSimplify(remaining, out)
		if e != nil {
			return brFailed, e
		}
		if simplified {
			return brDone, nil
		}
	}
	if !changed {
		return brFailed, nil
	}
	var result ast.ASTHandle
	if disjunction {
		result, e = m.MkOrArgs(remaining)
	} else {
		result, e = m.MkAndArgs(remaining)
	}
	if e != nil {
		return brFailed, e
	}
	return brDone, m.SetRef(out, result)
}

func (rw *boolRewriter) mkFlatAndCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkFlatCore(astSliceArguments(args), out, false)
}
func (rw *boolRewriter) mkFlatOrCore(args []ast.ASTHandle, out ast.ASTRef) (brStatus, error) {
	return rw.mkFlatCore(astSliceArguments(args), out, true)
}
func (rw *boolRewriter) mkFlatCore(args ast.AppArguments, out ast.ASTRef, disjunction bool) (brStatus, error) {
	op := int32(ast.ASTOpAnd)
	if disjunction {
		op = ast.ASTOpOr
	}
	i := uint32(0)
	for ; i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return brFailed, e
		}
		nested, e := rw.isOperator(arg, op)
		if e != nil {
			return brFailed, e
		}
		if nested {
			break
		}
	}
	if i == args.Count() {
		return rw.mkNFlatCore(args, out, disjunction)
	}
	m := rw.manager
	var flat []ast.ASTHandle
	for j := uint32(0); j < i; j++ {
		arg, e := args.At(j)
		if e != nil {
			return brFailed, e
		}
		flat = append(flat, arg)
	}
	for ; i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return brFailed, e
		}
		nested, e := rw.isOperator(arg, op)
		if e != nil {
			return brFailed, e
		}
		if !nested {
			flat = append(flat, arg)
			continue
		}
		n, e := m.GetNumArgs(arg)
		if e != nil {
			return brFailed, e
		}
		for j := uint32(0); j < n; j++ {
			h, e := m.GetArg(arg, j)
			if e != nil {
				return brFailed, e
			}
			flat = append(flat, h)
		}
	}
	status, e := rw.mkNFlatCore(astSliceArguments(flat), out, disjunction)
	if e != nil {
		return brFailed, e
	}
	if status == brFailed {
		var result ast.ASTHandle
		if disjunction {
			result, e = m.MkOrArgs(flat)
		} else {
			result, e = m.MkAndArgs(flat)
		}
		if e != nil {
			return brFailed, e
		}
		if e = m.SetRef(out, result); e != nil {
			return brFailed, e
		}
	}
	return brDone, nil
}
