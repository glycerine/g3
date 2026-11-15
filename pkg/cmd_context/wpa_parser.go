package cmd_context

// Source: src/muz/fp/datalog_parser.cpp wpa_parser_impl. The retained shell
// directory path loads map files, declarations/rules and relation tuples.
//
// z3 4.7.1 cannot run this path outside Windows: dl_util.cpp
// get_file_names is NOT_IMPLEMENTED_YET there. The fork implements it for
// POSIX (FORK FIX #66, AUDIT.md cmd_context_02-F11) and this loader follows
// that fork.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type wpaParser struct {
	context   *Context
	manager   *ast.ASTManager
	directory string
	// mapNames is m_number_names (num2sym); only looked up, never iterated.
	mapNames map[uint64]string
	// sorts holds the sym2nums payloads; sortOrder is the sym2nums table
	// itself (map<symbol, uint64_set*>), whose iteration order
	// finish_map_files registers the sorts in.
	sorts        map[string]*wpaSort
	sortOrder    *symbolDictionary
	declarations map[string]ast.ASTHandle
	rules        *datalogParser
	useMapNames  bool
	// m_bool_sort and m_short_sort.
	boolSort, shortSort ast.ASTHandle
	// m_current_file and m_current_line, for the diagnostics.
	currentFile string
	currentLine int
}

type wpaSort struct {
	name    string
	numbers map[uint64]bool
	size    uint64
}

func newWPAParser(context *Context, directory string) (*wpaParser, error) {
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	engine, err := context.fixedpointContext()
	if err != nil {
		return nil, err
	}
	useNames, err := engine.DatalogUseMapNames()
	if err != nil {
		return nil, err
	}
	rules, err := newDatalogParser(context)
	if err != nil {
		return nil, err
	}
	return &wpaParser{context: context, manager: manager, directory: directory, mapNames: make(map[uint64]string), sorts: make(map[string]*wpaSort), sortOrder: newSymbolDictionary(), declarations: make(map[string]ast.ASTHandle), rules: rules, useMapNames: useNames}, nil
}

// ensureSortContent is ensure_sort_content: m_sort_contents
// insert_if_not_there2(sort_name, nullptr), then allocate the set.
func (p *wpaParser) ensureSortContent(name string) *wpaSort {
	p.sortOrder.insertIfNotThere(name)
	entry := p.sorts[name]
	if entry == nil {
		entry = &wpaSort{name: name, numbers: make(map[uint64]bool)}
		p.sorts[name] = entry
	}
	return entry
}

// wpaVerbose is IF_VERBOSE(10, ...); new progress lines are g3trace-only.
func wpaVerbose(text string) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	return diagnostic.ProgressVerbose(10, text)
}

// ParseWPADirectory parses the legacy directory benchmark selected by the
// Datalog shell when its input path is a directory. It is parse_directory:
// a returned error carries the z3_exception message the source prints.
func (c *Context) ParseWPADirectory(directory string) (resultErr error) {
	p, err := newWPAParser(c, directory)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.rules.close()) }()
	if err = p.parseDirectoryCore(); err != nil {
		return err
	}
	return c.rememberDatalogDisplay(p.rules)
}

// parseDirectoryCore is parse_directory_core.
func (p *wpaParser) parseDirectoryCore() error {
	if err := wpaVerbose("Start parsing directory " + p.directory + "\n"); err != nil {
		return err
	}
	if err := p.readMaps(); err != nil {
		return err
	}
	if err := p.finishSorts(); err != nil {
		return err
	}
	if err := p.readRules(); err != nil {
		return err
	}
	if err := p.indexDeclarations(); err != nil {
		return err
	}
	if err := p.readRelations(); err != nil {
		return err
	}
	return wpaVerbose("Done parsing directory " + p.directory + "\n")
}

// registerFiniteSort is dparser::register_finite_sort. numeric selects
// context::SK_UINT64, otherwise SK_SYMBOL.
func (p *wpaParser) registerFiniteSort(name string, size uint64, numeric bool) (ast.ASTHandle, error) {
	if _, found := p.rules.sorts[name]; found {
		return ast.ASTHandle{}, fmt.Errorf("sort %s already declared", name)
	}
	symbol, err := p.manager.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	root, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.rules.pins = append(p.rules.pins, root)
	handle, err := p.manager.MkDatalogFiniteSort(symbol, size)
	if err = errors.Join(err, p.manager.SetRef(root, handle)); err != nil {
		return ast.ASTHandle{}, err
	}
	p.rules.sorts[name] = handle
	if numeric {
		p.rules.numericConstants[handle] = make(map[uint64]uint64)
	} else {
		p.rules.constants[handle] = make(map[string]uint64)
	}
	return handle, p.rules.seedDatalogDomain(handle, numeric)
}

// finishSorts is finish_map_files: BOOL and SHORT first, then the map sorts
// in m_sort_contents table order. A map file named BOOL or SHORT fails with
// register_finite_sort's "already declared".
func (p *wpaParser) finishSorts() error {
	p.rules.numericConstants = make(map[ast.ASTHandle]map[uint64]uint64)
	var err error
	if p.boolSort, err = p.registerFiniteSort("BOOL", 2, true); err != nil {
		return err
	}
	if p.shortSort, err = p.registerFiniteSort("SHORT", 65536, true); err != nil {
		return err
	}
	for _, e := range p.sortOrder.entries() {
		entry := p.sorts[e.name]
		// the +1 is for a zero element which happens to appear in the problem files
		entry.size = uint64(len(entry.numbers)) + 1
		if _, err = p.registerFiniteSort(e.name, entry.size, !p.useMapNames); err != nil {
			return err
		}
	}
	return nil
}
