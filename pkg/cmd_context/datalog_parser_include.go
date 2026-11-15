package cmd_context

// Source: dparser::parse_include with source-relative nested resolution.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *datalogParser) parseInclude(domains bool) error {
	if err := p.advance(); err != nil {
		return err
	}
	if p.current.kind != dlString {
		return p.expected("include filename string")
	}
	filename := p.current.text
	// IF_VERBOSE(2, "include: " << filename) (AUDIT.md FUZZ-F5; g3trace only).
	if err := datalogParserVerbose(2, "include: "+filename+"\n"); err != nil {
		return err
	}
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(p.path, filename)
	}
	input, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("datalog include %s: %w", filename, err)
	}
	outerLexer, outerCurrent, outerPath := p.lexer, p.current, p.path
	p.lexer, p.path = newDatalogLexer(input), filepath.Dir(filename)
	p.lexer.parsingDomains = domains
	if err = p.advance(); err == nil {
		if domains {
			err = p.parseDomains()
		} else {
			err = p.parseRules()
		}
	}
	p.lexer, p.current, p.path = outerLexer, outerCurrent, outerPath
	if err != nil {
		return err
	}
	if err = p.advance(); err != nil {
		return err
	}
	if domains && p.current.kind == dlNewline {
		return p.advance()
	}
	return nil
}

// datalogParserVerbose is IF_VERBOSE(level, verbose_stream() << text) for
// the datalog/WPA parsers, compiled only into g3trace builds (AUDIT.md
// FUZZ-F5).
func datalogParserVerbose(level uint32, text string) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	return diagnostic.ProgressVerbose(level, text)
}
