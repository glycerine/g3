//go:build g3checked || g3debug

package ast

// AST read validation switch (see ast_validate_handles_off.go for the
// normal build).
//
// In g3checked and g3debug builds the hot read accessors (GetKind,
// GetNumArgs, GetArg, GetDecl, GetRefCount, GetFamilyID, GetDeclKind,
// NodeHeader, GetSort, IsGround, ...) validate every call as before: lock()'s
// manager state checks, then the handle's origin, slot range, liveness and
// generation (internal/validNode). Stale or foreign handles return
// ErrASTHandle and a closed or re-entered manager its lock() error.
const astValidateHandles = true
