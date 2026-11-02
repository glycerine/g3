package fp

// Source: dparser::parse_include, parse_mapfile and line_reader.

import (
	"bufio"
	"errors"
	"os"

	"github.com/glycerine/g3/pkg/ast"
)

func joinErrors(left, right error) error { return errors.Join(left, right) }

func (p *datalogParser) parseInclude(domains bool) error {
	p.advance()
	if p.token.kind != datalogString {
		return p.fail("expected include file string")
	}
	path := p.includePath(p.token.text)
	p.advance()
	if p.token.kind == datalogNewline {
		p.advance()
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	// Includes share the declaration and domain dictionaries but have their own
	// lexer, matching the source's scoped lexer replacement.
	lexer, err := newDatalogLexer(file)
	if err != nil {
		return err
	}
	child := *p
	child.lexer = lexer
	child.path = path
	child.dir = p.includePath(".")
	child.lexer.parsingDomains = domains
	child.advance()
	if domains {
		for child.token.kind == datalogIdentifier || child.token.kind == datalogInclude {
			if child.token.kind == datalogInclude {
				err = child.parseInclude(true)
			} else {
				err = child.parseDomain()
			}
			if err != nil {
				return err
			}
		}
	}
	child.lexer.parsingDomains = false
	for child.token.kind != datalogEOF {
		if child.token.kind == datalogNewline {
			child.advance()
			continue
		}
		if child.token.kind == datalogInclude {
			err = child.parseInclude(false)
		} else {
			err = child.parseDeclarationOrRule()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *datalogParser) loadMap(sort ast.ASTHandle, name string) (resultErr error) {
	file, err := os.Open(p.includePath(name))
	if err != nil {
		// Source parse_mapfile treats an unavailable optional map as a warning
		// and continues. The Go parser has no diagnostic stream, so preserve
		// the acceptance behavior without writing process-global stderr.
		return nil
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	scanner := bufio.NewScanner(file)
	// Source line_reader grows its buffer to the complete map entry. Avoid
	// Scanner's unrelated 64 KiB token ceiling while retaining line semantics.
	scanner.Buffer(make([]byte, 1024), int(^uint(0)>>1))
	for scanner.Scan() {
		symbol, err := p.symbol(scanner.Text())
		if err != nil {
			return err
		}
		if _, err = p.context.SymbolNumber(sort, symbol); err != nil {
			return err
		}
	}
	return scanner.Err()
}
