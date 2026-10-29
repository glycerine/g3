package cmd_context

// Source: wpa_parser_impl::finish_map_files and parse_rules_file.

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func (p *wpaParser) readRules() error {
	files, err := p.files("rules")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(p.sorts))
	for name := range p.sorts {
		names = append(names, name)
	}
	sort.Strings(names)
	var program strings.Builder
	for _, name := range names {
		size := uint64(0)
		switch name {
		case "BOOL":
			size = 2
		case "SHORT":
			size = 65536
		default:
			size = uint64(len(p.sorts[name].indices))
		}
		if size == 0 {
			size = 1
		}
		fmt.Fprintf(&program, "%s %d\n", name, size)
	}
	program.WriteByte('\n')
	for _, path := range files {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		program.Write(contents)
		if len(contents) == 0 || contents[len(contents)-1] != '\n' {
			program.WriteByte('\n')
		}
	}
	return p.context.ParseDatalogString(program.String())
}

func (p *wpaParser) indexDeclarations() error {
	engine, err := p.context.fixedpointContext()
	if err != nil {
		return err
	}
	declarations, err := engine.Predicates()
	if err != nil {
		return err
	}
	for _, declaration := range declarations {
		name, getErr := p.manager.GetASTName(declaration)
		if getErr != nil {
			return getErr
		}
		text, getErr := p.manager.Symbols.String(name)
		if getErr != nil {
			return getErr
		}
		p.declarations[text] = declaration
	}
	return nil
}
