package cmd_context

// Source: dparser::parse_mapfile. Each line of the map file, including the
// (possibly empty) text after its last newline, is numbered through
// context::get_constant_number.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *datalogParser) parseMapfile(tok dtoken, s ast.ASTHandle, filename string) (dtoken, error) {
	path := p.path + filename
	reader := newDatalogLineReader(path)

	if !reader.ok {
		if _, err := io.WriteString(p.errOut, "Warning: could not open file '"+path+"'.\n"); err != nil {
			return tok, err
		}
		return p.lexer.nextToken(), nil
	}

	for !reader.eof {
		if _, err := p.getConstantNumber(s, string(reader.getLine())); err != nil {
			return tok, err
		}
	}
	return p.lexer.nextToken(), nil
}
