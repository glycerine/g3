package api

// Source: engine_base::get_model in src/muz/base/dl_engine_base.h. The API
// result facade must preserve the inherited default for relation and expression
// queries, including successful unknown and reachable results.

import (
	"testing"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointLastDefaultModel(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc", "tab", "clp", "ddnf"} {
		for _, expression := range []bool{false, true} {
			name := backend + "/relation"
			if expression {
				name = backend + "/expression"
			}
			t.Run(name, func(t *testing.T) {
				f := newAPIFixedpointFixture(t)
				astTestMust(t, f.c.SetFixedpointEngine(backend))
				if md, err := f.fp.LastModel(); md != nil || err == nil {
					t.Fatal("pre-query default model", md, err)
				}
				astTestMust(t, f.fp.AddRule(f.pAtom, "default-model-fact"))
				var saved []*model.Model
				defer func() {
					for _, md := range saved {
						astTestMust(t, md.Close())
					}
				}()
				checkEmpty := func(md *model.Model) {
					t.Helper()
					n, err := md.Core().NumDeclarations()
					astTestMust(t, err)
					if n != 0 {
						t.Fatal("inherited model acquired interpretations", n)
					}
				}
				for _, reachable := range []bool{false, true} {
					query, app, want := f.u, f.uAtom, lbool.False
					if reachable {
						query, app, want = f.p, f.pAtom, lbool.True
					}
					if backend == "ddnf" {
						want = lbool.Undef
					}
					var status lbool.LBool
					var err error
					if expression {
						status, err = f.fp.QueryExpression(app)
					} else {
						status, err = f.fp.Query(query)
					}
					astTestMust(t, err)
					if status != want {
						t.Fatal("query result", status, want)
					}
					for repeat := 0; repeat < 2; repeat++ {
						md, err := f.fp.LastModel()
						astTestMust(t, err)
						for _, previous := range saved {
							if previous == md || previous.Core() == md.Core() {
								t.Fatal("model accessor reused a caller owner")
							}
						}
						saved = append(saved, md)
						checkEmpty(md)
					}
				}
				astTestMust(t, f.fp.AddRule(f.pAtom, "invalidate-default-model"))
				if md, err := f.fp.LastModel(); md != nil || err == nil {
					t.Fatal("stale default model", md, err)
				}
				astTestMust(t, f.fp.Close())
				for _, md := range saved {
					checkEmpty(md)
				}
				if md, err := f.fp.LastModel(); md != nil || err == nil {
					t.Fatal("closed default model accessor", md, err)
				}
			})
		}
	}
}
