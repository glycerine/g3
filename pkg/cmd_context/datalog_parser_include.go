package cmd_context

// Source: dparser::parse_include. The included file is named relative to
// the directory prefix of the top-level file (m_path).

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *datalogParser) parseInclude(filename string, parsingDomain bool) (dtoken, error) {
	// IF_VERBOSE(2, ...) progress line: g3trace builds only (owner decision
	// 2026-11-15).
	if diagnostic.VerboseProgress {
		if err := diagnostic.ProgressVerbose(2, "include: "+filename+"\n"); err != nil {
			return dtkError, err
		}
	}
	path := p.path + filename
	stream := newDatalogCharReader(path)
	if !stream.ok() {
		_, err := io.WriteString(p.errOut, "ERROR: could not open file '"+path+"'.\n")
		return dtkError, err
	}
	lexer := newDlexer()
	outer := p.lexer
	p.lexer = lexer
	tok, err := func() (dtoken, error) {
		defer func() { p.lexer = outer }()
		p.lexer.setStream(nil, stream)
		tok := p.lexer.nextToken()
		var err error
		if parsingDomain {
			if tok, err = p.parseDomains(tok); err != nil {
				return tok, err
			}
		}
		return p.parseDecls(tok)
	}()
	if err != nil {
		return tok, err
	}
	if tok == dtkEOS {
		tok = p.lexer.nextToken()
	}
	return tok, nil
}
