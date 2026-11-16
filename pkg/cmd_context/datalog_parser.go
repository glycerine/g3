package cmd_context

// Source: src/muz/fp/datalog_parser.cpp dparser (parse_file, parse_string,
// reset, set_path, parse_stream).

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type datalogParser struct {
	context *Context
	manager *ast.ASTManager
	// owner is the datalog context's sort domains: register_finite_sort and
	// get_constant_number.
	owner *base.DatalogContext
	lexer *dlexer

	numVars  uint32
	vars     map[string]ast.ASTHandle
	path     string
	sortDict map[string]ast.ASTHandle
	// predicates is context::m_preds_by_name, for try_get_predicate_decl.
	predicates map[string]ast.ASTHandle
	// errOut is get_err(), std::cerr.
	errOut io.Writer
	pins   []ast.ASTRef
}

func newDatalogParser(context *Context, errOut io.Writer) (*datalogParser, error) {
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	if context.datalogDisplay == nil {
		owner, err := base.NewDatalogContext(manager)
		if err != nil {
			return nil, err
		}
		context.datalogDisplay = owner
	}
	if errOut == nil {
		errOut = io.Discard
	}
	p := &datalogParser{context: context, manager: manager, owner: context.datalogDisplay, sortDict: make(map[string]ast.ASTHandle), predicates: make(map[string]ast.ASTHandle), vars: make(map[string]ast.ASTHandle), errOut: errOut}
	// Predicates registered by an earlier parse of the same context are
	// found by name, as context::try_get_predicate_decl finds them.
	engine, err := context.fixedpointContext()
	if err != nil {
		return nil, err
	}
	engine.SetDatalogConstantNames(context.datalogDisplay)
	declarations, err := engine.Predicates()
	if err != nil {
		return nil, err
	}
	for _, declaration := range declarations {
		name, err := manager.GetASTName(declaration)
		if err != nil {
			return nil, err
		}
		text, err := manager.Symbols.String(name)
		if err != nil {
			return nil, err
		}
		p.predicates[text] = declaration
	}
	return p, nil
}

func (p *datalogParser) close() (err error) {
	for i := len(p.pins) - 1; i >= 0; i-- {
		err = errors.Join(err, p.manager.DelRef(&p.pins[i]))
	}
	p.pins = nil
	return err
}

// hold is an expr_ref/app_ref assignment (inc_ref); release its
// destructor (dec_ref), which may delete the node.
func (p *datalogParser) hold(h ast.ASTHandle) (ast.ASTRef, error) {
	return p.manager.NewRef(h)
}

func (p *datalogParser) release(r *ast.ASTRef) error {
	if *r == (ast.ASTRef{}) {
		return nil
	}
	return p.manager.DelRef(r)
}

// releaseAll is ~ref_vector: dec_ref front to back.
func (p *datalogParser) releaseAll(refs []ast.ASTRef) (err error) {
	for i := range refs {
		err = errors.Join(err, p.release(&refs[i]))
	}
	return err
}

func (p *datalogParser) pin(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := p.manager.NewRef(h)
	if err == nil {
		p.pins = append(p.pins, r)
	}
	return h, err
}

// parseFile is dparser::parse_file for a named file.
func (p *datalogParser) parseFile(filename string) (bool, error) {
	p.reset()
	p.setPath(filename)
	reader := newDatalogCharReader(filename)
	if !reader.ok() {
		_, err := io.WriteString(p.errOut, "ERROR: could not open file '"+filename+"'.\n")
		return false, err
	}
	return p.parseStream(nil, reader)
}

// parseString is dparser::parse_string.
func (p *datalogParser) parseString(input string) (bool, error) {
	p.reset()
	return p.parseStream(&datalogStreamInput{data: []byte(input)}, nil)
}

func (p *datalogParser) reset() {
	p.numVars = 0
	p.vars = make(map[string]ast.ASTHandle)
	p.path = ""
	p.sortDict = make(map[string]ast.ASTHandle)
}

func (p *datalogParser) setPath(filename string) {
	div := strings.LastIndexByte(filename, '/')
	if div < 0 {
		div = strings.LastIndexByte(filename, '\\')
	}
	if div >= 0 {
		p.path = filename[:div+1]
	}
}

// parseStream is dparser::parse_stream. A z3_exception (datalogException)
// is printed with std::endl and makes the parse fail; other Go errors are
// internal failures.
func (p *datalogParser) parseStream(is *datalogStreamInput, r *datalogCharReader) (bool, error) {
	p.lexer = newDlexer()
	p.lexer.setStream(is, r)
	tok := p.lexer.nextToken()
	tok, err := p.parseDomains(tok)
	if err == nil {
		tok, err = p.parseDecls(tok)
	}
	if err != nil {
		var exception *datalogException
		if errors.As(err, &exception) {
			_, err = io.WriteString(p.errOut, exception.msg+"\n")
			return false, err
		}
		return false, err
	}
	// m_error is never set (unexpected throws), so only the token decides.
	return tok == dtkEOS, nil
}

// datalogParseFailure is a false result of parse_file/parse_string for the
// error-returning API: the text the parser printed to std::cerr.
type datalogParseFailure struct{ printed string }

func (e *datalogParseFailure) Error() string {
	if e.printed == "" {
		return "datalog: failed to parse"
	}
	return strings.TrimRight(e.printed, "\n")
}

func (c *Context) parseDatalog(errOut io.Writer, run func(*datalogParser) (bool, error)) (ok bool, resultErr error) {
	p, err := newDatalogParser(c, errOut)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.close()) }()
	ok, err = run(p)
	if err != nil || !ok {
		return false, err
	}
	return true, c.rememberDatalogDisplay(p)
}

// ParseDatalogFileTo is parser::parse_file: the messages the C++ parser
// prints to std::cerr go to errOut, and ok is the C++ result. err reports
// only internal failures.
func (c *Context) ParseDatalogFileTo(filename string, errOut io.Writer) (ok bool, err error) {
	return c.parseDatalog(errOut, func(p *datalogParser) (bool, error) { return p.parseFile(filename) })
}

// ParseDatalogStringTo is parser::parse_string, reporting like
// ParseDatalogFileTo.
func (c *Context) ParseDatalogStringTo(input string, errOut io.Writer) (ok bool, err error) {
	return c.parseDatalog(errOut, func(p *datalogParser) (bool, error) { return p.parseString(input) })
}

// ParseDatalogString parses the legacy Datalog syntax and adds declarations,
// facts and Horn rules to this context's fixedpoint engine. A failed parse
// is an error carrying the parser's message.
func (c *Context) ParseDatalogString(input string) error {
	var printed bytes.Buffer
	ok, err := c.ParseDatalogStringTo(input, &printed)
	if err == nil && !ok {
		err = &datalogParseFailure{printed: printed.String()}
	}
	return err
}

// ParseDatalogFile is ParseDatalogString for a file; includes and map files
// are resolved against the file's directory prefix.
func (c *Context) ParseDatalogFile(filename string) error {
	var printed bytes.Buffer
	ok, err := c.ParseDatalogFileTo(filename, &printed)
	if err == nil && !ok {
		err = &datalogParseFailure{printed: printed.String()}
	}
	return err
}
