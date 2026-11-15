package transforms

// Source: src/muz/transforms/dl_mk_interp_tail_simplifier.cpp normalizer_cfg
// and normalizer_rw (rewriter_tpl<normalizer_cfg>): the Boolean pass between
// the two context theory-rewriter invocations of simplify_expr.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// normalizerConfig is mk_interp_tail_simplifier::normalizer_cfg.
type normalizerConfig struct {
	astrewriter.DefaultConfiguration
	manager *ast.ASTManager
	// brwr is m_brwr (bool_rewriter with default parameters); nil only for
	// a plugin built without parameter services, where mk_not falls back to
	// ast_util's mk_not.
	brwr *astrewriter.BoolRewriter
	// appArgs is m_app_args, reset by each reduce_app.
	appArgs []ast.ASTRef
}

func (n *normalizerConfig) resetAppArgs() error {
	for len(n.appArgs) != 0 {
		if err := n.manager.DelRef(&n.appArgs[0]); err != nil {
			return err
		}
		n.appArgs = n.appArgs[1:]
	}
	return nil
}

func (n *normalizerConfig) close() error {
	if err := n.resetAppArgs(); err != nil {
		return err
	}
	if n.brwr != nil {
		if err := n.brwr.Close(); err != nil {
			return err
		}
		n.brwr = nil
	}
	return nil
}

func (n *normalizerConfig) pushAppArg(handle ast.ASTHandle) error {
	reference, err := n.manager.NewRef(handle)
	if err == nil {
		n.appArgs = append(n.appArgs, reference)
	}
	return err
}

func (n *normalizerConfig) appArgHandles() ([]ast.ASTHandle, error) {
	handles := make([]ast.ASTHandle, len(n.appArgs))
	for index := range n.appArgs {
		handle, err := n.manager.RefGet(n.appArgs[index])
		if err != nil {
			return nil, err
		}
		handles[index] = handle
	}
	return handles, nil
}

// basicOp reports the basic-family operation of an application or function
// declaration.
func (n *normalizerConfig) basicOp(handle ast.ASTHandle) (int32, bool, error) {
	kind, err := n.manager.GetKind(handle)
	if err != nil || (kind != ast.ASTApp && kind != ast.ASTFuncDecl) {
		return 0, false, err
	}
	family, err := n.manager.GetFamilyID(handle)
	if errors.Is(err, ast.ErrASTDomain) {
		// Plain uninterpreted declarations have no family metadata.
		return 0, false, nil
	}
	if err != nil || family != n.manager.BasicFamilyID() {
		return 0, false, err
	}
	operation, err := n.manager.GetDeclKind(handle)
	return operation, err == nil, err
}

func (n *normalizerConfig) isOp(handle ast.ASTHandle, operation int32) (bool, error) {
	op, basic, err := n.basicOp(handle)
	return basic && op == operation, err
}

// pushNot is m_brwr.mk_not(arg, tmp) followed by m_app_args.push_back(tmp).
func (n *normalizerConfig) pushNot(argument ast.ASTHandle) error {
	if n.brwr == nil {
		negated, err := n.manager.MkNotSimplified(argument)
		if err != nil {
			return err
		}
		return n.pushAppArg(negated)
	}
	tmp, err := n.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = n.brwr.MkNot(argument, tmp); err != nil {
		return errors.Join(err, n.manager.DelRef(&tmp))
	}
	n.appArgs = append(n.appArgs, tmp)
	return nil
}

func (n *normalizerConfig) compare(left, right ast.ASTHandle, depth int) (int, error) {
	if left == right {
		return 0, nil
	}
	leftNegative, leftBare, err := n.manager.IsNot(left)
	if err != nil {
		return 0, err
	}
	if leftNegative {
		left = leftBare
	}
	rightNegative, rightBare, err := n.manager.IsNot(right)
	if err != nil {
		return 0, err
	}
	if rightNegative {
		right = rightBare
	}
	if left == right {
		if leftNegative == rightNegative {
			return 0, nil
		}
		if !leftNegative {
			return -1, nil
		}
		return 1, nil
	}
	leftKind, err := n.manager.GetKind(left)
	if err != nil {
		return 0, err
	}
	rightKind, err := n.manager.GetKind(right)
	if err != nil {
		return 0, err
	}
	if leftKind != ast.ASTApp || rightKind != ast.ASTApp {
		if leftKind != ast.ASTApp && rightKind == ast.ASTApp {
			return -1, nil
		}
		if leftKind == ast.ASTApp && rightKind != ast.ASTApp {
			return 1, nil
		}
		leftID, e := n.manager.GetASTID(left)
		if e != nil {
			return 0, e
		}
		rightID, e := n.manager.GetASTID(right)
		if e != nil {
			return 0, e
		}
		return compareUint32(leftID, rightID), nil
	}
	leftDecl, err := n.manager.GetDecl(left)
	if err != nil {
		return 0, err
	}
	rightDecl, err := n.manager.GetDecl(right)
	if err != nil {
		return 0, err
	}
	if leftDecl != rightDecl {
		leftID, e := n.manager.GetASTID(leftDecl)
		if e != nil {
			return 0, e
		}
		rightID, e := n.manager.GetASTID(rightDecl)
		if e != nil {
			return 0, e
		}
		return compareUint32(leftID, rightID), nil
	}
	leftCount, err := n.manager.GetNumArgs(left)
	if err != nil {
		return 0, err
	}
	rightCount, err := n.manager.GetNumArgs(right)
	if err != nil {
		return 0, err
	}
	if leftCount != rightCount {
		return compareUint32(leftCount, rightCount), nil
	}
	if depth == 0 {
		leftID, e := n.manager.GetASTID(left)
		if e != nil {
			return 0, e
		}
		rightID, e := n.manager.GetASTID(right)
		if e != nil {
			return 0, e
		}
		return compareUint32(leftID, rightID), nil
	}
	negationComparison := 0
	for index := uint32(0); index < leftCount; index++ {
		leftArg, e := n.manager.GetArg(left, index)
		if e != nil {
			return 0, e
		}
		rightArg, e := n.manager.GetArg(right, index)
		if e != nil {
			return 0, e
		}
		leftNegated, leftArgBare, e := n.manager.IsNot(leftArg)
		if e != nil {
			return 0, e
		}
		if leftNegated {
			leftArg = leftArgBare
		}
		rightNegated, rightArgBare, e := n.manager.IsNot(rightArg)
		if e != nil {
			return 0, e
		}
		if rightNegated {
			rightArg = rightArgBare
		}
		if negationComparison == 0 && leftNegated != rightNegated {
			if leftNegated {
				negationComparison = -1
			} else {
				negationComparison = 1
			}
		}
		comparison, e := n.compare(leftArg, rightArg, depth-1)
		if e != nil || comparison != 0 {
			return comparison, e
		}
	}
	if negationComparison != 0 {
		return negationComparison, nil
	}
	leftID, err := n.manager.GetASTID(left)
	if err != nil {
		return 0, err
	}
	rightID, err := n.manager.GetASTID(right)
	if err != nil {
		return 0, err
	}
	return compareUint32(leftID, rightID), nil
}

func compareUint32(left, right uint32) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

// argPair is normalizer_cfg::arg_pair.
type argPair struct {
	first  ast.ASTHandle
	second ast.ASTHandle
}

// matchArgPair is match_arg_pair: m.is_and(e, a, b) / m.is_or(e, a, b) match
// binary applications only.
func (n *normalizerConfig) matchArgPair(expression ast.ASTHandle, seekConjunction bool) (argPair, bool, error) {
	operation := int32(ast.ASTOpOr)
	if seekConjunction {
		operation = ast.ASTOpAnd
	}
	matched, err := n.isOp(expression, operation)
	if err != nil || !matched {
		return argPair{}, false, err
	}
	kind, err := n.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return argPair{}, false, err
	}
	count, err := n.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return argPair{}, false, err
	}
	first, err := n.manager.GetArg(expression, 0)
	if err != nil {
		return argPair{}, false, err
	}
	second, err := n.manager.GetArg(expression, 1)
	return argPair{first: first, second: second}, err == nil, err
}

// detectEquivalence is detect_equivalence; the returned equality is held by
// the caller's m_app_args slot.
func (n *normalizerConfig) detectEquivalence(p1, p2 argPair, insideDisjunction bool) (ast.ASTHandle, bool, error) {
	firstLeftNegated, firstLeft, err := n.manager.IsNot(p1.first)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	secondLeftNegated, secondLeft, err := n.manager.IsNot(p2.first)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if firstLeftNegated == secondLeftNegated {
		return ast.ASTHandle{}, false, nil
	}
	firstRightNegated, firstRight, err := n.manager.IsNot(p1.second)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	secondRightNegated, secondRight, err := n.manager.IsNot(p2.second)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if firstRightNegated == secondRightNegated {
		return ast.ASTHandle{}, false, nil
	}
	var firstBare ast.ASTHandle
	if firstLeftNegated {
		if p2.first != firstLeft {
			return ast.ASTHandle{}, false, nil
		}
		firstBare = firstLeft
	}
	if secondLeftNegated {
		if p1.first != secondLeft {
			return ast.ASTHandle{}, false, nil
		}
		firstBare = secondLeft
	}
	var secondBare ast.ASTHandle
	if firstRightNegated {
		if p2.second != firstRight {
			return ast.ASTHandle{}, false, nil
		}
		secondBare = firstRight
	}
	if secondRightNegated {
		if p1.second != secondRight {
			return ast.ASTHandle{}, false, nil
		}
		secondBare = secondRight
	}
	boolean, err := n.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	firstSort, err := n.manager.GetSort(firstBare)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	secondSort, err := n.manager.GetSort(secondBare)
	if err != nil || firstSort != boolean || secondSort != boolean {
		return ast.ASTHandle{}, false, err
	}
	// both negations are in the same pair
	negsTogether := firstLeftNegated == firstRightNegated
	right := secondBare
	if negsTogether != insideDisjunction {
		if right, err = n.manager.MkNot(right); err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	equivalence, err := n.manager.MkEq(firstBare, right)
	return equivalence, err == nil, err
}

// detectEquivalences is detect_equivalences over m_app_args.
func (n *normalizerConfig) detectEquivalences(insideDisjunction bool) (bool, error) {
	havePair := false
	prevPairIndex := 0
	var pair argPair
	readIndex, writeIndex := 0, 0
	for readIndex < len(n.appArgs) {
		expression, err := n.manager.RefGet(n.appArgs[readIndex])
		if err != nil {
			return false, err
		}
		newPair, matched, err := n.matchArgPair(expression, insideDisjunction)
		if err != nil {
			return false, err
		}
		if matched {
			var equivalence ast.ASTHandle
			found := false
			if havePair {
				if equivalence, found, err = n.detectEquivalence(pair, newPair, insideDisjunction); err != nil {
					return false, err
				}
			}
			if found {
				havePair = false
				if err = n.manager.SetRef(n.appArgs[prevPairIndex], equivalence); err != nil {
					return false, err
				}
				readIndex++
				continue
			}
			havePair = true
			prevPairIndex = writeIndex
			pair = newPair
		} else {
			havePair = false
		}
		if writeIndex != readIndex {
			if err = n.manager.SetRef(n.appArgs[writeIndex], expression); err != nil {
				return false, err
			}
		}
		readIndex++
		writeIndex++
	}
	for index := writeIndex; index < len(n.appArgs); index++ {
		if err := n.manager.DelRef(&n.appArgs[index]); err != nil {
			return false, err
		}
	}
	n.appArgs = n.appArgs[:writeIndex]
	return readIndex != writeIndex, nil
}

// ReduceApp is normalizer_cfg::reduce_app.
func (n *normalizerConfig) ReduceApp(declaration ast.ASTHandle, arguments ast.AppArguments, out, _ ast.ASTRef) (astrewriter.RewriteStatus, error) {
	isNot, err := n.isOp(declaration, ast.ASTOpNot)
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	if isNot && arguments.Count() == 1 {
		argument, err := arguments.At(0)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		argumentAnd, err := n.isOp(argument, ast.ASTOpAnd)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		argumentOr, err := n.isOp(argument, ast.ASTOpOr)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		if argumentAnd || argumentOr {
			if err = n.resetAppArgs(); err != nil {
				return astrewriter.RewriteFailed, err
			}
			count, err := n.manager.GetNumArgs(argument)
			if err != nil {
				return astrewriter.RewriteFailed, err
			}
			for index := uint32(0); index < count; index++ {
				child, err := n.manager.GetArg(argument, index)
				if err != nil {
					return astrewriter.RewriteFailed, err
				}
				if err = n.pushNot(child); err != nil {
					return astrewriter.RewriteFailed, err
				}
			}
			handles, err := n.appArgHandles()
			if err != nil {
				return astrewriter.RewriteFailed, err
			}
			var result ast.ASTHandle
			if argumentAnd {
				result, err = n.manager.MkOrArray(handles)
			} else {
				result, err = n.manager.MkAndArray(handles)
			}
			if err == nil {
				err = n.manager.SetRef(out, result)
			}
			if err != nil {
				return astrewriter.RewriteFailed, err
			}
			return astrewriter.Rewrite2, nil
		}
	}
	isAnd, err := n.isOp(declaration, ast.ASTOpAnd)
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	isOr, err := n.isOp(declaration, ast.ASTOpOr)
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	if !isAnd && !isOr {
		return astrewriter.RewriteFailed, nil
	}
	count := arguments.Count()
	if count == 0 {
		result, err := n.manager.MkBoolVal(isAnd)
		if err == nil {
			err = n.manager.SetRef(out, result)
		}
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		return astrewriter.RewriteDone, nil
	}
	if count == 1 {
		argument, err := arguments.At(0)
		if err == nil {
			err = n.manager.SetRef(out, argument)
		}
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		return astrewriter.RewriteDone, nil
	}
	if err = n.resetAppArgs(); err != nil {
		return astrewriter.RewriteFailed, err
	}
	handles := make([]ast.ASTHandle, count)
	for index := uint32(0); index < count; index++ {
		if handles[index], err = arguments.At(index); err != nil {
			return astrewriter.RewriteFailed, err
		}
	}
	// std::sort with expr_cmp: a total order (ties only for identical
	// expressions), so the sorted sequence does not depend on the algorithm.
	var compareErr error
	sort.Slice(handles, func(left, right int) bool {
		if compareErr != nil {
			return false
		}
		comparison, e := n.compare(handles[left], handles[right], 4)
		if e != nil {
			compareErr = e
			return false
		}
		return comparison == -1
	})
	if compareErr != nil {
		return astrewriter.RewriteFailed, compareErr
	}
	// remove_duplicates
	unique := handles[:1]
	for _, handle := range handles[1:] {
		if unique[len(unique)-1] != handle {
			unique = append(unique, handle)
		}
	}
	for _, handle := range unique {
		if err = n.pushAppArg(handle); err != nil {
			return astrewriter.RewriteFailed, err
		}
	}
	haveRewrittenArgs, err := n.detectEquivalences(isOr)
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	resultArgs, err := n.appArgHandles()
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	var result ast.ASTHandle
	if len(resultArgs) == 1 {
		result = resultArgs[0]
	} else if isAnd {
		result, err = n.manager.MkAndArgs(resultArgs)
	} else {
		result, err = n.manager.MkOrArgs(resultArgs)
	}
	if err == nil {
		err = n.manager.SetRef(out, result)
	}
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	if haveRewrittenArgs {
		return astrewriter.Rewrite1, nil
	}
	return astrewriter.RewriteDone, nil
}

// booleanNormalizer is mk_interp_tail_simplifier's m_cfg and m_rw: one
// rewriter_tpl<normalizer_cfg> per plugin, so its cache outlives each
// simplify_expr call as in the source.
type booleanNormalizer struct {
	cfg *normalizerConfig
	rw  *astrewriter.Traversal
}

func newBooleanNormalizer(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (_ *booleanNormalizer, resultErr error) {
	n := &booleanNormalizer{cfg: &normalizerConfig{manager: manager}}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, n.close())
		}
	}()
	if context != nil && globals != nil {
		brwr, err := astrewriter.NewBoolRewriter(manager, context, globals, nil)
		if err != nil {
			if brwr != nil {
				err = errors.Join(err, brwr.Close())
			}
			return nil, err
		}
		n.cfg.brwr = brwr
	}
	rw, err := astrewriter.NewTraversal(manager, n.cfg)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return nil, err
	}
	n.rw = rw
	return n, nil
}

// close is ~mk_interp_tail_simplifier: dealloc(m_rw), then dealloc(m_cfg).
// A refused release leaves the remaining owners reachable for a retry.
func (n *booleanNormalizer) close() error {
	if n == nil {
		return nil
	}
	if n.rw != nil {
		if err := n.rw.Close(); err != nil {
			return err
		}
		n.rw = nil
	}
	if n.cfg != nil {
		if err := n.cfg.close(); err != nil {
			return err
		}
		n.cfg = nil
	}
	return nil
}

// normalize is (*m_rw)(expression, result).
func (n *booleanNormalizer) normalize(expression ast.ASTHandle) (ast.ASTRef, error) {
	out, err := n.cfg.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = n.rw.Rewrite(expression, out); err != nil {
		return ast.ASTRef{}, errors.Join(err, n.cfg.manager.DelRef(&out))
	}
	return out, nil
}
