package protomodel

// Source: src/model/model_pp.cpp model_pp on a proto_model, whose
// get_num_uninterpreted_sorts/get_uninterpreted_sort/get_universe come from
// the user sort factory (src/smt/proto_model/proto_model.cpp).

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// WriteDebug is model_pp(out, *this).
func (p *ProtoModel) WriteDebug(out io.Writer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) error {
	if err := p.open(); err != nil {
		return err
	}
	sorts := p.user.Sorts()
	universes := make([][]ast.ASTHandle, len(sorts))
	for i, s := range sorts {
		universes[i] = p.user.KnownUniverse(s)
	}
	return p.core.Core().WriteDebugUniverses(out, context, globals, sorts, universes)
}
