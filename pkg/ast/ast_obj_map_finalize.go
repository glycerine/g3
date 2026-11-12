package ast

// Finalize is core_hashtable::finalize (src/util/hashtable.h): a table
// larger than SMALL_TABLE_CAPACITY is replaced by an empty one of that
// capacity; a smaller one is reset.
func (o *ObjMap) Finalize() {
	if len(o.table) > objMapSmallCapacity {
		o.table = make([]ObjMapEntry, objMapSmallCapacity)
		o.size, o.numDeleted = 0, 0
		return
	}
	o.Reset()
}
