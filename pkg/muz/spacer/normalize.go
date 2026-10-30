package spacer

// Source: src/muz/spacer/spacer_util.cpp normalize.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func normalizeLess(manager *ast.ASTManager, values []ast.ASTRef) error {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0; j-- {
			left, err := manager.RefGet(values[j])
			if err != nil {
				return err
			}
			right, err := manager.RefGet(values[j-1])
			if err != nil {
				return err
			}
			less, err := manager.ASTLess(left, right)
			if err != nil {
				return err
			}
			if !less {
				break
			}
			values[j-1], values[j] = values[j], values[j-1]
		}
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

func normalizeAdhocReplacement(manager *ast.ASTManager, family ast.FamilyID, value ast.ASTHandle) (ast.ASTHandle, bool, error) {
	kind, err := manager.GetKind(value)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	if yes, child, notErr := manager.IsNot(value); notErr != nil {
		return ast.ASTHandle{}, false, notErr
	} else if yes {
		if nested, grandchild, nestedErr := manager.IsNot(child); nestedErr != nil {
			return ast.ASTHandle{}, false, nestedErr
		} else if nested {
			return grandchild, true, nil
		}
	}
	fid, err := manager.GetFamilyID(value)
	if err != nil || fid != family {
		return ast.ASTHandle{}, false, err
	}
	op, err := manager.GetDeclKind(value)
	if err != nil || (op != 2 && op != 3) {
		return ast.ASTHandle{}, false, err
	}
	left, err := manager.GetArg(value, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	right, err := manager.GetArg(value, 1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
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

// normalizeAdhoc is the concrete bottom-up adhoc_rewriter_cfg traversal.
// Rebuild parents from rewritten children before applying the local rule.
func normalizeAdhoc(manager *ast.ASTManager, root ast.ASTRef) (resultErr error) {
	handle, err := manager.RefGet(root)
	if err != nil {
		return err
	}
	family, err := manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	type frame struct {
		node     ast.ASTHandle
		children []ast.ASTHandle
		patterns int
		ready    bool
	}
	todo := []frame{{node: handle}}
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	owned := make([]ast.ASTRef, 0)
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, owned)) }()
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if _, done := cache[current.node]; done {
			continue
		}
		kind, err := manager.GetKind(current.node)
		if err != nil {
			return err
		}
		if !current.ready {
			switch kind {
			case ast.ASTApp:
				count, err := manager.GetNumArgs(current.node)
				if err != nil {
					return err
				}
				for i := uint32(0); i < count; i++ {
					child, err := manager.GetArg(current.node, i)
					if err != nil {
						return err
					}
					current.children = append(current.children, child)
				}
			case ast.ASTQuantifier:
				body, err := manager.GetQuantifierBody(current.node)
				if err != nil {
					return err
				}
				current.children = append(current.children, body)
				count, err := manager.GetNumPatterns(current.node)
				if err != nil {
					return err
				}
				current.patterns = int(count)
				for i := uint32(0); i < count; i++ {
					child, err := manager.GetPattern(current.node, i)
					if err != nil {
						return err
					}
					current.children = append(current.children, child)
				}
				count, err = manager.GetNumNoPatterns(current.node)
				if err != nil {
					return err
				}
				for i := uint32(0); i < count; i++ {
					child, err := manager.GetNoPattern(current.node, i)
					if err != nil {
						return err
					}
					current.children = append(current.children, child)
				}
			}
			current.ready = true
			todo = append(todo, current)
			for i := len(current.children); i > 0; i-- {
				todo = append(todo, frame{node: current.children[i-1]})
			}
			continue
		}
		changed := false
		children := make([]ast.ASTHandle, len(current.children))
		for i, child := range current.children {
			children[i] = cache[child]
			changed = changed || children[i] != child
		}
		rebuilt := current.node
		if changed {
			switch kind {
			case ast.ASTApp:
				decl, err := manager.GetDecl(current.node)
				if err != nil {
					return err
				}
				rebuilt, err = manager.MkApp(decl, children)
				if err != nil {
					return err
				}
			case ast.ASTQuantifier:
				rebuilt, err = manager.UpdateQuantifierAnnotations(current.node, children[1:1+current.patterns], children[1+current.patterns:], children[0])
				if err != nil {
					return err
				}
			}
		}
		ref, err := manager.NewRef(rebuilt)
		if err != nil {
			return err
		}
		owned = append(owned, ref)
		replacement, replace, err := normalizeAdhocReplacement(manager, family, rebuilt)
		if err != nil {
			return err
		}
		if replace {
			if err = manager.SetRef(ref, replacement); err != nil {
				return err
			}
			rebuilt = replacement
		}
		cache[current.node] = rebuilt
	}
	return manager.SetRef(root, cache[handle])
}

// normalizeFormula returns an owning reference. It follows the retained
// Spacer normalization pipeline; factorEqualities is only enabled by callers
// that use the corresponding source overload.
func normalizeFormula(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, formula ast.ASTHandle, useSimplifyBounds, factorEqualities bool) (_ ast.ASTRef, resultErr error) {
	if manager == nil || context == nil || globals == nil {
		return ast.ASTRef{}, ErrManagerOwner
	}
	local, err := context.NewValue()
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Del(&local)) }()
	for _, name := range []string{"sort_sums", "gcd_rounding", "arith_lhs", "som", "flat"} {
		key := name
		if err = context.SetBoolName(&local, &key, true); err != nil {
			return ast.ASTRef{}, err
		}
	}
	rw, err := astrewriter.NewTheoryRewriter(manager, context, globals, &local)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, rw.Close()) }()
	out, err := manager.NewRef(formula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, manager.DelRef(&out))
		}
	}()
	if err = rw.Rewrite(formula, out); err != nil {
		return ast.ASTRef{}, err
	}
	if err = normalizeAdhoc(manager, out); err != nil {
		return ast.ASTRef{}, err
	}
	rewritten, err := manager.RefGet(out)
	if err != nil {
		return ast.ASTRef{}, err
	}
	conjunction, err := normalizeIsAnd(manager, rewritten)
	if err != nil || !conjunction {
		committed = err == nil
		return out, err
	}
	handles, err := manager.FlattenAnd([]ast.ASTHandle{rewritten})
	if err != nil || len(handles) <= 1 {
		if err == nil {
			committed = true
		}
		return out, err
	}
	values, err := refsFromHandles(manager, handles)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, values)) }()
	if err = normalizeLess(manager, values); err != nil {
		return ast.ASTRef{}, err
	}
	if useSimplifyBounds {
		values, err = simplifyBounds(manager, context, globals, values)
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
		defer func() { resultErr = errors.Join(resultErr, equivalence.Close()) }()
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
			_ = releaseNormalizeRefs(manager, factored)
			return ast.ASTRef{}, refErr
		}
		if err = releaseNormalizeRefs(manager, values); err != nil {
			_ = releaseNormalizeRefs(manager, remaining)
			_ = releaseNormalizeRefs(manager, factored)
			values = nil
			return ast.ASTRef{}, err
		}
		values = append(remaining, factored...)
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
	committed = true
	return out, nil
}
