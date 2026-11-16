//go:build !g3checked && !g3debug

package memory

// objectReaderValidate selects whether SmallObjectReader re-validates the
// allocator, the object and its runtime block on every word (checked and
// debug builds) or once when the reader is made (normal builds).
const objectReaderValidate = false
