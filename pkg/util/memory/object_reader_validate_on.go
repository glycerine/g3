//go:build g3checked || g3debug

package memory

// objectReaderValidate: see object_reader_validate_off.go. Checked and debug
// builds read every word through SmallAllocator.ReadUint32.
const objectReaderValidate = true
