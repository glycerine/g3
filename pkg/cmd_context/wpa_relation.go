package cmd_context

// Source: wpa_parser_impl::parse_rel_file and inp_num_to_element.

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *wpaParser) readRelations() error {
	files, err := p.files("rel")
	if err != nil {
		return err
	}
	for _, path := range files {
		name := wpaBase(path)
		if strings.Contains(name, "DirectCall") || strings.Contains(name, "FunctionFormals") || strings.Contains(name, "IndirectCall") {
			continue
		}
		declaration := p.declarations[name]
		if declaration.IsNull() {
			return fmt.Errorf("tuple file %s has undeclared predicate %s", path, name)
		}
		if err = p.readRelation(path, declaration); err != nil {
			return err
		}
	}
	return nil
}

func (p *wpaParser) readRelation(path string, declaration ast.ASTHandle) error {
	arity, err := p.manager.GetArity(declaration)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		values, present, parseErr := parseWPARelationLine(scanner.Text())
		if parseErr != nil {
			file.Close()
			return fmt.Errorf("%s:%d: %w", path, line, parseErr)
		}
		if !present {
			continue
		}
		if len(values) != int(arity) {
			file.Close()
			return fmt.Errorf("%s:%d: got %d elements, want %d", path, line, len(values), arity)
		}
		fact := make([]uint32, len(values))
		valid := true
		for i, value := range values {
			sortHandle, getErr := p.manager.GetDomain(declaration, uint32(i))
			if getErr != nil {
				file.Close()
				return getErr
			}
			converted, found, getErr := p.relationElement(sortHandle, value)
			if getErr != nil {
				file.Close()
				return getErr
			}
			if !found {
				valid = false
				break
			}
			fact[i] = converted
		}
		if valid {
			if err = p.context.AddFixedpointTableFact(declaration, fact); err != nil {
				file.Close()
				return err
			}
		}
	}
	err = scanner.Err()
	return joinClose(err, file.Close())
}

func (p *wpaParser) relationElement(sortHandle ast.ASTHandle, number uint64) (uint32, bool, error) {
	name, err := p.manager.GetASTName(sortHandle)
	if err != nil {
		return 0, false, err
	}
	text, err := p.manager.Symbols.String(name)
	if err != nil {
		return 0, false, err
	}
	entry := p.sorts[text]
	if entry == nil {
		return 0, false, fmt.Errorf("unknown WPA sort %s", text)
	}
	if text == "SHORT" {
		if number >= 65536 {
			return 0, false, nil
		}
		return uint32(number), true, nil
	}
	index, found := entry.indices[number]
	return index, found, nil
}
