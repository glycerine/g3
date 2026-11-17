package api

// Z3_fixedpoint_* call sequences (test_vectors/fixedpoint_seq/seq.c reads
// the same files): runFixedpointSequence makes the g3 API calls the C
// driver makes, in its order, and prints what it prints.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const fixedpointSequenceEnv = "G3_API_FIXEDPOINT_SEQUENCE"

type fixedpointSequence struct {
	t      *testing.T
	f      *apiFixedpointFixture
	params *ParameterSet
	out    strings.Builder
	sorts  map[string]ast.ASTHandle
	vars   []string
	vsorts []ast.ASTHandle
	rels   map[string]ast.ASTHandle
	line   string
}

// fail prints the error as seq.c's error handler does.
func (s *fixedpointSequence) fail(err error) bool {
	if err == nil {
		return false
	}
	// Z3_CATCH: context::handle_exception, then the handler's
	// Z3_get_error_msg.
	results := &apiContextResults{}
	results.handleAPIError(err)
	fmt.Fprintf(&s.out, "error: %s\n", results.errorMessage(results.errorCode))
	return true
}

func (s *fixedpointSequence) pin(h ast.ASTHandle, err error) ast.ASTHandle {
	if s.fail(err) {
		return ast.ASTHandle{}
	}
	return s.f.pin(h, nil)
}

func (s *fixedpointSequence) token() string {
	s.line = strings.TrimLeft(s.line, " \t")
	i := strings.IndexAny(s.line, " \t()\n")
	if i < 0 {
		i = len(s.line)
	}
	t := s.line[:i]
	s.line = s.line[i:]
	return t
}

func (s *fixedpointSequence) sort(name string) ast.ASTHandle {
	h, ok := s.sorts[name]
	if !ok {
		s.t.Fatalf("no sort %s", name)
	}
	return h
}

func (s *fixedpointSequence) rel(name string) ast.ASTHandle {
	h, ok := s.rels[name]
	if !ok {
		s.t.Fatalf("no relation %s", name)
	}
	return h
}

// expr is seq.c's expr(): arguments are built first to last.
func (s *fixedpointSequence) expr() ast.ASTHandle {
	c := s.f.c
	s.line = strings.TrimLeft(s.line, " \t")
	if strings.HasPrefix(s.line, "(") {
		s.line = s.line[1:]
		op := s.token()
		var args []ast.ASTHandle
		for {
			s.line = strings.TrimLeft(s.line, " \t")
			if strings.HasPrefix(s.line, ")") {
				s.line = s.line[1:]
				break
			}
			args = append(args, s.expr())
		}
		switch op {
		case "and":
			return s.pin(c.And(args...))
		case "or":
			return s.pin(c.Or(args...))
		case "not":
			return s.pin(c.Not(args[0]))
		case "=>":
			return s.pin(c.Implies(args[0], args[1]))
		case "=":
			return s.pin(c.Equal(args[0], args[1]))
		}
		return s.pin(c.Apply(s.rel(op), args...))
	}
	t := s.token()
	switch t {
	case "true":
		return s.pin(c.Bool(true))
	case "false":
		return s.pin(c.Bool(false))
	}
	if i := strings.IndexByte(t, ':'); i >= 0 {
		v, err := strconv.ParseUint(t[:i], 10, 32)
		if err != nil {
			s.t.Fatal(err)
		}
		return s.pin(c.UintNumeral(v, s.sort(t[i+1:])))
	}
	for i, name := range s.vars {
		if name == t {
			return s.pin(c.BoundVariable(uint32(i), s.vsorts[i]))
		}
	}
	return s.pin(c.Apply(s.rel(t)))
}

func (s *fixedpointSequence) answer() {
	f := s.f
	answer, err := f.fp.LastAnswer()
	if s.fail(err) || answer == (ast.ASTRef{}) {
		fmt.Fprintf(&s.out, "null\n")
		return
	}
	h, err := f.m.RefGet(answer)
	astTestMust(s.t, err)
	text, err := f.c.ASTString(h, APIASTPrintSMTFull)
	astTestMust(s.t, err)
	// Z3_fixedpoint_get_answer's result stays in the context's trail.
	*f.trail = append(*f.trail, answer)
	fmt.Fprintf(&s.out, "%s\n", text)
}

func (s *fixedpointSequence) result(status lbool.LBool, err error) {
	s.fail(err)
	r := 0
	switch status {
	case lbool.True:
		r = 1
	case lbool.False:
		r = -1
	}
	fmt.Fprintf(&s.out, "result %d\n", r)
	if status != lbool.Undef {
		s.answer()
		return
	}
	reason, err := s.f.fp.ReasonUnknown()
	astTestMust(s.t, err)
	fmt.Fprintf(&s.out, "reason %s\n", reason)
}

// runFixedpointSequence runs one sequence file and returns seq.c's output.
func runFixedpointSequence(t *testing.T, path string) string {
	return runFixedpointSequenceTo(t, path, nil)
}

// runFixedpointSequenceTo also writes each command's output to echo as it
// completes (seq.c's line-buffered stdout, between warnings on stderr).
func runFixedpointSequenceTo(t *testing.T, path string, echo io.Writer) string {
	data, err := os.ReadFile(path)
	astTestMust(t, err)
	sf := newAPIProgramFixture(t)
	fixedpoint, err := sf.c.NewFixedpoint()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, fixedpoint.Close()) })
	params, err := sf.c.NewParameterSet()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, params.Close()) })
	s := &fixedpointSequence{t: t, f: &apiFixedpointFixture{smtAPIFixture: sf, fp: fixedpoint}, params: params,
		sorts: make(map[string]ast.ASTHandle), rels: make(map[string]ast.ASTHandle)}
	c, fp := sf.c, fixedpoint
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	done := 0
	flush := func() {
		if echo != nil {
			io.WriteString(echo, s.out.String()[done:])
			done = s.out.Len()
		}
	}
	defer flush()
	for sc.Scan() {
		flush()
		s.line = sc.Text()
		cmd := s.token()
		if cmd == "" || strings.HasPrefix(cmd, ";") {
			continue
		}
		fmt.Fprintf(&s.out, "> %s\n", sc.Text())
		flush()
		switch cmd {
		case "sort":
			name, kind, arg := s.token(), s.token(), s.token()
			var h ast.ASTHandle
			switch kind {
			case "bv":
				w, err := strconv.ParseUint(arg, 10, 32)
				astTestMust(t, err)
				h = s.pin(c.BitVectorSort(uint32(w)))
			case "fin":
				n, err := strconv.ParseUint(arg, 10, 64)
				astTestMust(t, err)
				h = s.pin(c.FiniteDomainSort(name, n))
			case "int":
				h = s.pin(c.IntegerSort())
			default:
				h = s.pin(c.BoolSort())
			}
			s.sorts[name] = h
		case "vars":
			for {
				name := s.token()
				if name == "" {
					break
				}
				s.vars = append(s.vars, name)
				s.vsorts = append(s.vsorts, s.sort(s.token()))
			}
		case "rel":
			name := s.token()
			var domain []ast.ASTHandle
			for {
				t := s.token()
				if t == "" {
					break
				}
				domain = append(domain, s.sort(t))
			}
			d := s.f.programDecl(name, domain)
			s.fail(fp.RegisterRelation(d))
			s.rels[name] = d
		case "param":
			k, v := s.token(), s.token()
			switch {
			case v == "true" || v == "false":
				s.fail(params.SetBool(k, v == "true"))
			case v != "" && v[0] >= '0' && v[0] <= '9':
				n, err := strconv.ParseUint(v, 10, 32)
				astTestMust(t, err)
				s.fail(params.SetUInt(k, uint32(n)))
			default:
				s.fail(params.SetSymbol(k, v))
			}
		case "setparams":
			s.fail(fp.SetParameters(params))
		case "repr":
			name, kind := s.token(), s.token()
			s.fail(fp.SetPredicateRepresentation(s.rel(name), kind))
		case "fact":
			d := s.rel(s.token())
			var values []uint32
			for {
				t := s.token()
				if t == "" {
					break
				}
				v, err := strconv.ParseUint(t, 10, 32)
				astTestMust(s.t, err)
				values = append(values, uint32(v))
			}
			s.fail(fp.AddTableFact(d, values))
		case "rule":
			name := s.token()
			body := s.expr()
			if len(s.vars) != 0 {
				body = s.pin(c.Forall(s.vsorts, s.vars, body))
			}
			s.fail(fp.AddRule(body, name))
		case "query":
			q := s.expr()
			s.result(fp.QueryExpression(q))
		case "qrel":
			var ds []ast.ASTHandle
			for {
				t := s.token()
				if t == "" {
					break
				}
				ds = append(ds, s.rel(t))
			}
			s.result(fp.QueryRelations(ds))
		case "push":
			s.fail(fp.Push())
		case "pop":
			s.fail(fp.Pop())
		case "cover":
			level, err := strconv.Atoi(s.token())
			astTestMust(t, err)
			d := s.rel(s.token())
			s.fail(fp.AddCover(level, d, s.expr()))
		case "delta":
			level, err := strconv.Atoi(s.token())
			astTestMust(t, err)
			delta, err := fp.CoverDelta(level, s.rel(s.token()))
			if s.fail(err) || delta == (ast.ASTRef{}) {
				fmt.Fprintf(&s.out, "null\n")
				continue
			}
			h, err := s.f.m.RefGet(delta)
			astTestMust(t, err)
			text, err := c.ASTString(h, APIASTPrintSMTFull)
			astTestMust(t, err)
			*s.f.trail = append(*s.f.trail, delta)
			fmt.Fprintf(&s.out, "%s\n", text)
		case "levels":
			n, err := fp.NumLevels(s.rel(s.token()))
			s.fail(err)
			fmt.Fprintf(&s.out, "levels %d\n", n)
		default:
			t.Fatalf("bad command %s", cmd)
		}
	}
	return s.out.String()
}

// TestAPIFixedpointSequence runs the sequence named by
// G3_API_FIXEDPOINT_SEQUENCE and prints its output (the sequence sweep's
// g3 side).
func TestAPIFixedpointSequence(t *testing.T) {
	path := os.Getenv(fixedpointSequenceEnv)
	if path == "" {
		t.Skip("set " + fixedpointSequenceEnv)
	}
	runFixedpointSequenceTo(t, path, os.Stdout)
}

// TestAPIFixedpointSequences: the sequences in test_vectors/fixedpoint_seq
// (whose AST events TestAPIASTEventOrder also compares) and in its
// outputs/ directory against the fixed fork's output of seq.c
// (<name>.txt): datalog with relation queries, representations, facts,
// parameter updates mid-session, push/pop and errors; BMC; PDR and Spacer
// with levels, cover deltas and covers.
func TestAPIFixedpointSequences(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("test_vectors", "fixedpoint_seq", "*.seq"))
	astTestMust(t, err)
	more, err := filepath.Glob(filepath.Join("test_vectors", "fixedpoint_seq", "outputs", "*.seq"))
	astTestMust(t, err)
	names = append(names, more...)
	if len(names) == 0 {
		t.Fatal("no sequences")
	}
	for _, name := range names {
		t.Run(filepath.Base(name), func(t *testing.T) {
			want, err := os.ReadFile(strings.TrimSuffix(name, ".seq") + ".txt")
			astTestMust(t, err)
			if got := runFixedpointSequence(t, name); got != string(want) {
				t.Fatalf("got\n%s\nwant\n%s", got, want)
			}
		})
	}
}
