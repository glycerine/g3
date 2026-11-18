/*++
Copyright (c) 2026 Jason E. Aten, Ph.D. (insdict, MIT License)

Module Name:

    insdict.h

Abstract:

    Insertion-ordered hash set: a C++ port of the core of the Go package
    github.com/glycerine/insdict v0.14.6 (Dict, modeled on CPython's 3.7+
    compact dict).

    FORK FIX #140 (owner decision 2026-11-18, see FORK_BUGFIXES.md): this
    is a declared divergence from z3 4.7.1, not a bug fix. ast_table uses
    it instead of chashtable so that z3 and its Go port g3 iterate the AST
    table (teardown free order included) in the same order and report the
    same capacity.

    Layout (unchanged from the Go insdict):

      m_entries   dense array of {hash, key, live} in insertion order;
                  erase leaves a hole (live == false) that is only
                  compacted away when the index is rebuilt.
      m_indices   open-addressed index of m_size slots (a power of two,
                  minimum 8): slot_empty, slot_dummy (erased), or an
                  index into m_entries.
      m_tags      one control byte per slot: tag_empty, tag_dummy, or
                  tag_used | the top 7 bits of the 64-bit hash.

    Probing is CPython's perturbed sequence i = (i*5 + perturb + 1) & mask,
    perturb >>= 5 each step. A new key is refused room once the entry
    count (holes included) reaches 2/3 of the index (usable()); the table
    is then rebuilt to sizeFor(live) slots, the smallest power of two
    >= 3*live, which drops holes and preserves insertion order. So both
    the capacity (index length) and the iteration order are functions of
    the insert/erase sequence alone, never of the hash values.

    Only the parts of Dict reached by z3 are ported: Put (insert,
    insert_if_not_there), Get (find, contains), Del (erase), Pack,
    DeleteAll (finalize/reset), Len (size), All (iteration). The sorted
    index, SlowWriteAll, Clone and the presized constructors are not.

    HashProc must return a uint64_t (well mixed in its high bits, which
    form the tags; insdict_mix64 is the Go package's Mix64). EqProc may be
    a structural equality, so a probe that is not itself in the table can
    find its equal. Key must be trivially copyable (entries live in raw
    memory::allocate storage); z3 instantiates it with pointers.

--*/
#ifndef INSDICT_H_
#define INSDICT_H_

#include <stdint.h>
#include <string.h>
#include <utility>
#include "util/memory_manager.h"
#include "util/debug.h"
#include "util/z3_exception.h"

// insdict.Mix64: the splitmix64 finalizer.
inline uint64_t insdict_mix64(uint64_t x) {
    x += 0x9e3779b97f4a7c15ull;
    x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9ull;
    x = (x ^ (x >> 27)) * 0x94d049bb133111ebull;
    return x ^ (x >> 31);
}

template<typename Key, typename HashProc, typename EqProc>
class insdict : private HashProc, private EqProc {
public:
    static const int           slot_empty = -1;
    static const int           slot_dummy = -2;
    static const unsigned      min_size   = 8;
    static const unsigned char tag_empty  = 0;
    static const unsigned char tag_dummy  = 1;
    static const unsigned char tag_used   = 0x80;

    struct entry {
        uint64_t m_hash;
        Key      m_key;
        bool     m_live;
    };

protected:
    int *           m_indices;      // m_size slots, or nullptr when m_size == 0
    unsigned char * m_tags;         // m_size control bytes
    unsigned        m_size;         // index length (a power of two >= min_size), or 0
    uint64_t        m_mask;
    entry *         m_entries;      // dense, insertion ordered, may contain holes
    unsigned        m_num_entries;  // len(entries), holes included
    unsigned        m_entries_cap;
    unsigned        m_live;         // live entry count

    uint64_t get_hash(Key const & k) const { return HashProc::operator()(k); }
    bool equals(Key const & a, Key const & b) const { return EqProc::operator()(a, b); }

    static unsigned char hash_tag(uint64_t h) {
        return static_cast<unsigned char>(tag_used | static_cast<unsigned char>(h >> 57));
    }

    // usable is the maximum entry count (holes included) before a rebuild.
    unsigned usable() const { return static_cast<unsigned>((static_cast<uint64_t>(m_size) * 2) / 3); }

    // sizeFor: the smallest power of two >= max(min_size, 3*live), so live
    // entries fill at most ~1/3 of the slots (~1/2 of usable()).
    static unsigned size_for(unsigned live) {
        if (live > (0x7fffffffu - 1) / 3)
            throw default_exception("insdict: entry count exceeds maximum table capacity");
        unsigned size = min_size;
        while (size < live * 3) {
            if (size > (0x7fffffffu >> 1))
                throw default_exception("insdict: table size overflow");
            size <<= 1;
        }
        return size;
    }

    // find returns the entries index of the key equal to k (and its slot),
    // or -1 and the empty slot where the probe ended. Requires m_size != 0.
    template<typename Eq>
    int find_core(Key const & k, uint64_t h, unsigned char tag, Eq const & eq, uint64_t & slot) const {
        uint64_t i = h & m_mask, perturb = h;
        while (m_tags[i] != tag_empty) {
            if (m_tags[i] == tag) {
                int ix = m_indices[i];
                entry const & e = m_entries[ix];
                if (e.m_hash == h && eq(e.m_key, k)) {
                    slot = i;
                    return ix;
                }
            }
            perturb >>= 5;
            i = (i * 5 + perturb + 1) & m_mask;
        }
        slot = i;
        return -1;
    }

    struct struct_eq {
        insdict const & d;
        struct_eq(insdict const & d): d(d) {}
        bool operator()(Key const & a, Key const & b) const { return d.equals(a, b); }
    };

    struct ident_eq {
        bool operator()(Key const & a, Key const & b) const { return a == b; }
    };

    // first empty-or-dummy slot on h's probe path (the key is known absent).
    uint64_t free_slot(uint64_t h) const {
        uint64_t i = h & m_mask, perturb = h;
        while (m_tags[i] >= tag_used) {
            perturb >>= 5;
            i = (i * 5 + perturb + 1) & m_mask;
        }
        return i;
    }

    void free_storage() {
        if (m_indices)
            memory::deallocate(m_indices);
        if (m_entries)
            memory::deallocate(m_entries);
        m_indices     = nullptr;
        m_tags        = nullptr;
        m_entries     = nullptr;
        m_size        = 0;
        m_mask        = 0;
        m_num_entries = 0;
        m_entries_cap = 0;
        m_live        = 0;
    }

    // Rebuild the table to size slots, compacting holes and preserving
    // insertion order (insdict rebuild / rebuildEntries).
    void rebuild(unsigned size) {
        unsigned capacity = static_cast<unsigned>((static_cast<uint64_t>(size) * 2) / 3);
        if (size != m_size) {
            // indices followed by size control bytes in one allocation.
            void * mem = memory::allocate(sizeof(int) * size + size);
            if (m_indices)
                memory::deallocate(m_indices);
            m_indices = static_cast<int *>(mem);
            m_tags    = reinterpret_cast<unsigned char *>(m_indices + size);
            m_size    = size;
        }
        memset(m_tags, tag_empty, size);
        for (unsigned i = 0; i < size; i++)
            m_indices[i] = slot_empty;
        m_mask = size - 1;
        entry * old = m_entries;
        unsigned old_n = m_num_entries;
        entry * dst = old;
        if (old == nullptr || m_entries_cap < capacity) {
            dst = static_cast<entry *>(memory::allocate(sizeof(entry) * (capacity == 0 ? 1 : capacity)));
            m_entries_cap = capacity;
        }
        // In-place compaction is safe: the write position never passes the
        // read position.
        unsigned n = 0;
        for (unsigned j = 0; j < old_n; j++) {
            if (!old[j].m_live)
                continue;
            uint64_t slot = free_slot(old[j].m_hash);
            m_indices[slot] = static_cast<int>(n);
            m_tags[slot]    = hash_tag(old[j].m_hash);
            if (dst + n != old + j)
                dst[n] = old[j];
            n++;
        }
        if (dst != old && old != nullptr)
            memory::deallocate(old);
        m_entries     = dst;
        m_num_entries = n;
    }

    // Put: returns the live key equal to k, inserting k if there is none.
    Key & put(Key const & k, bool & inserted) {
        if (m_size == 0)
            rebuild(min_size);
        uint64_t h = get_hash(k);
        unsigned char tag = hash_tag(h);
        uint64_t slot;
        int ix = find_core(k, h, tag, struct_eq(*this), slot);
        if (ix >= 0) {
            inserted = false;
            return m_entries[ix].m_key;
        }
        // New key: rebuild when the table's entry budget is full (holes included).
        if (m_num_entries >= usable()) {
            rebuild(size_for(m_live));
            slot = free_slot(h);
        }
        else if (m_live != m_num_entries) {
            // Deletions leave tombstones: prefer the first one on the probe path.
            slot = free_slot(h);
        }
        ix = static_cast<int>(m_num_entries);
        SASSERT(m_num_entries < m_entries_cap);
        entry & e = m_entries[ix];
        e.m_hash = h;
        e.m_key  = k;
        e.m_live = true;
        m_num_entries++;
        m_indices[slot] = ix;
        m_tags[slot]    = tag;
        m_live++;
        inserted = true;
        return e.m_key;
    }

    template<typename Eq>
    bool del(Key const & k, Eq const & eq) {
        if (m_live == 0)
            return false;
        uint64_t h = get_hash(k);
        uint64_t slot;
        int ix = find_core(k, h, hash_tag(h), eq, slot);
        if (ix < 0)
            return false;
        m_indices[slot] = slot_dummy;
        m_tags[slot]    = tag_dummy;
        m_entries[ix].m_live = false;
        m_entries[ix].m_hash = 0;
        m_entries[ix].m_key  = Key();
        m_live--;
        return true;
    }

public:
    insdict(HashProc const & h = HashProc(), EqProc const & e = EqProc()):
        HashProc(h),
        EqProc(e),
        m_indices(nullptr),
        m_tags(nullptr),
        m_size(0),
        m_mask(0),
        m_entries(nullptr),
        m_num_entries(0),
        m_entries_cap(0),
        m_live(0) {
    }

    ~insdict() {
        free_storage();
    }

    // DeleteAll: drop every entry and the storage.
    void finalize() { free_storage(); }
    void reset() { free_storage(); }

    unsigned size() const { return m_live; }
    bool empty() const { return m_live == 0; }

    // capacity() is the length of the open-addressed index (len(indices)
    // in the Go insdict): 0 before the first insertion, else a power of
    // two >= 8. Entries (holes included) never exceed 2/3 of it.
    unsigned capacity() const { return m_size; }

    // Number of entry positions in use, holes included (len(entries)).
    unsigned num_entries() const { return m_num_entries; }

    void insert(Key const & k) {
        bool inserted;
        put(k, inserted);
    }

    // Returns the key equal to k already in the table, or inserts k and
    // returns it.
    Key insert_if_not_there(Key const & k) {
        bool inserted;
        return put(k, inserted);
    }

    bool insert_if_not_there_core(Key const & k, Key & r) {
        bool inserted;
        r = put(k, inserted);
        return inserted;
    }

    bool find(Key const & k, Key & r) const {
        if (m_live == 0)
            return false;
        uint64_t h = get_hash(k);
        uint64_t slot;
        int ix = find_core(k, h, hash_tag(h), struct_eq(*this), slot);
        if (ix < 0)
            return false;
        r = m_entries[ix].m_key;
        return true;
    }

    bool contains(Key const & k) const {
        Key r;
        return find(k, r);
    }

    // Del: removes the key equal to k, reporting whether it was present.
    // The index is never repacked here, so erasing is safe during iteration.
    bool erase(Key const & k) { return del(k, struct_eq(*this)); }

    // Del of a key known to be stored as itself: entries are matched with
    // operator== instead of EqProc (same result, no structural compare).
    bool erase_identical(Key const & k) { return del(k, ident_eq()); }

    // Pack: drop holes. Without force, only when there are more than 32
    // entries and fewer than 1/4 of them are live.
    void pack(bool force) {
        if (m_live == m_num_entries)
            return;
        if (!force && m_num_entries > 32 && m_live < m_num_entries / 4)
            force = true;
        if (force)
            rebuild(size_for(m_live));
    }

    void swap(insdict & other) {
        std::swap(m_indices,     other.m_indices);
        std::swap(m_tags,        other.m_tags);
        std::swap(m_size,        other.m_size);
        std::swap(m_mask,        other.m_mask);
        std::swap(m_entries,     other.m_entries);
        std::swap(m_num_entries, other.m_num_entries);
        std::swap(m_entries_cap, other.m_entries_cap);
        std::swap(m_live,        other.m_live);
    }

    // All: live keys in insertion order.
    class iterator {
        entry const * m_curr;
        entry const * m_end;
        void move_to_used() {
            while (m_curr != m_end && !m_curr->m_live)
                ++m_curr;
        }
    public:
        iterator(entry const * start, entry const * end): m_curr(start), m_end(end) { move_to_used(); }
        iterator(): m_curr(nullptr), m_end(nullptr) {}
        Key const & operator*() const { return m_curr->m_key; }
        Key const * operator->() const { return &(m_curr->m_key); }
        iterator & operator++() { ++m_curr; move_to_used(); return *this; }
        iterator operator++(int) { iterator tmp = *this; ++*this; return tmp; }
        bool operator==(iterator const & it) const { return m_curr == it.m_curr; }
        bool operator!=(iterator const & it) const { return m_curr != it.m_curr; }
    };

    iterator begin() const { return iterator(m_entries, m_entries + m_num_entries); }
    iterator end() const { return iterator(m_entries + m_num_entries, m_entries + m_num_entries); }

private:
    insdict(insdict const &);
    insdict & operator=(insdict const &);
};

#endif /* INSDICT_H_ */
