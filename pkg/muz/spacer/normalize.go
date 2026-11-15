package spacer

// Source: src/muz/spacer/spacer_util.cpp normalize.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type normalizeCleanupFailure struct {
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	local       uparams.ParamsRef
	localLive   bool
	rewriter    *astrewriter.TheoryRewriter
	adhoc       *astrewriter.Traversal
	equivalence *ast.ExprEquivClass
	values      []ast.ASTRef
	output      ast.ASTRef
	cause       error
	cleanupErr  error
}

func (f *normalizeCleanupFailure) Error() string { return errors.Join(f.cause, f.cleanupErr).Error() }
func (f *normalizeCleanupFailure) Unwrap() error { return errors.Join(f.cause, f.cleanupErr) }
func (f *normalizeCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.manager.NumeralService().RuntimeOwner()
}
func (f *normalizeCleanupFailure) OriginalCause() error { return f.cause }
func (f *normalizeCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (f *normalizeCleanupFailure) close(includeOutput bool) (result error) {
	if f.equivalence != nil {
		if err := f.equivalence.Close(); err != nil {
			return err
		}
		f.equivalence = nil
	}
	if err := releaseNormalizeRefs(f.manager, f.values); err != nil {
		return err
	}
	f.values = nil
	if includeOutput && f.output != (ast.ASTRef{}) {
		if err := f.manager.DelRef(&f.output); err != nil {
			return err
		}
	}
	if f.adhoc != nil {
		if err := f.adhoc.Close(); err != nil {
			return err
		}
		f.adhoc = nil
	}
	if f.rewriter != nil {
		if err := f.rewriter.Close(); err != nil {
			return err
		}
		f.rewriter = nil
	}
	if f.localLive {
		if err := f.context.Del(&f.local); err != nil {
			return err
		}
		f.localLive = false
	}
	return result
}

func (f *normalizeCleanupFailure) Cleanup() error {
	f.cleanupErr = f.close(true)
	return f.cleanupErr
}

// Source ast_lt_proc compares get_id(), unlike the structural ast_lt order.
// Read IDs once while the concrete ASTRef owners keep every node live.
func normalizeLess(manager *ast.ASTManager, values []ast.ASTRef) error {
	type entry struct {
		ref ast.ASTRef
		id  uint32
	}
	entries := make([]entry, len(values))
	for i, ref := range values {
		handle, err := manager.RefGet(ref)
		if err != nil {
			return err
		}
		id, err := manager.GetASTID(handle)
		if err != nil {
			return err
		}
		entries[i] = entry{ref: ref, id: id}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	for i, item := range entries {
		values[i] = item.ref
	}
	return nil
}

func normalizeIsAnd(manager *ast.ASTManager, value ast.ASTHandle) (bool, error) {
	kind, err := manager.GetKind(value)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := manager.GetFamilyID(value)
	if err != nil || family != manager.BasicFamilyID() {
		return false, err
	}
	op, err := manager.GetDeclKind(value)
	return op == ast.ASTOpAnd, err
}

func releaseNormalizeRefs(manager *ast.ASTManager, values []ast.ASTRef) error {
	var result error
	for i := range values {
		if values[i] != (ast.ASTRef{}) {
			result = errors.Join(result, manager.DelRef(&values[i]))
		}
	}
	return result
}

// normalizeAdhocReduce is adhoc_rewriter_cfg::reduce_app(f, args): it sees
// the rewritten arguments before any application is built.
func normalizeAdhocReduce(manager *ast.ASTManager, family ast.FamilyID, decl ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	fid, err := manager.GetFamilyID(decl)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	op, err := manager.GetDeclKind(decl)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if fid == manager.BasicFamilyID() && op == ast.ASTOpNot {
		if len(args) != 1 {
			return ast.ASTHandle{}, false, nil
		}
		// not (not e) ==> e
		if nested, grandchild, err := manager.IsNot(args[0]); err != nil || nested {
			return grandchild, nested, err
		}
		return ast.ASTHandle{}, false, nil
	}
	if fid != family || (op != 2 && op != 3) || len(args) != 2 {
		return ast.ASTHandle{}, false, nil
	}
	left, right := args[0], args[1]
	sort, err := manager.GetSort(left)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	sortFamily, err := manager.GetFamilyID(sort)
	if err != nil || sortFamily != family {
		return ast.ASTHandle{}, false, err
	}
	sortKind, err := manager.GetDeclKind(sort)
	if err != nil || sortKind != 1 {
		return ast.ASTHandle{}, false, err
	}
	text, _, numeral, err := manager.ArithmeticNumeralText(right)
	if err != nil || !numeral || (op == 2 && text != "-1") || (op == 3 && text != "1") {
		return ast.ASTHandle{}, false, err
	}
	// t <= -1 ==> !(t >= 0); t >= 1 ==> !(t <= 0)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	comparison, err := manager.MkFamilyApp(family, 5-op, nil, []ast.ASTHandle{left, zero})
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	result, err := manager.MkNot(comparison)
	return result, err == nil, err
}

// adhocRewriterConfig is adhoc_rewriter_cfg, run by rewriter_tpl.
type adhocRewriterConfig struct {
	astrewriter.DefaultConfiguration
	manager *ast.ASTManager
	family  ast.FamilyID
}

func (c *adhocRewriterConfig) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, _ ast.ASTRef) (astrewriter.RewriteStatus, error) {
	values := make([]ast.ASTHandle, args.Count())
	for i := range values {
		value, err := args.At(uint32(i))
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		values[i] = value
	}
	result, done, err := normalizeAdhocReduce(c.manager, c.family, decl, values)
	if err != nil || !done {
		return astrewriter.RewriteFailed, err
	}
	if err = c.manager.SetRef(out, result); err != nil {
		return astrewriter.RewriteFailed, err
	}
	return astrewriter.RewriteDone, nil
}

// newAdhocTraversal is rewriter_tpl<adhoc_rewriter_cfg> adhoc_rw(m, false,
// adhoc_cfg). Its cache lives until Close, as the source local's does.
func newAdhocTraversal(manager *ast.ASTManager) (*astrewriter.Traversal, error) {
	family, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	traversal, err := astrewriter.NewTraversal(manager, &adhocRewriterConfig{manager: manager, family: family})
	if err != nil && traversal != nil {
		err = errors.Join(err, traversal.Close())
	}
	return traversal, err
}

// normalizeAdhoc is adhoc_rw(out.get(), out) with its own rewriter.
func normalizeAdhoc(manager *ast.ASTManager, root ast.ASTRef) (resultErr error) {
	traversal, err := newAdhocTraversal(manager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, traversal.Close()) }()
	handle, err := manager.RefGet(root)
	if err != nil {
		return err
	}
	return traversal.Rewrite(handle, root)
}

// normalizeFormula returns an owning reference. It follows the retained
// Spacer normalization pipeline; factorEqualities is only enabled by callers
// that use the corresponding source overload.
func normalizeFormula(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, formula ast.ASTHandle, useSimplifyBounds, factorEqualities bool) (result ast.ASTRef, resultErr error) {
	if manager == nil || context == nil || globals == nil {
		return ast.ASTRef{}, ErrManagerOwner
	}
	local, err := context.NewValue()
	if err != nil {
		return ast.ASTRef{}, err
	}
	cleanup := &normalizeCleanupFailure{manager: manager, context: context, local: local, localLive: true}
	defer func() {
		if resultErr == nil && result != (ast.ASTRef{}) {
			cleanup.output = result
			if err := cleanup.close(false); err != nil {
				cleanup.cause, cleanup.cleanupErr = err, err
				result, resultErr = ast.ASTRef{}, cleanup
				return
			}
			cleanup.output = ast.ASTRef{}
			return
		}
		result = ast.ASTRef{}
		if err := cleanup.close(true); err != nil {
			cleanup.cause, cleanup.cleanupErr = errors.Join(resultErr, err), err
			resultErr = cleanup
		}
	}()
	for _, name := range []string{"sort_sums", "gcd_rounding", "arith_lhs", "som", "flat"} {
		key := name
		if err = context.SetBoolName(&local, &key, true); err != nil {
			return ast.ASTRef{}, err
		}
	}
	rw, err := astrewriter.NewTheoryRewriter(manager, context, globals, &local)
	cleanup.rewriter = rw
	if err != nil {
		return ast.ASTRef{}, err
	}
	out, err := manager.NewRef(formula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	cleanup.output = out
	if err = rw.Rewrite(formula, out); err != nil {
		return ast.ASTRef{}, err
	}
	// adhoc_rw is a normalize local: its cache dies after v, before rw.
	adhoc, err := newAdhocTraversal(manager)
	cleanup.adhoc = adhoc
	if err != nil {
		return ast.ASTRef{}, err
	}
	rewritten, err := manager.RefGet(out)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = adhoc.Rewrite(rewritten, out); err != nil {
		return ast.ASTRef{}, err
	}
	rewritten, err = manager.RefGet(out)
	if err != nil {
		return ast.ASTRef{}, err
	}
	conjunction, err := normalizeIsAnd(manager, rewritten)
	if err != nil || !conjunction {
		return out, err
	}
	handles, err := manager.FlattenAnd([]ast.ASTHandle{rewritten})
	if err != nil || len(handles) <= 1 {
		return out, err
	}
	values, err := refsFromHandles(manager, handles)
	if err != nil {
		return ast.ASTRef{}, err
	}
	cleanup.values = values
	if err = normalizeLess(manager, values); err != nil {
		return ast.ASTRef{}, err
	}
	if useSimplifyBounds {
		values, err = simplifyBounds(manager, context, globals, values)
		cleanup.values = values
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	if factorEqualities {
		handles = handles[:0]
		for i := range values {
			handle, getErr := manager.RefGet(values[i])
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			handles = append(handles, handle)
		}
		equivalence, newErr := ast.NewExprEquivClass(manager)
		if newErr != nil {
			return ast.ASTRef{}, newErr
		}
		cleanup.equivalence = equivalence
		handles, err = manager.FactorEqualities(handles, equivalence)
		if err != nil {
			return ast.ASTRef{}, err
		}
		factored, factorErr := manager.EquivToExpr(equivalence)
		if factorErr != nil {
			return ast.ASTRef{}, factorErr
		}
		remaining, refErr := refsFromHandles(manager, handles)
		if refErr != nil {
			cleanup.values = append(append([]ast.ASTRef(nil), values...), factored...)
			return ast.ASTRef{}, refErr
		}
		oldCount := len(values)
		allOwned := make([]ast.ASTRef, 0, oldCount+len(remaining)+len(factored))
		allOwned = append(allOwned, values...)
		allOwned = append(allOwned, remaining...)
		allOwned = append(allOwned, factored...)
		cleanup.values = allOwned
		if err = releaseNormalizeRefs(manager, allOwned[:oldCount]); err != nil {
			return ast.ASTRef{}, err
		}
		values = allOwned[oldCount:]
		cleanup.values = values
	}
	handles = handles[:0]
	for i := range values {
		handle, getErr := manager.RefGet(values[i])
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		handles = append(handles, handle)
	}
	normalized, err := manager.MkAndArray(handles)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = manager.SetRef(out, normalized); err != nil {
		return ast.ASTRef{}, err
	}
	return out, nil
}
