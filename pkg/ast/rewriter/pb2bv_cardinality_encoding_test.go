package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp cardinality_encoding/updt_params.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPB2BVCardinalityEncodingProfileDispatch(t *testing.T) {
	tests := []struct {
		name string
		want ast.PsortASTEncoding
	}{{"grouped", ast.PsortASTGrouped}, {"bimander", ast.PsortASTBimander}, {"ordered", ast.PsortASTOrdered}, {"unate", ast.PsortASTUnate}, {"circuit", ast.PsortASTCircuit}, {"unknown", ast.PsortASTGrouped}}
	for _, test := range tests {
		m, r := pb2BVFixture(t)
		pb2BVSetProfile(t, r, "cardinality.encoding", test.name)
		if got, e := r.network.Encoding(); e != nil || got != test.want {
			t.Fatal(test.name, got, e)
		}
		args := []ast.ASTHandle{goalTestAtom(t, m, "encoding-a"), goalTestAtom(t, m, "encoding-b"), goalTestAtom(t, m, "encoding-c"), goalTestAtom(t, m, "encoding-d")}
		formula, e := m.MkPseudoBooleanAtMost(args, 1)
		astTestMust(t, e)
		out := astTestPin(t, m, ast.ASTHandle{})
		astTestMust(t, r.Rewrite(formula, out, ast.ASTRef{}))
		result, e := m.RefGet(out)
		astTestMust(t, e)
		clauses, e := r.FlushSideConstraints()
		astTestMust(t, e)
		oracle := pb2BVBooleanOracle{ids: map[ast.ASTHandle]int{}}
		rid := oracle.gate(t, m, result)
		for _, clause := range clauses {
			oracle.clauses = append(oracle.clauses, []int{oracle.gate(t, m, clause)})
		}
		for _, arg := range args {
			oracle.gate(t, m, arg)
		}
		for assignment := 0; assignment < 16; assignment++ {
			values := make([]int8, oracle.count+1)
			count := 0
			for i, arg := range args {
				values[oracle.ids[arg]] = -1
				if assignment&(1<<i) != 0 {
					values[oracle.ids[arg]] = 1
					count++
				}
			}
			values[rid] = 1
			if got := pb2BVOracleSAT(oracle.clauses, values); got != (count <= 1) {
				t.Fatal(test.name, assignment, count, got)
			}
		}
		for _, clause := range clauses {
			astTestMust(t, m.DecRef(clause))
		}
	}
}
