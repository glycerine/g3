//go:build !g3checked && !g3debug

package ast

// AST read validation switch (see ast_validate_handles_on.go for g3checked
// and g3debug builds).
//
// In normal builds the hot read accessors read the node and app records
// directly, as z3 dereferences its AST pointers: they skip lock()'s manager
// state checks (Self, trace/format re-entry, Closed, constructor state) and
// the handle's origin, slot range, liveness and generation checks. Kind and
// index checks stay (GetNumArgs of a non-application is still ErrASTDomain,
// GetArg out of range too), a null handle is still ErrASTHandle, and pending
// node deletions still run first. A stale handle, a handle of another
// manager or a closed manager is a caller bug: it reads another node's
// fields or panics, and only g3checked/g3debug builds report it.
const astValidateHandles = false
