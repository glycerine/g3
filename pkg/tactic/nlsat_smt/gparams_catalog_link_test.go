package nlsatsmt

// These integration tests initialize the complete source parameter catalog.
// Import its collector owners so their package initializers register the
// actual collectors after the source tree was divided into Go packages.
import (
	_ "github.com/glycerine/g3/pkg/ackermannization"
	_ "github.com/glycerine/g3/pkg/ast/fpa"
	_ "github.com/glycerine/g3/pkg/ast/normal_forms"
	_ "github.com/glycerine/g3/pkg/ast/rewriter"
	_ "github.com/glycerine/g3/pkg/math/realclosure"
	_ "github.com/glycerine/g3/pkg/muz/base"
	_ "github.com/glycerine/g3/pkg/nlsat"
	_ "github.com/glycerine/g3/pkg/opt"
	_ "github.com/glycerine/g3/pkg/parsers/util"
	_ "github.com/glycerine/g3/pkg/sat"
	_ "github.com/glycerine/g3/pkg/solver"
	_ "github.com/glycerine/g3/pkg/tactic/sls"
	_ "github.com/glycerine/g3/pkg/tactic/smtlogics/params"
	_ "github.com/glycerine/g3/pkg/util/lp"
)
