package cmd_context

// Source: dparser::parse_domains, extract_domain_name, parse_domain,
// register_finite_sort, register_int_sort and get_sort.

import (
	"fmt"
	"math"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (p *datalogParser) parseDomains(tok dtoken) (dtoken, error) {
	saved := p.lexer.parsingDomains
	p.lexer.parsingDomains = true
	defer func() { p.lexer.parsingDomains = saved }()
	var err error
	for tok != dtkEOS && tok != dtkError {
		switch tok {
		case dtkID:
			tok, err = p.parseDomain()
		case dtkNewline:
			return p.lexer.nextToken(), nil
		case dtkInclude:
			tok = p.lexer.nextToken()
			if tok != dtkString {
				tok, err = p.unexpected(tok, "a string")
				break
			}
			tok, err = p.parseInclude(p.lexer.tokenData(), true)
			if err != nil {
				break
			}
			if tok != dtkNewline {
				tok, err = p.unexpected(tok, "newline expected after include statement")
			} else {
				tok = p.lexer.nextToken()
			}
		default:
			tok, err = p.unexpected(tok, "identifier, newline or include")
		}
		if err != nil {
			return tok, err
		}
	}
	return tok, nil
}

// extractDomainName drops the trailing digits of a sort name (V1234 is V).
// A name of digits only is not a domain name.
func extractDomainName(s string) (string, bool) {
	trimmed := strings.TrimRight(s, "0123456789")
	if trimmed == "" {
		return s, false
	}
	return trimmed, true
}

// datalogAtoi is glibc atoi: (int) strtol(s, NULL, 10). strtol clamps an
// out-of-range value to LONG_MIN/LONG_MAX before the conversion to int.
func datalogAtoi(s string) int32 {
	i := 0
	for i < len(s) && (s[i] == ' ' || (s[i] >= '\t' && s[i] <= '\r')) {
		i++
	}
	negative := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		negative = s[i] == '-'
		i++
	}
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
	var result int64
	switch {
	case negative && (overflow || value > 1<<63):
		result = math.MinInt64
	case negative:
		result = -int64(value)
	case overflow || value > math.MaxInt64:
		result = math.MaxInt64
	default:
		result = int64(value)
	}
	return int32(result)
}

func (p *datalogParser) parseDomain() (dtoken, error) {
	domainName, ok := extractDomainName(p.lexer.tokenData())
	if !ok {
		return p.unexpected(dtkID, "domain name")
	}
	tok := p.lexer.nextToken()
	if tok == dtkID && p.lexer.tokenData() == "int" {
		if err := p.registerIntSort(domainName); err != nil {
			return tok, err
		}
		tok = p.lexer.nextToken()
		if tok != dtkNewline {
			return p.unexpected(tok, "end of line")
		}
		return tok, nil
	}
	if tok != dtkNum {
		return p.unexpected(tok, "numeral or 'int'")
	}
	num := uint32(datalogAtoi(p.lexer.tokenData()))
	s, err := p.registerFiniteSort(domainName, uint64(num), base.DatalogSortSymbol)
	if err != nil {
		return tok, err
	}
	tok = p.lexer.nextToken()
	if tok == dtkID {
		if tok, err = p.parseMapfile(tok, s, p.lexer.tokenData()); err != nil {
			return tok, err
		}
	}
	if tok == dtkNewline {
		tok = p.lexer.nextToken()
	}
	return tok, nil
}

// mkDatalogSort is dl_decl_util::mk_sort.
func (p *datalogParser) mkDatalogSort(name string, domainSize uint64) (ast.ASTHandle, error) {
	if domainSize == 0 {
		return ast.ASTHandle{}, datalogExceptionf("Domain size of sort '%s' may not be 0", name)
	}
	symbol, err := p.manager.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s, err := p.manager.MkDatalogFiniteSort(symbol, domainSize)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.pin(s)
}

func (p *datalogParser) registerFiniteSort(name string, domainSize uint64, kind base.DatalogSortKind) (ast.ASTHandle, error) {
	if _, found := p.sortDict[name]; found {
		return ast.ASTHandle{}, datalogExceptionf("sort %s already declared", name)
	}
	s, err := p.mkDatalogSort(name, domainSize)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// context::register_finite_sort. A sort that an earlier parse of this
	// context registered keeps its domain and numbering (the C++ front end
	// parses one file per context).
	known, err := p.owner.HasSortDomain(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !known {
		if err = p.owner.RegisterFiniteSort(s, kind); err != nil {
			return ast.ASTHandle{}, err
		}
	} else if existing, err := p.owner.SortDomainKind(s); err != nil {
		return ast.ASTHandle{}, err
	} else if existing != kind {
		return ast.ASTHandle{}, fmt.Errorf("datalog: finite domain already registered with a different constant kind")
	}
	p.sortDict[name] = s
	return s, nil
}

func (p *datalogParser) registerIntSort(name string) error {
	if _, found := p.sortDict[name]; found {
		return datalogExceptionf("sort %s already declared", name)
	}
	arithmetic, err := p.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	s, err := p.manager.MkFamilySort(arithmetic, 1, nil)
	if err != nil {
		return err
	}
	p.sortDict[name] = s
	return nil
}

func (p *datalogParser) getSort(str string) (ast.ASTHandle, error) {
	res, found := p.sortDict[str]
	if !found {
		return ast.ASTHandle{}, datalogExceptionf("unknown sort \"%s\"", str)
	}
	return res, nil
}
