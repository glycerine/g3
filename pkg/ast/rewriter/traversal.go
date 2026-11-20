package rewriter

// Source: src/ast/rewriter/rewriter.h:194-408;
// src/ast/rewriter/rewriter_def.h:23-750; src/model/model_evaluator.cpp:282-289.

import (
	"github.com/glycerine/g3/pkg/ast"
	"reflect"
)

// RewriteStatus preserves the source bounded revisit depths and final statuses.
type RewriteStatus = brStatus

const (
	Rewrite1      RewriteStatus = brRewrite1
	Rewrite2      RewriteStatus = brRewrite2
	Rewrite3      RewriteStatus = brRewrite3
	RewriteFull   RewriteStatus = brRewriteFull
	RewriteDone   RewriteStatus = brDone
	RewriteFailed RewriteStatus = brFailed
)

// Configuration is borrowed by the original traversal. Callback result
// and proof references belong to the traversal. Embed DefaultConfiguration for
// the source defaults. Resource callbacks may return the source exception as a
// Go error; a true step-limit result produces the original max-steps exception.
type Configuration interface {
	CacheAllResults() bool
	CacheResults() bool
	FlatAssoc(d ast.ASTHandle) (bool, error)
	RewritePatterns() bool
	MaxScopesExceeded(n uint32) bool
	MaxFramesExceeded(n uint32) bool
	MaxStepsExceeded(n uint32) (bool, error)
	PreVisit(t ast.ASTHandle) (bool, error)
	ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (RewriteStatus, error)
	ReduceVar(v ast.ASTHandle, out, proof ast.ASTRef) (bool, error)
	ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (bool, error)
	GetMacro(d ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error)
	GetSubst(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error)
	Reset()
	Cleanup()
}

// Traversal exposes the original rewriter_tpl specialization across package
// boundaries. The configuration must outlive this owner.
type Traversal struct {
	engine rewriterEngine
}

// NewTraversal charges the enclosing owner once. A nonnil partial owner returned
// with an error must be closed, as with the other explicit AST owners.
func NewTraversal(m *ast.ASTManager, cfg Configuration) (*Traversal, error) {
	return newTraversalMode(m, cfg, false)
}

// NewProofTraversal constructs the proof-generating source specialization.
// The manager must have proofs enabled before construction.
func NewProofTraversal(m *ast.ASTManager, cfg Configuration) (*Traversal, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	proofs, e := m.ProofsEnabled()
	if e != nil {
		return nil, e
	}
	if !proofs {
		return nil, ast.ErrASTDomain
	}
	return newTraversalMode(m, cfg, true)
}

func newTraversalMode(m *ast.ASTManager, cfg Configuration, proofs bool) (*Traversal, error) {
	if m == nil || cfg == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(Traversal{}).Size()) + engineShifterCapacity()
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	r := &Traversal{}
	r.engine.header = bytes
	return r, r.engine.initMode(m, cfg, proofs)
}
func (r *Traversal) Rewrite(t ast.ASTHandle, out ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Rewrite(t, out)
}

// EnableModelBooleanShortCircuit avoids evaluating dead recursive model
// branches after AND/OR already has a determining operand. Ordinary rewriting
// retains source traversal order; this correction is confined to model use.
//
// BUG FIX relative to z3 4.7.1 (TODO.md "z3 4.7.1 bugs fixed in g3"):
// rewriter_tpl::constant_fold (src/ast/rewriter/rewriter_def.h) folds only
// ite early, so z3 4.7.1 never finishes evaluating mutually recursive
// define-fun-rec predicates in a model (get-value of (even 0) in
// test_vectors/recursive_mutual_boolean_model.smt2). That vector is excluded
// from z3 identity testing; recursive_mutual_safe_get_value.smt2 is its
// comparable companion. Upstream z3 answers at once via its recfun theory.
func (r *Traversal) EnableModelBooleanShortCircuit() error {
	if err := r.engine.open(); err != nil {
		return err
	}
	if r.engine.core.proofGen {
		return ast.ErrASTDomain
	}
	r.engine.modelBooleanShortCircuit = true
	return nil
}

// ProofGen is m_proof_gen, fixed when the rewriter was constructed.
func (r *Traversal) ProofGen() bool { return r != nil && r.engine.core.proofGen }

func (r *Traversal) RewriteWithProof(t ast.ASTHandle, out, proof ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.RewriteWithProof(t, out, proof)
}
func (r *Traversal) Resume(out ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Resume(out)
}
func (r *Traversal) ResumeWithProof(out, proof ast.ASTRef) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.ResumeWithProof(out, proof)
}
func (r *Traversal) Reset() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Reset()
}
func (r *Traversal) Cleanup() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Cleanup()
}
func (r *Traversal) Close() error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.Close()
}
func (r *Traversal) SetBindings(args []ast.ASTHandle) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.SetBindings(args)
}
func (r *Traversal) SetInvBindings(args []ast.ASTHandle) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	return r.engine.SetInvBindings(args)
}
func (r *Traversal) SetCancelCheck(enabled bool) error {
	if r == nil {
		return ast.ErrASTHandle
	}
	if e := r.engine.open(); e != nil {
		return e
	}
	return r.engine.core.SetCancelCheck(enabled)
}
func (r *Traversal) NumSteps() (uint32, error) {
	if r == nil {
		return 0, ast.ErrASTHandle
	}
	if e := r.engine.open(); e != nil {
		return 0, e
	}
	return r.engine.numSteps, nil
}
func (r *Traversal) CacheSize() (uint32, error) {
	if r == nil {
		return 0, ast.ErrASTHandle
	}
	if e := r.engine.open(); e != nil {
		return 0, e
	}
	return r.engine.core.getCacheSize()
}
