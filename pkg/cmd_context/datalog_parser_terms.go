package cmd_context

// Source: dparser::parse_arg, parse_infix, mk_const and mk_symbol_const;
// string_to_uint64 (src/muz/base/dl_util.cpp).

import (
	"errors"
	"math"
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (p *datalogParser) parseArg(tok dtoken, s ast.ASTHandle, args []ast.ASTHandle) (dtoken, []ast.ASTHandle, error) {
	switch tok {
	case dtkWild:
		v, err := p.manager.MkVar(p.numVars, s)
		if err != nil {
			return tok, args, err
		}
		p.numVars++
		if v, err = p.pin(v); err != nil {
			return tok, args, err
		}
		args = append(args, v)
	case dtkID:
		// All IDs are variables (dparser::is_var).
		data := p.lexer.tokenData()
		v, found := p.vars[data]
		if !found {
			idx := p.numVars
			p.numVars++
			var err error
			if v, err = p.manager.MkVar(idx, s); err != nil {
				return tok, args, err
			}
			if v, err = p.pin(v); err != nil {
				return tok, args, err
			}
			p.vars[data] = v
		} else {
			vs, err := p.manager.GetSort(v)
			if err != nil {
				return tok, args, err
			}
			if s != vs {
				expected, err := p.sortName(s)
				if err != nil {
					return tok, args, err
				}
				got, err := p.sortName(vs)
				if err != nil {
					return tok, args, err
				}
				return tok, args, datalogExceptionf("sort: %s expected, but got: %s\n", expected, got)
			}
		}
		args = append(args, v)
	case dtkString:
		c, err := p.mkConst(p.lexer.tokenData(), s)
		if err != nil {
			return tok, args, err
		}
		args = append(args, c)
	case dtkNum:
		// rational num(data); num.is_uint64().
		num, ok := new(big.Int).SetString(p.lexer.tokenData(), 10)
		if !ok || !num.IsUint64() {
			tok, err := p.unexpected(tok, "integer expected")
			return tok, args, err
		}
		numeral, err := p.mkSymbolConst(num.Uint64(), s)
		if err != nil {
			return tok, args, err
		}
		args = append(args, numeral)
	default:
	}
	return p.lexer.nextToken(), args, nil
}

func (p *datalogParser) sortName(s ast.ASTHandle) (string, error) {
	name, err := p.manager.GetASTName(s)
	if err != nil {
		return "", err
	}
	return p.manager.Symbols.String(name)
}

// parseInfix parses
//
//	infix: Sym REL Sym
//	Sym ::= String | NUM | Var
func (p *datalogParser) parseInfix(tok1 dtoken, td1 string) (dtoken, ast.ASTHandle, error) {
	var v1, v2 ast.ASTHandle
	var have1, have2 bool
	tok2 := p.lexer.nextToken()
	if tok2 != dtkNEQ && tok2 != dtkGT && tok2 != dtkLT && tok2 != dtkEQ {
		tok, err := p.unexpected(tok2, "built-in infix operator")
		return tok, ast.ASTHandle{}, err
	}
	tok3 := p.lexer.nextToken()
	td2 := p.lexer.tokenData()
	_, isVar := p.vars[td2]
	if tok3 != dtkString && tok3 != dtkNum && !(tok3 == dtkID && isVar) {
		tok, err := p.unexpected(tok3, "identifier")
		return tok, ast.ASTHandle{}, err
	}

	if tok1 == dtkID {
		v1, have1 = p.vars[td1]
	}
	if tok3 == dtkID {
		v2, have2 = p.vars[td2]
	}
	if !have1 && !have2 {
		tok, err := p.unexpected(tok3, "at least one argument should be a variable")
		return tok, ast.ASTHandle{}, err
	}
	var s ast.ASTHandle
	var err error
	if have1 {
		s, err = p.manager.GetSort(v1)
	} else {
		s, err = p.manager.GetSort(v2)
	}
	if err != nil {
		return tok3, ast.ASTHandle{}, err
	}
	if !have1 {
		if v1, err = p.mkConst(td1, s); err != nil {
			return tok3, ast.ASTHandle{}, err
		}
	}
	if !have2 {
		if v2, err = p.mkConst(td2, s); err != nil {
			return tok3, ast.ASTHandle{}, err
		}
	}

	var pred ast.ASTHandle
	switch tok2 {
	case dtkEQ:
		pred, err = p.mkEq(v1, v2)
	case dtkNEQ:
		var eq ast.ASTHandle
		if eq, err = p.mkEq(v1, v2); err == nil {
			pred, err = p.manager.MkNot(eq)
		}
	case dtkLT:
		pred, err = p.mkDatalogLT(v1, v2)
	case dtkGT:
		pred, err = p.mkDatalogLT(v2, v1)
	}
	if err != nil {
		return tok3, ast.ASTHandle{}, err
	}
	if pred, err = p.pin(pred); err != nil {
		return tok3, ast.ASTHandle{}, err
	}
	return p.lexer.nextToken(), pred, nil
}

// mkEq is ast_manager::mk_eq: mk_app(basic_family_id, OP_EQ, lhs, rhs).
// The sort error of the basic plugin ("Sorts V and W are incompatible") is
// an ast_exception, which parse_stream catches.
func (p *datalogParser) mkEq(lhs, rhs ast.ASTHandle) (ast.ASTHandle, error) {
	res, err := p.manager.MkFamilyApp(0, ast.ASTOpEq, nil, []ast.ASTHandle{lhs, rhs})
	var sortError *ast.ASTSortError
	if errors.As(err, &sortError) {
		return ast.ASTHandle{}, &datalogException{msg: sortError.Message}
	}
	return res, err
}

// mkDatalogLT is dl_decl_util::mk_lt, with the sort checks of
// dl_decl_plugin::mk_compare that raise the ast_manager exceptions.
func (p *datalogParser) mkDatalogLT(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	sa, err := p.manager.GetSort(a)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	finite, err := p.manager.IsDatalogFiniteSort(sa)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !finite {
		return ast.ASTHandle{}, datalogExceptionf("expecting finite domain sort")
	}
	sb, err := p.manager.GetSort(b)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sa != sb {
		return ast.ASTHandle{}, datalogExceptionf("expecting two identical finite domain sorts")
	}
	return p.manager.MkDatalogLT(a, b)
}

// isIntSort is arith_util::is_int.
func (p *datalogParser) isIntSort(s ast.ASTHandle) (bool, error) {
	arithmetic, err := p.manager.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	intSort, err := p.manager.MkFamilySort(arithmetic, 1, nil)
	if err != nil {
		return false, err
	}
	return s == intSort, nil
}

func (p *datalogParser) mkConst(name string, s ast.ASTHandle) (ast.ASTHandle, error) {
	integer, err := p.isIntSort(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var res ast.ASTHandle
	if integer {
		val, ok := stringToUint64(name)
		if !ok {
			return ast.ASTHandle{}, datalogExceptionf("Invalid integer: \"%s\"", name)
		}
		res, err = p.manager.MkDatalogNumeral(val, s)
	} else {
		var idx uint64
		if idx, err = p.getConstantNumber(s, name); err != nil {
			return ast.ASTHandle{}, err
		}
		res, err = p.manager.MkDatalogNumeral(idx, s)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.pin(res)
}

// mkSymbolConst makes a constant for an SK_SYMBOL sort out of an integer.
func (p *datalogParser) mkSymbolConst(el uint64, s ast.ASTHandle) (ast.ASTHandle, error) {
	integer, err := p.isIntSort(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var res ast.ASTHandle
	if integer {
		res, err = p.manager.MkDatalogNumeral(el, s)
	} else {
		var idx uint64
		if idx, err = p.getConstantNumber(s, strconv.FormatUint(el, 10)); err != nil {
			return ast.ASTHandle{}, err
		}
		res, err = p.manager.MkDatalogNumeral(idx, s)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.pin(res)
}

// getConstantNumber is context::get_constant_number(srt, symbol). Its
// "contains more constants than its declared size" error is a
// default_exception.
func (p *datalogParser) getConstantNumber(s ast.ASTHandle, name string) (uint64, error) {
	symbol, err := p.manager.Symbols.Intern(name)
	if err != nil {
		return 0, err
	}
	idx, err := p.owner.SymbolNumber(s, symbol)
	if err != nil {
		var overflow *base.DatalogDomainSizeError
		if errors.As(err, &overflow) {
			return 0, &datalogException{msg: overflow.Message}
		}
		return 0, err
	}
	return idx, nil
}

// stringToUint64 is string_to_uint64: sscanf(s, "%" SCNu64, &res) != 0.
// The conversion is strtoull's: leading white space, an optional sign (a
// negative value wraps), and a value past the range reads as ULLONG_MAX.
// sscanf returns EOF for a string of only white space; the C++ then reads
// res uninitialized, which is not reproduced: such a string is not an
// integer here.
func stringToUint64(s string) (uint64, bool) {
	i := 0
	for i < len(s) && (s[i] == ' ' || (s[i] >= '\t' && s[i] <= '\r')) {
		i++
	}
	negative := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		negative = s[i] == '-'
		i++
	}
	start := i
	var value uint64
	overflow := false
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		digit := uint64(s[i] - '0')
		if value > (math.MaxUint64-digit)/10 {
			overflow = true
			continue
		}
		value = value*10 + digit
	}
	if i == start {
		return 0, false
	}
	if overflow {
		return math.MaxUint64, true
	}
	if negative {
		value = -value
	}
	return value, true
}
