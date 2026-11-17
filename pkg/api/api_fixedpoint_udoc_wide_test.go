package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// TestAPIFixedpointUDocWideFacts is test_vectors/udoc_wide/wide.c: facts 5
// and 7 (Z3_fixedpoint_add_fact) of a bit-vector relation under a doc (or
// hashtable) favourite, then Z3_fixedpoint_query_relations. Wider than a
// table element, the relation is udoc_relation (relation_manager::
// mk_empty_relation) on the compiled route. The .txt files are the fixed
// fork's output: z3 4.7.1 sets bits above 63 by an undefined shift
// (tbv_manager::set, FORK FIX #113).
func TestAPIFixedpointUDocWideFacts(t *testing.T) {
	for _, spec := range []struct {
		bits     uint32
		relation string
	}{{160, "doc"}, {65, "doc"}, {64, "doc"}, {8, "doc"}, {160, "hashtable"}, {200, "doc"}} {
		name := fmt.Sprintf("wide_%d_%s.txt", spec.bits, spec.relation)
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("test_vectors", "udoc_wide", name))
			astTestMust(t, err)
			if got := runUDocWide(t, spec.bits, spec.relation); got != string(want) {
				t.Fatalf("g3:\n%s\nfixed fork:\n%s", got, want)
			}
		})
	}
}

// runUDocWide is wide.c <bits> <relation>.
func runUDocWide(t *testing.T, bits uint32, relation string) string {
	sf := newAPIProgramFixture(t)
	fixedpoint, err := sf.c.NewFixedpoint()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, fixedpoint.Close()) })
	f := &apiFixedpointFixture{smtAPIFixture: sf, fp: fixedpoint}
	params, err := f.c.NewParameterSet()
	astTestMust(t, err)
	defer func() { astTestMust(t, params.Close()) }()
	astTestMust(t, params.SetSymbol("engine", "datalog"))
	astTestMust(t, params.SetSymbol("datalog.default_relation", relation))
	astTestMust(t, f.fp.SetParameters(params))
	bv := f.pin(f.c.BitVectorSort(bits))
	p := f.programDecl("p", []ast.ASTHandle{bv})
	astTestMust(t, f.fp.RegisterRelation(p))
	astTestMust(t, f.fp.AddTableFact(p, []uint32{5}))
	astTestMust(t, f.fp.AddTableFact(p, []uint32{7}))
	status, err := f.fp.QueryRelations([]ast.ASTHandle{p})
	astTestMust(t, err)
	var out strings.Builder
	r := 0
	switch status {
	case lbool.True:
		r = 1
	case lbool.False:
		r = -1
	}
	fmt.Fprintf(&out, "result %d\n", r)
	answer, err := f.fp.LastAnswer()
	astTestMust(t, err)
	h, err := f.m.RefGet(answer)
	astTestMust(t, err)
	text, err := f.c.ASTString(h, APIASTPrintSMTFull)
	astTestMust(t, err)
	*f.trail = append(*f.trail, answer)
	fmt.Fprintf(&out, "%s\n", text)
	return out.String()
}
