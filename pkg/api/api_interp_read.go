package api

// Source: src/api/api_interp.cpp Z3_read_interpolation_problem.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// InterpolationProblem is a parsed sequence or tree interpolation problem.
// Its AST handles are borrowed from the reading InterpolationContext until
// that context is closed. Parents is nil for a sequence; tree parents use the
// interp.ScopeTop as the root sentinel, matching the source API.
type InterpolationProblem struct {
	Constraints []ast.ASTHandle
	Parents     []int
	Theory      []ast.ASTHandle
}

// ReadInterpolationProblem parses the retained interpolation file format.
// The optional first-line comment `;! THEORY=n` marks the leading n
// assertions as background theory. In tree mode, each remaining assertion
// encodes child markers and a local constraint as nested implications.
func (c *InterpolationContext) ReadInterpolationProblem(input io.Reader, tree bool) (result InterpolationProblem, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	if input == nil {
		return result, ast.ErrASTHandle
	}
	reader := bufio.NewReader(input)
	firstLine, readErr := reader.ReadString('\n')
	if readErr != nil && readErr != io.EOF {
		return result, readErr
	}
	if readErr == io.EOF && firstLine == "" {
		return result, nil
	}
	theoryCount, err := interpolationTheoryCount(firstLine)
	if err != nil {
		return result, err
	}
	parseContext, err := NewSMTContext()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, parseContext.Close()) }()
	parsed, err := parseContext.ParseSMT2(io.MultiReader(strings.NewReader(firstLine), reader), SMT2Bindings{})
	if err != nil {
		return result, err
	}
	sourceManager, err := parseContext.Manager()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, sourceManager.DelRef(&parsed)) }()
	combined, err := sourceManager.RefGet(parsed)
	if err != nil {
		return result, err
	}
	assertions, err := interpolationConjuncts(sourceManager, combined)
	if err != nil {
		return result, err
	}
	if theoryCount > uint32(len(assertions)) {
		theoryCount = uint32(len(assertions))
	}
	translated, refs, err := c.translateInterpolationAssertions(sourceManager, assertions)
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil {
			for i := len(refs) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[i]))
			}
		}
	}()
	theoryCountInt := int(theoryCount)
	result.Theory = append([]ast.ASTHandle(nil), translated[:theoryCountInt]...)
	formulas := translated[theoryCountInt:]
	if tree {
		result.Constraints, result.Parents, err = readInterpolationTree(c.manager, formulas)
		if err != nil {
			return InterpolationProblem{}, err
		}
	} else {
		result.Constraints = append([]ast.ASTHandle(nil), formulas...)
	}
	c.retained = append(c.retained, refs...)
	return result, nil
}

// interpolationTheoryCount is get_file_params plus atoi: the first line
// must begin with ";!" (no trimming), tokens are split on spaces only, the
// last THEORY=... wins, and atoi reads an optional sign and leading digits
// (0 when there are none) into the unsigned num_theory.
func interpolationTheoryCount(line string) (uint32, error) {
	line = strings.TrimSuffix(line, "\n") // std::getline drops the newline
	if len(line) < 2 || line[0] != ';' || line[1] != '!' {
		return 0, nil
	}
	value, found := "", false
	for _, token := range strings.Split(line[2:], " ") {
		if key, right, ok := strings.Cut(token, "="); ok && key == "THEORY" {
			value, found = right, true
		}
	}
	if !found {
		return 0, nil
	}
	return uint32(interpolationAtoi(value)), nil
}

func interpolationAtoi(text string) int32 {
	i := 0
	for i < len(text) && (text[i] == ' ' || text[i] >= '\t' && text[i] <= '\r') {
		i++
	}
	negative := false
	if i < len(text) && (text[i] == '+' || text[i] == '-') {
		negative = text[i] == '-'
		i++
	}
	var v int32
	for ; i < len(text) && text[i] >= '0' && text[i] <= '9'; i++ {
		v = 10*v + int32(text[i]-'0')
	}
	if negative {
		v = -v
	}
	return v
}

// interpolationConjuncts is iZ3_parse: Z3_parse_smtlib2_file returns
// api::context::mk_and of the assertions, which is true for none and the
// assertion itself for one, and the arguments of that application are the
// assertions. So a single assertion (or a b) yields a and b (z3 4.7.1's
// behavior, kept). A single quantified assertion is not an application:
// z3 4.7.1's Z3_to_app on it was undefined, and FORK FIX #137 keeps it as
// the one assertion, as here.
func interpolationConjuncts(manager *ast.ASTManager, expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	kind, err := manager.GetKind(expression)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTApp {
		return []ast.ASTHandle{expression}, nil
	}
	count, err := manager.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, count)
	for i := range result {
		result[i], err = manager.GetArg(expression, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (c *InterpolationContext) translateInterpolationAssertions(source *ast.ASTManager, assertions []ast.ASTHandle) (result []ast.ASTHandle, refs []ast.ASTRef, resultErr error) {
	translation, err := ast.NewASTTranslation(source, c.manager, true)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, translation.Close())
		if resultErr != nil {
			for i := len(refs) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[i]))
			}
			result = nil
			refs = nil
		}
	}()
	translated := make([]ast.ASTHandle, 0, len(assertions))
	refs = make([]ast.ASTRef, 0, len(assertions))
	for _, assertion := range assertions {
		handle, translateErr := translation.Translate(assertion)
		if translateErr != nil {
			return nil, refs, translateErr
		}
		ref, refErr := c.manager.NewRef(handle)
		if refErr != nil {
			return nil, refs, refErr
		}
		translated = append(translated, handle)
		refs = append(refs, ref)
	}
	result = translated
	return result, refs, nil
}

func readInterpolationTree(manager *ast.ASTManager, formulas []ast.ASTHandle) ([]ast.ASTHandle, []int, error) {
	if len(formulas) == 0 {
		return nil, nil, ast.ErrASTDomain
	}
	constraints := make([]ast.ASTHandle, len(formulas))
	parents := make([]int, len(formulas))
	for i := range parents {
		parents[i] = -1
	}
	names := make(map[ast.ASTHandle]int, len(formulas))
	for index, formula := range formulas {
		chain, name, err := interpolationTreeChain(manager, formula)
		if err != nil {
			return nil, nil, fmt.Errorf("interpolation: formula %d: %w", index+1, err)
		}
		if len(chain) == 0 {
			return nil, nil, fmt.Errorf("interpolation: formula %d: expected (implies {children} formula parent)", index+1)
		}
		for _, child := range chain[:len(chain)-1] {
			childIndex, found := names[child]
			if !found {
				return nil, nil, fmt.Errorf("interpolation: formula %d: unknown child marker", index+1)
			}
			if parents[childIndex] != -1 {
				return nil, nil, fmt.Errorf("interpolation: formula %d: child marker has multiple parents", index+1)
			}
			parents[childIndex] = index
		}
		local := chain[len(chain)-1]
		if _, duplicate := names[name]; duplicate {
			return nil, nil, fmt.Errorf("interpolation: formula %d: duplicate node marker", index+1)
		}
		constraints[index] = local
		names[name] = index
	}
	for i := 0; i < len(parents)-1; i++ {
		if parents[i] == -1 {
			return nil, nil, fmt.Errorf("interpolation: formula %d: unreferenced node", i+1)
		}
	}
	parents[len(parents)-1] = interp.ScopeTop
	return constraints, parents, nil
}

func interpolationTreeChain(manager *ast.ASTManager, formula ast.ASTHandle) ([]ast.ASTHandle, ast.ASTHandle, error) {
	var antecedents []ast.ASTHandle
	current := formula
	for {
		kind, err := manager.GetDeclKind(current)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		if kind != ast.ASTOpImplies {
			break
		}
		left, err := manager.GetArg(current, 0)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		right, err := manager.GetArg(current, 1)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		leftKind, err := manager.GetDeclKind(left)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		if len(antecedents) == 0 && leftKind == ast.ASTOpAnd {
			parts, splitErr := interpolationConjuncts(manager, left)
			if splitErr != nil {
				return nil, ast.ASTHandle{}, splitErr
			}
			antecedents = append(antecedents, parts...)
		} else {
			antecedents = append(antecedents, left)
		}
		current = right
	}
	return antecedents, current, nil
}
