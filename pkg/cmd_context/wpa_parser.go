package cmd_context

// Source: src/muz/fp/datalog_parser.cpp wpa_parser_impl. The retained shell
// directory path loads map files, declarations/rules and relation tuples.

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type wpaParser struct {
	context      *Context
	manager      *ast.ASTManager
	directory    string
	mapNames     map[uint64]string
	sorts        map[string]*wpaSort
	declarations map[string]ast.ASTHandle
}

type wpaSort struct {
	name    string
	numbers map[uint64]bool
	indices map[uint64]uint32
}

func newWPAParser(context *Context, directory string) (*wpaParser, error) {
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	return &wpaParser{context: context, manager: manager, directory: directory, mapNames: make(map[uint64]string), sorts: make(map[string]*wpaSort), declarations: make(map[string]ast.ASTHandle)}, nil
}

// ParseWPADirectory parses the legacy directory benchmark selected by the
// Datalog shell when its input path is a directory.
func (c *Context) ParseWPADirectory(directory string) error {
	p, err := newWPAParser(c, directory)
	if err != nil {
		return err
	}
	if err = p.readMaps(); err != nil {
		return err
	}
	p.finishSorts()
	if err = p.readRules(); err != nil {
		return err
	}
	if err = p.indexDeclarations(); err != nil {
		return err
	}
	return p.readRelations()
}

func (p *wpaParser) finishSorts() {
	for _, entry := range p.sorts {
		numbers := make([]uint64, 0, len(entry.numbers))
		for number := range entry.numbers {
			if number != 0 {
				numbers = append(numbers, number)
			}
		}
		sort.Slice(numbers, func(i, j int) bool { return numbers[i] < numbers[j] })
		entry.indices = map[uint64]uint32{0: 0}
		for i, number := range numbers {
			entry.indices[number] = uint32(i + 1)
		}
	}
	p.sorts["BOOL"] = &wpaSort{name: "BOOL", indices: map[uint64]uint32{0: 0, 1: 1}}
	p.sorts["SHORT"] = &wpaSort{name: "SHORT", indices: nil}
}
