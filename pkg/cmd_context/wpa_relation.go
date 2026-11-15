package cmd_context

// Source: wpa_parser_impl::parse_rel_file and inp_num_to_element.

import (
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (p *wpaParser) readRelations() error {
	for _, path := range p.files("rel") {
		// skip relations which we do not support yet (the source tests the
		// whole path, not only the file name)
		if strings.Contains(path, "DirectCall") || strings.Contains(path, "FunctionFormals") || strings.Contains(path, "IndirectCall") {
			continue
		}
		if err := p.parseRelFile(path); err != nil {
			return err
		}
	}
	return nil
}

// parseRelFile is parse_rel_file.
func (p *wpaParser) parseRelFile(fname string) error {
	if err := wpaVerbose("Parsing relation file " + fname + "\n"); err != nil {
		return err
	}
	p.currentFile, p.currentLine = fname, 0
	predicateName := wpaBase(fname)
	declaration := p.declarations[predicateName]
	if declaration.IsNull() {
		return fmt.Errorf("tuple file %s for undeclared predicate %s", p.currentFile, predicateName)
	}
	arity, err := p.manager.GetArity(declaration)
	if err != nil {
		return err
	}
	lines, err := readWPALines(fname)
	if err != nil {
		return err
	}
	for _, line := range lines {
		p.currentLine++
		values, present, rest, parseErr := parseWPARelationLine(line)
		switch parseErr {
		case nil:
		case errWPANumberExpected:
			return fmt.Errorf("number expected on line %d in file %s", p.currentLine, p.currentFile)
		case errWPASpaceExpected:
			return fmt.Errorf("' ' expected to separate numbers on line %d in file %s, got '%s'", p.currentLine, p.currentFile, rest)
		default:
			return parseErr
		}
		if !present {
			continue
		}
		if len(values) != int(arity) {
			return fmt.Errorf("invalid number of arguments on line %d in file %s", p.currentLine, p.currentFile)
		}
		fact := make([]uint32, len(values))
		factFail := false
		for i, value := range values {
			sortHandle, getErr := p.manager.GetDomain(declaration, uint32(i))
			if getErr != nil {
				return getErr
			}
			converted, found, getErr := p.inpNumToElement(sortHandle, value)
			if getErr != nil {
				return getErr
			}
			if !found {
				factFail = true
				break
			}
			fact[i] = converted
		}
		if factFail {
			continue
		}
		if err = p.context.AddFixedpointTableFact(declaration, fact); err != nil {
			return err
		}
	}
	return nil
}

// mkNumericTableConst is mk_table_const(uint64_t, s): uint64_sort_domain
// get_number numbers values by first use.
func (p *wpaParser) mkNumericTableConst(sortHandle ast.ASTHandle, number uint64) (uint32, error) {
	index, err := p.rules.owner.Uint64Number(sortHandle, number)
	return uint32(index), err
}

func (p *wpaParser) sortName(sortHandle ast.ASTHandle) (string, error) {
	name, err := p.manager.GetASTName(sortHandle)
	if err != nil {
		return "", err
	}
	return p.manager.Symbols.String(name)
}

// inpNumToElement is inp_num_to_element; found=false is its false return
// after the "does not belong to sort" warning.
func (p *wpaParser) inpNumToElement(sortHandle ast.ASTHandle, number uint64) (uint32, bool, error) {
	if sortHandle == p.boolSort || sortHandle == p.shortSort {
		index, err := p.mkNumericTableConst(sortHandle, number)
		return index, err == nil, err
	}
	if number == 0 {
		if !p.useMapNames {
			index, err := p.mkNumericTableConst(sortHandle, 0)
			return index, err == nil, err
		}
		index, err := p.rules.getConstantNumber(sortHandle, "<zero element>")
		return uint32(index), err == nil, err
	}
	text, err := p.sortName(sortHandle)
	if err != nil {
		return 0, false, err
	}
	// Every WPA sort but BOOL and SHORT comes from a map file (rules files
	// cannot declare domains), so the source's find_core always succeeds.
	entry := p.sorts[text]
	if entry == nil {
		return 0, false, fmt.Errorf("unknown WPA sort %s", text)
	}
	if !entry.numbers[number] {
		// z3 4.7.1 formats the number with MSVC's %I64u, which glibc reads
		// as a 64-wide %u of a 32-bit argument (undefined behavior). The
		// fork prints it with PRIu64 (FORK FIX #67).
		return 0, false, diagnostic.BorrowProcessWarningService().Warningf("symbol number %d on line %d in file %s does not belong to sort %s", number, p.currentLine, p.currentFile, text)
	}
	if !p.useMapNames {
		index, err := p.mkNumericTableConst(sortHandle, number)
		return index, err == nil, err
	}
	constant, found := p.mapNames[number]
	if !found {
		return 0, false, fmt.Errorf("unknown symbol number %d on line %d in file %s", number, p.currentLine, p.currentFile)
	}
	index, err := p.rules.getConstantNumber(sortHandle, constant)
	return uint32(index), err == nil, err
}
