package fp

import "github.com/glycerine/g3/pkg/ast"

// InstallContextModelConverter puts the engine in the state an earlier
// context::query leaves datalog::context in: m_mc is no longer null, so later
// relation queries record and apply their transformations' model converters.
func InstallContextModelConverter(e *PropositionalEngine) {
	e.contextModelConverter = true
}

// ContextModelConverterInstalled reports whether a context::query has
// installed m_mc.
func ContextModelConverterInstalled(e *PropositionalEngine) bool {
	return e.contextModelConverter
}

// EngineManager returns the engine's AST manager.
func EngineManager(e *PropositionalEngine) *ast.ASTManager { return e.manager }
